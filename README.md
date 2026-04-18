# 小说文本清洗工具

使用Trae AI生成的一个基于Go语言的小说文本清洗工具，用于解决小说txt文档中的错别字、乱码、广告内容、重复或缺失片段等问题。

## 功能特性

- **三阶段处理流水线**：基础清洗 → 向量检测去重 → 模型修复，每阶段可独立启用/禁用
- **基础文本清洗**：编码规范化、广告内容识别与移除、特殊字符处理、空白字符规范化、繁体转简体
- **向量检测去重**：基于向量相似度的重复段落检测与移除
- **模型修复**：调用外部LLM纠正错别字和语法错误，支持本地字典兜底
- **人工审核**：修改建议生成、交互式审核界面、单次/批量修改选项
- **版本管理**：多版本备份、版本恢复、备份清理
- **自定义规则**：支持用户自定义正则表达式清理规则
- **外部API集成**：支持OpenAI兼容API（阿里云DashScope、DeepSeek等）
- **Web界面**：直观的Web操作界面，支持文件上传、处理状态查看、审核操作等

## 技术栈

- **后端**：Go语言、Gin框架
- **前端**：HTML、CSS、JavaScript
- **本地NLP**：prose/v2、levenshtein
- **容器化**：Docker

## 安装与部署

### 方法一：使用Docker（推荐）

1. 克隆项目
   ```bash
   git clone https://github.com:handy-h/txtCleaning.git
   cd txt-cleaning
   ```
2. 构建Docker镜像
   ```bash
   docker build -t txtcleaning .
   ```
3. 运行容器
   ```bash
   docker run -d -p 8080:8080 -v ./data:/app/data --name txtcleaning txtcleaning
   ```
4. 访问应用
   打开浏览器，访问 `http://localhost:8080`

### 方法二：本地运行

1. 克隆项目
   ```bash
   git clone https://github.com:handy-h/txtCleaning.git
   cd txt-cleaning
   ```
2. 配置环境变量
   ```bash
   cp .env.template .env
   # 编辑 .env 文件，填入API配置等
   ```
3. 安装依赖并运行
   ```bash
   chmod +x scripts/run.sh
   ./scripts/run.sh
   ```
4. 访问应用
   打开浏览器，访问 `http://localhost:8080`

## 三阶段处理流程

### 第一阶段：基础文本清洗（规则先行）

在调用AI模型之前，使用轻量级规则处理明显问题，降低后续AI模型的工作量和成本。

- **HTML实体清理**：移除 `&nbsp;`、`&amp;` 等网页转义字符
- **标点规范化**：统一全角/半角标点，修正混用问题
- **空白字符规范化**：移除多余空格、制表符、连续空行
- **繁体转简体**：可选的繁简转换（需启用）
- **广告移除**：基于正则表达式匹配常见推广语句

### 第二阶段：向量检测去重

通过向量模型将文本转化为向量，在语义层面进行"查重"和"定位"。

- **段落分割**：按换行符将文本分割为段落
- **向量生成**：支持本地简化向量计算或外部API嵌入
- **相似度计算**：基于余弦相似度检测语义重复
- **重复移除**：自动移除相似度超过阈值的重复段落

### 第三阶段：模型修复

调用文本生成模型（LLM）纠正错别字和语法错误。

- **API修复**：调用外部LLM进行智能校对
- **本地兜底**：API不可用时使用本地错别字字典
- **Prompt工程**：内置专业的中文小说校对提示词
- **变更追踪**：记录所有修改，支持人工审核

## 使用指南

### 1. 上传文件

- 在首页点击「选择文件」按钮，选择要处理的txt文件
- 点击「上传并处理」按钮开始处理

### 2. 查看处理状态

- 上传后会显示处理进度和状态
- 处理完成后会自动跳转到审核页面

### 3. 审核修改建议

- 查看系统生成的修改建议
- 对每个建议点击「通过」或「拒绝」
- 可以点击「全部通过」或「全部拒绝」进行批量操作
- 点击「保存进度」保存当前审核进度

### 4. 版本管理

- 在版本管理页面查看所有版本
- 点击「恢复」按钮恢复到特定版本
- 点击「删除」按钮删除不需要的版本

### 5. 自定义规则

- 在自定义规则页面添加新的正则表达式规则
- 输入规则名称、正则表达式和替换内容
- 点击「添加规则」保存规则

### 6. 外部API配置

- 在外部API配置页面输入API URL和API Key
- 点击"保存配置"应用配置

## 配置说明

### 环境变量

通过 `.env` 文件配置，参考 `.env.template` 获取完整模板。

#### 基础配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PORT` | 服务端口 | 8080 |
| `DATA_DIR` | 数据目录 | ./data |
| `MODELS_DIR` | 模型目录 | ./models |
| `MAX_FILE_SIZE` | 最大文件大小(字节) | 104857600 |
| `BACKUP_KEEP_DAYS` | 备份保留天数 | 7 |

#### 第一阶段：基础文本清洗

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ENABLE_BASIC_CLEANING` | 是否启用基础清洗 | true |
| `BASIC_CLEANING_TOOL` | 清洗工具 | regex |
| `TRADITIONAL_TO_SIMPLE` | 繁体转简体 | false |

#### 第二阶段：向量检测

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ENABLE_VECTOR_DETECTION` | 是否启用向量检测 | true |
| `VECTOR_MODEL_NAME` | 向量模型名称 | all-MiniLM-L6-v2 |
| `VECTOR_MODEL_TYPE` | 向量模型类型(local/api) | local |
| `VECTOR_SIMILARITY_THRESHOLD` | 相似度阈值(0.0~1.0) | 0.95 |

#### 第三阶段：模型修复

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ENABLE_MODEL_REPAIR` | 是否启用模型修复 | true |
| `REPAIR_MODEL_NAME` | 修复模型名称 | gpt-3.5-turbo-instruct |
| `REPAIR_MODEL_TYPE` | 修复模型类型(local/api) | api |

#### 外部API配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `EXTERNAL_API_URL` | API基础URL | (空) |
| `EXTERNAL_API_KEY` | API密钥 | (空) |
| `EMBEDDING_MODEL_NAME` | 嵌入模型名称 | text-embedding-ada-002 |
| `COMPLETION_MODEL_NAME` | 文本生成模型名称 | gpt-3.5-turbo-instruct |

#### 文本生成模型调用参数

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `COMPLETION_TEMPERATURE` | 生成温度(0.0~2.0) | 0.3 |
| `COMPLETION_MAX_TOKENS` | 最大生成token数 | 2048 |

### 配置示例

#### 阿里云DashScope + qwen模型

```env
EXTERNAL_API_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
EXTERNAL_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
EMBEDDING_MODEL_NAME=text-embedding-v3
COMPLETION_MODEL_NAME=qwen-max-2025-01-25
COMPLETION_TEMPERATURE=0.3
COMPLETION_MAX_TOKENS=2048
```

#### DeepSeek模型

```env
EXTERNAL_API_URL=https://api.deepseek.com/v1
EXTERNAL_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COMPLETION_MODEL_NAME=deepseek-chat
COMPLETION_TEMPERATURE=0.3
COMPLETION_MAX_TOKENS=2048
```

### 配置文件

也可以通过 `configs/config.yaml` 文件进行配置：

```yaml
port: 8080
data_dir: ./data
models_dir: ./models
max_file_size: 104857600
backup_keep_days: 7

enable_basic_cleaning: true
basic_cleaning_tool: "regex"
traditional_to_simple: false

enable_vector_detection: true
vector_model_name: "all-MiniLM-L6-v2"
vector_model_type: "local"
vector_similarity_threshold: 0.95

enable_model_repair: true
repair_model_name: "qwen-max-2025-01-25"
repair_model_type: "api"

external_api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
external_api_key: "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
embedding_model_name: "text-embedding-v3"
completion_model_name: "qwen-max-2025-01-25"
completion_temperature: 0.3
completion_max_tokens: 2048
```

## 项目结构

```
txt-cleaning/
├── cmd/                  # 命令行工具
│   └── txtclean/         # 主入口
├── internal/             # 内部包
│   ├── config/           # 配置管理
│   ├── processor/        # 文本处理器
│   │   ├── preprocess/   # 预处理
│   │   ├── model/        # NLP模型
│   │   ├── postprocess/  # 后处理
│   │   ├── rules/        # 规则管理
│   │   ├── basic_cleaner.go    # 第一阶段：基础清洗
│   │   ├── vector_detector.go  # 第二阶段：向量检测
│   │   ├── model_repairer.go   # 第三阶段：模型修复
│   │   └── processor.go        # 处理流水线
│   ├── file/             # 文件操作
│   ├── review/           # 审核管理
│   ├── external/         # 外部API调用
│   └── utils/            # 工具函数
├── pkg/                  # 公共包
│   ├── nlp/              # NLP工具
│   ├── detector/         # 检测工具
│   └── corrector/        # 纠正工具
├── web/                  # Web界面
│   ├── frontend/         # 前端代码
│   └── backend/          # 后端API
├── configs/              # 配置文件
├── data/                 # 数据目录
├── models/               # 模型目录
├── scripts/              # 脚本
├── tests/                # 测试代码
├── .env.template         # 环境变量模板
├── go.mod                # Go模块文件
└── README.md             # 项目说明
```

## 常见问题

### 1. 文件上传失败

- 检查文件大小是否超过限制（默认100MB）
- 检查文件类型是否为txt文件

### 2. 处理速度慢

- 大文件处理可能需要较长时间，请耐心等待
- 可以在处理过程中关闭浏览器，稍后再访问查看结果
- 第三阶段模型修复调用API较慢，可设置 `ENABLE_MODEL_REPAIR=false` 跳过

### 3. 外部API调用失败

- 检查API URL和API Key是否正确
- 检查网络连接是否正常
- 外部API调用失败不会影响本地处理功能
- 模型修复阶段会自动降级为本地字典修复

### 4. 版本恢复失败

- 检查版本是否存在
- 检查文件权限是否正确

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 联系方式

- 作者：handy
- 邮箱：mikelon@aliyun.com
- 项目地址：https://github.com/handy-h/txtCleaning.git