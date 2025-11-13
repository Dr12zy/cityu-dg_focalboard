package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/focalboard/server/model"
	"github.com/mattermost/focalboard/server/services/audit"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// AIRequest represents the chat request from the frontend.
type AIRequest struct {
	Message     string    `json:"message"`
	Messages    []Message `json:"messages,omitempty"` // For conversation history.
	Stream      bool      `json:"stream,omitempty"`   // Whether to use streaming.
	Model       string    `json:"model,omitempty"`    // AI model to use.
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// AIResponse represents a non-streaming response.
type AIResponse struct {
	Message string `json:"message"`
	Model   string `json:"model,omitempty"`
}

// AIStreamChunk represents a chunk in streaming response (Server-Sent Events).
type AIStreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// OpenAIRequest represents the request format for OpenAI API (and Qwen compatible mode).
type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"` // 关键：这个必须为 true
}

// OpenAIResponse represents the response format from OpenAI API (and Qwen compatible mode).
type OpenAIResponse struct {
	Choices []struct {
		// 在流式模式下, 我们会收到 Delta (增量)
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		// 在非流式模式下, 我们会收到 Message
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Model string `json:"model"`
}

func (a *API) registerAIRoutes(r *mux.Router) {
	// AI chat APIs
	r.HandleFunc("/ai/chat", a.sessionRequired(a.handleAIChat)).Methods("POST")

	// 把路由从 "Not Implemented" 改回到指向 handleAIChatStream
	r.HandleFunc("/ai/chat/stream", a.sessionRequired(a.handleAIChatStream)).Methods("POST")
}

// handleAIChat (非流式) 保持不变, 作为对比.
func (a *API) handleAIChat(w http.ResponseWriter, r *http.Request) {
	// ... (swagger comments remain the same) ...
	userID := getUserID(r)

	auditRec := a.makeAuditRecord(r, "aiChat", audit.Fail)
	defer a.audit.LogRecord(audit.LevelRead, auditRec)
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}
	var aiReq AIRequest
	if err = json.Unmarshal(requestBody, &aiReq); err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
		return
	}
	messages := buildMessages(aiReq)
	apiKey, apiURL, modelName := a.getAIConfig(aiReq.Model)
	if apiKey == "" {
		a.errorResponse(w, r, model.NewErrBadRequest("AI API key not configured (DASHSCOPE_API_KEY)"))
		return
	}
	oreq := OpenAIRequest{
		Model:       modelName,
		Messages:    messages,
		Stream:      false, // 非流式
		Temperature: aiReq.Temperature,
		MaxTokens:   aiReq.MaxTokens,
	}
	if oreq.Temperature == 0 {
		oreq.Temperature = 0.7
	}
	if oreq.MaxTokens == 0 {
		oreq.MaxTokens = 2000
	}
	reqBody, err := json.Marshal(oreq)
	if err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
		return
	}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		a.logger.Error("AI API request failed", mlog.Err(err))
		a.errorResponse(w, r, model.NewErrBadRequest("Failed to connect to AI service"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Error("AI API returned error",
			mlog.Int("status", resp.StatusCode),
			mlog.String("body", string(body)),
		)
		a.errorResponse(w, r, model.NewErrBadRequest(fmt.Sprintf("AI API error: %d", resp.StatusCode)))
		return
	}
	var oresp OpenAIResponse
	if err = json.NewDecoder(resp.Body).Decode(&oresp); err != nil {
		a.errorResponse(w, r, err)
		return
	}
	if len(oresp.Choices) == 0 {
		a.errorResponse(w, r, model.NewErrBadRequest("No response from AI"))
		return
	}
	outMsg := oresp.Choices[0].Message.Content // 注意: 非流式用 'Message'
	outModel := oresp.Model
	response := AIResponse{
		Message: outMsg,
		Model:   outModel,
	}
	data, err := json.Marshal(response)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}
	a.logger.Debug("AIChat",
		mlog.String("userID", userID),
		mlog.String("model", modelName),
	)
	jsonBytesResponse(w, http.StatusOK, data)
	auditRec.Success()
}

// handleAIChatStream handles streaming AI chat requests (Server-Sent Events).
func (a *API) handleAIChatStream(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /api/v2/ai/chat/stream aiChatStream
	// ... (swagger comments) ...

	userID := getUserID(r)

	auditRec := a.makeAuditRecord(r, "aiChatStream", audit.Fail)
	defer a.audit.LogRecord(audit.LevelRead, auditRec)

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}

	var aiReq AIRequest
	if err = json.Unmarshal(requestBody, &aiReq); err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
		return
	}

	// --------------------------------------------------------------------
	// ↓↓↓↓↓↓ 【RAG 核心逻辑】 ↓↓↓↓↓↓
	// --------------------------------------------------------------------

	// 2. 尝试调用 RAG 服务
	finalPrompt, err := a.ragService.PrepareRAGResponse(userID, aiReq.Message)

	var streamMessages []Message
	if err != nil {
		// 3a. RAG 失败 (例如意图是 'chat', 或者 RAG 崩溃了)
		//    我们打印日志, 然后回退到使用用户的原始消息
		a.logger.Warn("RAGService: PrepareRAGResponse failed, falling back to original message.", mlog.Err(err))
		streamMessages = buildMessages(aiReq)
	} else {
		// 3b. RAG 成功!
		//    我们使用 RAG 服务返回的“最终 Prompt”
		a.logger.Debug("RAGService: PrepareRAGResponse success, using augmented prompt.")
		streamMessages = []Message{
			{Role: "user", Content: finalPrompt},
		}
	}
	// --------------------------------------------------------------------
	// ↑↑↑↑↑↑ 【RAG 逻辑结束】 ↑↑↑↑↑↑
	// --------------------------------------------------------------------

	// 4. 获取 AI API 配置
	apiKey, apiURL, modelName := a.getAIConfig(aiReq.Model)
	if apiKey == "" {
		a.errorResponse(w, r, model.NewErrBadRequest("AI API key not configured (DASHSCOPE_API_KEY)"))
		return
	}

	// 5. 准备 Qwen 请求 (注意: Messages 使用的是我们刚处理过的 streamMessages)
	oreq := OpenAIRequest{
		Model:       modelName,
		Messages:    streamMessages, // <-- 关键点在这里!
		Stream:      true,
		Temperature: aiReq.Temperature,
		MaxTokens:   aiReq.MaxTokens,
	}
	if oreq.Temperature == 0 {
		oreq.Temperature = 0.7
	}
	if oreq.MaxTokens == 0 {
		oreq.MaxTokens = 2000
	}

	reqBody, err := json.Marshal(oreq)
	if err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
		return
	}

	// Call AI API
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		a.logger.Error("AI API request failed", mlog.Err(err))
		a.errorResponse(w, r, model.NewErrBadRequest("Failed to connect to AI service"))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Error("AI API returned error",
			mlog.Int("status", resp.StatusCode),
			mlog.String("body", string(body)),
		)
		a.errorResponse(w, r, model.NewErrBadRequest(fmt.Sprintf("AI API error: %d", resp.StatusCode)))
		return
	}

	// 设置 Server-Sent Events (SSE) 的响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 循环读取流式响应
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var oresp OpenAIResponse
			if err := json.Unmarshal([]byte(data), &oresp); err == nil {
				if len(oresp.Choices) > 0 {
					content := oresp.Choices[0].Delta.Content
					if content != "" {
						chunk := AIStreamChunk{
							Content: content,
							Done:    false,
						}
						chunkData, _ := json.Marshal(chunk)
						fmt.Fprintf(w, "data: %s\n\n", chunkData)
						w.(http.Flusher).Flush()
					}
					if oresp.Choices[0].FinishReason != "" {
						break
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		a.logger.Error("Error reading stream", mlog.Err(err))
	}

	finalChunk := AIStreamChunk{
		Content: "",
		Done:    true,
	}
	chunkData, _ := json.Marshal(finalChunk)
	fmt.Fprintf(w, "data: %s\n\n", chunkData)
	w.(http.Flusher).Flush()

	a.logger.Debug("AIChatStream",
		mlog.String("userID", userID),
		mlog.String("model", modelName),
	)
	auditRec.Success()
}

// buildMessages (保持不变).
func buildMessages(aiReq AIRequest) []Message {
	var messages []Message
	if len(aiReq.Messages) > 0 {
		messages = aiReq.Messages
	} else if aiReq.Message != "" {
		messages = []Message{
			{
				Role:    "user",
				Content: aiReq.Message,
			},
		}
	}
	return messages
}

// getAIConfig (保持不变).
func (a *API) getAIConfig(requestedModel string) (apiKey, apiURL, modelName string) {
	modelName = requestedModel
	if modelName == "" {
		modelName = "qwen-plus"
	}
	apiKey = getEnv("DASHSCOPE_API_KEY", "")
	apiURL = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	return apiKey, apiURL, modelName
}

// getEnv (保持不变).
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
