// 主应用模块
const AppModule = (function() {
  // 初始化应用
  async function init() {
    console.log('初始化应用模块...');

    // 初始化所有模块
    FileManager.init();
    ProcessingModule.init();
    ReviewModule.init();
    RulesConfigModule.init();

    // 设置DomUtils回调
    setupDomUtilsCallbacks();

    // 显示文件列表
    FileManager.showSection('file-list');

    console.log('应用模块初始化完成');
  }

  // 设置DomUtils回调
  function setupDomUtilsCallbacks() {
    DomUtils.deleteFile = FileManager.deleteFile;
    DomUtils.configureRules = RulesConfigModule.configureRules;
    DomUtils.viewProgress = ProcessingModule.viewProgress;
    DomUtils.resumeFile = FileManager.resumeFile;
    DomUtils.downloadFile = FileManager.downloadFile;
    DomUtils.viewReport = FileManager.viewReport;
    DomUtils.reprocessFile = FileManager.reprocessFile;
  }

  // 公共API
  return {
    init
  };
})();

// 页面加载完成后初始化应用
document.addEventListener('DOMContentLoaded', function() {
  console.log('DOM加载完成，开始初始化应用...');

  // 初始化应用
  AppModule.init();

  console.log('应用初始化完成');
});

// 导出到全局作用域
window.AppModule = AppModule;
