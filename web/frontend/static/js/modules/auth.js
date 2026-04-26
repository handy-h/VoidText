// 认证模块
const AuthModule = (function() {
  // 私有变量
  let authToken = null;
  let authEnabled = false;
  
  // 初始化认证
  function init() {
    authEnabled = AppConfig.isAuthEnabled();
    if (authEnabled) {
      authToken = AppConfig.getAuthToken();
      updateAuthUI();
    }
  }
  
  // 更新认证UI
  function updateAuthUI() {
    const app = document.getElementById('app');
    const authSection = document.getElementById('auth-section');
    
    if (!app || !authSection) return;
    
    if (isAuthenticated()) {
      app.style.display = 'block';
      authSection.style.display = 'none';
    } else {
      app.style.display = 'none';
      authSection.style.display = 'block';
    }
  }
  
  // 检查是否已认证
  function isAuthenticated() {
    if (!authEnabled) return true;
    return !!authToken;
  }
  
  // 登录
  function login(token) {
    if (!authEnabled) return true;
    
    const trimmedToken = token.trim();
    if (!trimmedToken) {
      showFeedback('请输入认证token', 'error');
      return false;
    }
    
    if (AppConfig.setAuthToken(trimmedToken)) {
      authToken = trimmedToken;
      updateAuthUI();
      showFeedback('登录成功', 'success');
      return true;
    } else {
      showFeedback('token无效', 'error');
      return false;
    }
  }
  
  // 登出
  function logout() {
    if (!authEnabled) return;
    
    AppConfig.clearAuthToken();
    authToken = null;
    updateAuthUI();
    showFeedback('已登出', 'success');
  }
  
  // 获取认证token
  function getToken() {
    return authToken;
  }
  
  // 检查认证状态
  function checkAuthStatus() {
    if (!authEnabled) return;
    
    if (isAuthenticated()) {
      document.getElementById('app').style.display = 'block';
      document.getElementById('auth-section').style.display = 'none';
    } else {
      document.getElementById('app').style.display = 'none';
      document.getElementById('auth-section').style.display = 'block';
    }
  }
  
  // 显示反馈消息
  function showFeedback(message, type) {
    const fb = document.getElementById('feedback');
    if (!fb) return;
    
    fb.textContent = message;
    fb.className = 'feedback ' + (type || 'success');
    fb.style.display = 'block';
    setTimeout(() => {
      fb.style.display = 'none';
    }, 3000);
  }
  
  // 公共API
  return {
    init,
    login,
    logout,
    isAuthenticated,
    getToken,
    checkAuthStatus
  };
})();

// 导出到全局作用域
window.AuthModule = AuthModule;