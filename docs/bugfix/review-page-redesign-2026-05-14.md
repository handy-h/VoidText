# 审核页面重构 - 问题记录

**日期**: 2026-05-14 ~ 2026-05-15
**任务**: 将审核页面从双栏布局改为全屏沉浸式逐字符对齐 Diff 视图

---

## 问题 1: 没有展示全文

### 问题描述
- 从第 15 行开始显示，没有展示全文的所有行
- 只渲染了审核项所在的行，没有渲染全文的其他行

### 原因分析
- 初始实现只加载了审核项数据 (`/api/files/{md5}/review-items`)
- 没有加载文件全文内容

### 解决方案
- 并行加载审核项和全文内容
- 使用 `/api/files/{md5}/content` API 获取全文
- 按行号分页显示所有行，审核项所在行以 diff 方式高亮

```javascript
Promise.all([
  AppConfig.apiRequest('/files/' + currentFileMd5 + '/review-items'),
  AppConfig.apiRequest('/files/' + currentFileMd5 + '/content')
])
```

---

## 问题 2: 文本顺序混乱

### 问题描述
- 同一行里出现了多个区块，文本排列不按顺序

### 原因分析
- 多个审核项在同一行时，没有按 position 排序

### 解决方案
- 按行号 + position 排序审核项

```javascript
filteredItems.sort(function(a, b) {
  var lineA = a.lineNum || 0;
  var lineB = b.lineNum || 0;
  if (lineA !== lineB) return lineA - lineB;
  return (a.position || 0) - (b.position || 0);
});
```

---

## 问题 3: 同一行多处审核项换行混乱

### 问题描述
- 同一行文本里有多处要审核时，原文和建议混在一起显示

### 原因分析
- 把原文和建议都放在同一行里显示，导致混乱

### 解决方案
- **原文行** - 单独一行显示，标记需要修改的部分（红色高亮）
- **建议行** - 紧跟原文行下方显示，绿色高亮建议文本

---

## 问题 4: 红绿底色丢失

### 问题描述
- 原文行和建议行的背景色没有正确显示

### 原因分析
- CSS 类名不匹配，`.diff-original` 和 `.diff-suggested` 样式未正确定义

### 解决方案
- 使用 `.line-original` 和 `.line-suggested` 类名

```css
.line-original {
  background: rgba(255, 107, 107, 0.15);
  border-left: 3px solid rgba(255, 107, 107, 0.6);
}

.line-suggested {
  background: rgba(80, 200, 120, 0.15);
  border-left: 3px solid rgba(80, 200, 120, 0.6);
}
```

---

## 问题 5: 浮动操作框未浮动

### 问题描述
- 编辑工具栏没有实现浮动效果，文字和样式是纵向排列

### 原因分析
- 使用了 `flex-direction: column` 导致纵向排列

### 解决方案
- 使用 `position: absolute` 定位到原文行右侧
- 使用 `flex-direction: row` 实现横向排列
- 只在 hover/active 原文行时显示

```css
.floating-actions {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  flex-direction: row;
  gap: 8px;
}

.line-original:hover .floating-actions,
.line-original.active .floating-actions {
  opacity: 1;
}
```

---

## 问题 6: 字符不对齐

### 问题描述
- 原文行和建议行的字符位置没有对齐，建议文本比原文短时后面文本错位

### 原因分析
- 没有实现逐字符对齐算法，建议文本直接替换原文

### 解决方案
- 实现 `renderAlignedDiff()` 逐字符对齐函数
- 双向空格填充：哪边短就在哪边填充空格
- 空格只用于展示，实际保存时不包含

```javascript
function renderAlignedDiff(lineText, item) {
  var origText = item.original;
  var suggText = item.suggested;
  var pos = lineText.indexOf(origText);
  if (pos < 0) return { originalHtml: escapeHtml(lineText), suggestedHtml: escapeHtml(lineText) };

  var prefix = lineText.substring(0, pos);
  var origSuffix = lineText.substring(pos + origText.length);
  var diff = origText.length - suggText.length;

  var origDisplay = origText;
  var suggDisplay = suggText;
  if (diff > 0) suggDisplay = suggText + ' '.repeat(diff);
  else if (diff < 0) origDisplay = origText + ' '.repeat(-diff);

  return {
    originalHtml: escAndVisualize(prefix) + '<del class="diff-del">' + escAndVisualize(origDisplay) + '</del>' + escAndVisualize(origSuffix),
    suggestedHtml: escAndVisualize(prefix) + '<ins class="diff-ins">' + escAndVisualize(suggDisplay) + '</ins>' + escAndVisualize(origSuffix)
  };
}
```

---

## 问题 7: position 使用错误导致原文对不上

### 问题描述
- 审核页面显示的原文行内容与实际文件不匹配
- 例如：审核项显示第 57 行 `"就是那种女人。"`，但实际文件中该文本在第 55 行的中间位置

### 原因分析
- `renderAlignedDiff` 函数使用 `item.position`（来自后端的字节偏移量）直接截取行内容
- 但 `position` 是相对于整个文件的字节偏移量，不是行内偏移量
- 导致截取的前后缀错误，显示的文本与实际不符

### 解决方案
- 改用 `lineText.indexOf(origText)` 在当前行中查找原文的实际位置
- 不再依赖 `item.position` 进行行内截取

```javascript
var pos = lineText.indexOf(origText);
if (pos < 0) {
  return { originalHtml: escAndVisualize(lineText), suggestedHtml: escAndVisualize(lineText) };
}
var prefix = lineText.substring(0, pos);
var origSuffix = lineText.substring(pos + origText.length);
```

---

## 问题 8: DiffUtils.renderAlignedDiff is not a function

### 问题描述
- 前端控制台报错：`TypeError: DiffUtils.renderAlignedDiff is not a function`
- 审核页面无法加载

### 原因分析
- `diff-utils.js` 中新增的 `renderAlignedDiff` 函数已正确导出
- 但浏览器缓存了旧版本的 JS 文件

### 解决方案
- 硬刷新页面（Ctrl+Shift+R）清除缓存
- 在 HTML 中为 script 标签添加版本号参数（`?v=5`）防止缓存

---

## 问题 9: LLM 修复项 Position 偏移量错误（后端）

### 问题描述
- 后端返回的审核项 `position_start` 值不正确（如为 0）
- 导致前端无法正确定位修改位置，行号和行内容都不对

### 原因分析
- LLM 修复步骤采用分块（chunk）处理
- 每个 chunk 内部的 `detectChanges` 函数计算的 Position 是相对于 chunk 起始位置的
- 合并 chunk 结果时，没有将 chunk 的偏移量（`StartIndex`）加到 Position 上
- 代码位置：`internal/processor/model_repairer.go` 第 157 行

### 解决方案
- 在合并 chunk 结果时，将 `chunkResult.ChunkID`（即 chunk 的 `StartIndex`）加到每个 Change 的 Position 上

```go
// 修复前
result.Changes = append(result.Changes, chunkResult.Changes...)

// 修复后
for _, change := range chunkResult.Changes {
    adjustedChange := change
    adjustedChange.Position = change.Position + chunkResult.ChunkID
    result.Changes = append(result.Changes, adjustedChange)
}
```

**注意**：此修复需要重新处理文件才能使已有的审核项位置正确。

---

## 问题 10: 浮动工具栏遮挡文本内容

### 问题描述
- 浮动操作框（保留/撤销按钮）遮挡了建议行的文本内容
- 用户无法看到完整的建议文本

### 原因分析
- 浮动操作框使用 `position: absolute` 定位在行下方或行右侧
- 当文本较长时，工具栏会覆盖在文本上方

### 解决方案
- 将工具栏定位在**原文行的右侧**（`right: 8px; top: 50%; transform: translateY(-50%)`）
- 只在 hover 或 active 原文行时显示（`.line-original:hover/active .floating-actions`）
- 工具栏不覆盖建议行，因为建议行是独立的 DOM 元素

---

## 问题 11: 底色分层不明显

### 问题描述
- 原文行和建议行的修改部分高亮不够明显
- 无法清晰区分"整行底色"和"修改部分底色"

### 原因分析
- `del`/`ins` 元素的背景色透明度不够（0.5）
- 内边距太小（`padding: 0 2px`），视觉上不够突出

### 解决方案
- 加深 `del`/`ins` 的背景色透明度（0.5 → 0.6）
- 增加内边距（`padding: 2px 4px`）
- 增加圆角（`border-radius: 3px`）

```css
.line-original del.diff-del {
  background: rgba(255, 80, 80, 0.6);
  color: #8b0000;
  font-weight: 600;
  text-decoration: line-through;
  border-radius: 3px;
  padding: 2px 4px;
}

.line-suggested ins.diff-ins {
  background: rgba(60, 180, 80, 0.6);
  color: #006400;
  font-weight: 600;
  text-decoration: none;
  border-radius: 3px;
  padding: 2px 4px;
}
```

---

## 涉及文件

| 文件 | 职责 |
|------|------|
| `web/frontend/index.html` | HTML 结构 |
| `web/frontend/static/css/style.css` | CSS 样式 |
| `web/frontend/static/js/diff-utils.js` | Diff 算法（含逐字符对齐） |
| `web/frontend/static/js/modules/review.js` | 审核模块逻辑 |
| `internal/processor/model_repairer.go` | Position 偏移量修复（后端） |
