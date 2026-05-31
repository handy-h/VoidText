# 文件处理状态修复

## 问题描述

用户报告：重新提交文件时，一直提示"等待配置处理"，但是提交又提示"文件已在处理中"。

## 根本原因

这是一个**状态不一致**问题，发生在服务器重启后：

### 状态管理机制

系统使用**双重状态管理**：

1. **数据库状态**（持久化）
   - 存储在 SQLite 的 `files` 表中
   - 字段：`status`（pending/processing/reviewing/completed/failed/cancelled）
   - 服务器重启后**保留**

2. **内存状态**（临时）
   - 存储在 `processingFiles` map 中（`web/backend/handlers/process.go:21`）
   - 用于防止同一文件被重复提交处理
   - 服务器重启后**清空**

### 问题场景

```
1. 用户上传文件 A，开始处理
   - 数据库状态：status = "processing"
   - 内存状态：processingFiles["A"] = true

2. 服务器重启（或崩溃）
   - 数据库状态：status = "processing" ✅ 保留
   - 内存状态：processingFiles = {} ❌ 清空

3. 用户重新提交文件 A
   - 检查数据库：status = "processing" → 提示"文件正在处理中"
   - 检查内存：processingFiles["A"] = false → 允许提交
   - 结果：**状态冲突**
```

### 代码位置

在 [process.go:37-50](web/backend/handlers/process.go:37-50)：

```go
// 检查是否真有 goroutine 在处理：数据库状态为 processing 但内存中无对应任务
// 说明是服务器重启后的残留状态，允许重新处理
fileProcessingMu.Lock()
actuallyProcessing := processingFiles[fileMd5]
fileProcessingMu.Unlock()

if record.Status == "reviewing" {
    c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "文件正在审核中，请使用审核页面操作"})
    return
}
if record.Status == "processing" && actuallyProcessing {
    c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "文件正在处理中"})
    return
}
```

代码已经考虑了这种情况（`actuallyProcessing` 检查），但是**数据库状态仍然是 `processing`**，导致前端显示混乱。

## 修复方案

### 方案：启动时清理残留状态

在服务器启动时，自动将所有残留的 `processing` 状态重置为 `pending`。

### 实现

#### 1. 添加清理函数（`internal/database/file_repo.go`）

```go
// CleanupStaleProcessingStatus 清理残留的 processing 状态
// 服务器重启后，内存中的处理任务会丢失，但数据库状态可能还是 processing
// 此函数将所有 processing 状态重置为 pending，允许重新处理
func CleanupStaleProcessingStatus() error {
	result, err := db.Exec(`
		UPDATE files
		SET status = 'pending',
		    error_msg = '服务器重启，状态已重置',
		    updated_at = ?
		WHERE status = 'processing'`,
		time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("清理残留状态失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("[启动清理] 已重置 %d 个残留的 processing 状态为 pending\n", rowsAffected)
	}

	return nil
}
```

#### 2. 在启动时调用（`cmd/voidtext/main.go`）

```go
// 初始化数据库
if err := database.Init(config.AppConfigInstance.DataDir); err != nil {
    logging.Fatal("初始化数据库失败", err, nil)
}
defer database.Close()

logging.Info("数据库初始化完成", nil)

// 清理服务器重启后残留的 processing 状态
if err := database.CleanupStaleProcessingStatus(); err != nil {
    logging.Warn("清理残留状态失败", map[string]interface{}{"error": err.Error()})
}
```

## 效果

### 修复前

```
1. 服务器重启
2. 用户看到文件状态：processing（但实际没有任务在运行）
3. 用户点击"重新处理"
4. 提示："文件正在处理中"（但实际可以处理）
5. 用户困惑 😕
```

### 修复后

```
1. 服务器重启
2. 自动清理：processing → pending
3. 日志输出：[启动清理] 已重置 1 个残留的 processing 状态为 pending
4. 用户看到文件状态：pending
5. 用户点击"开始处理"
6. 正常处理 ✅
```

## 其他考虑的方案

### 方案 A：持久化内存状态（未采用）

将 `processingFiles` 持久化到数据库或文件。

**缺点：**
- 增加复杂度
- 需要处理进程 ID、goroutine 状态等
- 服务器崩溃时仍然无法恢复真实的处理进度

### 方案 B：添加心跳机制（未采用）

处理任务定期更新心跳时间戳，启动时检查心跳超时的任务。

**缺点：**
- 增加数据库写入频率
- 需要额外的心跳字段和清理逻辑
- 对于短时间处理的文件，心跳意义不大

### 方案 C：当前方案（已采用）✅

启动时简单粗暴地重置所有 `processing` 状态。

**优点：**
- 简单可靠
- 无额外开销
- 符合"服务器重启 = 所有任务中断"的语义

**缺点：**
- 如果有多个服务器实例（分布式部署），可能误杀其他实例的任务
  - **当前不适用**：项目是单实例部署，SQLite 不支持多实例写入

## 测试验证

### 测试步骤

1. 上传一个文件并开始处理
2. 在处理过程中，强制停止服务器（Ctrl+C 或 kill）
3. 重新启动服务器
4. 检查日志输出
5. 检查文件状态
6. 尝试重新处理文件

### 预期结果

```bash
# 启动日志
[启动清理] 已重置 1 个残留的 processing 状态为 pending

# 文件状态
status: "pending"
error_msg: "服务器重启，状态已重置"

# 重新处理
✅ 可以正常提交处理任务
```

## 相关文件

- [internal/database/file_repo.go](../internal/database/file_repo.go) - 添加 `CleanupStaleProcessingStatus` 函数
- [cmd/voidtext/main.go](../cmd/voidtext/main.go) - 启动时调用清理函数
- [web/backend/handlers/process.go](../web/backend/handlers/process.go) - 处理状态检查逻辑

## 注意事项

1. **不影响正常运行的任务**：只在服务器启动时执行一次清理，不会影响正在运行的任务。

2. **用户友好的错误信息**：重置后的文件会显示 `error_msg: "服务器重启，状态已重置"`，让用户知道发生了什么。

3. **日志记录**：清理操作会输出日志，方便运维人员监控。

4. **幂等性**：多次调用清理函数是安全的，不会产生副作用。

5. **向后兼容**：不影响现有的处理逻辑和数据结构。
