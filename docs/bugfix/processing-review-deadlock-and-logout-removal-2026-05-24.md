# 处理进度卡死/退出登录入口删除修复报告

> 创建日期：2026-05-24
> 状态：✅ **已解决** — 2026-05-24

---

## 问题描述

用户反馈两类问题：

1. 上传文件后，处理块全部完成却没有进入审核页面
2. 页面右上角存在“退出登录”按钮，但实际无用，需要删除

---

## 根本原因分析

### 1. 处理完成后未进入审核页面

- `internal/processor/progress_tracker.go` 之前把 `TotalChunks` 直接截断到 1000
- 大文件会出现 `已处理 > 总数` 的假进度，前端显示与真实处理量不一致
- `internal/processor/pipeline.go` 的 LLM 并发收尾存在通道关闭顺序问题
- `wg.Wait()` 会在 `jobs` 关闭前阻塞，导致最后一批结果无法进入提交和审核阶段

### 2. 无用退出登录入口

- `web/frontend/index.html` 顶部导航中存在 `退出登录` 按钮
- `web/frontend/static/js/main.js` 只有空的 `handleLogout()` 占位函数
- `web/frontend/static/css/style.css` 还保留了对应的 `.nav-logout` 样式

---

## 修复内容

### P1：修复 LLM 进度追踪与审核切换
**文件**：`internal/processor/progress_tracker.go`
- 保留真实 `TotalChunks`
- 仅限制内存中的 `ChunkTimes` 样本数量
- 对 `ProcessedChunks`、`RemainingChunks`、`Progress` 做边界保护

**文件**：`internal/processor/pipeline.go`
- 调整 LLM 并发收尾流程
- 先关闭 `jobs`，再等待 worker 退出，最后关闭 `results`
- 确保最后一批块完成后能继续写入审核项并进入 `reviewing`

**文件**：`internal/processor/progress_tracker_test.go`
- 新增大文件块数超过 1000 时的进度测试
- 新增 `ProcessedChunks` 超过 `TotalChunks` 时的边界测试

**文件**：`internal/processor/pipeline_test.go`
- 新增并发完成路径测试，防止通道死锁回归

### P2：删除无用退出登录入口
**文件**：`web/frontend/index.html`
- 删除顶部导航中的“退出登录”按钮

**文件**：`web/frontend/static/js/main.js`
- 删除空的 `handleLogout()` 占位函数

**文件**：`web/frontend/static/css/style.css`
- 删除 `.nav-logout` 样式

---

## 测试结果

```
go test ./...   ✅ 通过
go vet ./...    ✅ 通过
```

