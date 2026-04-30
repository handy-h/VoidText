# Debug 2026-04-30 (第二轮): UI/UX 优化 + 上下文预览 + 文件下载

## 问题概述

本日志覆盖一次大型 UI/UX 重构及其后续暴露的一系列 bug。初始需求为按设计文档重构前端界面，完成后暴露出 5 个连锁缺陷：

1. **拖拽上传区点击无效** — `pointer-events: none` 挡了 file input
2. **审核页上下文太少 + 行号=0** — 流水线修改文件后，review_items 位置偏移
3. **下载按钮无反应** — 全局 `currentFileMd5` 未同步
4. **进度显示 0%、文件 stuck** — `saveIntermediateFile` 缺少 `uploads/` 目录创建
5. **下载文件名 URI 编码** — 中文文件名未做 RFC 5987 编码

---

## 一、UI/UX 重构（初始需求）

### 1.1 设计文档要求

详见 `UIUX 优化方案设计文档.md`，核心变更：

1. **初始化页面** — 巨大的拖拽上传区替代空列表，上传后自动跳到规则配置
2. **规则配置** — 预设模板（基础清洗/深度修复/仅清洗）+ "保存并开始处理"
3. **处理进度** — 卡片进度条外显 + 自动跳转开关
4. **审核页面** — 双栏导航架构：左栏聚合面板（按类型分组 + 批量操作），右栏内联对比卡片（原文浅红背景+删除线，建议浅绿背景+加粗，悬浮操作按钮）
5. **键盘快捷键** — ↑/↓ 切换卡片，Enter 采纳，Esc 拒绝
6. **状态标签** — 灰色待配置、蓝色处理中、橙色待审核、绿色已完成

### 1.2 变更文件

| 文件 | 变更说明 |
|------|---------|
| `index.html` | 新增拖拽上传区、预设模板按钮、自动跳转开关、双栏审核布局 |
| `style.css` | ~500 行新样式：拖拽区、双栏、内联 diff、快捷提示、切换开关等 |
| `review.js` | 完全重写：分类聚合 + 内联对比卡片 + 键盘快捷键 |
| `file-manager.js` | 拖拽上传支持、上传后自动跳转、进度条外显 |
| `rules-config.js` | 预设模板（basic/deep/minimal） |
| `processing.js` | 自动跳转开关、状态转换检测 |
| `main.js` | 重构为全局兼容层 |
| `dom-utils.js` | 文件卡片添加进度条 |
| `app.js` | 简化集成 |

### 1.3 响应式

- 1024px 以下侧栏宽度收缩
- 768px 以下双栏变单栏，快捷提示隐藏

---

## 二、Bug: 拖拽上传区点击无效

### 2.1 问题

首次进入页面时，页面正中的拖拽上传区点击无反应。右上方的"上传文件"按钮工作正常。

### 2.2 根因

```css
.drop-zone-content { pointer-events: none; }
```

`pointer-events: none` 本意是防止图标和文字干扰点击事件，但它**同时也拦截了其中的 `<input type="file">`**，使文件选择框无法弹出。

### 2.3 修复

```css
.drop-zone-input { pointer-events: auto; }
```

在 `<input>` 元素上强制恢复指针事件，覆盖父级的 `none`。

---

## 三、Bug: 审核项上下文缺失 + 行号=0

### 3.1 问题

审核列表待审核项没有上下文预览，显示行数为 0。部分能定位的行也发现行号和内容不正确。

### 3.2 根因 — 位置偏移链

```
清洗步骤 → 保存 {md5}_cleaning.txt, 更新 record.FilePath
索引步骤 → 读 record.FilePath (=清洗输出), 创建审核项(位置相对清洗输出)
         → 删除重复段落 → 内容改变 → 保存 {md5}_indexing.txt, 更新 record.FilePath
LLM修复 → 读 record.FilePath (=索引输出), 创建审核项(位置相对索引输出)
        → 替换错别字 → 内容改变 → 保存 {md5}_llm_fix.txt, 更新 record.FilePath
审核    → GetReviewItems 读 record.FilePath (=LLM修复输出)
        → review_items 的 PositionStart 指向旧版本 → 位置无效
        → findMatchLine 搜索失败 → 行号=0, 无上下文
```

关键问题：
- **索引步骤的审核项**：`original` 是重复段落文本 → 已在索引输出中被删除 → 搜索不到
- **LLM修复步骤的审核项**：`original` 是错别字 → 已在LLM修复输出中被替换为 `suggested` → 原文不存在

### 3.3 修复 — 三层匹配策略

#### 第一层: 多版本文件回退 (`GetReviewItems` → `readAvailableContents`)

新增 `readAvailableContents` 函数，按从新到旧顺序读取所有中间版本文件：

```
1. record.FilePath (当前文件, =LLM修复输出)
2. {md5}_llm_fix.txt  (LLM修复输出)
3. {md5}_indexing.txt (索引步骤输出)
4. {md5}_cleaning.txt (清洗步骤输出)
```

对每个审核项，`getLineContextMulti` 逐一在各个版本内容中尝试定位，使用第一个成功的结果。

#### 第二层: 智能内容匹配 (`findMatchLine`)

`findMatchLine` 按优先级搜索：

| 优先级 | 策略 | 应对场景 |
|--------|------|---------|
| 1a | 在储存位置 ±8 行内搜索 `original` | 内容轻微偏移 |
| 1b | ±8 行内搜索 `suggested` | LLM 已替换原文 |
| 1c | ±8 行内搜索逐步缩短的原文片段 | 原文部分保留 |
| 2 | 全局搜索原文及其片段 | 位置完全失效 |
| 3 | 全局搜索建议文本及其片段 | 原文已不存 |
| 4 | 纯位置估算 | 无文本可匹配 |

#### 第三层: 绝望扫描 (`finalScan`)

新增 `finalScan` 函数，当所有策略都失败时，对整个文件逐行扫描 `original` → `suggested` → 逐步缩短到 3 个字的片段。确保即使数据高度不一致也能返回**某个**近似行。

### 3.4 涉及文件

| 文件 | 变更 |
|------|------|
| `process.go` | `GetReviewItems` 改为多版本读取；`readAvailableContents` 新增；`getLineContextMulti` 新增；`findMatchLine` 重写；`finalScan` 新增 |

---

## 四、Bug: 下载按钮无反应

### 4.1 问题

审核完成后点"下载最终文件"按钮，或处理进度页点"下载当前版本"按钮，均无反应。

### 4.2 根因

`main.js` 第 6 行的全局 `let currentFileMd5 = null` 在整个上传→配置→处理→完成的流程中**从未被设置**。

```javascript
// main.js
let currentFileMd5 = null;  // ← 永远 null

function downloadFinalFile() {
  if (currentFileMd5) {    // ← false, 提前返回
    FileManager.downloadFile(currentFileMd5);
  }
}
```

每个模块（`FileManager`、`ProcessingModule`、`RulesConfigModule`）各自管理自己的 `currentFileMd5`，但全局变量一直是 `null`。

### 4.3 修复

新增 `getCurrentFileMd5()` 辅助函数（main.js），按优先级获取 md5：全局变量 → ProcessingModule → FileManager。

```javascript
function getCurrentFileMd5() {
  return currentFileMd5 || ProcessingModule.getCurrentFileMd5() || FileManager.getCurrentFileMd5();
}
```

同时 `ProcessingModule.viewProgress(md5)` 中同步设置 `window.currentFileMd5 = md5`。

---

## 五、Bug: 自动跳转打断查看进度

### 5.1 问题

已处于审核中的文件，点文件列表"查看进度"按钮，进度页刚打开就自动跳转到审核页或文件列表，用户看不到进度。

### 5.2 根因

`updateProgress` 对已处于 `reviewing` 状态的文件也会触发自动跳转逻辑。用户主动查看进度时应该停留在进度页，只有流水线刚完成的自动跳转才应该触发。

### 5.3 修复

新增 `previousStatus` 变量跟踪状态变化（`processing.js`）。**仅当状态从上一次轮询发生了变化**（如 `processing → reviewing`）才自动跳转。用户主动点"查看进度"时 `previousStatus = null`，不会触发跳转。

---

## 六、Bug: 进度显示 0% + 文件 stuck 在"处理中"

### 6.1 问题

文件列表里文件状态显示"处理中"，进度为 0%，且永远不变。查看进度页也显示 0%。

### 6.2 根因

`saveIntermediateFile` 写入 `{DATA_DIR}/uploads/{md5}_{step}.txt` 时，`uploads/` 子目录**从未被创建**。

```go
// pipeline.go (修复前)
func saveIntermediateFile(fileMd5, step, content string) error {
    filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileMd5+"_"+step+".txt")
    if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
        return err  // ← 目录不存在, 报错!
    }
    ...
}
```

`os.WriteFile` 不会自动创建父目录。同时 `database.Init` 只创建了 `dataDir`，没有创建 `uploads/` 子目录。

### 6.3 连锁故障链

```
os.WriteFile 失败（uploads/目录不存在）
→ saveIntermediateFile 返回 err
→ processCleaningStep 返回 err
→ FileProcessingJob.Execute 中断
→ onComplete 回调调用 FailProcessingStep
→ FailProcessingStep 本身的错误被静默忽略
→ 文件状态永远 stuck 在 "processing", progress=0
```

### 6.4 修复

```go
func saveIntermediateFile(fileMd5, step, content string) error {
    dir := filepath.Join(config.AppConfigInstance.DataDir, "uploads")
    if err := os.MkdirAll(dir, 0755); err != nil {     // ← 新增
        return fmt.Errorf("创建上传目录失败: %w", err)
    }
    filePath := filepath.Join(dir, fileMd5+"_"+step+".txt")
    if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
        return err
    }
    ...
}
```

对已 stuck 的存量文件，重启服务后重新处理即可走通完整流程。

---

## 七、Bug: 下载文件名 URI 编码

### 7.1 问题

下载文件时，浏览器保存对话框显示 URI 编码的文件名（如 `%E7%8B%BC%E7%AA%9D.txt`）而不是中文。

### 7.2 根因

Gin 的 `c.FileAttachment(filePath, fileName)` 对于中文文件名只会设置：

```
Content-Disposition: attachment; filename="%E7%8B%BC%E7%AA%9D.txt"
```

浏览器只认识 ASCII 的 `filename` 参数，把 URI 编码当做了字面文件名。

### 7.3 修复

手动设置 `Content-Disposition` 头，同时提供两种参数（RFC 5987）：

```
Content-Disposition: attachment; filename="download.txt"; filename*=UTF-8''%E7%8B%BC%E7%AA%9D.txt
```

- `filename`：纯 ASCII fallback（浏览器兜底用）
- `filename*`：RFC 5987 UTF-8 编码（支持中文的正确方式）

浏览器优先使用 `filename*`，正确显示中文。

---

## 八、文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/frontend/index.html` | 重写 | 双栏审核布局、拖拽上传区、预设模板、自动跳转开关 |
| `web/frontend/static/css/style.css` | 扩充 +500行 | 拖拽区、双栏、内联 diff、切换开关、状态标签、滚动条、响应式 |
| `web/frontend/static/js/modules/review.js` | 重写 | 分类聚合 + 内联 diff 卡片 + 键盘快捷键 |
| `web/frontend/static/js/modules/file-manager.js` | 修改 | 拖拽上传、upload 暴露公共 API、下载文件名从 Content-Disposition 解析 |
| `web/frontend/static/js/modules/rules-config.js` | 修改 | 预设模板 3 套 |
| `web/frontend/static/js/modules/processing.js` | 修改 | 自动跳转开关、状态转换检测（previousStatus） |
| `web/frontend/static/js/modules/app.js` | 清理 | 简化集成 |
| `web/frontend/static/js/main.js` | 重写 | 全局兼容层 + getCurrentFileMd5 |
| `web/frontend/static/js/dom-utils.js` | 修改 | 文件卡片进度条 |
| `web/backend/handlers/process.go` | 重写多处 | `getLineContext`/`findMatchLine`/`GetReviewItems` 重写；新增 `readAvailableContents`/`getLineContextMulti`/`finalScan`/`findMatchLine` |
| `web/backend/handlers/files.go` | 修改 | `DownloadFile` RFC 5987 编码 |
| `internal/processor/pipeline.go` | 修改 | `saveIntermediateFile` 新增 `os.MkdirAll` |

---

## 九、验证方法

### 后端验证

```bash
go build -o voidtext ./cmd/voidtext/ && go vet ./...
# 预期: 无输出 (编译通过)
```

### 手动测试路径

```
1. 上传文件 → 看到拖拽上传区 → 点击/拖拽上传 → 自动跳到规则配置
2. 规则配置 → 点击预设模板（基础/深度/仅清洗）→ 表单自动填充
3. 点击保存并开始处理 → 跳到进度页 → 进度从 0% → 20% → 40% → 60%
4. 进度到 60% → 自动跳到审核页（双栏布局）
5. 审核页 → 左栏显示分类 → 右栏显示内联 diff 卡片 → 有上下各 3 行上下文
6. ↑/↓ 切换卡片 → Enter 采纳 → Esc 拒绝
7. 全部处理完成 → 点下载 → 文件名正确显示中文
8. 列表页 → 已完成文件 → 下载 → 文件名含"修订版"或原名
```

### 键盘快捷键测试

```javascript
// 打开审核页后，在控制台验证快捷键绑定
document.addEventListener('keydown', function(e) {
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') e.preventDefault();
});
// 按 ↑/↓ 切换卡片 → 卡片高亮移动
// 按 Enter → 当前卡片变为已通过
// 按 Esc → 当前卡片变为已拒绝
```

---

## 十、架构反思

### 10.1 位置偏移的系统性缺陷

当前流水线架构中，`review_items.PositionStart` 是相对于**创建时刻**的文件内容的字节偏移。但由于每个步骤都会修改文件并更新 `record.FilePath`，步骤之间缺少位置同步机制。这是设计层面的缺陷。

**当前缓解方案：** `readAvailableContents` 读取所有中间版本 + `findMatchLine` 多策略降级匹配。

**长期解决方案建议：**
1. 在 `review_items` 表中增加 `source_step` 字段，记录创建步骤
2. 按步骤读取对应的中间文件进行上下文定位（不再盲目尝试所有版本）
3. 或使用更鲁棒的标识方式（如全文搜索+位置校验取代纯字节位置）

### 10.2 文件路径管理的分散化

`record.FilePath` 被多个地方修改：
- `createNewFileRecord` → 上传时设置
- `saveIntermediateFile` → 每步骤更新
- `FinalizeFile` handler → 最终文件

路径更新与状态更新不同步，是位置偏移的根因。建议用一个统一的状态机管理 `FilePath`、`Status`、`CurrentStep` 的原子更新。

### 10.3 前端双轨架构

`main.js`（遗留全局函数）与 `modules/*.js`（新模块化架构）并存。HTML 的 `onclick=` 属性引用全局函数，但回调逻辑通过 `app.js` 映射到模块函数。两者各自持有 `currentFileMd5` 副本，需要手动同步。

**建议：** 逐步消灭 `main.js` 中的函数定义，将所有 `onclick="fn()"` 替换为 `addEventListener` 绑定，最终移除 `main.js`。

---

## 十一、关键发现

1. **中间文件目录丢失是系统性缺陷**：`os.MkdirAll` 应在应用启动时统一创建所有子目录，而非依赖各步骤各自创建
2. **位置偏移是流水线架构的核心问题**：一旦文件被修改（去重、替换等），所有后续步骤的字节位置都失效
3. **多版本回退 + 多策略匹配是有效的缓解方案**：`readAvailableContents` + `findMatchLine` 的 4 层降级策略能覆盖绝大多数场景
4. **Content-Disposition 中文编码是 Go/Gin 的已知坑**：`c.FileAttachment` 不处理非 ASCII 文件名，需要手动 RFC 5987
5. **pointer-events 继承链坑**：父元素的 `pointer-events: none` 会影响绝对定位的子元素，需要子元素显式声明 `pointer-events: auto`

---

**记录时间**: 2026-04-30
**记录人**: opencode
**问题状态**: ✅ 主要问题已修复并验证（编译通过 `go build` + `go vet`）
**验证状态**: ✅ `go build` 编译通过 | ✅ `go vet` 无警告 | 待浏览器端全面回归
**影响范围**:
- 前端: 所有页面（文件列表、上传、规则配置、处理进度、审核页、完成页）
- 后端: `GetReviewItems`、`DownloadFile`、`saveIntermediateFile`
- 数据库: 无 schema 变更
**未解决问题**:
- 存量文件的中间文件可能不存在（已在 `saveIntermediateFile` 修复，需重新处理）
- `main.js` 与 `modules/*.js` 的双轨并存问题（见 10.3 节）
- 审核项位置偏移的系统性缺陷（见 10.1 节）
