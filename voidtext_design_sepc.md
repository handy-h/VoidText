# 湮文 VoidText — UI设计规范

> 归于寂灭，方见真章
>
> 本规范为湮文项目的前端设计标准，涵盖视觉风格、组件系统、动效原则和文案风格。
> 所有后续优化和更新必须遵循本规范。

---

## 一、设计原则

### 1.1 核心哲学：寂灭

「寂灭」不是死亡，而是归于本真后的空灵与宁静。

| 特征 | 表现 |
|------|------|
| **静** | 克制使用动效，减少视觉噪音 |
| **空** | 合理留白，让界面有呼吸感 |
| **净** | 去除冗余装饰，信息层级分明 |
| **敛** | 弱化装饰性元素，强化功能性表达 |

### 1.2 设计四则

```
克制 · 留白 · 层次 · 一致
```

1. **克制** — 不炫技，不过度设计，每一处视觉元素都有存在理由
2. **留白** — 空间是界面的一部分，不是浪费
3. **层次** — 通过对比（大小、颜色、深浅）建立清晰的信息层级
4. **一致** — 相同场景相同呈现，减少用户认知负担

---

## 二、色彩系统

### 2.1 设计令牌

所有颜色必须通过 CSS 变量引用，禁止硬编码。

```css
:root {
  /* ==================== 虚空黑系 ==================== */
  /* 寂灭风格的「底色」，承载一切的虚空 */
  --void-void: #050508;      /* 最深，用于极端场景 */
  --void-black: #0a0a0f;     /* 主背景 */
  --void-deep: #12121a;      /* 卡片/容器背景 */
  --void-card: #1a1a24;      /* 元素背景 */
  --void-border: #2a2a3a;    /* 边框 */
  --void-subtle: #3a3a4a;    /* 次要边框/分隔线 */

  /* ==================== 寂灭青系 ==================== */
  /* 代表「生机」的颜色，在虚空中点亮希望 */
  --annihilation-dim: #00a884;    /* 暗调（次要状态） */
  --annihilation-cyan: #00d4aa;   /* 主色调（品牌色） */
  --annihilation-glow: #00f5c4;  /* 亮调（强调/悬停） */
  --annihilation-dark: #008f6b;  /* 深调（按下状态） */

  /* ==================== 灰烬紫系 ==================== */
  /* 代表「过渡」的颜色，介于虚空与生机之间 */
  --ash-dim: #4a3fb5;        /* 暗调 */
  --ash-purple: #6b5ce7;     /* 主色调 */
  --ash-glow: #8b7cf7;       /* 亮调 */

  /* ==================== 语义色 ==================== */
  --color-success: #00a884;
  --color-warning: #f59e0b;
  --color-error: #ff4757;
  --color-info: var(--annihilation-cyan);

  /* ==================== 文字色 ==================== */
  --text-primary: #e8e8f0;   /* 主要文字 */
  --text-secondary: #a0a0b0; /* 次要文字 */
  --text-muted: #606070;     /* 辅助/禁用文字 */

  /* ==================== 状态色 ==================== */
  /* 简洁的状态指示，仅用颜色表达 */
  --status-pending: #4a5568;
  --status-processing: var(--annihilation-cyan);
  --status-reviewing: var(--ash-purple);
  --status-completed: var(--color-success);
  --status-failed: var(--color-error);
}
```

### 2.2 色彩使用规则

| 场景 | 颜色 | 用途 |
|------|------|------|
| 主背景 | `--void-black` | 页面根背景 |
| 容器背景 | `--void-deep` | header、main、card 容器 |
| 元素背景 | `--void-card` | 按钮、输入框、文件卡片的背景 |
| 主按钮 | 渐变 `--annihilation-cyan` → `--ash-purple` | 主要操作 |
| 次要按钮 | `--void-card` + `--void-border` | 次要操作 |
| 状态指示 | `--status-*` 系列 | 状态标识 |
| 文字主色 | `--text-primary` | 标题、重要信息 |
| 文字次色 | `--text-secondary` | 正文、说明文字 |
| 文字辅助 | `--text-muted` | 辅助信息、时间戳 |

### 2.3 禁止事项

```css
/* ❌ 禁止：禁止硬编码颜色值 */
.element { color: #ffffff; background: #000; }

/* ✅ 必须：使用设计令牌 */
.element { color: var(--text-primary); background: var(--void-black); }

/* ❌ 禁止：禁止在寂灭风格中使用过于鲜艳的颜色 */
.element { color: #ff00ff; background: #00ff00; }

/* ❌ 禁止：发光效果滥用 */
.element { box-shadow: 0 0 30px 10px rgba(0, 212, 170, 0.5); }
```

---

## 三、字体系统

### 3.1 字体栈

```css
:root {
  /* 展示字体：用于标题、品牌文字 */
  --font-display: "Noto Serif SC", "Source Han Serif SC", "Source Han Serif CN", serif;

  /* 正文字体：用于正文、UI元素 */
  --font-body: "Inter", "Noto Sans SC", -apple-system, BlinkMacSystemFont, sans-serif;

  /* 等宽字体：用于代码、时间戳、元数据 */
  --font-mono: "JetBrains Mono", "Fira Code", "SF Mono", monospace;
}
```

### 3.2 字体比例

```css
:root {
  /* 基于 16px 基准的完美比例 */
  --text-xs: 0.75rem;     /* 12px - 辅助信息 */
  --text-sm: 0.875rem;    /* 14px - 次要文字 */
  --text-base: 1rem;       /* 16px - 正文 */
  --text-lg: 1.125rem;     /* 18px - 强调正文 */
  --text-xl: 1.25rem;      /* 20px - 小标题 */
  --text-2xl: 1.5rem;      /* 24px - 区块标题 */
  --text-3xl: 1.875rem;   /* 30px - 页面标题 */
  --text-4xl: 2.25rem;     /* 36px - 大标题（极少使用） */
}
```

### 3.3 字重系统

```css
:root {
  --font-normal: 400;    /* 正常 - 正文 */
  --font-medium: 500;   /* 中等 - 按钮、标签 */
  --font-semibold: 600; /* 半粗 - 副标题 */
  --font-bold: 700;     /* 加粗 - 标题、品牌 */
}
```

### 3.4 字体使用规则

| 元素 | 字体 | 字重 | 字号 | 用途 |
|------|------|------|------|------|
| Logo/品牌 | `--font-display` | 700 | 26px | 标题 |
| 页面区块标题 | `--font-display` | 600 | 20px | 章节标题 |
| 正文 | `--font-body` | 400 | 14px | 主要内容 |
| 按钮文字 | `--font-body` | 500 | 14px | 操作引导 |
| 元数据 | `--font-mono` | 400 | 13px | 时间、状态 |
| 错误信息 | `--font-mono` | 400 | 13px | 技术信息 |

### 3.5 行高规范

```css
:root {
  --leading-tight: 1.25;   /* 紧凑 - 标题 */
  --leading-normal: 1.5;    /* 正常 - 正文 */
  --leading-relaxed: 1.75; /* 宽松 - 长文本阅读 */
}
```

---

## 四、间距系统

### 4.1 基准间距

基于 **4px** 网格系统，确保所有间距都是 4 的倍数。

```css
:root {
  --space-0: 0;
  --space-1: 0.25rem;    /* 4px  - 元素内微调 */
  --space-2: 0.5rem;     /* 8px  - 小间距 */
  --space-3: 0.75rem;    /* 12px - 元素内 */
  --space-4: 1rem;       /* 16px - 标准间距 */
  --space-5: 1.25rem;    /* 20px - 卡片内 */
  --space-6: 1.5rem;     /* 24px - 区块内 */
  --space-8: 2rem;       /* 32px - 区块间 */
  --space-10: 2.5rem;    /* 40px - 大区块 */
  --space-12: 3rem;      /* 48px - 页面级 */
  --space-16: 4rem;      /* 64px - 区域分隔（慎用） */
}
```

### 4.2 组件间距规范

```css
/* ==================== 按钮 ==================== */
.btn {
  padding: 10px 16px;        /* 高度 40px，触摸友好 */
  border-radius: 8px;
  gap: var(--space-2);
}

/* 按钮组内间距 */
.btn-group {
  gap: var(--space-3);
  margin: var(--space-3) 0;
}

/* ==================== 卡片 ==================== */
.card {
  padding: var(--space-5);   /* 20px */
  border-radius: 10px;
  gap: var(--space-4);
}

.file-card {
  padding: var(--space-5) var(--space-6);  /* 20px 24px */
  margin-bottom: var(--space-3);            /* 12px */
}

/* ==================== 表单 ==================== */
.form-group {
  margin-bottom: var(--space-5);  /* 20px */
}

.form-group label {
  margin-bottom: var(--space-2);  /* 8px */
}

.form-input {
  padding: var(--space-3) var(--space-4);  /* 12px 16px */
}

/* ==================== 区块 ==================== */
.section {
  margin-bottom: var(--space-6);  /* 24px */
}

.section-header {
  margin-bottom: var(--space-5);  /* 20px */
}

/* ==================== 页面级 ==================== */
.page-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: var(--space-6);       /* 24px */
}

.page-header {
  padding: var(--space-6) var(--space-8); /* 24px 32px */
  margin-bottom: var(--space-6);  /* 24px */
}
```

---

## 五、组件规范

### 5.1 按钮系统

#### 5.1.1 按钮类型

```css
/* ==================== 主按钮 ==================== */
/* 用于最重要的操作，每个区块最多出现 1-2 次 */
.btn-primary {
  background: linear-gradient(135deg, var(--annihilation-cyan), var(--ash-purple));
  color: var(--void-void);
  font-weight: 600;
  border: none;
}

.btn-primary:hover {
  /* 寂灭风格：克制，改为微妙提升而非夸张效果 */
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 212, 170, 0.15);
}

/* ==================== 次要按钮 ==================== */
/* 用于次要操作，可多次出现 */
.btn-secondary {
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--void-border);
}

.btn-secondary:hover {
  color: var(--text-primary);
  border-color: var(--annihilation-dim);
}

/* ==================== 危险按钮 ==================== */
/* 用于删除等危险操作 */
.btn-danger {
  background: transparent;
  color: var(--color-error);
  border: 1px solid var(--color-error);
}

.btn-danger:hover {
  background: var(--color-error);
  color: var(--void-void);
}

/* ==================== 文字按钮 ==================== */
/* 用于辅助操作，视觉权重最低 */
.btn-text {
  background: transparent;
  color: var(--text-secondary);
  border: none;
  padding: var(--space-2) var(--space-3);
}

.btn-text:hover {
  color: var(--text-primary);
}
```

#### 5.1.2 按钮状态

```css
/* ==================== 悬停 ==================== */
.btn:hover {
  cursor: pointer;
}

/* ==================== 按下 ==================== */
.btn:active {
  transform: translateY(0);
  opacity: 0.9;
}

/* ==================== 禁用 ==================== */
.btn:disabled,
.btn--disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

/* ==================== 加载中 ==================== */
.btn--loading {
  position: relative;
  color: transparent;
  pointer-events: none;
}

.btn--loading::after {
  content: "";
  position: absolute;
  width: 16px;
  height: 16px;
  top: 50%;
  left: 50%;
  margin: -8px 0 0 -8px;
  border: 2px solid transparent;
  border-top-color: currentColor;
  border-radius: 50%;
  animation: btn-spin 0.8s linear infinite;
}

@keyframes btn-spin {
  to { transform: rotate(360deg); }
}
```

#### 5.1.3 按钮尺寸

```css
.btn--sm {
  padding: 6px 12px;
  font-size: var(--text-xs);
}

.btn--lg {
  padding: 14px 24px;
  font-size: var(--text-lg);
}
```

### 5.2 表单元素

#### 5.2.1 输入框

```css
.form-input {
  display: block;
  width: 100%;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--void-border);
  border-radius: 8px;
  background: var(--void-card);
  color: var(--text-primary);
  font-size: var(--text-base);
  font-family: var(--font-body);
  transition: border-color var(--duration-fast) var(--ease-annihilation),
              box-shadow var(--duration-fast) var(--ease-annihilation);
}

.form-input::placeholder {
  color: var(--text-muted);
}

.form-input:focus {
  outline: none;
  border-color: var(--annihilation-cyan);
  box-shadow: 0 0 0 3px rgba(0, 212, 170, 0.1);
}

.form-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.form-input--error {
  border-color: var(--color-error);
}
```

#### 5.2.2 文本域

```css
.form-textarea {
  min-height: 100px;
  resize: vertical;
  font-family: var(--font-mono);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}
```

#### 5.2.3 复选框

```css
/* 使用 accent-color 作为主要方案 */
.form-checkbox {
  width: 18px;
  height: 18px;
  accent-color: var(--annihilation-cyan);
  cursor: pointer;
}

/* 如需更精细控制，使用自定义样式 */
.form-checkbox-wrapper {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  cursor: pointer;
}

.form-checkbox-wrapper input[type="checkbox"] {
  appearance: none;
  width: 18px;
  height: 18px;
  border: 1px solid var(--void-border);
  border-radius: 4px;
  background: var(--void-card);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-annihilation);
}

.form-checkbox-wrapper input[type="checkbox"]:checked {
  background: var(--annihilation-cyan);
  border-color: var(--annihilation-cyan);
}

.form-checkbox-wrapper input[type="checkbox"]:checked::after {
  content: "✓";
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--void-void);
  font-size: 12px;
  font-weight: bold;
}
```

### 5.3 卡片组件

```css
.card {
  background: var(--void-card);
  border: 1px solid var(--void-border);
  border-radius: 10px;
  transition: transform var(--duration-normal) var(--ease-annihilation),
              box-shadow var(--duration-normal) var(--ease-annihilation);
}

.card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
}

/* ==================== 文件卡片 ==================== */
.file-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-5) var(--space-6);
  margin-bottom: var(--space-3);
  position: relative;
  overflow: hidden;
}

/* 状态指示条 - 左侧 4px 色条 */
.file-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  width: 4px;
  height: 100%;
  background: var(--status-pending);
}

.file-card.status-processing::before {
  background: var(--status-processing);
  box-shadow: 0 0 8px var(--status-processing);
}

.file-card.status-reviewing::before {
  background: var(--status-reviewing);
  box-shadow: 0 0 8px var(--status-reviewing);
}

.file-card.status-completed::before {
  background: var(--status-completed);
}

.file-card.status-failed::before {
  background: var(--status-failed);
  box-shadow: 0 0 8px var(--status-failed);
}

/* 文件信息 */
.file-info {
  flex: 1;
  min-width: 0;
  padding-left: var(--space-4);
}

.file-title {
  font-size: var(--text-base);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: var(--space-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
  font-size: var(--text-sm);
  font-family: var(--font-mono);
  color: var(--text-muted);
}

.file-meta span {
  display: inline-flex;
  align-items: center;
}

.file-meta strong {
  color: var(--text-secondary);
  margin-right: var(--space-1);
}

/* 操作按钮组 */
.file-actions {
  display: flex;
  gap: var(--space-2);
  flex-shrink: 0;
  margin-left: var(--space-5);
}
```

### 5.4 进度指示

```css
/* ==================== 进度条容器 ==================== */
.progress-container {
  width: 100%;
  max-width: 500px;
  height: 24px;
  background: var(--void-card);
  border: 1px solid var(--void-border);
  border-radius: 12px;
  overflow: hidden;
  position: relative;
}

.progress-bar {
  height: 100%;
  background: linear-gradient(90deg, var(--annihilation-cyan), var(--ash-purple));
  border-radius: 12px;
  transition: width var(--duration-slow) var(--ease-annihilation);
}

.progress-text {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--text-primary);
  font-family: var(--font-mono);
}

/* ==================== 步骤指示器 ==================== */
.steps-progress {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  flex-wrap: wrap;
}

.step-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  opacity: 0.4;
  transition: opacity var(--duration-normal) var(--ease-annihilation);
}

.step-item.active {
  opacity: 1;
}

.step-item.completed {
  opacity: 0.8;
}

.step-icon {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--text-sm);
  font-weight: 700;
  background: var(--void-card);
  border: 2px solid var(--void-border);
  color: var(--text-muted);
  transition: all var(--duration-normal) var(--ease-annihilation);
}

.step-item.active .step-icon {
  background: var(--annihilation-cyan);
  border-color: var(--annihilation-cyan);
  color: var(--void-void);
  animation: step-pulse 2s ease-in-out infinite;
}

.step-item.completed .step-icon {
  background: var(--status-completed);
  border-color: var(--status-completed);
  color: var(--void-void);
}

@keyframes step-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(0, 212, 170, 0.3); }
  50% { box-shadow: 0 0 0 6px rgba(0, 212, 170, 0); }
}

.step-connector {
  width: 40px;
  height: 2px;
  background: var(--void-border);
  margin-bottom: 24px;
  flex-shrink: 0;
}
```

### 5.5 空状态

```css
/* ==================== 空状态 ==================== */
/* 寂灭风格：简洁、有意境 */
.empty-state {
  text-align: center;
  padding: var(--space-16) var(--space-4);
  color: var(--text-muted);
  font-size: var(--text-base);
  background: var(--void-card);
  border: 1px solid var(--void-border);
  border-radius: 8px;
  margin: var(--space-6) 0;
}

.empty-state__icon {
  font-size: 48px;
  margin-bottom: var(--space-4);
  opacity: 0.5;
}

.empty-state__title {
  font-size: var(--text-lg);
  font-weight: 500;
  color: var(--text-secondary);
  margin-bottom: var(--space-2);
}

.empty-state__description {
  font-size: var(--text-sm);
  color: var(--text-muted);
  max-width: 300px;
  margin: 0 auto;
}

/* 寂灭风格空状态文案示例 */
.empty-state--files {
  /* 「虚空之中，尚无涟漪」 */
}

.empty-state--search {
  /* 「此中无物，空余寂寥」 */
}
```

---

## 六、动效规范

### 6.1 设计原则

寂灭风格的动效应遵循：**静、缓、隐**。

| 原则 | 含义 | 示例 |
|------|------|------|
| 静 | 动效应安静，不抢焦点 | hover 时仅微微提升 |
| 缓 | 动效应舒缓，不急促 | 使用 ease-out，不使用 linear |
| 隐 | 动效应隐蔽，不张扬 | 取消闪烁、弹跳等夸张效果 |

### 6.2 动效令牌

```css
:root {
  /* 缓动函数 */
  --ease-annihilation: cubic-bezier(0.16, 1, 0.3, 1);
  --ease-out: cubic-bezier(0, 0, 0.2, 1);
  --ease-in: cubic-bezier(0.4, 0, 1, 1);

  /* 时长 */
  --duration-instant: 100ms;   /* 微交互反馈 */
  --duration-fast: 150ms;      /* 快速状态变化 */
  --duration-normal: 300ms;    /* 标准过渡 */
  --duration-slow: 500ms;      /* 大型动画 */
}
```

### 6.3 动效使用场景

| 场景 | 效果 | 时长 |
|------|------|------|
| 按钮悬停 | transform: translateY(-1px) | 150ms |
| 卡片悬停 | transform: translateY(-2px) + shadow | 300ms |
| 模态出现 | opacity 0→1 + scale 0.95→1 | 300ms |
| Toast提示 | opacity 0→1 + translateY | 300ms |
| 进度条 | width 变化 | 500ms |
| 步骤脉冲 | box-shadow 呼吸 | 2s |

### 6.4 禁用效果

```css
/* ❌ 禁止：以下效果在寂灭风格中禁用 */
.no-animation {
  animation: none !important;
  transition: none !important;
}

/* 禁用场景：用户设置「减少动画」 */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 七、布局规范

### 7.1 页面结构

```css
/* ==================== 基础布局 ==================== */
.page {
  max-width: 1200px;
  margin: 0 auto;
  padding: var(--space-6);
}

.page-header {
  background: linear-gradient(135deg, var(--void-deep), var(--void-card));
  border: 1px solid var(--void-border);
  border-radius: 12px;
  padding: var(--space-6) var(--space-8);
  margin-bottom: var(--space-6);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.page-header__brand {
  /* 品牌区域 */
}

.page-header__nav {
  display: flex;
  gap: var(--space-3);
}

.page-main {
  background: var(--void-deep);
  border: 1px solid var(--void-border);
  padding: var(--space-6);
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.2);
}

/* ==================== 区块结构 ==================== */
.section {
  margin-bottom: var(--space-6);
}

.section:last-child {
  margin-bottom: 0;
}

.section__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-5);
}

.section__title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
  font-weight: 600;
  color: var(--text-primary);
  padding-bottom: var(--space-3);
  position: relative;
}

.section__title::after {
  content: "";
  position: absolute;
  bottom: 0;
  left: 0;
  width: 60px;
  height: 2px;
  background: linear-gradient(90deg, var(--annihilation-cyan), var(--ash-purple));
}
```

### 7.2 响应式断点

```css
/* ==================== 移动优先 ==================== */

/* 小型设备（640px 及以上）平板竖屏 */
@media (min-width: 640px) {
  .container { max-width: 640px; }
}

/* 中型设备（768px 及以上）平板横屏 */
@media (min-width: 768px) {
  .container { max-width: 768px; }
}

/* 大型设备（1024px 及以上）笔记本 */
@media (min-width: 1024px) {
  .container { max-width: 1024px; }
  .page { padding: var(--space-8); }
}

/* 超大型设备（1280px 及以上）桌面 */
@media (min-width: 1280px) {
  .container { max-width: 1200px; }
}

/* ==================== 移动端适配 ==================== */
@media (max-width: 767px) {
  .page-header {
    flex-direction: column;
    gap: var(--space-4);
    text-align: center;
  }

  .page-header__nav {
    justify-content: center;
  }

  .file-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .file-actions {
    margin-left: 0;
    margin-top: var(--space-3);
    width: 100%;
    justify-content: flex-end;
  }

  .section__header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-3);
  }

  .steps-progress {
    flex-direction: column;
    gap: var(--space-3);
  }

  .step-connector {
    width: 2px;
    height: 20px;
    margin: 0;
  }
}
```

---

## 八、文案风格规范

### 8.1 寂灭风格文案原则

| 原则 | 说明 | 示例 |
|------|------|------|
| **简洁** | 去除冗余，保留核心 | 「上传失败」而非「哎呀，上传好像出了问题呢」 |
| **直接** | 避免套话，直奔主题 | 「处理中」而非「正在为您努力处理中」 |
| **克制** | 情绪适度，不过度 | 「成功」而非「太棒了！您已成功完成！」 |
| **意境** | 寂灭场景可用留白暗示 | 空状态可用「虚空之中，尚无涟漪」 |

### 8.2 按钮文案

| 场景 | 推荐文案 | 禁用文案 |
|------|----------|----------|
| 提交 | 确认、提交、保存 | 立即提交、马上开始 |
| 取消 | 取消、返回 | 算了、算了吧 |
| 删除 | 删除 | 彻底删除、永久删除 |
| 下载 | 下载 | 立即下载、快速下载 |
| 上传 | 上传文件 | 点击上传、拖拽上传 |

### 8.3 提示文案

```javascript
// ==================== 反馈提示 ====================
// 成功
const messages = {
  uploadSuccess: "文件上传成功",
  saveSuccess: "保存成功",
  deleteSuccess: "已删除",
  loginSuccess: "登录成功",
  logoutSuccess: "已退出登录"
};

// 错误
const messages = {
  uploadFailed: "上传失败",
  saveFailed: "保存失败",
  deleteFailed: "删除失败",
  loginFailed: "认证失败",
  networkError: "网络连接异常",
  serverError: "服务器错误"
};

// 处理中
const messages = {
  processing: "处理中",
  uploading: "上传中",
  downloading: "下载中",
  saving: "保存中"
};
```

### 8.4 空状态文案

| 场景 | 文案 |
|------|------|
| 无文件 | 虚空之中，尚无涟漪 |
| 无搜索结果 | 此中无物，空余寂寥 |
| 无审核项 | 万象皆净，无需审核 |
| 无历史记录 | 往事如烟，未留痕迹 |

### 8.5 禁止文案模式

```javascript
// ❌ 禁止：过度感叹
"哎呀！出错了！"
"太棒了！您成功了！"
"正在拼命处理中..."

// ✅ 正确：简洁克制
"操作失败"
"已完成"
"处理中"

// ❌ 禁止：冗长说明
"由于网络原因，您的文件可能无法成功上传，请检查网络后重试"

// ✅ 正确：简洁直接
"上传失败，请检查网络"
```

---

## 九、代码规范

### 9.1 CSS 规范

```css
/* ==================== 命名规范 ==================== */
/* 使用 BEM 命名法 */
/* Block__Element--Modifier */

/* 示例 */
.nav {}
.nav__item {}
.nav__item--active {}

.btn {}
.btn--primary {}
.btn--secondary {}
.btn__icon {}

/* ==================== 样式顺序 ==================== */
/* 1. 布局 */
display
position
top / right / bottom / left
float
clear
overflow

/* 2. 盒子模型 */
width
height
margin
padding
border
box-sizing

/* 3. 视觉 */
background
color
font
line-height
text-align

/* 4. 动效 */
transition
animation
transform

/* ==================== 注释规范 ==================== */
/* ============================================
   区块标题
   描述信息
   ============================================ */

/* -------------------
   子区块
   ------------------- */

/* 注释说明 */
.selector {}
```

### 9.2 JavaScript 规范

```javascript
// ==================== 常量定义 ====================
// 状态映射
const STATUS_MAP = {
  pending: { text: "待处理", class: "status-pending" },
  processing: { text: "处理中", class: "status-processing" },
  reviewing: { text: "审核中", class: "status-reviewing" },
  completed: { text: "已完成", class: "status-completed" },
  failed: { text: "失败", class: "status-failed" }
};

// 步骤映射
const STEP_MAP = {
  cleaning: "基础清洗",
  indexing: "向量检测",
  llm_fix: "LLM修复",
  review: "人工审核",
  finalizing: "生成文件"
};

// ==================== 工具函数 ====================
// 转义 HTML
function escapeHtml(text) {
  if (!text) return "";
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

// 格式化时间
function formatTime(timestamp) {
  if (!timestamp) return "";
  try {
    return new Date(timestamp).toLocaleString("zh-CN");
  } catch {
    return timestamp;
  }
}

// 格式化文件大小
function formatFileSize(bytes) {
  const units = ["B", "KB", "MB", "GB"];
  let unitIndex = 0;
  let size = bytes;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  return `${size.toFixed(1)}${units[unitIndex]}`;
}

// ==================== DOM 操作规范 ====================
// 使用语义化选择器名称
const elements = {
  feedback: document.getElementById("feedback"),
  fileList: document.getElementById("file-list-container"),
  uploadArea: document.getElementById("upload-area"),
  uploadResult: document.getElementById("upload-result")
};

// 统一的反馈显示
function showFeedback(message, type = "success") {
  const fb = elements.feedback;
  fb.textContent = message;
  fb.className = `feedback ${type}`;
  fb.style.display = "block";
  setTimeout(() => {
    fb.style.display = "none";
  }, 3000);
}

// ==================== 事件处理规范 ====================
// 事件处理函数命名：handle + 动作
function handleFileUpload(event) { }
function handleLogin() { }
function handleLogout() { }

// 初始化
function initApp() {
  checkAuthStatus();
  showSection("file-list");
  setupEventListeners();
}

document.addEventListener("DOMContentLoaded", initApp);
```

---

## 十、可访问性规范

### 10.1 色彩对比

| 类型 | 比例要求 | 用途 |
|------|----------|------|
| 正常文本 | ≥ 4.5:1 | 正文、标签 |
| 大文本 (≥18px) | ≥ 3:1 | 标题 |
| UI 组件 | ≥ 3:1 | 按钮、输入框边框 |

### 10.2 焦点管理

```css
/* 焦点样式 - 寂灭风格的克制实现 */
:focus-visible {
  outline: 2px solid var(--annihilation-cyan);
  outline-offset: 2px;
}

/* 确保焦点的视觉可感知 */
button:focus-visible,
input:focus-visible {
  border-color: var(--annihilation-cyan);
  box-shadow: 0 0 0 3px rgba(0, 212, 170, 0.1);
}
```

### 10.3 触摸目标

```css
/* 交互元素最小尺寸 44x44px */
button,
input[type="checkbox"],
input[type="radio"] {
  min-width: 44px;
  min-height: 44px;
}
```

---

## 附录：设计令牌速查表

```css
/* ==================== 完整令牌清单 ==================== */

/* 颜色 */
--void-void: #050508;
--void-black: #0a0a0f;
--void-deep: #12121a;
--void-card: #1a1a24;
--void-border: #2a2a3a;
--annihilation-cyan: #00d4aa;
--annihilation-dim: #00a884;
--annihilation-glow: #00f5c4;
--ash-purple: #6b5ce7;
--ash-glow: #8b7cf7;
--text-primary: #e8e8f0;
--text-secondary: #a0a0b0;
--text-muted: #606070;

/* 字体 */
--font-display: "Noto Serif SC", serif;
--font-body: "Inter", "Noto Sans SC", sans-serif;
--font-mono: "JetBrains Mono", monospace;
--text-xs: 0.75rem;
--text-sm: 0.875rem;
--text-base: 1rem;
--text-lg: 1.125rem;
--text-xl: 1.25rem;
--text-2xl: 1.5rem;
--text-3xl: 1.875rem;

/* 间距 */
--space-1: 0.25rem;  /* 4px */
--space-2: 0.5rem;   /* 8px */
--space-3: 0.75rem;  /* 12px */
--space-4: 1rem;     /* 16px */
--space-5: 1.25rem;  /* 20px */
--space-6: 1.5rem;   /* 24px */
--space-8: 2rem;     /* 32px */

/* 动效 */
--ease-annihilation: cubic-bezier(0.16, 1, 0.3, 1);
--duration-fast: 150ms;
--duration-normal: 300ms;
--duration-slow: 500ms;

/* 圆角 */
--radius-sm: 6px;
--radius-md: 8px;
--radius-lg: 12px;
--radius-full: 9999px;
```

---

> **版本**: v1.0
>
> **更新日期**: 2026-04-26
>
> **维护**: 遵循本规范，确保前端系统的一致性和可维护性
