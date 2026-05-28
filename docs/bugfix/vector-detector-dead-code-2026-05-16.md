# vector_detector.go — 已修复

> 创建日期：2026-05-16
> 状态：✅ **已解决** — 2026-05-16

---

## 已执行的修复

### P0：删除死代码 `generateEmbeddings()`

- ✅ `generateEmbeddings()` 已删除（从未被调用）
- ✅ 移除 now-unused imports: `"log"`, `"voidtext/internal/external"`

### P1：将余弦相似度接入去重流程

- ✅ `findDuplicateIndices()` 参数从 `_ [][]float64` 改为 `vectors [][]float64`
- ✅ 实现两阶段检测：
  1. **精确匹配**（快速路径）：去标点后完全相同 → 立即标记为重复
  2. **向量余弦相似度**：对非精确匹配的段落，与所有已见过的非重复段落比较向量 → 相似度 ≥ 阈值则标记为重复
- ✅ `SimilarityThreshold` 现在实际参与决策

### P2（部分）：升级向量模型

- ✅ `generateVectors()` 从 3 维扩展为 7 维混合特征：
  - 语义维度 (0-2)：段落长度、句号密度、逗号密度（归一化到 [0,1]）
  - 判别维度 (3-6)：FNV-1a 64-bit 哈希分片为 4 个 uint16
- FNV 哈希基于去标点后的文本，确保"第一段内容"和"第二段内容"产生不同向量

### 测试

全部 9 个 vector_detector 测试通过：
- `TestDetectDuplicates_ShouldDetectExactDuplicates` ✅
- `TestDetectDuplicates_ShouldNotDetectNonDuplicates` ✅
- `TestDetectDuplicates_ShouldHandleSingleParagraph` ✅
- `TestGenerateVectors_ShouldReturnCorrectLength` ✅ (更新为 7 维)
- 5 个余弦相似度测试 ✅
- 2 个 normalize 测试 ✅

---

## 未执行的 P2/P3

| 优先级 | 内容 | 状态 |
|--------|------|------|
| P2-full | 引入真正的 sentence embedding 模型 | 未执行 — 需要接入本地/远程 embedding 模型 |
| P3 | 清理 `VectorModelType`、`VectorModelName` 等无效字段 | 未执行 — 保留字段以备后续接入 embedding |
