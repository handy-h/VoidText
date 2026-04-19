# API 接口

## 1. 接口概述

- **基础路径**: `/api`
- **数据格式**: JSON
- **认证**: 无

## 2. 文件接口

### 2.1 上传文件

```
POST /api/files/upload
Content-Type: multipart/form-data
```

**请求参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| file | file | 上传的 txt 文件 |

**响应**:
```json
{
  "success": true,
  "md5": "79df38754d28c2b46a3b9d4f77d67740",
  "message": "文件上传成功"
}
```

### 2.2 列出所有文件

```
GET /api/files
```

**响应**:
```json
{
  "success": true,
  "files": [
    {
      "id": 1,
      "md5": "79df38754d28c2b46a3b9d4f77d67740",
      "author": "肖忉",
      "title": "赶尸家族",
      "fileName": "肖忉~赶尸家族.txt",
      "fileSize": 2048576,
      "status": "processing",
      "currentStep": "llm_fix",
      "progress": 60,
      "createdAt": "2026-04-19T10:00:00Z",
      "updatedAt": "2026-04-19T12:00:00Z"
    }
  ]
}
```

### 2.3 获取文件详情

```
GET /api/files/:md5
```

**响应**:
```json
{
  "success": true,
  "file": {
    "md5": "79df38754d28c2b46a3b9d4f77d67740",
    "author": "肖忉",
    "title": "赶尸家族",
    "fileName": "肖忉~赶尸家族.txt",
    "fileSize": 2048576,
    "status": "processing",
    "currentStep": "llm_fix",
    "progress": 60,
    "rulesConfig": {},
    "createdAt": "2026-04-19T10:00:00Z",
    "updatedAt": "2026-04-19T12:00:00Z"
  }
}
```

### 2.4 获取文件内容

```
GET /api/files/:md5/content
```

**响应**:
```json
{
  "success": true,
  "content": "第1章 初遇\n\n小明是一个普通的学生..."
}
```

### 2.5 下载文件

```
GET /api/files/:md5/download
```

**响应**: 文件下载（Content-Type: text/plain）

### 2.6 删除文件

```
DELETE /api/files/:md5
```

**响应**:
```json
{
  "success": true,
  "message": "文件已删除"
}
```

### 2.7 恢复文件处理

```
POST /api/files/:md5/resume
```

**支持的恢复场景**:
- `processing`: 重置为 pending，保留 current_step
- `failed`: 重置为 pending，保留 current_step
- `completed`: 清除审核记录，重置为 pending

**响应**:
```json
{
  "success": true,
  "message": "文件已恢复"
}
```

## 3. 处理接口

### 3.1 执行处理

```
POST /api/files/:md5/run
```

**响应**:
```json
{
  "success": true,
  "message": "处理已启动",
  "currentStep": "cleaning",
  "nextStep": "indexing"
}
```

### 3.2 获取处理状态

```
GET /api/files/:md5/status
```

**响应**:
```json
{
  "success": true,
  "md5": "79df38754d28c2b46a3b9d4f77d67740",
  "status": "processing",
  "currentStep": "llm_fix",
  "progress": 60,
  "message": "正在修复段落 5/100",
  "currentAction": "正在修复段落 5/100: 她高兴及了...",
  "errorMsg": "",
  "author": "肖忉",
  "title": "赶尸家族",
  "fileName": "肖忉~赶尸家族.txt"
}
```

### 3.3 生成最终文件

```
POST /api/files/:md5/finalize
```

**前置条件**: 所有审核项已处理完毕

**响应**:
```json
{
  "success": true,
  "message": "最终文件已生成",
  "md5": "abc123..."
}
```

## 4. 审核接口

### 4.1 获取审核项列表

```
GET /api/files/:md5/review-items
```

**查询参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| status | string | 筛选状态（pending/accepted/rejected/edited） |

**响应**:
```json
{
  "success": true,
  "items": [
    {
      "id": 1,
      "originalText": "她高兴及了",
      "suggestedText": "她高兴极了",
      "modificationType": "错别字",
      "confidence": 0.95,
      "lineNumber": 5,
      "contextBefore": "小明跑过来迎接她。",
      "contextAfter": "他开心地笑了。",
      "status": "pending",
      "createdAt": "2026-04-19T10:00:00Z"
    }
  ],
  "total": 100,
  "pending": 60,
  "accepted": 30,
  "rejected": 10
}
```

### 4.2 通过审核项

```
POST /api/files/:md5/approve
Content-Type: application/json
```

**请求体**:
```json
{
  "id": 1
}
```

**响应**:
```json
{
  "success": true,
  "message": "已通过"
}
```

### 4.3 拒绝审核项

```
POST /api/files/:md5/reject
Content-Type: application/json
```

**请求体**:
```json
{
  "id": 1
}
```

**响应**:
```json
{
  "success": true,
  "message": "已拒绝"
}
```

### 4.4 编辑审核项

```
POST /api/files/:md5/edit
Content-Type: application/json
```

**请求体**:
```json
{
  "id": 1,
  "editedText": "她高兴极了"
}
```

**响应**:
```json
{
  "success": true,
  "message": "已更新"
}
```

### 4.5 恢复审核项

```
POST /api/files/:md5/restore
Content-Type: application/json
```

**请求体**:
```json
{
  "id": 1
}
```

**响应**:
```json
{
  "success": true,
  "message": "已恢复"
}
```

### 4.6 批量通过

```
POST /api/files/:md5/batch-approve
Content-Type: application/json
```

**请求体**:
```json
{
  "ids": [1, 2, 3]
}
```

**响应**:
```json
{
  "success": true,
  "message": "已批量通过",
  "count": 3
}
```

### 4.7 批量拒绝

```
POST /api/files/:md5/batch-reject
Content-Type: application/json
```

**请求体**:
```json
{
  "ids": [4, 5, 6]
}
```

**响应**:
```json
{
  "success": true,
  "message": "已批量拒绝",
  "count": 3
}
```

## 5. 规则接口

### 5.1 获取文件规则

```
GET /api/files/:md5/rules
```

**响应**:
```json
{
  "success": true,
  "rules": {
    "enableBasicCleaning": true,
    "enableVectorDetection": true,
    "enableModelRepair": true,
    "traditionalToSimple": false,
    "vectorSimilarityThreshold": 0.95,
    "customMappings": {
      "高兴及了": "高兴极了"
    },
    "adBlacklist": [
      "小说大全",
      "免费阅读"
    ]
  }
}
```

### 5.2 更新文件规则

```
PUT /api/files/:md5/rules
Content-Type: application/json
```

**请求体**:
```json
{
  "enableBasicCleaning": true,
  "enableVectorDetection": true,
  "enableModelRepair": true,
  "traditionalToSimple": false,
  "vectorSimilarityThreshold": 0.95,
  "customMappings": {
    "高兴及了": "高兴极了"
  },
  "adBlacklist": [
    "小说大全"
  ]
}
```

**响应**:
```json
{
  "success": true,
  "message": "规则已更新"
}
```

## 6. 版本接口

### 6.1 获取版本链

```
GET /api/files/:md5/versions
```

**响应**:
```json
{
  "success": true,
  "versions": [
    {
      "md5": "abc123...",
      "versionType": "original",
      "createdAt": "2026-04-19T10:00:00Z"
    },
    {
      "md5": "def456...",
      "versionType": "intermediate",
      "parentMd5": "abc123...",
      "createdAt": "2026-04-19T10:05:00Z"
    }
  ]
}
```

### 6.2 获取处理报告

```
GET /api/files/:md5/report
```

**响应**:
```json
{
  "success": true,
  "report": {
    "fileMd5": "79df38754d28c2b46a3b9d4f77d67740",
    "author": "肖忉",
    "title": "赶尸家族",
    "status": "completed",
    "stats": {
      "totalItems": 100,
      "accepted": 85,
      "rejected": 15,
      "edited": 0
    },
    "versions": [...],
    "processingLogs": [...],
    "generatedAt": "2026-04-19T14:00:00Z"
  }
}
```

## 7. 错误响应

所有接口的错误响应格式：

```json
{
  "success": false,
  "message": "错误描述信息"
}
```

常见 HTTP 状态码：
| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 404 | 文件不存在 |
| 500 | 服务器内部错误 |