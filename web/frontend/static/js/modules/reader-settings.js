// =============================================
// ReaderSettingsModule - 阅读设置模块
// 字体选择、自定义字体、间距调节、设置重置
// =============================================

const ReaderSettingsModule = (function() {
  // localStorage 键名
  const FONT_FAMILY_KEY = 'voidtext-font-family';
  const CUSTOM_FONT_URLS_KEY = 'voidtext-custom-font-urls';
  const LETTER_SPACING_KEY = 'voidtext-reader-letter-spacing';
  const LINE_HEIGHT_KEY = 'voidtext-reader-line-height';
  const PARAGRAPH_GAP_KEY = 'voidtext-reader-paragraph-gap';

  // 字体选项
  const FONT_OPTIONS = [
    { name: '默认无衬线', family: '"Inter", "Noto Sans SC", -apple-system, sans-serif' },
    { name: '宋体', family: '"Noto Serif SC", "Source Han Serif SC", serif' },
    { name: '黑体', family: '"Noto Sans SC", "Microsoft YaHei", "SimHei", sans-serif' },
    { name: '楷体', family: '"KaiTi", "STKaiti", "AR PL UKai CN", serif' },
    { name: '等线', family: '"DengXian", "Microsoft YaHei Mono", sans-serif' },
    { name: '霞鹜文楷', family: '"LXGW WenKai", "KaiTi", serif' }
  ];

  // 间距默认值
  const DEFAULTS = {
    letterSpacing: 0,
    lineHeight: 1.8,
    paragraphGap: 1.0
  };

  // 自定义字体缓存
  var customFonts = [];
  var panelVisible = false;
  var addFontDialogOpen = false;

  // ==================== 初始化 ====================
  function init() {
    // 加载自定义字体
    loadCustomFonts();

    // 应用保存的设置
    applyFontFamily(localStorage.getItem(FONT_FAMILY_KEY) || '默认无衬线');
    applyLetterSpacing(parseFloat(localStorage.getItem(LETTER_SPACING_KEY)) || DEFAULTS.letterSpacing);
    applyLineHeight(parseFloat(localStorage.getItem(LINE_HEIGHT_KEY)) || DEFAULTS.lineHeight);
    applyParagraphGap(parseFloat(localStorage.getItem(PARAGRAPH_GAP_KEY)) || DEFAULTS.paragraphGap);

    // 绑定事件
    bindEvents();
  }

  // ==================== 事件绑定（拆分为子函数） ====================
  function bindSettingsPanelEvents() {
    var settingsBtn = document.getElementById('btn-reader-settings');
    if (settingsBtn) {
      settingsBtn.addEventListener('click', togglePanel);
    }

    document.addEventListener('click', function(e) {
      var panel = document.getElementById('reader-settings-panel');
      var btn = document.getElementById('btn-reader-settings');
      if (panelVisible && panel && btn && !panel.contains(e.target) && !btn.contains(e.target)) {
        closePanel();
      }
    });

    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') {
        if (addFontDialogOpen) {
          closeAddFontDialog();
        } else if (panelVisible) {
          closePanel();
        }
      }
    });
  }

  function bindFontEvents() {
    var fontSelect = document.getElementById('font-family-select');
    if (fontSelect) {
      fontSelect.addEventListener('change', function() {
        applyFontFamily(this.value);
        localStorage.setItem(FONT_FAMILY_KEY, this.value);
      });
    }

    var addFontBtn = document.getElementById('btn-add-font');
    if (addFontBtn) {
      addFontBtn.addEventListener('click', openAddFontDialog);
    }
  }

  function bindSpacingEvents() {
    bindSlider('letter-spacing-slider', 'letter-spacing-value', function(val) {
      applyLetterSpacing(val);
      localStorage.setItem(LETTER_SPACING_KEY, val.toString());
    }, function(val) { return val.toFixed(2) + 'em'; });

    bindSlider('line-height-slider', 'line-height-value', function(val) {
      applyLineHeight(val);
      localStorage.setItem(LINE_HEIGHT_KEY, val.toString());
    }, function(val) { return val.toFixed(1); });

    bindSlider('paragraph-gap-slider', 'paragraph-gap-value', function(val) {
      applyParagraphGap(val);
      localStorage.setItem(PARAGRAPH_GAP_KEY, val.toString());
    }, function(val) { return val.toFixed(1) + 'em'; });

    var resetBtn = document.getElementById('btn-reset-settings');
    if (resetBtn) {
      resetBtn.addEventListener('click', resetAllSettings);
    }
  }

  function bindEditorModalEvents() {
    var editorClose = document.getElementById('theme-editor-close');
    if (editorClose) {
      editorClose.addEventListener('click', function() {
        ThemeModule.closeThemeEditor();
      });
    }

    var editorSave = document.getElementById('theme-editor-save');
    if (editorSave) {
      editorSave.addEventListener('click', function() {
        ThemeModule.saveEditorTheme();
      });
    }

    var editorCancel = document.getElementById('theme-editor-cancel');
    if (editorCancel) {
      editorCancel.addEventListener('click', function() {
        ThemeModule.closeThemeEditor();
      });
    }

    var editorOverlay = document.getElementById('theme-editor-overlay');
    if (editorOverlay) {
      editorOverlay.addEventListener('click', function(e) {
        if (e.target === editorOverlay) {
          ThemeModule.closeThemeEditor();
        }
      });
    }
  }

  function bindFontDialogEvents() {
    var addFontSave = document.getElementById('add-font-save');
    if (addFontSave) {
      addFontSave.addEventListener('click', handleAddFont);
    }

    var addFontCancel = document.getElementById('add-font-cancel');
    if (addFontCancel) {
      addFontCancel.addEventListener('click', closeAddFontDialog);
    }

    var addFontCloseBtn = document.getElementById('add-font-close-btn');
    if (addFontCloseBtn) {
      addFontCloseBtn.addEventListener('click', closeAddFontDialog);
    }

    var addFontOverlay = document.getElementById('add-font-overlay');
    if (addFontOverlay) {
      addFontOverlay.addEventListener('click', function(e) {
        if (e.target === addFontOverlay) {
          closeAddFontDialog();
        }
      });
    }
  }

  function bindEvents() {
    bindSettingsPanelEvents();
    bindFontEvents();
    bindSpacingEvents();
    bindEditorModalEvents();
    bindFontDialogEvents();
  }

  // ==================== 面板开关 ====================
  function togglePanel() {
    if (panelVisible) {
      closePanel();
    } else {
      openPanel();
    }
  }

  function openPanel() {
    var panel = document.getElementById('reader-settings-panel');
    var btn = document.getElementById('btn-reader-settings');
    if (panel && btn) {
      // 渲染主题按钮
      ThemeModule.renderThemeButtons();
      // 渲染字体选项
      renderFontOptions();
      // 渲染自定义字体列表
      renderCustomFontList();
      // 更新滑块状态
      updateSliderValues();

      panel.classList.remove('hidden');
      btn.classList.add('active');
      panelVisible = true;
    }
  }

  function closePanel() {
    var panel = document.getElementById('reader-settings-panel');
    var btn = document.getElementById('btn-reader-settings');
    if (panel && btn) {
      panel.classList.add('hidden');
      btn.classList.remove('active');
      panelVisible = false;
    }
  }

  // ==================== 字体选择 ====================
  function renderFontOptions() {
    var select = document.getElementById('font-family-select');
    if (!select) return;

    var currentValue = localStorage.getItem(FONT_FAMILY_KEY) || '默认无衬线';
    select.innerHTML = '';

    FONT_OPTIONS.forEach(function(font) {
      var option = document.createElement('option');
      option.value = font.name;
      option.textContent = font.name;
      option.style.fontFamily = font.family;
      if (font.name === currentValue) option.selected = true;
      select.appendChild(option);
    });

    // 自定义字体选项
    customFonts.forEach(function(cf) {
      var option = document.createElement('option');
      option.value = cf.name;
      option.textContent = cf.name + ' (自定义)';
      option.style.fontFamily = '"' + cf.name + '"';
      if (cf.name === currentValue) option.selected = true;
      select.appendChild(option);
    });
  }

  function applyFontFamily(fontName) {
    var fontFamily = null;

    // 从内置列表查找
    FONT_OPTIONS.forEach(function(f) {
      if (f.name === fontName) fontFamily = f.family;
    });

    // 从自定义字体查找
    if (!fontFamily) {
      customFonts.forEach(function(cf) {
        if (cf.name === fontName) fontFamily = '"' + cf.name + '", sans-serif';
      });
    }

    if (!fontFamily) fontFamily = FONT_OPTIONS[0].family;

    document.documentElement.style.setProperty('--reader-font-family', fontFamily);
  }

  // ==================== 自定义字体 ====================
  function loadCustomFonts() {
    try {
      customFonts = JSON.parse(localStorage.getItem(CUSTOM_FONT_URLS_KEY) || '[]');
    } catch (e) {
      customFonts = [];
    }

    // 注册所有自定义字体
    customFonts.forEach(function(cf) {
      registerFontFace(cf.name, cf.url);
    });
  }

  function registerFontFace(name, url) {
    try {
      var font = new FontFace(name, 'url(' + encodeURI(url) + ')');
      var FONT_TIMEOUT_MS = 30000;
      var timeoutPromise = new Promise(function(_, reject) {
        setTimeout(function() { reject(new Error('字体加载超时 (' + FONT_TIMEOUT_MS + 'ms)')); }, FONT_TIMEOUT_MS);
      });
      Promise.race([font.load(), timeoutPromise]).then(function(loaded) {
        document.fonts.add(loaded);
      }).catch(function(err) {
        console.warn('字体加载失败:', name, err.message || err);
      });
    } catch (e) {
      console.warn('FontFace API 不可用:', e);
    }
  }

  function addCustomFont(name, url) {
    // 检查重名
    var exists = customFonts.some(function(cf) { return cf.name === name; });
    if (exists) return false;

    customFonts.push({ name: name, url: url });
    localStorage.setItem(CUSTOM_FONT_URLS_KEY, JSON.stringify(customFonts));
    registerFontFace(name, url);
    return true;
  }

  function removeCustomFont(name) {
    var idx = customFonts.findIndex(function(cf) { return cf.name === name; });
    if (idx === -1) return false;

    customFonts.splice(idx, 1);
    localStorage.setItem(CUSTOM_FONT_URLS_KEY, JSON.stringify(customFonts));

    // 如果当前正在使用该字体，回退到默认
    var current = localStorage.getItem(FONT_FAMILY_KEY);
    if (current === name) {
      applyFontFamily('默认无衬线');
      localStorage.setItem(FONT_FAMILY_KEY, '默认无衬线');
    }

    return true;
  }

  function renderCustomFontList() {
    var container = document.getElementById('custom-font-list');
    if (!container) return;

    container.innerHTML = '';

    customFonts.forEach(function(cf) {
      var item = document.createElement('div');
      item.className = 'custom-font-item';

      var nameSpan = document.createElement('span');
      nameSpan.className = 'custom-font-item-name';
      nameSpan.textContent = cf.name;
      nameSpan.style.fontFamily = '"' + cf.name + '"';
      item.appendChild(nameSpan);

      var deleteBtn = document.createElement('button');
      deleteBtn.className = 'custom-font-item-delete';
      deleteBtn.textContent = '✕';
      deleteBtn.title = '删除字体';
      deleteBtn.addEventListener('click', function() {
        removeCustomFont(cf.name);
        renderCustomFontList();
        renderFontOptions();
      });
      item.appendChild(deleteBtn);

      container.appendChild(item);
    });
  }

  function openAddFontDialog() {
    var overlay = document.getElementById('add-font-overlay');
    if (!overlay) return;

    var nameInput = document.getElementById('add-font-name');
    var urlInput = document.getElementById('add-font-url');
    var errorDiv = document.getElementById('add-font-error');

    if (nameInput) nameInput.value = '';
    if (urlInput) urlInput.value = '';
    if (errorDiv) errorDiv.classList.remove('visible');

    overlay.classList.remove('hidden');
    addFontDialogOpen = true;
  }

  function closeAddFontDialog() {
    var overlay = document.getElementById('add-font-overlay');
    if (overlay) overlay.classList.add('hidden');
    addFontDialogOpen = false;
  }

  function handleAddFont() {
    var nameInput = document.getElementById('add-font-name');
    var urlInput = document.getElementById('add-font-url');
    var errorDiv = document.getElementById('add-font-error');

    var name = nameInput ? nameInput.value.trim() : '';
    var url = urlInput ? urlInput.value.trim() : '';

    // 校验
    if (!name) {
      if (nameInput) nameInput.classList.add('error');
      if (errorDiv) {
        errorDiv.textContent = '请输入字体名称';
        errorDiv.classList.add('visible');
      }
      return;
    }

    if (!url) {
      if (urlInput) urlInput.classList.add('error');
      if (errorDiv) {
        errorDiv.textContent = '请输入字体 URL';
        errorDiv.classList.add('visible');
      }
      return;
    }

    if (!url.endsWith('.woff2') && !url.endsWith('.woff') && !url.endsWith('.ttf')) {
      if (urlInput) urlInput.classList.add('error');
      if (errorDiv) {
        errorDiv.textContent = '仅支持 woff2/woff/ttf 格式的字体文件';
        errorDiv.classList.add('visible');
      }
      return;
    }

    var success = addCustomFont(name, url);
    if (!success) {
      if (errorDiv) {
        errorDiv.textContent = '字体名称已存在';
        errorDiv.classList.add('visible');
      }
      return;
    }

    closeAddFontDialog();
    renderCustomFontList();
    renderFontOptions();
  }

  // ==================== 间距调节 ====================
  function bindSlider(sliderId, valueId, onChange, formatValue) {
    var slider = document.getElementById(sliderId);
    var valueDisplay = document.getElementById(valueId);
    if (!slider) return;

    slider.addEventListener('input', function() {
      var val = parseFloat(this.value);
      if (valueDisplay && formatValue) {
        valueDisplay.textContent = formatValue(val);
      }
      onChange(val);
    });
  }

  function applyLetterSpacing(val) {
    document.documentElement.style.setProperty('--reader-letter-spacing', val + 'em');
  }

  function applyLineHeight(val) {
    document.documentElement.style.setProperty('--reader-line-height', val.toString());
  }

  function applyParagraphGap(val) {
    document.documentElement.style.setProperty('--reader-paragraph-gap', val + 'em');
  }

  function updateSliderValues() {
    var ls = parseFloat(localStorage.getItem(LETTER_SPACING_KEY));
    var lh = parseFloat(localStorage.getItem(LINE_HEIGHT_KEY));
    var pg = parseFloat(localStorage.getItem(PARAGRAPH_GAP_KEY));

    var lsSlider = document.getElementById('letter-spacing-slider');
    var lsValue = document.getElementById('letter-spacing-value');
    if (lsSlider) {
      lsSlider.value = isNaN(ls) ? DEFAULTS.letterSpacing : ls;
    }
    if (lsValue) {
      var lsVal = isNaN(ls) ? DEFAULTS.letterSpacing : ls;
      lsValue.textContent = lsVal.toFixed(2) + 'em';
    }

    var lhSlider = document.getElementById('line-height-slider');
    var lhValue = document.getElementById('line-height-value');
    if (lhSlider) {
      lhSlider.value = isNaN(lh) ? DEFAULTS.lineHeight : lh;
    }
    if (lhValue) {
      var lhVal = isNaN(lh) ? DEFAULTS.lineHeight : lh;
      lhValue.textContent = lhVal.toFixed(1);
    }

    var pgSlider = document.getElementById('paragraph-gap-slider');
    var pgValue = document.getElementById('paragraph-gap-value');
    if (pgSlider) {
      pgSlider.value = isNaN(pg) ? DEFAULTS.paragraphGap : pg;
    }
    if (pgValue) {
      var pgVal = isNaN(pg) ? DEFAULTS.paragraphGap : pg;
      pgValue.textContent = pgVal.toFixed(1) + 'em';
    }
  }

  // ==================== 自定义确认弹窗 ====================
  function showCustomConfirmDialog(message, onConfirm) {
    var existingOverlay = document.getElementById('custom-confirm-overlay');
    if (existingOverlay) existingOverlay.remove();

    var overlay = document.createElement('div');
    overlay.id = 'custom-confirm-overlay';
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
    msg.style.lineHeight = '1.6';
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

    // Esc 关闭
    var escHandler = function(e) {
      if (e.key === 'Escape') {
        overlay.remove();
        document.removeEventListener('keydown', escHandler);
      }
    };
    document.addEventListener('keydown', escHandler);

    // 点击遮罩关闭
    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) {
        overlay.remove();
      }
    });
  }

  // ==================== 重置所有设置 ====================
  function resetAllSettings() {
    showCustomConfirmDialog('确认恢复默认设置？所有自定义主题、字体和间距设置将被清除。', function() {
      performReset();
    });
  }

  function performReset() {
    // 清除所有 voidtext- 开头的 localStorage 键
    var keysToRemove = [];
    for (var i = 0; i < localStorage.length; i++) {
      var key = localStorage.key(i);
      if (key && key.startsWith('voidtext-')) {
        keysToRemove.push(key);
      }
    }
    keysToRemove.forEach(function(key) {
      localStorage.removeItem(key);
    });

    // 重置主题
    ThemeModule.applyTheme('light');

    // 重置字体
    applyFontFamily('默认无衬线');

    // 重置间距
    applyLetterSpacing(DEFAULTS.letterSpacing);
    applyLineHeight(DEFAULTS.lineHeight);
    applyParagraphGap(DEFAULTS.paragraphGap);

    // 重置自定义字体
    customFonts = [];

    // 刷新面板
    if (panelVisible) {
      ThemeModule.renderThemeButtons();
      renderFontOptions();
      renderCustomFontList();
      updateSliderValues();
    }

    // 重新保存默认值
    localStorage.setItem(FONT_FAMILY_KEY, '默认无衬线');
    localStorage.setItem(LETTER_SPACING_KEY, DEFAULTS.letterSpacing.toString());
    localStorage.setItem(LINE_HEIGHT_KEY, DEFAULTS.lineHeight.toString());
    localStorage.setItem(PARAGRAPH_GAP_KEY, DEFAULTS.paragraphGap.toString());
  }

  // ==================== 公共 API ====================
  return {
    init: init,
    togglePanel: togglePanel,
    openPanel: openPanel,
    closePanel: closePanel,
    resetAllSettings: resetAllSettings
  };
})();

window.ReaderSettingsModule = ReaderSettingsModule;
