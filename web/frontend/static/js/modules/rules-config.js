// 规则配置模块
const RulesConfigModule = (function() {
  // 私有变量
  let currentFileMd5 = null;

  // 预设模板定义
  const PRESETS = {
    basic: {
      name: '基础清洗模式',
      description: '标准清洗流程，适用于大多数小说文本',
      rules: {
        enableBasicCleaning: true,
        traditionalToSimple: false,
        enableVectorDetection: true,
        vectorSimilarityThreshold: 0.95,
        enableModelRepair: true,
        enableNewlineFix: true,
        typoMap: '',
        adBlacklist: ''
      }
    },
    deep: {
      name: '深度修复模式',
      description: '全面修复模式，包含繁转简、LLM智能修复、向量检测',
      rules: {
        enableBasicCleaning: true,
        traditionalToSimple: true,
        enableVectorDetection: true,
        vectorSimilarityThreshold: 0.90,
        enableModelRepair: true,
        enableNewlineFix: true,
        typoMap: '',
        adBlacklist: ''
      }
    },
    minimal: {
      name: '仅清洗',
      description: '仅执行基础清洗和广告过滤，不启用AI检测和修复',
      rules: {
        enableBasicCleaning: true,
        traditionalToSimple: false,
        enableVectorDetection: false,
        vectorSimilarityThreshold: 0.95,
        enableModelRepair: false,
        enableNewlineFix: true,
        typoMap: '',
        adBlacklist: ''
      }
    }
  };

  // 初始化
  function init() {
    bindEvents();

    // 高亮当前激活的预设
    const presets = document.querySelectorAll('.preset-btn');
    presets.forEach(btn => {
      btn.addEventListener('click', function() {
        presets.forEach(b => b.classList.remove('active'));
        this.classList.add('active');
      });
    });
  }

  // 绑定事件
  function bindEvents() {
    // 保存规则按钮（保留对 ID 的兼容）
    const saveRulesBtn = document.getElementById('save-rules-btn');
    if (saveRulesBtn) {
      saveRulesBtn.addEventListener('click', saveRulesAndProcess);
    }

    // 下载当前版本按钮
    const downloadCurrentBtn = document.getElementById('download-current-btn');
    if (downloadCurrentBtn) {
      downloadCurrentBtn.addEventListener('click', downloadCurrentVersion);
    }
  }

  // 应用预设模板
  function applyPreset(presetName) {
    const preset = PRESETS[presetName];
    if (!preset) return;

    const rules = preset.rules;

    // 设置复选框状态
    const basicCleaning = document.getElementById('rule-basic-cleaning');
    const traditionalSimple = document.getElementById('rule-traditional-simple');
    const vectorDetection = document.getElementById('rule-vector-detection');
    const modelRepair = document.getElementById('rule-model-repair');
    const newlineFix = document.getElementById('rule-newline-fix');

    if (basicCleaning) basicCleaning.checked = rules.enableBasicCleaning;
    if (traditionalSimple) traditionalSimple.checked = rules.traditionalToSimple;
    if (vectorDetection) vectorDetection.checked = rules.enableVectorDetection;
    if (modelRepair) modelRepair.checked = rules.enableModelRepair;
    if (newlineFix) newlineFix.checked = rules.enableNewlineFix !== false;

    // 设置数值
    const similarity = document.getElementById('rule-similarity');
    if (similarity) similarity.value = rules.vectorSimilarityThreshold;

    // 清空文本域（预设不包含自定义规则）
    const typoMap = document.getElementById('rule-typo-map');
    const adBlacklist = document.getElementById('rule-ad-blacklist');
    if (typoMap) typoMap.value = '';
    if (adBlacklist) adBlacklist.value = '';

    // 高亮预设按钮
    document.querySelectorAll('.preset-btn').forEach(btn => {
      btn.classList.toggle('active', btn.dataset.preset === presetName);
    });

    showFeedback('\u5DF2\u5E94\u7528\uFF1A' + preset.name, 'success');
  }

  // 配置规则
  function configureRules(md5) {
    currentFileMd5 = md5;
    window.currentFileMd5 = md5;

    // 重置预设高亮
    document.querySelectorAll('.preset-btn').forEach(b => b.classList.remove('active'));

    AppConfig.apiRequest(`/files/${md5}`)
      .then((data) => {
        if (!data.success) {
          showFeedback(data.message, "error");
          return;
        }

        const file = data.file;
        const infoDiv = document.getElementById("file-info-display");
        if (infoDiv) {
          const parts = [];
          if (file.title) parts.push('\u6807\u9898: ' + file.title);
          if (file.author) parts.push('\u4F5C\u8005: ' + file.author);
          if (file.fileSize) parts.push('\u5927\u5C0F: ' + DomUtils.formatFileSize(file.fileSize));
          DomUtils.setTextContent(infoDiv, parts.join(" | "));
        }

        // 加载规则配置
        loadRulesConfig(file.rulesConfig);

        FileManager.showSection("rules-config");
      })
      .catch((err) => showFeedback("\u83B7\u53D6\u6587\u4EF6\u4FE1\u606F\u5931\u8D25: " + err.message, "error"));
  }

  // 加载规则配置
  function loadRulesConfig(rulesConfigStr) {
    try {
      const rulesConfig = rulesConfigStr ? JSON.parse(rulesConfigStr) : {};

      // 设置复选框状态
      const basicCleaning = document.getElementById('rule-basic-cleaning');
      const traditionalSimple = document.getElementById('rule-traditional-simple');
      const vectorDetection = document.getElementById('rule-vector-detection');
      const modelRepair = document.getElementById('rule-model-repair');
      const newlineFix = document.getElementById('rule-newline-fix');

      if (basicCleaning) basicCleaning.checked = rulesConfig.enableBasicCleaning !== false;
      if (traditionalSimple) traditionalSimple.checked = rulesConfig.traditionalToSimple === true;
      if (vectorDetection) vectorDetection.checked = rulesConfig.enableVectorDetection !== false;
      if (modelRepair) modelRepair.checked = rulesConfig.enableModelRepair !== false;
      if (newlineFix) newlineFix.checked = rulesConfig.enableNewlineFix !== false;

      // 设置数值输入
      const similarity = document.getElementById('rule-similarity');
      if (similarity) similarity.value = rulesConfig.vectorSimilarityThreshold || 0.95;

      // 设置文本区域
      const typoMap = document.getElementById('rule-typo-map');
      const adBlacklist = document.getElementById('rule-ad-blacklist');

      if (typoMap && rulesConfig.typoMap) {
        typoMap.value = typeof rulesConfig.typoMap === 'string'
          ? rulesConfig.typoMap
          : JSON.stringify(rulesConfig.typoMap, null, 2);
      }

      if (adBlacklist && rulesConfig.adBlacklist) {
        adBlacklist.value = typeof rulesConfig.adBlacklist === 'string'
          ? rulesConfig.adBlacklist
          : JSON.stringify(rulesConfig.adBlacklist, null, 2);
      }
    } catch (err) {
      console.error('\u89E3\u6790\u89C4\u5219\u914D\u7F6E\u5931\u8D25:', err);
    }
  }

  // 保存规则并开始处理
  function saveRulesAndProcess() {
    if (!currentFileMd5) currentFileMd5 = window.currentFileMd5 || null;
    if (!currentFileMd5) return;

    const rulesConfig = {
      enableBasicCleaning: document.getElementById('rule-basic-cleaning').checked,
      traditionalToSimple: document.getElementById('rule-traditional-simple').checked,
      enableVectorDetection: document.getElementById('rule-vector-detection').checked,
      vectorSimilarityThreshold: parseFloat(document.getElementById('rule-similarity').value) || 0.95,
      enableModelRepair: document.getElementById('rule-model-repair').checked,
      enableNewlineFix: document.getElementById('rule-newline-fix').checked,
      typoMap: document.getElementById('rule-typo-map').value,
      adBlacklist: document.getElementById('rule-ad-blacklist').value
    };

    AppConfig.apiRequest(`/files/${currentFileMd5}/rules`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rulesConfig: JSON.stringify(rulesConfig) })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('\u89C4\u5219\u5DF2\u4FDD\u5B58', 'success');
          startProcessing();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('\u4FDD\u5B58\u89C4\u5219\u5931\u8D25: ' + err.message, 'error');
      });
  }

  // 开始处理
  function startProcessing() {
    if (!currentFileMd5) currentFileMd5 = window.currentFileMd5 || null;
    if (!currentFileMd5) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/run`, { method: 'POST' })
      .then((data) => {
        if (data.success) {
          FileManager.showSection('processing');
          ProcessingModule.setCurrentFileMd5(currentFileMd5);
          ProcessingModule.startPolling();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('\u542F\u52A8\u5904\u7406\u5931\u8D25: ' + err.message, 'error');
      });
  }

  // 下载当前版本
  function downloadCurrentVersion() {
    if (currentFileMd5) {
      FileManager.downloadFile(currentFileMd5);
    }
  }

  // 公共API
  return {
    init,
    configureRules,
    saveRulesAndProcess,
    startProcessing,
    downloadCurrentVersion,
    applyPreset
  };
})();

// 导出到全局作用域
window.RulesConfigModule = RulesConfigModule;
