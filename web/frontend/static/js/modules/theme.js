// =============================================
// ThemeModule - 主题管理模块
// 内置主题切换、自定义主题创建/应用/删除
// =============================================

const ThemeModule = (function() {
  const STORAGE_KEY = 'voidtext-theme';
  const CUSTOM_LIST_KEY = 'voidtext-custom-theme-list';
  const CUSTOM_PREFIX = 'voidtext-custom-theme-';

  // 内置主题定义
  const BUILTIN_THEMES = {
    light: {
      name: '亮色护眼',
      dotColor: '#f1f5f9',
      isDefault: true,
      vars: {
        '--void-void': '#f1f5f9',
        '--void-black': '#f8fafc',
        '--void-deep': '#f1f5f9',
        '--void-card': '#ffffff',
        '--void-border': '#e2e8f0',
        '--void-subtle': '#cbd5e1',
        '--annihilation-dim': '#0f766e',
        '--annihilation-cyan': '#14b8a6',
        '--annihilation-glow': '#2dd4bf',
        '--annihilation-dark': '#115e59',
        '--ash-dim': '#6366f1',
        '--ash-purple': '#818cf8',
        '--ash-glow': '#a5b4fc',
        '--status-pending': '#cbd5e1',
        '--status-processing': '#14b8a6',
        '--status-reviewing': '#818cf8',
        '--status-completed': '#10b981',
        '--status-failed': '#f87171',
        '--text-primary': '#334155',
        '--text-secondary': '#64748b',
        '--text-muted': '#8898a8',
        '--glow-cyan': '0 0 20px rgba(20, 184, 166, 0.15)',
        '--glow-purple': '0 0 30px rgba(129, 140, 248, 0.12)'
      }
    },
    dark: {
      name: '寂灭黑',
      dotColor: '#0a0a0f',
      vars: {
        '--void-void': '#050508',
        '--void-black': '#0a0a0f',
        '--void-deep': '#12121a',
        '--void-card': '#1a1a24',
        '--void-border': '#2a2a3a',
        '--void-subtle': '#3a3a4a',
        '--annihilation-dim': '#00a884',
        '--annihilation-cyan': '#00d4aa',
        '--annihilation-glow': '#00f5c4',
        '--annihilation-dark': '#008f6b',
        '--ash-dim': '#4a3fb5',
        '--ash-purple': '#6b5ce7',
        '--ash-glow': '#8b7cf7',
        '--status-pending': '#4a5568',
        '--status-processing': '#00d4aa',
        '--status-reviewing': '#6b5ce7',
        '--status-completed': '#00a884',
        '--status-failed': '#ff4757',
        '--text-primary': '#e8e8f0',
        '--text-secondary': '#a0a0b0',
        '--text-muted': '#606070',
        '--glow-cyan': '0 0 20px rgba(0, 212, 170, 0.3)',
        '--glow-purple': '0 0 30px rgba(107, 92, 231, 0.2)'
      }
    },
    paper: {
      name: '羊皮纸',
      dotColor: '#f5f0e8',
      vars: {
        '--void-void': '#e8e0d0',
        '--void-black': '#f5f0e8',
        '--void-deep': '#e8e0d0',
        '--void-card': '#ffffff',
        '--void-border': '#d0c8b8',
        '--void-subtle': '#c8c0b0',
        '--annihilation-dim': '#6a5010',
        '--annihilation-cyan': '#8b6914',
        '--annihilation-glow': '#a07818',
        '--annihilation-dark': '#5a4208',
        '--ash-dim': '#5a4090',
        '--ash-purple': '#7a5cb5',
        '--ash-glow': '#9a7cd5',
        '--status-pending': '#7a7060',
        '--status-processing': '#8b6914',
        '--status-reviewing': '#7a5cb5',
        '--status-completed': '#5a7a18',
        '--status-failed': '#c83030',
        '--text-primary': '#3a3028',
        '--text-secondary': '#6a6050',
        '--text-muted': '#9a9080',
        '--glow-cyan': '0 0 20px rgba(139, 105, 20, 0.2)',
        '--glow-purple': '0 0 30px rgba(122, 92, 181, 0.15)'
      }
    },
    green: {
      name: '绿豆沙',
      dotColor: '#c7edcc',
      vars: {
        '--void-void': '#b8e6be',
        '--void-black': '#c7edcc',
        '--void-deep': '#b8e6be',
        '--void-card': '#d4f0d8',
        '--void-border': '#a8d8ad',
        '--void-subtle': '#98c89d',
        '--annihilation-dim': '#1a6a28',
        '--annihilation-cyan': '#2d8a42',
        '--annihilation-glow': '#3aa855',
        '--annihilation-dark': '#1a5a22',
        '--ash-dim': '#3a2a80',
        '--ash-purple': '#5a4a9a',
        '--ash-glow': '#7a6aba',
        '--status-pending': '#5a7a5e',
        '--status-processing': '#2d8a42',
        '--status-reviewing': '#5a4a9a',
        '--status-completed': '#1a6a28',
        '--status-failed': '#b03030',
        '--text-primary': '#1a3a1e',
        '--text-secondary': '#2d5a32',
        '--text-muted': '#5a8a5e',
        '--glow-cyan': '0 0 20px rgba(45, 138, 66, 0.2)',
        '--glow-purple': '0 0 30px rgba(90, 74, 154, 0.15)'
      }
    },
    'night-blue': {
      name: '深夜蓝',
      dotColor: '#0d1b2a',
      vars: {
        '--void-void': '#0a1420',
        '--void-black': '#0d1b2a',
        '--void-deep': '#1b2a3a',
        '--void-card': '#243447',
        '--void-border': '#3a4a5a',
        '--void-subtle': '#4a5a6a',
        '--annihilation-dim': '#3070c0',
        '--annihilation-cyan': '#4a9eff',
        '--annihilation-glow': '#6ab0ff',
        '--annihilation-dark': '#2060a0',
        '--ash-dim': '#5a3fa5',
        '--ash-purple': '#7a5ce7',
        '--ash-glow': '#9a7cf7',
        '--status-pending': '#506070',
        '--status-processing': '#4a9eff',
        '--status-reviewing': '#7a5ce7',
        '--status-completed': '#30a080',
        '--status-failed': '#ff4757',
        '--text-primary': '#c8d8e8',
        '--text-secondary': '#a0b0c0',
        '--text-muted': '#708090',
        '--glow-cyan': '0 0 20px rgba(74, 158, 255, 0.25)',
        '--glow-purple': '0 0 30px rgba(122, 92, 231, 0.2)'
      }
    },
    warm: {
      name: '暖黄灯',
      dotColor: '#faf0e0',
      vars: {
        '--void-void': '#e8dac0',
        '--void-black': '#faf0e0',
        '--void-deep': '#f0e6d0',
        '--void-card': '#ffffff',
        '--void-border': '#e0d0b0',
        '--void-subtle': '#d0c0a0',
        '--annihilation-dim': '#905820',
        '--annihilation-cyan': '#c07830',
        '--annihilation-glow': '#d89040',
        '--annihilation-dark': '#704810',
        '--ash-dim': '#5a3fa5',
        '--ash-purple': '#7a5cb5',
        '--ash-glow': '#9a7cd5',
        '--status-pending': '#8a7a6a',
        '--status-processing': '#c07830',
        '--status-reviewing': '#7a5cb5',
        '--status-completed': '#5a8a20',
        '--status-failed': '#c03030',
        '--text-primary': '#3a2a1a',
        '--text-secondary': '#6a5a4a',
        '--text-muted': '#9a8a7a',
        '--glow-cyan': '0 0 20px rgba(192, 120, 48, 0.2)',
        '--glow-purple': '0 0 30px rgba(122, 92, 181, 0.15)'
      }
    }
  };

  // 自定义主题编辑器中可编辑的变量（Rime-See-Me 风格）
  const EDITABLE_VARS = [
    { key: '--void-black', label: '主背景色' },
    { key: '--void-deep', label: '次要背景色' },
    { key: '--void-card', label: '卡片底色' },
    { key: '--void-border', label: '边框色' },
    { key: '--text-primary', label: '主文字色' },
    { key: '--text-secondary', label: '次要文字色' },
    { key: '--text-muted', label: '弱化文字色' },
    { key: '--annihilation-cyan', label: '强调/高亮色' }
  ];

  const DEFAULT_THEME = 'dark'; // 统一默认主题，与 localStorage 回退值保持一致
  let currentTheme = DEFAULT_THEME;
  let editorState = null; // 编辑器临时状态

  // ==================== 初始化 ====================
  function init() {
    const savedTheme = localStorage.getItem(STORAGE_KEY) || DEFAULT_THEME;
    applyTheme(savedTheme);
  }

  // ==================== 应用主题 ====================
  function applyTheme(themeName) {
    var reviewSection = document.getElementById('review-section');
    var batchSection = document.getElementById('batch-review-section');

    // 清除之前的行内 CSS 变量
    var targetElements = [reviewSection, batchSection].filter(Boolean);
    targetElements.forEach(function(el) {
      Object.keys(BUILTIN_THEMES.dark.vars).forEach(function(varName) {
        el.style.removeProperty(varName);
      });
    });

    var effectiveTheme = themeName;
    if (!BUILTIN_THEMES[themeName] && !themeName.startsWith('custom-')) {
      effectiveTheme = 'light';
    }

    targetElements.forEach(function(el) {
      el.setAttribute('data-theme', effectiveTheme);
    });

    if (effectiveTheme.startsWith('custom-')) {
      var customData = getCustomTheme(effectiveTheme.replace('custom-', ''));
      if (customData) {
        targetElements.forEach(function(el) {
          Object.entries(customData.vars).forEach(function(entry) {
            el.style.setProperty(entry[0], entry[1]);
          });
        });
      }
    }

    currentTheme = effectiveTheme;
    localStorage.setItem(STORAGE_KEY, effectiveTheme);
    updateThemeButtons();
  }

  // ==================== 获取当前主题 ====================
  function getCurrentTheme() {
    return currentTheme;
  }

  // ==================== 获取所有主题（内置 + 自定义） ====================
  function getAllThemes() {
    var themes = [];
    Object.entries(BUILTIN_THEMES).forEach(function(entry) {
      themes.push({
        id: entry[0],
        name: entry[1].name,
        dotColor: entry[1].dotColor,
        isBuiltin: true
      });
    });
    var customList = getCustomThemeList();
    customList.forEach(function(name) {
      var data = getCustomTheme(name);
      if (data) {
        themes.push({
          id: 'custom-' + name,
          name: name,
          dotColor: data.vars['--void-black'] || '#333',
          isBuiltin: false
        });
      }
    });
    return themes;
  }

  // ==================== 自定义主题 CRUD ====================
  function getCustomThemeList() {
    try {
      return JSON.parse(localStorage.getItem(CUSTOM_LIST_KEY) || '[]');
    } catch (e) {
      return [];
    }
  }

  function getCustomTheme(name) {
    try {
      return JSON.parse(localStorage.getItem(CUSTOM_PREFIX + name) || 'null');
    } catch (e) {
      return null;
    }
  }

  function saveCustomTheme(name, vars) {
    if (!name || name.length > 20) return false;
    // 不允许与内置主题同名
    if (BUILTIN_THEMES[name]) return false;

    var list = getCustomThemeList();
    var isNew = list.indexOf(name) === -1;

    // 保存主题数据
    var themeData = {
      name: name,
      createdAt: isNew ? Date.now() : (getCustomTheme(name) || {}).createdAt || Date.now(),
      vars: vars
    };
    localStorage.setItem(CUSTOM_PREFIX + name, JSON.stringify(themeData));

    // 更新主题列表
    if (isNew) {
      list.push(name);
      localStorage.setItem(CUSTOM_LIST_KEY, JSON.stringify(list));
    }

    return true;
  }

  function deleteCustomTheme(name) {
    var list = getCustomThemeList();
    var idx = list.indexOf(name);
    if (idx === -1) return false;

    list.splice(idx, 1);
    localStorage.setItem(CUSTOM_LIST_KEY, JSON.stringify(list));
    localStorage.removeItem(CUSTOM_PREFIX + name);

    // 如果当前正在使用该主题，回退到默认主题
    if (currentTheme === 'custom-' + name) {
      applyTheme(DEFAULT_THEME);
    }

    return true;
  }

  // ==================== 主题按钮 UI 更新 ====================
  function updateThemeButtons() {
    document.querySelectorAll('.theme-btn').forEach(function(btn) {
      var isActive = btn.dataset.themeId === currentTheme;
      btn.classList.toggle('active', isActive);
    });
  }

  // ==================== 渲染主题按钮 ====================
  function renderThemeButtons() {
    var container = document.getElementById('theme-grid');
    if (!container) return;

    container.innerHTML = '';

    getAllThemes().forEach(function(theme) {
      var btn = document.createElement('button');
      btn.className = 'theme-btn' + (theme.id === currentTheme ? ' active' : '');
      btn.dataset.themeId = theme.id;

      var dot = document.createElement('span');
      dot.className = 'theme-btn-dot';
      dot.style.backgroundColor = theme.dotColor;
      btn.appendChild(dot);

      var label = document.createElement('span');
      label.textContent = theme.name;
      btn.appendChild(label);

      // 自定义主题添加删除按钮
      if (!theme.isBuiltin) {
        var deleteBtn = document.createElement('button');
        deleteBtn.className = 'theme-btn-delete';
        deleteBtn.textContent = '✕';
        deleteBtn.title = '删除主题';
        deleteBtn.addEventListener('click', function(e) {
          e.stopPropagation();
          showCustomConfirm('确认删除主题 "' + theme.name + '"？', function() {
            deleteCustomTheme(theme.name);
            renderThemeButtons();
          });
        });
        btn.appendChild(deleteBtn);
      }

      btn.addEventListener('click', function() {
        applyTheme(theme.id);
      });

      container.appendChild(btn);
    });

    // 添加自定义主题按钮
    var addBtn = document.createElement('button');
    addBtn.className = 'theme-btn-add';
    addBtn.textContent = '+ 自定义';
    addBtn.addEventListener('click', function() {
      openThemeEditor();
    });
    container.appendChild(addBtn);
  }

  // ==================== 自定义主题编辑器 ====================
  function openThemeEditor(editName) {
    editorState = {
      name: editName || '',
      vars: {}
    };

    // 初始化变量值
    EDITABLE_VARS.forEach(function(v) {
      if (editName) {
        var customData = getCustomTheme(editName);
        editorState.vars[v.key] = customData ? (customData.vars[v.key] || '') : '';
      } else {
        // 默认从 BUILTIN_THEMES 读取值
        var baseVars = BUILTIN_THEMES[currentTheme] ? BUILTIN_THEMES[currentTheme].vars : BUILTIN_THEMES.light.vars;
        editorState.vars[v.key] = baseVars[v.key] || '';
      }
    });

    var overlay = document.getElementById('theme-editor-overlay');
    if (!overlay) return;

    // 填充名称
    var nameInput = document.getElementById('theme-editor-name');
    if (nameInput) {
      nameInput.value = editName || '';
      nameInput.disabled = !!editName;
    }

    // 填充颜色配置
    renderColorConfigList();

    // 更新预览
    updateEditorPreview();

    overlay.classList.remove('hidden');
  }

  function closeThemeEditor() {
    var overlay = document.getElementById('theme-editor-overlay');
    if (overlay) {
      overlay.classList.add('hidden');
    }
    editorState = null;
  }

  function renderColorConfigList() {
    var container = document.getElementById('color-config-list');
    if (!container) return;

    container.innerHTML = '';

    EDITABLE_VARS.forEach(function(v) {
      var item = document.createElement('div');
      item.className = 'color-config-item';

      // 色块
      var swatch = document.createElement('span');
      swatch.className = 'color-config-swatch';
      swatch.id = 'swatch-' + v.key;
      swatch.style.backgroundColor = editorState.vars[v.key] || 'transparent';
      item.appendChild(swatch);

      // 标签
      var label = document.createElement('span');
      label.className = 'color-config-label';
      label.textContent = v.label;
      item.appendChild(label);

      // 输入框
      var input = document.createElement('input');
      input.type = 'text';
      input.className = 'color-config-input';
      input.id = 'input-' + v.key;
      input.value = editorState.vars[v.key] || '';
      input.placeholder = '#000000';
      input.dataset.varKey = v.key;
      input.addEventListener('input', function() {
        handleColorInput(v.key, this.value, swatch, this);
      });
      item.appendChild(input);

      container.appendChild(item);
    });
  }

  function handleColorInput(varKey, value, swatch, input) {
    // 自动补全 #
    if (value && !value.startsWith('#')) {
      value = '#' + value;
      input.value = value;
    }

    // 校验 hex 格式
    var isValid = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(value);

    if (isValid) {
      input.classList.remove('error');
      swatch.style.backgroundColor = value;
      editorState.vars[varKey] = value;
    } else if (value === '' || value === '#') {
      input.classList.remove('error');
      swatch.style.backgroundColor = 'transparent';
      editorState.vars[varKey] = '';
    } else {
      input.classList.add('error');
    }

    updateEditorPreview();
  }

  function updateEditorPreview() {
    var box = document.getElementById('theme-preview-box');
    if (!box || !editorState) return;

    var vars = editorState.vars;

    // 通过 CSS 变量设置预览区样式，与主渲染机制保持一致
    Object.entries(vars).forEach(function(entry) {
      if (entry[1]) {
        box.style.setProperty(entry[0], entry[1]);
      }
    });
  }

  function saveEditorTheme() {
    if (!editorState) return;

    var nameInput = document.getElementById('theme-editor-name');
    var name = nameInput ? nameInput.value.trim() : '';

    if (!name) {
      if (nameInput) nameInput.classList.add('error');
      return;
    }

    if (name.length > 20) {
      if (nameInput) nameInput.classList.add('error');
      return;
    }

    // 构建完整的 vars 对象（继承当前主题的非编辑变量）
    var fullVars = {};
    // 先填充当前主题的完整变量（支持自定义主题作为 base）
    var baseTheme;
    if (currentTheme.startsWith('custom-')) {
      var customData = getCustomTheme(currentTheme.replace('custom-', ''));
      baseTheme = customData || BUILTIN_THEMES.dark;
    } else {
      baseTheme = BUILTIN_THEMES[currentTheme] || BUILTIN_THEMES.dark;
    }
    Object.entries(baseTheme.vars).forEach(function(entry) {
      fullVars[entry[0]] = entry[1];
    });
    // 用编辑器的值覆盖
    Object.entries(editorState.vars).forEach(function(entry) {
      if (entry[1]) {
        fullVars[entry[0]] = entry[1];
      }
    });

    var success = saveCustomTheme(name, fullVars);
    if (success) {
      applyTheme('custom-' + name);
      renderThemeButtons();
      closeThemeEditor();
    }
  }

  // ==================== 自定义确认弹窗 ====================
  function showCustomConfirm(message, onConfirm) {
    var existingOverlay = document.getElementById('theme-confirm-overlay');
    if (existingOverlay) existingOverlay.remove();

    var overlay = document.createElement('div');
    overlay.id = 'theme-confirm-overlay';
    overlay.className = 'theme-editor-overlay';
    overlay.style.display = 'flex';
    overlay.style.alignItems = 'center';
    overlay.style.justifyContent = 'center';

    var dialog = document.createElement('div');
    dialog.className = 'theme-editor';
    dialog.style.width = '360px';
    dialog.style.textAlign = 'center';

    var body = document.createElement('div');
    body.className = 'theme-editor-body';
    body.style.padding = '32px 24px';

    var msg = document.createElement('p');
    msg.style.margin = '0 0 24px';
    msg.style.color = 'var(--text-primary)';
    msg.style.fontSize = '14px';
    msg.textContent = message;
    body.appendChild(msg);

    var btnRow = document.createElement('div');
    btnRow.style.display = 'flex';
    btnRow.style.justifyContent = 'center';
    btnRow.style.gap = '12px';

    var cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn-secondary';
    cancelBtn.textContent = '取消';
    cancelBtn.addEventListener('click', function() { overlay.remove(); });
    btnRow.appendChild(cancelBtn);

    var confirmBtn = document.createElement('button');
    confirmBtn.className = 'btn-primary';
    confirmBtn.textContent = '确认';
    confirmBtn.addEventListener('click', function() {
      overlay.remove();
      if (typeof onConfirm === 'function') onConfirm();
    });
    btnRow.appendChild(confirmBtn);

    body.appendChild(btnRow);
    dialog.appendChild(body);
    overlay.appendChild(dialog);
    document.body.appendChild(overlay);

    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) overlay.remove();
    });
    document.addEventListener('keydown', function escHandler(e) {
      if (e.key === 'Escape') { overlay.remove(); document.removeEventListener('keydown', escHandler); }
    });
  }

  // ==================== 公共 API ====================
  return {
    init: init,
    applyTheme: applyTheme,
    getCurrentTheme: getCurrentTheme,
    getAllThemes: getAllThemes,
    getCustomThemeList: getCustomThemeList,
    getCustomTheme: getCustomTheme,
    saveCustomTheme: saveCustomTheme,
    deleteCustomTheme: deleteCustomTheme,
    renderThemeButtons: renderThemeButtons,
    openThemeEditor: openThemeEditor,
    closeThemeEditor: closeThemeEditor,
    saveEditorTheme: saveEditorTheme,
    BUILTIN_THEMES: BUILTIN_THEMES,
    EDITABLE_VARS: EDITABLE_VARS
  };
})();

window.ThemeModule = ThemeModule;
