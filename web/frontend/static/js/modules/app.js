// 主应用模块
const AppModule = (function() {
  async function init() {
    ThemeModule.init();
    ReaderSettingsModule.init();

    FileManager.init();
    ProcessingModule.init();
    ReviewModule.init();
    RulesConfigModule.init();

    setupDomUtilsCallbacks();

    FileManager.showSection('file-list');
  }

  function setupDomUtilsCallbacks() {
    DomUtils.deleteFile = FileManager.deleteFile;
    DomUtils.configureRules = RulesConfigModule.configureRules;
    DomUtils.viewProgress = ProcessingModule.viewProgress;
    DomUtils.resumeFile = FileManager.resumeFile;
    DomUtils.downloadFile = FileManager.downloadFile;
    DomUtils.viewReport = FileManager.viewReport;
    DomUtils.reprocessFile = FileManager.reprocessFile;
  }

  return { init };
})();

document.addEventListener('DOMContentLoaded', function() {
  AppModule.init().catch(err => console.error('应用初始化失败:', err));
});

window.AppModule = AppModule;
