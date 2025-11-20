Focal Board AI (RAG) - 后端配置与运行指南

本文档用于说明如何配置和启动已集成 AI RAG 功能的 Focal Board 后端服务。

此功能 (ai_rag_service.go) 使用 Text-to-SQL 管道，通过阿里云百炼 (DashScope) 的 Qwen 模型实现对 focalboard.db (SQLite) 数据库的实时查询。

1. 环境变量配置 (必须)

在启动服务器之前，必须在环境中设置以下两个环境变量。

1.1 DASHSCOPE_API_KEY

此变量用于 Qwen 模型的 API 认证。

用途: RAG 管道中的“意图识别”和“SQL 生成”步骤 (callQwenInternal)，以及 AI 聊天（handleAIChatStream）都需要此 Key。

如果缺失: RAG 管道将因 401 invalid_api_key 错误而失败，并回退到纯聊天模式。纯聊天模式也将因 401 错误而失败。

示例: export DASHSCOPE_API_KEY="sk-..."

1.2 FOCALBOARD_SINGLE_USER_TOKEN

此变量用于 --single-user 模式的身份认证。

用途: 当使用 --single-user 标志启动服务器时，程序强制要求此环境变量必须存在。

前端调用: 前端（或 cURL）发送的 Authorization: Bearer <token> 中的 <token> 必须与此环境变量的值完全匹配。

示例: export FOCALBOARD_SINGLE_USER_TOKEN="test-token"

2. 编译与运行 (本地)

在上传到 GitHub (CI/CD) 或在本地运行时，请遵循以下编译和运行步骤。

2.1 编译 (Build)

每次修改 .go 源代码后 (例如 ai.go, ai_rag_service.go)，都必须重新编译以生成新的可执行文件。

# 1. (可选) 清理旧的编译产物
make clean

# 2. 编译服务器
# 这将读取 server/api/ 目录下的所有 .go 文件
# 并生成新的 ./bin/focalboard-server
make server


2.2 运行 (Run)

编译完成后，使用以下命令启动服务：

# 1. 设置 Qwen Key
export DASHSCOPE_API_KEY="sk-..."

# 2. 设置单用户 Token
export FOCALBOARD_SINGLE_USER_TOKEN="test-token"

# 3. 运行已编译的程序
./bin/focalboard-server --config ./server-config.json --single-user
