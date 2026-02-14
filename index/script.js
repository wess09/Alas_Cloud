const API_BASE_URL = "https://alas.nanoda.work";

// 格式化数字 (添加千位分隔符)
function formatNumber(num) {
  if (num === null || num === undefined) return "--";
  return new Intl.NumberFormat().format(num);
}

// 格式化百分比
function formatPercent(num) {
  if (num === null || num === undefined) return "--";
  return (num * 100).toFixed(2);
}

// SSE 连接加载数据（实时推送）
function connectSSE() {
  const evtSource = new EventSource(`${API_BASE_URL}/api/telemetry/stats/stream`);

  evtSource.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      updateUI(data);
    } catch (e) {
      console.error("Error parsing SSE data:", e);
    }
  };

  evtSource.onerror = (err) => {
    console.warn("SSE connection error, will auto-reconnect:", err);
    // EventSource 会自动重连，无需手动处理
  };

  return evtSource;
}

// 更新 UI
function updateUI(data) {
  const set = (id, val) => {
    const el = document.getElementById(id);
    if (el) el.innerText = val;
  };

  set("total-devices", formatNumber(data.total_devices));
  set("total-battles", formatNumber(data.total_battle_count));
  set("total-rounds", formatNumber(data.total_battle_rounds));
  set("total-cost", formatNumber(data.total_sortie_cost));
  set("akashi-encounters", formatNumber(data.total_akashi_encounters));

  // 遇见明石概率
  set("akashi-prob-card", formatPercent(data.avg_akashi_probability) + "%");

  // 循环效率
  set("cycle-efficiency", formatPercent(data.cycle_efficiency) + "%");

  // 平均体力
  const avgStaminaDisplay =
    data.total_akashi_encounters > 0 && data.avg_stamina !== undefined
      ? data.avg_stamina.toFixed(5)
      : "-";
  set("avg-stamina-card", avgStaminaDisplay);

  // 净赚体力
  const netStaminaDisplay = data.net_stamina_gain !== undefined
    ? Math.round(data.net_stamina_gain)
    : 0;

  // 更新详细表格
  const tbody = document.getElementById("details-table-body");
  if (tbody) {
    tbody.innerHTML = `
            <tr>
                <td>${formatNumber(data.total_battle_count)}</td>
                <td>${formatNumber(data.total_battle_rounds)}</td>
                <td>${formatNumber(data.total_sortie_cost)}</td>
                <td>${formatNumber(data.total_akashi_encounters)}</td>
                <td class="highlight">${formatPercent(data.avg_akashi_probability)}%</td>
                <td>${avgStaminaDisplay}</td>
                <td class="highlight text-success">${netStaminaDisplay >= 0 ? '+' : ''}${formatNumber(netStaminaDisplay)}</td>
                <td class="highlight">${formatPercent(data.cycle_efficiency)}%</td>
            </tr>
        `;
  }
}

// 页面加载完成后执行
// Dashboard Configuration
const dashboardCards = [
  { id: 'total-devices', className: 'total-users', icon: 'icon-users.svg', title: '指挥官总数', label: '活跃设备数量' },
  { id: 'total-battles', className: 'total-battles', icon: 'icon-swords.svg', title: '战斗场次', label: '累计通关次数' },
  { id: 'total-rounds', className: 'total-rounds', icon: 'icon-refresh.svg', title: '战斗轮次', label: '累计所有轮次' },
  { id: 'total-cost', className: 'total-cost', icon: 'icon-droplet.svg', title: '出击消耗', label: '体力消耗总量' },
  { id: 'akashi-encounters', className: 'akashi', icon: 'icon-cat.svg', title: '遇见明石次数', label: '累计遭遇次数' },
  { id: 'akashi-prob-card', className: 'akashi-prob', icon: 'icon-dice.svg', title: '遇见明石概率', label: '平均遭遇率', unit: '%' },
  { id: 'avg-stamina-card', className: 'avg-stamina', icon: 'icon-battery.svg', title: '平均体力', label: '每次遇见明石获取' },
  { id: 'cycle-efficiency', className: 'cycle-efficiency', icon: 'icon-trending-up.svg', title: '循环效率', label: '净收益/出击消耗', unit: '%' },
];

function renderDashboard() {
  const grid = document.querySelector('.stats-grid');
  if (!grid) {
    console.error('Stats grid not found');
    return;
  }

  grid.innerHTML = dashboardCards.map(card => `
        <div class="card stat-card ${card.className}">
          <div class="stat-icon">
             <img src="${card.icon}" alt="${card.title}" class="icon-svg" />
          </div>
          <div class="stat-content">
            <h3>${card.title}</h3>
            <div class="value" id="${card.id}">--${card.unit || ''}</div>
            <div class="label">${card.label}</div>
          </div>
        </div>
    `).join('');
}

// 页面加载完成后执行
document.addEventListener("DOMContentLoaded", () => {
  renderDashboard();
  connectSSE();
});
