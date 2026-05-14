// 应用配置
const AppConfig = {
  // API基础URL
  apiBaseUrl: '/api',

  // 获取请求头
  getHeaders: function(contentType = 'application/json') {
    return {
      'Content-Type': contentType
    };
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
          reject(new Error(`上传失败: ${xhr.status} ${xhr.statusText}`));
        }
      });

      // 错误事件
      xhr.addEventListener('error', () => {
        reject(new Error('网络错误'));
      });

      xhr.open('POST', `${this.apiBaseUrl}/files/upload`);
      xhr.send(formData);
    });
  }
};

// 导出配置
if (typeof module !== 'undefined' && module.exports) {
  module.exports = AppConfig;
}
