# 向量检测去重统计增强方案

## 背景

当前向量检测去重步骤（`indexing`）在处理日志中仅记录了 `"检测到重复段: N"`，缺少减少的文字数信息。前端处理日志区域也未做专门的格式化展示。用户希望：

1. 处理日志中记录去重段落数 **和** 减少的文字数
2. Web 页面处理日志中以清晰的方式展示这些统计信息，并能查看被去除的重复段落内容

## 现状分析

### 数据流

```
vector_detector.go          pipeline.go                 process.go              processing.js
removeDuplicateParagraphs() processIndexingStep()       GetFileStatus()         updateLogs()
  ├ Stats[dup_count]         ├ 写 processing_logs        ├ currentAction         ├ 渲染日志列表
  └ Changes[原文]             └ Details: "检测到重复段: N"  └ ❌ 无 logs 字段        └ ❌ 无特殊格式化
```

### 问题点

| 位置 | 问题 |
|---|---|
| [`vector_detector.go:165`](internal/processor/vector_detector.go:165) `removeDuplicateParagraphs()` | `Stats` 仅记录段落数，缺少字符数统计 |
| [`pipeline.go:246`](internal/processor/pipeline.go:246) `processIndexingStep()` | 日志 Details 仅含段落数，格式为普通字符串 |
| [`process.go:168`](web/backend/handlers/process.go:168) `GetFileStatus()` | 响应中不含 `logs` 字段，前端处理期间看不到日志 |
| [`processing.js:115`](web/frontend/static/js/modules/processing.js:115) `updateLogs()` | 无针对 indexing 步骤的特殊格式化，日志展示为纯文本 |

## 方案设计

### 1. 后端：补充统计字段

#### 1.1 [`vector_detector.go`](internal/processor/vector_detector.go) — `removeDuplicateParagraphs()`

在现有 `Stats["duplicate_paragraphs_removed"]` 基础上，新增 `Stats["duplicate_chars_removed"]`，累加每个被删除段落的 `len([]rune(paragraph))`。

```go
// 现有
result.Stats["duplicate_paragraphs_removed"] = len(duplicateIndices)

// 新增
charsRemoved := 0
for _, idx := range duplicateIndices {
    charsRemoved += len([]rune(paragraphs[idx]))
}
result.Stats["duplicate_chars_removed"] = charsRemoved
```

#### 1.2 [`pipeline.go`](internal/processor/pipeline.go) — `processIndexingStep()`

将日志 Details 从普通字符串改为 JSON 结构，便于前端解析和格式化展示：

```go
// 现有
Details: fmt.Sprintf("检测到重复段: %d", len(records))

// 改为
detailsJSON, _ := json.Marshal(map[string]interface{}{
    "action":              "vector_dedup_complete",
    "duplicate_paragraphs": len(records),
    "duplicate_chars":     detectResult.Stats["duplicate_chars_removed"],
})
Details: string(detailsJSON)
```

同时将重复段落的原文列表也存入日志，供前端展开查看：

```go
var dupParagraphs []string
for _, change := range detectResult.Changes {
    if change.Type == "duplicate_paragraph" {
        dupParagraphs = append(dupParagraphs, change.Original)
    }
}
detailsJSON, _ := json.Marshal(map[string]interface{}{
    "action":               "vector_dedup_complete",
    "duplicate_paragraphs": len(records),
    "duplicate_chars":      detectResult.Stats["duplicate_chars_removed"],
    "removed_contents":     dupParagraphs,  // 被去除的段落原文
})
```

> **注意**：`removed_contents` 可能较大。如果段落数超过 50 或总字符超过 5000，只存前 20 条 + 截断提示，避免日志表膨胀。

#### 1.3 [`process.go`](web/backend/handlers/process.go) — `GetFileStatus()`

在响应中补充 `logs` 字段，让前端轮询时能获取日志：

```go
// 现有：仅 latestLog → currentAction
// 新增：返回完整日志列表
logs, _ := database.GetProcessingLogsByFileMd5(fileMd5)
response["logs"] = logs
```

### 2. 前端：格式化日志展示

#### 2.1 [`processing.js`](web/frontend/static/js/modules/processing.js) — `updateLogs()`

在现有的 JSON 解析分支中，增加对 `vector_dedup_complete` action 的处理：

```
解析 details JSON
├ action === "vector_dedup_complete"
│   ├ 显示摘要：`🔍 向量检测完成 — 去除重复段落 N 段，减少 M 字`
│   └ 展开按钮：点击后显示被去除的段落原文列表
├ action === "step_started" / "step_completed" / ...  （已有逻辑）
└ 其他：保持现有纯文本展示
```

展开区域的 DOM 结构：

```html
<div class="log-item log-success">
  <span class="log-time">14:30:05</span>
  <span class="log-step">向量检测</span>
  <span class="log-message">
    <span class="dedup-badge">去除 3 段</span>
    <span class="dedup-badge">减少 512 字</span>
    <button class="dedup-detail-toggle">查看详情 ▼</button>
  </span>
</div>
<div class="dedup-detail-panel" style="display:none">
  <div class="dedup-paragraph">重复段落1的原文内容...</div>
  <div class="dedup-paragraph">重复段落2的原文内容...</div>
  <div class="dedup-paragraph">重复段落3的原文内容...</div>
</div>
```

#### 2.2 [`style.css`](web/frontend/static/css/style.css) — 新增样式

为去重统计添加专用样式：

- `.dedup-badge`：彩色徽章，段落数用蓝色，字数用绿色
- `.dedup-detail-toggle`：展开/收起按钮
- `.dedup-detail-panel`：展开面板，带灰色背景和左边框
- `.dedup-paragraph`：每个重复段落的样式，带序号

## 涉及文件

| 文件 | 改动类型 | 说明 |
|---|---|---|
| [`internal/processor/vector_detector.go`](internal/processor/vector_detector.go) | 修改 | `removeDuplicateParagraphs()` 增加字符数统计 |
| [`internal/processor/pipeline.go`](internal/processor/pipeline.go) | 修改 | `processIndexingStep()` 日志 Details 改为 JSON 格式，含段落数+字符数+原文 |
| [`web/backend/handlers/process.go`](web/backend/handlers/process.go) | 修改 | `GetFileStatus()` 响应补充 `logs` 字段 |
| [`web/frontend/static/js/modules/processing.js`](web/frontend/static/js/modules/processing.js) | 修改 | `updateLogs()` 增加 `vector_dedup_complete` 的格式化渲染和展开逻辑 |
| [`web/frontend/static/css/style.css`](web/frontend/static/css/style.css) | 修改 | 新增去重统计相关样式 |

## 数据示例

### processing_logs.details（JSON 格式）

```json
{
  "action": "vector_dedup_complete",
  "duplicate_paragraphs": 3,
  "duplicate_chars": 512,
  "removed_contents": [
    "他慢慢地站起身来，目光扫过四周的废墟。",
    "少年握紧手中的剑，深吸一口气。",
    "远处传来阵阵轰鸣声，大地开始颤抖。"
  ]
}
```

### 前端展示效果

```
14:30:05  [向量检测]  🔍 向量检测完成 — 去除 3 段 · 减少 512 字  [查看详情 ▼]
```

点击"查看详情"后展开：

```
14:30:05  [向量检测]  🔍 向量检测完成 — 去除 3 段 · 减少 512 字  [收起 ▲]
  ┌─────────────────────────────────────────────────────┐
  │ #1 他慢慢地站起身来，目光扫过四周的废墟。           │
  │ #2 少年握紧手中的剑，深吸一口气。                   │
  │ #3 远处传来阵阵轰鸣声，大地开始颤抖。               │
  └─────────────────────────────────────────────────────┘
```

## 注意事项

1. **日志膨胀控制**：`removed_contents` 超过 20 条或总字符超过 5000 时截断，末尾加 `"…等共 N 段"`
2. **向后兼容**：现有日志 Details 为普通字符串，前端 JSON.parse 失败时 fallback 到纯文本展示（已有逻辑）
3. **GetFileStatus 响应体积**：logs 数组可能较大，如果文件处理日志很多，考虑只返回最近 50 条
4. **重复段落已在审核页可见**：`duplicate_paragraph` 类型的审核项会在审核页面展示，处理日志中的展开是辅助查看
