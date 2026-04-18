# 小说文本清洗工具

使用Trae AI生成的一个基于Go语言的小说文本清洗工具，用于解决小说txt文档中的错别字、乱码、广告内容、重复或缺失片段等问题。

## 功能特性

- **文本预处理**：编码规范化、广告内容识别与移除、特殊字符处理、空白字符规范化
- **NLP分析**：错别字检测与纠正、乱码识别与修复、重复内容检测
- **人工审核**：修改建议生成、交互式审核界面、单次/批量修改选项
- **版本管理**：多版本备份、版本恢复、备份清理
- **自定义规则**：支持用户自定义正则表达式清理规则
- **外部API集成**：支持调用外部模型API进行更高级的文本处理
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
2. 安装依赖
   ```bash
   go mod tidy
   ```
3. 构建项目
   ```bash
   go build -o txtclean ./cmd/txtclean
   ```
4. 运行应用
   ```bash
   ./txtclean
   ```
5. 访问应用
   打开浏览器，访问 `http://localhost:8080`

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
- 点击「保存配置」应用配置

## 配置说明

### 环境变量

可以通过 `.env` 文件或环境变量配置以下参数：

- `PORT`：服务端口，默认8080
- `DATA_DIR`：数据目录，默认./data
- `MODELS_DIR`：模型目录，默认./models
- `MAX_FILE_SIZE`：最大文件大小，默认104857600（100MB）
- `BACKUP_KEEP_DAYS`：备份保留天数，默认7
- `EXTERNAL_API_URL`：外部API URL
- `EXTERNAL_API_KEY`：外部API Key

### 配置文件

也可以通过 `configs/config.yaml` 文件进行配置：

```yaml
port: 8080
data_dir: ./data
models_dir: ./models
max_file_size: 104857600
backup_keep_days: 7
external_api_url: ""
external_api_key: ""
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
│   │   └── rules/        # 规则管理
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
├── Dockerfile            # Dockerfile
├── docker-compose.yml    # Docker Compose配置
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

### 3. 外部API调用失败

- 检查API URL和API Key是否正确
- 检查网络连接是否正常
- 外部API调用失败不会影响本地处理功能

### 4. 版本恢复失败

- 检查版本是否存在
- 检查文件权限是否正确

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 联系方式

- 作者：Your Name
- 邮箱：<your.email@example.com>
- 项目地址：<https://github.com/yourusername/txt-cleaning>

