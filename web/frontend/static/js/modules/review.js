// 审核模块 — 双栏面板 + 内联对比 + 快捷键
const ReviewModule = (function() {
  // 私有状态
  let reviewItems = [];
  let currentFileMd5 = null;
  let selectedCardIndex = -1;
  let currentCategory = 'all';
  let categorizedItems = {};
  let sidebarCategories = [];

  // 分类定义
  const CATEGORY_DEFS = {
    all: { label: '全部待审核', icon: 'all' },
    typo: { label: '高频词汇类', icon: 'typo' },
    semantic: { label: '语义逻辑类', icon: 'semantic' },
    cleaning: { label: '清洗类', icon: 'cleaning' }
  };

  // 映射：type -> category
  const TYPE_TO_CATEGORY = {
    typo: 'typo',
    typo_correction: 'typo',
    character_correction: 'typo',
    punctuation: 'typo',
    grammar: 'semantic',
    style: 'semantic',
    llm_fix: 'semantic',
    duplicate_paragraph: 'cleaning',
    advertisement: 'cleaning',
    text_deletion: 'cleaning',
    text_insertion: 'cleaning',
    html_entity: 'cleaning',
    whitespace: 'cleaning',
    encoding_fix: 'cleaning',
    garbled_text_removal: 'cleaning',
    traditional_to_simple: 'cleaning'
  };

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
      statusFilter.addEventListener('change', function() {
        loadReviewItems();
      });
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

    // 全局键盘快捷键
    document.addEventListener('keydown', handleKeyboardShortcut);
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
        categorizeItems();
        renderSidebar();
        renderCurrentCategory();
        updateReviewProgress();
      })
      .catch((err) => {
        console.error("加载审核项失败:", err);
      });
  }

  // ---------------------------------------------------------------
  // 分类逻辑
  // ---------------------------------------------------------------
  function categorizeItems() {
    const statusFilter = getFilterValue();

    let filtered = reviewItems;
    if (statusFilter === 'pending') {
      filtered = reviewItems.filter(item => item.status === 'pending');
    } else if (statusFilter !== 'all') {
      filtered = reviewItems.filter(item => item.status === statusFilter);
    }

    // 构建分类
    const categories = {
      all: { items: filtered, label: '全部待审核', icon: 'all' },
      typo: { items: [], label: '高频词汇类', icon: 'typo' },
      semantic: { items: [], label: '语义逻辑类', icon: 'semantic' },
      cleaning: { items: [], label: '清洗类', icon: 'cleaning' }
    };

    // 按 type 归类
    filtered.forEach(item => {
      const type = item.type || item.modificationType || '';
      const cat = TYPE_TO_CATEGORY[type] || 'semantic';
      if (categories[cat]) {
        categories[cat].items.push(item);
      } else {
        categories.semantic.items.push(item);
      }
    });

    // 对 typo 类进一步按原文→建议分组
    categories.typo.subGroups = groupTypoItems(categories.typo.items);

    categorizedItems = categories;

    // 构建侧边栏导航列表
    sidebarCategories = [];
    for (const [key, cat] of Object.entries(categories)) {
      const count = cat.items.length;
      if (count > 0 || key === 'all') {
        sidebarCategories.push({
          key: key,
          label: cat.label,
          icon: cat.icon,
          count: count,
          subGroups: cat.subGroups || null
        });
      }
    }
  }

  // 对高频词汇类按相同原文→建议分组（带子导航）
  function groupTypoItems(items) {
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
    const groups = Object.values(groupMap);
    groups.sort((a, b) => b.count - a.count);
    return groups;
  }

  function getFilterValue() {
    const el = document.getElementById('review-status-filter');
    return el ? el.value : 'pending';
  }

  // ---------------------------------------------------------------
  // 渲染左侧导航
  // ---------------------------------------------------------------
  function renderSidebar() {
    const nav = document.getElementById('sidebar-nav');
    if (!nav) return;

    nav.innerHTML = '';

    sidebarCategories.forEach(cat => {
      const item = DomUtils.createElement('div', {
        className: 'sidebar-nav-item' + (cat.key === currentCategory ? ' active' : ''),
        onclick: function() { selectCategory(cat.key); }
      });

      const icon = DomUtils.createElement('div', {
        className: 'sidebar-nav-icon icon-' + cat.icon
      });
      DomUtils.setTextContent(icon, getCategoryEmoji(cat.key));
      item.appendChild(icon);

      const label = DomUtils.createElement('span', { className: 'sidebar-nav-label' });
      DomUtils.setTextContent(label, cat.label);
      item.appendChild(label);

      const countEl = DomUtils.createElement('span', { className: 'sidebar-nav-count' });
      DomUtils.setTextContent(countEl, String(cat.count));
      item.appendChild(countEl);

      nav.appendChild(item);

      // 如果有子分组，在 typo 类别下显示子项
      if (cat.subGroups && cat.key === currentCategory) {
        cat.subGroups.forEach((group, idx) => {
          const subItem = DomUtils.createElement('div', {
            className: 'sidebar-nav-item sub-nav-item',
            style: 'padding-left: 44px; font-size: 13px;',
            onclick: function(e) {
              e.stopPropagation();
              scrollToGroup(idx);
            }
          });
          const subLabel = DomUtils.createElement('span', {
            className: 'sidebar-nav-label',
            style: 'font-size: 13px; color: var(--text-secondary);'
          });
          const displayOrig = group.original.length > 20
            ? group.original.slice(0, 20) + '…'
            : group.original;
          const displaySugg = group.suggested.length > 20
            ? group.suggested.slice(0, 20) + '…'
            : group.suggested;
          DomUtils.setTextContent(subLabel, displayOrig + ' → ' + displaySugg);
          subItem.appendChild(subLabel);

          const subCount = DomUtils.createElement('span', {
            className: 'sidebar-nav-count',
            style: 'font-size: 11px;'
          });
          DomUtils.setTextContent(subCount, String(group.count) + '处');
          subItem.appendChild(subCount);

          nav.appendChild(subItem);
        });
      }
    });
  }

  function getCategoryEmoji(key) {
    const map = {
      all: '\u2211',
      typo: 'A',
      semantic: '\u25B3',
      cleaning: '\u221E'
    };
    return map[key] || '?';
  }

  // 选择分类
  function selectCategory(key) {
    currentCategory = key;
    selectedCardIndex = -1;
    renderSidebar();
    renderCurrentCategory();
  }

  function scrollToGroup(idx) {
    const container = document.getElementById('review-items-scroll');
    if (!container) return;
    const cards = container.querySelectorAll('.diff-card');
    if (cards[idx]) {
      cards[idx].scrollIntoView({ behavior: 'smooth', block: 'center' });
      cards[idx].classList.add('active');
      setTimeout(() => cards[idx].classList.remove('active'), 2000);
    }
  }

  // ---------------------------------------------------------------
  // 渲染当前分类的审核项
  // ---------------------------------------------------------------
  function renderCurrentCategory() {
    const container = document.getElementById('review-items-list');
    if (!container) return;

    container.innerHTML = '';

    const catData = categorizedItems[currentCategory];
    if (!catData || catData.items.length === 0) {
      const emptyDiv = DomUtils.createElement('div', { className: 'empty-state-sm' });
      DomUtils.setTextContent(emptyDiv, '没有待审核项');
      container.appendChild(emptyDiv);
      updateCategoryLabel('当前无待审核项');
      return;
    }

    const categoryLabel = catData.label + ' (' + catData.items.length + '项)';
    updateCategoryLabel(categoryLabel);

    // 如果有子分组（typo类），渲染批量操作横幅 + 组内项
    if (catData.subGroups && catData.subGroups.length > 0) {
      catData.subGroups.forEach((group, gIdx) => {
        // 组标题 + 批量操作
        const batchBar = DomUtils.createElement('div', {
          className: 'category-batch-bar'
        });

        const batchLabel = DomUtils.createElement('span', {
          className: 'category-batch-label'
        });
        const displayOrig = group.original.length > 40
          ? group.original.slice(0, 40) + '…'
          : group.original;
        const displaySugg = group.suggested.length > 40
          ? group.suggested.slice(0, 40) + '…'
          : group.suggested;
        DomUtils.setTextContent(batchLabel, '\u201C' + displayOrig + '\u201D \u2192 \u201C' + displaySugg + '\u201D \u00D7 ' + group.count);

        const batchActions = DomUtils.createElement('div', {
          className: 'category-batch-actions'
        });

        const approveAllBtn = DomUtils.createElement('button', {
          className: 'btn-approve',
          onclick: function() {
            batchApproveGroup(group.items.map(i => i.id));
          }
        });
        DomUtils.setTextContent(approveAllBtn, '\u2713 全部采纳');
        batchActions.appendChild(approveAllBtn);

        const rejectAllBtn = DomUtils.createElement('button', {
          className: 'btn-reject',
          onclick: function() {
            batchRejectGroup(group.items.map(i => i.id));
          }
        });
        DomUtils.setTextContent(rejectAllBtn, '\u2717 全部忽略');
        batchActions.appendChild(rejectAllBtn);

        batchBar.appendChild(batchLabel);
        batchBar.appendChild(batchActions);
        container.appendChild(batchBar);

        // 组内每个项渲染为卡片
        group.items.forEach((item, idx) => {
          const card = createDiffCard(item, idx);
          container.appendChild(card);
        });
      });
    } else {
      // 无子分组，直接渲染卡片
      catData.items.forEach((item, idx) => {
        const card = createDiffCard(item, idx);
        container.appendChild(card);
      });
    }

    // 重新计算选中索引
    if (selectedCardIndex < 0) {
      selectedCardIndex = 0;
    }
    highlightSelectedCard();
  }

  function updateCategoryLabel(text) {
    const el = document.getElementById('review-current-category');
    if (el) {
      DomUtils.setTextContent(el, text);
    }
  }

  // ---------------------------------------------------------------
  // 创建 Diff 对比卡片
  // ---------------------------------------------------------------
  function createDiffCard(item, index) {
    const card = DomUtils.createElement('div', {
      className: 'diff-card',
      'data-index': String(index),
      'data-item-id': String(item.id)
    });

    // 点击卡片选中
    card.addEventListener('click', function(e) {
      // 不处理按钮点击冒泡
      if (e.target.closest('.diff-card-btn') || e.target.closest('.diff-edit-inline') ||
          e.target.closest('.diff-expand-toggle') || e.target.closest('.diff-edit-actions')) {
        return;
      }
      selectedCardIndex = index;
      highlightSelectedCard();
    });

    // ---- 头部 ----
    const header = DomUtils.createElement('div', { className: 'diff-card-header' });

    const typeLabel = DomUtils.createElement('span', {
      className: 'diff-card-type type-' + getCategoryForItem(item)
    });
    DomUtils.setTextContent(typeLabel, getModificationTypeText(item.type || item.modificationType));
    header.appendChild(typeLabel);

    if (item.lineNum !== undefined && item.lineNum !== null) {
      const lineSpan = DomUtils.createElement('span', { className: 'diff-card-line' });
      DomUtils.setTextContent(lineSpan, '行 ' + item.lineNum);
      header.appendChild(lineSpan);
    }

    card.appendChild(header);

    // ---- 浮动操作按钮 ----
    const actions = DomUtils.createElement('div', { className: 'diff-card-actions' });
    if (item.status === 'pending') {
      const approveBtn = DomUtils.createElement('button', {
        className: 'diff-card-btn btn-approve-sm',
        title: '采纳 (Enter)',
        onclick: function(e) {
          e.stopPropagation();
          approveReviewItem(item.id);
        }
      });
      DomUtils.setTextContent(approveBtn, '\u2713');
      actions.appendChild(approveBtn);

      const rejectBtn = DomUtils.createElement('button', {
        className: 'diff-card-btn btn-reject-sm',
        title: '拒绝 (Esc)',
        onclick: function(e) {
          e.stopPropagation();
          rejectReviewItem(item.id);
        }
      });
      DomUtils.setTextContent(rejectBtn, '\u2717');
      actions.appendChild(rejectBtn);

      const editBtn = DomUtils.createElement('button', {
        className: 'diff-card-btn btn-edit-sm',
        title: '手动微调',
        onclick: function(e) {
          e.stopPropagation();
          toggleEditInline(item.id);
        }
      });
      DomUtils.setTextContent(editBtn, '\u270E');
      actions.appendChild(editBtn);
    }
    card.appendChild(actions);

    // ---- 主体内容 ----
    const body = DomUtils.createElement('div', { className: 'diff-card-body' });

    // 上文背景：显示前3行上下文
    if (item.prevLines && item.prevLines.length > 0) {
      const contextBlock = DomUtils.createElement('div', { className: 'diff-context-block' });
      item.prevLines.forEach(function(line) {
        const lineEl = DomUtils.createElement('div', { className: 'diff-context-line' });
        DomUtils.setTextContent(lineEl, line);
        contextBlock.appendChild(lineEl);
      });
      body.appendChild(contextBlock);
    } else if (item.prevLine) {
      // 向后兼容：单行上下文
      const contextBlock = DomUtils.createElement('div', { className: 'diff-context-block' });
      const lineEl = DomUtils.createElement('div', { className: 'diff-context-line' });
      DomUtils.setTextContent(lineEl, item.prevLine);
      contextBlock.appendChild(lineEl);
      body.appendChild(contextBlock);
    }

    // 原文行（浅红背景，带删除线高亮）
    const orig = item.original || item.originalText || '';
    if (orig) {
      const origLine = DomUtils.createElement('div', { className: 'diff-original-line' });
      const origLabel = DomUtils.createElement('span', { className: 'diff-line-label label-original' });
      DomUtils.setTextContent(origLabel, '原文');
      origLine.appendChild(origLabel);

      const sugg = item.suggested || item.suggestedText || '';
      const hasDiffUtils = typeof DiffUtils !== 'undefined';
      if (hasDiffUtils && sugg && orig !== sugg) {
        const diffResult = DiffUtils.renderInlineDiff(orig, sugg);
        const origText = DomUtils.createElement('span');
        DomUtils.setHTML(origText, diffResult.originalHtml);
        origLine.appendChild(origText);
      } else {
        const origText = DomUtils.createElement('span');
        DomUtils.setTextContent(origText, orig);
        origLine.appendChild(origText);
      }

      body.appendChild(origLine);
    }

    // 建议行（浅绿背景，加粗高亮）
    const sugg = item.suggested || item.suggestedText || '';
    if (sugg) {
      const suggLine = DomUtils.createElement('div', { className: 'diff-suggested-line' });
      const suggLabel = DomUtils.createElement('span', { className: 'diff-line-label label-suggested' });
      DomUtils.setTextContent(suggLabel, '建议');
      suggLine.appendChild(suggLabel);

      const hasDiffUtils = typeof DiffUtils !== 'undefined';
      if (hasDiffUtils && orig && orig !== sugg) {
        const diffResult = DiffUtils.renderInlineDiff(orig, sugg);
        const suggText = DomUtils.createElement('span');
        DomUtils.setHTML(suggText, diffResult.suggestedHtml);
        suggLine.appendChild(suggText);
      } else {
        const suggText = DomUtils.createElement('span');
        DomUtils.setTextContent(suggText, sugg);
        suggLine.appendChild(suggText);
      }

      body.appendChild(suggLine);
    }

    // 编辑后文本显示（如果是 edited 状态）
    if (item.editedText) {
      const editedLine = DomUtils.createElement('div', {
        className: 'diff-original-line',
        style: 'border-left-color: var(--ash-purple); background: rgba(107,92,231,0.06);'
      });
      const editedLabel = DomUtils.createElement('span', {
        className: 'diff-line-label',
        style: 'color: var(--ash-purple);'
      });
      DomUtils.setTextContent(editedLabel, '编辑后');
      editedLine.appendChild(editedLabel);
      const editedTextEl = DomUtils.createElement('span');
      DomUtils.setTextContent(editedTextEl, item.editedText);
      editedLine.appendChild(editedTextEl);
      body.appendChild(editedLine);
    }

    // 内联编辑区（隐藏）
    const editContainer = DomUtils.createElement('div', {
      className: 'diff-edit-inline',
      id: 'edit-inline-' + item.id,
      style: 'display: none;'
    });
    const editTextarea = DomUtils.createElement('textarea', {
      id: 'edit-textarea-' + item.id
    });
    DomUtils.setTextContent(editTextarea, item.editedText || sugg || orig || '');
    editContainer.appendChild(editTextarea);

    const editActions = DomUtils.createElement('div', { className: 'diff-edit-actions' });
    const saveEditBtn = DomUtils.createElement('button', {
      className: 'btn-edit',
      onclick: function() { saveInlineEdit(item.id); }
    });
    DomUtils.setTextContent(saveEditBtn, '保存');
    editActions.appendChild(saveEditBtn);

    const cancelEditBtn = DomUtils.createElement('button', {
      className: 'btn-secondary',
      onclick: function() { cancelInlineEdit(item.id); }
    });
    DomUtils.setTextContent(cancelEditBtn, '取消');
    editActions.appendChild(cancelEditBtn);

    editContainer.appendChild(editActions);
    body.appendChild(editContainer);

    // 下文背景：显示后3行上下文
    if (item.nextLines && item.nextLines.length > 0) {
      const contextBlock = DomUtils.createElement('div', { className: 'diff-context-block diff-context-block-after' });
      item.nextLines.forEach(function(line) {
        const lineEl = DomUtils.createElement('div', { className: 'diff-context-line' });
        DomUtils.setTextContent(lineEl, line);
        contextBlock.appendChild(lineEl);
      });
      body.appendChild(contextBlock);
    } else if (item.nextLine) {
      // 向后兼容：单行上下文
      const contextBlock = DomUtils.createElement('div', { className: 'diff-context-block diff-context-block-after' });
      const lineEl = DomUtils.createElement('div', { className: 'diff-context-line' });
      DomUtils.setTextContent(lineEl, item.nextLine);
      contextBlock.appendChild(lineEl);
      body.appendChild(contextBlock);
    }

    // 状态显示
    if (item.status && item.status !== 'pending') {
      const statusLine = DomUtils.createElement('div', {
        style: 'margin-top: 8px; font-size: 11px; font-family: var(--font-mono);'
      });
      const statusSpan = DomUtils.createElement('span', {
        className: 'review-status review-status-' + item.status
      });
      DomUtils.setTextContent(statusSpan, getReviewStatusText(item.status));
      statusLine.appendChild(statusSpan);
      body.appendChild(statusLine);
    } else if (item.confidence !== undefined && item.confidence !== null) {
      const confLine = DomUtils.createElement('div', {
        style: 'margin-top: 6px; font-size: 11px; color: var(--text-muted); font-family: var(--font-mono);'
      });
      DomUtils.setTextContent(confLine, '置信度: ' + (item.confidence * 100).toFixed(1) + '%');
      body.appendChild(confLine);
    }

    card.appendChild(body);
    return card;
  }

  // 创建上下文行
  function createContextLine(text, isCollapsed) {
    if (isCollapsed) {
      const line = DomUtils.createElement('div', {
        className: 'diff-context-line context-collapsed'
      });
      DomUtils.setTextContent(line, '\u2026 \u70B9\u51FB\u5C55\u5F00 \u2026');
      return line;
    }
    const line = DomUtils.createElement('div', { className: 'diff-context-line' });
    DomUtils.setTextContent(line, text);
    return line;
  }

  // 获取项的分类 key
  function getCategoryForItem(item) {
    const type = item.type || item.modificationType || '';
    return TYPE_TO_CATEGORY[type] || 'semantic';
  }

  // ---------------------------------------------------------------
  // 选中高亮
  // ---------------------------------------------------------------
  function highlightSelectedCard() {
    const container = document.getElementById('review-items-list');
    if (!container) return;
    const cards = container.querySelectorAll('.diff-card');
    cards.forEach((card, idx) => {
      card.classList.toggle('active', idx === selectedCardIndex);
    });
    // 确保选中卡片可见
    if (selectedCardIndex >= 0 && selectedCardIndex < cards.length) {
      cards[selectedCardIndex].scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
  }

  // ---------------------------------------------------------------
  // 键盘快捷键
  // ---------------------------------------------------------------
  function handleKeyboardShortcut(event) {
    const reviewSection = document.getElementById('review-section');
    if (!reviewSection || reviewSection.style.display === 'none') return;

    // 忽略输入框中的快捷键
    if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA' ||
        event.target.tagName === 'SELECT') return;

    const catData = categorizedItems[currentCategory];
    if (!catData || catData.items.length === 0) return;

    switch (event.key) {
      case 'ArrowUp':
        event.preventDefault();
        if (selectedCardIndex > 0) {
          selectedCardIndex--;
          highlightSelectedCard();
        }
        break;

      case 'ArrowDown':
        event.preventDefault();
        if (selectedCardIndex < catData.items.length - 1) {
          selectedCardIndex++;
          highlightSelectedCard();
        }
        break;

      case 'Enter':
        event.preventDefault();
        if (selectedCardIndex >= 0 && selectedCardIndex < catData.items.length) {
          const item = catData.items[selectedCardIndex];
          if (item.status === 'pending') {
            approveReviewItem(item.id);
          }
        }
        break;

      case 'Escape':
        event.preventDefault();
        if (selectedCardIndex >= 0 && selectedCardIndex < catData.items.length) {
          const item = catData.items[selectedCardIndex];
          if (item.status === 'pending') {
            rejectReviewItem(item.id);
          }
        }
        break;
    }
  }

  // ---------------------------------------------------------------
  // 内联编辑
  // ---------------------------------------------------------------
  function toggleEditInline(itemId) {
    const container = document.getElementById('edit-inline-' + itemId);
    if (!container) return;
    const isHidden = container.style.display === 'none';
    // 隐藏所有编辑区
    document.querySelectorAll('.diff-edit-inline').forEach(el => {
      el.style.display = 'none';
    });
    container.style.display = isHidden ? 'block' : 'none';
  }

  function saveInlineEdit(itemId) {
    const textarea = document.getElementById('edit-textarea-' + itemId);
    if (!textarea) return;
    const editedText = textarea.value.trim();
    if (!editedText) {
      showFeedback('请输入编辑后的文本', 'error');
      return;
    }

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

  function cancelInlineEdit(itemId) {
    const container = document.getElementById('edit-inline-' + itemId);
    if (container) {
      container.style.display = 'none';
    }
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
          showFeedback('\u2713 已采纳修改建议', 'success');
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
          showFeedback('\u2717 已拒绝修改建议', 'success');
          loadReviewItems();
        } else {
          showFeedback(data.message || '操作失败', 'error');
        }
      })
      .catch((err) => {
        showFeedback('操作失败: ' + err.message, 'error');
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
    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-approve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemIds: itemIds })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('\u2713 已采纳 ' + itemIds.length + ' 项修改', 'success');
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
    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-reject`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemIds: itemIds })
    })
      .then((data) => {
        if (data.success) {
          showFeedback('\u2717 已拒绝 ' + itemIds.length + ' 项修改', 'success');
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
    if (!currentFileMd5) return;
    const pendingItems = reviewItems.filter(item => item.status === 'pending');
    if (pendingItems.length === 0) {
      showFeedback('没有待审核项', 'info');
      return;
    }
    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-approve`, {
      method: 'POST'
    })
      .then((data) => {
        if (data.success) {
          showFeedback('\u2713 已通过所有待审核项 (' + pendingItems.length + ')', 'success');
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
    if (!currentFileMd5) return;
    const pendingItems = reviewItems.filter(item => item.status === 'pending');
    if (pendingItems.length === 0) {
      showFeedback('没有待审核项', 'info');
      return;
    }
    AppConfig.apiRequest(`/files/${currentFileMd5}/batch-reject`, {
      method: 'POST'
    })
      .then((data) => {
        if (data.success) {
          showFeedback('\u2717 已拒绝所有待审核项 (' + pendingItems.length + ')', 'success');
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
    if (!currentFileMd5) return;
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
    const processed = total - pending;
    const progressText = document.getElementById('review-progress-text');
    const finalizeBtn = document.getElementById('finalize-btn');

    if (progressText) {
      DomUtils.setTextContent(progressText, processed + '/' + total);
    }

    if (finalizeBtn) {
      finalizeBtn.style.display = pending === 0 && total > 0 ? 'inline-block' : 'none';
    }
  }

  // ---------------------------------------------------------------
  // 类型/状态文本
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
      traditional_to_simple: '繁转简'
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
  // Toast
  // ---------------------------------------------------------------
  function showFeedback(message, type) {
    const fb = document.getElementById('feedback');
    if (!fb) return;
    fb.textContent = message;
    fb.className = 'feedback ' + (type || 'success');
    fb.style.display = 'block';
    setTimeout(() => {
      fb.style.display = 'none';
    }, 3000);
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
      // Fallback prompt-based edit
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
