const API_BASE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
    ? 'http://localhost:8000/api'
    : '/api';

function formatNumber(num) {
    if (num === null || num === undefined) return "--";
    return new Intl.NumberFormat().format(num);
}

// Fetch data on load
document.addEventListener('DOMContentLoaded', () => {
    fetchSuspects();
    fetchBanned();
});

async function fetchSuspects() {
    const list = document.getElementById('suspects-list');
    const token = localStorage.getItem('alas_admin_token');
    try {
        const res = await fetch(`${API_BASE}/reports`);
        const data = await res.json();

        if (!data || data.length === 0) {
            list.innerHTML = '<div class="empty-state" style="grid-column: 1/-1;">没有嫌疑人 / No suspects found.</div>';
            return;
        }

        list.innerHTML = data.map(user => {
            let adminBtns = '';
            if (token) {
                adminBtns = `
                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem;">
                        <button class="vote-btn" style="background: rgba(255, 71, 87, 0.4); border-color: #ff4757; color: white;" onclick="executeDirectBan('${user.target_id}')">
                            🔨 DirecBan
                        </button>
                        <button class="vote-btn" style="background: rgba(46, 213, 115, 0.4); border-color: #2ed573; color: white;" onclick="executeDismiss('${user.target_id}')">
                            🛡️ Dismiss
                        </button>
                    </div>
                `;
            }

            return `
                <div class="suspect-card">
                    <div class="suspect-header">
                        <div>
                            <div class="suspect-name">${escapeHtml(user.username)}</div>
                            <span class="suspect-id">${user.target_id}</span>
                        </div>
                        <div class="report-count-badge">
                            ${user.report_count} / 5
                        </div>
                    </div>

                    <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.5rem; margin: 0.75rem 0;">
                        <div style="text-align: center; padding: 0.5rem; background: rgba(0,0,0,0.3); border-radius: 0.5rem;">
                            <div style="font-size: 0.65rem; color: var(--text-secondary); text-transform: uppercase;">⚔️ Rounds</div>
                            <div style="font-size: 1.1rem; font-weight: 700; color: var(--text-primary);">${formatNumber(user.battle_rounds)}</div>
                        </div>
                        <div style="text-align: center; padding: 0.5rem; background: rgba(0,0,0,0.3); border-radius: 0.5rem;">
                            <div style="font-size: 0.65rem; color: var(--text-secondary); text-transform: uppercase;">⚡ Stamina</div>
                            <div style="font-size: 1.1rem; font-weight: 700; color: ${user.net_stamina_gain >= 0 ? '#2ed573' : '#ff4757'};">${(user.net_stamina_gain >= 0 ? '+' : '') + formatNumber(user.net_stamina_gain)}</div>
                        </div>
                        <div style="text-align: center; padding: 0.5rem; background: rgba(0,0,0,0.3); border-radius: 0.5rem;">
                            <div style="font-size: 0.65rem; color: var(--text-secondary); text-transform: uppercase;">🐱 Akashi</div>
                            <div style="font-size: 1.1rem; font-weight: 700; color: var(--text-primary);">${formatNumber(user.akashi_encounters)}</div>
                        </div>
                    </div>
                    
                    <div class="reason-box">
                        <strong>Latest Report:</strong><br>
                        "${escapeHtml(user.latest_reason)}"
                    </div>

                    <div style="display:flex; flex-direction:column; gap:0.5rem;">
                        <button class="vote-btn" onclick="votePunish('${user.target_id}')">
                            💀 投票封禁 / Vote to Ban
                        </button>
                        ${adminBtns}
                    </div>
                </div>
            `;
        }).join('');
    } catch (e) {
        console.error(e);
        list.innerHTML = '<div style="text-align: center; color: #ff6b81;">Failed to load suspects.</div>';
    }
}

async function fetchBanned() {
    const tbody = document.getElementById('banned-list');
    const token = localStorage.getItem('alas_admin_token');
    try {
        const res = await fetch(`${API_BASE}/bans`);
        const data = await res.json();

        if (!data || data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="empty-state" style="border:none;">没有封禁记录 / No banned users.</td></tr>';
            return;
        }

        tbody.innerHTML = data.map(user => {
            let adminCell = '';
            if (token) {
                adminCell = `
                    <td>
                        <button class="btn" style="height: 2rem; font-size: 0.75rem; background: #10b981; border: none;" onclick="executeUnban('${user.device_id}')">
                            解封 / Unban
                        </button>
                    </td>
                `;
            }
            return `
                <tr>
                    <td>${user.device_id}</td>
                    <td>${escapeHtml(user.username)}</td>
                    <td>${maskIP(user.ip_address)}</td>
                    <td>${escapeHtml(user.reason)}</td>
                    <td>${new Date(user.banned_at).toLocaleString()}</td>
                    ${adminCell}
                </tr>
            `;
        }).join('');
    } catch (e) {
        console.error(e);
        tbody.innerHTML = `<tr><td colspan="${token ? 6 : 5}" style="text-align: center; color: red;">Failed to load blacklist.</td></tr>`;
    }
}

async function votePunish(targetId) {
    if (!confirm('确定要投票封禁该用户吗？\nAre you sure you want to vote to ban this user?')) return;

    try {
        // Voting is essentially reporting again with a "vote" reason
        const token = localStorage.getItem('alas_admin_token');
        const headers = { 'Content-Type': 'application/json' };
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const res = await fetch(`${API_BASE}/report`, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify({
                target_id: targetId,
                reason: 'Vote to ban from Court'
            })
        });
        const json = await res.json();

        if (res.ok) {
            alert('投票成功！\nVote submitted.');
            fetchSuspects(); // Refresh list
            fetchBanned();   // Refresh blacklist in case they got banned
        } else {
            alert('投票失败: ' + json.error);
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

// Check login status on load
document.addEventListener('DOMContentLoaded', () => {
    checkLogin();
});

function checkLogin() {
    const token = localStorage.getItem('alas_admin_token');
    if (token) {
        document.getElementById('admin-panel').style.display = 'block';
    } else {
        document.getElementById('admin-panel').style.display = 'none';
    }
}

async function performLogin() {
    const username = document.getElementById('login-username').value.trim();
    const password = document.getElementById('login-password').value.trim();

    if (!username || !password) {
        alert("请输入用户名和密码");
        return;
    }

    try {
        const res = await fetch(`${API_BASE}/admin/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });
        const data = await res.json();

        if (res.ok) {
            localStorage.setItem('alas_admin_token', data.token);
            document.getElementById('login-modal').style.display = 'none';
            checkLogin();
            document.getElementById('login-password').value = ''; // clear
        } else {
            alert("Login Failed: " + (data.detail || "Unknown error"));
        }
    } catch (e) {
        alert("Network Error");
    }
}

function logout() {
    localStorage.removeItem('alas_admin_token');
    checkLogin();
}

// Unified Execution Functions
async function executeDirectBan(targetId, reason = "Admin shortcut") {
    const token = localStorage.getItem('alas_admin_token');
    if (!token) {
        alert("Session expired. Please login again.");
        logout();
        return;
    }

    if (!confirm(`Are you sure you want to BAN user ${targetId}?`)) return;

    try {
        const res = await fetch(`${API_BASE}/admin/ban`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                target_id: targetId,
                reason: reason
            })
        });
        const json = await res.json();

        if (res.ok) {
            alert('User BANNED successfully.');
            fetchBanned();
            fetchSuspects();
        } else {
            if (res.status === 401) {
                logout();
                alert("Session expired");
                return;
            }
            alert('Error: ' + (json.error || 'Unknown error'));
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

async function executeUnban(targetId) {
    const token = localStorage.getItem('alas_admin_token');
    if (!token) {
        alert("Session expired. Please login again.");
        logout();
        return;
    }

    if (!confirm(`Are you sure you want to UNBAN user ${targetId}?`)) return;

    try {
        const res = await fetch(`${API_BASE}/admin/unban`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                target_id: targetId
            })
        });
        const json = await res.json();

        if (res.ok) {
            alert('User unbanned successfully.');
            fetchBanned();
        } else {
            if (res.status === 401) {
                logout();
                alert("Session expired");
                return;
            }
            alert('Error: ' + (json.error || 'Unknown error'));
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

async function executeDismiss(targetId) {
    const token = localStorage.getItem('alas_admin_token');
    if (!token) {
        alert("Session expired. Please login again.");
        logout();
        return;
    }

    if (!confirm(`Are you sure you want to DISMISS all reports for user ${targetId}?`)) return;

    try {
        const res = await fetch(`${API_BASE}/admin/dismiss`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                target_id: targetId
            })
        });
        const json = await res.json();

        if (res.ok) {
            alert('All reports dismissed for user.');
            fetchSuspects();
        } else {
            if (res.status === 401) {
                logout();
                alert("Session expired");
                return;
            }
            alert('Error: ' + (json.error || 'Unknown error'));
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

// Manual Form Wrappers
async function directBanUser() {
    const targetId = document.getElementById('ban-id').value.trim();
    const reason = document.getElementById('ban-reason').value.trim();
    if (!targetId) {
        alert('请输入 Device ID');
        return;
    }
    await executeDirectBan(targetId, reason);
    document.getElementById('ban-id').value = '';
    document.getElementById('ban-reason').value = '';
}

async function unbanUser() {
    const targetId = document.getElementById('unban-id').value.trim();
    if (!targetId) {
        alert('请输入 Device ID');
        return;
    }
    await executeUnban(targetId);
    document.getElementById('unban-id').value = '';
}

function escapeHtml(text) {
    if (!text) return '';
    return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function maskIP(ip) {
    if (!ip) return 'Unknown';
    if (ip.includes(':')) return 'IPv6 (Masked)'; // Simplify IPv6
    const parts = ip.split('.');
    if (parts.length === 4) {
        return `${parts[0]}.${parts[1]}.*.*`;
    }
    return ip;
}
