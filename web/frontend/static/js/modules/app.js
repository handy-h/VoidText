// 主应用模块
const AppModule = (function() {
  // 初始化应用
  async function init() {
    console.log('初始化应用模块...');
    
    // 先从后端获取认证配置
    await AppConfig.fetchAuthConfig();
    console.log('认证配置:', AppConfig.authConfig);
    
    // 初始化所有模块
    AuthModule.init();
    FileManager.init();
    ProcessingModule.init();
    ReviewModule.init();
    RulesConfigModule.init();
    
    // 设置DomUtils回调
    setupDomUtilsCallbacks();
    
    // 检查认证状态
    checkAuthStatus();
    
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
  
  // 检查认证状态
  function checkAuthStatus() {
    if (AuthModule.isAuthenticated()) {
      // 已认证，可以正常访问
      document.getElementById('app').style.display = 'block';
      document.getElementById('auth-section').style.display = 'none';
    } else {
      // 未认证，显示登录界面
      document.getElementById('app').style.display = 'none';
      document.getElementById('auth-section').style.display = 'block';
    }
  }
  
  // 处理登录
  function handleLogin() {
    const tokenInput = document.getElementById('auth-token');
    const token = tokenInput ? tokenInput.value.trim() : '';
    
    if (AuthModule.login(token)) {
      checkAuthStatus();
    }
  }
  
  // 处理登出
  function handleLogout() {
    AuthModule.logout();
    checkAuthStatus();
  }
  
  // 绑定全局事件
  function bindGlobalEvents() {
    // 登录按钮
    const loginBtn = document.getElementById('login-btn');
    if (loginBtn) {
      loginBtn.addEventListener('click', handleLogin);
    }
    
    // 登出按钮
    const logoutBtn = document.getElementById('logout-btn');
    if (logoutBtn) {
      logoutBtn.addEventListener('click', handleLogout);
    }
    
    // 认证token输入框回车键
    const authTokenInput = document.getElementById('auth-token');
    if (authTokenInput) {
      authTokenInput.addEventListener('keypress', function(event) {
        if (event.key === 'Enter') {
          handleLogin();
        }
      });
    }
  }
  
  // 公共API
  return {
    init,
    checkAuthStatus,
    handleLogin,
    handleLogout,
    bindGlobalEvents
  };
})();

// 页面加载完成后初始化应用
document.addEventListener('DOMContentLoaded', function() {
  console.log('DOM加载完成，开始初始化应用...');
  
  // 绑定全局事件
  AppModule.bindGlobalEvents();
  
  // 初始化应用
  AppModule.init();
  
  console.log('应用初始化完成');
});

// 导出到全局作用域
window.AppModule = AppModule;