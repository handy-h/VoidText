# 综合修复报告 — 向量检测/Ollama/安全/死代码

> 创建日期：2026-05-23
> 状态：✅ **已解决** — 2026-05-23

---

## 问题描述

用户发现四类问题：

1. **向量检测无效**：API 后台 `text-embedding-v1` 调用量为零，向量检测从未调用外部 Embedding API
2. **Ollama 无效**：本地 Ollama 后台未收到任何请求，混合模型架构未接入流水线
3. **安全漏洞**：`AuthMiddleware` 已实现但未挂载，`API_TOKEN` 配置形同虚设
4. **大量死代码**：多个文件和函数从未被调用

---

## 根本原因分析

### 1. 向量检测（Embedding API 零调用）

- `generateVectors()` 永远执行本地 FNV-1a 哈希，不读取 `VectorModelType` 字段
- `VectorDetector` 未持有任何 API 客户端引用
- `NewAPI()` 使用 `LLMApiURL/LLMApiKey`，不适合 Embedding；`VectorModelURL/VectorModelApiKey` 从未被使用
- 历史原因：上次清理死代码时删除了 `generateEmbeddings()`（P2-full 标注未执行）

### 2. Ollama 未收到请求

- `cmd/voidtext/main.go` 从未调用 `processor.GetHealthManager().Start()`，健康检查后台从未启动
- `model_repairer.go:RepairParagraph()` 只有 `api/本地字典` 两条路径，无 Ollama 分支
- `OllamaClient.Generate()` 在整个项目中零调用

### 3. AuthMiddleware 未挂载

- `web/backend/server.go` 没有 `import middleware`，也没有 `r.Use(middleware.AuthMiddleware())`
- 同时 CORS `AllowHeaders` 缺少 `X-API-Token`，即使挂载也会被 CORS 拦截

### 4. 死代码

- `state_manager.go`、`evolver_monitor.go`、`prompt_manager.go`：整个文件零调用
- `handlers/health.go`：5 个 handler 均未注册路由
- 多个孤立函数：`detectCommonTypos`、`formatFileSize`、`NewLocalAPI`、`GenerateSummary` 等

---

## 修复内容

### P0：挂载 AuthMiddleware
**文件**：`web/backend/server.go`
- 添加 `import "voidtext/web/backend/middleware"`
- CORS `AllowHeaders` 追加 `"X-API-Token"`
- 添加 `r.Use(middleware.AuthMiddleware())` 在 CORS 之后

### P1：Embedding API 接入向量检测
**文件**：`internal/external/api.go`
- 新增 `NewEmbeddingAPI()`，使用 `VectorModelURL`/`VectorModelApiKey`/`VectorModelName`

**文件**：`internal/processor/vector_detector.go`
- 添加 `apiClient *external.API` 字段
- `NewVectorDetector`：`modelType=="api"` 时调用 `NewEmbeddingAPI()`
- `generateVectors()` 签名改为 `([]string) ([][]float64, error)`：API 模式调用 `GenerateEmbedding()`，提取 `resp.Data[i].Embedding`，失败直接报错
- `DetectDuplicates()` 签名改为 `(string) (VectorDetectionResult, error)`

**文件**：`internal/processor/pipeline.go`
- 更新 `processIndexingStep` 处理 `DetectDuplicates` 返回的 error

### P1：Ollama 接入修复流水线
**文件**：`cmd/voidtext/main.go`
- 添加 `processor.GetHealthManager().Start()`

**文件**：`internal/processor/model_repairer.go`
- 添加 `ollamaClient *external.OllamaClient` 字段
- `NewModelRepairer`：`EnableLocalModel==true` 时创建 OllamaClient
- `RepairParagraph`：新增优先级 Ollama → 远程 API → 本地字典
- 新增 `repairWithOllama()` 方法，失败时降级到远程 API 或本地字典

### P1：修复 removeAdContent 空实现
**文件**：`internal/processor/pipeline.go`
- 实现真正的正则替换，`pattern` 参数现在生效

### P2：死代码清理（删除文件）
- ✅ 删除 `internal/processor/state_manager.go`
- ✅ 删除 `internal/processor/evolver_monitor.go`
- ✅ 删除 `internal/processor/prompt_manager.go`
- ✅ 删除 `web/backend/handlers/health.go`
- ✅ 删除 `web/backend/handlers/test/health_test.go`

### P2：死代码清理（删除函数）
- `model_repairer.go`：删除 `detectCommonTypos()`
- `handlers/files.go`：删除 `formatFileSize()`
- `external/api.go`：删除 `NewLocalAPI()`、`GenerateSummary()`、`SetRetryConfig()`、`GetRetryConfig()`、`UpdateBaseURL()`、`UpdateAPIKey()`、`IsLocalModel()`、`GetBaseURL()`、`GetCompletionModelName()`
- `external/ollama.go`：删除 `GetModelName()`、`GetBaseURL()`
- `health_check.go`：删除孤立方法 `runHealthChecks()`

### P3：ListFiles 真实分页
**文件**：`internal/database/file_repo.go`
- `ListAllFiles()` 升级为 `ListAllFiles(limit, offset int) ([]FileRecord, int, error)`，SQL 添加 `LIMIT ? OFFSET ?` 和 `COUNT(*)`

**文件**：`web/backend/handlers/files.go`
- `ListFiles` handler 将 `limit`/`offset` 真正传递给数据库查询

---

## 测试结果

```
go build ./...   ✅ 无错误
go test ./...    ✅ 全部通过（跳过1个预先存在的未实现测试）
```

同步更新了受影响的测试文件：
- `internal/processor/vector_detector_test.go`：更新 `DetectDuplicates`/`generateVectors` 调用签名
- `internal/processor/model_repairer_test.go`：修复 `repairLocally` 参数
- `internal/database/db_test.go`：适配新的分页签名
