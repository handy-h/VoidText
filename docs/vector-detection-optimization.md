# 向量检测性能优化

## 问题描述

原始实现在处理接近 1MB 的小说文件时，向量检测步骤耗时过长。主要瓶颈在于 O(n²) 的段落相似度比较。

## 性能瓶颈分析

### 原始实现

对于 n 个段落，需要进行 **n×(n-1)/2** 次余弦相似度计算：

```go
for i := 0; i < n; i++ {
    for j := 0; j < i; j++ {  // O(n²) 嵌套循环
        similarity := calculateCosineSimilarity(vectors[i], vectors[j])
        // ...
    }
}
```

**示例计算量：**
- 1MB 文件 ≈ 3000 个段落
- 需要计算：3000 × 2999 / 2 ≈ **450 万次**相似度计算
- 每次计算遍历 32 维向量（local 模式）

## 优化方案

### 1. 滑动窗口优化 ⭐⭐⭐⭐⭐

**核心思想：** 重复段落通常距离不远，只需比较最近 N 个段落。

**实现：**
```go
const defaultWindowSize = 300  // 只比较最近 300 个段落

// 滑动窗口
start := i - vd.WindowSize
if start < 0 {
    start = 0
}
for j := start; j < i; j++ {
    // 比较 i 和 j
}
```

**性能提升：**
- 原始：O(n²) → 优化后：O(n × windowSize)
- 3000 个段落：450 万次 → **90 万次**（减少 **83%**）
- 窗口大小可配置，设为 0 则恢复原始行为

### 2. 预计算向量模长 ⭐⭐⭐⭐

**核心思想：** 余弦相似度计算需要向量模长，避免重复计算 `sqrt(magnitude)`。

**实现：**
```go
type vectorMagnitude struct {
    vector    []float64
    magnitude float64  // 预计算的模长
}

// 一次性预计算所有向量模长
vecMags := make([]vectorMagnitude, n)
for i, vec := range vectors {
    mag := 0.0
    for _, v := range vec {
        mag += v * v
    }
    vecMags[i] = vectorMagnitude{
        vector:    vec,
        magnitude: math.Sqrt(mag),
    }
}

// 使用预计算的模长
func calculateCosineSimilarityOptimized(vm1, vm2 vectorMagnitude) float64 {
    dotProduct := 0.0
    for i := range vm1.vector {
        dotProduct += vm1.vector[i] * vm2.vector[i]
    }
    return dotProduct / (vm1.magnitude * vm2.magnitude)  // 直接使用
}
```

**性能提升：**
- 原始：每次计算都要 2 次 sqrt + 2 次向量遍历
- 优化后：只需 1 次点积计算
- 减少约 **30-40%** 的计算时间

### 3. 并行计算 ⭐⭐⭐

**核心思想：** 使用 goroutine 并行处理相似度计算。

**实现：**
```go
numWorkers := runtime.NumCPU()  // 根据 CPU 核心数
if numWorkers > 8 {
    numWorkers = 8  // 限制最大并行度
}

// Worker pool 模式
taskChan := make(chan compareTask, 1000)
resultChan := make(chan compareResult, 1000)

for w := 0; w < numWorkers; w++ {
    go func() {
        for task := range taskChan {
            similarity := calculateCosineSimilarity(...)
            if similarity >= threshold {
                resultChan <- result
            }
        }
    }()
}
```

**性能提升：**
- 在多核 CPU 上可获得 **2-4 倍**加速
- 8 核 CPU 理论加速比：约 **4-6 倍**（考虑调度开销）

## 综合性能提升

### 理论分析

| 优化项 | 计算量减少 | 实际加速比 |
|--------|-----------|-----------|
| 滑动窗口 (300) | 83% | 5-6x |
| 预计算模长 | 30-40% | 1.3-1.5x |
| 并行计算 (8核) | - | 4-6x |
| **综合效果** | - | **20-40x** |

### 实际测试场景

**测试文件：** 1MB 小说文本，约 3000 个段落

| 实现版本 | 计算次数 | 预计耗时 | 加速比 |
|---------|---------|---------|--------|
| 原始实现 | 450 万次 | 60-120 秒 | 1x |
| 滑动窗口 | 90 万次 | 12-24 秒 | 5x |
| + 预计算模长 | 90 万次 | 8-16 秒 | 7.5x |
| + 并行计算 (8核) | 90 万次 | **2-4 秒** | **30x** |

## 配置选项

### 滑动窗口大小

在 `vector_detector.go` 中修改：

```go
const defaultWindowSize = 300  // 默认值
```

**建议值：**
- **小文件 (<500KB):** 200-300
- **中文件 (500KB-2MB):** 300-500
- **大文件 (>2MB):** 500-1000
- **全局扫描:** 0（禁用窗口，比较所有段落）

**权衡：**
- 窗口越大，检测越全面，但速度越慢
- 窗口越小，速度越快，但可能漏掉距离较远的重复段落
- 实践中，重复段落距离超过 300 个段落的情况极少

### 并行度控制

代码会自动根据 CPU 核心数调整，最大限制为 8 个 worker：

```go
numWorkers := runtime.NumCPU()
if numWorkers > 8 {
    numWorkers = 8
}
```

## 使用方式

优化已自动启用，无需修改配置文件。原有的环境变量仍然有效：

```bash
# .env 配置
ENABLE_VECTOR_DETECTION=true
VECTOR_SIMILARITY_THRESHOLD=0.95
VECTOR_MODEL_TYPE=local  # 或 ollama / api
```

## 监控日志

优化后的实现会输出详细的性能日志：

```
[向量检测] 开始处理 3000 个段落
[向量检测] 精确匹配重复: 段落 150 与 120 相同
[向量检测] 向量相似重复: 段落 890 与 850 相似度 0.9612
[向量检测] 完成，耗时 2.34 秒，移除 45 个重复段落
```

## 注意事项

1. **滑动窗口的局限性：** 如果小说中存在距离很远的重复章节（如番外、回忆），可能无法检测到。可以通过增大 `defaultWindowSize` 或设为 0 来解决。

2. **内存使用：** 预计算模长会额外占用少量内存（每个段落多存储 1 个 float64），对于 3000 个段落约增加 24KB，可忽略不计。

3. **并行计算开销：** 对于小文件（<500 个段落），并行计算的调度开销可能大于收益，但代码会自动处理，无需担心。

4. **向量模型类型：** 优化对所有模型类型（local/ollama/api）都有效，但 ollama 和 api 模式的主要耗时在向量生成阶段，相似度计算优化的收益相对较小。

## 后续优化方向

如果仍需进一步提升性能，可以考虑：

1. **局部敏感哈希（LSH）：** 将相似向量哈希到同一个桶，只比较同桶内的段落，可将复杂度降至 O(n)
2. **近似最近邻搜索（ANN）：** 使用 HNSW、Annoy 等算法库
3. **GPU 加速：** 使用 CUDA 进行批量向量计算
4. **分块处理：** 将文本分成多个块，块内独立去重

这些优化较为复杂，建议在当前优化仍不满足需求时再考虑。
