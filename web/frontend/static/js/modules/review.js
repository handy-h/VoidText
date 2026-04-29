// 审核模块 — Diff 对比 + 同类修改分组批量处理
const ReviewModule = (function() {
  // 私有状态
  let reviewItems = [];
  let currentFileMd5 = null;

  // ---------------------------------------------------------------
  // 初始化
  // ---------------------------------------------------------------
  function init() {
    bindEvents();
  }

  function bindEvents() {
    // 状态筛选
    const statusFilter = document.getElementById('review-status-filter');
    if (statusFilter) {
      statusFilter.addEventListener('change', loadReviewItems);
    }

    // 全局批量通过
    const batchApproveBtn = document.getElementById('batch-approve-btn');
    if (batchApproveBtn) {
      batchApproveBtn.addEventListener('click', batchApproveAll);
    }

    // 全局批量拒绝
    const batchRejectBtn = document.getElementById('batch-reject-btn');
    if (batchRejectBtn) {
      batchRejectBtn.addEventListener('click', batchRejectAll);
    }

    // 完成审核
    const finalizeBtn = document.getElementById('finalize-btn');
    if (finalizeBtn) {
      finalizeBtn.addEventListener('click', finalizeFile);
    }
  }

  // ---------------------------------------------------------------
  // 数据加载
  // ---------------------------------------------------------------
  function loadReviewItems() {
    if (!currentFileMd5) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/review-items`)
      .then((data) => {
        if (!data.success) return;
        reviewItems = data.suggestions || data.items || [];
        renderReviewItems();
        updateReviewProgress();
      })
      .catch((err) => {
        console.error("加载审核项失败:", err);
      });
  }

  // ---------------------------------------------------------------
  // 分组逻辑
  // ---------------------------------------------------------------
  function groupReviewItems(items) {
    const groupMap = {};
    items.forEach(item => {
      const orig = item.original || item.originalText || '';
      const sugg = item.suggested || item.suggestedText || '';
      const key = orig + '|||' + sugg;
      if (!groupMap[key]) {
        groupMap[key] = {
          original: orig,
          suggested: sugg,
          items: [],
          count: 0
        };
      }
      groupMap[key].items.push(item);
      groupMap[key].count++;
    });
    return Object.values(groupMap);
  }

  // ---------------------------------------------------------------
  // 渲染
  // ---------------------------------------------------------------
  function renderReviewItems() {
    const container = document.getElementById("review-items-list");
    if (!container) return;

    container.innerHTML = "";

    const statusFilter = document.getElementById("review-status-filter");
    const filterValue = statusFilter ? statusFilter.value : "pending";

    const pendingItems = reviewItems.filter(item => item.status === 'pending');
    const processedItems = reviewItems.filter(item => item.status !== 'pending');

    // 检查 DiffUtils 是否可用
    const hasDiffUtils = typeof DiffUtils !== 'undefined' && DiffUtils.renderInlineDiff;

    if (filterValue === 'pending' || filterValue === 'all') {
      if (pendingItems.length > 0) {
        const groups = groupReviewItems(pendingItems);
        // 按出现次数降序排列
        groups.sort((a, b) => b.count - a.count);

        const pendingHeader = DomUtils.createElement('h3', {
          className: 'review-section-title'
        });
        DomUtils.setTextContent(pendingHeader,
          `待审核 (${pendingItems.length} 项，${groups.length} 类)`);
        container.appendChild(pendingHeader);

        groups.forEach(group => {
          const groupEl = createGroupElement(group, hasDiffUtils);
          container.appendChild(groupEl);
        });
      } else if (filterValue === 'pending') {
        const emptyDiv = DomUtils.createElement('div', { className: 'empty-state' });
        DomUtils.setTextContent(emptyDiv, '没有待审核项');
        container.appendChild(emptyDiv);
      }
    }

    if (filterValue === 'all' && processedItems.length > 0) {
      const processedHeader = DomUtils.createElement('h3', {
        className: 'review-section-title'
      });
      DomUtils.setTextContent(processedHeader,
        `已处理 (${processedItems.length} 项)`);
      container.appendChild(processedHeader);

      processedItems.forEach((item, idx) => {
        const itemEl = createProcessedItemElement(item, idx, hasDiffUtils);
        container.appendChild(itemEl);
      });
    }

    if (filterValue !== 'pending' && filterValue !== 'all') {
      const filteredItems = reviewItems.filter(item => item.status === filterValue);
      if (filteredItems.length === 0) {
        const emptyDiv = DomUtils.createElement('div', { className: 'empty-state' });
        DomUtils.setTextContent(emptyDiv, '没有符合条件的项');
        container.appendChild(emptyDiv);
      } else {
        filteredItems.forEach((item, idx) => {
          const itemEl = createProcessedItemElement(item, idx, hasDiffUtils);
          container.appendChild(itemEl);
        });
      }
    }
  }

  // ---------------------------------------------------------------
  // 组渲染
  // ---------------------------------------------------------------
  function createGroupElement(group, hasDiffUtils) {
    const groupDiv = DomUtils.createElement('div', { className: 'review-group' });

    // ---- 头部 ----
    const header = DomUtils.createElement('div', { className: 'review-group-header' });

    // Diff 对比摘要（聚焦变更部分，而非从头显示全文）
    const diffContainer = DomUtils.createElement('div', { className: 'review-group-diff' });

    const originalDiv = DomUtils.createElement('div', { className: 'review-original' });
    const origLabel = DomUtils.createElement('strong');
    DomUtils.setTextContent(origLabel, '原文: ');
    originalDiv.appendChild(origLabel);
    const origSpan = DomUtils.createElement('span');
    if (hasDiffUtils && group.original !== group.suggested) {
      const diffResult = DiffUtils.renderDiffPreview(group.original, group.suggested, 20);
      DomUtils.setHTML(origSpan, diffResult.originalHtml);
    } else {
      DomUtils.setTextContent(origSpan, group.original);
    }
    originalDiv.appendChild(origSpan);
    diffContainer.appendChild(originalDiv);

    const suggestedDiv = DomUtils.createElement('div', { className: 'review-suggested' });
    const suggLabel = DomUtils.createElement('strong');
    DomUtils.setTextContent(suggLabel, '建议: ');
    suggestedDiv.appendChild(suggLabel);
    const suggSpan = DomUtils.createElement('span');
    if (hasDiffUtils && group.original !== group.suggested) {
      const diffResult = DiffUtils.renderDiffPreview(group.original, group.suggested, 20);
      DomUtils.setHTML(suggSpan, diffResult.suggestedHtml);
    } else {
      DomUtils.setTextContent(suggSpan, group.suggested);
    }
    suggestedDiv.appendChild(suggSpan);
    diffContainer.appendChild(suggestedDiv);

    header.appendChild(diffContainer);

    // 计数徽标
    const countBadge = DomUtils.createElement('span', { className: 'review-group-count' });
    DomUtils.setTextContent(countBadge, `共 ${group.count} 处`);
    header.appendChild(countBadge);

    // 组批量操作按钮
    const actionsDiv = DomUtils.createElement('div', { className: 'review-group-actions' });

    const batchApproveBtn = DomUtils.createElement('button', {
      className: 'btn-approve',
      onclick: function(e) {
        e.stopPropagation();
        batchApproveGroup(group.items.map(i => i.id));
      }
    });
    DomUtils.setTextContent(batchApproveBtn, '全部通过');
    actionsDiv.appendChild(batchApproveBtn);

    const batchRejectBtn = DomUtils.createElement('button', {
      className: 'btn-reject',
      onclick: function(e) {
        e.stopPropagation();
        batchRejectGroup(group.items.map(i => i.id));
      }
    });
    DomUtils.setTextContent(batchRejectBtn, '全部拒绝');
    actionsDiv.appendChild(batchRejectBtn);

    header.appendChild(actionsDiv);

    // 点击头部折叠/展开
    header.addEventListener('click', function(e) {
      if (e.target.tagName === 'BUTTON') return;
      const body = this.nextElementSibling;
      if (body) body.classList.toggle('collapsed');
    });

    groupDiv.appendChild(header);

    // ---- 主体（可折叠） ----
    const body = DomUtils.createElement('div', { className: 'review-group-body collapsed' });
    group.items.forEach((item, idx) => {
      const itemEl = createGroupItemElement(item, idx);
      body.appendChild(itemEl);
    });
    groupDiv.appendChild(body);

    return groupDiv;
  }

  // 组内单个审核项
  function createGroupItemElement(item, index) {
    const itemDiv = DomUtils.createElement('div', { className: 'review-item-compact' });

    // 上下文信息行
    const contextDiv = DomUtils.createElement('div', { className: 'item-context' });
    const contextParts = [];
    if (item.lineNum !== undefined && item.lineNum !== null) {
      contextParts.push(`行 ${item.lineNum}`);
    }
    if (item.type || item.modificationType) {
      contextParts.push(getModificationTypeText(item.type || item.modificationType));
    }
    if (item.confidence !== undefined && item.confidence !== null) {
      contextParts.push(`置信度: ${(item.confidence * 100).toFixed(1)}%`);
    }
    contextParts.push(`#${item.id}`);
    DomUtils.setTextContent(contextDiv, contextParts.join(' | '));
    itemDiv.appendChild(contextDiv);

    // 上下文：前一行
    if (item.prevLine) {
      const prevDiv = DomUtils.createElement('div', { className: 'item-context-line' });
      DomUtils.setTextContent(prevDiv, item.prevLine);
      itemDiv.appendChild(prevDiv);
    }

    // 当前行（高亮）
    if (item.fullLine) {
      const fullDiv = DomUtils.createElement('div', {
        className: 'item-context-line item-context-line-highlight'
      });
      DomUtils.setTextContent(fullDiv, item.fullLine);
      itemDiv.appendChild(fullDiv);
    }

    // 上下文：后一行
    if (item.nextLine) {
      const nextDiv = DomUtils.createElement('div', { className: 'item-context-line' });
      DomUtils.setTextContent(nextDiv, item.nextLine);
      itemDiv.appendChild(nextDiv);
    }

    // 操作按钮
    const actionsDiv = DomUtils.createElement('div', { className: 'item-actions' });

    const approveBtn = DomUtils.createElement('button', {
      className: 'btn-approve',
      onclick: function() { approveReviewItem(item.id); }
    });
    DomUtils.setTextContent(approveBtn, '通过');
    actionsDiv.appendChild(approveBtn);

    const rejectBtn = DomUtils.createElement('button', {
      className: 'btn-reject',
      onclick: function() { rejectReviewItem(item.id); }
    });
    DomUtils.setTextContent(rejectBtn, '拒绝');
    actionsDiv.appendChild(rejectBtn);

    const editBtn = DomUtils.createElement('button', {
      className: 'btn-edit',
      onclick: function() { editReviewItem(item.id); }
    });
    DomUtils.setTextContent(editBtn, '编辑');
    actionsDiv.appendChild(editBtn);

    const statusSpan = DomUtils.createElement('span', {
      className: 'review-status review-status-pending'
    });
    DomUtils.setTextContent(statusSpan, getReviewStatusText(item.status));
    actionsDiv.appendChild(statusSpan);

    itemDiv.appendChild(actionsDiv);

    return itemDiv;
  }

  // 已处理项渲染
  function createProcessedItemElement(item, index, hasDiffUtils) {
    const itemDiv = DomUtils.createElement('div', { className: 'review-item' });

    // 头部
    const header = DomUtils.createElement('div', { className: 'review-item-header' });

    const typeSpan = DomUtils.createElement('span', { className: 'review-item-type' });
    DomUtils.setTextContent(typeSpan,
      getModificationTypeText(item.type || item.modificationType));
    header.appendChild(typeSpan);

    if (item.lineNum !== undefined && item.lineNum !== null) {
      const lineSpan = DomUtils.createElement('span');
      DomUtils.setTextContent(lineSpan, `行 ${item.lineNum}`);
      header.appendChild(lineSpan);
    }

    if (item.confidence !== undefined && item.confidence !== null) {
      const confSpan = DomUtils.createElement('span');
      DomUtils.setTextContent(confSpan,
        `置信度: ${(item.confidence * 100).toFixed(1)}%`);
      header.appendChild(confSpan);
    }

    itemDiv.appendChild(header);

    // 内容
    const contentDiv = DomUtils.createElement('div', { className: 'review-item-content' });

    const orig = item.original || item.originalText || '';
    const sugg = item.suggested || item.suggestedText || '';

    if (orig || sugg) {
      const originalDiv = DomUtils.createElement('div', { className: 'review-original' });
      const origLabel = DomUtils.createElement('strong');
      DomUtils.setTextContent(origLabel, '原文: ');
      originalDiv.appendChild(origLabel);
      const origText = DomUtils.createElement('span');
      if (item.status === 'rejected' && hasDiffUtils && sugg) {
        DomUtils.setHTML(origText, DiffUtils.renderInlineDiff(orig, sugg).originalHtml);
      } else {
        DomUtils.setTextContent(origText, orig);
      }
      originalDiv.appendChild(origText);
      contentDiv.appendChild(originalDiv);

      const suggestedDiv = DomUtils.createElement('div', { className: 'review-suggested' });
      const suggLabel = DomUtils.createElement('strong');
      DomUtils.setTextContent(suggLabel, '建议: ');
      suggestedDiv.appendChild(suggLabel);
      const suggText = DomUtils.createElement('span');
      if (item.status === 'approved' && hasDiffUtils && orig) {
        DomUtils.setHTML(suggText, DiffUtils.renderInlineDiff(orig, sugg).suggestedHtml);
      } else {
        DomUtils.setTextContent(suggText, sugg);
      }
      suggestedDiv.appendChild(suggText);
      contentDiv.appendChild(suggestedDiv);
    }

    if (item.editedText) {
      const editedDiv = DomUtils.createElement('div', { className: 'review-edited' });
      const editedLabel = DomUtils.createElement('strong');
      DomUtils.setTextContent(editedLabel, '编辑后: ');
      editedDiv.appendChild(editedLabel);
      const editedTextEl = DomUtils.createElement('span');
      DomUtils.setTextContent(editedTextEl, item.editedText);
      editedDiv.appendChild(editedTextEl);
      contentDiv.appendChild(editedDiv);
    }

    itemDiv.appendChild(contentDiv);

    // 操作
    const actionsDiv = DomUtils.createElement('div', { className: 'review-item-actions' });

    if (item.status === 'edited') {
      const restoreBtn = DomUtils.createElement('button', {
        className: 'btn-restore',
        onclick: function() { restoreReviewItem(item.id); }
      });
      DomUtils.setTextContent(restoreBtn, '恢复原文');
      actionsDiv.appendChild(restoreBtn);
    }

    const statusSpan = DomUtils.createElement('span', {
      className: 'review-status review-status-' + item.status
    });
    DomUtils.setTextContent(statusSpan, getReviewStatusText(item.status));
    actionsDiv.appendChild(statusSpan);

    itemDiv.appendChild(actionsDiv);

    return itemDiv;
  }

  // ---------------------------------------------------------------
  // 类型 / 状态文本
  // ---------------------------------------------------------------
  function getModificationTypeText(type) {
    const map = {
      typo: '错别字',
      typo_correction: '错别字修正',
      duplicate_paragraph: '重复段落',
      advertisement: '广告',
      grammar: '语法错误',
      style: '风格问题',
      character_correction: '错误内容',
      text_deletion: '多余内容',
      text_insertion: '缺失内容',
      llm_fix: '智能修复',
      punctuation: '标点符号错用',
      html_entity: 'HTML实体',
      whitespace: '空白格式',
      encoding_fix: '编码修复',
      garbled_text_removal: '乱码清除',
      traditional_to_simple: '繁转简',
    };
    return map[type] || type || '';
  }

  function getReviewStatusText(status) {
    const map = {
      pending: '待审核',
      approved: '已通过',
      rejected: '已拒绝',
      edited: '已编辑'
    };
    return map[status] || status || '';
  }

  // ---------------------------------------------------------------
  // 单条操作
  // ---------------------------------------------------------------
  function approveReviewItem(itemId) {
    if (!currentFileMd5) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/approve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId: itemId })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已通过修改建议', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '操作失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('操作失败: ' + err.message, 'error');
      });
  }

  function rejectReviewItem(itemId) {
    if (!currentFileMd5) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/reject`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId: itemId })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已拒绝修改建议', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '操作失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('操作失败: ' + err.message, 'error');
      });
  }

  function editReviewItem(itemId) {
    const item = reviewItems.find(i => i.id === itemId);
    if (!item) return;

    const editDefault = item.suggested || item.suggestedText ||
      item.original || item.originalText || '';
    const editedText = prompt('请输入编辑后的文本:', editDefault);
    if (editedText === null) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/edit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId: itemId, editedText: editedText })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已保存编辑', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '编辑失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('编辑失败: ' + err.message, 'error');
      });
  }

  function restoreReviewItem(itemId) {
    AppConfig.apiRequest(`/files/${currentFileMd5}/restore`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId: itemId })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已恢复原文', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '恢复失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('恢复失败: ' + err.message, 'error');
      });
  }

  // ---------------------------------------------------------------
  // 组批量操作
  // ---------------------------------------------------------------
  function batchApproveGroup(itemIds) {
    if (!currentFileMd5 || !itemIds.length) return;
    if (!confirm(`确定通过这 ${itemIds.length} 项修改？`)) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-approve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemIds: itemIds })
    })
      .then((data) => {
        if (data.success) {
          showFeedback(`已通过 ${itemIds.length} 项修改`, 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '批量操作失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('批量操作失败: ' + err.message, 'error');
      });
  }

  function batchRejectGroup(itemIds) {
    if (!currentFileMd5 || !itemIds.length) return;
    if (!confirm(`确定拒绝这 ${itemIds.length} 项修改？`)) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-reject`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemIds: itemIds })
    })
      .then((data) => {
        if (data.success) {
          showFeedback(`已拒绝 ${itemIds.length} 项修改`, 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '批量操作失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('批量操作失败: ' + err.message, 'error');
      });
  }

  // ---------------------------------------------------------------
  // 全局批量操作
  // ---------------------------------------------------------------
  function batchApproveAll() {
    if (!currentFileMd5 || !confirm('确定通过所有待审核项？')) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-approve`, {
      method: 'POST'
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已通过所有待审核项', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '操作失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('批量操作失败: ' + err.message, 'error');
      });
  }

  function batchRejectAll() {
    if (!currentFileMd5 || !confirm('确定拒绝所有待审核项？')) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-reject`, {
      method: 'POST'
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已拒绝所有待审核项', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '操作失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('批量操作失败: ' + err.message, 'error');
      });
  }

  // ---------------------------------------------------------------
  // 完成审核
  // ---------------------------------------------------------------
  function finalizeFile() {
    if (!currentFileMd5 || !confirm('确定完成审核并生成最终文件？')) return;

    AppConfig.apiRequest(`/files/${currentFileMd5}/finalize`, {
      method: 'POST'
    })
      .then((data) => {
        if (data.success) {
          showFeedback('文件处理完成', 'success');
          FileManager.showSection('completed');
        } else {
          showFeedback(data.message || '完成处理失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('完成处理失败: ' + err.message, 'error');
      });
  }

  // ---------------------------------------------------------------
  // 进度更新
  // ---------------------------------------------------------------
  function updateReviewProgress() {
    const total = reviewItems.length;
    const pending = reviewItems.filter(item => item.status === 'pending').length;
    const progressText = document.getElementById('review-progress-text');
    const finalizeBtn = document.getElementById('finalize-btn');

    if (progressText) {
      DomUtils.setTextContent(progressText, `${total - pending}/${total}`);
    }

    if (finalizeBtn) {
      finalizeBtn.style.display = pending === 0 ? 'inline-block' : 'none';
    }
  }

  // ---------------------------------------------------------------
  // Feedback
  // ---------------------------------------------------------------
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

  // ---------------------------------------------------------------
  // 公共 API
  // ---------------------------------------------------------------
  return {
    init: init,
    loadReviewItems: loadReviewItems,
    renderReviewItems: renderReviewItems,
    approveReviewItem: approveReviewItem,
    rejectReviewItem: rejectReviewItem,
    editReviewItem: editReviewItem,
    restoreReviewItem: restoreReviewItem,
    batchApproveGroup: batchApproveGroup,
    batchRejectGroup: batchRejectGroup,
    batchApproveAll: batchApproveAll,
    batchRejectAll: batchRejectAll,
    finalizeFile: finalizeFile,
    setCurrentFileMd5: function(md5) { currentFileMd5 = md5; }
  };
})();

// 导出到全局作用域
window.ReviewModule = ReviewModule;
