// 审核模块 — 段级审核：每段一行，左原文右建议
const ReviewModule = (function() {
  let paragraphs = [];
  let baselineParagraphs = [];
  let currentFileMd5 = null;
  let selectedIndex = -1;
  let currentPage = 1;
  let pageSize = 50;
  let filtered = [];
  let statusFilter = 'pending';

  function init() {
    bindEvents();
    initFontScale();
    initPageSize();
  }

  // ----- 字体缩放 -----
  const FONT_SIZE_MIN = 12;
  const FONT_SIZE_MAX = 20;
  const FONT_SIZE_DEFAULT = 13;
  const FONT_SIZE_STORAGE_KEY = 'voidtext-review-font-size';

  function initFontScale() {
    const saved = localStorage.getItem(FONT_SIZE_STORAGE_KEY);
    let size = saved ? parseInt(saved, 10) : FONT_SIZE_DEFAULT;
    size = Math.max(FONT_SIZE_MIN, Math.min(FONT_SIZE_MAX, size));
    applyFontSize(size);
    const upBtn = document.getElementById('font-scale-up');
    const downBtn = document.getElementById('font-scale-down');
    if (upBtn) upBtn.addEventListener('click', () => adjustFontSize(1));
    if (downBtn) downBtn.addEventListener('click', () => adjustFontSize(-1));
  }

  function adjustFontSize(delta) {
    const current = parseInt(
      getComputedStyle(document.documentElement).getPropertyValue('--review-font-size')
    ) || FONT_SIZE_DEFAULT;
    const next = Math.max(FONT_SIZE_MIN, Math.min(FONT_SIZE_MAX, current + delta));
    applyFontSize(next);
    localStorage.setItem(FONT_SIZE_STORAGE_KEY, String(next));
  }

  function applyFontSize(size) {
    document.documentElement.style.setProperty('--review-font-size', size + 'px');
    const display = document.getElementById('font-scale-value');
    if (display) display.textContent = size + 'px';
  }

  // ----- 分页 -----
  const PAGE_SIZE_STORAGE_KEY = 'voidtext-review-page-size';

  function initPageSize() {
    const saved = localStorage.getItem(PAGE_SIZE_STORAGE_KEY);
    if (saved) {
      pageSize = Math.max(25, Math.min(100, parseInt(saved, 10) || 50));
    }
    const select = document.getElementById('page-size-select');
    if (select) {
      select.value = String(pageSize);
      select.addEventListener('change', function() {
        pageSize = parseInt(this.value, 10) || 50;
        localStorage.setItem(PAGE_SIZE_STORAGE_KEY, String(pageSize));
        currentPage = 1;
        render();
      });
    }
  }

  // ----- 事件 -----
  function bindEvents() {
    const filterEl = document.getElementById('review-status-filter');
    if (filterEl) {
      filterEl.value = statusFilter;
      filterEl.addEventListener('change', function() {
        statusFilter = this.value;
        currentPage = 1;
        loadReviewItems();
      });
    }

    const batchApproveBtn = document.getElementById('batch-approve-all-btn');
    if (batchApproveBtn) batchApproveBtn.addEventListener('click', batchApproveAll);
    const batchRejectBtn = document.getElementById('batch-reject-all-btn');
    if (batchRejectBtn) batchRejectBtn.addEventListener('click', batchRejectAll);

    const finalizeBtn = document.getElementById('finalize-btn');
    if (finalizeBtn) finalizeBtn.addEventListener('click', finalizeFile);

    const pagePrev = document.getElementById('page-prev');
    const pageNext = document.getElementById('page-next');
    if (pagePrev) pagePrev.addEventListener('click', function() {
      if (currentPage > 1) { currentPage--; render(); }
    });
    if (pageNext) pageNext.addEventListener('click', function() {
      const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
      if (currentPage < totalPages) { currentPage++; render(); }
    });

    document.addEventListener('keydown', handleKeydown);
  }

  function handleKeydown(e) {
    const section = document.getElementById('review-section');
    if (!section || section.style.display === 'none') return;
    if (e.target.matches('input, textarea, select')) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      moveSelection(1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      moveSelection(-1);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (selectedIndex >= 0 && filtered[selectedIndex]) approveParagraph(filtered[selectedIndex].id);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      if (selectedIndex >= 0 && filtered[selectedIndex]) rejectParagraph(filtered[selectedIndex].id);
    } else if (e.key === 'PageDown') {
      e.preventDefault();
      const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
      if (currentPage < totalPages) { currentPage++; render(); }
    } else if (e.key === 'PageUp') {
      e.preventDefault();
      if (currentPage > 1) { currentPage--; render(); }
    }
  }

  function moveSelection(delta) {
    const start = (currentPage - 1) * pageSize;
    const end = Math.min(start + pageSize, filtered.length);
    if (selectedIndex < start || selectedIndex >= end) {
      selectedIndex = start;
    } else {
      selectedIndex = Math.max(start, Math.min(end - 1, selectedIndex + delta));
    }
    renderSelection();
    const row = document.querySelector('.review-row[data-index="' + selectedIndex + '"]');
    if (row) row.scrollIntoView({ block: 'nearest' });
  }

  // ----- 数据加载 -----
  function loadReviewItems() {
    if (!currentFileMd5) return;
    showLoading(true);
    AppConfig.apiRequest('/files/' + currentFileMd5 + '/review-items?status=' + statusFilter)
      .then(function(data) {
        if (!data.success) {
          showFeedback(data.message || '加载审核项失败', 'error');
          return;
        }
        paragraphs = data.paragraphs || [];
        baselineParagraphs = (data.baselineContent || '').split('\n');
        applyFilter();
        render();
        updateProgress();
      })
      .catch(function(err) {
        showFeedback('加载失败: ' + err.message, 'error');
      })
      .finally(function() {
        showLoading(false);
      });
  }

  function applyFilter() {
    // 后端已经按 status 过滤，前端直接用
    filtered = paragraphs.slice();
    filtered.sort(function(a, b) { return a.paragraphIndex - b.paragraphIndex; });
    if (selectedIndex >= filtered.length) selectedIndex = filtered.length - 1;
    if (selectedIndex < 0 && filtered.length > 0) selectedIndex = 0;
  }

  // ----- 渲染 -----
  function render() {
    const container = document.getElementById('review-page-container');
    if (!container) return;

    const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
    if (currentPage > totalPages) currentPage = totalPages;

    const start = (currentPage - 1) * pageSize;
    const end = Math.min(start + pageSize, filtered.length);
    const pageItems = filtered.slice(start, end);

    if (pageItems.length === 0) {
      container.innerHTML = '<div class="review-empty">' +
        (filtered.length === 0 ? '没有匹配状态的段落' : '本页没有内容') +
        '</div>';
    } else {
      const html = pageItems.map(function(p, i) {
        return renderRow(p, start + i);
      }).join('');
      container.innerHTML = html;
    }

    document.getElementById('current-page').textContent = String(currentPage);
    document.getElementById('total-pages').textContent = String(totalPages);

    bindRowEvents();
    renderSelection();
  }

  function renderRow(p, index) {
    const original = escapeHtml(p.original || '');
    const suggested = escapeHtml(p.suggested || '');
    const isDuplicate = p.type === 'duplicate_paragraph';
    const status = p.status || 'pending';
    const editedText = p.editedText || '';

    const leftHtml = isDuplicate
      ? '<div class="review-cell review-cell-original is-duplicate">' + original + '</div>'
      : '<div class="review-cell review-cell-original">' + (window.DiffUtils ? DiffUtils.diffOriginal(p.original || '', p.suggested || '') : original) + '</div>';

    const rightHtml = isDuplicate
      ? '<div class="review-cell review-cell-suggested is-duplicate">（删除整段）</div>'
      : '<div class="review-cell review-cell-suggested">' + (window.DiffUtils ? DiffUtils.diffSuggested(p.original || '', p.suggested || '') : suggested) + '</div>';

    const editedHtml = (status === 'edited' && editedText)
      ? '<div class="review-cell-edited">编辑后: ' + escapeHtml(editedText) + '</div>'
      : '';

    return '' +
      '<div class="review-row review-row-' + status + '" data-index="' + index + '" data-id="' + p.id + '">' +
        '<div class="review-row-meta">#' + (p.paragraphIndex + 1) + '</div>' +
        '<div class="review-row-body">' +
          leftHtml + rightHtml + editedHtml +
        '</div>' +
        '<div class="review-row-actions">' +
          (status === 'pending'
            ? '<button class="btn-approve" data-action="approve" title="保留 (Enter)">✓</button>' +
              '<button class="btn-reject" data-action="reject" title="撤销 (Esc)">✗</button>' +
              (isDuplicate ? '' : '<button class="btn-edit" data-action="edit" title="编辑">✎</button>')
            : '<span class="status-tag status-' + status + '">' + statusLabel(status) + '</span>' +
              '<button class="btn-restore" data-action="restore" title="恢复待审核">↺</button>') +
        '</div>' +
      '</div>';
  }

  function statusLabel(s) {
    return s === 'approved' ? '已保留' :
           s === 'rejected' ? '已撤销' :
           s === 'edited' ? '已编辑' : '待审核';
  }

  function renderSelection() {
    document.querySelectorAll('.review-row').forEach(function(el) {
      el.classList.toggle('selected', parseInt(el.getAttribute('data-index'), 10) === selectedIndex);
    });
  }

  function bindRowEvents() {
    document.querySelectorAll('.review-row').forEach(function(row) {
      row.addEventListener('click', function(e) {
        const idx = parseInt(row.getAttribute('data-index'), 10);
        selectedIndex = idx;
        renderSelection();

        const btn = e.target.closest('button[data-action]');
        if (!btn) return;
        const id = parseInt(row.getAttribute('data-id'), 10);
        const action = btn.getAttribute('data-action');
        if (action === 'approve') approveParagraph(id);
        else if (action === 'reject') rejectParagraph(id);
        else if (action === 'restore') restoreParagraph(id);
        else if (action === 'edit') editParagraph(id);
      });
    });
  }

  // ----- API 操作 -----
  function reviewAction(endpoint, body, msg) {
    return AppConfig.apiRequest('/files/' + currentFileMd5 + endpoint, {
      method: 'POST',
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : null
    }).then(function(data) {
      if (data.success) {
        if (msg) showFeedback(msg, 'success');
        loadReviewItems();
      } else {
        showFeedback(data.message || '操作失败', 'error');
      }
      return data;
    }).catch(function(err) {
      showFeedback('操作失败: ' + err.message, 'error');
    });
  }

  function approveParagraph(id) {
    return reviewAction('/approve', { itemId: id }, '✓ 已保留');
  }
  function rejectParagraph(id) {
    return reviewAction('/reject', { itemId: id }, '✗ 已撤销');
  }
  function restoreParagraph(id) {
    return reviewAction('/restore', { itemId: id }, '↺ 已恢复待审核');
  }
  function editParagraph(id) {
    const p = paragraphs.find(function(x) { return x.id === id; });
    if (!p) return;
    const def = p.suggested || p.original || '';
    const text = prompt('请输入编辑后的段落:', def);
    if (text === null) return;
    return reviewAction('/edit', { itemId: id, editedText: text }, '已保存编辑');
  }

  function batchApproveAll() {
    if (!confirm('确定要保留全部待审段落吗？')) return;
    return reviewAction('/batch-approve', null, '已批量保留');
  }
  function batchRejectAll() {
    if (!confirm('确定要撤销全部待审段落吗？')) return;
    return reviewAction('/batch-reject', null, '已批量撤销');
  }

  function finalizeFile() {
    if (!confirm('确认完成审核并生成最终文件？')) return;
    showLoading(true);
    AppConfig.apiRequest('/files/' + currentFileMd5 + '/finalize', { method: 'POST' })
      .then(function(data) {
        if (data.success) {
          showFeedback('已生成最终文件', 'success');
          showSection('completed');
          if (window.FileManager && FileManager.refreshAfterFinalize) {
            FileManager.refreshAfterFinalize(currentFileMd5);
          }
        } else {
          showFeedback(data.message || '生成失败', 'error');
        }
      })
      .catch(function(err) {
        showFeedback('生成失败: ' + err.message, 'error');
      })
      .finally(function() {
        showLoading(false);
      });
  }

  // ----- 进度与按钮可见性 -----
  function updateProgress() {
    const total = paragraphs.length;
    const resolved = paragraphs.filter(function(p) { return p.status !== 'pending'; }).length;
    const text = document.getElementById('review-progress-text');
    if (text) text.textContent = resolved + '/' + total;
    const cat = document.getElementById('review-current-category');
    if (cat) cat.textContent = filterLabel(statusFilter) + ' (' + filtered.length + ')';

    // finalize 按钮：只有当所有段都已 resolve 时才显示
    const finalizeBtn = document.getElementById('finalize-btn');
    if (finalizeBtn) {
      // 需要查全量（pending=0），用 statusFilter 过滤的列表无法判断；额外发请求
      AppConfig.apiRequest('/files/' + currentFileMd5 + '/status')
        .then(function(s) {
          const pending = (s.reviewTotal || 0) - (s.reviewResolved || 0);
          finalizeBtn.style.display = (s.reviewTotal > 0 && pending === 0) ? '' : 'none';
        })
        .catch(function() {});
    }
  }

  function filterLabel(s) {
    return s === 'pending' ? '待审核' :
           s === 'approved' ? '已保留' :
           s === 'rejected' ? '已撤销' :
           s === 'edited' ? '已编辑' : '全部';
  }

  // ----- 工具 -----
  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function showLoading(show) {
    const overlay = document.getElementById('review-loading-overlay');
    if (overlay) overlay.style.display = show ? '' : 'none';
  }

  function showFeedback(msg, type) {
    if (window.AppConfig && AppConfig.showFeedback) {
      AppConfig.showFeedback(msg, type);
    } else {
      console.log('[' + type + '] ' + msg);
    }
  }

  function showSection(name) {
    if (typeof window.showSection === 'function') window.showSection(name);
  }

  return {
    init: init,
    loadReviewItems: loadReviewItems,
    approveReviewItem: approveParagraph,
    rejectReviewItem: rejectParagraph,
    editReviewItem: editParagraph,
    restoreReviewItem: restoreParagraph,
    batchApproveAll: batchApproveAll,
    batchRejectAll: batchRejectAll,
    finalizeFile: finalizeFile,
    setCurrentFileMd5: function(md5) { currentFileMd5 = md5; },
    getCurrentFileMd5: function() { return currentFileMd5; }
  };
})();

window.ReviewModule = ReviewModule;
