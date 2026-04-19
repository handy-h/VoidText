# 小说文本清洗工具

基于 Go 语言的小说文本清洗工具，用于解决小说 TXT 文档中的错别字、乱码、广告内容、重复或缺失片段等问题。

## 功能特性

- **五步处理流水线**：基础清洗 → 向量检测 → LLM修复 → 人工审核 → 生成文件
- **文件生命周期管理**：MD5 唯一标识、状态追踪（pending/processing/completed/failed）、进度恢复
- **版本链管理**：自动维护原始文件与中间版本的父子关系，任何版本可追溯
- **基础文本清洗**：编码检测与转换（GBK/UTF-8）、广告移除、特殊字符处理、繁体转简体
- **向量检测去重**：基于向量相似度的重复段落检测与移除
- **LLM修复**：调用外部 LLM 纠正错别字和语法错误，支持本地字典兜底
- **人工审核**：逐条审核修改建议，支持通过/拒绝/编辑/恢复/批量操作
- **自定义规则**：每个文件可独立配置规则（错别字映射、广告黑名单等）
- **处理报告**：生成包含审核统计、版本历史、处理日志的完整报告
- **SQLite持久化**：所有数据本地存储，无需外部数据库依赖

## 环境依赖

- **Go**: 1.21+
- **SQLite**: 无需单独安装（使用 modernc.org/sqlite 驱动）
- **外部服务**（可选）:
  - 阿里云 DashScope API 或 DeepSeek API（用于 LLM 修复）
  - 向量模型 API（用于语义相似度检测，可选本地计算）

## 系统初始化

### 1. 克隆项目

```bash
git clone https://github.com/handy-h/txtCleaning.git
cd txtCleaning
```

### 2. 配置环境变量

```bash
cp .env.template .env
```

编辑 `.env` 文件，配置以下必要项：

```env
# 服务端口
PORT=8080

# 数据目录
DATA_DIR=./data

# LLM API 配置（必填，否则无法使用 LLM 修复）
LLM_API_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
LLM_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COMPLETION_MODEL_NAME=qwen-max-2025-01-25
```

### 3. 编译项目

```bash
# 编译 Linux/macOS
go build -o txtclean ./cmd/txtclean/

# 编译 ARM64（树莓派）
GOOS=linux GOARCH=arm64 go build -o txtclean ./cmd/txtclean/
```

### 4. 运行服务

```bash
# 直接运行
./txtclean

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
- 支持的分隔符：`-`、`—`、`·`、`_`、`~`

### 配置与处理

- 上传后可配置该文件的处理规则
- 点击「保存并开始处理」执行五步流水线
- 处理过程中可查看进度和当前步骤
- 可随时下载当前版本的中间文件

### 审核

- 到达人工审核步骤时，逐条审核修改建议
- 支持通过/拒绝/编辑/批量操作
- 可随时中断审核，关闭浏览器后可继续

### 完成

- 所有审核项处理完毕后，点击「生成最终文件」
- 下载清洗后的最终文件
- 可查看处理报告

## 常见问题

### 1. 文件上传失败

- 检查文件大小是否超过限制（默认 100MB）
- 检查文件类型是否为 txt 文件
- 检查网络连接是否正常

### 2. 处理速度慢

- LLM 修复阶段耗时较长，请耐心等待
- 可关闭浏览器，处理会在后台继续
- 可设置 `ENABLE_MODEL_REPAIR=false` 跳过 LLM 修复

### 3. 外部 API 调用失败

- 检查 API URL 和 API Key 是否正确
- 检查网络连接是否正常
- 模型修复阶段会自动降级为本地字典修复

## 项目文档

更多技术细节请参阅 [doc/00-目录.md](doc/00-目录.md)

## 许可证

MIT License

## 联系方式

- 作者：handy
- 邮箱：mikelon@aliyun.com
- 项目地址：https://github.com/handy-h/txtCleaning.git