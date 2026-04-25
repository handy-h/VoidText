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

## 环境依赖

- **Go**: 1.25+
- **SQLite**: 无需单独安装（使用 modernc.org/sqlite 驱动）
- **外部服务**（可选）:
  - 远程 LLM API（阿里云 DashScope、DeepSeek、OpenAI 等）
  - 本地 Ollama 模型（用于混合架构，低成本优先）
  - 向量模型（用于语义相似度检测，默认本地计算）

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
# 编译 Linux/macOS
go build -o voidtext ./cmd/txtclean/

# 编译 ARM64（树莓派）
GOOS=linux GOARCH=arm64 go build -o voidtext ./cmd/txtclean/
```

### 4. 运行服务

```bash
# 直接运行
./voidtext

# 或使用启动脚本（支持后台运行）
chmod +x scripts/raspberrypi-start.sh
./scripts/raspberrypi-start.sh
```

### 5. 访问应用

打开浏览器，访问 `http://localhost:8080`

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

| 配置组 | 说明 |
|--------|------|
| 基础配置 | 服务端口、数据目录、最大文件上传大小、备份策略 |
| 文件名解析 | 作者/标题分隔符自定义 |
| 基础清洗 | 启用/禁用、繁体转简体 |
| 向量检测 | 模型选择、相似度阈值、API配置 |
| LLM修复 | 启用/禁用、自进化监控、API配置、生成参数 |
| 本地模型 | 混合架构：Ollama配置、置信度阈值、降级策略 |

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

| 日志关键字 | 含义 |
|-----------|------|
| `local_model_processing_start` | 本地模型开始处理 |
| `local_model_processing_success` | 本地模型处理成功（含耗时、置信度） |
| `local_model_processing_failed` | 本地模型处理失败，将触发远程降级 |
| `ollama_generate_success` | Ollama API 调用成功 |
| `ollama_request_failed` | Ollama API 调用失败 |

#### 5.5 查看处理统计信息

处理完成后，通过 API 查看详细的统计信息：

```bash
# 获取文件处理状态
curl -s http://localhost:8080/api/files/<文件MD5>/status | python3 -m json.tool
```

响应中的 `stats` 字段包含以下统计：

| 字段 | 说明 |
|------|------|
| `local_success` | 本地模型成功处理的块数 |
| `local_failure` | 本地模型失败且未降级的块数 |
| `remote_fallback` | 本地失败后降级到远程 API 的块数 |
| `cache_hits` | 缓存命中次数（无需调用模型） |
| `cache_misses` | 缓存未命中次数（需要调用模型） |
| `total_chunks` | 总块数 |

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