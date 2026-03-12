const API_BASE_URL = "https://alas.nanoda.work"; // Keep consistent with index/script.js

let currentPage = 1;
const pageSize = 50;
let currentSort = 'rounds'; // 'rounds' or 'stamina'

// Format numbers
function formatNumber(num) {
    if (num === null || num === undefined) return "--";
    return new Intl.NumberFormat().format(num);
}

// Prevent XSS
function escapeHtml(text) {
    if (!text) return text;
    return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

// Update Profile
async function updateProfile() {
    const deviceId = document.getElementById('device-id').value.trim();
    const username = document.getElementById('username').value.trim();
    const msgEl = document.getElementById('form-message');

    if (!deviceId) {
        msgEl.innerText = "⚠️ 请填写 Device ID";
        msgEl.style.color = "#f87171";
        return;
    }
    if (!username) {
        msgEl.innerText = "⚠️ 请填写昵称";
        msgEl.style.color = "#f87171";
        return;
    }

    msgEl.innerText = "正在保存...";
    msgEl.style.color = "#fbbf24";

    try {
        const res = await fetch(`${API_BASE_URL}/api/user/profile`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_id: deviceId, username: username })
        });

        const data = await res.json();

        if (res.ok) {
            msgEl.innerText = "✅ 信息已更新！刷新排行榜中...";
            msgEl.style.color = "#4ade80";
            // Save device ID to localStorage for convenience
            localStorage.setItem('alas_device_id', deviceId);
            // Refresh list
            loadLeaderboard(currentPage);
        } else {
            msgEl.innerText = "❌ 错误: " + (data.error || "未知错误");
            msgEl.style.color = "#f87171";
        }
    } catch (e) {
        msgEl.innerText = "❌ 网络错误";
        msgEl.style.color = "#f87171";
    }
}

function switchTab(sortType) {
    if (currentSort === sortType) return;
    currentSort = sortType;
    currentPage = 1;

    // Update UI
    document.querySelector('#tab-rounds').className = sortType === 'rounds' ? 'segment-btn active' : 'segment-btn';
    document.querySelector('#tab-stamina').className = sortType === 'stamina' ? 'segment-btn active' : 'segment-btn';

    // Highlight Column Header
    document.querySelector('#col-rounds').className = sortType === 'rounds' ? 'highlight' : '';
    document.querySelector('#col-stamina').className = sortType === 'stamina' ? 'highlight' : '';

    loadLeaderboard(currentPage);
}

// Load Leaderboard
async function loadLeaderboard(page) {
    const tbody = document.getElementById('leaderboard-body');
    tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; opacity:0.7;">加载中...</td></tr>';

    try {
        const res = await fetch(`${API_BASE_URL}/api/leaderboard?page=${page}&size=${pageSize}&sort=${currentSort}`);
        const data = await res.json();

        if (!data.data || data.data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; opacity:0.5;">暂无数据</td></tr>';
            return;
        }

        let html = '';
        data.data.forEach((entry, index) => {
            const rank = (page - 1) * pageSize + index + 1;
            let rankClass = '';
            if (rank === 1) rankClass = ' rank-1';
            else if (rank === 2) rankClass = ' rank-2';
            else if (rank === 3) rankClass = ' rank-3';

            // Highlight current user if device ID matches (backend returns truncated ID)
            const myDeviceId = localStorage.getItem('alas_device_id');
            const isMe = myDeviceId && entry.device_id && myDeviceId.startsWith(entry.device_id);
            const rowClass = isMe ? 'style="background: rgba(99, 102, 241, 0.15);"' : '';

            // Format Last Active (Assume Go sends standard Time format, simplified)
            const date = new Date(entry.last_active);
            const dateStr = date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

            html += `
                <tr ${rowClass}>
                    <td><div class="rank-badge${rankClass}">#${rank}</div></td>
                    <td>
                        <div class="user-info">
                            <div class="user-avatar">${(entry.username && entry.username.length > 0) ? entry.username[0].toUpperCase() : '?'}</div>
                            <div>
                                <div class="user-name">${escapeHtml(entry.username) || '未知指挥官'}</div>
                                <div class="user-id">${entry.device_id.substring(0, 8)}...</div>
                            </div>
                        </div>
                    </td>
                    <td class="metric-cell ${currentSort === 'rounds' ? 'highlight' : ''}">${formatNumber(entry.battle_rounds)}</td>
                    <td class="metric-cell" style="color: #32d74b; font-weight: ${currentSort === 'stamina' ? '600' : '500'};">${(entry.net_stamina_gain >= 0 ? '+' : '') + formatNumber(entry.net_stamina_gain)}</td>
                    <td class="metric-cell">${formatNumber(entry.akashi_encounters)}</td>
                    <td style="color:var(--text-secondary); font-size:0.85em;">${dateStr}</td>
                    <td>
                        <div style="display:flex; gap:0.5rem; align-items: center;">
                            <a href="history.html?device_id=${entry.device_id}" class="btn btn-secondary" style="padding: 0.4rem 0.8rem; font-size: 0.8em; height: auto; text-decoration: none; border-radius: 999px; color: #0A84FF; border-color: rgba(10,132,255,0.2); background: rgba(10,132,255,0.05);">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="margin-right:0.25rem;"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg> 履历
                            </a>
                            <button class="btn btn-secondary" style="padding: 0.4rem 0.8rem; font-size: 0.8em; height: auto; border-radius: 999px; color: #ff375f; border-color: rgba(255,55,95,0.2); background: rgba(255,55,95,0.05);" onclick="reportUser('${entry.device_id}', '${escapeHtml(entry.username)}')">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="margin-right:0.25rem;"><path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"></path><line x1="4" y1="22" x2="4" y2="15"></line></svg> 举报
                            </button>
                        </div>
                    </td>
                </tr>
            `;
        });

        tbody.innerHTML = html;

        // Update Pagination Controls
        document.getElementById('page-indicator').innerText = `第 ${page} 页`;
        document.getElementById('prev-btn').disabled = page <= 1;
        // Simple next check: if we got less than pageSize, we are at end
        document.getElementById('next-btn').disabled = data.data.length < pageSize;

    } catch (e) {
        console.error(e);
        tbody.innerHTML = '<tr><td colspan="6" class="text-error" style="text-align:center;">加载失败 / Failed to load</td></tr>';
    }
}

async function reportUser(targetId, username) {
    const reason = prompt(`请输入举报 [${username}] 的原因：\nPlease enter the reason for reporting [${username}]:`);
    if (reason === null) return; // Cancelled
    if (!reason.trim()) {
        alert("原因不能为空 / Reason cannot be empty");
        return;
    }

    try {
        const token = localStorage.getItem('alas_admin_token');
        const headers = { 'Content-Type': 'application/json' };
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const res = await fetch(`${API_BASE_URL}/api/report`, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify({
                target_id: targetId,
                reason: reason
            })
        });
        const json = await res.json();

        if (res.ok) {
            alert("举报成功，我们将尽快处理。\nReport submitted successfully.");
        } else {
            alert("举报失败: " + (json.error || "Unknown error"));
        }
    } catch (e) {
        alert("网络错误 / Network Error");
    }
}

function changePage(delta) {
    const newPage = currentPage + delta;
    if (newPage < 1) return;
    currentPage = newPage;
    loadLeaderboard(currentPage);
}

// Init
document.addEventListener("DOMContentLoaded", () => {
    // Auto-fill Device ID if known
    const savedId = localStorage.getItem('alas_device_id');
    if (savedId) {
        document.getElementById('device-id').value = savedId;
    }

    // Initial Load
    loadLeaderboard(currentPage);
});
