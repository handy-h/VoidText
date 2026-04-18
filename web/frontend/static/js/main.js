// 全局变量
let currentProcessId = null;
let currentFileId = null;

// 页面加载完成后执行
document.addEventListener('DOMContentLoaded', function() {
    // 绑定上传表单提交事件
    document.getElementById('upload-form').addEventListener('submit', function(e) {
        e.preventDefault();
        uploadFile();
    });

    // 绑定审核操作按钮事件
    document.getElementById('approve-all').addEventListener('click', function() {
        approveAllSuggestions();
    });

    document.getElementById('reject-all').addEventListener('click', function() {
        rejectAllSuggestions();
    });

    document.getElementById('save-progress').addEventListener('click', function() {
        saveProgress();
    });

    // 绑定规则表单提交事件
    document.getElementById('rule-form').addEventListener('submit', function(e) {
        e.preventDefault();
        addRule();
    });

    // 绑定外部API配置表单提交事件
    document.getElementById('external-config-form').addEventListener('submit', function(e) {
        e.preventDefault();
        updateExternalConfig();
    });

    // 加载规则列表
    loadRules();

    // 加载外部API配置
    loadExternalConfig();
});

// 上传文件
function uploadFile() {
    const fileInput = document.getElementById('file-input');
    const file = fileInput.files[0];

    if (!file) {
        alert('请选择一个文件');
        return;
    }

    if (file.size > 100 * 1024 * 1024) { // 100MB
        alert('文件大小不能超过100MB');
        return;
    }

    const formData = new FormData();
    formData.append('file', file);

    // 显示状态部分
    document.getElementById('upload-section').style.display = 'none';
    document.getElementById('status-section').style.display = 'block';
    document.getElementById('status-message').textContent = '正在上传文件...';

    fetch('/api/files/upload', {
        method: 'POST',
        body: formData
    }) .then(response => response.json())
    .then(data => {
        if (data.success) {
            currentFileId = data.fileId;
            startProcessing(data.fileId);
        } else {
            document.getElementById('status-message').textContent = '上传失败: ' + data.message;
        }
    })
    .catch(error => {
        document.getElementById('status-message').textContent = '上传失败: ' + error.message;
    });
}

// 开始处理文件
function startProcessing(fileId) {
    document.getElementById('status-message').textContent = '正在处理文件...';

    fetch('/api/process', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ fileId: fileId })
    }) .then(response => response.json())
    .then(data => {
        if (data.success) {
            currentProcessId = data.processId;
            checkProcessStatus();
        } else {
            document.getElementById('status-message').textContent = '处理失败: ' + data.message;
        }
    })
    .catch(error => {
        document.getElementById('status-message').textContent = '处理失败: ' + error.message;
    });
}

// 检查处理状态
function checkProcessStatus() {
    if (!currentProcessId) return;

    fetch(`/api/process/${currentProcessId}/status`)
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            document.getElementById('status-message').textContent = data.status;
            document.getElementById('progress-fill').style.width = data.progress + '%';

            if (data.status === 'completed') {
                setTimeout(() => {
                    document.getElementById('status-section').style.display = 'none';
                    document.getElementById('review-section').style.display = 'block';
                    document.getElementById('versions-section').style.display = 'block';
                    loadSuggestions();
                    loadVersions();
                }, 1000);
            } else if (data.status === 'failed') {
                document.getElementById('status-message').textContent = '处理失败: ' + data.message;
            } else {
                // 继续检查状态
                setTimeout(checkProcessStatus, 2000);
            }
        }
    })
    .catch(error => {
        document.getElementById('status-message').textContent = '检查状态失败: ' + error.message;
    });
}

// 加载修改建议
function loadSuggestions() {
    if (!currentProcessId) return;

    fetch(`/api/process/${currentProcessId}/suggestions`)
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            const suggestionsList = document.getElementById('suggestions-list');
            suggestionsList.innerHTML = '';

            if (data.suggestions.length === 0) {
                suggestionsList.innerHTML = '<p>没有发现需要修改的内容</p>';
                return;
            }

            data.suggestions.forEach(suggestion => {
                const suggestionItem = document.createElement('div');
                suggestionItem.className = 'suggestion-item';
                suggestionItem.innerHTML = `
                    <div class="original">
                        <strong>原文:</strong> ${suggestion.original}
                    </div>
                    <div class="suggested">
                        <strong>建议:</strong> ${suggestion.suggested}
                    </div>
                    <div class="suggestion-actions">
                        <button onclick="approveSuggestion('${suggestion.id}')">通过</button>
                        <button onclick="rejectSuggestion('${suggestion.id}')">拒绝</button>
                    </div>
                `;
                suggestionsList.appendChild(suggestionItem);
            });
        }
    })
    .catch(error => {
        console.error('加载建议失败:', error);
    });
}

// 批准单个建议
function approveSuggestion(suggestionId) {
    fetch(`/api/process/${currentProcessId}/approve`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ suggestionId: suggestionId })
    }) .then(response => response.json())
    .then(data => {
        if (data.success) {
            loadSuggestions();
        } else {
            alert('操作失败: ' + data.message);
        }
    })
    .catch(error => {
        alert('操作失败: ' + error.message);
    });
}

// 拒绝单个建议
function rejectSuggestion(suggestionId) {
    fetch(`/api/process/${currentProcessId}/reject`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ suggestionId: suggestionId })
    }) .then(response => response.json())
    .then(data => {
        if (data.success) {
            loadSuggestions();
        } else {
            alert('操作失败: ' + data.message);
        }
    })
    .catch(error => {
        alert('操作失败: ' + error.message);
    });
}

// 批准所有建议
function approveAllSuggestions() {
    if (confirm('确定要批准所有建议吗？')) {
        // 这里可以实现批量批准逻辑
        alert('功能开发中');
    }
}

// 拒绝所有建议
function rejectAllSuggestions() {
    if (confirm('确定要拒绝所有建议吗？')) {
        // 这里可以实现批量拒绝逻辑
        alert('功能开发中');
    }
}

// 保存进度
function saveProgress() {
    // 这里可以实现保存进度逻辑
    alert('进度已保存');
    loadVersions();
}

// 加载版本列表
function loadVersions() {
    if (!currentFileId) return;

    fetch(`/api/files/${currentFileId}/versions`)
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            const versionsList = document.getElementById('versions-list');
            versionsList.innerHTML = '';

            if (data.versions.length === 0) {
                versionsList.innerHTML = '<p>暂无版本记录</p>';
                return;
            }

            data.versions.forEach(version => {
                const versionItem = document.createElement('div');
                versionItem.className = 'version-item';
                versionItem.innerHTML = `
                    <div class="version-info">
                        <strong>版本 ${version.version}:</strong> ${version.timestamp}
                    </div>
                    <div class="version-actions">
                        <button onclick="restoreVersion('${version.version}')">恢复</button>
                        <button onclick="deleteVersion('${version.version}')">删除</button>
                    </div>
                `;
                versionsList.appendChild(versionItem);
            });
        }
    })
    .catch(error => {
        console.error('加载版本失败:', error);
    });
}

// 恢复版本
function restoreVersion(version) {
    if (confirm('确定要恢复到这个版本吗？')) {
        fetch(`/api/files/${currentFileId}/versions/${version}/restore`, {
            method: 'POST'
        }) .then(response => response.json())
        .then(data => {
            if (data.success) {
                alert('版本恢复成功');
                loadVersions();
            } else {
                alert('恢复失败: ' + data.message);
            }
        })
        .catch(error => {
            alert('恢复失败: ' + error.message);
        });
    }
}

// 删除版本
function deleteVersion(version) {
    if (confirm('确定要删除这个版本吗？')) {
        fetch(`/api/files/${currentFileId}/versions/${version}`, {
            method: 'DELETE'
        }) .then(response => response.json())
        .then(data => {
            if (data.success) {
                alert('版本删除成功');
                loadVersions();
            } else {
                alert('删除失败: ' + data.message);
            }
        })
        .catch(error => {
            alert('删除失败: ' + error.message);
        });
    }
}

// 添加规则
function addRule() {
    const name = document.getElementById('rule-name').value;
    const pattern = document.getElementById('rule-pattern').value;
    const replacement = document.getElementById('rule-replacement').value;

    if (!name || !pattern) {
        alert('请填写规则名称和正则表达式');
        return;
    }

    fetch('/api/rules', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ name, pattern, replacement })
    }) .then(response => response.json())
    .then(data => {
        if (data.success) {
            alert('规则添加成功');
            document.getElementById('rule-name').value = '';
            document.getElementById('rule-pattern').value = '';
            document.getElementById('rule-replacement').value = '';
            loadRules();
        } else {
            alert('添加失败: ' + data.message);
        }
    })
    .catch(error => {
        alert('添加失败: ' + error.message);
    });
}

// 加载规则列表
function loadRules() {
    fetch('/api/rules')
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            const rulesList = document.getElementById('rules-list');
            rulesList.innerHTML = '';

            if (data.rules.length === 0) {
                rulesList.innerHTML = '<p>暂无自定义规则</p>';
                return;
            }

            data.rules.forEach(rule => {
                const ruleItem = document.createElement('div');
                ruleItem.className = 'rule-item';
                ruleItem.innerHTML = `
                    <div class="rule-info">
                        <strong>${rule.name}:</strong> ${rule.pattern} → ${rule.replacement}
                    </div>
                    <div class="rule-actions">
                        <button onclick="deleteRule('${rule.id}')">删除</button>
                    </div>
                `;
                rulesList.appendChild(ruleItem);
            });
        }
    })
    .catch(error => {
        console.error('加载规则失败:', error);
    });
}

// 删除规则
function deleteRule(ruleId) {
    if (confirm('确定要删除这个规则吗？')) {
        fetch(`/api/rules/${ruleId}`, {
            method: 'DELETE'
        }) .then(response => response.json())
        .then(data => {
            if (data.success) {
                alert('规则删除成功');
                loadRules();
            } else {
                alert('删除失败: ' + data.message);
            }
        })
        .catch(error => {
            alert('删除失败: ' + error.message);
        });
    }
}

// 加载外部API配置
function loadExternalConfig() {
    fetch('/api/config/external')
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            document.getElementById('api-url').value = data.config.url || '';
            document.getElementById('api-key').value = data.config.key || '';
        }
    })
    .catch(error => {
        console.error('加载配置失败:', error);
    });
}

// 更新外部API配置
function updateExternalConfig() {
    const url = document.getElementById('api-url').value;
    const key = document.getElementById('api-key').value;

    fetch('/api/config/external', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ url, key })
    }) .then(response => response.json())
    .then(data => {
        if (data.success) {
            alert('配置保存成功');
        } else {
            alert('保存失败: ' + data.message);
        }
    })
    .catch(error => {
        alert('保存失败: ' + error.message);
    });
}