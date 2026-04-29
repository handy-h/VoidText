// 文件管理模块
const FileManager = (function() {
  // 私有变量
  let currentFileMd5 = null;
  let fileList = [];
  
  // 初始化
  function init() {
    // 绑定事件
    bindEvents();
    // 加载文件列表
    refreshFileList();
  }
  
  // 绑定事件
  function bindEvents() {
    // 文件上传
    const fileInput = document.getElementById('file-input');
    if (fileInput) {
      fileInput.addEventListener('change', handleFileUpload);
    }
    
    // 导航按钮
    const navButtons = document.querySelectorAll('.nav-btn');
    navButtons.forEach(btn => {
      btn.addEventListener('click', function() {
        const section = this.getAttribute('data-section');
        if (section) {
          showSection(section);
        }
      });
    });
  }
  
  // 显示指定部分
  function showSection(section) {
    const sections = [
      "file-list",
      "upload",
      "rules-config",
      "processing",
      "review",
      "completed",
    ];
    
    sections.forEach((s) => {
      const el = document.getElementById(s + "-section");
      if (el) el.style.display = s === section ? "block" : "none";
    });
    
    document.querySelectorAll(".nav-btn").forEach((btn) => btn.classList.remove("active"));
    const navMap = { "file-list": 0, upload: 1 };
    if (navMap[section] !== undefined) {
      document.querySelectorAll(".nav-btn")[navMap[section]].classList.add("active");
    }
    
    if (section === "file-list") {
      refreshFileList();
      stopPolling();
    }
  }
  
  // 刷新文件列表
  function refreshFileList() {
    AppConfig.apiRequest("/files")
      .then((data) => {
        if (!data.success) return;
        
        const container = document.getElementById("file-list-container");
        if (!container) return;
        
        if (!data.files || data.files.length === 0) {
          container.innerHTML = "";
          const emptyState = DomUtils.createElement('div', { className: 'empty-state' });
          DomUtils.setTextContent(emptyState, '暂无文件，请上传文件开始处理');
          container.appendChild(emptyState);
          return;
        }
        
        // 清空容器
        container.innerHTML = "";
        
        // 使用安全的DOM操作创建文件卡片
        data.files.forEach(file => {
          const card = DomUtils.createFileCard(file);
          container.appendChild(card);
        });
        
        fileList = data.files;
      })
      .catch((err) => showFeedback("加载文件列表失败: " + err.message, "error"));
  }
  
  // 处理文件上传
  function handleFileUpload(event) {
    const file = event.target.files[0];
    if (!file) return;
    
    const resultDiv = document.getElementById("upload-result");
    if (!resultDiv) return;
    
    resultDiv.style.display = "block";
    resultDiv.innerHTML = "";
    
    const loadingDiv = DomUtils.createElement('div', { className: 'upload-loading' });
    DomUtils.setTextContent(loadingDiv, '正在上传...');
    resultDiv.appendChild(loadingDiv);
    
    AppConfig.uploadFile(file, (progress) => {
      DomUtils.setTextContent(loadingDiv, `正在上传... ${progress}%`);
    })
      .then((data) => {
        resultDiv.innerHTML = "";
        const resultContent = DomUtils.createUploadResult(data);
        resultDiv.appendChild(resultContent);
        
        // 上传成功后显示 toast 并立即跳转
        if (data.success) {
          showFeedback(`文件 "${file.name}" 上传成功！`, "success");
          refreshFileList();
          showSection('file-list');
        }
      })
      .catch((err) => {
        resultDiv.innerHTML = "";
        const errorDiv = DomUtils.createElement('div', { className: 'upload-error' });
        const errorMsg = DomUtils.createElement('p');
        DomUtils.setTextContent(errorMsg, `上传失败: ${err.message}`);
        errorDiv.appendChild(errorMsg);
        
        const actions = DomUtils.createElement('div', { className: 'upload-actions' });
        const closeBtn = DomUtils.createElement('button', {
          className: 'btn-secondary',
          onclick: () => {
            resultDiv.style.display = 'none';
          }
        });
        DomUtils.setTextContent(closeBtn, '关闭');
        actions.appendChild(closeBtn);
        errorDiv.appendChild(actions);
        
        resultDiv.appendChild(errorDiv);
      });
    
    event.target.value = "";
  }
  
  // 删除文件
  function deleteFile(md5, fileName) {
    if (!confirm(`确定删除文件"${fileName || md5}"的所有处理记录？\n\n此操作将删除该文件的所有审核记录、版本历史和处理日志，且不可恢复！`)) {
      return;
    }
    
    if (!confirm("二次确认：真的要删除吗？此操作不可撤销！")) return;
    
    AppConfig.apiRequest(`/files/${md5}`, { method: "DELETE" })
      .then((data) => {
        if (data.success) {
          showFeedback("文件已删除");
          refreshFileList();
        } else {
          showFeedback(data.message, "error");
        }
      })
      .catch((err) => showFeedback("删除失败: " + err.message, "error"));
  }
  
  // 重新处理文件
  function reprocessFile(md5) {
    if (!confirm("确定重新处理该文件？之前的处理结果将被清除。")) return;
    
    AppConfig.apiRequest(`/files/${md5}/resume`, { method: "POST" })
      .then((data) => {
        if (data.success) {
          currentFileMd5 = md5;
          configureRules(md5);
        } else {
          showFeedback(data.message, "error");
        }
      })
      .catch((err) => showFeedback("重新处理失败: " + err.message, "error"));
  }
  
  // 恢复文件处理
  function resumeFile(md5) {
    AppConfig.apiRequest(`/files/${md5}/resume`, { method: "POST" })
      .then((data) => {
        if (data.success) {
          currentFileMd5 = md5;
          AppConfig.apiRequest(`/files/${md5}/run`, { method: "POST" })
            .then((data2) => {
              if (data2.success) {
                showSection("processing");
                startPolling();
              } else {
                showFeedback(data2.message, "error");
              }
            })
            .catch((err) => showFeedback("启动处理失败: " + err.message, "error"));
        } else {
          showFeedback(data.message, "error");
        }
      })
      .catch((err) => showFeedback("恢复文件失败: " + err.message, "error"));
  }
  
  // 下载文件
  function downloadFile(md5) {
    const token = AppConfig.getAuthToken();
    const url = `/api/files/${md5}/download`;
    const headers = {};
    if (token) {
      headers['X-API-Token'] = token;
    }
    
    fetch(url, { headers })
      .then(response => {
        if (!response.ok) {
          throw new Error(`下载失败: ${response.status}`);
        }
        return response.blob();
      })
      .then(blob => {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `file_${md5}.txt`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
      })
      .catch(err => showFeedback(err.message, "error"));
  }
  
  // 查看报告
  function viewReport(md5) {
    const token = AppConfig.getAuthToken();
    const url = `/api/files/${md5}/report?format=html`;
    const headers = {};
    if (token) {
      headers['X-API-Token'] = token;
    }
    
    fetch(url, { headers })
      .then(response => {
        if (!response.ok) {
          throw new Error(`获取报告失败: ${response.status}`);
        }
        return response.blob();
      })
      .then(blob => {
        const url = window.URL.createObjectURL(blob);
        window.open(url, '_blank');
        window.URL.revokeObjectURL(url);
      })
      .catch(err => showFeedback(err.message, "error"));
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
    showSection,
    refreshFileList,
    deleteFile,
    reprocessFile,
    resumeFile,
    downloadFile,
    viewReport,
    getCurrentFileMd5: () => currentFileMd5,
    setCurrentFileMd5: (md5) => { currentFileMd5 = md5; },
    getFileList: () => fileList
  };
})();

// 导出到全局作用域
window.FileManager = FileManager;