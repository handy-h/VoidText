// 规则配置模块
const RulesConfigModule = (function() {
  // 私有变量
  let currentFileMd5 = null;

  // 初始化
  function init() {
    bindEvents();
  }

  // 绑定事件
  function bindEvents() {
    // 下载当前版本按钮
    const downloadCurrentBtn = document.getElementById('download-current-btn');
    if (downloadCurrentBtn) {
      downloadCurrentBtn.addEventListener('click', downloadCurrentVersion);
    }
  }

  // 配置规则
  function configureRules(md5) {
    currentFileMd5 = md5;
    window.currentFileMd5 = md5;

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
          if (file.title) parts.push('标题: ' + file.title);
          if (file.author) parts.push('作者: ' + file.author);
          if (file.fileSize) parts.push('大小: ' + DomUtils.formatFileSize(file.fileSize));
          DomUtils.setTextContent(infoDiv, parts.join(" | "));
        }

        // 加载规则配置
        loadRulesConfig(file.rulesConfig);

        FileManager.showSection("rules-config");
      })
      .catch((err) => showFeedback("获取文件信息失败: " + err.message, "error"));
  }

  // 加载规则配置
  function loadRulesConfig(rulesConfigStr) {
    try {
      const rulesConfig = rulesConfigStr ? JSON.parse(rulesConfigStr) : {};

      // 基础清洗开关
      const basicCleaning = document.getElementById('rule-basic-cleaning');
      if (basicCleaning) basicCleaning.checked = rulesConfig.enableBasicCleaning !== false;

      // 广告清洗
      const adCleaning = document.getElementById('rule-ad-cleaning');
      if (adCleaning) adCleaning.checked = rulesConfig.enableAdCleaning !== false;

      // 繁简转换
      const traditionalSimple = document.getElementById('rule-traditional-simple');
      if (traditionalSimple) traditionalSimple.checked = rulesConfig.traditionalToSimple === true;

      const vectorDetection = document.getElementById('rule-vector-detection');
      const paragraphReconstruct = document.getElementById('rule-paragraph-reconstruct');
      const modelRepair = document.getElementById('rule-model-repair');

      if (vectorDetection) vectorDetection.checked = rulesConfig.enableVectorDetection !== false;
      if (paragraphReconstruct) paragraphReconstruct.checked = rulesConfig.enableParagraphReconstruct !== false;
      if (modelRepair) modelRepair.checked = rulesConfig.enableModelRepair !== false;

      // 相似度阈值
      const similarity = document.getElementById('rule-similarity');
      if (similarity) similarity.value = rulesConfig.similarityThreshold || 0.95;

      // 广告黑名单
      const adBlacklist = document.getElementById('rule-ad-blacklist');
      if (adBlacklist && rulesConfig.adBlacklist) {
        adBlacklist.value = Array.isArray(rulesConfig.adBlacklist)
          ? rulesConfig.adBlacklist.join('\n')
          : rulesConfig.adBlacklist;
      }

      // 错别字映射
      const typoMap = document.getElementById('rule-typo-map');
      if (typoMap && rulesConfig.typoMap) {
        if (typeof rulesConfig.typoMap === 'object' && rulesConfig.typoMap !== null) {
          const entries = [];
          for (var key in rulesConfig.typoMap) {
            if (rulesConfig.typoMap.hasOwnProperty(key)) {
              entries.push(key + '=' + rulesConfig.typoMap[key]);
            }
          }
          typoMap.value = entries.join('\n');
        } else {
          typoMap.value = String(rulesConfig.typoMap);
        }
      }
    } catch (err) {
      console.error('解析规则配置失败:', err);
    }
  }

  // 保存规则并开始处理
  function saveRulesAndProcess() {
    if (!currentFileMd5) currentFileMd5 = window.currentFileMd5 || null;
    if (!currentFileMd5) return;

    var elBasicCleaning = document.getElementById('rule-basic-cleaning');
    var elAdCleaning = document.getElementById('rule-ad-cleaning');
    var elTraditionalSimple = document.getElementById('rule-traditional-simple');
    var elVectorDetection = document.getElementById('rule-vector-detection');
    var elSimilarity = document.getElementById('rule-similarity');
    var elParagraphReconstruct = document.getElementById('rule-paragraph-reconstruct');
    var elModelRepair = document.getElementById('rule-model-repair');
    var elTypoMap = document.getElementById('rule-typo-map');
    var elAdBlacklist = document.getElementById('rule-ad-blacklist');

    // 解析错别字映射
    var typoMapObj = {};
    if (elTypoMap && elTypoMap.value) {
      var lines = elTypoMap.value.split('\n');
      for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line) continue;
        var idx = line.indexOf('=');
        if (idx > 0) {
          typoMapObj[line.substring(0, idx).trim()] = line.substring(idx + 1).trim();
        }
      }
    }

    // 解析广告黑名单（按行分割为数组）
    var adBlacklistArr = [];
    if (elAdBlacklist && elAdBlacklist.value) {
      var blLines = elAdBlacklist.value.split('\n');
      for (var j = 0; j < blLines.length; j++) {
        var blLine = blLines[j].trim();
        if (blLine) adBlacklistArr.push(blLine);
      }
    }

    const rulesConfig = {
      enableBasicCleaning: elBasicCleaning ? elBasicCleaning.checked : true,
      enableAdCleaning: elAdCleaning ? elAdCleaning.checked : true,
      traditionalToSimple: elTraditionalSimple ? elTraditionalSimple.checked : false,
      enableVectorDetection: elVectorDetection ? elVectorDetection.checked : true,
      similarityThreshold: (elSimilarity ? parseFloat(elSimilarity.value) : 0) || 0.95,
      enableModelRepair: elModelRepair ? elModelRepair.checked : true,
      enableParagraphReconstruct: elParagraphReconstruct ? elParagraphReconstruct.checked : true,
      typoMap: typoMapObj,
      adBlacklist: adBlacklistArr
    };

    AppConfig.apiRequest(`/files/${currentFileMd5}/rules`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rulesConfig: JSON.stringify(rulesConfig) })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('规则已保存', 'success');
          startProcessing();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('保存规则失败: ' + err.message, 'error');
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
        showFeedback('启动处理失败: ' + err.message, 'error');
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
    downloadCurrentVersion
  };
})();

// 导出到全局作用域
window.RulesConfigModule = RulesConfigModule;
