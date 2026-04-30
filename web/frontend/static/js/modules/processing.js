// 处理进度模块
const ProcessingModule = (function () {
  // 私有变量
  let pollingTimer = null;
  let currentFileMd5 = null;
  let previousStatus = null; // 记录上次轮询到的状态，用于检测状态转换

  // 初始化
  function init() {
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
    // 同步到全局，使下载等操作可用
    window.currentFileMd5 = md5;
    // 重置状态跟踪，后续轮询检测到状态变化时才自动跳转
    previousStatus = null;
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

  // 检查自动跳转开关
  function isAutoNavEnabled() {
    const toggle = document.getElementById('auto-nav-review');
    return toggle ? toggle.checked : true;
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
          let message = '\u72B6\u6001: ' + getStatusText(data.status);
          if (data.currentStep) {
            message += ' | \u5F53\u524D\u6B65\u9AA4: ' + getStepText(data.currentStep);
          }
          DomUtils.setTextContent(processingMessage, message);
        }

        if (currentAction && data.currentAction) {
          DomUtils.setTextContent(currentAction, data.currentAction);
        }

        updateStepProgress(data.currentStep);
        updateLogs(data.logs);
        updateChunkProgress(data.chunkProgress);

        // 检测状态转换：仅当状态从上一次轮询发生了变化才自动跳转
        // 用户主动点"查看进度"时 previousStatus=null，不会触发跳转
        // 流水线完成的自动跳转则 previousStatus 会从 processing→reviewing
        var statusChanged = previousStatus !== null && previousStatus !== data.status;
        previousStatus = data.status;

        if (
          data.status === "completed" ||
          data.status === "failed" ||
          data.status === "reviewing"
        ) {
          if (statusChanged) {
            stopPolling();

            if (data.status === "reviewing") {
              // 根据开关决定是否自动跳转
              if (isAutoNavEnabled()) {
                FileManager.showSection("review");
                ReviewModule.setCurrentFileMd5(currentFileMd5);
                ReviewModule.loadReviewItems();
              } else {
                showFeedback('\u5904\u7406\u5B8C\u6210\uFF0C\u6B63\u5728\u7B49\u5F85\u5BA1\u6838', 'success');
                FileManager.refreshFileList();
                FileManager.showSection('file-list');
              }
            } else if (data.status === "completed") {
              FileManager.showSection("completed");
              updateCompletedInfo();
            } else if (data.status === "failed") {
              showFeedback('\u5904\u7406\u5931\u8D25: ' + (data.error || '\u672A\u77E5\u9519\u8BEF'), 'error');
              FileManager.refreshFileList();
              FileManager.showSection('file-list');
            }
          } else {
            // 状态未变化（用户主动查看已完成/审核中的状态页）
            // 停留在当前进度页面，让用户查看并下载
            stopPolling();
          }
        }
      })
      .catch((err) => {
        console.error("\u8F6E\u8BE2\u5931\u8D25:", err);
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
      pending: "\u5F85\u5904\u7406",
      processing: "\u5904\u7406\u4E2D",
      reviewing: "\u5BA1\u6838\u4E2D",
      completed: "\u5DF2\u5B8C\u6210",
      failed: "\u5931\u8D25",
    };
    return map[status] || status;
  }

  // 获取步骤文本
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

  // 更新日志
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
            step_started: "\u6B65\u9AA4\u5F00\u59CB",
            step_completed: "\u6B65\u9AA4\u5B8C\u6210",
            step_skipped: "\u6B65\u9AA4\u8DF3\u8FC7",
            step_failed: "\u6B65\u9AA4\u5931\u8D25",
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
                parts.push("\u4FEE\u6539\u6570: " + parsed.details.result.changes_count);
              }
              if (parsed.details.result.duplicates_detected !== undefined) {
                parts.push(
                  "\u91CD\u590D: " + parsed.details.result.duplicates_detected,
                );
              }
              if (parsed.details.result.total_chunks !== undefined) {
                parts.push("\u5757\u6570: " + parsed.details.result.total_chunks);
              }
              if (parsed.details.result.total_changes !== undefined) {
                parts.push("\u53D8\u66F4: " + parsed.details.result.total_changes);
              }
              if (parsed.details.result.cache_hits !== undefined) {
                parts.push("\u7F13\u5B58\u547D\u4E2D: " + parsed.details.result.cache_hits);
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

  // 更新块进度
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
        "\u5DF2\u5904\u7406: " + chunkProgress.processedChunks + "/" + chunkProgress.totalChunks + " \u5757",
      );
    }

    const statsEl = document.getElementById("chunk-api-stats");
    if (statsEl) {
      DomUtils.setTextContent(
        statsEl,
        "API\u8C03\u7528: " + (chunkProgress.apiCalls || 0) + "\u6B21 | \u7F13\u5B58\u547D\u4E2D: " + (chunkProgress.cacheHits || 0) + "\u6B21",
      );
    }

    const avgEl = document.getElementById("chunk-avg-time");
    if (avgEl) {
      DomUtils.setTextContent(
        avgEl,
        "\u5E73\u5747\u8017\u65F6: " + (chunkProgress.avgChunkTimeMs || 0) + "ms/\u5757",
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
            ? "\u9884\u8BA1\u5269\u4F59: " + mins + "\u5206" + remainSecs + "\u79D2"
            : "\u9884\u8BA1\u5269\u4F59: " + remainSecs + "\u79D2",
        );
      } else {
        DomUtils.setTextContent(etaEl, "");
      }
    }
  }

  // 切换日志显示
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
        if (file.author) parts.push("\u4F5C\u8005: " + file.author);
        parts.push("\u5927\u5C0F: " + formatFileSize(file.fileSize));
        parts.push("\u72B6\u6001: " + getStatusText(file.status));
        parts.push("\u5B8C\u6210\u65F6\u95F4: " + formatTime(file.updatedAt));
        DomUtils.setTextContent(details, parts.join(" | "));
        info.appendChild(details);

        infoDiv.innerHTML = "";
        infoDiv.appendChild(info);
      })
      .catch((err) => {
        console.error("\u83B7\u53D6\u5B8C\u6210\u4FE1\u606F\u5931\u8D25:", err);
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

  // 显示反馈
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
