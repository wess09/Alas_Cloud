const API_BASE_URL = "https://alas.nanoda.work"; // Keep consistent with index/script.js

let currentPage = 1;
const pageSize = 50;
let currentSort = 'rounds'; // 'rounds' or 'stamina'

// Format numbers
function formatNumber(num) {
    if (num === null || num === undefined) return "--";
    return new Intl.NumberFormat().format(num);
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
    document.querySelector('#tab-rounds').className = sortType === 'rounds' ? 'btn' : 'btn btn-secondary';
    document.querySelector('#tab-stamina').className = sortType === 'stamina' ? 'btn' : 'btn btn-secondary';

    // Highlight Column Header
    document.querySelector('#col-rounds').className = sortType === 'rounds' ? 'highlight' : '';
    document.querySelector('#col-stamina').className = sortType === 'stamina' ? 'text-success highlight' : '';

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
            // Highlight current user if device ID matches (backend returns truncated ID)
            const myDeviceId = localStorage.getItem('alas_device_id');
            const isMe = myDeviceId && entry.device_id && myDeviceId.startsWith(entry.device_id);
            const rowClass = isMe ? 'style="background: rgba(99, 102, 241, 0.15);"' : '';

            // Format Last Active (Assume Go sends standard Time format, simplified)
            const date = new Date(entry.last_active);
            const dateStr = date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

            html += `
                <tr ${rowClass}>
                    <td class="rank-cell">#${rank}</td>
                    <td>
                        <span style="font-weight:600;">${entry.username || '未知指挥官'}</span>
                        <br><span class="code" style="font-size:0.7em; opacity:0.6;">${entry.device_id.substring(0, 8)}...</span>
                    </td>
                    <td class="${currentSort === 'rounds' ? 'highlight' : ''}">${formatNumber(entry.battle_rounds)}</td>
                    <td class="text-success ${currentSort === 'stamina' ? 'highlight' : ''}">${(entry.net_stamina_gain >= 0 ? '+' : '') + formatNumber(entry.net_stamina_gain)}</td>
                    <td>${formatNumber(entry.akashi_encounters)}</td>
                    <td style="color:var(--text-secondary); font-size:0.85em;">${dateStr}</td>
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
        tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; color:#f87171;">加载失败</td></tr>';
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
