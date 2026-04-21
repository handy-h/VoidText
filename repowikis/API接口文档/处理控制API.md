# 处理控制API

<cite>
**本文档引用的文件**
- [server.go](file://web/backend/server.go)
- [process.go](file://web/backend/handlers/process.go)
- [files.go](file://web/backend/handlers/files.go)
- [pipeline.go](file://internal/processor/pipeline.go)
- [processor.go](file://internal/processor/processor.go)
- [model_repairer.go](file://internal/processor/model_repairer.go)
- [vector_detector.go](file://internal/processor/vector_detector.go)
- [file_repo.go](file://internal/database/file_repo.go)
- [review_repo.go](file://internal/database/review_repo.go)
- [manager.go](file://internal/review/manager/manager.go)
- [05-API接口.md](file://doc/05-API接口.md)
</cite>

## 目录
1. [简介](#简介)
2. [项目结构](#项目结构)
3. [核心组件](#核心组件)
4. [架构概览](#架构概览)
5. [详细组件分析](#详细组件分析)
6. [依赖关系分析](#依赖关系分析)
7. [性能考虑](#性能考虑)
8. [故障排除指南](#故障排除指南)
9. [结论](#结论)
10. [附录](#附录)

## 简介

处理控制API是文本清理系统的核心接口层，负责管理整个文本处理流水线的执行控制、状态查询和结果生成。该API提供了完整的异步处理机制，支持多步骤处理流水线、实时状态监控、审核管理以及处理报告生成功能。

系统采用分层架构设计，包括：
- **API层**：HTTP接口暴露处理控制功能
- **处理器层**：核心业务逻辑实现
- **数据访问层**：数据库操作和状态管理
- **外部服务集成**：模型修复和向量检测服务

## 项目结构

```mermaid
graph TB
subgraph "API层"
A[server.go<br/>路由注册]
B[process.go<br/>处理控制API]
C[files.go<br/>文件管理API]
end
subgraph "处理器层"
D[pipeline.go<br/>处理流水线]
E[processor.go<br/>文本处理]
F[model_repairer.go<br/>模型修复器]
G[vector_detector.go<br/>向量检测器]
end
subgraph "数据访问层"
H[file_repo.go<br/>文件仓库]
I[review_repo.go<br/>审核仓库]
J[manager.go<br/>审核管理器]
end
subgraph "外部服务"
K[external API<br/>模型服务]
L[数据库<br/>SQLite]
end
A --> B
A --> C
B --> D
C --> H
D --> E
D --> F
D --> G
E --> H
F --> H
G --> H
D --> I
I --> J
F --> K
G --> K
H --> L
I --> L
```

**图表来源**
- [server.go:10-57](file://web/backend/server.go#L10-L57)
- [process.go:1-511](file://web/backend/handlers/process.go#L1-L511)
- [files.go:1-393](file://web/backend/handlers/files.go#L1-L393)

**章节来源**
- [server.go:10-57](file://web/backend/server.go#L10-L57)
- [process.go:15-77](file://web/backend/handlers/process.go#L15-L77)
- [files.go:18-71](file://web/backend/handlers/files.go#L18-L71)

## 核心组件

### 处理流水线组件

处理流水线包含五个核心步骤，每个步骤都有特定的功能和状态管理：

```mermaid
flowchart TD
A["开始处理"] --> B["基础清洗<br/>cleaning"]
B --> C["向量检测<br/>indexing"]
C --> D["LLM修复<br/>llm_fix"]
D --> E["审核<br/>review"]
E --> F["最终化<br/>finalizing"]
F --> G["处理完成"]
H["处理状态"] --> I["processing<br/>处理中"]
H --> J["reviewing<br/>审核中"]
H --> K["failed<br/>失败"]
H --> L["completed<br/>完成"]
M["进度计算"] --> N["每步20%基础进度"]
N --> O["步骤内进度贡献"]
```

**图表来源**
- [pipeline.go:19-25](file://internal/processor/pipeline.go#L19-L25)
- [pipeline.go:96-102](file://internal/processor/pipeline.go#L96-L102)

### 审核管理系统

审核系统提供完整的变更管理功能，支持多种审核状态和批量操作：

```mermaid
stateDiagram-v2
[*] --> 待审核
待审核 --> 已批准 : approve
待审核 --> 已拒绝 : reject
待审核 --> 已编辑 : edit
待审核 --> 恢复 : restore
已批准 --> [*]
已拒绝 --> [*]
已编辑 --> [*]
恢复 --> 待审核
note right of 待审核
原始建议
状态 : pending
end note
note right of 已批准
用户确认
状态 : approved
end note
note right of 已拒绝
用户拒绝
状态 : rejected
end note
note right of 已编辑
手动修改
状态 : edited
end note
```

**图表来源**
- [manager.go:17-22](file://internal/review/manager/manager.go#L17-L22)
- [review_repo.go:106-126](file://internal/database/review_repo.go#L106-L126)

**章节来源**
- [pipeline.go:19-25](file://internal/processor/pipeline.go#L19-L25)
- [manager.go:17-22](file://internal/review/manager/manager.go#L17-L22)
- [review_repo.go:106-126](file://internal/database/review_repo.go#L106-L126)

## 架构概览

### 系统架构图

```mermaid
graph TB
subgraph "客户端层"
A[前端应用]
B[第三方集成]
end
subgraph "API网关层"
C[Gin Web框架]
D[路由处理]
E[中间件]
end
subgraph "业务逻辑层"
F[处理控制器]
G[文件处理器]
H[审核管理器]
end
subgraph "数据持久层"
I[文件仓库]
J[审核仓库]
K[版本仓库]
end
subgraph "外部服务层"
L[模型修复API]
M[向量检测服务]
N[存储服务]
end
A --> C
B --> C
C --> F
C --> G
C --> H
F --> I
G --> I
H --> J
I --> K
F --> L
F --> M
I --> N
J --> N
K --> N
```

**图表来源**
- [server.go:11-57](file://web/backend/server.go#L11-L57)
- [process.go:15-77](file://web/backend/handlers/process.go#L15-L77)
- [files.go:18-71](file://web/backend/handlers/files.go#L18-L71)

### 数据流图

```mermaid
sequenceDiagram
participant Client as 客户端
participant API as API层
participant Handler as 处理器
participant Processor as 处理器核心
participant DB as 数据库
participant External as 外部服务
Client->>API : POST /api/files/ : md5/run
API->>Handler : RunAllSteps()
Handler->>DB : 查询文件状态
Handler->>Processor : ProcessStep(StepCleaning)
Processor->>External : 调用模型修复API
External-->>Processor : 返回修复结果
Processor->>DB : 更新文件状态
Processor-->>Handler : 返回处理结果
Handler-->>Client : {"success" : true, "message" : "处理已启动"}
Client->>API : GET /api/files/ : md5/status
API->>Handler : GetFileStatus()
Handler->>DB : 查询文件状态
Handler-->>Client : {"status" : "processing", "progress" : 60}
```

**图表来源**
- [process.go:16-77](file://web/backend/handlers/process.go#L16-L77)
- [pipeline.go:104-158](file://internal/processor/pipeline.go#L104-L158)

## 详细组件分析

### 处理控制API

#### 文件上传和管理

文件上传接口支持智能识别和重复处理检测：

```mermaid
flowchart TD
A["文件上传"] --> B{"验证文件类型"}
B --> |txt文件| C{"检查文件大小"}
B --> |其他| D["返回错误: 不支持的文件类型"]
C --> |超限| E["返回错误: 文件过大"]
C --> |正常| F{"MD5计算"}
F --> |失败| G["返回错误: MD5计算失败"]
F --> |成功| H{"查询数据库"}
H --> |找到记录| I{"检查状态"}
H --> |无记录| J["创建新文件记录"]
I --> |已完成| K["返回已完成状态"]
I --> |失败| L["返回失败状态"]
I --> |处理中| M["返回处理中状态"]
J --> N["保存文件到uploads目录"]
K --> O["提示重新处理"]
L --> P["提示从失败步骤继续"]
M --> Q["提示继续上次进度"]
```

**图表来源**
- [files.go:18-71](file://web/backend/handlers/files.go#L18-L71)
- [files.go:73-111](file://web/backend/handlers/files.go#L73-L111)

#### 处理执行控制

处理执行接口提供完整的异步处理机制：

```mermaid
sequenceDiagram
participant Client as 客户端
participant API as RunAllSteps
participant DB as 数据库
participant Processor as 处理器
participant Worker as 工作线程
Client->>API : POST /api/files/ : md5/run
API->>DB : GetFileByMd5(md5)
DB-->>API : 文件记录
API->>API : 检查状态(processing/reviewing)
API->>DB : UpdateFileStatus(processing, startStep, 0)
API->>Worker : 启动异步处理
Worker->>Processor : ProcessStep(StepCleaning)
Processor->>DB : CreateProcessingLog(start)
Processor->>Processor : 执行清洗步骤
Processor->>DB : UpdateFileStatus(processing, nextStep, progress)
Processor->>DB : CreateProcessingLog(complete)
Processor-->>Worker : 返回结果
Worker->>Processor : ProcessStep(StepIndexing)
Worker->>Processor : ProcessStep(StepLlmFix)
Worker->>Processor : ProcessStep(StepReview)
Processor->>DB : UpdateFileStatus(reviewing, review, progress)
Worker-->>API : 处理完成
API-->>Client : {"success" : true, "message" : "处理已启动"}
```

**图表来源**
- [process.go:16-77](file://web/backend/handlers/process.go#L16-L77)
- [pipeline.go:104-158](file://internal/processor/pipeline.go#L104-L158)

#### 处理状态查询

状态查询接口提供实时的处理进度和审核状态：

```mermaid
flowchart TD
A["GetFileStatus"] --> B["查询文件记录"]
B --> C{"文件存在?"}
C --> |否| D["返回404错误"]
C --> |是| E["查询最新处理日志"]
E --> F["查询审核进度"]
F --> G["构建响应数据"]
G --> H{"处理中且有错误?"}
H --> |是| I["添加错误消息"]
H --> |否| J["跳过错误消息"]
J --> K["添加当前动作"]
K --> L["添加审核统计"]
L --> M["返回完整状态信息"]
```

**图表来源**
- [process.go:79-121](file://web/backend/handlers/process.go#L79-L121)
- [process.go:105-118](file://web/backend/handlers/process.go#L105-L118)

#### 审核管理API

审核管理提供完整的变更审查功能：

```mermaid
classDiagram
class ReviewItem {
+int64 id
+string fileMd5
+string originalText
+string suggestedText
+string modificationType
+float64 confidence
+int positionStart
+int positionEnd
+string status
+string editedText
+string createdAt
+string resolvedAt
}
class ReviewStatus {
<<enumeration>>
pending
approved
rejected
edited
}
class ReviewSession {
+string id
+string fileId
+string processId
+array items
+datetime createdAt
+datetime lastModified
+bool completed
}
class Manager {
+map sessions
+CreateSession(sessionID, fileID, processID, suggestions)
+GetSession(sessionID)
+UpdateItemStatus(sessionID, itemID, status, note)
+GetPendingItems(sessionID)
+GetApprovedSuggestions(sessionID)
}
ReviewItem --> ReviewStatus : uses
ReviewSession --> ReviewItem : contains
Manager --> ReviewSession : manages
Manager --> ReviewItem : updates
```

**图表来源**
- [review_repo.go:9-23](file://internal/database/review_repo.go#L9-L23)
- [manager.go:24-42](file://internal/review/manager/manager.go#L24-L42)
- [manager.go:44-54](file://internal/review/manager/manager.go#L44-L54)

**章节来源**
- [process.go:123-326](file://web/backend/handlers/process.go#L123-L326)
- [review_repo.go:9-23](file://internal/database/review_repo.go#L9-L23)
- [manager.go:44-54](file://internal/review/manager/manager.go#L44-L54)

### 处理流水线组件

#### 步骤执行机制

处理流水线采用顺序执行和状态驱动的设计：

```mermaid
flowchart LR
subgraph "步骤顺序"
A[cleaning] --> B[indexing] --> C[llm_fix] --> D[review] --> E[finalizing]
end
subgraph "状态转换"
F[pending] --> G[processing] --> H[reviewing] --> I[completed]
F --> J[failed]
end
subgraph "进度计算"
K[每步基础20%]
L[步骤内进度贡献]
M[CalculateProgress(step, stepProgress)]
end
A --> K
K --> L
L --> M
```

**图表来源**
- [pipeline.go:27-25](file://internal/processor/pipeline.go#L27-L25)
- [pipeline.go:96-102](file://internal/processor/pipeline.go#L96-L102)

#### 模型修复器

模型修复器采用智能分块和并发处理机制：

```mermaid
graph TB
subgraph "智能分块"
A[SplitIntoChunks] --> B[1500-2000字符]
B --> C[段落完整性保持]
end
subgraph "并发处理"
D[WorkerPool] --> E[10个工作线程]
E --> F[任务队列]
F --> G[结果收集]
end
subgraph "缓存机制"
H[ChunkCacheRepo] --> I[幂等性缓存]
I --> J[命中率统计]
end
subgraph "错误处理"
K[API重试] --> L[本地回退]
L --> M[错误记录]
end
A --> D
D --> H
D --> K
H --> I
I --> J
K --> L
L --> M
```

**图表来源**
- [model_repairer.go:124-211](file://internal/processor/model_repairer.go#L124-L211)
- [model_repairer.go:547-563](file://internal/processor/model_repairer.go#L547-L563)
- [model_repairer.go:725-731](file://internal/processor/model_repairer.go#L725-L731)

#### 向量检测器

向量检测器提供重复内容检测功能：

```mermaid
flowchart TD
A["DetectDuplicates"] --> B["按段落分割"]
B --> C{"段落数 > 1?"}
C --> |否| D["直接返回"]
C --> |是| E["生成向量表示"]
E --> F["计算相似度"]
F --> G{"超过阈值?"}
G --> |否| D
G --> |是| H["标记重复段落"]
H --> I["移除重复段落"]
I --> J["生成变更记录"]
J --> K["返回检测结果"]
```

**图表来源**
- [vector_detector.go:36-69](file://internal/processor/vector_detector.go#L36-L69)
- [vector_detector.go:113-130](file://internal/processor/vector_detector.go#L113-L130)

**章节来源**
- [pipeline.go:104-158](file://internal/processor/pipeline.go#L104-L158)
- [model_repairer.go:77-122](file://internal/processor/model_repairer.go#L77-L122)
- [vector_detector.go:36-69](file://internal/processor/vector_detector.go#L36-L69)

## 依赖关系分析

### 组件依赖图

```mermaid
graph TB
subgraph "API层"
A[server.go]
B[process.go]
C[files.go]
end
subgraph "处理器层"
D[pipeline.go]
E[processor.go]
F[model_repairer.go]
G[vector_detector.go]
end
subgraph "数据访问层"
H[file_repo.go]
I[review_repo.go]
J[manager.go]
end
subgraph "外部依赖"
K[external API]
L[数据库]
M[文件系统]
end
A --> B
A --> C
B --> D
C --> H
D --> E
D --> F
D --> G
E --> H
F --> H
G --> H
D --> I
I --> J
F --> K
G --> K
H --> L
I --> L
J --> M
```

**图表来源**
- [server.go:11-57](file://web/backend/server.go#L11-L57)
- [process.go:11-12](file://web/backend/handlers/process.go#L11-L12)
- [files.go:13-15](file://web/backend/handlers/files.go#L13-L15)

### 数据模型关系

```mermaid
erDiagram
FILE {
string md5 PK
string original_md5
string author
string title
string file_name
int64 file_size
string file_path
string status
string current_step
int progress
string rules_config
string created_at
string updated_at
string error_msg
}
REVIEW_ITEM {
int64 id PK
string file_md5 FK
string original_text
string suggested_text
string modification_type
float64 confidence
int position_start
int position_end
string status
string edited_text
string created_at
string resolved_at
}
VERSION {
int64 id PK
string original_md5 FK
string version_md5
string parent_md5
string version_type
string file_path
string step
string created_at
}
PROCESSING_LOG {
int64 id PK
string file_md5 FK
string step
string action
string status
string details
string created_at
}
FILE ||--o{ REVIEW_ITEM : contains
FILE ||--o{ VERSION : generates
FILE ||--o{ PROCESSING_LOG : records
```

**图表来源**
- [file_repo.go:9-26](file://internal/database/file_repo.go#L9-L26)
- [review_repo.go:9-23](file://internal/database/review_repo.go#L9-L23)
- [file_repo.go:28-47](file://internal/database/file_repo.go#L28-L47)

**章节来源**
- [file_repo.go:9-26](file://internal/database/file_repo.go#L9-L26)
- [review_repo.go:9-23](file://internal/database/review_repo.go#L9-L23)

## 性能考虑

### 并发处理优化

系统采用多线程并发处理来提升性能：

- **工作池并发**：默认10个工作线程处理文本块
- **智能分块**：1500-2000字符的块大小平衡处理效率
- **缓存机制**：幂等性缓存减少重复API调用
- **异步处理**：处理过程完全异步执行，不阻塞API响应

### 内存管理

- **流式处理**：大文件采用流式读取避免内存溢出
- **分块处理**：文本按段落分块处理，保持内存使用稳定
- **及时释放**：处理完成后及时释放临时资源

### 错误恢复策略

```mermaid
flowchart TD
A["处理失败"] --> B{"检查错误类型"}
B --> |网络错误| C["重试机制"]
B --> |API限流| D["指数退避"]
B --> |缓存错误| E["本地回退"]
B --> |业务错误| F["状态标记失败"]
C --> G["最多3次重试"]
D --> H["等待1秒后重试"]
E --> I["使用本地修复算法"]
F --> J["更新文件状态"]
G --> K["重试成功?"]
H --> K
I --> L["继续处理"]
K --> |是| L
K --> |否| M["记录最终失败"]
J --> N["通知监控系统"]
M --> N
L --> O["继续后续步骤"]
```

**图表来源**
- [model_repairer.go:251-319](file://internal/processor/model_repairer.go#L251-L319)
- [pipeline.go:145-155](file://internal/processor/pipeline.go#L145-L155)

## 故障排除指南

### 常见问题诊断

#### 处理状态异常

**问题**：文件状态长时间停留在processing
**诊断步骤**：
1. 检查数据库连接状态
2. 验证外部API服务可用性
3. 查看处理日志文件
4. 检查工作池线程状态

**解决方案**：
- 重启API服务
- 检查网络连接
- 清理缓存数据
- 重新启动处理流程

#### 审核进度不更新

**问题**：审核进度显示为0%
**诊断步骤**：
1. 检查审核项是否正确创建
2. 验证数据库连接
3. 查看审核项状态更新日志

**解决方案**：
- 重新创建审核会话
- 检查数据库权限
- 验证审核项数据完整性

#### API响应超时

**问题**：处理请求响应时间过长
**诊断步骤**：
1. 检查服务器CPU使用率
2. 验证数据库性能
3. 监控外部API响应时间
4. 检查网络延迟

**解决方案**：
- 增加工作线程数量
- 优化数据库查询
- 调整API超时设置
- 实施负载均衡

**章节来源**
- [process.go:52-71](file://web/backend/handlers/process.go#L52-L71)
- [model_repairer.go:251-319](file://internal/processor/model_repairer.go#L251-L319)

## 结论

处理控制API提供了完整的文本处理流水线管理功能，具有以下特点：

**核心优势**：
- **异步处理**：支持长时间运行的处理任务
- **状态监控**：实时跟踪处理进度和状态
- **审核管理**：完整的变更审查流程
- **错误恢复**：健壮的错误处理和恢复机制

**技术特色**：
- 分层架构设计，职责清晰
- 并发处理优化，性能优异
- 缓存机制，减少重复计算
- 完整的日志记录，便于调试

**适用场景**：
- 大规模文本清理项目
- 需要人工审核的文本处理
- 高并发的文本处理服务
- 需要详细处理报告的业务场景

## 附录

### API调用示例

#### 基本使用流程

1. **上传文件**
```bash
curl -X POST http://localhost:8080/api/files/upload \
  -F "file=@/path/to/text.txt"
```

2. **启动处理**
```bash
curl -X POST http://localhost:8080/api/files/{md5}/run
```

3. **监控状态**
```bash
curl http://localhost:8080/api/files/{md5}/status
```

4. **获取审核项**
```bash
curl http://localhost:8080/api/files/{md5}/review-items
```

5. **批准审核项**
```bash
curl -X POST http://localhost:8080/api/files/{md5}/approve \
  -H "Content-Type: application/json" \
  -d '{"itemId": 1}'
```

6. **生成最终文件**
```bash
curl -X POST http://localhost:8080/api/files/{md5}/finalize
```

#### 最佳实践

**状态监控建议**：
- 每30秒轮询一次处理状态
- 监控处理进度变化趋势
- 设置合理的超时时间
- 实现自动重试机制

**错误处理策略**：
- 实现指数退避重试
- 记录详细的错误日志
- 提供用户友好的错误信息
- 实施降级处理方案

**性能优化建议**：
- 合理设置工作线程数量
- 优化数据库查询性能
- 实施缓存策略
- 监控系统资源使用情况