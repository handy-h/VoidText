// 规则配置模块
const RulesConfigModule = (function() {
  // 私有变量
  let currentFileMd5 = null;
  
  // 初始化
  function init() {
    // 绑定事件
    bindEvents();
  }
  
  // 绑定事件
  function bindEvents() {
    // 保存规则按钮
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
  
  // 配置规则
  function configureRules(md5) {
    currentFileMd5 = md5;
    
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
          if (file.title) parts.push(`标题: ${file.title}`);
          if (file.author) parts.push(`作者: ${file.author}`);
          if (file.fileSize) parts.push(`大小: ${formatFileSize(file.fileSize)}`);
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
      
      // 设置复选框状态
      const basicCleaning = document.getElementById('rule-basic-cleaning');
      const traditionalSimple = document.getElementById('rule-traditional-simple');
      const vectorDetection = document.getElementById('rule-vector-detection');
      const modelRepair = document.getElementById('rule-model-repair');
      
      if (basicCleaning) basicCleaning.checked = rulesConfig.enableBasicCleaning !== false;
      if (traditionalSimple) traditionalSimple.checked = rulesConfig.traditionalToSimple === true;
      if (vectorDetection) vectorDetection.checked = rulesConfig.enableVectorDetection !== false;
      if (modelRepair) modelRepair.checked = rulesConfig.enableModelRepair !== false;
      
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
      console.error('解析规则配置失败:', err);
    }
  }
  
  // 保存规则并开始处理
  function saveRulesAndProcess() {
    if (!currentFileMd5) return;
    
    const rulesConfig = {
      enableBasicCleaning: document.getElementById('rule-basic-cleaning').checked,
      traditionalToSimple: document.getElementById('rule-traditional-simple').checked,
      enableVectorDetection: document.getElementById('rule-vector-detection').checked,
      vectorSimilarityThreshold: parseFloat(document.getElementById('rule-similarity').value) || 0.95,
      enableModelRepair: document.getElementById('rule-model-repair').checked,
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
    if (!currentFileMd5) return;
    
    AppConfig.apiRequest(`/files/${currentFileMd5}/run`, { method: 'POST' })
      .then((data) => {
        if (data.success) {
          FileManager.showSection('processing');
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
  
  // 格式化文件大小
  function formatFileSize(bytes) {
    const kb = 1024;
    const mb = kb * 1024;
    const gb = mb * 1024;
    switch (true) {
      case bytes >= gb:
        return (bytes / gb).toFixed(1) + "GB";
      case bytes >= mb:
        return (bytes / mb).toFixed(1) + "MB";
      case bytes >= kb:
        return (bytes / kb).toFixed(1) + "KB";
      default:
        return bytes + "B";
    }
  }
  
  // 显示反馈消息
  function showFeedback(message, type) {
    const fb = document.getElementById("feedback");
    if (!fb) return;
    
    fb.textContent = message;
    fb.className = "feedback " + (type || "success");
    fb.style.display = "block";
    setTimeout(() => {
      fb.style.display = "none";
    }, 3000);
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