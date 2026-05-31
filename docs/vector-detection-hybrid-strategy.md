# 向量检测混合策略优化方案

## 问题背景

- **Local 模式（32 维）**：速度快（5-10 秒），但对短片段区分度不够，误判率高
- **Ollama 模式（768 维）**：精度高，但 CPU 推理慢（6-7 小时）
- **需求**：既要精度（避免误判短片段），又要速度

## 推荐方案对比

### 方案 1：使用阿里云 Embedding API ⭐⭐⭐⭐⭐

**配置修改：**
```bash
# .env 文件
VECTOR_MODEL_TYPE=api
VECTOR_MODEL_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
VECTOR_MODEL_API_KEY=sk-a26d8b56526d412dbcb2003d317accd3
VECTOR_MODEL_NAME=text-embedding-v3
```

**性能指标：**
| 指标 | 数值 |
|------|------|
| 耗时 | 2-5 分钟 |
| 精度 | 1536 维（高） |
| 成本 | < 1 元/文件 |
| 误判率 | 极低 |

**优点：**
- ✅ 速度快（比 Ollama CPU 快 100 倍）
- ✅ 精度高（1536 维，不会误判短片段）
- ✅ 无需本地 GPU
- ✅ 成本极低

**缺点：**
- ⚠️ 需要网络连接
- ⚠️ 有少量费用（但很便宜）

**成本估算：**
```
阿里云 text-embedding-v3 价格：0.0007 元/千 tokens
1.27 MB 文件 ≈ 130 万字符 ≈ 200k tokens
成本：200 × 0.0007 = 0.14 元
```

### 方案 2：优化 Ollama 配置 ⭐⭐⭐

**2.1 使用更小的模型**

```bash
# 下载更快的模型
ollama pull mxbai-embed-large

# 修改 .env
VECTOR_MODEL_NAME=mxbai-embed-large
```

**性能对比：**
| 模型 | 维度 | 速度 | 精度 |
|------|------|------|------|
| nomic-embed-text | 768 | 慢 | 高 |
| mxbai-embed-large | 384 | 中 | 中高 |
| all-minilm | 384 | 快 | 中 |

**2.2 增大批次大小**

修改 `internal/processor/vector_detector.go:135`：
```go
const batchSize = 200  // 从 50 增加到 200
```

修改 `.env`：
```bash
LOCAL_MODEL_TIMEOUT=600
```

**预期效果：**
- 批次数：193 → 49
- 耗时：6-7 小时 → 4-5 小时（提速 30%）

### 方案 3：混合检测策略 ⭐⭐⭐⭐

**思路：**
1. **第一轮**：Local 模式快速筛选（5 秒）
2. **第二轮**：对可疑段落使用远程 API 精确验证（1-2 分钟）

**实现逻辑：**
```
1. Local 32 维向量检测（阈值 0.98）
   → 找出 100 个高度相似的段落对

2. 对这 100 个段落对，使用远程 API embedding
   → 精确验证是否真的重复

3. 最终去重
```

**效果：**
- 总耗时：5 秒 + 1 分钟 = **1-2 分钟**
- 精度：接近纯 API 模式
- 成本：只对少量段落调用 API，成本 < 0.1 元

### 方案 4：调整检测策略（无需改代码）⭐⭐

**针对短片段误判问题，调整配置：**

```bash
# .env 文件

# 1. 提高相似度阈值（更严格）
VECTOR_SIMILARITY_THRESHOLD=0.98  # 从 0.95 提高到 0.98

# 2. 使用 local 模式
VECTOR_MODEL_TYPE=local
```

**同时修改代码，增加短片段保护：**

在 `vector_detector.go` 的 `findDuplicateIndices` 函数中，增加段落长度检查：

```go
// 跳过过短的段落（避免误判）
runeLen := len([]rune(normalized))
if runeLen < 20 {  // 少于 20 个字符的段落不参与向量检测
    continue
}
```

**效果：**
- 速度：5-10 秒
- 避免短片段误判
- 免费

## 实施建议

### 立即可行：方案 1（推荐）

**步骤：**

1. **停止当前处理**
```bash
make stop
```

2. **修改 `.env`**
```bash
vim .env

# 修改这几行：
VECTOR_MODEL_TYPE=api
VECTOR_MODEL_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
VECTOR_MODEL_API_KEY=sk-a26d8b56526d412dbcb2003d317accd3
VECTOR_MODEL_NAME=text-embedding-v3
```

3. **重启服务**
```bash
make dev
```

4. **重新处理文件**
- 应该在 2-5 分钟内完成
- 精度高，不会误判短片段
- 成本 < 1 元

### 验证效果

查看日志应该显示：
```
[向量检测] 开始处理 9615 个段落
[向量检测] 完成，耗时 2.34 分钟，移除 45 个重复段落
```

### 如果 API 不可用

**备选：方案 2.1 + 2.2**

```bash
# 1. 下载更快的模型
ollama pull mxbai-embed-large

# 2. 修改 .env
VECTOR_MODEL_NAME=mxbai-embed-large
LOCAL_MODEL_TIMEOUT=600

# 3. 修改代码增大批次
vim internal/processor/vector_detector.go
# 将 const batchSize = 50 改为 200

# 4. 重新编译运行
make rebuild
```

**预期：**
- 耗时：1-2 小时（比原来快 3-5 倍）
- 精度：仍然足够高（384 维）

## 成本对比

| 方案 | 1MB 文件耗时 | 精度 | 单次成本 | 100 次成本 |
|------|-------------|------|---------|-----------|
| Local | 5-10 秒 | 低 | 0 元 | 0 元 |
| Ollama CPU | 6-7 小时 | 高 | 0 元 | 0 元 |
| **API（推荐）** | **2-5 分钟** | **高** | **< 1 元** | **< 100 元** |
| 混合策略 | 1-2 分钟 | 高 | < 0.1 元 | < 10 元 |

## 总结

**最佳方案：使用阿里云 Embedding API**

- ✅ 速度快：2-5 分钟（vs 6-7 小时）
- ✅ 精度高：1536 维（vs 32 维）
- ✅ 成本低：< 1 元/文件
- ✅ 无需 GPU
- ✅ 不会误判短片段

**操作：**
```bash
# 修改 .env 三行配置
VECTOR_MODEL_TYPE=api
VECTOR_MODEL_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
VECTOR_MODEL_NAME=text-embedding-v3

# 重启即可
make dev
```
