// uniteOS Dashboard — Client Application
// Connects to the local REST API and renders the dashboard UI

const API_BASE = '';  // Same origin

// ─── State ────────────────────────────────────────────────────
let currentPage = 'dashboard';
let refreshTimer = null;

// ─── API Client ───────────────────────────────────────────────
async function api(path, opts = {}) {
    try {
        const res = await fetch(`${API_BASE}/api${path}`, {
            headers: { 'Content-Type': 'application/json' },
            ...opts,
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return await res.json();
    } catch (err) {
        console.error(`API error: ${path}`, err);
        return null;
    }
}

// ─── Navigation ───────────────────────────────────────────────
document.querySelectorAll('.nav-item[data-page]').forEach(btn => {
    btn.addEventListener('click', () => navigateTo(btn.dataset.page));
});

function navigateTo(page) {
    currentPage = page;
    document.querySelectorAll('.nav-item').forEach(b => b.classList.remove('active'));
    document.querySelector(`[data-page="${page}"]`)?.classList.add('active');
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.getElementById(`page-${page}`)?.classList.add('active');

    const titles = {
        dashboard: ['Dashboard', 'System overview'],
        files: ['Files', 'Tracked files and versions'],
        sync: ['Sync Status', 'Multi-device synchronization'],
        snapshots: ['Snapshots', 'Point-in-time backups'],
        security: ['Security', 'Encryption and integrity'],
        events: ['Event Log', 'System activity'],
        settings: ['Settings', 'Configuration'],
    };
    const [title, subtitle] = titles[page] || [page, ''];
    document.getElementById('pageTitle').textContent = title;
    document.getElementById('pageSubtitle').textContent = subtitle;

    loadPageData(page);
}

// ─── Data Loading ─────────────────────────────────────────────
async function loadPageData(page) {
    switch (page) {
        case 'dashboard': await loadDashboard(); break;
        case 'files': await loadFiles(); break;
        case 'sync': await loadSync(); break;
        case 'snapshots': await loadSnapshots(); break;
        case 'events': await loadEvents(); break;
        case 'settings': await loadSettings(); break;
    }
}

async function loadDashboard() {
    const [stats, info, status, events] = await Promise.all([
        api('/stats'), api('/info'), api('/status'), api('/events'),
    ]);

    if (stats) {
        document.getElementById('statFiles').textContent = stats.tracked_files ?? 0;
        document.getElementById('statSize').textContent = formatSize(stats.total_size_bytes || 0);
        document.getElementById('statVersions').textContent = stats.versions ?? 0;
        document.getElementById('statSnapshots').textContent = stats.snapshots ?? 0;
    }

    if (info) {
        document.getElementById('version').textContent = `v${info.version || '0.1.0'}`;
        document.getElementById('deviceName').textContent = info.device_name || 'Unknown';
        document.getElementById('deviceId').textContent = (info.device_id || '').substring(0, 16) + '...';
    }

    // File changes
    if (status?.files) {
        const changesEl = document.getElementById('fileChanges');
        const changed = status.files.filter(f => f.status !== 'unchanged');
        document.getElementById('changesCount').textContent = changed.length;

        if (changed.length === 0) {
            changesEl.innerHTML = `<div class="empty-state small">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
                <p>All files are up to date</p>
            </div>`;
        } else {
            changesEl.innerHTML = changed.map(f => `
                <div class="event-item">
                    <span class="status-dot ${f.status === 'modified' ? 'modified' : 'deleted'}"></span>
                    <div class="event-details">
                        <span class="event-source">${escapeHtml(f.path)}</span>
                        <span class="event-data">${f.status} · ${formatSize(f.size)}</span>
                    </div>
                </div>
            `).join('');
        }
    }

    // Recent events
    if (events) {
        renderEvents(document.getElementById('recentEvents'), (events || []).slice(-8).reverse());
    }

    // System health
    const healthSync = document.getElementById('healthSync');
    const healthApi = document.getElementById('healthApi');
    healthSync.className = 'health-dot health-ok';
    document.getElementById('healthSyncStatus').textContent = 'Ready';
    healthApi.className = 'health-dot health-ok';
    document.getElementById('healthApiStatus').textContent = 'Online';
}

async function loadFiles() {
    const status = await api('/status');
    const tbody = document.getElementById('fileTableBody');

    if (!status?.files || status.files.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" style="text-align:center;padding:40px;color:var(--text-muted)">No tracked files</td></tr>`;
        return;
    }

    tbody.innerHTML = status.files.map(f => {
        const statusClass = f.status === 'unchanged' ? 'ok' : f.status === 'modified' ? 'modified' : 'deleted';
        const hash = (f.old_hash || '').substring(0, 16);
        const time = f.mod_time ? new Date(f.mod_time).toLocaleString() : '—';
        const safePath = escapeHtml(f.path).replace(/\\/g, '\\\\');
        return `<tr>
            <td><span class="status-dot ${statusClass}"></span>${f.status}</td>
            <td>
                <a href="#" onclick="openFile('${safePath}')" title="Click to open natively">${escapeHtml(f.path)}</a>
            </td>
            <td>${formatSize(f.size)}</td>
            <td class="hash">${hash}</td>
            <td style="color:var(--text-muted);font-size:12px">${time}</td>
            <td>
                <button class="btn btn-sm" onclick="deleteFile('${safePath}')" style="color:#ff4444; border: 1px solid #ff4444; padding: 2px 8px;">Delete</button>
            </td>
        </tr>`;
    }).join('');
}

async function loadSync() {
    const health = await api('/health');
    document.getElementById('syncStatus').textContent = health ? 'Connected' : 'Offline';
    document.getElementById('peerCount').textContent = health?.peers || '0';
    document.getElementById('conflictCount').textContent = health?.conflicts || '0';
    if (health?.lan_ip && health?.api_port) {
        document.getElementById('androidAccessUrl').textContent = `${health.lan_ip}:${health.api_port}`;
    } else {
        document.getElementById('androidAccessUrl').textContent = 'Unable to determine IP';
    }

    // Render connected devices dynamically
    const peerListContainer = document.getElementById('peerList');
    if (!health?.peer_list || health.peer_list.length === 0) {
        peerListContainer.innerHTML = `<div class="empty-state">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>
            <p>No peers connected</p>
            <span class="text-muted">Run uniteOS mobile app on another device</span>
        </div>`;
    } else {
        peerListContainer.innerHTML = health.peer_list.map(p => `
            <div class="health-item" style="padding: 15px 0;">
                <div class="health-dot health-ok"></div>
                <div style="display: flex; flex-direction: column;">
                    <span style="font-weight: 600;">${escapeHtml(p.device_name || p.device_id)}</span>
                    <span class="text-muted" style="font-family: var(--font-mono);">${escapeHtml(p.address)}</span>
                </div>
                <div class="health-status">Online</div>
            </div>
        `).join('');
    }
}

async function loadSnapshots() {
    const snapshots = await api('/snapshots');
    const container = document.getElementById('snapshotList');

    if (!snapshots || snapshots.length === 0) {
        container.innerHTML = `<div class="empty-state">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>
            <p>No snapshots yet</p>
            <span class="text-muted">Click "New Snapshot" to create one</span>
        </div>`;
        return;
    }

    container.innerHTML = snapshots.map(s => `
        <div class="snapshot-item">
            <div class="snapshot-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>
            </div>
            <div class="snapshot-info">
                <div class="snapshot-name">${escapeHtml(s.name || s.snapshot_id)}</div>
                <div class="snapshot-meta">${s.file_count} files · ${formatSize(s.total_size)} · ${new Date(s.created_at).toLocaleString()}</div>
            </div>
            <div class="snapshot-actions">
                <button class="btn btn-sm btn-ghost" onclick="restoreSnapshot('${s.snapshot_id}')">Restore</button>
            </div>
        </div>
    `).join('');
}

async function loadEvents() {
    const events = await api('/events');
    const container = document.getElementById('eventLog');
    document.getElementById('eventCount').textContent = events?.length || 0;
    renderEvents(container, (events || []).reverse());
}

async function loadSettings() {
    const info = await api('/info');
    const stats = await api('/stats');
    const display = document.getElementById('configDisplay');
    
    let androidMsg = '';
    if (info && info.port) {
        // Try to construct Android access URL (Assuming 10.25.x.x or standard local IPs)
        // Usually, the hostname can resolve locally, or we just instruct them to use their PC's IP
        androidMsg = `\n// Android / Web Access URL:\n// Open your phone browser and navigate to:\n// http://<YOUR-PC-IP-ADDRESS>:${info.port}\n\n`;
    }
    
    display.textContent = androidMsg + JSON.stringify({ system: info, storage: stats }, null, 2);
}

// ─── Event Rendering ──────────────────────────────────────────
function renderEvents(container, events) {
    if (!events || events.length === 0) {
        container.innerHTML = `<div class="empty-state small"><p>No events recorded</p></div>`;
        return;
    }

    container.innerHTML = events.map(e => {
        const category = getEventCategory(e.type);
        const time = e.timestamp ? new Date(e.timestamp).toLocaleTimeString() : '';
        const dataStr = e.data ? Object.entries(e.data).map(([k, v]) => `${k}: ${v}`).join(' · ') : '';
        return `<div class="event-item">
            <span class="event-type ${category}">${e.type}</span>
            <div class="event-details">
                <span class="event-source">${e.source || ''}</span>
                <span class="event-data">${escapeHtml(dataStr)}</span>
            </div>
            <span class="event-time">${time}</span>
        </div>`;
    }).join('');
}

function getEventCategory(type) {
    if (!type) return 'engine';
    if (type.startsWith('file.')) return 'file';
    if (type.startsWith('sync.')) return 'sync';
    if (type.startsWith('snapshot.')) return 'snapshot';
    if (type.startsWith('device.')) return 'device';
    return 'engine';
}

// ─── Actions ──────────────────────────────────────────────────
document.getElementById('refreshBtn').addEventListener('click', () => {
    loadPageData(currentPage);
    showToast('Refreshed', 'info');
});

// Snapshot modal
document.getElementById('btnCreateSnapshot')?.addEventListener('click', () => {
    document.getElementById('snapshotModal').classList.remove('hidden');
    document.getElementById('snapshotName').focus();
});
document.getElementById('closeSnapshotModal')?.addEventListener('click', () => {
    document.getElementById('snapshotModal').classList.add('hidden');
});
document.getElementById('cancelSnapshot')?.addEventListener('click', () => {
    document.getElementById('snapshotModal').classList.add('hidden');
});
document.getElementById('confirmSnapshot')?.addEventListener('click', async () => {
    const name = document.getElementById('snapshotName').value || 'Untitled Snapshot';
    const desc = document.getElementById('snapshotDesc').value || '';
    const result = await api('/snapshots', {
        method: 'POST',
        body: JSON.stringify({ name, description: desc }),
    });
    document.getElementById('snapshotModal').classList.add('hidden');
    if (result) {
        showToast(`Snapshot "${name}" created`, 'success');
        loadSnapshots();
    } else {
        showToast('Failed to create snapshot', 'error');
    }
});

// Search
document.getElementById('searchInput')?.addEventListener('keydown', async (e) => {
    if (e.key === 'Enter') {
        const q = e.target.value.trim();
        if (!q) return;
        const results = await api(`/search?q=${encodeURIComponent(q)}`);
        if (results && results.length > 0) {
            navigateTo('files');
            showToast(`Found ${results.length} file(s)`, 'info');
        } else {
            showToast('No files found', 'info');
        }
    }
});

// Integrity verification
document.getElementById('btnVerifyIntegrity')?.addEventListener('click', async () => {
    const result = await api('/integrity/verify');
    if (result) {
        const ok = result.passed || 0;
        const failed = result.failed || 0;
        if (failed > 0) {
            showToast(`Integrity check: ${failed} file(s) corrupted!`, 'error');
        } else {
            showToast(`Integrity check: ${ok} file(s) verified OK`, 'success');
        }
    } else {
        showToast('Integrity check completed', 'info');
    }
});

// Secure sharing
document.getElementById('btnGenerateShare')?.addEventListener('click', async () => {
    const path = document.getElementById('shareFilePath').value.trim();
    if (!path) { showToast('Enter a file path', 'error'); return; }
    const result = await api('/share', {
        method: 'POST',
        body: JSON.stringify({ path }),
    });
    const el = document.getElementById('shareResult');
    if (result?.token) {
        el.textContent = `Share token: ${result.token}\nExpires: ${result.expires}`;
        el.classList.remove('hidden');
    } else {
        showToast('Failed to generate share link', 'error');
    }
});

// ─── System Browser ─────────────────────────────────────────────
document.getElementById('btnBrowseSystem')?.addEventListener('click', () => {
    document.getElementById('systemBrowserModal').classList.remove('hidden');
    loadSystemPath('');
});

document.getElementById('closeSystemBrowser')?.addEventListener('click', () => {
    document.getElementById('systemBrowserModal').classList.add('hidden');
});
document.getElementById('cancelSystemBrowser')?.addEventListener('click', () => {
    document.getElementById('systemBrowserModal').classList.add('hidden');
});

async function loadSystemPath(path) {
    const result = await api(`/system/browse?path=${encodeURIComponent(path)}`);
    if (!result) {
        showToast('Failed to read system path', 'error');
        return;
    }
    
    document.getElementById('currentSystemPath').value = result.current_path;
    const list = document.getElementById('systemBrowserList');
    
    list.innerHTML = (result.entries || []).map(e => {
        const icon = e.is_dir ? 
            `<svg viewBox="0 0 24 24" fill="none" stroke="#eab308" stroke-width="2" width="18" height="18"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2v11z"/></svg>` : 
            `<svg viewBox="0 0 24 24" fill="none" stroke="#94a3b8" stroke-width="2" width="18" height="18"><path d="M13 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V9z"/><polyline points="13,2 13,9 20,9"/></svg>`;
            
        // Must escape backslashes for inline JS string literal!
        const jsPath = e.path.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
            
        return `<tr style="cursor:pointer" onclick="handleSystemEntryClick('${jsPath}', ${e.is_dir})">
            <td style="width: 30px">${icon}</td>
            <td>${escapeHtml(e.name)}</td>
        </tr>`;
    }).join('');
}

window.handleSystemEntryClick = async function(path, isDir) {
    if (isDir) {
        loadSystemPath(path);
    } else {
        // Track the file
        const res = await api('/files', {
            method: 'POST',
            body: JSON.stringify({ path: path }),
        });
        if (res) {
            showToast('File added to uniteOS', 'success');
            document.getElementById('systemBrowserModal').classList.add('hidden');
            loadFiles();
        } else {
            showToast('Failed to add file', 'error');
        }
    }
};

// Web Upload (Android/Web Interface)
document.getElementById('webUploadInput')?.addEventListener('change', async (e) => {
    const files = e.target.files;
    if (files.length === 0) return;

    for (let i = 0; i < files.length; i++) {
        const file = files[i];
        const formData = new FormData();
        formData.append('file', file);

        showToast(`Uploading ${file.name}...`, 'info');
        
        try {
            const res = await fetch(`${API_BASE}/api/upload`, {
                method: 'POST',
                body: formData
            });
            
            if (res.ok) {
                showToast(`Successfully uploaded ${file.name}`, 'success');
            } else {
                showToast(`Failed to upload ${file.name}`, 'error');
            }
        } catch (err) {
            showToast(`Error uploading ${file.name}`, 'error');
        }
    }
    
    // Clear input
    e.target.value = '';
    loadFiles();
});

async function openFile(path) {
    try {
        const res = await fetch(`${API_BASE}/api/open?path=${encodeURIComponent(path)}`);
        if (!res.ok) throw new Error('Failed to open file');
        showToast('Opening file...', 'info');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function deleteFile(path) {
    if (!confirm(`Are you sure you want to delete ${path}?`)) return;
    try {
        const res = await fetch(`${API_BASE}/api/delete?path=${encodeURIComponent(path)}`, { method: 'DELETE' });
        if (!res.ok) throw new Error('Failed to delete file');
        showToast('File deleted', 'success');
        loadFiles();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

// ─── Utilities ────────────────────────────────────────────────
function formatSize(bytes) {
    if (bytes == null || bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span>${escapeHtml(message)}</span>`;
    container.appendChild(toast);
    setTimeout(() => toast.remove(), 4000);
}

// ─── Auto Refresh ─────────────────────────────────────────────
function startAutoRefresh() {
    refreshTimer = setInterval(() => loadPageData(currentPage), 10000);
}

// ─── AI Chat ──────────────────────────────────────────────────
document.getElementById('btnSendAiMessage')?.addEventListener('click', async () => {
    const input = document.getElementById('aiChatInput');
    const msg = input.value.trim();
    if (!msg) return;
    
    const log = document.getElementById('aiChatLog');
    
    // Add user message
    log.innerHTML += `
        <div class="event-item" style="border:none; padding: 10px; background: rgba(255, 255, 255, 0.05); border-radius: 12px; align-self: flex-end; max-width: 80%;">
            <div class="event-details" style="text-align: right;">
                <span class="event-source" style="color: var(--text-primary);">You</span>
                <div class="event-data" style="margin-top: 4px; font-size: 13px;">${escapeHtml(msg)}</div>
            </div>
        </div>
    `;
    input.value = '';
    log.scrollTop = log.scrollHeight;

    try {
        const response = await fetch('/api/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ message: msg })
        });
        const data = await response.json();
        
        // Add AI message
        log.innerHTML += `
            <div class="event-item" style="border:none; padding: 10px; background: rgba(99, 102, 241, 0.1); border-radius: 12px; align-self: flex-start; max-width: 80%;">
                <div class="event-details">
                    <span class="event-source" style="color: var(--accent-indigo);">uniteOS AI</span>
                    <div class="event-data" style="margin-top: 4px; font-size: 13px;">${escapeHtml(data.reply || 'No response')}</div>
                </div>
            </div>
        `;
        log.scrollTop = log.scrollHeight;
    } catch (e) {
        showToast('Failed to connect to AI', 'error');
    }
});

document.getElementById('aiChatInput')?.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') document.getElementById('btnSendAiMessage').click();
});

// ─── Initialize ───────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    navigateTo('dashboard');   // sets title + loads data
    startAutoRefresh();
});
