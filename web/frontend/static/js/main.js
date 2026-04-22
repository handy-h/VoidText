let currentFileMd5 = null;
let reviewItems = [];
let pollingTimer = null;

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
  fetch("/api/files")
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) return;
      const container = document.getElementById("file-list-container");
      if (!data.files || data.files.length === 0) {
        container.innerHTML =
          '<div class="empty-state">暂无文件，请上传文件开始处理</div>';
        return;
      }

      container.innerHTML = data.files
        .map(
          (f) => `
        <div class="file-card ${getStatusClass(f.status)}">
          <div class="file-info">
            <div class="file-title">${escapeHtml(f.title || f.fileName)}</div>
            <div class="file-meta">
              ${f.author ? `<span>作者: ${escapeHtml(f.author)}</span>` : ""}
              <span>状态: <strong>${getStatusText(f.status)}</strong></span>
              <span>步骤: ${getStepText(f.currentStep)}</span>
              <span>进度: ${f.progress}%</span>
              <span>更新: ${formatTime(f.updatedAt)}</span>
            </div>
            ${f.errorMsg ? `<div class="file-error">错误: ${escapeHtml(f.errorMsg)}</div>` : ""}
          </div>
          <div class="file-actions">
            ${getActionButtons(f)}
          </div>
        </div>
      `,
        )
        .join("");
    })
    .catch((err) => showFeedback("加载文件列表失败: " + err.message, "error"));
}

function getActionButtons(f) {
  const md5 = f.md5;
  const deleteBtn = `<button class="btn-danger" onclick="deleteFile('${md5}', '${escapeHtml(f.title || f.fileName)}')">删除</button>`;
  switch (f.status) {
    case "pending":
      return `<button class="btn-primary" onclick="configureRules('${md5}')">配置并处理</button>${deleteBtn}`;
    case "processing":
      return `<button class="btn-primary" onclick="viewProgress('${md5}')">查看进度</button>
              <button class="btn-secondary" onclick="resumeFile('${md5}')">继续处理</button>
              <button class="btn-secondary" onclick="downloadFile('${md5}')">下载</button>${deleteBtn}`;
    case "reviewing":
      return `<button class="btn-primary" onclick="viewProgress('${md5}')">查看进度</button>
              <button class="btn-secondary" onclick="downloadFile('${md5}')">下载</button>${deleteBtn}`;
    case "completed":
      return `<button class="btn-primary" onclick="downloadFile('${md5}')">下载最终文件</button>
              <button class="btn-secondary" onclick="viewReport('${md5}')">查看报告</button>
              <button class="btn-secondary" onclick="reprocessFile('${md5}')">重新处理</button>${deleteBtn}`;
    case "failed":
      return `<button class="btn-primary" onclick="resumeFile('${md5}')">从失败处恢复</button>${deleteBtn}`;
    default:
      return deleteBtn;
  }
}

function deleteFile(md5, fileName) {
  if (
    !confirm(
      `确定删除文件"${fileName || md5}"的所有处理记录？\n\n此操作将删除该文件的所有审核记录、版本历史和处理日志，且不可恢复！`,
    )
  )
    return;
  if (!confirm("二次确认：真的要删除吗？此操作不可撤销！")) return;

  fetch(`/api/files/${md5}`, { method: "DELETE" })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback("文件已删除");
        refreshFileList();
      } else showFeedback(data.message, "error");
    });
}

function reprocessFile(md5) {
  if (!confirm("确定重新处理该文件？之前的处理结果将被清除。")) return;
  fetch(`/api/files/${md5}/resume`, { method: "POST" })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        currentFileMd5 = md5;
        configureRules(md5);
      } else showFeedback(data.message, "error");
    });
}

function resumeFile(md5) {
  fetch(`/api/files/${md5}/resume`, { method: "POST" })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        currentFileMd5 = md5;
        fetch(`/api/files/${md5}/run`, { method: "POST" })
          .then((r2) => r2.json())
          .then((data2) => {
            if (data2.success) {
              showSection("processing");
              startPolling();
            } else {
              showFeedback(data2.message, "error");
            }
          });
      } else showFeedback(data.message, "error");
    });
}

function downloadFile(md5) {
  window.open(`/api/files/${md5}/download`, "_blank");
}

function viewReport(md5) {
  window.open(`/api/files/${md5}/report?format=html`, "_blank");
}

// ========== 文件上传 ==========

function handleFileUpload(event) {
  const file = event.target.files[0];
  if (!file) return;

  const formData = new FormData();
  formData.append("file", file);

  const resultDiv = document.getElementById("upload-result");
  resultDiv.style.display = "block";
  resultDiv.innerHTML = '<div class="upload-loading">正在上传...</div>';

  fetch("/api/files/upload", { method: "POST", body: formData })
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) {
        resultDiv.innerHTML = `
          <div class="upload-error">
            <p>上传失败: ${escapeHtml(data.message || "未知错误")}</p>
            <div class="upload-actions">
              <button class="btn-secondary" onclick="document.getElementById('upload-result').style.display='none'">关闭</button>
            </div>
          </div>`;
        return;
      }

      if (data.exists) {
        resultDiv.innerHTML = `
          <div class="upload-exists">
            <p>${escapeHtml(data.message)}</p>
            <p>当前状态: ${getStatusText(data.status)} ${data.currentStep ? "(" + getStepText(data.currentStep) + ")" : ""}</p>
            <div class="upload-actions">
              <button class="btn-primary" onclick="currentFileMd5='${data.md5}'; resumeFile('${data.md5}')">${data.suggestion || "继续处理"}</button>
              <button class="btn-secondary" onclick="showSection('file-list')">返回列表</button>
            </div>
          </div>`;
      } else if (data.isIntermediate) {
        currentFileMd5 = data.md5;
        resultDiv.innerHTML = `
          <div class="upload-intermediate">
            <p>检测到中间版本文件</p>
            <p>可从步骤 "${getStepText(data.resumeStep)}" 继续处理</p>
            <div class="upload-actions">
              <button class="btn-primary" onclick="currentFileMd5='${data.md5}'; configureRules('${data.md5}')">配置并继续</button>
              <button class="btn-secondary" onclick="showSection('file-list')">返回列表</button>
            </div>
          </div>`;
      } else {
        currentFileMd5 = data.md5;
        resultDiv.innerHTML = `
          <div class="upload-success">
            <p>文件上传成功</p>
            <div class="upload-actions">
              <button class="btn-primary" onclick="configureRules('${data.md5}')">配置规则并处理</button>
              <button class="btn-secondary" onclick="showSection('file-list')">返回列表</button>
            </div>
          </div>`;
      }
    })
    .catch((err) => {
      resultDiv.innerHTML = `
        <div class="upload-error">
          <p>上传失败: ${escapeHtml(err.message)}</p>
          <div class="upload-actions">
            <button class="btn-secondary" onclick="document.getElementById('upload-result').style.display='none'">关闭</button>
          </div>
        </div>`;
    });

  event.target.value = "";
}

// ========== 规则配置 ==========

function configureRules(md5) {
  currentFileMd5 = md5;

  fetch(`/api/files/${md5}`)
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) {
        showFeedback(data.message, "error");
        return;
      }

      let rules = {};
      if (data.file.rulesConfig) {
        try {
          rules = JSON.parse(data.file.rulesConfig);
        } catch {}
      }

      document.getElementById("rule-basic-cleaning").checked =
        rules.enableBasicCleaning !== false;
      document.getElementById("rule-traditional-simple").checked =
        rules.traditionalToSimple === true;
      document.getElementById("rule-vector-detection").checked =
        rules.enableVectorDetection !== false;
      document.getElementById("rule-similarity").value =
        rules.similarityThreshold || 0.95;
      document.getElementById("rule-model-repair").checked =
        rules.enableModelRepair !== false;

      if (rules.typoMap) {
        document.getElementById("rule-typo-map").value = Object.entries(
          rules.typoMap,
        )
          .map(([k, v]) => k + "=" + v)
          .join("\n");
      }
      if (rules.adBlacklist) {
        document.getElementById("rule-ad-blacklist").value =
          rules.adBlacklist.join("\n");
      }

      showSection("rules-config");
    });
}

function buildRulesConfig() {
  const typoMapText = document.getElementById("rule-typo-map").value.trim();
  const typoMap = {};
  if (typoMapText) {
    typoMapText.split("\n").forEach((line) => {
      const parts = line.split("=");
      if (parts.length >= 2) {
        typoMap[parts[0].trim()] = parts.slice(1).join("=").trim();
      }
    });
  }

  const adBlacklistText = document
    .getElementById("rule-ad-blacklist")
    .value.trim();
  const adBlacklist = adBlacklistText
    ? adBlacklistText
        .split("\n")
        .map((l) => l.trim())
        .filter((l) => l)
    : [];

  return {
    enableBasicCleaning: document.getElementById("rule-basic-cleaning").checked,
    traditionalToSimple: document.getElementById("rule-traditional-simple")
      .checked,
    enableVectorDetection: document.getElementById("rule-vector-detection")
      .checked,
    similarityThreshold:
      parseFloat(document.getElementById("rule-similarity").value) || 0.95,
    enableModelRepair: document.getElementById("rule-model-repair").checked,
    typoMap: typoMap,
    adBlacklist: adBlacklist,
  };
}

function saveRulesAndProcess() {
  if (!currentFileMd5) return;

  const rulesConfig = buildRulesConfig();

  fetch(`/api/files/${currentFileMd5}/rules`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ rulesConfig: JSON.stringify(rulesConfig) }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) {
        showFeedback(data.message, "error");
        return;
      }
      startProcessing();
    });
}

// ========== 处理流程 ==========

function startProcessing() {
  if (!currentFileMd5) return;

  fetch(`/api/files/${currentFileMd5}/run`, { method: "POST" })
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) {
        showFeedback(data.message, "error");
        return;
      }

      showSection("processing");
      startPolling();
    })
    .catch((err) => {
      showFeedback("启动处理失败: " + err.message, "error");
    });
}

function viewProgress(md5) {
  currentFileMd5 = md5;
  fetch(`/api/files/${md5}/status`)
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) return;

      if (data.status === "reviewing") {
        showSection("review");
        loadReviewItems();
      } else if (data.status === "completed") {
        showCompleted();
      } else {
        showSection("processing");
        updateProcessingUI(data);
        startPolling();
      }
    });
}

function startPolling() {
  stopPolling();
  pollingTimer = setInterval(() => {
    if (!currentFileMd5) {
      stopPolling();
      return;
    }

    fetch(`/api/files/${currentFileMd5}/status`)
      .then((r) => r.json())
      .then((data) => {
        if (!data.success) {
          stopPolling();
          return;
        }

        updateProcessingUI(data);

        if (data.status === "reviewing") {
          stopPolling();
          showSection("review");
          loadReviewItems();
        } else if (data.status === "completed") {
          stopPolling();
          showCompleted();
        } else if (data.status === "failed") {
          stopPolling();
          showFeedback("处理失败: " + (data.errorMsg || "未知错误"), "error");
        }
      });
  }, 2000);
}

function stopPolling() {
  if (pollingTimer) {
    clearInterval(pollingTimer);
    pollingTimer = null;
  }
}

function updateProcessingUI(data) {
  const progress = data.progress || 0;
  const progressBar = document.getElementById("overall-progress");
  const progressText = document.getElementById("progress-text");
  if (progressBar) progressBar.style.width = progress + "%";
  if (progressText) progressText.textContent = progress + "%";

  const stepMap = {
    cleaning: 0,
    indexing: 1,
    llm_fix: 2,
    review: 3,
    finalizing: 4,
  };
  const currentIdx = stepMap[data.currentStep] || 0;

  document.querySelectorAll(".step-item").forEach((el, idx) => {
    el.classList.remove("active", "completed");
    if (idx < currentIdx) el.classList.add("completed");
    else if (idx === currentIdx) el.classList.add("active");
  });

  const msgEl = document.getElementById("processing-message");
  if (msgEl) msgEl.textContent = data.message || "";

  const fileInfoEl = document.getElementById("file-info-display");
  if (fileInfoEl) {
    const parts = [];
    if (data.author) parts.push(data.author);
    if (data.title) parts.push(data.title);
    const displayName =
      parts.length > 0 ? parts.join(" - ") : data.fileName || currentFileMd5;
    fileInfoEl.textContent = `文件: ${displayName}`;
  }

  const actionEl = document.getElementById("current-action");
  if (actionEl) actionEl.textContent = data.currentAction || "";

  updateChunkProgress(data.chunkProgress);

  updateProcessingLogs(data.logs);
}

function updateChunkProgress(chunkProgress) {
  const container = document.getElementById("chunk-progress-container");
  if (!container) return;

  if (!chunkProgress || chunkProgress.totalChunks === 0) {
    container.style.display = "none";
    return;
  }

  container.style.display = "block";

  const progressBar = document.getElementById("chunk-progress-bar");
  const etaEl = document.getElementById("chunk-eta");
  const countEl = document.getElementById("chunk-count");
  const apiStatsEl = document.getElementById("chunk-api-stats");
  const avgTimeEl = document.getElementById("chunk-avg-time");

  const progress = chunkProgress.progress || 0;
  if (progressBar) progressBar.style.width = progress + "%";

  if (countEl) {
    countEl.textContent = `已处理: ${chunkProgress.processedChunks}/${chunkProgress.totalChunks} 块`;
  }

  if (apiStatsEl) {
    apiStatsEl.textContent = `API调用: ${chunkProgress.apiCalls}次 | 缓存命中: ${chunkProgress.cacheHits}次`;
  }

  if (avgTimeEl) {
    avgTimeEl.textContent = `平均耗时: ${chunkProgress.avgChunkTimeMs}ms/块`;
  }

  if (etaEl) {
    const remainingSecs = chunkProgress.estimatedRemainingSecs || 0;
    if (remainingSecs > 0) {
      etaEl.textContent = `预计剩余: ${formatEta(remainingSecs)}`;
    } else if (progress < 100) {
      etaEl.textContent = "计算中...";
    } else {
      etaEl.textContent = "处理完成";
    }
  }
}

function formatEta(seconds) {
  if (seconds < 60) {
    return `${seconds}秒`;
  }
  const minutes = Math.floor(seconds / 60);
  const remainingSecs = seconds % 60;
  if (minutes < 60) {
    return remainingSecs > 0 ? `${minutes}分${remainingSecs}秒` : `${minutes}分钟`;
  }
  const hours = Math.floor(minutes / 60);
  const remainingMins = minutes % 60;
  return `${hours}小时${remainingMins}分钟`;
}

function updateProcessingLogs(logs) {
  const listEl = document.getElementById("logs-list");
  if (!listEl || !logs || logs.length === 0) return;

  const stepLabels = {
    cleaning: "基础清洗",
    indexing: "向量检测",
    llm_fix: "LLM修复",
    review: "人工审核",
    finalizing: "生成文件",
  };

  const html = logs.map((log) => {
    const time = log.timestamp ? new Date(log.timestamp).toLocaleTimeString("zh-CN") : "";
    const step = stepLabels[log.step] || log.step || "";
    const detail = log.details || "";
    const statusClass = log.status === "running" ? "log-running" : log.status === "success" ? "log-success" : "log-default";
    return `<div class="log-item ${statusClass}"><span class="log-time">${time}</span><span class="log-step">[${step}]</span><span class="log-detail">${detail}</span></div>`;
  }).join("");

  listEl.innerHTML = html;
}

function toggleLogs() {
  const content = document.getElementById("logs-content");
  const icon = document.getElementById("logs-toggle-icon");
  if (!content || !icon) return;
  const isHidden = content.style.display === "none";
  content.style.display = isHidden ? "block" : "none";
  icon.textContent = isHidden ? "▲" : "▼";
}

function downloadCurrentVersion() {
  if (currentFileMd5)
    window.open(`/api/files/${currentFileMd5}/download`, "_blank");
}

// ========== 审核界面 ==========

function loadReviewItems() {
  if (!currentFileMd5) return;

  const statusFilter = document.getElementById("review-status-filter").value;

  fetch(`/api/files/${currentFileMd5}/review-items?status=${statusFilter}`)
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) return;

      reviewItems = data.suggestions || [];
      renderReviewItems();
      updateReviewProgress();
    });
}

function renderReviewItems() {
  const list = document.getElementById("review-items-list");
  if (!reviewItems || reviewItems.length === 0) {
    list.innerHTML = '<div class="empty-state">没有审核项</div>';
    return;
  }

  list.innerHTML = reviewItems
    .map((item) => {
      const confidenceHtml =
        item.confidence > 0
          ? `<span class="confidence-badge">置信度: ${(item.confidence * 100).toFixed(0)}%</span>`
          : "";

      let actionsHtml = "";
      if (item.status === "pending") {
        actionsHtml = `
        <div class="item-actions">
          <button class="btn-approve" onclick="approveItem(${item.id})">通过</button>
          <button class="btn-reject" onclick="rejectItem(${item.id})">拒绝</button>
          <button class="btn-edit" onclick="showEditDialog(${item.id})">编辑</button>
        </div>`;
      } else if (item.status === "approved" || item.status === "rejected") {
        actionsHtml = `
        <div class="item-actions">
          <button class="btn-restore" onclick="restoreItem(${item.id})">恢复</button>
        </div>`;
      } else if (item.status === "edited") {
        actionsHtml = `
        <div class="item-actions">
          <span class="edited-text">已编辑为: ${escapeHtml(item.editedText)}</span>
          <button class="btn-restore" onclick="restoreItem(${item.id})">恢复</button>
        </div>`;
      }

      const statusBadge = `<span class="status-badge ${item.status}">${getStatusText(item.status)}</span>`;

      return `
      <div class="review-item ${item.status}">
        <div class="item-header">
          <span class="line-number">第 ${item.lineNum} 行</span>
          ${statusBadge}
          ${confidenceHtml}
          <span class="item-type">${item.type}</span>
        </div>
        <div class="item-context">
          ${item.prevLine ? `<div class="context-line prev">${escapeHtml(item.prevLine)}</div>` : ""}
          <div class="current-line">${highlightInLine(item.fullLine, item.original)}</div>
          ${item.nextLine ? `<div class="context-line next">${escapeHtml(item.nextLine)}</div>` : ""}
        </div>
        <div class="item-detail">
          <div class="original"><strong>待修复:</strong> ${escapeHtml(item.original)}</div>
          <div class="suggested"><strong>修正为:</strong> ${escapeHtml(item.suggested)}</div>
        </div>
        ${actionsHtml}
      </div>`;
    })
    .join("");
}

function highlightInLine(fullLine, target) {
  if (!fullLine || !target) return escapeHtml(fullLine);
  const escaped = escapeHtml(fullLine);
  const escapedTarget = escapeHtml(target);
  return escaped.replace(escapedTarget, `<mark>${escapedTarget}</mark>`);
}

function approveItem(itemId) {
  fetch(`/api/files/${currentFileMd5}/approve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback("已通过");
        loadReviewItems();
      } else showFeedback(data.message, "error");
    });
}

function rejectItem(itemId) {
  fetch(`/api/files/${currentFileMd5}/reject`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback("已拒绝");
        loadReviewItems();
      } else showFeedback(data.message, "error");
    });
}

function showEditDialog(itemId) {
  const item = reviewItems.find((i) => i.id === itemId);
  if (!item) return;

  const editedText = prompt("请输入修改后的文本:", item.suggested);
  if (editedText === null) return;

  fetch(`/api/files/${currentFileMd5}/edit`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId, editedText }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback("已编辑");
        loadReviewItems();
      } else showFeedback(data.message, "error");
    });
}

function restoreItem(itemId) {
  fetch(`/api/files/${currentFileMd5}/restore`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemId }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback("已恢复");
        loadReviewItems();
      } else showFeedback(data.message, "error");
    });
}

function batchApproveAll() {
  const pendingItems = reviewItems.filter((i) => i.status === "pending");
  if (pendingItems.length === 0) {
    showFeedback("没有待审核项");
    return;
  }
  if (!confirm(`确定通过全部 ${pendingItems.length} 条待审核建议？`)) return;

  const itemIds = pendingItems.map((i) => i.id);
  fetch(`/api/files/${currentFileMd5}/batch-approve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemIds }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback(data.message);
        loadReviewItems();
      } else showFeedback(data.message, "error");
    });
}

function batchRejectAll() {
  const pendingItems = reviewItems.filter((i) => i.status === "pending");
  if (pendingItems.length === 0) {
    showFeedback("没有待审核项");
    return;
  }
  if (!confirm(`确定拒绝全部 ${pendingItems.length} 条待审核建议？`)) return;

  const itemIds = pendingItems.map((i) => i.id);
  fetch(`/api/files/${currentFileMd5}/batch-reject`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ itemIds }),
  })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback(data.message);
        loadReviewItems();
      } else showFeedback(data.message, "error");
    });
}

function updateReviewProgress() {
  fetch(`/api/files/${currentFileMd5}/status`)
    .then((r) => r.json())
    .then((data) => {
      if (!data.success) return;

      const progressText = document.getElementById("review-progress-text");
      if (progressText && data.reviewTotal !== undefined) {
        progressText.textContent = `${data.reviewResolved}/${data.reviewTotal}`;
      }

      const finalizeBtn = document.getElementById("finalize-btn");
      if (finalizeBtn) {
        const allDone =
          data.reviewTotal > 0 && data.reviewResolved === data.reviewTotal;
        finalizeBtn.style.display = allDone ? "inline-block" : "none";
      }
    });
}

function finalizeFile() {
  if (!confirm("确定完成审核并生成最终文件？")) return;

  fetch(`/api/files/${currentFileMd5}/finalize`, { method: "POST" })
    .then((r) => r.json())
    .then((data) => {
      if (data.success) {
        showFeedback(data.message);
        showCompleted();
      } else showFeedback(data.message, "error");
    });
}

// ========== 完成界面 ==========

function showCompleted() {
  showSection("completed");
  const infoEl = document.getElementById("completed-info");
  if (infoEl && currentFileMd5) {
    infoEl.innerHTML = `<p>文件处理完成！</p><p>MD5: ${currentFileMd5}</p>`;
  }
}

function downloadFinalFile() {
  if (currentFileMd5)
    window.open(`/api/files/${currentFileMd5}/download`, "_blank");
}

// ========== 初始化 ==========

document.addEventListener("DOMContentLoaded", () => {
  refreshFileList();

  const uploadArea = document.getElementById("upload-area");
  if (uploadArea) {
    uploadArea.addEventListener("dragover", (e) => {
      e.preventDefault();
      uploadArea.classList.add("drag-over");
    });
    uploadArea.addEventListener("dragleave", () => {
      uploadArea.classList.remove("drag-over");
    });
    uploadArea.addEventListener("drop", (e) => {
      e.preventDefault();
      uploadArea.classList.remove("drag-over");
      const file = e.dataTransfer.files[0];
      if (file) {
        const input = document.getElementById("file-input");
        const dt = new DataTransfer();
        dt.items.add(file);
        input.files = dt.files;
        handleFileUpload({ target: input });
      }
    });
  }
});
