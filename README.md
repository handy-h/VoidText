# 湮文 VoidText

基于 Go 语言的小说文本清洗工具，用于解决小说 TXT 文档中的错别字、乱码、广告内容、重复或缺失片段等问题。

![项目图片](./湮文VoidText.png)

> 归于寂灭，方见真章

## 功能特性

- **五步处理流水线**：基础清洗 → 向量检测 → LLM修复 → 人工审核 → 生成文件
- **文件生命周期管理**：MD5 唯一标识、状态追踪（pending/processing/completed/failed）、断点续传
- **版本链管理**：自动维护原始文件与中间版本的父子关系，任何版本可追溯
- **基础文本清洗**：编码检测与转换（GBK/UTF-8）、广告移除、特殊字符处理、繁体转简体
- **向量检测去重**：基于向量相似度的重复段落检测与移除
- **LLM修复**：调用外部 LLM 纠正错别字和语法错误，支持本地字典兜底
- **人工审核**：逐条审核修改建议，支持通过/拒绝/编辑/恢复/批量操作
- **自定义规则**：每个文件可独立配置规则（错别字映射、广告黑名单等）
- **处理报告**：生成包含审核统计、版本历史、处理日志的完整报告
- **SQLite持久化**：所有数据本地存储，无需外部数据库依赖
- **混合架构**：支持本地 Ollama 模型与远程 API 混合使用，智能降级
- **自进化监控**：自动监控缓存命中率和API错误率，优化提示词
- **智能分块**：自动将长文本分块处理，支持并发Worker池
- **API限流与重试**：指数退避+抖动、失败区块重试队列
- **动态提示词管理**：支持多版本提示词、热重载、自进化优化

## 环境依赖

- **Go**: 1.25+
- **SQLite**: 无需单独安装（使用 modernc.org/sqlite 驱动）
- **外部服务**（可选）:
  - 远程 LLM API（阿里云 DashScope、DeepSeek、OpenAI 等）
  - 本地 Ollama 模型（用于混合架构，低成本优先）
  - 向量模型（用于语义相似度检测，默认本地计算）

## 提示词管理

VoidText 采用动态提示词管理系统，支持多版本、热重载和自进化优化。

### 提示词来源（按优先级）

1. **数据库存储** (`prompt_versions` 表)
   - Evolver 优化后的提示词自动保存到数据库
   - 系统重启后从数据库加载最新版本

2. **文件系统** (`config/prompts/` 目录)
   - 支持热重载：修改文件后自动生效（无需重启）
   - 文件命名格式：`{prompt_name}_{version}.txt`
   - 示例：`config/prompts/novel_repair_v1.0.1.txt`

3. **默认硬编码提示词**
   - 当数据库和文件都不可用时使用
   - 当前默认提示词：`"You are a professional Chinese novel proofreader. Please correct typos and grammatical errors in the following text while preserving the original meaning. Only output the corrected text, no explanations."`

### 配置提示词

#### 1. 文件方式（推荐用于开发环境）
```bash
# 创建提示词目录
mkdir -p config/prompts

# 创建提示词文件
cat > config/prompts/novel_repair_v1.0.0.txt << 'EOF'
你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。

重要要求：
1. 只修正明显的错别字和语法错误，不要改变原文意思
2. 保持口语化表达，不要过度正式化
3. 如果原文没有错误，直接返回原文
4. 输出格式：只输出修正后的文本，不要添加任何额外说明

示例：
输入：她高兴及了，跑过去抱住他。
输出：她高兴极了，跑过去抱住他。

当前任务：请修正以下文本：
EOF
```

#### 2. 数据库方式（生产环境）
- 系统自动管理，无需手动干预
- Evolver 优化后的提示词自动保存
- 可通过 API 查看当前使用的提示词版本

#### 3. 环境变量配置
```env
# 自进化监控配置
ENABLE_EVOLVER=false                    # 是否启用自进化监控
EVOLVER_ERROR_RATE_THRESHOLD=0.2        # 错误率阈值（>20%触发优化）
EVOLVER_HIT_RATE_THRESHOLD=0.3          # 命中率阈值（<30%触发优化）
EVOLVER_CHECK_INTERVAL=300              # 检查间隔（秒）
```

### 自进化监控（Evolver）

当启用自进化监控时，系统会：
1. **监控性能指标**：API错误率、缓存命中率
2. **触发优化**：当错误率 >20% 或命中率 <30% 时自动优化提示词
3. **调用优化脚本**：执行 `scripts/evolver.py` 生成优化后的提示词
4. **保存新版本**：将优化后的提示词保存到数据库，并更新当前使用的提示词

### 查看当前提示词

#### 通过日志查看
启动服务时，日志会显示当前使用的提示词版本：
```
[ModelRepairer] 提示词管理器初始化完成 (版本: v1.0.0, 来源: database, 长度: 256)
```

#### 通过数据库查看
```sql
-- 查看所有提示词版本
SELECT prompt_name, prompt_version, source, success_rate, total_uses, created_at 
FROM prompt_versions 
ORDER BY created_at DESC;

-- 查看最新版本
SELECT prompt_name, prompt_version, prompt_content, source, success_rate
FROM prompt_versions 
WHERE prompt_name = 'novel_repair'
ORDER BY created_at DESC 
LIMIT 1;
```

### 手动管理提示词

#### 重置为默认提示词
```bash
# 通过 API 重置
curl -X POST http://localhost:8080/api/prompts/reset \
  -H "Content-Type: application/json" \
  -d '{"prompt_name": "novel_repair"}'
```

#### 查看提示词统计
```bash
curl http://localhost:8080/api/prompts/stats
```

## 系统初始化

### 1. 克隆项目

```bash
git clone https://github.com/handy-h/voidtext.git
cd voidtext
```

### 2. 配置环境变量

```bash
cp .env.template .env
```

编辑 `.env` 文件，主要配置项说明：

```env
# 基础配置
PORT=8080
DATA_DIR=./data

# LLM API 配置（至少配置一项）
LLM_API_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
LLM_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COMPLETION_MODEL_NAME=qwen-max

# 本地模型配置（可选，启用后优先使用本地模型）
ENABLE_LOCAL_MODEL=false
LOCAL_MODEL_URL=http://localhost:11434
LOCAL_MODEL_NAME=qwen2.5:7b-instruct-q4_K_M
```

### 3. 编译项目

```bash
# 使用 Makefile 编译（推荐）
make build

# 或直接使用 go 命令
go build -o voidtext ./cmd/voidtext/
```

### 4. 运行服务

```bash
# 开发者模式（控制台打印日志，便于调试）
make dev

# 生产环境运行
make run

# 或直接运行编译好的二进制
./voidtext
```

### 5. 访问应用

打开浏览器，访问 `http://localhost:8080`

### 6. 项目管理

项目提供 `Makefile` 管理常用操作，在项目根目录下直接执行：

| 命令 | 功能 | 说明 |
|------|------|------|
| `make build` | 编译二进制 | 自动检查依赖并编译，输出 `./voidtext` |
| `make dev` | 开发者模式 | `go run` 热启动，控制台打印结构化日志，Gin debug 模式 |
| `make run` | 生产运行 | 运行已编译的二进制文件 `./voidtext` |
| `make clean` | 清理 | 结束进程 + 删除编译产物 + 清理运行时数据 |
| `make help` | 显示帮助 | 列出所有可用命令 |

> **开发调试**建议使用 `make dev`，日志同时输出到文件和控制台，方便实时排查问题。
>
> **生产部署**建议先 `make build` 编译，再 `make run` 启动。

## 使用指南

### 上传文件

- 点击「上传文件」按钮，选择要处理的 txt 文件
- 文件名格式建议为 `作者 - 标题.txt`，系统自动解析作者和标题
- 支持的分隔符：`-`、`—`、`·`、`_`、`~`（可通过 NAME_SEPARATORS 自定义）

### 配置与处理

- 上传后可配置该文件的处理规则
- 点击「保存并开始处理」执行五步流水线
- 处理过程中可查看进度和当前步骤
- 可随时下载当前版本的中间文件
- 支持断点续传，关闭浏览器后可继续

### 审核

- 到达人工审核步骤时，逐条审核修改建议
- 支持通过/拒绝/编辑/恢复/批量操作
- 可随时中断审核，关闭浏览器后可继续

### 完成

- 所有审核项处理完毕后，点击「生成最终文件」
- 下载清洗后的最终文件
- 可查看处理报告

## 完整配置选项

详细配置说明请参阅 [.env.template](.env.template)，主要配置分组：

| 配置组     | 说明                                           |
| ---------- | ---------------------------------------------- |
| 基础配置   | 服务端口、数据目录、最大文件上传大小、备份策略 |
| 文件名解析 | 作者/标题分隔符自定义                          |
| 基础清洗   | 启用/禁用、繁体转简体                          |
| 向量检测   | 模型选择、相似度阈值、API配置                  |
| LLM修复    | 启用/禁用、自进化监控、API配置、生成参数       |
| 本地模型   | 混合架构：Ollama配置、置信度阈值、降级策略     |

## 常见问题

### 1. 文件上传失败

- 检查文件大小是否超过限制（默认 100MB，可通过 MAX_FILE_SIZE 调整）
- 检查文件类型是否为 txt 文件
- 检查网络连接是否正常

### 2. 处理速度慢

- LLM 修复阶段耗时较长，请耐心等待
- 可关闭浏览器，处理会在后台继续
- 可启用本地 Ollama 模型 (`ENABLE_LOCAL_MODEL=true`) 加速
- 可设置 `ENABLE_MODEL_REPAIR=false` 跳过 LLM 修复
- 可设置 `ENABLE_VECTOR_DETECTION=false` 跳过向量检测

### 3. 外部 API 调用失败

- 检查 API URL 和 API Key 是否正确
- 检查网络连接是否正常
- 模型修复阶段会自动降级：本地模型失败 → 远程API → 本地字典

### 4. 如何使用本地模型

```env
ENABLE_LOCAL_MODEL=true
LOCAL_MODEL_URL=http://localhost:11434
LOCAL_MODEL_NAME=qwen2.5:7b-instruct-q4_K_M
```

需要先安装 [Ollama](https://ollama.ai/) 并拉取模型：

```bash
ollama pull qwen2.5:7b-instruct-q4_K_M
```

### 5. 验证本地模型调用

#### 5.1 确认 Ollama 服务运行正常

```bash
# 检查 Ollama 服务状态
curl -s http://localhost:11434/api/tags | python3 -m json.tool

# 应返回已下载的模型列表，例如：
# {
#     "models": [
#         {
#             "name": "qwen2.5:7b-instruct-q4_K_M",
#             "model": "qwen2.5:7b-instruct-q4_K_M",
#             ...
#         }
#     ]
# }
```

### 6. 提示词相关配置与使用

#### 6.1 启用自进化监控
```env
# 在 .env 文件中启用 Evolver
ENABLE_EVOLVER=true
EVOLVER_ERROR_RATE_THRESHOLD=0.2    # 错误率 >20% 时触发优化
EVOLVER_HIT_RATE_THRESHOLD=0.3      # 命中率 <30% 时触发优化
EVOLVER_CHECK_INTERVAL=300          # 每5分钟检查一次
```

#### 6.2 使用自定义提示词文件
1. 创建提示词目录（如果不存在）：
   ```bash
   mkdir -p config/prompts
   ```

2. 创建提示词文件（命名格式：`{prompt_name}_{version}.txt`）：
   ```bash
   # 创建中文提示词
   cat > config/prompts/novel_repair_v1.0.1.txt << 'EOF'
   你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。

   重要要求：
   1. 只修正明显的错别字和语法错误，不要改变原文意思
   2. 保持口语化表达，不要过度正式化
   3. 如果原文没有错误，直接返回原文
   4. 输出格式：只输出修正后的文本，不要添加任何额外说明

   示例：
   输入：她高兴及了，跑过去抱住他。
   输出：她高兴极了，跑过去抱住他。

   当前任务：请修正以下文本：
   EOF
   ```

3. 重启服务或等待热重载（每5秒检查一次文件变化）

#### 6.3 查看当前使用的提示词
```bash
# 查看日志中的提示词信息
grep "提示词管理器" logs/app.log

# 或通过数据库查询
sqlite3 data/cleaning.db "SELECT prompt_version, source, LENGTH(prompt_content) as length FROM prompt_versions WHERE prompt_name='novel_repair' ORDER BY created_at DESC LIMIT 1;"
```

#### 6.4 手动触发提示词优化
```bash
# 通过 Evolver 脚本手动优化（需要安装 Node.js 和 Evolver CLI）
python3 scripts/evolver.py \
  --prompt "当前提示词内容" \
  --context '{"error_type":"high_error_rate","error_count":10,"total_requests":50}'
```

#### 5.2 测试本地模型响应

```bash
# 发送测试请求
curl -s http://localhost:11434/api/generate \
  -d '{"model":"qwen2.5:7b-instruct-q4_K_M","prompt":"你好","stream":false}' \
  | python3 -m json.tool

# 成功响应示例：
# {
#     "model": "qwen2.5:7b-instruct-q4_K_M",
#     "response": "你好！有什么可以帮助你的吗？",
#     "done": true,
#     "total_duration": 1234567890,
#     ...
# }
```

#### 5.3 查看服务启动日志

启动服务后，在日志中搜索以下关键字确认本地模型已启用：

```
[ModelRepairer] 本地模型已启用 (URL: http://localhost:11434, Model: qwen2.5:7b-instruct-q4_K_M, Timeout: 60s)
```

如果看到以下日志，说明本地模型未启用，将使用远程 API：

```
[ModelRepairer] 本地模型未启用，将使用远程API
```

#### 5.4 查看处理过程中的调用日志

在处理文件时，日志中会出现以下关键字：

| 日志关键字                       | 含义                               |
| -------------------------------- | ---------------------------------- |
| `local_model_processing_start`   | 本地模型开始处理                   |
| `local_model_processing_success` | 本地模型处理成功（含耗时、置信度） |
| `local_model_processing_failed`  | 本地模型处理失败，将触发远程降级   |
| `ollama_generate_success`        | Ollama API 调用成功                |
| `ollama_request_failed`          | Ollama API 调用失败                |

#### 5.5 查看处理统计信息

处理完成后，通过 API 查看详细的统计信息：

```bash
# 获取文件处理状态
curl -s http://localhost:8080/api/files/<文件MD5>/status | python3 -m json.tool
```

响应中的 `stats` 字段包含以下统计：

| 字段              | 说明                            |
| ----------------- | ------------------------------- |
| `local_success`   | 本地模型成功处理的块数          |
| `local_failure`   | 本地模型失败且未降级的块数      |
| `remote_fallback` | 本地失败后降级到远程 API 的块数 |
| `cache_hits`      | 缓存命中次数（无需调用模型）    |
| `cache_misses`    | 缓存未命中次数（需要调用模型）  |
| `total_chunks`    | 总块数                          |

#### 5.6 常见问题排查

**问题：日志中没有本地模型相关信息**

原因：文件处理任务是在修改配置前启动的，旧任务仍使用旧配置。

解决：重新触发文件处理：

```bash
curl -X POST http://localhost:8080/api/files/<文件MD5>/run
```

**问题：本地模型调用失败**

排查步骤：

1. 确认 Ollama 服务正在运行：`systemctl status ollama` 或 `docker ps | grep ollama`
2. 确认模型已下载：`ollama list`
3. 检查模型名称是否与 `.env` 中配置一致
4. 查看 Ollama 日志：`journalctl -u ollama -f` 或 `docker logs ollama`

**问题：频繁触发远程降级**

可能原因：

- 本地模型置信度低于阈值（默认 0.7）
- 本地模型响应超时（默认 60 秒）

调整建议：

- 降低置信度阈值：`LOCAL_CONFIDENCE_THRESHOLD=0.5`
- 增加超时时间：`LOCAL_MODEL_TIMEOUT=120`
- 关闭降级（仅使用本地模型）：`LOCAL_FALLBACK_ENABLED=false`

## 项目文档

更多技术细节请参阅 [repowikis/00-目录.md](repowikis/00-目录.md)

## 许可证

MIT License

## 联系方式

- 作者：handy
- 邮箱：mikelon@aliyun.com
- 项目地址：https://github.com/handy-h/voidtext.git
