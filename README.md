# 小说文本清洗工具

基于Go语言的小说文本清洗工具，用于解决小说txt文档中的错别字、乱码、广告内容、重复或缺失片段等问题。

## 功能特性

- **五步处理流水线**：基础清洗 → 向量检测 → LLM修复 → 人工审核 → 生成文件，每步可独立执行
- **文件生命周期管理**：MD5唯一标识、状态追踪（pending/processing/completed/failed）、进度恢复
- **版本链管理**：自动维护原始文件与中间版本的父子关系，任何版本可追溯到原始文件
- **中间版本识别**：上传之前下载的中间版本文件，自动识别并继续处理
- **基础文本清洗**：编码检测与转换（GBK/UTF-8）、广告移除、特殊字符处理、繁体转简体
- **向量检测去重**：基于向量相似度的重复段落检测与移除
- **LLM修复**：调用外部LLM纠正错别字和语法错误，支持本地字典兜底
- **人工审核**：逐条审核修改建议，支持通过/拒绝/编辑/恢复/批量操作
- **自定义规则**：每个文件可独立配置规则（错别字映射、广告黑名单等）
- **处理报告**：生成包含审核统计、版本历史、处理日志的完整报告
- **SQLite持久化**：所有数据本地存储，无需外部数据库依赖

## 技术栈

- **后端**：Go语言、Gin框架、SQLite（modernc.org/sqlite）
- **前端**：HTML、CSS、JavaScript（原生）
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

## 处理流程

### 文件生命周期

一个文件从上传到完成经历以下状态：

```
pending → processing → reviewing → completed
                └──→ failed（可重试）
```

- **pending**：已上传，等待开始处理
- **processing**：处理中（包含多个子步骤）
- **reviewing**：等待人工审核
- **completed**：所有审核项已完成，生成了最终文件
- **failed**：处理失败（可从失败步骤重试）

### 五步处理流水线

#### 第一步：基础文本清洗

使用轻量级规则处理明显问题，降低后续AI模型的工作量和成本。

- **编码检测与转换**：自动检测GBK编码并转换为UTF-8
- **HTML实体清理**：移除 `&nbsp;`、`&amp;` 等网页转义字符
- **标点规范化**：统一全角/半角标点，修正混用问题
- **空白字符规范化**：移除多余空格、制表符、连续空行
- **繁体转简体**：可选的繁简转换
- **广告移除**：基于正则表达式匹配常见推广语句

#### 第二步：向量索引与检测

通过向量模型将文本转化为向量，在语义层面进行"查重"和"定位"。

- **段落分割**：按换行符将文本分割为段落
- **向量生成**：支持本地简化向量计算或外部API嵌入
- **相似度计算**：基于余弦相似度检测语义重复
- **重复移除**：自动移除相似度超过阈值的重复段落

#### 第三步：LLM修复

调用文本生成模型（LLM）纠正错别字和语法错误。

- **API修复**：调用外部LLM进行智能校对
- **本地兜底**：API不可用时使用本地错别字字典
- **Prompt工程**：内置专业的中文小说校对提示词
- **变更追踪**：记录所有修改，支持人工审核

#### 第四步：人工审核

生成待审核列表，用户逐条确认或修改。这是核心交互点，用户可能分多次完成审核。

- **逐条审核**：通过/拒绝/编辑每条修改建议
- **批量操作**：全部通过/全部拒绝
- **恢复操作**：已审核项可恢复为待审核
- **进度保存**：审核进度自动持久化，关闭浏览器后可继续

#### 第五步：生成最终文件

所有审核通过后，输出清洗后的TXT文件。

- **文件命名**：`{作者}_{标题}_cleaned_{时间戳}.txt`
- **版本记录**：最终文件MD5记录到版本链
- **处理报告**：生成包含统计信息的完整报告

### 版本链管理

工具自动维护版本链，确保任何版本都能追溯到原始文件：

```
原始文件(MD5-A) → 清洗后(MD5-B) → 索引后(MD5-C) → LLM修复后(MD5-D) → 最终文件(MD5-E)
```

- 每个处理步骤完成后自动创建中间版本
- 用户可随时下载当前版本的中间文件
- 上传中间版本文件时，自动识别并继续处理
- 版本链记录在SQLite数据库中，支持完整追溯

### 进度恢复

支持两种方式恢复处理进度：

1. **直接上传文件**：上传同一文件（MD5匹配）后，工具自动检测到未完成的处理记录，提示继续
2. **从列表选择**：在待处理列表中选择文件，点击"继续处理"

## 使用指南

### 1. 上传文件

- 点击「上传文件」按钮，选择要处理的txt文件
- 文件名格式建议为 `作者 - 标题.txt`，系统自动解析作者和标题
- 如果上传已存在的文件，系统会提示继续上次进度
- 如果上传的是之前下载的中间版本，系统会自动识别并跳到对应步骤

### 2. 配置规则

- 上传后可配置该文件的处理规则
- 可启用/禁用各处理阶段
- 可设置错别字映射和广告黑名单
- 规则配置随文件保存，下次继续处理时自动加载

### 3. 处理与审核

- 点击「保存并开始处理」执行五步流水线
- 处理过程中可查看进度和当前步骤
- 到达人工审核步骤时，逐条审核修改建议
- 可随时下载当前版本的中间文件

### 4. 完成与下载

- 所有审核项处理完毕后，点击「生成最终文件」
- 下载清洗后的最终文件
- 可查看处理报告（包含审核统计、版本历史、处理日志）

### 5. 删除文件

- 每个文件卡片都有「删除」按钮
- 删除操作需要二次确认
- 删除将移除该文件的所有审核记录、版本历史和处理日志

## 配置说明

### 环境变量

通过 `.env` 文件配置，参考 `.env.template` 获取完整模板。

#### 基础配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PORT` | 服务端口 | 8080 |
| `DATA_DIR` | 数据目录（含数据库） | ./data |
| `MODELS_DIR` | 模型目录 | ./models |
| `MAX_FILE_SIZE` | 最大文件大小(字节) | 104857600 |
| `BACKUP_KEEP_DAYS` | 备份保留天数 | 7 |
| `NAME_SEPARATORS` | 文件名分隔符列表 | -\|—\|·\|·\|_\| |

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
| `VECTOR_MODEL_URL` | 向量模型API地址 | (空) |
| `VECTOR_MODEL_API_KEY` | 向量模型API密钥 | (空) |

#### 第三阶段：LLM修复

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `ENABLE_MODEL_REPAIR` | 是否启用模型修复 | true |
| `REPAIR_MODEL_NAME` | 修复模型名称 | gpt-3.5-turbo-instruct |
| `REPAIR_MODEL_TYPE` | 修复模型类型(local/api) | api |
| `LLM_API_URL` | LLM API地址 | (空) |
| `LLM_API_KEY` | LLM API密钥 | (空) |
| `COMPLETION_MODEL_NAME` | 文本生成模型名称 | gpt-3.5-turbo-instruct |
| `COMPLETION_TEMPERATURE` | 生成温度(0.0~2.0) | 0.3 |
| `COMPLETION_MAX_TOKENS` | 最大生成token数 | 2048 |

#### 兼容旧配置

| 旧变量名 | 映射到 |
|-----------|--------|
| `EXTERNAL_API_URL` | `VECTOR_MODEL_URL` 和 `LLM_API_URL` |
| `EXTERNAL_API_KEY` | `VECTOR_MODEL_API_KEY` 和 `LLM_API_KEY` |
| `EMBEDDING_MODEL_NAME` | `COMPLETION_MODEL_NAME` |

### 配置示例

#### 阿里云DashScope + qwen模型

```env
VECTOR_MODEL_TYPE=api
VECTOR_MODEL_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
VECTOR_MODEL_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
LLM_API_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
LLM_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COMPLETION_MODEL_NAME=qwen-max-2025-01-25
COMPLETION_TEMPERATURE=0.3
COMPLETION_MAX_TOKENS=2048
```

#### DeepSeek模型

```env
LLM_API_URL=https://api.deepseek.com/v1
LLM_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COMPLETION_MODEL_NAME=deepseek-chat
COMPLETION_TEMPERATURE=0.3
COMPLETION_MAX_TOKENS=2048
```

## 项目结构

```
txt-cleaning/
├── cmd/                          # 命令行工具
│   └── txtclean/                 # 主入口
├── internal/                     # 内部包
│   ├── config/                   # 配置管理（.env加载）
│   ├── database/                 # SQLite数据库
│   │   ├── db.go                 # 数据库初始化与表结构
│   │   ├── file_repo.go          # 文件记录CRUD
│   │   ├── version_repo.go       # 版本链CRUD
│   │   ├── review_repo.go        # 审核项CRUD
│   │   └── log_repo.go           # 处理日志CRUD
│   ├── file/                     # 文件操作
│   │   ├── md5.go                # MD5计算
│   │   ├── parser.go             # 文件名解析（作者+标题）
│   │   └── version.go            # 版本管理
│   ├── processor/                # 文本处理器
│   │   ├── pipeline.go           # 五步处理流水线控制器
│   │   ├── basic_cleaner.go      # 第一步：基础清洗
│   │   ├── vector_detector.go    # 第二步：向量检测
│   │   ├── model_repairer.go     # 第三步：LLM修复
│   │   ├── processor.go          # 审核应用与最终生成
│   │   ├── preprocess/           # 预处理（编码检测等）
│   │   ├── postprocess/          # 后处理
│   │   ├── rules/                # 规则管理
│   │   └── model/                # NLP模型
│   ├── review/                   # 审核管理
│   │   └── manager/              # 审核管理器
│   └── external/                 # 外部API调用
├── web/                          # Web界面
│   ├── frontend/                 # 前端代码
│   │   ├── index.html            # 主页面
│   │   └── static/
│   │       ├── css/style.css     # 样式
│   │       └── js/main.js        # 交互逻辑
│   └── backend/                  # 后端API
│       ├── server.go             # 路由注册
│       └── handlers/             # 请求处理器
│           ├── files.go          # 文件上传/下载/删除
│           ├── process.go        # 处理流程/审核操作
│           ├── rules.go          # 规则管理
│           └── versions.go       # 版本管理
├── scripts/                      # 脚本
│   ├── run.sh                    # 运行脚本
│   ├── test.sh                   # 测试脚本
│   └── raspberrypi-start.sh      # 树莓派启动脚本
├── data/                         # 数据目录（运行时生成）
│   ├── cleaning.db               # SQLite数据库
│   ├── uploads/                  # 上传文件
│   ├── backups/                  # 备份文件
│   └── temp/                     # 临时文件
├── .env.template                 # 环境变量模板
├── .gitignore                    # Git忽略规则
├── go.mod                        # Go模块文件
└── README.md                     # 项目说明
```

## API接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/files/upload` | 上传文件（支持MD5识别） |
| GET | `/api/files` | 列出所有文件 |
| GET | `/api/files/pending` | 列出待处理文件 |
| GET | `/api/files/:md5` | 获取文件详情 |
| GET | `/api/files/:md5/content` | 获取文件内容 |
| GET | `/api/files/:md5/download` | 下载文件 |
| DELETE | `/api/files/:md5` | 删除文件及记录 |
| POST | `/api/files/:md5/resume` | 恢复文件处理 |
| PUT | `/api/files/:md5/rules` | 更新文件规则配置 |
| POST | `/api/files/:md5/run` | 执行全部处理步骤 |
| GET | `/api/files/:md5/status` | 获取文件处理状态 |
| GET | `/api/files/:md5/review-items` | 获取审核项列表 |
| POST | `/api/files/:md5/approve` | 通过审核项 |
| POST | `/api/files/:md5/reject` | 拒绝审核项 |
| POST | `/api/files/:md5/edit` | 编辑审核项 |
| POST | `/api/files/:md5/restore` | 恢复审核项为待审核 |
| POST | `/api/files/:md5/batch-approve` | 批量通过 |
| POST | `/api/files/:md5/batch-reject` | 批量拒绝 |
| POST | `/api/files/:md5/finalize` | 生成最终文件 |
| GET | `/api/files/:md5/report` | 获取处理报告 |

## 常见问题

### 1. 文件上传失败

- 检查文件大小是否超过限制（默认100MB）
- 检查文件类型是否为txt文件（不区分大小写）
- 检查文件名是否包含特殊字符

### 2. 上传后提示找不到文件

- 上传失败时页面会显示具体错误信息
- 不要在错误提示页面点击操作按钮
- 返回文件列表重新操作

### 3. 处理速度慢

- 大文件处理可能需要较长时间，请耐心等待
- 可以在处理过程中关闭浏览器，稍后再访问继续审核
- 第三阶段模型修复调用API较慢，可设置 `ENABLE_MODEL_REPAIR=false` 跳过
- 可在规则配置中禁用不需要的处理阶段

### 4. 外部API调用失败

- 检查API URL和API Key是否正确
- 检查网络连接是否正常
- 向量检测和LLM修复的API地址可分别配置
- 模型修复阶段会自动降级为本地字典修复

### 5. 中间版本文件无法识别

- 确保中间版本文件是从本工具下载的
- 中间版本通过MD5关联到原始文件
- 如果数据库被清除，中间版本将无法自动关联

### 6. 编码问题（乱码）

- 工具自动检测GBK编码并转换为UTF-8
- 如果仍有乱码，可能是混合编码，建议先用文本编辑器统一转为UTF-8

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 联系方式

- 作者：handy
- 邮箱：mikelon@aliyun.com
- 项目地址：https://github.com/handy-h/txtCleaning.git
