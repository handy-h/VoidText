// =============================================
// main.js - 全局兼容层
// 桥接 inline onclick 到模块方法
// =============================================

let currentFileMd5 = null;
// completed 页面当前正在查看的文件 md5（从状态响应中精确同步，不依赖轮询全局变量）
let completedFileMd5 = null;

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
  FileManager.handleFileUpload(event);
}

// ========== Toast 反馈 ==========
function showFeedback(message, type) {
  DomUtils.showFeedback(message, type);
}

// ========== 工具方法（供内联使用） ==========
function formatFileSize(bytes) {
  return DomUtils.formatFileSize(bytes);
}

function formatTime(ts) {
  return DomUtils.formatTime(ts);
}

function getStatusText(status) {
  return DomUtils.getStatusText(status);
}

function getStepText(step) {
  return DomUtils.getStepText(step);
}

// ========== 下载等操作 ==========
function getCurrentFileMd5() {
  return currentFileMd5 || ProcessingModule.getCurrentFileMd5() || FileManager.getCurrentFileMd5();
}

function downloadCurrentVersion() {
  // 优先用 completed 页面同步的 md5，然后用全局回退
  const md5 = completedFileMd5 || getCurrentFileMd5();
  if (md5) FileManager.downloadFile(md5);
}

// downloadFinalFile 与 downloadCurrentVersion 完全相同，保留供 HTML onclick 兼容
function downloadFinalFile() {
  downloadCurrentVersion();
}

function viewReport() {
  // 优先用 completed 页面同步的 md5
  const md5 = completedFileMd5 || getCurrentFileMd5();
  if (md5) FileManager.viewReport(md5);
}

function toggleLogs() {
  const logsContent = document.getElementById('logs-content');
  const toggleIcon = document.getElementById('logs-toggle-icon');
  if (logsContent && toggleIcon) {
    if (logsContent.style.display === 'none') {
      logsContent.style.display = 'block';
      toggleIcon.textContent = '▲';
    } else {
      logsContent.style.display = 'none';
      toggleIcon.textContent = '▼';
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
