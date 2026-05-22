// 审核模块 — 全屏沉浸式 Diff 视图 + 分页 + 快捷键
const ReviewModule = (function() {
  // 私有状态
  let reviewItems = [];
  let currentFileMd5 = null;
  let selectedItemIndex = -1;
  let currentPage = 1;
  let pageSize = 50;
  let filteredItems = [];

  // 全文内容
  let fullContent = '';
  let fullLines = [];
  let lineGroups = {}; // 按行号分组的审核项

  // 批量审核组
  let batchGroups = {};
  let currentBatchGroup = null;

  // ---------------------------------------------------------------
  // 初始化
  // ---------------------------------------------------------------
  function init() {
    bindEvents();
    initFontScale();
    initPageSize();
  }

  // ---------------------------------------------------------------
  // 字体缩放
  // ---------------------------------------------------------------
  var FONT_SIZE_MIN = 12;
  var FONT_SIZE_MAX = 20;
  var FONT_SIZE_DEFAULT = 13;
  var FONT_SIZE_STEP = 1;
  var FONT_SIZE_STORAGE_KEY = 'voidtext-review-font-size';

  function initFontScale() {
    var saved = localStorage.getItem(FONT_SIZE_STORAGE_KEY);
    var currentSize = saved ? parseInt(saved, 10) : FONT_SIZE_DEFAULT;
    currentSize = Math.max(FONT_SIZE_MIN, Math.min(FONT_SIZE_MAX, currentSize));
    applyFontSize(currentSize);

    var upBtn = document.getElementById('font-scale-up');
    var downBtn = document.getElementById('font-scale-down');
    if (upBtn) upBtn.addEventListener('click', function() { adjustFontSize(FONT_SIZE_STEP); });
    if (downBtn) downBtn.addEventListener('click', function() { adjustFontSize(-FONT_SIZE_STEP); });
  }

  function adjustFontSize(delta) {
    var current = parseInt(
      getComputedStyle(document.documentElement).getPropertyValue('--review-font-size')
    ) || FONT_SIZE_DEFAULT;
    var next = Math.max(FONT_SIZE_MIN, Math.min(FONT_SIZE_MAX, current + delta));
    applyFontSize(next);
    localStorage.setItem(FONT_SIZE_STORAGE_KEY, String(next));
  }

  function applyFontSize(size) {
    document.documentElement.style.setProperty('--review-font-size', size + 'px');
    var display = document.getElementById('font-scale-value');
    if (display) display.textContent = size + 'px';
  }

  // ---------------------------------------------------------------
  // 分页设置
  // ---------------------------------------------------------------
  var PAGE_SIZE_STORAGE_KEY = 'voidtext-review-page-size';

  function initPageSize() {
    var saved = localStorage.getItem(PAGE_SIZE_STORAGE_KEY);
    if (saved) {
      pageSize = Math.max(25, Math.min(100, parseInt(saved, 10) || 50));
    }
    var select = document.getElementById('page-size-select');
    if (select) {
      select.value = String(pageSize);
      select.addEventListener('change', function() {
        pageSize = parseInt(this.value, 10) || 50;
        localStorage.setItem(PAGE_SIZE_STORAGE_KEY, String(pageSize));
        currentPage = 1;
        renderCurrentPage();
      });
    }
  }

  // ---------------------------------------------------------------
  // 事件绑定
  // ---------------------------------------------------------------
  function bindEvents() {
    // 状态筛选
    var statusFilter = document.getElementById('review-status-filter');
    if (statusFilter) {
      statusFilter.addEventListener('change', function() {
        currentPage = 1;
        loadReviewItems();
      });
    }

    // 全局批量通过
    var batchApproveBtn = document.getElementById('batch-approve-all-btn');
    if (batchApproveBtn) {
      batchApproveBtn.addEventListener('click', batchApproveAll);
    }

    // 全局批量拒绝
    var batchRejectBtn = document.getElementById('batch-reject-all-btn');
    if (batchRejectBtn) {
      batchRejectBtn.addEventListener('click', batchRejectAll);
    }

    // 完成审核
    var finalizeBtn = document.getElementById('finalize-btn');
    if (finalizeBtn) {
      finalizeBtn.addEventListener('click', finalizeFile);
    }

    // 翻页按钮
    var prevBtn = document.getElementById('page-prev');
    var nextBtn = document.getElementById('page-next');
    if (prevBtn) prevBtn.addEventListener('click', function() { goToPage(currentPage - 1); });
    if (nextBtn) nextBtn.addEventListener('click', function() { goToPage(currentPage + 1); });

    // 批量审核返回按钮
    var batchBackBtn = document.getElementById('batch-review-back');
    if (batchBackBtn) {
      batchBackBtn.addEventListener('click', function() {
        showMainReview();
      });
    }

    // 批量审核全部同意
    var batchApproveAllBtn = document.getElementById('batch-approve-all-selected');
    if (batchApproveAllBtn) {
      batchApproveAllBtn.addEventListener('click', function() {
        if (currentBatchGroup) {
          batchApproveGroup(currentBatchGroup.items.map(function(i) { return i.id; }));
          showMainReview();
        }
      });
    }

    // 全局键盘快捷键
    document.addEventListener('keydown', handleKeyboardShortcut);
  }

  // ---------------------------------------------------------------
  // 数据加载
  // ---------------------------------------------------------------
  function loadReviewItems() {
    if (!currentFileMd5) return;

    // 并行加载审核项和全文
    Promise.all([
      AppConfig.apiRequest('/files/' + currentFileMd5 + '/review-items'),
      AppConfig.apiRequest('/files/' + currentFileMd5 + '/content')
    ])
      .then(function(results) {
        var reviewData = results[0];
        var contentData = results[1];

        if (reviewData.success) {
          reviewItems = reviewData.suggestions || reviewData.items || [];
        }

        if (contentData.success) {
          fullContent = contentData.content || '';
          fullLines = fullContent.split('\n');
        }

        filterAndSortItems();
        groupItemsByLine();
        findBatchGroups();
        renderCurrentPage();
        updateReviewProgress();
      })
      .catch(function(err) {
        console.error('加载数据失败:', err);
      });
  }

  // ---------------------------------------------------------------
  // 过滤和排序
  // ---------------------------------------------------------------
  function filterAndSortItems() {
    var statusFilter = getFilterValue();

    filteredItems = reviewItems;
    if (statusFilter === 'pending') {
      filteredItems = reviewItems.filter(function(item) { return item.status === 'pending'; });
    } else if (statusFilter !== 'all') {
      filteredItems = reviewItems.filter(function(item) { return item.status === statusFilter; });
    }

    // 按行号排序，同一行内按 position 排序
    filteredItems.sort(function(a, b) {
      var lineA = a.lineNum || 0;
      var lineB = b.lineNum || 0;
      if (lineA !== lineB) return lineA - lineB;
      return (a.position || 0) - (b.position || 0);
    });
  }

  function getFilterValue() {
    var el = document.getElementById('review-status-filter');
    return el ? el.value : 'pending';
  }

  // ---------------------------------------------------------------
  // 按行号分组审核项
  // ---------------------------------------------------------------
  function groupItemsByLine() {
    lineGroups = {};
    filteredItems.forEach(function(item) {
      var lineNum = item.lineNum || 0;
      if (!lineGroups[lineNum]) {
        lineGroups[lineNum] = [];
      }
      lineGroups[lineNum].push(item);
    });
  }

  // ---------------------------------------------------------------
  // 批量审核组检测
  // ---------------------------------------------------------------
  function findBatchGroups() {
    batchGroups = {};
    var pendingItems = reviewItems.filter(function(item) { return item.status === 'pending'; });

    pendingItems.forEach(function(item) {
      var orig = item.original || item.originalText || '';
      var sugg = item.suggested || item.suggestedText || '';
      var key = orig + '|||' + sugg;
      if (!batchGroups[key]) {
        batchGroups[key] = {
          original: orig,
          suggested: sugg,
          items: []
        };
      }
      batchGroups[key].items.push(item);
    });

    // 只保留有多个项的组
    var multiItemGroups = {};
    Object.keys(batchGroups).forEach(function(key) {
      if (batchGroups[key].items.length > 1) {
        multiItemGroups[key] = batchGroups[key];
      }
    });
    batchGroups = multiItemGroups;
  }

  // ---------------------------------------------------------------
  // 分页渲染
  // ---------------------------------------------------------------
  function goToPage(page) {
    var totalPages = Math.ceil(fullLines.length / pageSize) || 1;
    if (page < 1) page = 1;
    if (page > totalPages) page = totalPages;
    currentPage = page;
    selectedItemIndex = -1;
    renderCurrentPage();
  }

  function renderCurrentPage() {
    var container = document.getElementById('review-page-container');
    if (!container) return;

    container.innerHTML = '';

    var totalPages = Math.ceil(fullLines.length / pageSize) || 1;
    if (currentPage > totalPages) currentPage = totalPages;

    // 更新页码显示
    var currentPageEl = document.getElementById('current-page');
    var totalPagesEl = document.getElementById('total-pages');
    if (currentPageEl) currentPageEl.textContent = String(currentPage);
    if (totalPagesEl) totalPagesEl.textContent = String(totalPages);

    // 更新翻页按钮状态
    var prevBtn = document.getElementById('page-prev');
    var nextBtn = document.getElementById('page-next');
    if (prevBtn) prevBtn.disabled = currentPage <= 1;
    if (nextBtn) nextBtn.disabled = currentPage >= totalPages;

    // 计算当前页的行范围
    var startLine = (currentPage - 1) * pageSize;
    var endLine = Math.min(startLine + pageSize, fullLines.length);

    // 渲染每一行，用 DocumentFragment 批量插入
    var fragment = document.createDocumentFragment();
    var firstItemIndex = -1;

    for (var i = startLine; i < endLine; i++) {
      var lineNum = i + 1;
      var lineText = fullLines[i];
      var items = lineGroups[lineNum] || [];

      if (firstItemIndex < 0 && items.length > 0) firstItemIndex = i;

      var lineEls = createReviewLines(lineNum, lineText, items);
      lineEls.forEach(function(el) { fragment.appendChild(el); });
    }
    container.appendChild(fragment);

    // 选中第一个有审核项的行
    if (selectedItemIndex < 0 && firstItemIndex >= 0) {
      selectedItemIndex = firstItemIndex;
    }
    highlightSelectedItem();
  }

  // ---------------------------------------------------------------
  // 创建审核行（原文行 + 建议行）
  // 使用 renderAlignedDiff 实现逐字符对齐
  // ---------------------------------------------------------------
  function createReviewLines(lineNum, lineText, items) {
    var result = [];
    var hasReviewItems = items.length > 0;
    var hasPendingItems = items.some(function(item) { return item.status === 'pending'; });

    if (!hasReviewItems) {
      // 普通行
      var line = DomUtils.createElement('div', {
        className: 'review-line line-normal',
        'data-line-num': String(lineNum)
      });

      var gutter = DomUtils.createElement('div', { className: 'review-line-gutter' });
      DomUtils.setTextContent(gutter, String(lineNum));
      line.appendChild(gutter);

      var content = DomUtils.createElement('div', { className: 'review-line-content' });
      DomUtils.setTextContent(content, lineText);
      line.appendChild(content);

      result.push(line);
    } else {
      // 有审核项的行 - 使用 renderAlignedDiff 实现逐字符对齐
      // 按 position 排序
      var sortedItems = items.slice().sort(function(a, b) {
        return (a.position || 0) - (b.position || 0);
      });

      // 为每个审核项生成对齐的 HTML
      var origHtmlParts = [];
      var suggHtmlMap = {}; // itemId -> suggestedHtml

      sortedItems.forEach(function(item) {
        var sugg = item.suggested || item.suggestedText || '';
        var orig = item.original || item.originalText || '';

        if (sugg && sugg !== orig) {
          // 使用 renderAlignedDiff 生成对齐的 HTML
          var aligned = DiffUtils.renderAlignedDiff(lineText, item);
          origHtmlParts.push(aligned.originalHtml);
          suggHtmlMap[item.id] = aligned.suggestedHtml;
        } else {
          // 没有修改的项，直接使用原文
          origHtmlParts.push(DiffUtils.renderAlignedDiff(lineText, {
            original: orig,
            suggested: orig,
            position: item.position || 0
          }).originalHtml);
        }
      });

      // 渲染原文行
      var origLine = DomUtils.createElement('div', {
        className: 'review-line line-original' + (selectedItemIndex === lineNum - 1 ? ' active' : ''),
        'data-line-num': String(lineNum),
        'data-type': 'original'
      });

      var gutter = DomUtils.createElement('div', { className: 'review-line-gutter' });
      DomUtils.setTextContent(gutter, String(lineNum));
      origLine.appendChild(gutter);

      var content = DomUtils.createElement('div', { className: 'review-line-content' });
      // 使用第一个审核项的对齐结果（所有审核项共用同一行）
      content.innerHTML = origHtmlParts.length > 0 ? origHtmlParts[0] : lineText;
      origLine.appendChild(content);

      // 浮动操作框
      if (hasPendingItems) {
        var actions = createFloatingActions(lineNum, items);
        origLine.appendChild(actions);
      }

      // 批量审核按钮
      var batchGroup = getBatchGroupForLine(items);
      if (batchGroup) {
        var batchBtn = DomUtils.createElement('button', {
          className: 'batch-review-btn',
          onclick: function(e) {
            e.stopPropagation();
            showBatchReview(batchGroup);
          }
        });
        DomUtils.setTextContent(batchBtn, '批量 (' + batchGroup.items.length + ')');
        origLine.appendChild(batchBtn);
      }

      // 点击选中
      origLine.addEventListener('click', function(e) {
        if (e.target.closest('.floating-actions') || e.target.closest('.batch-review-btn')) {
          return;
        }
        selectedItemIndex = lineNum - 1;
        highlightSelectedItem();
      });

      result.push(origLine);

      // 渲染建议行（每个审核项一行，使用对齐的 HTML）
      sortedItems.forEach(function(item) {
        var suggHtml = suggHtmlMap[item.id];
        if (suggHtml) {
          var suggLine = DomUtils.createElement('div', {
            className: 'review-line line-suggested',
            'data-line-num': String(lineNum),
            'data-type': 'suggested',
            'data-item-id': String(item.id)
          });

          var suggGutter = DomUtils.createElement('div', { className: 'review-line-gutter' });
          DomUtils.setTextContent(suggGutter, '→');
          suggLine.appendChild(suggGutter);

          var suggContent = DomUtils.createElement('div', { className: 'review-line-content' });
          suggContent.innerHTML = suggHtml;
          suggLine.appendChild(suggContent);

          result.push(suggLine);
        }
      });
    }

    return result;
  }

  // ---------------------------------------------------------------
  // 创建浮动操作框
  // ---------------------------------------------------------------
  function createFloatingActions(lineNum, items) {
    var actions = DomUtils.createElement('div', { className: 'floating-actions' });

    var pendingItems = items.filter(function(item) { return item.status === 'pending'; });
    if (pendingItems.length === 0) return actions;

    // 单个审核项时显示保留/撤销
    if (pendingItems.length === 1) {
      var item = pendingItems[0];

      var approveBtn = DomUtils.createElement('button', {
        className: 'floating-btn btn-approve',
        title: '保留 (Enter)',
        onclick: function(e) {
          e.stopPropagation();
          approveReviewItem(item.id);
        }
      });
      DomUtils.setTextContent(approveBtn, '保留');
      actions.appendChild(approveBtn);

      var rejectBtn = DomUtils.createElement('button', {
        className: 'floating-btn btn-reject',
        title: '撤销 (Esc)',
        onclick: function(e) {
          e.stopPropagation();
          rejectReviewItem(item.id);
        }
      });
      DomUtils.setTextContent(rejectBtn, '撤销');
      actions.appendChild(rejectBtn);
    } else {
      // 多个审核项时显示全部保留/全部撤销
      var approveAllBtn = DomUtils.createElement('button', {
        className: 'floating-btn btn-approve',
        title: '全部保留 (Enter)',
        onclick: function(e) {
          e.stopPropagation();
          var ids = pendingItems.map(function(i) { return i.id; });
          batchApproveGroup(ids);
        }
      });
      DomUtils.setTextContent(approveAllBtn, '全部保留');
      actions.appendChild(approveAllBtn);

      var rejectAllBtn = DomUtils.createElement('button', {
        className: 'floating-btn btn-reject',
        title: '全部撤销 (Esc)',
        onclick: function(e) {
          e.stopPropagation();
          var ids = pendingItems.map(function(i) { return i.id; });
          batchRejectGroup(ids);
        }
      });
      DomUtils.setTextContent(rejectAllBtn, '全部撤销');
      actions.appendChild(rejectAllBtn);
    }

    return actions;
  }

  function getBatchGroupForLine(items) {
    for (var i = 0; i < items.length; i++) {
      var item = items[i];
      if (item.status !== 'pending') continue;
      var orig = item.original || item.originalText || '';
      var sugg = item.suggested || item.suggestedText || '';
      var key = orig + '|||' + sugg;
      if (batchGroups[key]) {
        return batchGroups[key];
      }
    }
    return null;
  }

  // ---------------------------------------------------------------
  // 选中高亮
  // ---------------------------------------------------------------
  var _activeLineEl = null;

  function highlightSelectedItem() {
    var container = document.getElementById('review-page-container');
    if (!container) return;

    if (_activeLineEl) {
      _activeLineEl.classList.remove('active');
      _activeLineEl = null;
    }

    if (selectedItemIndex >= 0) {
      var selectedLine = container.querySelector('[data-line-num="' + (selectedItemIndex + 1) + '"][data-type="original"]');
      if (selectedLine) {
        selectedLine.classList.add('active');
        selectedLine.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        _activeLineEl = selectedLine;
      }
    }
  }

  // ---------------------------------------------------------------
  // 键盘快捷键
  // ---------------------------------------------------------------
  function handleKeyboardShortcut(event) {
    var reviewSection = document.getElementById('review-section');
    var batchSection = document.getElementById('batch-review-section');
    var isReviewVisible = reviewSection && reviewSection.style.display !== 'none';
    var isBatchVisible = batchSection && batchSection.style.display !== 'none';

    if (!isReviewVisible && !isBatchVisible) return;

    // 忽略输入框中的快捷键
    if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA' ||
        event.target.tagName === 'SELECT') return;

    var totalPages = Math.ceil(fullLines.length / pageSize) || 1;

    switch (event.key) {
      case 'ArrowUp':
      case 'p':
      case 'P':
        event.preventDefault();
        navigateToPrevItem();
        break;

      case 'ArrowDown':
      case 'n':
      case 'N':
        event.preventDefault();
        navigateToNextItem();
        break;

      case 'Enter':
        event.preventDefault();
        approveCurrentItem();
        break;

      case 'Escape':
        event.preventDefault();
        if (isBatchVisible) {
          showMainReview();
        } else {
          rejectCurrentItem();
        }
        break;

      case 'PageUp':
        event.preventDefault();
        goToPage(currentPage - 1);
        break;

      case 'PageDown':
        event.preventDefault();
        goToPage(currentPage + 1);
        break;

      case 'Home':
        event.preventDefault();
        goToPage(1);
        break;

      case 'End':
        event.preventDefault();
        goToPage(totalPages);
        break;

      case '+':
      case '=':
        if (event.ctrlKey) {
          event.preventDefault();
          adjustFontSize(FONT_SIZE_STEP);
        }
        break;

      case '-':
        if (event.ctrlKey) {
          event.preventDefault();
          adjustFontSize(-FONT_SIZE_STEP);
        }
        break;
    }
  }

  function navigateToPrevItem() {
    for (var i = selectedItemIndex - 1; i >= 0; i--) {
      var lineNum = i + 1;
      if (lineGroups[lineNum] && lineGroups[lineNum].length > 0) {
        var startLine = (currentPage - 1) * pageSize;
        if (i < startLine) {
          currentPage--;
          selectedItemIndex = i;
          renderCurrentPage();
        } else {
          selectedItemIndex = i;
          highlightSelectedItem();
        }
        return;
      }
    }
  }

  function navigateToNextItem() {
    for (var i = selectedItemIndex + 1; i < fullLines.length; i++) {
      var lineNum = i + 1;
      if (lineGroups[lineNum] && lineGroups[lineNum].length > 0) {
        var endLine = currentPage * pageSize;
        if (i >= endLine) {
          currentPage++;
          selectedItemIndex = i;
          renderCurrentPage();
        } else {
          selectedItemIndex = i;
          highlightSelectedItem();
        }
        return;
      }
    }
  }

  function approveCurrentItem() {
    if (selectedItemIndex < 0) return;
    var lineNum = selectedItemIndex + 1;
    var items = lineGroups[lineNum];
    if (!items) return;

    var pendingItems = items.filter(function(item) { return item.status === 'pending'; });
    if (pendingItems.length === 0) return;

    if (pendingItems.length === 1) {
      approveReviewItem(pendingItems[0].id);
    } else {
      var ids = pendingItems.map(function(i) { return i.id; });
      batchApproveGroup(ids);
    }
  }

  function rejectCurrentItem() {
    if (selectedItemIndex < 0) return;
    var lineNum = selectedItemIndex + 1;
    var items = lineGroups[lineNum];
    if (!items) return;

    var pendingItems = items.filter(function(item) { return item.status === 'pending'; });
    if (pendingItems.length === 0) return;

    if (pendingItems.length === 1) {
      rejectReviewItem(pendingItems[0].id);
    } else {
      var ids = pendingItems.map(function(i) { return i.id; });
      batchRejectGroup(ids);
    }
  }

  // ---------------------------------------------------------------
  // 单条操作
  // ---------------------------------------------------------------
  function _reviewAction(endpoint, body, successMsg) {
    if (!currentFileMd5) return;
    AppConfig.apiRequest('/files/' + currentFileMd5 + endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
      .then(function(data) {
        if (data.success) {
          showFeedback(successMsg, 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '操作失败', 'error');
        }
      })
      .catch(function(err) { showFeedback('操作失败: ' + err.message, 'error'); });
  }

  function approveReviewItem(itemId) {
    _reviewAction('/approve', { itemId: itemId }, '✓ 已采纳修改建议');
  }

  function rejectReviewItem(itemId) {
    _reviewAction('/reject', { itemId: itemId }, '✗ 已拒绝修改建议');
  }

  function restoreReviewItem(itemId) {
    _reviewAction('/restore', { itemId: itemId }, '已恢复原文');
  }

  // ---------------------------------------------------------------
  // 批量操作
  // ---------------------------------------------------------------
  function _batchAction(endpoint, itemIds, successMsg) {
    if (!currentFileMd5 || !itemIds.length) return;
    AppConfig.apiRequest('/files/' + currentFileMd5 + endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemIds: itemIds })
    })
      .then(function(data) {
        if (data.success) {
          showFeedback(successMsg, 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '批量操作失败', 'error');
        }
      })
      .catch(function(err) { showFeedback('批量操作失败: ' + err.message, 'error'); });
  }

  function batchApproveGroup(itemIds) {
    _batchAction('/batch-approve', itemIds, '✓ 已采纳 ' + itemIds.length + ' 项修改');
  }

  function batchRejectGroup(itemIds) {
    _batchAction('/batch-reject', itemIds, '✗ 已拒绝 ' + itemIds.length + ' 项修改');
  }

  // ---------------------------------------------------------------
  // 全局批量操作
  // ---------------------------------------------------------------
  function _batchAllAction(endpoint, successVerb) {
    if (!currentFileMd5) return;
    var pendingIds = reviewItems.filter(function(item) { return item.status === 'pending'; }).map(function(item) { return item.id; });
    if (pendingIds.length === 0) {
      showFeedback('没有待审核项', 'info');
      return;
    }
    _batchAction(endpoint, pendingIds, successVerb + '所有待审核项 (' + pendingIds.length + ')');
  }

  function batchApproveAll() {
    _batchAllAction('/batch-approve', '✓ 已通过');
  }

  function batchRejectAll() {
    _batchAllAction('/batch-reject', '✗ 已拒绝');
  }

  // ---------------------------------------------------------------
  // 完成审核
  // ---------------------------------------------------------------
  function finalizeFile() {
    if (!currentFileMd5) return;
    AppConfig.apiRequest('/files/' + currentFileMd5 + '/finalize', {
      method: 'POST'
    })
      .then(function(data) {
        if (data.success) {
          showFeedback('文件处理完成', 'success');
          FileManager.showSection('completed');
        } else {
          showFeedback(data.message || '完成处理失败', 'error');
        }
      })
      .catch(function(err) {
        showFeedback('完成处理失败: ' + err.message, 'error');
      });
  }

  // ---------------------------------------------------------------
  // 进度更新
  // ---------------------------------------------------------------
  function updateReviewProgress() {
    var total = reviewItems.length;
    var pending = reviewItems.filter(function(item) { return item.status === 'pending'; }).length;
    var processed = total - pending;
    var progressText = document.getElementById('review-progress-text');
    var finalizeBtn = document.getElementById('finalize-btn');

    if (progressText) {
      DomUtils.setTextContent(progressText, processed + '/' + total);
    }

    if (finalizeBtn) {
      finalizeBtn.style.display = pending === 0 && total > 0 ? 'inline-block' : 'none';
    }

    // 更新分类标签
    var categoryEl = document.getElementById('review-current-category');
    if (categoryEl) {
      var statusText = getFilterValue() === 'pending' ? '待审核' : '全部';
      var reviewLineCount = Object.keys(lineGroups).length;
      DomUtils.setTextContent(categoryEl, statusText + ' (' + reviewLineCount + '行 / ' + filteredItems.length + '项)');
    }
  }

  // ---------------------------------------------------------------
  // 批量审核页面
  // ---------------------------------------------------------------
  function showBatchReview(group) {
    currentBatchGroup = group;
    var reviewSection = document.getElementById('review-section');
    var batchSection = document.getElementById('batch-review-section');
    if (reviewSection) reviewSection.style.display = 'none';
    if (batchSection) batchSection.style.display = 'block';

    renderBatchReviewPage(group);
  }

  function showMainReview() {
    currentBatchGroup = null;
    var reviewSection = document.getElementById('review-section');
    var batchSection = document.getElementById('batch-review-section');
    if (batchSection) batchSection.style.display = 'none';
    if (reviewSection) reviewSection.style.display = 'block';
  }

  function renderBatchReviewPage(group) {
    var container = document.getElementById('batch-review-container');
    if (!container) return;
    container.innerHTML = '';

    // 标题
    var titleEl = document.getElementById('batch-review-title');
    if (titleEl) {
      var displayOrig = group.original.length > 40 ? group.original.slice(0, 40) + '…' : group.original;
      var displaySugg = group.suggested.length > 40 ? group.suggested.slice(0, 40) + '…' : group.suggested;
      DomUtils.setTextContent(titleEl, '"' + displayOrig + '" → "' + displaySugg + '"');
    }

    // 计数
    var countEl = document.getElementById('batch-review-count');
    if (countEl) {
      var pendingCount = group.items.filter(function(i) { return i.status === 'pending'; }).length;
      DomUtils.setTextContent(countEl, '共 ' + group.items.length + ' 项 (待审核 ' + pendingCount + '项)');
    }

    // 渲染每个项
    group.items.forEach(function(item) {
      var itemEl = DomUtils.createElement('div', { className: 'batch-group-item' });

      // 上下文
      var contextEl = DomUtils.createElement('div', { className: 'batch-item-context' });
      var lineNum = item.lineNum ? '行 ' + item.lineNum + ': ' : '';
      var context = (item.prevLines && item.prevLines[0]) || '';
      DomUtils.setTextContent(contextEl, lineNum + context);
      itemEl.appendChild(contextEl);

      // 操作按钮
      var actionsEl = DomUtils.createElement('div', { className: 'batch-item-actions' });

      if (item.status === 'pending') {
        var approveBtn = DomUtils.createElement('button', {
          className: 'btn-sm btn-primary',
          onclick: function() {
            approveReviewItem(item.id);
            var updatedItems = group.items.filter(function(i) { return i.id !== item.id; });
            if (updatedItems.length === 0) {
              showMainReview();
            } else {
              group.items = updatedItems;
              renderBatchReviewPage(group);
            }
          }
        });
        DomUtils.setTextContent(approveBtn, '✓ 同意');
        actionsEl.appendChild(approveBtn);

        var rejectBtn = DomUtils.createElement('button', {
          className: 'btn-sm',
          onclick: function() {
            rejectReviewItem(item.id);
            var updatedItems = group.items.filter(function(i) { return i.id !== item.id; });
            if (updatedItems.length === 0) {
              showMainReview();
            } else {
              group.items = updatedItems;
              renderBatchReviewPage(group);
            }
          }
        });
        DomUtils.setTextContent(rejectBtn, '✗ 拒绝');
        actionsEl.appendChild(rejectBtn);
      } else {
        var statusSpan = DomUtils.createElement('span', {
          className: 'btn-sm',
          style: 'opacity: 0.5;'
        });
        DomUtils.setTextContent(statusSpan, getReviewStatusText(item.status));
        actionsEl.appendChild(statusSpan);
      }

      itemEl.appendChild(actionsEl);
      container.appendChild(itemEl);
    });
  }

  // ---------------------------------------------------------------
  // 类型/状态文本
  // ---------------------------------------------------------------
  function getReviewStatusText(status) {
    var map = {
      pending: '待审核',
      approved: '已通过',
      rejected: '已拒绝',
      edited: '已编辑'
    };
    return map[status] || status || '';
  }

  // ---------------------------------------------------------------
  // 公共 API
  // ---------------------------------------------------------------
  return {
    init: init,
    loadReviewItems: loadReviewItems,
    approveReviewItem: approveReviewItem,
    rejectReviewItem: rejectReviewItem,
    editReviewItem: function(itemId) {
      var item = reviewItems.find(function(i) { return i.id === itemId; });
      if (!item) return;
      var editDefault = item.suggested || item.suggestedText ||
        item.original || item.originalText || '';
      var editedText = prompt('请输入编辑后的文本:', editDefault);
      if (editedText === null) return;
      AppConfig.apiRequest('/files/' + currentFileMd5 + '/edit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ itemId: itemId, editedText: editedText })
      })
        .then(function(data) {
          if (data.success) {
            showFeedback('已保存编辑', 'success');
            loadReviewItems();
          } else {
            showFeedback(data.message || '编辑失败', 'error');
          }
        })
        .catch(function(err) {
          showFeedback('编辑失败: ' + err.message, 'error');
        });
    },
    restoreReviewItem: restoreReviewItem,
    batchApproveGroup: batchApproveGroup,
    batchRejectGroup: batchRejectGroup,
    batchApproveAll: batchApproveAll,
    batchRejectAll: batchRejectAll,
    finalizeFile: finalizeFile,
    setCurrentFileMd5: function(md5) { currentFileMd5 = md5; },
    getCurrentFileMd5: function() { return currentFileMd5; }
  };
})();

// 导出到全局作用域
window.ReviewModule = ReviewModule;
