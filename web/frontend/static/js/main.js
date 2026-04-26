let currentFileMd5 = null;
let reviewItems = [];
let pollingTimer = null;

// 初始化应用
async function initApp() {
  // 先从后端获取认证配置
  await AppConfig.fetchAuthConfig();

  // 检查认证状态
  checkAuthStatus();

  // 显示文件列表
  showSection("file-list");
}

// 检查认证状态
function checkAuthStatus() {
  if (AppConfig.isAuthEnabled()) {
    // 认证已启用，检查是否已登录
    const token = AppConfig.getAuthToken();
    if (token) {
      document.getElementById("app").style.display = "block";
      document.getElementById("auth-section").style.display = "none";
    } else {
      document.getElementById("app").style.display = "none";
      document.getElementById("auth-section").style.display = "block";
    }
  } else {
    // 认证未启用，直接显示应用
    document.getElementById("app").style.display = "block";
    document.getElementById("auth-section").style.display = "none";
  }
}

// 处理登录
function handleLogin() {
  const tokenInput = document.getElementById("auth-token");
  const token = tokenInput.value.trim();

  if (!token) {
    showFeedback("请输入认证token", "error");
    return;
  }

  if (AppConfig.setAuthToken(token)) {
    showFeedback("登录成功", "success");
    checkAuthStatus();
  } else {
    showFeedback("token无效", "error");
  }
}

// 处理登出
function handleLogout() {
  AppConfig.clearAuthToken();
  showFeedback("已登出", "success");
  checkAuthStatus();
}

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

  document
    .querySelectorAll(".nav-btn")
    .forEach((btn) => btn.classList.remove("active"));
  const navMap = { "file-list": 0, upload: 1 };
  if (navMap[section] !== undefined) {
    document
      .querySelectorAll(".nav-btn")
      [navMap[section]].classList.add("active");
  }

  if (section === "file-list") {
    refreshFileList();
    stopPolling();
  }
}

function showFeedback(message, type) {
  const fb = document.getElementById("feedback");
  fb.textContent = message;
  fb.className = "feedback " + (type || "success");
  fb.style.display = "block";
  setTimeout(() => {
    fb.style.display = "none";
  }, 3000);
}

function escapeHtml(text) {
  if (!text) return "";
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
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
    pending: "待处理",
    processing: "处理中",
    reviewing: "审核中",
    completed: "已完成",
    failed: "失败",
  };
  return map[status] || status;
}

function getStatusClass(status) {
  const map = {
    pending: "status-pending",
    processing: "status-processing",
    reviewing: "status-reviewing",
    completed: "status-completed",
    failed: "status-failed",
  };
  return map[status] || "";
}

function getStepText(step) {
  const map = {
    cleaning: "基础清洗",
    indexing: "向量检测",
    llm_fix: "LLM修复",
    review: "人工审核",
    finalizing: "生成文件",
  };
  return map[step] || step || "-";
}

// ========== 文件列表 ==========

function refreshFileList() {
  AppConfig.apiRequest("/files")
    .then((data) => {
      if (!data.success) return;
      const container = document.getElementById("file-list-container");
      if (!data.files || data.files.length === 0) {
        container.innerHTML = "";
        const emptyState = DomUtils.createElement("div", {
          className: "empty-state",
        });
        DomUtils.setTextContent(emptyState, "暂无文件，请上传文件开始处理");
        container.appendChild(emptyState);
        return;
      }

      // 清空容器
      container.innerHTML = "";

      // 使用安全的DOM操作创建文件卡片
      data.files.forEach((file) => {
        const card = DomUtils.createFileCard(file);
        container.appendChild(card);
      });
    })
    .catch((err) => showFeedback("加载文件列表失败: " + err.message, "error"));
}

// 为DomUtils设置回调函数
DomUtils.deleteFile = deleteFile;
DomUtils.configureRules = configureRules;
DomUtils.viewProgress = viewProgress;
DomUtils.resumeFile = resumeFile;
DomUtils.downloadFile = downloadFile;
DomUtils.viewReport = viewReport;
DomUtils.reprocessFile = reprocessFile;

function deleteFile(md5, fileName) {
  if (
    !confirm(
      `确定删除文件"${fileName || md5}"的所有处理记录？\n\n此操作将删除该文件的所有审核记录、版本历史和处理日志，且不可恢复！`,
    )
  )
    return;
  if (!confirm("二次确认：真的要删除吗？此操作不可撤销！")) return;

  AppConfig.apiRequest(`/files/${md5}`, { method: "DELETE" })
    .then((data) => {
      if (data.success) {
        showFeedback("文件已删除");
        refreshFileList();
      } else showFeedback(data.message, "error");
    })
    .catch((err) => showFeedback("删除失败: " + err.message, "error"));
}

function reprocessFile(md5) {
  if (!confirm("确定重新处理该文件？之前的处理结果将被清除。")) return;
  AppConfig.apiRequest(`/files/${md5}/resume`, { method: "POST" })
    .then((data) => {
      if (data.success) {
        currentFileMd5 = md5;
        configureRules(md5);
      } else showFeedback(data.message, "error");
    })
    .catch((err) => showFeedback("重新处理失败: " + err.message, "error"));
}

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
          .catch((err) =>
            showFeedback("启动处理失败: " + err.message, "error"),
          );
      } else showFeedback(data.message, "error");
    })
    .catch((err) => showFeedback("恢复文件失败: " + err.message, "error"));
}

function downloadFile(md5) {
  const token = AppConfig.getAuthToken();
  const url = `/api/files/${md5}/download`;
  const headers = {};
  if (token) {
    headers["X-API-Token"] = token;
  }

  // 使用fetch下载文件
  fetch(url, { headers })
    .then((response) => {
      if (!response.ok) {
        throw new Error(`下载失败: ${response.status}`);
      }
      return response.blob();
    })
    .then((blob) => {
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `file_${md5}.txt`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    })
    .catch((err) => showFeedback(err.message, "error"));
}

function viewReport(md5) {
  const token = AppConfig.getAuthToken();
  const url = `/api/files/${md5}/report?format=html`;
  const headers = {};
  if (token) {
    headers["X-API-Token"] = token;
  }

  fetch(url, { headers })
    .then((response) => {
      if (!response.ok) {
        throw new Error(`获取报告失败: ${response.status}`);
      }
      return response.blob();
    })
    .then((blob) => {
      const url = window.URL.createObjectURL(blob);
      window.open(url, "_blank");
      window.URL.revokeObjectURL(url);
    })
    .catch((err) => showFeedback(err.message, "error"));
}

// ========== 文件上传 ==========

function handleFileUpload(event) {
  const file = event.target.files[0];
  if (!file) return;

  const resultDiv = document.getElementById("upload-result");
  resultDiv.style.display = "block";
  resultDiv.innerHTML = "";

  const loadingDiv = DomUtils.createElement("div", {
    className: "upload-loading",
  });
  DomUtils.setTextContent(loadingDiv, "正在上传...");
  resultDiv.appendChild(loadingDiv);

  AppConfig.uploadFile(file, (progress) => {
    DomUtils.setTextContent(loadingDiv, `正在上传... ${progress}%`);
  })
    .then((data) => {
      resultDiv.innerHTML = "";
      const resultContent = DomUtils.createUploadResult(data);
      resultDiv.appendChild(resultContent);
    })
    .catch((err) => {
      resultDiv.innerHTML = "";
      const errorDiv = DomUtils.createElement("div", {
        className: "upload-error",
      });
      const errorMsg = DomUtils.createElement("p");
      DomUtils.setTextContent(errorMsg, `上传失败: ${err.message}`);
      errorDiv.appendChild(errorMsg);

      const actions = DomUtils.createElement("div", {
        className: "upload-actions",
      });
      const closeBtn = DomUtils.createElement("button", {
        className: "btn-secondary",
        onclick: () => {
          resultDiv.style.display = "none";
        },
      });
      DomUtils.setTextContent(closeBtn, "关闭");
      actions.appendChild(closeBtn);
      errorDiv.appendChild(actions);

      resultDiv.appendChild(errorDiv);
    });

  event.target.value = "";
}

// ========== 规则配置 ==========

function configureRules(md5) {
  currentFileMd5 = md5;

  AppConfig.apiRequest(`/files/${md5}`)
    .then((data) => {
      if (!data.success) {
        showFeedback(data.message, "error");
        return;
      }

      const file = data.file;
      const infoDiv = document.getElementById("file-info-display");
      if (infoDiv) {
        const parts = [];
        if (file.title) parts.push(`标题: ${file.title}`);
        if (file.author) parts.push(`作者: ${file.author}`);
        if (file.fileSize) parts.push(`大小: ${formatFileSize(file.fileSize)}`);
        DomUtils.setTextContent(infoDiv, parts.join(" | "));
      }

      showSection("rules-config");
    })
    .catch((err) => showFeedback("获取文件信息失败: " + err.message, "error"));
}

function formatFileSize(bytes) {
  const kb = 1024;
  const mb = kb * 1024;
  const gb = mb * 1024;
  switch (true) {
    case bytes >= gb:
      return (bytes / gb).toFixed(1) + "GB";
    case bytes >= mb:
      return (bytes / mb).toFixed(1) + "MB";
    case bytes >= kb:
      return (bytes / kb).toFixed(1) + "KB";
    default:
      return bytes + "B";
  }
}

// ========== 处理进度相关函数 ==========

function viewProgress(md5) {
  currentFileMd5 = md5;
  showSection("processing");
  startPolling();
}

function startPolling() {
  stopPolling();
  pollingTimer = setInterval(updateProgress, 2000);
  updateProgress();
}

function stopPolling() {
  if (pollingTimer) {
    clearInterval(pollingTimer);
    pollingTimer = null;
  }
}

function updateProgress() {
  if (!currentFileMd5) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/status`)
    .then((data) => {
      if (!data.success) return;

      const progressBar = document.getElementById("overall-progress");
      const progressText = document.getElementById("progress-text");
      const processingMessage = document.getElementById("processing-message");
      const currentAction = document.getElementById("current-action");

      if (progressBar && progressText) {
        const progress = data.progress || 0;
        progressBar.style.width = progress + "%";
        DomUtils.setTextContent(progressText, progress + "%");
      }

      if (processingMessage) {
        let message = `状态: ${getStatusText(data.status)}`;
        if (data.currentStep) {
          message += ` | 当前步骤: ${getStepText(data.currentStep)}`;
        }
        DomUtils.setTextContent(processingMessage, message);
      }

      if (currentAction && data.currentAction) {
        DomUtils.setTextContent(currentAction, data.currentAction);
      }

      updateStepProgress(data.currentStep);
      renderLogs(data.logs);
      renderChunkProgress(data.chunkProgress);

      if (
        data.status === "completed" ||
        data.status === "failed" ||
        data.status === "reviewing"
      ) {
        stopPolling();
        if (data.status === "reviewing") {
          showSection("review");
          loadReviewItems();
        } else if (data.status === "completed") {
          showSection("completed");
          updateCompletedInfo();
        }
      }
    })
    .catch((err) => {
      console.error("轮询失败:", err);
    });
}

function updateStepProgress(currentStep) {
  const steps = ["cleaning", "indexing", "llm_fix", "review", "finalizing"];
  const stepIndex = steps.indexOf(currentStep);

  steps.forEach((step, index) => {
    const stepItem = document.querySelector(`.step-item[data-step="${step}"]`);
    if (stepItem) {
      stepItem.classList.remove("active", "completed");
      if (index < stepIndex) {
        stepItem.classList.add("completed");
      } else if (index === stepIndex) {
        stepItem.classList.add("active");
      }
    }
  });
}

function renderLogs(logs) {
  const logsList = document.getElementById("logs-list");
  if (!logsList || !logs || !logs.length) return;

  logsList.innerHTML = "";

  const reversedLogs = [...logs].reverse();
  reversedLogs.forEach((logEntry) => {
    const logItem = DomUtils.createElement("div", {
      className: "log-item log-" + (logEntry.status || "info"),
    });

    const timeSpan = DomUtils.createElement("span", { className: "log-time" });
    const ts = logEntry.timestamp || "";
    DomUtils.setTextContent(
      timeSpan,
      ts ? new Date(ts).toLocaleTimeString("zh-CN") : "",
    );
    logItem.appendChild(timeSpan);

    const stepSpan = DomUtils.createElement("span", { className: "log-step" });
    DomUtils.setTextContent(stepSpan, getStepText(logEntry.step));
    logItem.appendChild(stepSpan);

    const msgSpan = DomUtils.createElement("span", {
      className: "log-message",
    });
    let detail = logEntry.details || logEntry.action || "";
    try {
      const parsed = JSON.parse(detail);
      if (parsed.action) {
        const actionMap = {
          step_started: "步骤开始",
          step_completed: "步骤完成",
          step_skipped: "步骤跳过",
          step_failed: "步骤失败",
        };
        detail = actionMap[parsed.action] || parsed.action;
        if (parsed.details) {
          if (parsed.details.step) {
            detail += " - " + getStepText(parsed.details.step);
          }
          if (parsed.details.reason) {
            detail += " (" + parsed.details.reason + ")";
          }
          if (parsed.details.result) {
            const parts = [];
            if (parsed.details.result.changes_count !== undefined) {
              parts.push("修改数: " + parsed.details.result.changes_count);
            }
            if (parsed.details.result.duplicates_detected !== undefined) {
              parts.push("重复: " + parsed.details.result.duplicates_detected);
            }
            if (parsed.details.result.total_chunks !== undefined) {
              parts.push("块数: " + parsed.details.result.total_chunks);
            }
            if (parsed.details.result.total_changes !== undefined) {
              parts.push("变更: " + parsed.details.result.total_changes);
            }
            if (parsed.details.result.cache_hits !== undefined) {
              parts.push("缓存命中: " + parsed.details.result.cache_hits);
            }
            if (parts.length > 0) {
              detail += " [" + parts.join(", ") + "]";
            }
          }
        }
      }
    } catch (e) {
      // detail is already a plain string
    }
    DomUtils.setTextContent(msgSpan, detail);
    logItem.appendChild(msgSpan);

    logsList.appendChild(logItem);
  });
}

function renderChunkProgress(chunkProgress) {
  const container = document.getElementById("chunk-progress-container");
  if (!container) return;

  if (!chunkProgress || chunkProgress.totalChunks === 0) {
    container.style.display = "none";
    return;
  }

  container.style.display = "block";

  const bar = document.getElementById("chunk-progress-bar");
  if (bar) {
    bar.style.width = (chunkProgress.progress || 0) + "%";
  }

  const countEl = document.getElementById("chunk-count");
  if (countEl) {
    DomUtils.setTextContent(
      countEl,
      `已处理: ${chunkProgress.processedChunks}/${chunkProgress.totalChunks} 块`,
    );
  }

  const statsEl = document.getElementById("chunk-api-stats");
  if (statsEl) {
    DomUtils.setTextContent(
      statsEl,
      `API调用: ${chunkProgress.apiCalls || 0}次 | 缓存命中: ${chunkProgress.cacheHits || 0}次`,
    );
  }

  const avgEl = document.getElementById("chunk-avg-time");
  if (avgEl) {
    DomUtils.setTextContent(
      avgEl,
      `平均耗时: ${chunkProgress.avgChunkTimeMs || 0}ms/块`,
    );
  }

  const etaEl = document.getElementById("chunk-eta");
  if (etaEl) {
    const secs = chunkProgress.estimatedRemainingSecs || 0;
    if (secs > 0) {
      const mins = Math.floor(secs / 60);
      const remainSecs = secs % 60;
      DomUtils.setTextContent(
        etaEl,
        mins > 0
          ? `预计剩余: ${mins}分${remainSecs}秒`
          : `预计剩余: ${remainSecs}秒`,
      );
    } else {
      DomUtils.setTextContent(etaEl, "");
    }
  }
}

// ========== 审核相关函数 ==========

function loadReviewItems() {
  if (!currentFileMd5) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/review-items`)
    .then((data) => {
      if (!data.success) return;
      reviewItems = data.items || [];
      renderReviewItems();
      updateReviewProgress();
    })
    .catch((err) => {
      console.error("加载审核项失败:", err);
    });
}

function renderReviewItems() {
  const container = document.getElementById("review-items-list");
  if (!container) return;

  container.innerHTML = "";

  const statusFilter = document.getElementById("review-status-filter");
  const filterValue = statusFilter ? statusFilter.value : "pending";

  const filteredItems = reviewItems.filter((item) => {
    if (filterValue === "all") return true;
    return item.status === filterValue;
  });

  filteredItems.forEach((item, index) => {
    const itemElement = createReviewItemElement(item, index);
    container.appendChild(itemElement);
  });
}

function createReviewItemElement(item, index) {
  const itemDiv = DomUtils.createElement("div", { className: "review-item" });

  const header = DomUtils.createElement("div", {
    className: "review-item-header",
  });

  const indexSpan = DomUtils.createElement("span", {
    className: "review-item-index",
  });
  DomUtils.setTextContent(indexSpan, `#${index + 1}`);
  header.appendChild(indexSpan);

  const typeSpan = DomUtils.createElement("span", {
    className: "review-item-type",
  });
  DomUtils.setTextContent(
    typeSpan,
    getModificationTypeText(item.modificationType),
  );
  header.appendChild(typeSpan);

  const confidenceSpan = DomUtils.createElement("span", {
    className: "review-item-confidence",
  });
  DomUtils.setTextContent(
    confidenceSpan,
    `置信度: ${(item.confidence * 100).toFixed(1)}%`,
  );
  header.appendChild(confidenceSpan);

  itemDiv.appendChild(header);

  const contentDiv = DomUtils.createElement("div", {
    className: "review-item-content",
  });

  const originalDiv = DomUtils.createElement("div", {
    className: "review-original",
  });
  const originalLabel = DomUtils.createElement("strong");
  DomUtils.setTextContent(originalLabel, "原文: ");
  originalDiv.appendChild(originalLabel);
  const originalText = DomUtils.createElement("span");
  DomUtils.setTextContent(originalText, item.originalText || "");
  originalDiv.appendChild(originalText);
  contentDiv.appendChild(originalDiv);

  const suggestedDiv = DomUtils.createElement("div", {
    className: "review-suggested",
  });
  const suggestedLabel = DomUtils.createElement("strong");
  DomUtils.setTextContent(suggestedLabel, "建议: ");
  suggestedDiv.appendChild(suggestedLabel);
  const suggestedText = DomUtils.createElement("span");
  DomUtils.setTextContent(suggestedText, item.suggestedText || "");
  suggestedDiv.appendChild(suggestedText);
  contentDiv.appendChild(suggestedDiv);

  if (item.editedText) {
    const editedDiv = DomUtils.createElement("div", {
      className: "review-edited",
    });
    const editedLabel = DomUtils.createElement("strong");
    DomUtils.setTextContent(editedLabel, "编辑后: ");
    editedDiv.appendChild(editedLabel);
    const editedText = DomUtils.createElement("span");
    DomUtils.setTextContent(editedText, item.editedText);
    editedDiv.appendChild(editedText);
    contentDiv.appendChild(editedDiv);
  }

  itemDiv.appendChild(contentDiv);

  const actionsDiv = DomUtils.createElement("div", {
    className: "review-item-actions",
  });

  if (item.status === "pending") {
    const approveBtn = DomUtils.createElement("button", {
      className: "btn-approve",
      onclick: () => approveReviewItem(item.id),
    });
    DomUtils.setTextContent(approveBtn, "通过");
    actionsDiv.appendChild(approveBtn);

    const rejectBtn = DomUtils.createElement("button", {
      className: "btn-reject",
      onclick: () => rejectReviewItem(item.id),
    });
    DomUtils.setTextContent(rejectBtn, "拒绝");
    actionsDiv.appendChild(rejectBtn);

    const editBtn = DomUtils.createElement("button", {
      className: "btn-edit",
      onclick: () => editReviewItem(item.id),
    });
    DomUtils.setTextContent(editBtn, "编辑");
    actionsDiv.appendChild(editBtn);
  } else if (item.status === "edited") {
    const restoreBtn = DomUtils.createElement("button", {
      className: "btn-restore",
      onclick: () => restoreReviewItem(item.id),
    });
    DomUtils.setTextContent(restoreBtn, "恢复原文");
    actionsDiv.appendChild(restoreBtn);
  }

  const statusSpan = DomUtils.createElement("span", {
    className: `review-status review-status-${item.status}`,
  });
  DomUtils.setTextContent(statusSpan, getReviewStatusText(item.status));
  actionsDiv.appendChild(statusSpan);

  itemDiv.appendChild(actionsDiv);

  return itemDiv;
}

function getModificationTypeText(type) {
  const map = {
    typo: "错别字",
    duplicate_paragraph: "重复段落",
    advertisement: "广告",
    grammar: "语法错误",
    style: "风格问题",
  };
  return map[type] || type;
}

function getReviewStatusText(status) {
  const map = {
    pending: "待审核",
    approved: "已通过",
    rejected: "已拒绝",
    edited: "已编辑",
  };
  return map[status] || status;
}

function approveReviewItem(itemId) {
  if (!currentFileMd5) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/approve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId }),
  })
    .then((data) => {
      if (data.success) {
        showFeedback("已通过修改建议", "success");
        loadReviewItems();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("操作失败: " + err.message, "error");
    });
}

function rejectReviewItem(itemId) {
  if (!currentFileMd5) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/reject`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId }),
  })
    .then((data) => {
      if (data.success) {
        showFeedback("已拒绝修改建议", "success");
        loadReviewItems();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("操作失败: " + err.message, "error");
    });
}

function editReviewItem(itemId) {
  const item = reviewItems.find((i) => i.id === itemId);
  if (!item) return;

  const editedText = prompt(
    "请输入编辑后的文本:",
    item.suggestedText || item.originalText,
  );
  if (editedText === null) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/edit`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId, editedText }),
  })
    .then((data) => {
      if (data.success) {
        showFeedback("已保存编辑", "success");
        loadReviewItems();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("编辑失败: " + err.message, "error");
    });
}

function restoreReviewItem(itemId) {
  AppConfig.apiRequest(`/files/${currentFileMd5}/restore`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId }),
  })
    .then((data) => {
      if (data.success) {
        showFeedback("已恢复原文", "success");
        loadReviewItems();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("恢复失败: " + err.message, "error");
    });
}

function batchApproveAll() {
  if (!currentFileMd5 || !confirm("确定通过所有待审核项？")) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/batch-approve`, {
    method: "POST",
  })
    .then((data) => {
      if (data.success) {
        showFeedback("已通过所有待审核项", "success");
        loadReviewItems();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("批量操作失败: " + err.message, "error");
    });
}

function batchRejectAll() {
  if (!currentFileMd5 || !confirm("确定拒绝所有待审核项？")) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/batch-reject`, {
    method: "POST",
  })
    .then((data) => {
      if (data.success) {
        showFeedback("已拒绝所有待审核项", "success");
        loadReviewItems();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("批量操作失败: " + err.message, "error");
    });
}

function updateReviewProgress() {
  const total = reviewItems.length;
  const pending = reviewItems.filter(
    (item) => item.status === "pending",
  ).length;
  const progressText = document.getElementById("review-progress-text");
  const finalizeBtn = document.getElementById("finalize-btn");

  if (progressText) {
    DomUtils.setTextContent(progressText, `${total - pending}/${total}`);
  }

  if (finalizeBtn) {
    finalizeBtn.style.display = pending === 0 ? "inline-block" : "none";
  }
}

function finalizeFile() {
  if (!currentFileMd5 || !confirm("确定完成审核并生成最终文件？")) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/finalize`, {
    method: "POST",
  })
    .then((data) => {
      if (data.success) {
        showFeedback("文件处理完成", "success");
        showSection("completed");
        updateCompletedInfo();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("完成处理失败: " + err.message, "error");
    });
}

function updateCompletedInfo() {
  const infoDiv = document.getElementById("completed-info");
  if (!infoDiv || !currentFileMd5) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}`)
    .then((data) => {
      if (!data.success) return;

      const file = data.file;
      const info = DomUtils.createElement("div");

      const title = DomUtils.createElement("h3");
      DomUtils.setTextContent(title, file.title || file.fileName);
      info.appendChild(title);

      const details = DomUtils.createElement("p");
      const parts = [];
      if (file.author) parts.push(`作者: ${file.author}`);
      parts.push(`大小: ${formatFileSize(file.fileSize)}`);
      parts.push(`状态: ${getStatusText(file.status)}`);
      parts.push(`完成时间: ${formatTime(file.updatedAt)}`);
      DomUtils.setTextContent(details, parts.join(" | "));
      info.appendChild(details);

      infoDiv.innerHTML = "";
      infoDiv.appendChild(info);
    })
    .catch((err) => {
      console.error("获取完成信息失败:", err);
    });
}

function downloadFinalFile() {
  if (currentFileMd5) {
    downloadFile(currentFileMd5);
  }
}

// ========== 规则配置相关函数 ==========

function saveRulesAndProcess() {
  if (!currentFileMd5) return;

  const rulesConfig = {
    enableBasicCleaning: document.getElementById("rule-basic-cleaning").checked,
    traditionalToSimple: document.getElementById("rule-traditional-simple")
      .checked,
    enableVectorDetection: document.getElementById("rule-vector-detection")
      .checked,
    vectorSimilarityThreshold:
      parseFloat(document.getElementById("rule-similarity").value) || 0.95,
    enableModelRepair: document.getElementById("rule-model-repair").checked,
    typoMap: document.getElementById("rule-typo-map").value,
    adBlacklist: document.getElementById("rule-ad-blacklist").value,
  };

  AppConfig.apiRequest(`/files/${currentFileMd5}/rules`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ rulesConfig: JSON.stringify(rulesConfig) }),
  })
    .then((data) => {
      if (data.success) {
        showFeedback("规则已保存", "success");
        startProcessing();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("保存规则失败: " + err.message, "error");
    });
}

function startProcessing() {
  if (!currentFileMd5) return;

  AppConfig.apiRequest(`/files/${currentFileMd5}/run`, { method: "POST" })
    .then((data) => {
      if (data.success) {
        showSection("processing");
        startPolling();
      } else {
        showFeedback(data.message, "error");
      }
    })
    .catch((err) => {
      showFeedback("启动处理失败: " + err.message, "error");
    });
}

function downloadCurrentVersion() {
  if (currentFileMd5) {
    downloadFile(currentFileMd5);
  }
}

function toggleLogs() {
  const logsContent = document.getElementById("logs-content");
  const toggleIcon = document.getElementById("logs-toggle-icon");
  if (logsContent && toggleIcon) {
    if (logsContent.style.display === "none") {
      logsContent.style.display = "block";
      toggleIcon.textContent = "▲";
    } else {
      logsContent.style.display = "none";
      toggleIcon.textContent = "▼";
    }
  }
}
