package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mattermost/focalboard/server/app"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// --- Linter 修复 (err113): 定义静态错误 ---.
var (
	ErrIntentIsChat          = errors.New("intent is chat, RAG not applicable")
	ErrUnknownIntent         = errors.New("unknown intent, RAG not applicable")
	ErrUnsupportedDBType     = errors.New("RAG executeQuery currently supports sqlite3 only")
	ErrAPIKeyNotSet          = errors.New("DASHSCOPE_API_KEY is not set")
	ErrQwenAPI               = errors.New("qwen api error")
	ErrQwenEmptyChoice       = errors.New("empty choices from qwen")
	ErrGeneratedSQLEmpty     = errors.New("generated SQL is empty")
	ErrGeneratedSQLNotSelect = errors.New("only SELECT is allowed")
	ErrGeneratedSQLForbidden = errors.New("forbidden keyword in SQL")
	ErrGeneratedSQLChars     = errors.New("forbidden characters in SQL")
)

// --- Linter 修复 (goconst): 定义常量字符串 ---.
const (
	intentChat      = "chat"
	intentQueryData = "query_data"
)

// 精简的 Focalboard 相关表结构（仅提供 Text-to-SQL 所需的最小上下文.
const ragSchemaDDL = `
-- boards: 看板
CREATE TABLE boards (
  id TEXT PRIMARY KEY,
  team_id TEXT,
  title TEXT,
  description TEXT,
  create_at INTEGER,
  update_at INTEGER,
  delete_at INTEGER
);

-- blocks: 不同内容块（包括卡片、分组、属性等），其中 type='card' 代表卡片
CREATE TABLE blocks (
  id TEXT PRIMARY KEY,
  board_id TEXT,
  parent_id TEXT,
  root_id TEXT,
  type TEXT,                 -- e.g. 'card', 'view', 'property'
  title TEXT,
  fields TEXT,               -- JSON string that may include assignee_id, status, due_date, etc.
  create_at INTEGER,
  update_at INTEGER,
  delete_at INTEGER
);

-- 典型的查询：按用户筛选其卡片（assignee_id 在 blocks.fields JSON 内部）
-- 例如在 sqlite 中：json_extract(fields, '$.assignee_id') = '<userID>'
`

// RAGService 封装 RAG 主流程.
type RAGService struct {
	app    *app.App
	logger mlog.LoggerIFace
}

func NewRAGService(app *app.App, logger mlog.LoggerIFace) *RAGService {
	return &RAGService{
		app:    app,
		logger: logger,
	}
}

// PrepareRAGResponse: 入口.
// 1) 意图识别：chat -> 返回 error 让外层回退；query_data -> 进入生成 SQL.
// 2) Text-to-SQL：带入 schema / userID / question.
// 3) 执行 SQL：严格安全检查，仅允许 SELECT.
// 4) 构造最终 Prompt：返回给上层用于流式回答.
func (s *RAGService) PrepareRAGResponse(userID string, question string) (string, error) {
	s.logger.Debug("RAGService: PrepareRAGResponse started", mlog.String("user_id", userID), mlog.String("question", question))

	intent, err := s.classifyIntent(question)
	if err != nil {
		s.logger.Error("RAGService: Step 1 (classifyIntent) failed", mlog.Err(err))
		return "", err
	}
	// Linter 修复 (goconst): 使用常量
	if intent == intentChat {
		s.logger.Debug("RAGService: Step 1 (classifyIntent) result is 'chat'. Skipping RAG.")
		return "", ErrIntentIsChat // Linter 修复 (err113): 使用静态错误
	}
	// Linter 修复 (goconst): 使用常量
	if intent != intentQueryData {
		s.logger.Warn("RAGService: Step 1 (classifyIntent) result is unknown. Skipping RAG.", mlog.String("intent", intent))
		return "", ErrUnknownIntent // Linter 修复 (err113): 使用静态错误
	}

	sqlText, err := s.generateSQL(ragSchemaDDL, userID, question)
	if err != nil {
		s.logger.Error("RAGService: Step 2 (generateSQL) failed", mlog.Err(err))
		return "", err
	}

	s.logger.Debug("RAGService: Step 2 (generateSQL) success", mlog.String("sql", sqlText))
	s.logger.Debug("RAGService: Step 3 (executeQuery) starting...")

	contextJSON, err := s.executeQuery(sqlText)
	if err != nil {
		s.logger.Error("RAGService: Step 3 (executeQuery) failed", mlog.Err(err))
		return "", err
	}

	s.logger.Debug("RAGService: Step 3 (executeQuery) success", mlog.Int("json_len", len(contextJSON)))

	finalPrompt := s.buildFinalPrompt(question, contextJSON)

	s.logger.Debug("RAGService: Step 4 (buildFinalPrompt) success. RAG pipeline complete.")
	return finalPrompt, nil
}

// classifyIntent: 调用一次 Qwen（非流式），输出 chat 或 query_data.
func (s *RAGService) classifyIntent(question string) (string, error) {
	prompt := fmt.Sprintf(`你是一个分类器。请只输出一个词：chat 或 query_data。
规则：
- 当用户是在闲聊、问候、或没有明确要求查询项目数据时，输出 chat。
- 当用户在请求和 Focalboard 项目数据相关的统计、筛选、列表、进度等查询时，输出 query_data。

用户问题：
%s

只输出 chat 或 query_data，不要多余解释。`, question)

	out, err := s.callQwenInternal(prompt)
	if err != nil {
		s.logger.Error("RAGService: classifyIntent callQwenInternal failed", mlog.Err(err), mlog.String("prompt", prompt))
		return "", err
	}

	ans := strings.ToLower(strings.TrimSpace(out))

	s.logger.Debug("RAGService: classifyIntent raw response",
		mlog.String("raw_output", out),
		mlog.String("parsed_ans", ans),
	)

	// Linter 修复 (goconst): 使用常量.
	if strings.Contains(ans, intentQueryData) {
		s.logger.Debug("RAGService: Intent classified as 'query_data'")
		return intentQueryData, nil
	}
	// Linter 修复 (goconst): 使用常量.
	if strings.Contains(ans, intentChat) {
		s.logger.Debug("RAGService: Intent classified as 'chat'")
		return intentChat, nil
	}

	s.logger.Warn("RAGService: Intent classification failed (no match), defaulting to 'chat'", mlog.String("raw_output", out))
	// Linter 修复 (goconst): 使用常量.
	return intentChat, nil
}

// generateSQL: 基于 schema / userID / question 生成只读 SQL.
func (s *RAGService) generateSQL(schema string, userID string, question string) (string, error) {
	prompt := fmt.Sprintf(`你是一个 Text-to-SQL 助手。请根据给定的数据库结构 (DDL) 和用户问题，生成一个只读、安全的 SQL。
要求：
- 只生成单条 SELECT 语句，不要包含任何其它内容（不要包含注释、解释、分号）。
- 根据数据库类型为 sqlite（默认）来生成（例如使用 json_extract(fields, '$.assignee_id') 访问 JSON）。
- 必须包含对用户 user_id 的约束：例如筛选 assignee_id = '%s' 的卡片或与之相关的数据。
- 如果问题涉及看板或卡片统计，请合理连接 boards 与 blocks（type='card' 代表卡片）。
- 尽量只返回必要的字段：例如卡片 id、title、board_id、状态、到期时间、更新时间等。
- 确保 WHERE 子句只读安全，不要使用子查询去修改数据。

数据库结构（DDL）：
%s

用户问题：
%s

只输出最终 SQL（仅一行 SELECT 开头的语句），不要任何其它文字。`, userID, schema, question)

	out, err := s.callQwenInternal(prompt)
	if err != nil {
		s.logger.Error("RAGService: generateSQL callQwenInternal failed", mlog.Err(err))
		return "", err
	}

	sqlText := s.extractSQL(strings.TrimSpace(out))

	s.logger.Debug("RAGService: generateSQL raw response", mlog.String("raw_output", out), mlog.String("extracted_sql", sqlText))

	if err := s.validateReadOnlySQL(sqlText); err != nil {
		s.logger.Error("RAGService: generateSQL validation failed", mlog.Err(err), mlog.String("sql", sqlText))
		return "", err
	}
	return sqlText, nil
}

// executeQuery: 对 sqlite3 执行只读查询，并将行序列化为 JSON 数组.
func (s *RAGService) executeQuery(query string) (string, error) {
	cfg := s.app.GetConfig()
	if strings.ToLower(cfg.DBType) != "sqlite3" {
		s.logger.Error("RAGService: executeQuery unsupported DBType", mlog.String("db_type", cfg.DBType))
		return "", ErrUnsupportedDBType // Linter 修复 (err113): 使用静态错误.
	}

	dbPath := cfg.DBConfigString
	if strings.TrimSpace(dbPath) == "" {
		dbPath = "./focalboard.db"
	}

	s.logger.Debug("RAGService: executeQuery connecting to DB", mlog.String("db_path", dbPath))

	dsn := dbPath + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		s.logger.Error("RAGService: executeQuery sql.Open failed", mlog.Err(err))
		return "", err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		s.logger.Error("RAGService: executeQuery db.Query failed", mlog.Err(err), mlog.String("sql", query))
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		s.logger.Error("RAGService: executeQuery rows.Columns failed", mlog.Err(err))
		return "", err
	}

	result := make([]map[string]interface{}, 0, 50)
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err = rows.Scan(valuePtrs...); err != nil {
			s.logger.Error("RAGService: executeQuery rows.Scan failed", mlog.Err(err))
			return "", err
		}
		rowMap := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			rowMap[col] = values[i]
		}
		result = append(result, rowMap)
	}
	if err = rows.Err(); err != nil {
		s.logger.Error("RAGService: executeQuery rows.Err failed", mlog.Err(err))
		return "", err
	}

	data, err := json.Marshal(result)
	if err != nil {
		s.logger.Error("RAGService: executeQuery json.Marshal failed", mlog.Err(err))
		return "", err
	}

	s.logger.Debug("RAGService: executeQuery success", mlog.Int("row_count", len(result)))
	return string(data), nil
}

// buildFinalPrompt: 把用户问题与上下文数据拼成最终给 LLM 的 Prompt.
func (s *RAGService) buildFinalPrompt(question string, contextData string) string {
	var b strings.Builder
	b.WriteString("你是一个友好的 Focal Board 助手。请根据用户的问题, 和我为你提供的实时数据, 来生成一个自然、友好的回答。\n\n")
	b.WriteString("用户问题: \"")
	b.WriteString(question)
	b.WriteString("\"\n\n实时数据 (JSON 格式):\n")
	b.WriteString(contextData)
	b.WriteString("\n\n你的回答:\n")
	return b.String()
}

// callQwenInternal: 使用当前项目中配置的阿里云百炼（OpenAI 兼容）接口进行一次非流式调用.
func (s *RAGService) callQwenInternal(prompt string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY"))
	if apiKey == "" {
		s.logger.Error("RAGService: callQwenInternal DASHSCOPE_API_KEY is not set")
		return "", ErrAPIKeyNotSet // Linter 修复 (err113): 使用静态错误
	}
	model := strings.TrimSpace(os.Getenv("DASHSCOPE_MODEL"))
	if model == "" {
		model = "qwen-plus"
	}
	url := "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream":      false,
		"temperature": 0.2, // 意图识别和 SQL 生成需要低T.
		"max_tokens":  800,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		s.logger.Error("RAGService: callQwenInternal http.NewRequest failed", mlog.Err(err))
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		s.logger.Error("RAGService: callQwenInternal httpClient.Do failed", mlog.Err(err))
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slurp, _ := ioReadAllLimit(resp.Body, 4<<20)
		s.logger.Error("RAGService: callQwenInternal API error", mlog.Int("status", resp.StatusCode), mlog.String("body", string(slurp)))
		// Linter 修复 (err113): 使用 %w 包装静态错误.
		return "", fmt.Errorf("%w: %d: %s", ErrQwenAPI, resp.StatusCode, string(slurp))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		s.logger.Error("RAGService: callQwenInternal response JSON decode failed", mlog.Err(err))
		return "", err
	}
	if len(parsed.Choices) == 0 {
		s.logger.Error("RAGService: callQwenInternal API returned empty choices")
		return "", ErrQwenEmptyChoice // Linter 修复 (err113): 使用静态错误.
	}
	return parsed.Choices[0].Message.Content, nil
}

// 从模型输出中抽取 SQL，支持三重反引号包裹、或纯文本.
func (s *RAGService) extractSQL(text string) string {
	// 优先匹配 ```sql ... ```
	re := regexp.MustCompile("(?s)```sql\\s*(SELECT[\\s\\S]*?)```")
	if m := re.FindStringSubmatch(text); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// 退化到第一行以 SELECT 开头的语句.
	lines := strings.Split(text, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(strings.ToUpper(ln), "SELECT") {
			// 去掉尾部分号
			return strings.TrimRight(ln, ";")
		}
	}
	// 如果上面都找不到, 返回原始文本(去掉分号), 让 validateReadOnlySQL 来处理.
	return strings.TrimRight(text, ";")
}

// 只读 SQL 校验：只允许 SELECT，禁止危险关键字与多语句.
func (s *RAGService) validateReadOnlySQL(sqlText string) error {
	if sqlText == "" {
		return ErrGeneratedSQLEmpty // Linter 修复 (err113): 使用静态错误.
	}
	up := strings.ToUpper(strings.TrimSpace(sqlText))
	if !strings.HasPrefix(up, "SELECT") {
		// Linter 修复 (err113): 使用 %w 包装.
		return fmt.Errorf("%w: %s", ErrGeneratedSQLNotSelect, sqlText)
	}

	forbiddenKeywords := []string{"DELETE", "UPDATE", "DROP", "INSERT", "TRUNCATE", "ALTER"}
	for _, kw := range forbiddenKeywords {
		// \b 匹配一个单词边界.
		re, err := regexp.Compile(`\b` + kw + `\b`)
		if err != nil {
			s.logger.Error("RAGService: validateReadOnlySQL regex compile failed", mlog.Err(err), mlog.String("keyword", kw))
			return fmt.Errorf("regex compile error for %s: %w", kw, err)
		}
		if re.MatchString(up) {
			// Linter 修复 (err113): 使用 %w 包装.
			return fmt.Errorf("%w: %s", ErrGeneratedSQLForbidden, kw)
		}
	}

	forbiddenChars := []string{";", "--", "/*"}
	for _, kw := range forbiddenChars {
		if strings.Contains(up, kw) {
			// Linter 修复 (err113): 使用 %w 包装.
			return fmt.Errorf("%w: %s", ErrGeneratedSQLChars, kw)
		}
	}

	return nil
}

// ioReadAllLimit: 安全读取响应体（限制大小）.
func ioReadAllLimit(reader io.Reader, limit int64) ([]byte, error) {
	lr := &io.LimitedReader{R: reader, N: limit}
	return io.ReadAll(lr)
}
