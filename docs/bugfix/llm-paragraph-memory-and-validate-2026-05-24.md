# LLM 段落重组内存泄漏与修复校验优化修复报告

> 创建日期：2026-05-24
> 状态：✅ **已解决** — 2026-05-24

---

## 问题描述

用户反馈一系列连续问题：

1. **修复后文本出现"他是是"重复字符** — `compareTexts` 如实反映 LLM 错误输出，没有被过滤
2. **被切断的短行未合并回同一段落** — `ENABLE_LLM_PARAGRAPH_RECONSTRUCT` 默认关闭
3. **LLM 修复阶段没有请求 Ollama** — `HealthCheckManager` 的 `ShouldUseLocalModel()` 被阻断
4. **LLM 处理导致系统内存不足/卡死** — 连续多次 OOM，最终系统无响应需强制重启
5. **文件卡在 finalizing 步骤 80%** — 审核项为零跳过审核，但无法推进到完成
6. **无换行符文件（36KB 连续文本）被当作一个段落** — `NewlineFixer` 未接入 cleaning 步骤
7. **段落重组的编辑距离计算消耗巨大** — 对 12508 字符计算 O(n²) 距离导致内存激增

---

## 根本原因分析

### 1. LLM 输出错误未被过滤

- `repairWithAPI` / `repairWithOllama` 接收 LLM 输出后直接通过 `compareTexts` 生成变更
- 无质量校验机制，不合理的输出（如"他是。"→"他是是"）被接受

### 2. ENABLE_LLM_PARAGRAPH_RECONSTRUCT 默认关闭

- `config.go` 中默认值为 `false`，段落重组不会执行
- `.env.template` 注释和默认值也未同步

### 3. HealthCheckManager 初始状态为"不健康"

- `health_check.go` 注册服务时 `Healthy: false`，需等待 `Start()` 同步检查后才变为 `true`
- 但此机制工作正常，问题只在用户未启动 Ollama 时出现

### 4. 多次内存不足/卡死

#### 4a. 段落重组 + 逐段修复双重内存开销
- `ENABLE_LLM_PARAGRAPH_RECONSTRUCT=true` 时，段落重组使用远程 API（ModelScope）
- 远程 API 返回大结果 → Go 进程持有额外副本 → 与逐段修复的 Ollama 推理叠加
- 虽 RTX 2060 6GB 显存足够，但**系统内存**（23GB）被 Go 文本副本 + Ollama 驱动缓冲区 + 编辑距离计算耗尽

#### 4b. validateRepair 编辑距离计算
- `levenshteinDistanceRunes` 对 12508 字符段落计算距离，需 12508×3103 ≈ 3800 万次字符比较
- 即使 O(min(m,n)) 空间优化，CP​​U 密集计算 + 内存分配在 Go 程序中造成压力

### 5. FilePath 被覆盖为 LLM 中间文件

- `processLlmFixStep` 更新 `record.FilePath` 指向 `xxx_llm_fix.txt` 中间文件
- 后续 `ProcessStep` 重新读取此路径，但对于无换行符文件，内容仍为连续文本
- 审核项为零（validateRepair 拒绝了所有变更）→ 跳过审核 → 卡在 finalizing

### 6. NewlineFixer 未接入清洗流水线

- `internal/processor/newline_fixer.go` 实现了完整的规则驱动换行修复
- 但 `processCleaningStep` 中从未调用它，导致模型层之前没有换行修复兜底

---

## 修复内容

### P1：新增 LLM 修复结果质量校验（首次修复）
**文件**：`internal/processor/model_repairer.go`

- 新增 `validateRepair()` 函数，校验修复结果质量
- 新增 `levenshteinDistanceRunes()` 函数（O(min(m,n)) 空间优化）
- 新增 `abs()`、`min3()` 辅助函数
- 在 `repairWithAPI` 和 `repairWithOllama` 的 `compareTexts` 前插入校验
- 校验规则：短文本（< 15 字符）长度不等即拒绝；长度差 > 2 且比例 > 20% 拒绝

### P2：段落重组配置优化
**文件**：`internal/config/config.go`（第 109 行）

- `ENABLE_LLM_PARAGRAPH_RECONSTRUCT` 默认值 `false` → `true`
- 同步更新 `.env.template`

### P3：NewlineFixer 接入清洗流水线
**文件**：`internal/processor/pipeline.go`

- 在 `processCleaningStep` 的规则应用之后、保存中间文件之前，调用 `NewlineFixer.Fix()`
- 为无换行符文件（OCR/排版文本）自动添加段落分隔

### P4：段落重组改为远程 API（移除本地 Ollama 路径）
**文件**：`internal/processor/model_repairer.go`

- `ReconstructParagraphsWithCheckpoint` 中移除 Ollama 本地路径
- 删除 `reconstructChunkWithOllama()` 方法
- 段落重组始终使用 `mr.api.GenerateChatCompletion`（远程 API）
- 本地 Ollama 仅用于逐段错别字修复

### P5：validateRepair 编辑距离范围限定（第二次修复）
**文件**：`internal/processor/model_repairer.go`

- 编辑距离计算仅对 ≤ 500 字符的短段落执行
- 长段落仅通过快速长度差比例判断
- 消除 O(n²) 计算在长文本上的内存压力

### P6：配置调优
**文件**：`.env`

| 配置 | 改前 | 改后 | 原因 |
|------|:---:|:---:|:----|
| `ENABLE_LLM_PARAGRAPH_RECONSTRUCT` | true | **false** | 段落重组内存开销太大，由 NewlineFixer 兜底 |
| `LLM_CONCURRENCY` | 2 | **1** | 串行调用 Ollama，降低内存 |
| `LOCAL_MODEL_TIMEOUT` | 180 | **300** | 长文本需要更长时间 |
| `PARAGRAPH_CHUNK_SIZE` | 8000 | **4000** | 减小分块，降低每批内存 |

---

## 最终架构

```
上传 → Cleaning (含 NewlineFixer 规则换行修复)
     → Indexing (去重)
     → LlmFix: 逐段错别字修复 (Ollama GPU, 串行 LLM_CONCURRENCY=1)
     → Review → Finalizing
```

**关键设计决策**：
- `ENABLE_LLM_PARAGRAPH_RECONSTRUCT=false` — 段落重组由远程 API 执行，但当前关闭以保稳定
- `NewlineFixer` 在 cleaning 中做规则驱动的基本换行修复
- `validateRepair` 仅对短段落计算编辑距离，长段落做快速长度校验
- GPU 推理正常（RTX 2060，93% Util，6GB 显存够用）

---

## 测试结果

```
go build ./...   ✅ 通过
go vet ./...     ✅ 通过
```

---

## 涉及文件

| 文件 | 修改类型 |
|------|:------:|
| `internal/config/config.go` | 修改默认值 |
| `internal/processor/pipeline.go` | 新增 NewlineFixer 调用 |
| `internal/processor/model_repairer.go` | 新增 validateRepair + 编辑距离计算 + 段落重组调整 |
| `internal/config/model_repairer.go` | — |
| `.env` | 调优配置 |
| `.env.template` | 同步配置模板 |
| `docs/bugfix/llm-paragraph-memory-and-validate-2026-05-24.md` | 本报告 |

---

## 后续建议

1. **如需启用段落重组**：将 `ENABLE_LLM_PARAGRAPH_RECONSTRUCT=true`，并确认远程 API 返回结果合理
2. **如需本地模型段落重组**：需部署 1B 以下模型（如 `qwen2.5:0.5b`），当前 3.2B 模型推理内存太大
3. **GPU 显存监控**：RTX 2060 6GB 中 kyara-model 占用 ~2.8GB，需确保推理时其他 GPU 进程退出
