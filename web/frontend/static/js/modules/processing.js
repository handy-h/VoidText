// 处理进度模块
const ProcessingModule = (function () {
  let pollingTimer = null;
  let currentFileMd5 = null;
  let previousStatus = null;
  let isUpdating = false;   // 竞态守卫：防止并发轮询请求
  let lastLogsJson = null;  // 脏检测：日志未变化时跳过 DOM 重建

  function init() {
    bindEvents();
  }

  function bindEvents() {
    const logsHeader = document.getElementById('logs-header');
    if (logsHeader) {
      logsHeader.addEventListener('click', toggleLogs);
    }
  }

  function viewProgress(md5) {
    currentFileMd5 = md5;
    window.currentFileMd5 = md5;
    previousStatus = null;
    FileManager.showSection('processing');
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

  function isAutoNavEnabled() {
    const toggle = document.getElementById('auto-nav-review');
    return toggle ? toggle.checked : true;
  }

  function updateProgress() {
    if (!currentFileMd5 || isUpdating) return;
    isUpdating = true;

    AppConfig.apiRequest(`/files/${currentFileMd5}/status`)
      .then((data) => {
        if (!data.success) return;

        const progressBar = document.getElementById('overall-progress');
        const progressText = document.getElementById('progress-text');
        const processingMessage = document.getElementById('processing-message');
        const currentAction = document.getElementById('current-action');

        if (progressBar && progressText) {
          const progress = data.progress || 0;
          progressBar.style.width = progress + '%';
          DomUtils.setTextContent(progressText, progress + '%');
        }

        if (processingMessage) {
          let message = '状态: ' + DomUtils.getStatusText(data.status);
          if (data.currentStep) {
            message += ' | 当前步骤: ' + DomUtils.getStepText(data.currentStep);
          }
          DomUtils.setTextContent(processingMessage, message);
        }

        if (currentAction && data.currentAction) {
          DomUtils.setTextContent(currentAction, data.currentAction);
        }

        updateStepProgress(data.currentStep);
        updateLogs(data.logs);
        updateChunkProgress(data.chunkProgress);

        const statusChanged = previousStatus !== null && previousStatus !== data.status;
        previousStatus = data.status;

        if (data.status === 'completed' || data.status === 'failed' || data.status === 'reviewing') {
          stopPolling();
          if (data.status === 'reviewing') {
            if (statusChanged && isAutoNavEnabled()) {
              FileManager.showSection('review');
              ReviewModule.setCurrentFileMd5(currentFileMd5);
              ReviewModule.loadReviewItems();
            } else if (statusChanged) {
              DomUtils.showFeedback('处理完成，正在等待审核', 'success');
              FileManager.refreshFileList();
              FileManager.showSection('file-list');
            } else {
              FileManager.showSection('review');
              ReviewModule.setCurrentFileMd5(currentFileMd5);
              ReviewModule.loadReviewItems();
            }
          } else if (data.status === 'completed') {
            FileManager.showSection('completed');
            updateCompletedInfo();
          } else if (data.status === 'failed') {
            DomUtils.showFeedback('处理失败: ' + (data.errorMsg || data.error || '未知错误'), 'error');
            FileManager.refreshFileList();
            FileManager.showSection('file-list');
          }
        }
      })
      .catch((err) => {
        console.error('轮询失败:', err);
      })
      .finally(() => {
        isUpdating = false;
      });
  }

  function updateStepProgress(currentStep) {
    const steps = ['cleaning', 'indexing', 'llm_fix', 'review', 'finalizing'];
    const stepIndex = steps.indexOf(currentStep);

    steps.forEach((step, index) => {
      const stepItem = document.querySelector(`.step-item[data-step="${step}"]`);
      if (stepItem) {
        stepItem.classList.remove('active', 'completed');
        if (index < stepIndex) {
          stepItem.classList.add('completed');
        } else if (index === stepIndex) {
          stepItem.classList.add('active');
        }
      }
    });
  }

  function updateLogs(logs) {
    const logsList = document.getElementById('logs-list');
    if (!logsList || !logs || !logs.length) return;

    // 脏检测：日志内容未变则跳过 DOM 重建
    const logsJson = JSON.stringify(logs);
    if (logsJson === lastLogsJson) return;
    lastLogsJson = logsJson;

    const fragment = document.createDocumentFragment();
    const reversedLogs = [...logs].reverse();

    reversedLogs.forEach((logEntry) => {
      const logItem = DomUtils.createElement('div', {
        className: 'log-item log-' + (logEntry.status || 'info'),
      });

      const timeSpan = DomUtils.createElement('span', { className: 'log-time' });
      const ts = logEntry.timestamp || '';
      DomUtils.setTextContent(timeSpan, ts ? new Date(ts).toLocaleTimeString('zh-CN') : '');
      logItem.appendChild(timeSpan);

      const stepSpan = DomUtils.createElement('span', { className: 'log-step' });
      DomUtils.setTextContent(stepSpan, DomUtils.getStepText(logEntry.step));
      logItem.appendChild(stepSpan);

      const msgSpan = DomUtils.createElement('span', { className: 'log-message' });
      let detail = logEntry.details || logEntry.action || '';
      let dedupPanel = null;
      try {
        const parsed = JSON.parse(detail);
        if (parsed.action === 'vector_dedup_complete') {
          // 向量去重统计：渲染徽章 + 展开详情
          const dupCount = parsed.duplicate_paragraphs || 0;
          const charCount = parsed.duplicate_chars || 0;
          const contents = parsed.removed_contents || [];

          const badgeP = DomUtils.createElement('span', { className: 'dedup-badge dedup-badge-paragraph' });
          DomUtils.setTextContent(badgeP, '去除 ' + dupCount + ' 段');
          msgSpan.appendChild(badgeP);

          const badgeC = DomUtils.createElement('span', { className: 'dedup-badge dedup-badge-chars' });
          DomUtils.setTextContent(badgeC, '减少 ' + charCount + ' 字');
          msgSpan.appendChild(badgeC);

          if (contents.length > 0) {
            const toggleBtn = DomUtils.createElement('button', { className: 'dedup-detail-toggle' });
            DomUtils.setTextContent(toggleBtn, '查看详情 ▼');
            msgSpan.appendChild(toggleBtn);

            dedupPanel = DomUtils.createElement('div', { className: 'dedup-detail-panel' });
            if (parsed.truncated) {
              const hint = DomUtils.createElement('div', { className: 'dedup-truncated-hint' });
              DomUtils.setTextContent(hint, '…等共 ' + (parsed.total_removed || contents.length) + ' 段（已截断）');
              dedupPanel.appendChild(hint);
            }
            contents.forEach(function (text, idx) {
              const pItem = DomUtils.createElement('div', { className: 'dedup-paragraph' });
              const numSpan = DomUtils.createElement('span', { className: 'dedup-paragraph-num' });
              DomUtils.setTextContent(numSpan, '#' + (idx + 1));
              pItem.appendChild(numSpan);
              const textSpan = DomUtils.createElement('span', { className: 'dedup-paragraph-text' });
              DomUtils.setTextContent(textSpan, text);
              pItem.appendChild(textSpan);
              dedupPanel.appendChild(pItem);
            });

            toggleBtn.addEventListener('click', function () {
              const visible = dedupPanel.style.display !== 'none';
              dedupPanel.style.display = visible ? 'none' : 'block';
              DomUtils.setTextContent(toggleBtn, visible ? '查看详情 ▼' : '收起 ▲');
            });
          }

          detail = null; // 标记已处理，跳过纯文本设置
        } else if (parsed.action) {
          const actionMap = {
            step_started: '步骤开始',
            step_completed: '步骤完成',
            step_skipped: '步骤跳过',
            step_failed: '步骤失败',
          };
          detail = actionMap[parsed.action] || parsed.action;
          if (parsed.details) {
            if (parsed.details.step) {
              detail += ' - ' + DomUtils.getStepText(parsed.details.step);
            }
            if (parsed.details.reason) {
              detail += ' (' + parsed.details.reason + ')';
            }
            if (parsed.details.result) {
              const parts = [];
              if (parsed.details.result.changes_count !== undefined) parts.push('修改数: ' + parsed.details.result.changes_count);
              if (parsed.details.result.duplicates_detected !== undefined) parts.push('重复: ' + parsed.details.result.duplicates_detected);
              if (parsed.details.result.total_chunks !== undefined) parts.push('块数: ' + parsed.details.result.total_chunks);
              if (parsed.details.result.total_changes !== undefined) parts.push('变更: ' + parsed.details.result.total_changes);
              if (parsed.details.result.cache_hits !== undefined) parts.push('缓存命中: ' + parsed.details.result.cache_hits);
              if (parts.length > 0) detail += ' [' + parts.join(', ') + ']';
            }
          }
        }
      } catch (e) {
        // detail 是普通字符串
      }
      if (detail !== null) {
        DomUtils.setTextContent(msgSpan, detail);
      }
      logItem.appendChild(msgSpan);

      fragment.appendChild(logItem);
      if (dedupPanel) {
        fragment.appendChild(dedupPanel);
      }
    });

    logsList.innerHTML = '';
    logsList.appendChild(fragment);
  }

  function updateChunkProgress(chunkProgress) {
    const container = document.getElementById('chunk-progress-container');
    if (!container) return;

    if (!chunkProgress || chunkProgress.totalChunks === 0) {
      container.style.display = 'none';
      return;
    }

    container.style.display = 'block';

    const bar = document.getElementById('chunk-progress-bar');
    if (bar) bar.style.width = (chunkProgress.progress || 0) + '%';

    const countEl = document.getElementById('chunk-count');
    if (countEl) {
      DomUtils.setTextContent(countEl, '已处理: ' + chunkProgress.processedChunks + '/' + chunkProgress.totalChunks + ' 块');
    }

    const statsEl = document.getElementById('chunk-api-stats');
    if (statsEl) {
      DomUtils.setTextContent(statsEl, 'API调用: ' + (chunkProgress.apiCalls || 0) + '次 | 缓存命中: ' + (chunkProgress.cacheHits || 0) + '次');
    }

    const avgEl = document.getElementById('chunk-avg-time');
    if (avgEl) {
      DomUtils.setTextContent(avgEl, '平均耗时: ' + (chunkProgress.avgChunkTimeMs || 0) + 'ms/块');
    }

    const etaEl = document.getElementById('chunk-eta');
    if (etaEl) {
      const secs = chunkProgress.estimatedRemainingSecs || 0;
      if (secs > 0) {
        const mins = Math.floor(secs / 60);
        const remainSecs = secs % 60;
        DomUtils.setTextContent(etaEl, mins > 0 ? '预计剩余: ' + mins + '分' + remainSecs + '秒' : '预计剩余: ' + remainSecs + '秒');
      } else {
        DomUtils.setTextContent(etaEl, '');
      }
    }
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

  function updateCompletedInfo() {
    const infoDiv = document.getElementById('completed-info');
    if (!infoDiv || !currentFileMd5) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}`)
      .then((data) => {
        if (!data.success) return;

        const file = data.file;
        const info = DomUtils.createElement('div');

        const title = DomUtils.createElement('h3');
        DomUtils.setTextContent(title, file.title || file.fileName);
        info.appendChild(title);

        const details = DomUtils.createElement('p');
        const parts = [];
        if (file.author) parts.push('作者: ' + file.author);
        parts.push('大小: ' + DomUtils.formatFileSize(file.fileSize));
        parts.push('状态: ' + DomUtils.getStatusText(file.status));
        parts.push('完成时间: ' + DomUtils.formatTime(file.updatedAt));
        DomUtils.setTextContent(details, parts.join(' | '));
        info.appendChild(details);

        infoDiv.innerHTML = '';
        infoDiv.appendChild(info);
      })
      .catch((err) => {
        console.error('获取完成信息失败:', err);
      });
  }

  return {
    init,
    viewProgress,
    startPolling,
    stopPolling,
    updateProgress,
    getCurrentFileMd5: () => currentFileMd5,
    setCurrentFileMd5: (md5) => { currentFileMd5 = md5; },
  };
})();

window.ProcessingModule = ProcessingModule;
