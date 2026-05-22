// DOM操作工具 - 安全的HTML生成和操作
const DomUtils = {
  // 安全地创建元素
  createElement: function(tagName, attributes = {}, children = []) {
    const element = document.createElement(tagName);
    
    // 设置属性
    for (const [key, value] of Object.entries(attributes)) {
      if (key.startsWith('on')) {
        // 事件处理器
        element[key] = value;
      } else if (key === 'className') {
        // class属性
        element.className = value;
      } else if (key === 'htmlFor') {
        // for属性
        element.setAttribute('for', value);
      } else if (key === 'style' && typeof value === 'object') {
        // 样式对象
        Object.assign(element.style, value);
      } else {
        // 其他属性
        element.setAttribute(key, value);
      }
    }
    
    // 添加子元素
    children.forEach(child => {
      if (typeof child === 'string') {
        element.appendChild(document.createTextNode(child));
      } else if (child instanceof Node) {
        element.appendChild(child);
      }
    });
    
    return element;
  },
  
  // 安全地设置元素内容
  setTextContent: function(element, text) {
    element.textContent = text;
  },
  
  // 安全地设置元素HTML（仅用于信任的内容）
  setHTML: function(element, html) {
    element.innerHTML = html;
  },
  
  // 安全地创建文件卡片
  createFileCard: function(file) {
    const card = this.createElement('div', {
      className: `file-card ${this.getStatusClass(file.status)}`
    });
    
    const fileInfo = this.createElement('div', { className: 'file-info' });
    
    // 文件标题
    const title = this.createElement('div', { className: 'file-title' });
    this.setTextContent(title, file.title || file.fileName);
    fileInfo.appendChild(title);
    
    // 文件元数据
    const meta = this.createElement('div', { className: 'file-meta' });
    
    if (file.author) {
      const authorSpan = this.createElement('span');
      this.setTextContent(authorSpan, `作者: ${file.author}`);
      meta.appendChild(authorSpan);
    }
    
    const statusSpan = this.createElement('span');
    this.setTextContent(statusSpan, `状态: `);
    const statusStrong = this.createElement('strong');
    this.setTextContent(statusStrong, this.getStatusText(file.status));
    statusSpan.appendChild(statusStrong);
    meta.appendChild(statusSpan);
    
    const stepSpan = this.createElement('span');
    this.setTextContent(stepSpan, `步骤: ${this.getStepText(file.currentStep)}`);
    meta.appendChild(stepSpan);
    
    const progressSpan = this.createElement('span');
    this.setTextContent(progressSpan, `进度: ${file.progress}%`);
    meta.appendChild(progressSpan);
    
    const timeSpan = this.createElement('span');
    this.setTextContent(timeSpan, `更新: ${this.formatTime(file.updatedAt)}`);
    meta.appendChild(timeSpan);
    
    fileInfo.appendChild(meta);
    
    // 错误信息
    if (file.errorMsg) {
      const errorDiv = this.createElement('div', { className: 'file-error' });
      this.setTextContent(errorDiv, `错误: ${file.errorMsg}`);
      fileInfo.appendChild(errorDiv);
    }

    // 进度条（仅处理中状态显示）
    if (file.status === 'processing' && file.progress !== undefined && file.progress !== null) {
      const progressContainer = this.createElement('div', { className: 'file-card-progress' });
      const progressBar = this.createElement('div', { className: 'file-card-progress-bar' });
      const progressFill = this.createElement('div', { className: 'file-card-progress-fill' });
      progressFill.style.width = file.progress + '%';
      progressBar.appendChild(progressFill);
      progressContainer.appendChild(progressBar);
      const progressText = this.createElement('span', { className: 'file-card-progress-text' });
      const stepText = this.getStepText(file.currentStep);
      this.setTextContent(progressText, stepText + '... ' + file.progress + '%');
      progressContainer.appendChild(progressText);
      fileInfo.appendChild(progressContainer);
    }
    
    card.appendChild(fileInfo);
    
    // 操作按钮
    const actions = this.createElement('div', { className: 'file-actions' });
    const actionButtons = this.getActionButtons(file);
    actionButtons.forEach(button => actions.appendChild(button));
    card.appendChild(actions);
    
    return card;
  },
  
  // 获取状态文本
  getStatusText: function(status) {
    const map = {
      pending: "待处理",
      processing: "处理中",
      reviewing: "审核中",
      completed: "已完成",
      failed: "失败",
    };
    return map[status] || status;
  },
  
  // 获取状态类名
  getStatusClass: function(status) {
    const map = {
      pending: "status-pending",
      processing: "status-processing",
      reviewing: "status-reviewing",
      completed: "status-completed",
      failed: "status-failed",
    };
    return map[status] || "";
  },
  
  // 获取步骤文本
  getStepText: function(step) {
    const map = {
      cleaning: "基础清洗",
      indexing: "向量检测",
      llm_fix: "LLM修复",
      review: "人工审核",
      finalizing: "生成文件",
    };
    return map[step] || step || "-";
  },
  
  // 格式化时间
  formatTime: function(ts) {
    if (!ts) return "";
    try {
      return new Date(ts).toLocaleString("zh-CN");
    } catch {
      return ts;
    }
  },
  
  // 获取操作按钮
  getActionButtons: function(file) {
    const buttons = [];
    const md5 = file.md5;
    const fileName = file.title || file.fileName;
    
    const deleteBtn = this.createElement('button', {
      className: 'btn-danger',
      onclick: () => this.deleteFile(md5, fileName)
    });
    this.setTextContent(deleteBtn, '删除');
    buttons.push(deleteBtn);
    
    switch (file.status) {
      case "pending":
        const configBtn = this.createElement('button', {
          className: 'btn-primary',
          onclick: () => this.configureRules(md5)
        });
        this.setTextContent(configBtn, '配置并处理');
        buttons.unshift(configBtn);
        break;
        
      case "processing":
        const viewBtn = this.createElement('button', {
          className: 'btn-primary',
          onclick: () => this.viewProgress(md5)
        });
        this.setTextContent(viewBtn, '查看进度');
        buttons.unshift(viewBtn);
        
        const resumeBtn = this.createElement('button', {
          className: 'btn-secondary',
          onclick: () => this.resumeFile(md5)
        });
        this.setTextContent(resumeBtn, '继续处理');
        buttons.splice(1, 0, resumeBtn);
        
        const downloadBtn = this.createElement('button', {
          className: 'btn-secondary',
          onclick: () => this.downloadFile(md5)
        });
        this.setTextContent(downloadBtn, '下载');
        buttons.splice(2, 0, downloadBtn);
        break;
        
      case "reviewing":
        const reviewViewBtn = this.createElement('button', {
          className: 'btn-primary',
          onclick: () => this.viewProgress(md5)
        });
        this.setTextContent(reviewViewBtn, '查看进度');
        buttons.unshift(reviewViewBtn);
        
        const reviewDownloadBtn = this.createElement('button', {
          className: 'btn-secondary',
          onclick: () => this.downloadFile(md5)
        });
        this.setTextContent(reviewDownloadBtn, '下载');
        buttons.splice(1, 0, reviewDownloadBtn);
        break;
        
      case "completed":
        const finalDownloadBtn = this.createElement('button', {
          className: 'btn-primary',
          onclick: () => this.downloadFile(md5)
        });
        this.setTextContent(finalDownloadBtn, '下载最终文件');
        buttons.unshift(finalDownloadBtn);
        
        const reportBtn = this.createElement('button', {
          className: 'btn-secondary',
          onclick: () => this.viewReport(md5)
        });
        this.setTextContent(reportBtn, '查看报告');
        buttons.splice(1, 0, reportBtn);
        
        const reprocessBtn = this.createElement('button', {
          className: 'btn-secondary',
          onclick: () => this.reprocessFile(md5)
        });
        this.setTextContent(reprocessBtn, '重新处理');
        buttons.splice(2, 0, reprocessBtn);
        break;
        
      case "failed":
        const resumeFailedBtn = this.createElement('button', {
          className: 'btn-primary',
          onclick: () => this.resumeFile(md5)
        });
        this.setTextContent(resumeFailedBtn, '从失败处恢复');
        buttons.unshift(resumeFailedBtn);
        break;
    }
    
    return buttons;
  },
  
  // 占位函数 - 需要在主文件中实现
  deleteFile: function(md5, fileName) {
    console.warn('deleteFile函数需要在主文件中实现');
  },
  
  configureRules: function(md5) {
    console.warn('configureRules函数需要在主文件中实现');
  },
  
  viewProgress: function(md5) {
    console.warn('viewProgress函数需要在主文件中实现');
  },
  
  resumeFile: function(md5) {
    console.warn('resumeFile函数需要在主文件中实现');
  },
  
  downloadFile: function(md5) {
    console.warn('downloadFile函数需要在主文件中实现');
  },
  
  viewReport: function(md5) {
    console.warn('viewReport函数需要在主文件中实现');
  },
  
  reprocessFile: function(md5) {
    console.warn('reprocessFile函数需要在主文件中实现');
  },
  
  // 显示 Toast 反馈
  showFeedback: function(message, type) {
    const fb = document.getElementById('feedback');
    if (!fb) return;
    fb.textContent = message;
    fb.className = 'feedback ' + (type || 'success');
    fb.style.display = 'block';
    setTimeout(() => { fb.style.display = 'none'; }, 3000);
  },

  // 格式化文件大小
  formatFileSize: function(bytes) {
    const kb = 1024, mb = kb * 1024, gb = mb * 1024;
    if (bytes >= gb) return (bytes / gb).toFixed(1) + 'GB';
    if (bytes >= mb) return (bytes / mb).toFixed(1) + 'MB';
    if (bytes >= kb) return (bytes / kb).toFixed(1) + 'KB';
    return bytes + 'B';
  },

  // 安全地创建上传结果显示
  createUploadResult: function(data) {
    const container = this.createElement('div');
    
    if (!data.success) {
      const errorDiv = this.createElement('div', { className: 'upload-error' });
      const errorMsg = this.createElement('p');
      this.setTextContent(errorMsg, `上传失败: ${data.message || "未知错误"}`);
      errorDiv.appendChild(errorMsg);
      
      const actions = this.createElement('div', { className: 'upload-actions' });
      const closeBtn = this.createElement('button', {
        className: 'btn-secondary',
        onclick: () => {
          const resultDiv = document.getElementById('upload-result');
          if (resultDiv) resultDiv.style.display = 'none';
        }
      });
      this.setTextContent(closeBtn, '关闭');
      actions.appendChild(closeBtn);
      errorDiv.appendChild(actions);
      
      container.appendChild(errorDiv);
      return container;
    }
    
    if (data.exists) {
      const existsDiv = this.createElement('div', { className: 'upload-exists' });
      
      const msg1 = this.createElement('p');
      this.setTextContent(msg1, data.message);
      existsDiv.appendChild(msg1);
      
      const msg2 = this.createElement('p');
      this.setTextContent(msg2, `当前状态: ${this.getStatusText(data.status)} ${data.currentStep ? "(" + this.getStepText(data.currentStep) + ")" : ""}`);
      existsDiv.appendChild(msg2);
      
      const actions = this.createElement('div', { className: 'upload-actions' });
      const continueBtn = this.createElement('button', {
        className: 'btn-primary',
        onclick: () => {
          currentFileMd5 = data.md5;
          this.resumeFile(data.md5);
        }
      });
      this.setTextContent(continueBtn, data.suggestion || "继续处理");
      actions.appendChild(continueBtn);
      
      const backBtn = this.createElement('button', {
        className: 'btn-secondary',
        onclick: () => showSection('file-list')
      });
      this.setTextContent(backBtn, '返回列表');
      actions.appendChild(backBtn);
      
      existsDiv.appendChild(actions);
      container.appendChild(existsDiv);
      
    } else if (data.isIntermediate) {
      const intermediateDiv = this.createElement('div', { className: 'upload-intermediate' });
      
      const msg1 = this.createElement('p');
      this.setTextContent(msg1, '检测到中间版本文件');
      intermediateDiv.appendChild(msg1);
      
      const msg2 = this.createElement('p');
      this.setTextContent(msg2, `可从步骤 "${this.getStepText(data.resumeStep)}" 继续处理`);
      intermediateDiv.appendChild(msg2);
      
      const actions = this.createElement('div', { className: 'upload-actions' });
      const configBtn = this.createElement('button', {
        className: 'btn-primary',
        onclick: () => {
          currentFileMd5 = data.md5;
          this.configureRules(data.md5);
        }
      });
      this.setTextContent(configBtn, '配置并继续');
      actions.appendChild(configBtn);
      
      const backBtn = this.createElement('button', {
        className: 'btn-secondary',
        onclick: () => showSection('file-list')
      });
      this.setTextContent(backBtn, '返回列表');
      actions.appendChild(backBtn);
      
      intermediateDiv.appendChild(actions);
      container.appendChild(intermediateDiv);
      
    } else {
      const successDiv = this.createElement('div', { className: 'upload-success' });
      
      const msg = this.createElement('p');
      this.setTextContent(msg, '文件上传成功');
      successDiv.appendChild(msg);
      
      const actions = this.createElement('div', { className: 'upload-actions' });
      const configBtn = this.createElement('button', {
        className: 'btn-primary',
        onclick: () => {
          currentFileMd5 = data.md5;
          this.configureRules(data.md5);
        }
      });
      this.setTextContent(configBtn, '配置规则并处理');
      actions.appendChild(configBtn);
      
      const backBtn = this.createElement('button', {
        className: 'btn-secondary',
        onclick: () => showSection('file-list')
      });
      this.setTextContent(backBtn, '返回列表');
      actions.appendChild(backBtn);
      
      successDiv.appendChild(actions);
      container.appendChild(successDiv);
    }
    
    return container;
  }
};

// 导出工具
if (typeof module !== 'undefined' && module.exports) {
  module.exports = DomUtils;
}