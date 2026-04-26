// 审核模块
const ReviewModule = (function() {
  // 私有变量
  let reviewItems = [];
  let currentFileMd5 = null;
  
  // 初始化
  function init() {
    // 绑定事件
    bindEvents();
  }
  
  // 绑定事件
  function bindEvents() {
    // 审核状态筛选
    const statusFilter = document.getElementById('review-status-filter');
    if (statusFilter) {
      statusFilter.addEventListener('change', renderReviewItems);
    }
    
    // 批量操作按钮
    const batchApproveBtn = document.getElementById('batch-approve-btn');
    if (batchApproveBtn) {
      batchApproveBtn.addEventListener('click', batchApproveAll);
    }
    
    const batchRejectBtn = document.getElementById('batch-reject-btn');
    if (batchRejectBtn) {
      batchRejectBtn.addEventListener('click', batchRejectAll);
    }
    
    // 完成审核按钮
    const finalizeBtn = document.getElementById('finalize-btn');
    if (finalizeBtn) {
      finalizeBtn.addEventListener('click', finalizeFile);
    }
  }
  
  // 加载审核项
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
  
  // 渲染审核项
  function renderReviewItems() {
    const container = document.getElementById("review-items-list");
    if (!container) return;
    
    container.innerHTML = "";
    
    const statusFilter = document.getElementById("review-status-filter");
    const filterValue = statusFilter ? statusFilter.value : "pending";
    
    const filteredItems = reviewItems.filter(item => {
      if (filterValue === "all") return true;
      return item.status === filterValue;
    });
    
    filteredItems.forEach((item, index) => {
      const itemElement = createReviewItemElement(item, index);
      container.appendChild(itemElement);
    });
  }
  
  // 创建审核项元素
  function createReviewItemElement(item, index) {
    const itemDiv = DomUtils.createElement('div', { className: 'review-item' });
    
    const header = DomUtils.createElement('div', { className: 'review-item-header' });
    
    const indexSpan = DomUtils.createElement('span', { className: 'review-item-index' });
    DomUtils.setTextContent(indexSpan, `#${index + 1}`);
    header.appendChild(indexSpan);
    
    const typeSpan = DomUtils.createElement('span', { className: 'review-item-type' });
    DomUtils.setTextContent(typeSpan, getModificationTypeText(item.modificationType));
    header.appendChild(typeSpan);
    
    const confidenceSpan = DomUtils.createElement('span', { className: 'review-item-confidence' });
    DomUtils.setTextContent(confidenceSpan, `置信度: ${(item.confidence * 100).toFixed(1)}%`);
    header.appendChild(confidenceSpan);
    
    itemDiv.appendChild(header);
    
    const contentDiv = DomUtils.createElement('div', { className: 'review-item-content' });
    
    const originalDiv = DomUtils.createElement('div', { className: 'review-original' });
    const originalLabel = DomUtils.createElement('strong');
    DomUtils.setTextContent(originalLabel, '原文: ');
    originalDiv.appendChild(originalLabel);
    const originalText = DomUtils.createElement('span');
    DomUtils.setTextContent(originalText, item.originalText || '');
    originalDiv.appendChild(originalText);
    contentDiv.appendChild(originalDiv);
    
    const suggestedDiv = DomUtils.createElement('div', { className: 'review-suggested' });
    const suggestedLabel = DomUtils.createElement('strong');
    DomUtils.setTextContent(suggestedLabel, '建议: ');
    suggestedDiv.appendChild(suggestedLabel);
    const suggestedText = DomUtils.createElement('span');
    DomUtils.setTextContent(suggestedText, item.suggestedText || '');
    suggestedDiv.appendChild(suggestedText);
    contentDiv.appendChild(suggestedDiv);
    
    if (item.editedText) {
      const editedDiv = DomUtils.createElement('div', { className: 'review-edited' });
      const editedLabel = DomUtils.createElement('strong');
      DomUtils.setTextContent(editedLabel, '编辑后: ');
      editedDiv.appendChild(editedLabel);
      const editedText = DomUtils.createElement('span');
      DomUtils.setTextContent(editedText, item.editedText);
      editedDiv.appendChild(editedText);
      contentDiv.appendChild(editedDiv);
    }
    
    itemDiv.appendChild(contentDiv);
    
    const actionsDiv = DomUtils.createElement('div', { className: 'review-item-actions' });
    
    if (item.status === 'pending') {
      const approveBtn = DomUtils.createElement('button', {
        className: 'btn-approve',
        onclick: () => approveReviewItem(item.id)
      });
      DomUtils.setTextContent(approveBtn, '通过');
      actionsDiv.appendChild(approveBtn);
      
      const rejectBtn = DomUtils.createElement('button', {
        className: 'btn-reject',
        onclick: () => rejectReviewItem(item.id)
      });
      DomUtils.setTextContent(rejectBtn, '拒绝');
      actionsDiv.appendChild(rejectBtn);
      
      const editBtn = DomUtils.createElement('button', {
        className: 'btn-edit',
        onclick: () => editReviewItem(item.id)
      });
      DomUtils.setTextContent(editBtn, '编辑');
      actionsDiv.appendChild(editBtn);
    } else if (item.status === 'edited') {
      const restoreBtn = DomUtils.createElement('button', {
        className: 'btn-restore',
        onclick: () => restoreReviewItem(item.id)
      });
      DomUtils.setTextContent(restoreBtn, '恢复原文');
      actionsDiv.appendChild(restoreBtn);
    }
    
    const statusSpan = DomUtils.createElement('span', { className: `review-status review-status-${item.status}` });
    DomUtils.setTextContent(statusSpan, getReviewStatusText(item.status));
    actionsDiv.appendChild(statusSpan);
    
    itemDiv.appendChild(actionsDiv);
    
    return itemDiv;
  }
  
  // 获取修改类型文本
  function getModificationTypeText(type) {
    const map = {
      typo: '错别字',
      duplicate_paragraph: '重复段落',
      advertisement: '广告',
      grammar: '语法错误',
      style: '风格问题'
    };
    return map[type] || type;
  }
  
  // 获取审核状态文本
  function getReviewStatusText(status) {
    const map = {
      pending: '待审核',
      approved: '已通过',
      rejected: '已拒绝',
      edited: '已编辑'
    };
    return map[status] || status;
  }
  
  // 通过审核项
  function approveReviewItem(itemId) {
    if (!currentFileMd5) return;
    
    AppConfig.apiRequest(`/files/${currentFileMd5}/approve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已通过修改建议', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('操作失败: ' + err.message, 'error');
      });
  }
  
  // 拒绝审核项
  function rejectReviewItem(itemId) {
    if (!currentFileMd5) return;
    
    AppConfig.apiRequest(`/files/${currentFileMd5}/reject`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已拒绝修改建议', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('操作失败: ' + err.message, 'error');
      });
  }
  
  // 编辑审核项
  function editReviewItem(itemId) {
    const item = reviewItems.find(i => i.id === itemId);
    if (!item) return;
    
    const editedText = prompt('请输入编辑后的文本:', item.suggestedText || item.originalText);
    if (editedText === null) return;
    
    AppConfig.apiRequest(`/files/${currentFileMd5}/edit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId, editedText })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已保存编辑', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('编辑失败: ' + err.message, 'error');
      });
  }
  
  // 恢复审核项原文
  function restoreReviewItem(itemId) {
    AppConfig.apiRequest(`/files/${currentFileMd5}/restore`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('已恢复原文', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('恢复失败: ' + err.message, 'error');
      });
  }
  
  // 批量通过所有
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
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('批量操作失败: ' + err.message, 'error');
      });
  }
  
  // 批量拒绝所有
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
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('批量操作失败: ' + err.message, 'error');
      });
  }
  
  // 更新审核进度
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
  
  // 完成文件处理
  function finalizeFile() {
    if (!currentFileMd5 || !confirm('确定完成审核并生成最终文件？')) return;
    
    AppConfig.apiRequest(`/files/${currentFileMd5}/finalize`, {
      method: 'POST'
    })
      .then((data) => {
        if (data.success) {
          showFeedback('文件处理完成', 'success');
          FileManager.showSection('completed');
          ProcessingModule.updateCompletedInfo();
        } else {
          showFeedback(data.message, 'error');
        }
      })
      .catch((err) => {
        showFeedback('完成处理失败: ' + err.message, 'error');
      });
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
    loadReviewItems,
    renderReviewItems,
    approveReviewItem,
    rejectReviewItem,
    editReviewItem,
    restoreReviewItem,
    batchApproveAll,
    batchRejectAll,
    finalizeFile,
    setCurrentFileMd5: (md5) => { currentFileMd5 = md5; }
  };
})();

// 导出到全局作用域
window.ReviewModule = ReviewModule;