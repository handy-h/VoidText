// diff-utils.js - 文本 diff 算法和渲染
const DiffUtils = (function() {
  // 转义 HTML 特殊字符
  function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  // 分词：按中文/英文/数字/标点/空白分词
  function tokenize(text) {
    if (!text) return [];
    // 匹配规则：
    //   1. CJK 字符（汉字、扩展A、兼容表意文字）→ 每个字符单独一个 token
    //   2. 连续英文字母 → 一个 token（英文单词）
    //   3. 连续数字 → 一个 token
    //   4. 连续空白（含换行）→ 一个 token
    //   5. 其他单个字符（标点符号等）→ 一个 token
    const pattern = /[\u4e00-\u9fff\u3400-\u4dbf\uf900-\ufaff]|[a-zA-Z]+|[0-9]+|\s+|./g;
    const matches = text.match(pattern);
    return matches || [];
  }

  // LCS 算法：计算两个 token 数组的最长公共子序列
  // 返回 DP 表，用于后续回溯生成 diff 操作
  function lcs(tokens1, tokens2) {
    const n = tokens1.length;
    const m = tokens2.length;
    // DP 表: dp[i][j] = tokens1[0..i-1] 和 tokens2[0..j-1] 的 LCS 长度
    const dp = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));

    for (let i = 1; i <= n; i++) {
      for (let j = 1; j <= m; j++) {
        if (tokens1[i - 1] === tokens2[j - 1]) {
          dp[i][j] = dp[i - 1][j - 1] + 1;
        } else {
          dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1]);
        }
      }
    }

    return dp;
  }

  // 生成 diff 操作列表：[{type: 'equal'|'insert'|'delete', value: string}]
  function diff(text1, text2) {
    const tokens1 = tokenize(text1);
    const tokens2 = tokenize(text2);
    const dp = lcs(tokens1, tokens2);

    const result = [];
    let i = tokens1.length;
    let j = tokens2.length;

    // 从 DP 表回溯，构建操作序列（逆序）
    while (i > 0 || j > 0) {
      if (i > 0 && j > 0 && tokens1[i - 1] === tokens2[j - 1]) {
        result.push({ type: 'equal', value: tokens1[i - 1] });
        i--;
        j--;
      } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
        result.push({ type: 'insert', value: tokens2[j - 1] });
        j--;
      } else if (i > 0) {
        result.push({ type: 'delete', value: tokens1[i - 1] });
        i--;
      }
    }

    // 反转为正序
    return result.reverse();
  }

  // 合并相邻的相同类型操作
  function mergeOps(ops) {
    if (ops.length === 0) return [];

    const merged = [];
    let current = { type: ops[0].type, value: ops[0].value };

    for (let k = 1; k < ops.length; k++) {
      if (ops[k].type === current.type) {
        current.value += ops[k].value;
      } else {
        merged.push(current);
        current = { type: ops[k].type, value: ops[k].value };
      }
    }
    merged.push(current);

    return merged;
  }

  // 渲染 diff 为 HTML（inline 方式）
  // 返回 { originalHtml, suggestedHtml }
  function renderInlineDiff(text1, text2) {
    if (text1 === text2) {
      const escaped = escapeHtml(text1).replace(/\n/g, '<br>');
      return { originalHtml: escaped, suggestedHtml: escaped };
    }

    const ops = diff(text1, text2);
    const merged = mergeOps(ops);

    let originalHtml = '';
    let suggestedHtml = '';

    merged.forEach(op => {
      const escaped = escapeHtml(op.value).replace(/\n/g, '<br>');

      switch (op.type) {
        case 'equal':
          originalHtml += escaped;
          suggestedHtml += escaped;
          break;
        case 'delete':
          originalHtml += `<del class="diff-del">${escaped}</del>`;
          break;
        case 'insert':
          suggestedHtml += `<ins class="diff-ins">${escaped}</ins>`;
          break;
      }
    });

    return { originalHtml, suggestedHtml };
  }

  // 渲染 diff 摘要（只显示变更位置附近的上下文，适合组预览）
  // contextLen: 变更两侧保留的字符数
  // 返回 { originalHtml, suggestedHtml }
  function renderDiffPreview(text1, text2, contextLen) {
    if (contextLen === undefined) contextLen = 20;
    if (text1 === text2) {
      const escaped = escapeHtml(text1).replace(/\n/g, '<br>');
      return { originalHtml: escaped, suggestedHtml: escaped };
    }

    const ops = diff(text1, text2);

    // 找到第一个和最后一个变更的位置（非 equal）
    let firstChangeIdx = -1;
    let lastChangeIdx = -1;
    for (let k = 0; k < ops.length; k++) {
      if (ops[k].type !== 'equal') {
        if (firstChangeIdx === -1) firstChangeIdx = k;
        lastChangeIdx = k;
      }
    }

    if (firstChangeIdx === -1) {
      // 没有变更（理论上不会到这里，因为 text1 !== text2）
      const escaped = escapeHtml(text1).replace(/\n/g, '<br>');
      return { originalHtml: escaped, suggestedHtml: escaped };
    }

    // 计算上下文范围：从 firstChange 往前，lastChange 往后
    // 统计字符数而不是 token 数，因为 token 长度不一
    // 构建原文和译文的片段
    let prefixOrig = '', middleOrig = '', suffixOrig = '';
    let prefixSugg = '', middleSugg = '', suffixSugg = '';
    let beforeLen = 0, afterLen = 0;
    let capturingBefore = true;

    for (let k = 0; k < ops.length; k++) {
      const op = ops[k];
      if (k < firstChangeIdx) {
        // 变更前的上下文
        const v = op.value;
        if (capturingBefore) {
          if (beforeLen + v.length <= contextLen) {
            if (op.type === 'equal') {
              prefixOrig += v;
              prefixSugg += v;
              beforeLen += v.length;
            } else {
              // 如果 equal 之前就有变更，也捕获
              prefixOrig += v;
              prefixSugg += v;
              beforeLen += v.length;
            }
          } else {
            // 超过 contextLen，截取尾部
            const needed = contextLen - beforeLen;
            if (needed > 0) {
              const tail = v.slice(-needed);
              prefixOrig += tail;
              prefixSugg += tail;
              beforeLen += tail.length;
            }
            capturingBefore = false;
          }
        }
      } else if (k > lastChangeIdx) {
        // 变更后的上下文
        const v = op.value;
        if (afterLen + v.length <= contextLen) {
          if (op.type === 'equal') {
            suffixOrig += v;
            suffixSugg += v;
            afterLen += v.length;
          } else {
            suffixOrig += v;
            suffixSugg += v;
            afterLen += v.length;
          }
        } else {
          const needed = contextLen - afterLen;
          if (needed > 0) {
            const head = v.slice(0, needed);
            suffixOrig += head;
            suffixSugg += head;
            afterLen += head.length;
          }
        }
      } else {
        // 变更区域
        const escaped = escapeHtml(op.value).replace(/\n/g, '<br>');
        switch (op.type) {
          case 'equal':
            middleOrig += escaped;
            middleSugg += escaped;
            break;
          case 'delete':
            middleOrig += `<del class="diff-del">${escaped}</del>`;
            break;
          case 'insert':
            middleSugg += `<ins class="diff-ins">${escaped}</ins>`;
            break;
        }
      }
    }

    // 添加省略号
    const hasPrefix = prefixOrig.length > 0 || prefixSugg.length > 0;
    const hasSuffix = suffixOrig.length > 0 || suffixSugg.length > 0;

    let originalHtml = '';
    let suggestedHtml = '';

    if (hasPrefix) {
      originalHtml += '<span class="diff-trunc">…</span>';
      suggestedHtml += '<span class="diff-trunc">…</span>';
    }
    originalHtml += escapeHtml(prefixOrig).replace(/\n/g, '<br>');
    suggestedHtml += escapeHtml(prefixSugg).replace(/\n/g, '<br>');
    originalHtml += middleOrig;
    suggestedHtml += middleSugg;
    originalHtml += escapeHtml(suffixOrig).replace(/\n/g, '<br>');
    suggestedHtml += escapeHtml(suffixSugg).replace(/\n/g, '<br>');
    if (hasSuffix) {
      originalHtml += '<span class="diff-trunc">…</span>';
      suggestedHtml += '<span class="diff-trunc">…</span>';
    }

    return { originalHtml, suggestedHtml };
  }

  // 将不可见字符转为可视化表示（用于 diff 展示）
  // 先 escape 再替换，避免 HTML 标签被二次转义
  function visualizeWhitespace(escapedText) {
    if (!escapedText) return '';
    return escapedText
      .replace(/\n/g, '<span class="ws-char">↵</span>')
      .replace(/\r/g, '<span class="ws-char">⏎</span>')
      .replace(/\t/g, '<span class="ws-char">→</span>');
  }

  // 转义并可视化（先转义文本，再替换不可见字符）
  function escAndVisualize(text) {
    return visualizeWhitespace(escapeHtml(text));
  }

  // 渲染逐字符对齐的 diff（用于审核页面展示）
  // 双向空格填充：哪边短就在哪边填充空格，保证字符位置对齐
  // 空格只用于展示，实际保存时不会包含这些填充空格
  // @param {string} lineText - 当前行的完整文本
  // @param {Object} item - 审核项 { original, suggested, position }
  // @returns {Object} { originalHtml, suggestedHtml }
  function renderAlignedDiff(lineText, item) {
    var origText = item.original || item.originalText || '';   // 被修改的原文片段
    var suggText = item.suggested || item.suggestedText || ''; // 替换后的建议片段

    // 在当前行中查找原文的实际位置（position 可能是旧版本的偏移量）
    var pos = lineText.indexOf(origText);
    if (pos < 0) {
      // 找不到原文，直接返回原文行
      return {
        originalHtml: escAndVisualize(lineText),
        suggestedHtml: escAndVisualize(lineText)
      };
    }

    // 计算前后缀
    var prefix = lineText.substring(0, pos);
    var origSuffix = lineText.substring(pos + origText.length);

    // 双向对齐：计算需要填充的空格数
    var origLen = origText.length;
    var suggLen = suggText.length;
    var diff = origLen - suggLen;

    var origDisplay = origText;
    var suggDisplay = suggText;

    if (diff > 0) {
      // 建议比原文短，在建议后填充空格
      suggDisplay = suggText + ' '.repeat(diff);
    } else if (diff < 0) {
      // 原文比建议短，在原文后填充空格
      origDisplay = origText + ' '.repeat(-diff);
    }

    return {
      originalHtml: escAndVisualize(prefix) + '<del class="diff-del">' + escAndVisualize(origDisplay) + '</del>' + escAndVisualize(origSuffix),
      suggestedHtml: escAndVisualize(prefix) + '<ins class="diff-ins">' + escAndVisualize(suggDisplay) + '</ins>' + escAndVisualize(origSuffix)
    };
  }

  // 公共 API
  return {
    tokenize,
    lcs,
    diff,
    renderInlineDiff,
    renderDiffPreview,
    renderAlignedDiff
  };
})();

// 导出到全局作用域
if (typeof module !== 'undefined' && module.exports) {
  module.exports = DiffUtils;
}
