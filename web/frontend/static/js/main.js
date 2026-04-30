// =============================================
// main.js - 全局兼容层
// 桥接 inline onclick 到模块方法
// =============================================

let currentFileMd5 = null;

// ========== 认证 ==========
function handleLogin() {
  AppModule.handleLogin();
}

function handleLogout() {
  AppModule.handleLogout();
}

// ========== 视图切换 ==========
function showSection(section) {
  FileManager.showSection(section);
}

// ========== 文件列表 ==========
function refreshFileList() {
  FileManager.refreshFileList();
}

// ========== 文件上传 ==========
function handleFileUpload(event) {
  FileManager.handleFileUpload ? FileManager.handleFileUpload(event) : legacyHandleFileUpload(event);
}

// 兼容：如果 FileManager 没有暴露 handleFileUpload，使用 dom-utils 方式
function legacyHandleFileUpload(event) {
  const file = event.target.files[0];
  if (!file) return;

  const resultDiv = document.getElementById("upload-result");
  if (!resultDiv) return;

  resultDiv.style.display = "block";
  resultDiv.innerHTML = "";

  const loadingDiv = DomUtils.createElement("div", { className: "upload-loading" });
  DomUtils.setTextContent(loadingDiv, "\u6B63\u5728\u4E0A\u4F20...");
  resultDiv.appendChild(loadingDiv);

  AppConfig.uploadFile(file, (progress) => {
    DomUtils.setTextContent(loadingDiv, "\u6B63\u5728\u4E0A\u4F20... " + progress + "%");
  })
    .then((data) => {
      resultDiv.innerHTML = "";
      const resultContent = DomUtils.createUploadResult(data);
      resultDiv.appendChild(resultContent);

      if (data.success) {
        showFeedback('\u6587\u4EF6 "' + file.name + '" \u4E0A\u4F20\u6210\u529F\uFF01', "success");
        refreshFileList();
        if (data.md5) {
          RulesConfigModule.configureRules(data.md5);
        } else {
          showSection('file-list');
        }
      }
    })
    .catch((err) => {
      resultDiv.innerHTML = "";
      const errorDiv = DomUtils.createElement("div", { className: "upload-error" });
      const errorMsg = DomUtils.createElement("p");
      DomUtils.setTextContent(errorMsg, "\u4E0A\u4F20\u5931\u8D25: " + err.message);
      errorDiv.appendChild(errorMsg);

      const actions = DomUtils.createElement("div", { className: "upload-actions" });
      const closeBtn = DomUtils.createElement("button", {
        className: "btn-secondary",
        onclick: () => { resultDiv.style.display = "none"; }
      });
      DomUtils.setTextContent(closeBtn, "\u5173\u95ED");
      actions.appendChild(closeBtn);
      errorDiv.appendChild(actions);

      resultDiv.appendChild(errorDiv);
    });

  event.target.value = "";
}

// ========== Toast 反馈 ==========
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

// ========== 工具方法（供内联使用） ==========
function formatFileSize(bytes) {
  const kb = 1024;
  const mb = kb * 1024;
  const gb = mb * 1024;
  switch (true) {
    case bytes >= gb: return (bytes / gb).toFixed(1) + "GB";
    case bytes >= mb: return (bytes / mb).toFixed(1) + "MB";
    case bytes >= kb: return (bytes / kb).toFixed(1) + "KB";
    default: return bytes + "B";
  }
}

function formatTime(ts) {
  if (!ts) return "";
  try {
    return new Date(ts).toLocaleString("zh-CN");
  } catch {
    return ts;
  }
}

function getStatusText(status) {
  const map = {
    pending: "\u5F85\u5904\u7406",
    processing: "\u5904\u7406\u4E2D",
    reviewing: "\u5BA1\u6838\u4E2D",
    completed: "\u5DF2\u5B8C\u6210",
    failed: "\u5931\u8D25",
  };
  return map[status] || status;
}

function getStepText(step) {
  const map = {
    cleaning: "\u57FA\u7840\u6E05\u6D17",
    indexing: "\u5411\u91CF\u68C0\u6D4B",
    llm_fix: "LLM\u4FEE\u590D",
    review: "\u4EBA\u5DE5\u5BA1\u6838",
    finalizing: "\u751F\u6210\u6587\u4EF6",
  };
  return map[step] || step || "-";
}

// ========== 下载等操作 ==========
function getCurrentFileMd5() {
  return currentFileMd5 || ProcessingModule.getCurrentFileMd5() || FileManager.getCurrentFileMd5();
}

function downloadCurrentVersion() {
  const md5 = getCurrentFileMd5();
  if (md5) {
    FileManager.downloadFile(md5);
  }
}

function downloadFinalFile() {
  const md5 = getCurrentFileMd5();
  if (md5) {
    FileManager.downloadFile(md5);
  }
}

function viewReport() {
  const md5 = getCurrentFileMd5();
  if (md5) {
    FileManager.viewReport(md5);
  }
}

function toggleLogs() {
  const logsContent = document.getElementById("logs-content");
  const toggleIcon = document.getElementById("logs-toggle-icon");
  if (logsContent && toggleIcon) {
    if (logsContent.style.display === "none") {
      logsContent.style.display = "block";
      toggleIcon.textContent = "\u25B2";
    } else {
      logsContent.style.display = "none";
      toggleIcon.textContent = "\u25BC";
    }
  }
}

// ========== 轮询全局适配 ==========
function startPolling() {
  ProcessingModule.startPolling();
}

function stopPolling() {
  ProcessingModule.stopPolling();
}

// 为 DomUtils 设置回调
if (typeof DomUtils !== 'undefined') {
  DomUtils.deleteFile = function(md5, fileName) { FileManager.deleteFile(md5, fileName); };
  DomUtils.configureRules = function(md5) { RulesConfigModule.configureRules(md5); };
  DomUtils.viewProgress = function(md5) { ProcessingModule.viewProgress(md5); };
  DomUtils.resumeFile = function(md5) { FileManager.resumeFile(md5); };
  DomUtils.downloadFile = function(md5) { FileManager.downloadFile(md5); };
  DomUtils.viewReport = function(md5) { FileManager.viewReport(md5); };
  DomUtils.reprocessFile = function(md5) { FileManager.reprocessFile(md5); };
}
