// 应用配置
const AppConfig = {
  // API基础URL
  apiBaseUrl: '/api',
  
  // 认证token（从localStorage或环境变量获取）
  getAuthToken: function() {
    // 优先从localStorage获取
    const token = localStorage.getItem('voidtext_auth_token');
    if (token) {
      return token;
    }
    
    // 如果localStorage中没有，检查是否有环境变量配置
    // 注意：在生产环境中，这应该从安全的配置源获取
    return '';
  },
  
  // 认证配置（从后端获取）
  authConfig: {
    enabled: null  // null表示尚未获取，true/false表示后端配置
  },
  
  // 从后端获取认证配置
  fetchAuthConfig: async function() {
    try {
      const response = await fetch('/health');
      if (response.ok) {
        const data = await response.json();
        if (data.services && data.services.authentication) {
          this.authConfig.enabled = data.services.authentication.status === 'enabled';
        }
      }
    } catch (error) {
      console.error('获取认证配置失败:', error);
    }
    return this.authConfig.enabled;
  },
  
  // 是否启用认证
  isAuthEnabled: function() {
    // 如果已获取后端配置，使用后端配置
    if (this.authConfig.enabled !== null) {
      return this.authConfig.enabled;
    }
    // 否则默认启用认证（安全起见）
    return true;
  },
  
  // 设置认证token
  setAuthToken: function(token) {
    if (token && token.length > 0) {
      localStorage.setItem('voidtext_auth_token', token);
      return true;
    }
    return false;
  },
  
  // 清除认证token
  clearAuthToken: function() {
    localStorage.removeItem('voidtext_auth_token');
  },
  
  // 获取请求头
  getHeaders: function(contentType = 'application/json') {
    const headers = {
      'Content-Type': contentType
    };
    
    const token = this.getAuthToken();
    if (token) {
      headers['X-API-Token'] = token;
    }
    
    return headers;
  },
  
  // 统一的API请求函数
  apiRequest: async function(url, options = {}) {
    const defaultOptions = {
      headers: this.getHeaders(),
      credentials: 'same-origin'
    };
    
    const mergedOptions = {
      ...defaultOptions,
      ...options,
      headers: {
        ...defaultOptions.headers,
        ...options.headers
      }
    };
    
    try {
      const response = await fetch(`${this.apiBaseUrl}${url}`, mergedOptions);
      
      // 处理认证错误
      if (response.status === 401) {
        this.clearAuthToken();
        showFeedback('认证失败，请重新登录', 'error');
        // TODO: 跳转到登录页面或显示登录弹窗
        throw new Error('认证失败');
      }
      
      // 处理其他错误
      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }
      
      return await response.json();
    } catch (error) {
      console.error('API请求失败:', error);
      throw error;
    }
  },
  
  // 上传文件的特殊处理
  uploadFile: async function(file, onProgress) {
    const formData = new FormData();
    formData.append('file', file);
    
    const token = this.getAuthToken();
    const headers = {};
    if (token) {
      headers['X-API-Token'] = token;
    }
    
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      
      // 进度事件
      if (onProgress) {
        xhr.upload.addEventListener('progress', (e) => {
          if (e.lengthComputable) {
            onProgress(Math.round((e.loaded / e.total) * 100));
          }
        });
      }
      
      // 完成事件
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText));
          } catch (error) {
            reject(new Error('响应解析失败'));
          }
        } else {
          if (xhr.status === 401) {
            this.clearAuthToken();
            showFeedback('认证失败，请重新登录', 'error');
          }
          reject(new Error(`上传失败: ${xhr.status} ${xhr.statusText}`));
        }
      });
      
      // 错误事件
      xhr.addEventListener('error', () => {
        reject(new Error('网络错误'));
      });
      
      xhr.open('POST', `${this.apiBaseUrl}/files/upload`);
      for (const [key, value] of Object.entries(headers)) {
        xhr.setRequestHeader(key, value);
      }
      xhr.send(formData);
    });
  }
};

// 导出配置
if (typeof module !== 'undefined' && module.exports) {
  module.exports = AppConfig;
}