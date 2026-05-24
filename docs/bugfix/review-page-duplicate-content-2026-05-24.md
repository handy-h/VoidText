# 审核页面重复内容问题记录

> 创建日期：2026-05-24
> 状态：⚠️ **待修复**

---

## 问题描述

审核页面在同一原文行存在多个审核项时，会出现大量“重复内容”：

1. 同一行原文附近连续出现多条几乎相同的建议行
2. 用户视觉上会感觉同一句被重复渲染了很多次
3. 当同一行有多个局部替换建议时，重复感尤其明显

该问题在 LLM 修复阶段生成的审核项中更常见，因为单行内可能同时包含多个字符级修改。

---

## 根本原因分析

### 1. 前端按“每个审核项一整行”展开显示

文件：`web/frontend/static/js/modules/review.js`

- `createReviewLines()` 会先渲染 1 条原文行
- 然后对同一行内的每个审核项分别渲染 1 条建议行
- 因此同一原文行只要命中多个审核项，就会被展开成 `1 条原文 + N 条建议`

关键逻辑：

- `sortedItems.forEach(...)` 为每个审核项单独生成 `suggestedHtml`
- 后续再次遍历 `sortedItems`，把每个审核项各自追加为一条 `.line-suggested`

这属于当前页面的展示策略问题，不是简单的 DOM 重复插入。

### 2. 后端审核项按单个 change 逐条入库

文件：`internal/processor/pipeline.go`

- LLM 修复完成后，`repairResult.Changes` 中的每个 change 都会单独写入 `review_items`
- 如果同一行存在多个修改点，就会生成多条 review item
- 前端再按逐条展开显示时，整行文本会被反复带出

这意味着数据模型本身是“一个修改点一条审核项”，而页面却用“整行 diff”承载“单点审核项”，两者粒度不一致。

### 3. 审核页底稿来自当前 `file_path`，不是稳定审核基线

文件：`web/backend/handlers/files.go`
文件：`web/backend/handlers/process.go`
文件：`internal/processor/pipeline.go`

- 审核页面会调用 `/api/files/:md5/content` 读取全文
- `GetFileContent()` 实际读取的是 `record.FilePath`
- `saveIntermediateFile()` 在每一步处理中都会把 `files.file_path` 更新为当前中间产物
- 到审核阶段时，`file_path` 往往已经指向 LLM 修复后的中间文件

结果是：

- 页面全文底稿可能已经是“修复后文本”
- 但审核项仍然以“原文片段 -> 建议片段”的形式逐条展示
- 同一行会出现“底稿已变化，但建议仍逐条展开”的混合视图，进一步放大重复和混乱感

---

## 影响范围

涉及文件：

- `web/frontend/static/js/modules/review.js`
- `web/frontend/static/js/diff-utils.js`
- `web/backend/handlers/files.go`
- `web/backend/handlers/process.go`
- `internal/processor/pipeline.go`

影响表现：

- 审核效率下降，用户难以判断哪些是独立建议、哪些只是同一行的重复展开
- 多修改点行的可读性明显变差
- “原文 / 建议”关系容易被误解为数据重复或审核项重复生成

---

## 结论

该问题本质上是三个因素叠加：

1. 审核项粒度是“单个 change”
2. 审核页面展示粒度是“整行 diff”
3. 审核底稿使用的是会被流水线持续覆盖的 `file_path`

因此用户看到的“重复内容”并不完全等同于数据重复写入，更准确地说，是当前审核页的数据粒度与展示粒度不匹配，且底稿选择不稳定。

---

## 备注

本记录仅用于归档问题，当前未实施修复。
