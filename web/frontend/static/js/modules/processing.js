// 处理进度模块
const ProcessingModule = (function () {
  // 私有变量
  let pollingTimer = null;
  let currentFileMd5 = null;

  // 初始化
  function init() {
    // 绑定事件
    bindEvents();
  }

  // 绑定事件
  function bindEvents() {
    // 处理日志切换
    const logsHeader = document.getElementById("logs-header");
    if (logsHeader) {
      logsHeader.addEventListener("click", toggleLogs);
    }
  }

  // 查看处理进度
  function viewProgress(md5) {
    currentFileMd5 = md5;
    FileManager.showSection("processing");
    startPolling();
  }

  // 开始轮询
  function startPolling() {
    stopPolling();
    pollingTimer = setInterval(updateProgress, 2000);
    updateProgress();
  }

  // 停止轮询
  function stopPolling() {
    if (pollingTimer) {
      clearInterval(pollingTimer);
      pollingTimer = null;
    }
  }

  // 更新进度
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
        updateLogs(data.logs);
        updateChunkProgress(data.chunkProgress);

        if (
          data.status === "completed" ||
          data.status === "failed" ||
          data.status === "reviewing"
        ) {
          stopPolling();
          if (data.status === "reviewing") {
            FileManager.showSection("review");
            ReviewModule.loadReviewItems();
          } else if (data.status === "completed") {
            FileManager.showSection("completed");
            updateCompletedInfo();
          }
        }
      })
      .catch((err) => {
        console.error("轮询失败:", err);
      });
  }

  // 更新步骤进度
  function updateStepProgress(currentStep) {
    const steps = ["cleaning", "indexing", "llm_fix", "review", "finalizing"];
    const stepIndex = steps.indexOf(currentStep);

    steps.forEach((step, index) => {
      const stepItem = document.querySelector(
        `.step-item[data-step="${step}"]`,
      );
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

  // 获取状态文本
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

  // 获取步骤文本
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

  // 切换日志显示
  function updateLogs(logs) {
    const logsList = document.getElementById("logs-list");
    if (!logsList || !logs || !logs.length) return;

    logsList.innerHTML = "";

    const reversedLogs = [...logs].reverse();
    reversedLogs.forEach((logEntry) => {
      const logItem = DomUtils.createElement("div", {
        className: "log-item log-" + (logEntry.status || "info"),
      });

      const timeSpan = DomUtils.createElement("span", {
        className: "log-time",
      });
      const ts = logEntry.timestamp || "";
      DomUtils.setTextContent(
        timeSpan,
        ts ? new Date(ts).toLocaleTimeString("zh-CN") : "",
      );
      logItem.appendChild(timeSpan);

      const stepSpan = DomUtils.createElement("span", {
        className: "log-step",
      });
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
                parts.push(
                  "重复: " + parsed.details.result.duplicates_detected,
                );
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

    const logsContent = document.getElementById("logs-content");
    if (logsContent && logsContent.style.display !== "none") {
      logsList.scrollTop = 0;
    }
  }

  function updateChunkProgress(chunkProgress) {
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

  // 更新完成信息
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

  // 格式化文件大小
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

  // 格式化时间
  function formatTime(ts) {
    if (!ts) return "";
    try {
      return new Date(ts).toLocaleString("zh-CN");
    } catch {
      return ts;
    }
  }

  // 公共API
  return {
    init,
    viewProgress,
    startPolling,
    stopPolling,
    updateProgress,
    getCurrentFileMd5: () => currentFileMd5,
    setCurrentFileMd5: (md5) => {
      currentFileMd5 = md5;
    },
  };
})();

// 导出到全局作用域
window.ProcessingModule = ProcessingModule;
