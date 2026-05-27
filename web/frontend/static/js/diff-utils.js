// diff-utils.js — 段级 diff：左原文（高亮删除）/ 右建议（高亮插入）
const DiffUtils = (function() {
  function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function tokenize(text) {
    if (!text) return [];
    const pattern = /[一-鿿㐀-䶿豈-﫿]|[a-zA-Z]+|[0-9]+|\s+|./g;
    return text.match(pattern) || [];
  }

  function diff(text1, text2) {
    const tokens1 = tokenize(text1);
    const tokens2 = tokenize(text2);
    const n = tokens1.length, m = tokens2.length;
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
    const result = [];
    let i = n, j = m;
    while (i > 0 || j > 0) {
      if (i > 0 && j > 0 && tokens1[i - 1] === tokens2[j - 1]) {
        result.push({ type: 'equal', value: tokens1[i - 1] });
        i--; j--;
      } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
        result.push({ type: 'insert', value: tokens2[j - 1] });
        j--;
      } else if (i > 0) {
        result.push({ type: 'delete', value: tokens1[i - 1] });
        i--;
      }
    }
    return result.reverse();
  }

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

  // 左侧原文：保留 equal + delete（高亮），跳过 insert
  function diffOriginal(orig, sugg) {
    if (!sugg || orig === sugg) return escapeHtml(orig || '');
    const merged = mergeOps(diff(orig, sugg));
    let html = '';
    merged.forEach(function(op) {
      const esc = escapeHtml(op.value);
      if (op.type === 'equal') html += esc;
      else if (op.type === 'delete') html += '<del class="diff-del">' + esc + '</del>';
    });
    return html;
  }

  // 右侧建议：保留 equal + insert（高亮），跳过 delete
  function diffSuggested(orig, sugg) {
    if (!sugg) return '';
    if (orig === sugg) return escapeHtml(sugg);
    const merged = mergeOps(diff(orig, sugg));
    let html = '';
    merged.forEach(function(op) {
      const esc = escapeHtml(op.value);
      if (op.type === 'equal') html += esc;
      else if (op.type === 'insert') html += '<ins class="diff-ins">' + esc + '</ins>';
    });
    return html;
  }

  return {
    tokenize: tokenize,
    diff: diff,
    diffOriginal: diffOriginal,
    diffSuggested: diffSuggested
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = DiffUtils;
}
