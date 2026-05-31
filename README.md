# 湮文 VoidText

> 归于寂灭，方见真章

基于 Go 的中文小说 TXT 清洗工具，解决错别字、乱码、广告、重复段落等问题。

![项目预览](./湮文VoidText.png)

---

## 快速开始

### 1. 获取项目

```bash
git clone https://github.com/handy-h/voidtext.git
cd voidtext
```

### 2. 配置环境

```bash
cp .env.template .env
# 编辑 .env，至少填写 LLM API 地址与密钥
```

### 3. 启动服务

**Windows（PowerShell）**

```powershell
.\voidtext.ps1 dev    # 开发者模式，控制台实时日志
```

**Linux / macOS**

```bash
make dev    # 开发者模式，控制台实时日志
```

打开浏览器访问 `http://localhost:8080`。

> **开发者模式** (`dev`)：控制台实时日志 + Gin debug 路由，便于排查问题。  
> **生产模式** (`run`)：先执行 `make build` 编译，再 `make run` 启动。

---

## 功能概览

| 模块 | 说明 |
|------|------|
| **五步流水线** | 基础清洗 → 向量去重 → LLM 修复 → 人工审核 → 生成文件 |
| **编码处理** | 自动检测 GBK / UTF-8，转换并规范化文本 |
| **向量去重** | 基于向量相似度识别重复段落，支持滑动窗口比对 |
| **LLM 修复** | 多模型自动切换（最多 3 个服务商），本地 Ollama 与远程 API 混合 |
| **人工审核** | 逐条查看修改建议，支持通过 / 拒绝 / 编辑 / 批量操作 |
| **版本管理** | MD5 内容标识，自动维护原始→中间→最终版本的父子链 |
| **规则引擎** | 每文件独立配置：错别字映射、广告黑名单、正则规则 |
| **提示词管理** | 数据库 / 文件热重载 / 硬编码 三级回退，支持自进化优化 |
| **断点续传** | 关闭浏览器后处理不中断，下次登录继续 |

---

## 项目命令

| 命令 | 作用 | 适用场景 |
|------|------|----------|
| `dev` | 开发者模式运行 | 开发调试，控制台实时日志 |
| `build` | 编译二进制 | 生产部署前 |
| `run` | 运行已编译文件 | 生产环境 |
| `stop` | 停止进程 | — |
| `clean` | 删除编译产物 | 保留数据，仅清二进制 |
| `rebuild` | 停止 + 清理 + 重新编译 | 代码更新后 |
| `run-background` | 后台静默运行 | 服务器长期运行 |
| `help` | 查看帮助 | — |

**Windows**：`.\voidtext.ps1 <命令>`  
**Linux / macOS**：`make <命令>`

---

## 使用指南

### 上传文件

点击「上传文件」，选择 TXT 文件。建议文件名格式为 `作者 - 标题.txt`，系统自动解析。

### 配置与处理

上传后可配置该文件的清洗规则（错别字映射、广告黑名单等），点击「保存并开始处理」启动五步流水线。处理过程中可查看进度，支持断点续传。

### 人工审核

到达审核步骤时，逐条查看 LLM 的修改建议，选择通过、拒绝或手动编辑。支持批量操作。

### 下载结果

审核完成后点击「生成最终文件」，下载清洗后的 TXT。可随时查看处理报告与版本历史。

---

## 配置说明

主要配置项请查阅 [`.env.template`](.env.template)，核心分组如下：

| 配置组 | 说明 |
|--------|------|
| 基础配置 | 端口、数据目录、上传大小限制、备份保留天数 |
| 基础清洗 | 启用开关、繁简转换 |
| 向量检测 | 模型类型（local / api / ollama）、相似度阈值 |
| LLM 修复 | 主服务商 + 2 个备用、温度、最大 token、模型名称 |
| 本地 Ollama | 地址、超时、模型名、段落重组开关 |

### 多模型自动切换

系统支持最多 3 个服务商，降级链路如下：

```
本地 Ollama（如启用）
    ↓ 失败
服务商 1（主模型 → 同域备模型，最多 5 个）
    ↓ 全部失败
服务商 2（备用）
    ↓ 失败
服务商 3（备用）
    ↓ 失败
内置错别字字典兜底
```

切换条件：连接超时、5xx、429 限流、额度耗尽等不可恢复错误。

### 提示词管理（进阶）

提示词加载优先级：

1. **数据库** (`prompt_versions` 表) — Evolver 自动优化后保存
2. **文件系统** (`config/prompts/{name}_{version}.txt`) — 修改后 5 秒热重载
3. **硬编码默认值** — 兜底

开发环境推荐文件方式：

```bash
mkdir -p config/prompts
cat > config/prompts/novel_repair_v1.0.0.txt << 'EOF'
你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。
EOF
```

生产环境使用数据库方式，系统自动管理，无需手动干预。

---

## 常见问题

**Q: 文件上传失败？**

- 检查文件大小是否超过 `MAX_FILE_SIZE`（默认 100MB）
- 确认文件为 `.txt` 格式

**Q: 处理速度慢？**

- LLM 修复阶段本身较慢，可关闭浏览器后台继续运行
- 启用本地 Ollama (`ENABLE_LOCAL_MODEL=true`) 减少网络延迟
- 跳过非必要阶段：`ENABLE_MODEL_REPAIR=false` 或 `ENABLE_VECTOR_DETECTION=false`

**Q: API 调用失败？**

- 检查 `.env` 中的 `LLM_API_URL` 与 `LLM_API_KEY`
- 确认网络可达，查看日志中的具体错误码
- 配置备用服务商 (`LLM_API_URL_2` / `LLM_API_URL_3`) 实现自动切换

**Q: 如何使用本地 Ollama？**

```env
ENABLE_LOCAL_MODEL=true
LOCAL_MODEL_URL=http://localhost:11434
LOCAL_MODEL_NAME=qwen2.5:7b-instruct-q4_K_M
```

先安装 [Ollama](https://ollama.ai/) 并拉取模型：

```bash
ollama pull qwen2.5:7b-instruct-q4_K_M
```

验证服务：

```bash
curl http://localhost:11434/api/tags
```

**Q: 向量模型与对话模型有什么区别？**

| 用途 | 配置项 | 示例 |
|------|--------|------|
| 向量嵌入（去重用） | `VECTOR_MODEL_NAME` | `nomic-embed-text:latest` |
| 文本生成（修复用） | `COMPLETION_MODEL_NAME` | `qwen-plus-2025-01-25` |

切勿将嵌入模型填入 `COMPLETION_MODEL_NAME`，否则 chat API 会返回 404。

---

## 项目文档

更多技术细节请参阅 [`docs/`](docs/) 目录。

## 许可证

MIT License

## 联系

- 作者：handy
- 邮箱：mikelon@aliyun.com
- 项目地址：https://github.com/handy-h/voidtext
