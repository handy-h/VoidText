# VoidText 仓库指南

## 项目概述

Go 单模块项目（`module voidtext`），用于清洗中文小说 TXT 文件。核心是五步处理流水线：基础清洗→向量去重→LLM 修复→人工审核→生成文件。

## 关键命令

```bash
# 开发（控制台日志 + Gin debug 模式，无需提前编译）
make dev

# 编译（自动 go mod tidy/verify）
make build

# 运行已编译的二进制
make run

# 后台运行（日志重定向到 /dev/null）
make run-background

# 停止进程
make stop

# 重新编译（停止 + 清理 + 编译）
make rebuild

# 测试
go test ./...
go test -v -count=1 ./internal/processor/  # 单包测试，禁用缓存

# 代码检查
go vet ./...
go fmt ./...
```

## 架构要点

### 目录结构

```
cmd/voidtext/main.go          入口：加载配置 → 初始化 SQLite → 启动 Gin
internal/config/               .env 加载器，AppConfig 全局单例
internal/database/             SQLite CRUD（WAL 模式，MaxOpenConns=1 防写冲突）
internal/processor/            核心处理层（pipeline.go 定义五步常量）
  preprocess/                  编码检测（GBK/UTF-8）、文本规范化
  postprocess/                 输出格式化
  rules/                       正则规则引擎（规则存于 rules.json）
internal/external/             LLM API 客户端(api.go) + Ollama 客户端(ollama.go)
web/backend/                   Gin 路由、handlers、middleware
  middleware/                  auth.go（API Token）、error_handler.go、rate_limit.go
web/frontend/                  纯原生 JS SPA，无构建步骤，Gin 直接 serve 静态文件
config/prompts/                提示词文件（热重载，命名：{name}_{version}.txt）
```

### 五步处理流水线（`internal/processor/pipeline.go`）

| 步骤         | 功能                                          | 核心文件                          |
| ------------ | --------------------------------------------- | --------------------------------- |
| `cleaning`   | 基础清洗：编码转换、广告移除、繁简转换        | `basic_cleaner.go`, `preprocess/` |
| `indexing`   | 向量相似度检测，移除重复段落                  | `vector_detector.go`              |
| `llm_fix`    | LLM 纠错（本地 Ollama → 远程 API → 本地字典） | `model_repairer.go`, `external/`  |
| `review`     | 人工审核修改建议（通过/拒绝/编辑）            | `database/review_repo.go`         |
| `finalizing` | 合并结果，生成最终文件                        | `postprocess/`                    |

### 并发模型

- `worker_pool.go`：全局 `WorkerPool` 单例，文件处理任务以 `FileProcessingJob` 形式入队
- SQLite 使用 `SetMaxOpenConns(1)` 序列化写操作（WAL 模式支持并发读）
- `progress_tracker.go`：跟踪各文件处理进度，供前端轮询

### 文件身份与版本

- 文件以 **MD5 内容哈希** 为唯一标识（上传时去重）
- 数据库表：`files`、`versions`（版本链，含父子关系）、`review_items`、`processing_logs`、`chunk_repair_cache`、`retry_queue`、`prompt_versions`
- 数据库文件：`{DATA_DIR}/cleaning.db`（默认 `./data/cleaning.db`，已 gitignore）

### 提示词管理（`internal/processor/prompt_manager.go`）

加载优先级：数据库 `prompt_versions` 表 → `config/prompts/{name}_{version}.txt`（热重载） → 硬编码默认值

### MCP 服务

`crush.json` 配置了 `code-context` MCP 服务，提供以下工具：

- **code_search** - 语义搜索代码片段（按业务含义模糊查找）
- **symbol_search** - 精确符号查找（函数名、变量名、类型名）
- **impact_analysis** - 修改影响分析（删除/重命名/修改签名后的依赖分析）
- **file_context** - 文件结构摘要（快速了解大文件结构）
- **index_project** - 重建代码索引（当搜索结果不匹配时使用）

使用前需确保 Ollama 运行且已拉取嵌入模型。**在需要搜索代码逻辑时，必须优先使用这些工具**。

## 编码约定

- Go 1.25+ 必须（见 `go.mod`）
- `modernc.org/sqlite` 纯 Go 实现，**无需 CGO**
- 缩进 2 空格，K&R 大括号，行宽 ≤ 120 字符
- 变量/函数 camelCase，布尔型加 `is`/`has`/`can` 前缀；类型 PascalCase；常量 UPPER_SNAKE_CASE
- 数据库字段 snake_case；文件名 kebab-case
- 禁用 `any`，严格类型；函数体 ≤ 80 行；禁止魔法值
- 测试函数命名：`should_行为_条件`（中文描述条件）
- API 响应统一格式：`{ code, message, data }`

## 配置与环境

- 从 `.env` 文件加载（使用 `godotenv`），模板见 `.env.template`
- `LOG_TO_CONSOLE=true` 控制台打印日志（`make dev` 自动设置）
- `GIN_MODE=debug` 显示路由详情（`make dev` 自动设置）
- 旧配置键（`EXTERNAL_API_URL` 等）自动映射到新键，保持向后兼容
- `API_TOKEN` 未设置则跳过鉴权（适合本地开发）；设置后客户端须在请求头 `X-API-Token` 传值
- `data/` 目录在运行时自动创建，已 gitignore

## 测试

- 测试文件放在源文件旁边或现有 `test/` 子目录
- 使用 `DATA_DIR=./test_data` 隔离测试数据
- 冒烟测试需要先启动服务器，且鉴权需关闭或提供有效 API Token
- 测试脚本：`./testing/unit_testing/run_unit_tests.sh [module]` 和 `./testing/smoking_testing/run_smoke_tests.sh [server-url]`

## 注意事项

- 上传文件名格式建议 `作者 - 标题.txt`，分隔符可通过 `NAME_SEPARATORS` 配置
- 混合模型架构降级顺序：本地 Ollama → 远程 LLM API → 本地字典
- `ENABLE_EVOLVER=true` 启用自进化监控（调用 `scripts/evolver.py`，需 Python）
- 前端是纯原生 JS SPA，无构建步骤，修改后刷新浏览器即可生效
- 规则引擎配置在 `rules.json`，支持运行时通过 API 修改
- 【重要】优先通过 code-context MCP 查询语义相关的代码片段，比纯 grep 更准确。

## 工具使用规范与优先级 (Tool Usage & Priority)

在探索、理解和分析本项目代码库时，**必须严格遵守以下工具调用优先级**。禁止直接使用效率低下的模糊搜索（如全局 Grep）或盲目读取大量文件源码，应优先使用专门的 Code Context MCP 服务。

### 1. 代码检索与理解决策流

当你需要寻找特定逻辑、函数或分析修改影响时，请遵循以下场景决策：

* **场景 A：寻找某种业务逻辑或功能描述**（例如："处理用户认证"、"配置加载相关代码"）
  * 🚀 **行动**：必须优先调用 `code_search` 进行语义搜索。
* **场景 B：已知具体函数名、类名或变量名**（例如：查找 `LoadConfig` 函数的所有引用）
  * 🚀 **行动**：必须优先调用 `symbol_search` 进行精确符号查找。
* **场景 C：准备修改或重构某个核心函数、接口**（例如：删除/重命名某函数后影响哪些文件）
  * 🚀 **行动**：必须在修改前调用 `impact_analysis` 评估影响范围。
* **场景 D：需要了解某个大文件的整体结构**（例如：先看摘要定位函数，再精确读取）
  * 🚀 **行动**：优先调用 `file_context` 获取结构摘要，严禁直接 Read 整个大文件。

### 2. 工具优先级矩阵 (Priority Matrix)

| 目标场景 | 🟢 首选工具 (MCP) | 🔴 禁用/降级工具 | 理由 |
| :--- | :--- | :--- | :--- |
| **按业务含义/功能模糊查找** | `code_search` | 全局 Grep / Glob | 语义搜索能更精准定位跨文件的关联逻辑，减少无关噪音。 |
| **查找特定符号及引用** | `symbol_search` | 文本全局搜索 | 符号查找具备 AST（语法树）级别精准度，避免同名字符串干扰。 |
| **评估修改影响/重构** | `impact_analysis` | 人肉追踪 / 逐个文件 Grep | 自动分析调用链，防止遗漏依赖。 |
| **快速熟悉新文件** | `file_context` | 直接读取整文件内容 | 节省 Context Token，先看摘要，按需精读。 |

### 3. 强制约束指标

* **禁止行为**：禁止在未进行符号或语义搜索的情况下，盲目遍历读取（Read）超过 3 个以上的文件。
* **索引维护**：如果发现本地索引未建立，或代码发生重大变更导致搜索结果不匹配，请主动调用 `index_project` 重新构建索引。