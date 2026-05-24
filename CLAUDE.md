# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

**湮文 VoidText** — 基于 Go 语言的中文小说 TXT 文本清洗工具。核心是一条五步处理流水线，将原始 TXT 文件经由基础清洗→向量去重→LLM 修复→人工审核→生成文件，最终输出高质量文本。提供 Web 界面（SPA），后端使用 Gin + SQLite。

## 常用命令

```bash
# 开发（go run，控制台日志，Gin debug 模式）
make dev

# 编译
make build

# 运行已编译的二进制
make run

# 后台运行（日志重定向 /dev/null）
make run-background

# 停止进程
make stop

# 停止 + 删除编译产物（保留数据目录）
make clean

# 停止 + 清理 + 重新编译
make rebuild

# 所有 Go 测试
go test ./...

# 单包测试（禁用缓存，显示详情）
go test -v -count=1 ./internal/processor/

# 联合测试脚本（设置 DATA_DIR=./test_data，运行前后自动清理）
./testing/unit_testing/run_unit_tests.sh [module]

# 冒烟测试（需要先启动服务器）
./testing/smoking_testing/run_smoke_tests.sh [server-url]

# 代码检查
go vet ./...
go fmt ./...
```

## 架构概览

### 分层结构

```
cmd/voidtext/main.go       入口：加载配置 → 初始化 SQLite → 启动 Gin 服务器
internal/config/           .env 加载器，AppConfig 全局单例
internal/database/         SQLite CRUD（WAL 模式，MaxOpenConns=1 防止写冲突）
internal/processor/        核心处理层（见下方详细说明）
  preprocess/              编码检测（GBK/UTF-8）、文本规范化
  postprocess/             输出格式化
  rules/                   正则规则引擎（RuleManager，规则存于 rules.json）
internal/errors/           统一错误码（ErrorCode 常量 + AppError 结构体）
internal/file/             MD5 计算、文件名→作者+标题解析
internal/external/         LLM API 客户端(api.go) + Ollama 客户端(ollama.go)
internal/logging/          结构化 JSON 日志
web/backend/               Gin 路由、handlers、middleware
  middleware/              auth.go（API Token）、error_handler.go、rate_limit.go、no_cache.go
web/frontend/              单页应用（index.html + static/js/modules/）
scripts/evolver.py         Python 脚本，用于提示词自进化优化
repowikis/                 中文技术文档（架构、API、部署、开发指南）
```

### 五步处理流水线（`internal/processor/pipeline.go`）

步骤常量定义于 `pipeline.go`，按顺序执行：

| 步骤常量 | 功能 | 核心文件 |
|---|---|---|
| `cleaning` | 基础清洗：编码转换(GBK/UTF-8)、广告移除、繁简转换 | `basic_cleaner.go`, `preprocess/` |
| `indexing` | 向量相似度检测，移除重复段落 | `vector_detector.go` |
| `llm_fix` | 调用 LLM 纠错（本地 Ollama 优先，降级到远程 API，再降级到本地字典） | `model_repairer.go`, `external/` |
| `review` | 人工逐条审核修改建议（通过/拒绝/编辑），通过 DB `review_items` 表直接操作 | `database/review_repo.go` |
| `finalizing` | 合并审核结果，生成最终文件并写版本记录 | `postprocess/` |

每步完成后保存中间文件和版本记录，支持断点续传。跳过的步骤自动推进到下一步。

### 并发模型

- `worker_pool.go`：全局 `WorkerPool` 单例，文件处理任务以 `FileProcessingJob` 形式入队
- SQLite 使用 `SetMaxOpenConns(1)` 序列化写操作（WAL 模式支持并发读）
- `progress_tracker.go`：跟踪各文件处理进度，供前端轮询

### 文件身份与版本

- 文件以 **MD5 内容哈希** 为唯一标识（上传时去重）
- 数据库表：`files`、`versions`（版本链，含父子关系）、`review_items`、`processing_logs`、`chunk_repair_cache`、`retry_queue`、`prompt_versions`
- 数据库文件：`{DATA_DIR}/cleaning.db`（默认 `./data/cleaning.db`，gitignore）

### 提示词管理（`internal/processor/prompt_manager.go`）

加载优先级：数据库 `prompt_versions` 表 → `config/prompts/{name}_{version}.txt`（热重载） → 硬编码默认值

### 前端

纯原生 JS SPA，无构建步骤。模块位于 `web/frontend/static/js/modules/`（`file-manager.js`、`processing.js`、`review.js`、`reader-settings.js`、`rules-config.js`、`theme.js`）。Gin 直接 serve 静态文件。

## 关键约定

### 编码风格

- 缩进 2 空格，K&R 大括号，行宽 ≤ 120 字符
- 变量/函数 camelCase，布尔型加 `is`/`has`/`can` 前缀；类型 PascalCase；常量 UPPER_SNAKE_CASE
- 数据库字段 snake_case；文件名 kebab-case
- 禁用 `any`，严格类型；函数体 ≤ 80 行；禁止魔法值
- 测试函数命名：`should_行为_条件`（中文描述条件）
- API 响应统一格式：`{ code, message, data }`

### 配置

- 从 `.env` 文件加载（使用 `godotenv`），模板见 `.env.template`
- `LOG_TO_CONSOLE=true` 控制台打印日志（`make dev` 自动设置）
- `GIN_MODE=debug` 显示路由详情（`make dev` 自动设置）
- 旧配置键（`EXTERNAL_API_URL` 等）自动映射到新键，保持向后兼容

### 注意事项

- Go 1.25+ 必须（见 `go.mod`）
- `modernc.org/sqlite` 纯 Go 实现，**无需 CGO**
- 上传文件名格式建议 `作者 - 标题.txt`，分隔符可通过 `NAME_SEPARATORS` 配置
- 混合模型架构降级顺序：本地 Ollama → 远程 LLM API → 本地字典
- `ENABLE_EVOLVER=true` 启用自进化监控（调用 `scripts/evolver.py`，需 Python）
- `data/` 目录在运行时自动创建，已 gitignore
- **认证**：`API_TOKEN` 未设置则跳过鉴权（适合本地开发）；设置后客户端须在请求头 `X-API-Token` 传值。冒烟测试前确认 token 状态
