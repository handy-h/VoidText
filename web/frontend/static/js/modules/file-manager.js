// 文件管理模块
const FileManager = (function() {
  let currentFileMd5 = null;
  let fileList = [];

  function init() {
    bindEvents();
    refreshFileList();
  }

  function bindEvents() {
    const fileInput = document.getElementById('file-input');
    if (fileInput) {
      fileInput.addEventListener('change', handleFileUpload);
    }

    const dropZone = document.getElementById('drop-zone-empty');
    const dropInput = document.getElementById('drop-zone-input');
    if (dropZone && dropInput) {
      dropInput.addEventListener('change', function(e) { handleFileUpload(e); });

      dropZone.addEventListener('dragover', function(e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.add('drag-over');
      });

      dropZone.addEventListener('dragleave', function(e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.remove('drag-over');
      });

      dropZone.addEventListener('drop', function(e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.remove('drag-over');
        const files = e.dataTransfer.files;
        if (files && files.length > 0) {
          const dataTransfer = new DataTransfer();
          dataTransfer.items.add(files[0]);
          dropInput.files = dataTransfer.files;
          dropInput.dispatchEvent(new Event('change'));
        }
      });
    }

    const navButtons = document.querySelectorAll('.nav-btn');
    navButtons.forEach(btn => {
      btn.addEventListener('click', function() {
        const section = this.getAttribute('data-section');
        if (section) showSection(section);
      });
    });
  }

  function showSection(section) {
    var appEl = document.getElementById('app');
    if (appEl) appEl.classList.toggle('review-fullwidth', section === 'review');

    const sections = ['file-list', 'upload', 'rules-config', 'processing', 'review', 'completed'];
    sections.forEach((s) => {
      const el = document.getElementById(s + '-section');
      if (el) el.style.display = s === section ? 'block' : 'none';
    });

    document.querySelectorAll('.nav-btn').forEach((btn) => btn.classList.remove('active'));
    const navMap = { 'file-list': 0, upload: 1 };
    if (navMap[section] !== undefined) {
      const navBtns = document.querySelectorAll('.nav-btn');
      if (navBtns[navMap[section]]) navBtns[navMap[section]].classList.add('active');
    }

    if (section === 'file-list') {
      refreshFileList();
      stopPolling();
    }
  }

  function refreshFileList() {
    AppConfig.apiRequest('/files')
      .then((data) => {
        if (!data.success) return;

        const container = document.getElementById('file-list-container');
        const dropZone = document.getElementById('drop-zone-empty');
        const countBadge = document.getElementById('file-count-badge');
        if (!container) return;

        if (!data.files || data.files.length === 0) {
          container.innerHTML = '';
          if (dropZone) dropZone.style.display = 'flex';
          if (countBadge) countBadge.style.display = 'none';
          return;
        }

        if (dropZone) dropZone.style.display = 'none';
        if (countBadge) {
          countBadge.style.display = 'inline-flex';
          DomUtils.setTextContent(countBadge, data.files.length + ' 个文件');
        }

        container.innerHTML = '';

        // DocumentFragment 批量插入，避免多次 reflow
        const fragment = document.createDocumentFragment();
        data.files.forEach(file => fragment.appendChild(DomUtils.createFileCard(file)));
        container.appendChild(fragment);

        fileList = data.files;
      })
      .catch((err) => DomUtils.showFeedback('加载文件列表失败: ' + err.message, 'error'));
  }

  function handleFileUpload(event) {
    const file = event.target.files[0];
    if (!file) return;

    const resultDiv = document.getElementById('upload-result');
    if (!resultDiv) return;

    resultDiv.style.display = 'block';
    resultDiv.innerHTML = '';

    const loadingDiv = DomUtils.createElement('div', { className: 'upload-loading' });
    DomUtils.setTextContent(loadingDiv, '正在上传...');
    resultDiv.appendChild(loadingDiv);

    AppConfig.uploadFile(file, (progress) => {
      DomUtils.setTextContent(loadingDiv, '正在上传... ' + progress + '%');
    })
      .then((data) => {
        resultDiv.innerHTML = '';
        resultDiv.appendChild(DomUtils.createUploadResult(data));

        if (data.success) {
          refreshFileList();
          if (data.md5) {
            RulesConfigModule.configureRules(data.md5);
          } else {
            showSection('file-list');
          }
          DomUtils.showFeedback('文件 "' + file.name + '" 上传成功！', 'success');
        }
      })
      .catch((err) => {
        resultDiv.innerHTML = '';
        const errorDiv = DomUtils.createElement('div', { className: 'upload-error' });
        const errorMsg = DomUtils.createElement('p');
        DomUtils.setTextContent(errorMsg, '上传失败: ' + err.message);
        errorDiv.appendChild(errorMsg);

        const actions = DomUtils.createElement('div', { className: 'upload-actions' });
        const closeBtn = DomUtils.createElement('button', {
          className: 'btn-secondary',
          onclick: () => { resultDiv.style.display = 'none'; }
        });
        DomUtils.setTextContent(closeBtn, '关闭');
        actions.appendChild(closeBtn);
        errorDiv.appendChild(actions);
        resultDiv.appendChild(errorDiv);
      });

    event.target.value = '';
  }

  function deleteFile(md5, fileName) {
    if (!confirm('确定删除文件"' + (fileName || md5) + '"的所有处理记录？\n\n此操作将删除该文件的所有审核记录、版本历史和处理日志，且不可恢复！')) {
      return;
    }
    if (!confirm('二次确认：真的要删除吗？此操作不可撤销！')) return;

    AppConfig.apiRequest(`/files/${md5}`, { method: 'DELETE' })
      .then((data) => {
        if (data.success) {
          DomUtils.showFeedback('文件已删除');
          refreshFileList();
        } else {
          DomUtils.showFeedback(data.message, 'error');
        }
      })
      .catch((err) => DomUtils.showFeedback('删除失败: ' + err.message, 'error'));
  }

  function reprocessFile(md5) {
    if (!confirm('确定重新处理该文件？之前的处理结果将被清除。')) return;

    AppConfig.apiRequest(`/files/${md5}/resume`, { method: 'POST' })
      .then((data) => {
        if (data.success) {
          currentFileMd5 = md5;
          RulesConfigModule.configureRules(md5);
        } else {
          DomUtils.showFeedback(data.message, 'error');
        }
      })
      .catch((err) => DomUtils.showFeedback('重新处理失败: ' + err.message, 'error'));
  }

  function resumeFile(md5) {
    AppConfig.apiRequest(`/files/${md5}/resume`, { method: 'POST' })
      .then((data) => {
        if (data.success) {
          currentFileMd5 = md5;
          AppConfig.apiRequest(`/files/${md5}/run`, { method: 'POST' })
            .then((data2) => {
              if (data2.success) {
                showSection('processing');
                startPolling();
              } else {
                DomUtils.showFeedback(data2.message, 'error');
              }
            })
            .catch((err) => DomUtils.showFeedback('启动处理失败: ' + err.message, 'error'));
        } else {
          DomUtils.showFeedback(data.message, 'error');
        }
      })
      .catch((err) => DomUtils.showFeedback('恢复文件失败: ' + err.message, 'error'));
  }

  function downloadFile(md5) {
    const url = `/api/files/${md5}/download`;

    fetch(url)
      .then(response => {
        if (!response.ok) throw new Error('下载失败: ' + response.status);
        const disposition = response.headers.get('Content-Disposition');
        let filename = null;
        if (disposition) {
          var matchUtf = disposition.match(/filename\*=UTF-8''([^;\s]+)/);
          if (matchUtf && matchUtf[1]) {
            filename = decodeURIComponent(matchUtf[1]);
          } else {
            var matchAscii = disposition.match(/filename="([^"]*)"/);
            if (matchAscii && matchAscii[1]) filename = matchAscii[1];
          }
        }
        return response.blob().then(blob => ({ blob, filename }));
      })
      .then(({ blob, filename }) => {
        var objUrl = window.URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = objUrl;
        a.download = filename || ('file_' + md5 + '.txt');
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(objUrl);
        document.body.removeChild(a);
      })
      .catch(err => DomUtils.showFeedback(err.message, 'error'));
  }

  function viewReport(md5) {
    fetch(`/api/files/${md5}/report?format=html`)
      .then(response => {
        if (!response.ok) throw new Error('获取报告失败: ' + response.status);
        return response.blob();
      })
      .then(blob => {
        const objUrl = window.URL.createObjectURL(blob);
        const win = window.open(objUrl, '_blank');
        // 等新窗口加载完再释放，防止过早撤销导致空白页
        if (win) {
          win.addEventListener('load', () => window.URL.revokeObjectURL(objUrl));
        }
      })
      .catch(err => DomUtils.showFeedback(err.message, 'error'));
  }

  return {
    init,
    showSection,
    refreshFileList,
    handleFileUpload,
    deleteFile,
    reprocessFile,
    resumeFile,
    downloadFile,
    viewReport,
    getCurrentFileMd5: () => currentFileMd5,
    setCurrentFileMd5: (md5) => { currentFileMd5 = md5; },
    getFileList: () => fileList
  };
})();

window.FileManager = FileManager;
