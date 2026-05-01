// 文件管理模块
const FileManager = (function() {
  // 私有变量
  let currentFileMd5 = null;
  let fileList = [];

  // 初始化
  function init() {
    bindEvents();
    refreshFileList();
  }

  // 绑定事件
  function bindEvents() {
    // 文件上传
    const fileInput = document.getElementById('file-input');
    if (fileInput) {
      fileInput.addEventListener('change', handleFileUpload);
    }

    // 拖拽上传区（文件列表空状态）
    const dropZone = document.getElementById('drop-zone-empty');
    const dropInput = document.getElementById('drop-zone-input');
    if (dropZone && dropInput) {
      dropInput.addEventListener('change', function(e) {
        handleFileUpload(e);
      });

      // 拖拽事件
      dropZone.addEventListener('dragover', function(e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.add('drag-over');
      });

      dropZone.addEventListener('dragleave', function(e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.remove('drag-over');
      });

      dropZone.addEventListener('drop', function(e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.remove('drag-over');
        const files = e.dataTransfer.files;
        if (files && files.length > 0) {
          // 模拟 file input change
          const dataTransfer = new DataTransfer();
          dataTransfer.items.add(files[0]);
          dropInput.files = dataTransfer.files;
          const event = new Event('change');
          dropInput.dispatchEvent(event);
        }
      });
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
        const dropZone = document.getElementById("drop-zone-empty");
        const countBadge = document.getElementById("file-count-badge");
        if (!container) return;

        if (!data.files || data.files.length === 0) {
          container.innerHTML = "";
          if (dropZone) dropZone.style.display = "flex";
          if (countBadge) countBadge.style.display = "none";
          return;
        }

        // 有文件时隐藏拖拽区
        if (dropZone) dropZone.style.display = "none";
        if (countBadge) {
          countBadge.style.display = "inline-flex";
          DomUtils.setTextContent(countBadge, data.files.length + " 个文件");
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
      DomUtils.setTextContent(loadingDiv, '\u6B63\u5728\u4E0A\u4F20... ' + progress + '%');
    })
      .then((data) => {
        resultDiv.innerHTML = "";
        const resultContent = DomUtils.createUploadResult(data);
        resultDiv.appendChild(resultContent);

        // 上传成功后：自动跳转到规则配置
        if (data.success) {
          // 先刷新列表确保文件可见
          refreshFileList();

          // 自动跳转到规则配置
          const md5 = data.md5;
          if (md5) {
            RulesConfigModule.configureRules(md5);
          } else {
            showSection('file-list');
          }

          showFeedback('\u6587\u4EF6 "' + file.name + '" \u4E0A\u4F20\u6210\u529F\uFF01', "success");
        }
      })
      .catch((err) => {
        resultDiv.innerHTML = "";
        const errorDiv = DomUtils.createElement('div', { className: 'upload-error' });
        const errorMsg = DomUtils.createElement('p');
        DomUtils.setTextContent(errorMsg, '\u4E0A\u4F20\u5931\u8D25: ' + err.message);
        errorDiv.appendChild(errorMsg);

        const actions = DomUtils.createElement('div', { className: 'upload-actions' });
        const closeBtn = DomUtils.createElement('button', {
          className: 'btn-secondary',
          onclick: () => {
            resultDiv.style.display = 'none';
          }
        });
        DomUtils.setTextContent(closeBtn, '\u5173\u95ED');
        actions.appendChild(closeBtn);
        errorDiv.appendChild(actions);

        resultDiv.appendChild(errorDiv);
      });

    event.target.value = "";
  }

  // 删除文件
  function deleteFile(md5, fileName) {
    if (!confirm('\u786E\u5B9A\u5220\u9664\u6587\u4EF6"' + (fileName || md5) + '"\u7684\u6240\u6709\u5904\u7406\u8BB0\u5F55\uFF1F\n\n\u6B64\u64CD\u4F5C\u5C06\u5220\u9664\u8BE5\u6587\u4EF6\u7684\u6240\u6709\u5BA1\u6838\u8BB0\u5F55\u3001\u7248\u672C\u5386\u53F2\u548C\u5904\u7406\u65E5\u5FD7\uFF0C\u4E14\u4E0D\u53EF\u6062\u590D\uFF01')) {
      return;
    }
    if (!confirm('\u4E8C\u6B21\u786E\u8BA4\uFF1A\u771F\u7684\u8981\u5220\u9664\u5417\uFF1F\u6B64\u64CD\u4F5C\u4E0D\u53EF\u64A4\u9500\uFF01')) return;

    AppConfig.apiRequest(`/files/${md5}`, { method: "DELETE" })
      .then((data) => {
        if (data.success) {
          showFeedback("\u6587\u4EF6\u5DF2\u5220\u9664");
          refreshFileList();
        } else {
          showFeedback(data.message, "error");
        }
      })
      .catch((err) => showFeedback("\u5220\u9664\u5931\u8D25: " + err.message, "error"));
  }

  // 重新处理文件
  function reprocessFile(md5) {
    if (!confirm("\u786E\u5B9A\u91CD\u65B0\u5904\u7406\u8BE5\u6587\u4EF6\uFF1F\u4E4B\u524D\u7684\u5904\u7406\u7ED3\u679C\u5C06\u88AB\u6E05\u9664\u3002")) return;

    AppConfig.apiRequest(`/files/${md5}/resume`, { method: "POST" })
      .then((data) => {
        if (data.success) {
          currentFileMd5 = md5;
          RulesConfigModule.configureRules(md5);
        } else {
          showFeedback(data.message, "error");
        }
      })
      .catch((err) => showFeedback("\u91CD\u65B0\u5904\u7406\u5931\u8D25: " + err.message, "error"));
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
            .catch((err) => showFeedback("\u542F\u52A8\u5904\u7406\u5931\u8D25: " + err.message, "error"));
        } else {
          showFeedback(data.message, "error");
        }
      })
      .catch((err) => showFeedback("\u6062\u590D\u6587\u4EF6\u5931\u8D25: " + err.message, "error"));
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
          throw new Error(`\u4E0B\u8F7D\u5931\u8D25: ${response.status}`);
        }
        // 从 Content-Disposition 头获取服务端文件名
        // 优先使用 filename*=UTF-8''...（支持中文），fallback 到 filename="..."
        const disposition = response.headers.get('Content-Disposition');
        let filename = null;
        if (disposition) {
          var matchUtf = disposition.match(/filename\*=UTF-8''([^;\s]+)/);
          if (matchUtf && matchUtf[1]) {
            filename = decodeURIComponent(matchUtf[1]);
          } else {
            var matchAscii = disposition.match(/filename="([^"]*)"/);
            if (matchAscii && matchAscii[1]) {
              filename = matchAscii[1];
            }
          }
        }
        return response.blob().then(function(blob) {
          return { blob: blob, filename: filename };
        });
      })
      .then(function(result) {
        var blob = result.blob;
        var filename = result.filename || ('file_' + md5 + '.txt');
        var url = window.URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = url;
        a.download = filename;
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
          throw new Error(`\u83B7\u53D6\u62A5\u544A\u5931\u8D25: ${response.status}`);
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
    handleFileUpload,
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
