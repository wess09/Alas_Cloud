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
// ---- 数字滚动动画 ----

// 存储每个元素的当前数值和动画帧
const animState = {};

/**
 * 数字滚动动画
 * @param {string} id - 元素 ID
 * @param {number} endVal - 目标值
 * @param {function} formatter - 格式化函数 (num) => string
 * @param {number} duration - 动画时长 ms
 */
function animateValue(id, endVal, formatter, duration = 600) {
  const el = document.getElementById(id);
  if (!el) return;

  // 取消之前的动画
  if (animState[id]?.rafId) {
    cancelAnimationFrame(animState[id].rafId);
  }

  const startVal = animState[id]?.value ?? 0;
  animState[id] = { value: endVal, rafId: null };

  // 值没变就不动画
  if (startVal === endVal) {
    el.innerText = formatter(endVal);
    return;
  }

  const startTime = performance.now();

  function tick(now) {
    const elapsed = now - startTime;
    const progress = Math.min(elapsed / duration, 1);

    // easeOutExpo 缓动
    const ease = progress === 1 ? 1 : 1 - Math.pow(2, -10 * progress);

    const current = startVal + (endVal - startVal) * ease;
    el.innerText = formatter(current);

    if (progress < 1) {
      animState[id].rafId = requestAnimationFrame(tick);
    } else {
      el.innerText = formatter(endVal);
      animState[id].rafId = null;
    }
  }

  animState[id].rafId = requestAnimationFrame(tick);
}

// ---- SSE ----

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
  };

  return evtSource;
}

// ---- 更新 UI ----

function updateUI(data) {
  // 整数格式化（带千位分隔符）
  const fmtInt = (n) => formatNumber(Math.round(n));
  // 百分比格式化
  const fmtPct = (n) => (n * 100).toFixed(2) + "%";
  // 小数格式化（5 位）
  const fmtDec5 = (n) => n.toFixed(5);

  // 卡片动画
  animateValue("total-devices", data.total_devices ?? 0, fmtInt);
  animateValue("total-battles", data.total_battle_count ?? 0, fmtInt);
  animateValue("total-rounds", data.total_battle_rounds ?? 0, fmtInt);
  animateValue("total-cost", data.total_sortie_cost ?? 0, fmtInt);
  animateValue("akashi-encounters", data.total_akashi_encounters ?? 0, fmtInt);
  animateValue("akashi-prob-card", data.avg_akashi_probability ?? 0, fmtPct);
  animateValue("cycle-efficiency", data.cycle_efficiency ?? 0, fmtPct);

  if (data.total_akashi_encounters > 0 && data.avg_stamina !== undefined) {
    animateValue("avg-stamina-card", data.avg_stamina, fmtDec5);
  } else {
    const el = document.getElementById("avg-stamina-card");
    if (el) el.innerText = "-";
  }

  // 净赚体力
  const netVal = data.net_stamina_gain !== undefined ? Math.round(data.net_stamina_gain) : 0;

  // 更新详细表格（表格也用动画）
  const tbody = document.getElementById("details-table-body");
  if (tbody) {
    // 首次需要创建行
    if (!tbody.querySelector("tr")) {
      tbody.innerHTML = `
            <tr>
                <td id="tbl-battles">--</td>
                <td id="tbl-rounds">--</td>
                <td id="tbl-cost">--</td>
                <td id="tbl-akashi">--</td>
                <td class="highlight" id="tbl-akashi-prob">--</td>
                <td id="tbl-avg-stamina">--</td>
                <td class="highlight text-success" id="tbl-net-stamina">--</td>
                <td class="highlight" id="tbl-cycle-eff">--</td>
            </tr>
        `;
    }

    animateValue("tbl-battles", data.total_battle_count ?? 0, fmtInt);
    animateValue("tbl-rounds", data.total_battle_rounds ?? 0, fmtInt);
    animateValue("tbl-cost", data.total_sortie_cost ?? 0, fmtInt);
    animateValue("tbl-akashi", data.total_akashi_encounters ?? 0, fmtInt);
    animateValue("tbl-akashi-prob", data.avg_akashi_probability ?? 0, fmtPct);
    animateValue("tbl-cycle-eff", data.cycle_efficiency ?? 0, fmtPct);

    if (data.total_akashi_encounters > 0 && data.avg_stamina !== undefined) {
      animateValue("tbl-avg-stamina", data.avg_stamina, fmtDec5);
    }

    animateValue("tbl-net-stamina", netVal, (n) => {
      const v = Math.round(n);
      return (v >= 0 ? "+" : "") + formatNumber(v);
    });
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
