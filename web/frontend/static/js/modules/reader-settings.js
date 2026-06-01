// =============================================
// ReaderSettingsModule - 阅读设置模块
// 字体选择、自定义字体、间距调节、设置重置
// =============================================

const ReaderSettingsModule = (function() {
  // localStorage 键名
  const FONT_FAMILY_KEY = 'voidtext-font-family';
  const CUSTOM_FONT_META_KEY = 'voidtext-custom-font-meta'; // 只存名称元数据，blob 存 IndexedDB
  const LETTER_SPACING_KEY = 'voidtext-reader-letter-spacing';
  const LINE_HEIGHT_KEY = 'voidtext-reader-line-height';
  const PARAGRAPH_GAP_KEY = 'voidtext-reader-paragraph-gap';

  // IndexedDB 配置
  const FONT_DB_NAME = 'voidtext-fonts';
  const FONT_DB_VERSION = 1;
  const FONT_STORE_NAME = 'fonts';

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
  let customFonts = [];
  let panelVisible = false;
  let addFontDialogOpen = false;
  const fontBlobUrls = new Map(); // name -> blobUrl，用于 revoke

  // ==================== 初始化 ====================
  function init() {
    // 加载自定义字体
    loadCustomFonts();

    // 应用保存的设置
    applyFontFamily(localStorage.getItem(FONT_FAMILY_KEY) || '默认无衬线');
    const ls = parseFloat(localStorage.getItem(LETTER_SPACING_KEY));
    applyLetterSpacing(Number.isNaN(ls) ? DEFAULTS.letterSpacing : ls);
    const lh = parseFloat(localStorage.getItem(LINE_HEIGHT_KEY));
    applyLineHeight(Number.isNaN(lh) ? DEFAULTS.lineHeight : lh);
    const pg = parseFloat(localStorage.getItem(PARAGRAPH_GAP_KEY));
    applyParagraphGap(Number.isNaN(pg) ? DEFAULTS.paragraphGap : pg);

    // 绑定事件
    bindEvents();
  }

  // ==================== 事件绑定（拆分为子函数） ====================
  function bindSettingsPanelEvents() {
    const settingsBtn = document.getElementById('btn-reader-settings');
    if (settingsBtn) {
      settingsBtn.addEventListener('click', togglePanel);
    }

    document.addEventListener('click', function(e) {
      const panel = document.getElementById('reader-settings-panel');
      const btn = document.getElementById('btn-reader-settings');
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
    const fontSelect = document.getElementById('font-family-select');
    if (fontSelect) {
      fontSelect.addEventListener('change', function() {
        applyFontFamily(this.value);
        localStorage.setItem(FONT_FAMILY_KEY, this.value);
      });
    }

    const addFontBtn = document.getElementById('btn-add-font');
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

    const resetBtn = document.getElementById('btn-reset-settings');
    if (resetBtn) {
      resetBtn.addEventListener('click', resetAllSettings);
    }
  }

  function bindEditorModalEvents() {
    const editorClose = document.getElementById('theme-editor-close');
    if (editorClose) {
      editorClose.addEventListener('click', function() {
        if (typeof ThemeModule !== 'undefined') ThemeModule.closeThemeEditor();
      });
    }

    const editorSave = document.getElementById('theme-editor-save');
    if (editorSave) {
      editorSave.addEventListener('click', function() {
        if (typeof ThemeModule !== 'undefined') ThemeModule.saveEditorTheme();
      });
    }

    const editorCancel = document.getElementById('theme-editor-cancel');
    if (editorCancel) {
      editorCancel.addEventListener('click', function() {
        if (typeof ThemeModule !== 'undefined') ThemeModule.closeThemeEditor();
      });
    }

    const editorOverlay = document.getElementById('theme-editor-overlay');
    if (editorOverlay) {
      editorOverlay.addEventListener('click', function(e) {
        if (e.target === editorOverlay) {
          if (typeof ThemeModule !== 'undefined') ThemeModule.closeThemeEditor();
        }
      });
    }
  }

  function bindFontDialogEvents() {
    const addFontSave = document.getElementById('add-font-save');
    if (addFontSave) {
      addFontSave.addEventListener('click', handleAddFont);
    }

    const addFontCancel = document.getElementById('add-font-cancel');
    if (addFontCancel) {
      addFontCancel.addEventListener('click', closeAddFontDialog);
    }

    const addFontCloseBtn = document.getElementById('add-font-close-btn');
    if (addFontCloseBtn) {
      addFontCloseBtn.addEventListener('click', closeAddFontDialog);
    }

    const addFontOverlay = document.getElementById('add-font-overlay');
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
    const panel = document.getElementById('reader-settings-panel');
    const btn = document.getElementById('btn-reader-settings');
    if (panel && btn) {
      // 渲染主题按钮
      if (typeof ThemeModule !== 'undefined') ThemeModule.renderThemeButtons();
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
    const panel = document.getElementById('reader-settings-panel');
    const btn = document.getElementById('btn-reader-settings');
    if (panel && btn) {
      panel.classList.add('hidden');
      btn.classList.remove('active');
      panelVisible = false;
    }
  }

  // ==================== 字体选择 ====================
  function renderFontOptions() {
    const select = document.getElementById('font-family-select');
    if (!select) return;

    const currentValue = localStorage.getItem(FONT_FAMILY_KEY) || '默认无衬线';
    select.innerHTML = '';

    FONT_OPTIONS.forEach(function(font) {
      const option = document.createElement('option');
      option.value = font.name;
      option.textContent = font.name;
      option.style.fontFamily = font.family;
      if (font.name === currentValue) option.selected = true;
      select.appendChild(option);
    });

    // 自定义字体选项
    customFonts.forEach(function(cf) {
      const option = document.createElement('option');
      option.value = cf.name;
      option.textContent = cf.name + ' (自定义)';
      option.style.fontFamily = '"' + cf.name + '"';
      if (cf.name === currentValue) option.selected = true;
      select.appendChild(option);
    });
  }

  function applyFontFamily(fontName) {
    let fontFamily = null;

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

  // ==================== IndexedDB 工具 ====================
  function openFontDB() {
    return new Promise(function(resolve, reject) {
      var req = indexedDB.open(FONT_DB_NAME, FONT_DB_VERSION);
      req.onupgradeneeded = function(e) {
        var db = e.target.result;
        if (!db.objectStoreNames.contains(FONT_STORE_NAME)) {
          db.createObjectStore(FONT_STORE_NAME);
        }
      };
      req.onsuccess = function() { resolve(req.result); };
      req.onerror = function() { reject(req.error); };
    });
  }

  function fontDBPut(name, blob) {
    return openFontDB().then(function(db) {
      return new Promise(function(resolve, reject) {
        var tx = db.transaction(FONT_STORE_NAME, 'readwrite');
        tx.objectStore(FONT_STORE_NAME).put(blob, name);
        tx.oncomplete = function() { resolve(); };
        tx.onerror = function() { reject(tx.error); };
      });
    });
  }

  function fontDBGet(name) {
    return openFontDB().then(function(db) {
      return new Promise(function(resolve, reject) {
        var tx = db.transaction(FONT_STORE_NAME, 'readonly');
        var req = tx.objectStore(FONT_STORE_NAME).get(name);
        req.onsuccess = function() { resolve(req.result); };
        req.onerror = function() { reject(req.error); };
      });
    });
  }

  function fontDBDelete(name) {
    return openFontDB().then(function(db) {
      return new Promise(function(resolve, reject) {
        var tx = db.transaction(FONT_STORE_NAME, 'readwrite');
        tx.objectStore(FONT_STORE_NAME).delete(name);
        tx.oncomplete = function() { resolve(); };
        tx.onerror = function() { reject(tx.error); };
      });
    });
  }

  // ==================== 自定义字体 ====================
  function loadCustomFonts() {
    try {
      customFonts = JSON.parse(localStorage.getItem(CUSTOM_FONT_META_KEY) || '[]');
    } catch (e) {
      customFonts = [];
    }

    // 从 IndexedDB 加载所有字体 blob 并注册
    customFonts.forEach(function(cf) {
      fontDBGet(cf.name).then(function(blob) {
        if (blob) registerFontFaceFromBlob(cf.name, blob);
      }).catch(function() {});
    });
  }

  function registerFontFaceFromBlob(name, blob) {
    try {
      // 先释放旧的 blob URL，防止内存泄漏
      if (fontBlobUrls.has(name)) {
        URL.revokeObjectURL(fontBlobUrls.get(name));
      }
      const url = URL.createObjectURL(blob);
      fontBlobUrls.set(name, url);
      const font = new FontFace(name, 'url(' + url + ')');
      font.load().then(function(loaded) {
        document.fonts.add(loaded);
      }).catch(function(err) {
        console.warn('字体加载失败:', name, err.message || err);
      });
    } catch (e) {
      console.warn('FontFace API 不可用:', e);
    }
  }

  function addCustomFontFromBlob(name, blob) {
    var exists = customFonts.some(function(cf) { return cf.name === name; });
    if (exists) return Promise.resolve(false);

    customFonts.push({ name: name });
    localStorage.setItem(CUSTOM_FONT_META_KEY, JSON.stringify(customFonts));
    return fontDBPut(name, blob).then(function() {
      registerFontFaceFromBlob(name, blob);
      return true;
    });
  }

  function removeCustomFont(name) {
    const idx = customFonts.findIndex(function(cf) { return cf.name === name; });
    if (idx === -1) return false;

    customFonts.splice(idx, 1);
    localStorage.setItem(CUSTOM_FONT_META_KEY, JSON.stringify(customFonts));
    fontDBDelete(name).catch(function() {});

    // 释放 blob URL，防止内存泄漏
    if (fontBlobUrls.has(name)) {
      URL.revokeObjectURL(fontBlobUrls.get(name));
      fontBlobUrls.delete(name);
    }

    // 如果当前正在使用该字体，回退到默认
    const current = localStorage.getItem(FONT_FAMILY_KEY);
    if (current === name) {
      applyFontFamily('默认无衬线');
      localStorage.setItem(FONT_FAMILY_KEY, '默认无衬线');
    }

    return true;
  }

  function renderCustomFontList() {
    const container = document.getElementById('custom-font-list');
    if (!container) return;

    container.innerHTML = '';

    customFonts.forEach(function(cf) {
      const item = document.createElement('div');
      item.className = 'custom-font-item';

      const nameSpan = document.createElement('span');
      nameSpan.className = 'custom-font-item-name';
      nameSpan.textContent = cf.name;
      nameSpan.style.fontFamily = '"' + cf.name + '"';
      item.appendChild(nameSpan);

      const deleteBtn = document.createElement('button');
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
    const overlay = document.getElementById('add-font-overlay');
    if (!overlay) return;

    const nameInput = document.getElementById('add-font-name');
    const fileInput = document.getElementById('add-font-file');
    const errorDiv = document.getElementById('add-font-error');

    if (nameInput) nameInput.value = '';
    if (fileInput) fileInput.value = '';
    if (errorDiv) errorDiv.classList.remove('visible');

    overlay.classList.remove('hidden');
    addFontDialogOpen = true;
  }

  function closeAddFontDialog() {
    const overlay = document.getElementById('add-font-overlay');
    if (overlay) overlay.classList.add('hidden');
    addFontDialogOpen = false;
  }

  function handleAddFont() {
    const nameInput = document.getElementById('add-font-name');
    const fileInput = document.getElementById('add-font-file');
    const errorDiv = document.getElementById('add-font-error');

    const file = fileInput && fileInput.files ? fileInput.files[0] : null;
    let name = nameInput ? nameInput.value.trim() : '';

    // 校验文件
    if (!file) {
      if (errorDiv) {
        errorDiv.textContent = '请选择字体文件';
        errorDiv.classList.add('visible');
      }
      return;
    }

    const validTypes = ['font/woff2', 'font/woff', 'font/ttf', 'font/otf',
                        'application/font-woff2', 'application/font-woff',
                        'application/x-font-ttf', 'application/x-font-opentype'];
    const validExt = /\.(woff2|woff|ttf|otf)$/i;
    if (!validTypes.includes(file.type) && !validExt.test(file.name)) {
      if (errorDiv) {
        errorDiv.textContent = '仅支持 woff2/woff/ttf/otf 格式的字体文件';
        errorDiv.classList.add('visible');
      }
      return;
    }

    // 名称：优先用户输入，否则取文件名（去掉扩展名）
    if (!name) {
      name = file.name.replace(/\.(woff2|woff|ttf|otf)$/i, '');
    }

    // 检查重名
    const exists = customFonts.some(function(cf) { return cf.name === name; });
    if (exists) {
      if (errorDiv) {
        errorDiv.textContent = '字体名称已存在';
        errorDiv.classList.add('visible');
      }
      return;
    }

    if (errorDiv) errorDiv.classList.remove('visible');

    addCustomFontFromBlob(name, file).then(function(ok) {
      if (ok) {
        closeAddFontDialog();
        renderCustomFontList();
        renderFontOptions();
      } else if (errorDiv) {
        errorDiv.textContent = '添加失败，请重试';
        errorDiv.classList.add('visible');
      }
    }).catch(function(err) {
      if (errorDiv) {
        errorDiv.textContent = '添加失败: ' + (err.message || err);
        errorDiv.classList.add('visible');
      }
    });
  }

  // ==================== 间距调节 ====================
  function bindSlider(sliderId, valueId, onChange, formatValue) {
    const slider = document.getElementById(sliderId);
    const valueDisplay = document.getElementById(valueId);
    if (!slider) return;

    slider.addEventListener('input', function() {
      const val = parseFloat(this.value);
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
    const existingOverlay = document.getElementById('custom-confirm-overlay');
    if (existingOverlay) existingOverlay.remove();

    const overlay = document.createElement('div');
    overlay.id = 'custom-confirm-overlay';
    overlay.className = 'theme-editor-overlay';
    overlay.style.display = 'flex';
    overlay.style.alignItems = 'center';
    overlay.style.justifyContent = 'center';

    const dialog = document.createElement('div');
    dialog.className = 'theme-editor';
    dialog.style.width = '360px';
    dialog.style.textAlign = 'center';

    const body = document.createElement('div');
    body.className = 'theme-editor-body';
    body.style.padding = '32px 24px';

    const msg = document.createElement('p');
    msg.style.margin = '0 0 24px';
    msg.style.color = 'var(--text-primary)';
    msg.style.fontSize = '14px';
    msg.style.lineHeight = '1.6';
    msg.textContent = message;
    body.appendChild(msg);

    const btnRow = document.createElement('div');
    btnRow.style.display = 'flex';
    btnRow.style.justifyContent = 'center';
    btnRow.style.gap = '12px';

    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn-secondary';
    cancelBtn.textContent = '取消';
    cancelBtn.addEventListener('click', function() { overlay.remove(); });
    btnRow.appendChild(cancelBtn);

    const confirmBtn = document.createElement('button');
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
    const escHandler = function(e) {
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
    const keysToRemove = [];
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.startsWith('voidtext-')) {
        keysToRemove.push(key);
      }
    }
    keysToRemove.forEach(function(key) {
      localStorage.removeItem(key);
    });

    // 清除 IndexedDB 中的字体文件并释放 blob URL
    customFonts.forEach(function(cf) {
      fontDBDelete(cf.name).catch(function() {});
      if (fontBlobUrls.has(cf.name)) {
        URL.revokeObjectURL(fontBlobUrls.get(cf.name));
      }
    });
    fontBlobUrls.clear();

    // 重置主题
    if (typeof ThemeModule !== 'undefined') ThemeModule.applyTheme('light');

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
      if (typeof ThemeModule !== 'undefined') ThemeModule.renderThemeButtons();
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
