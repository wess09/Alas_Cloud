function formatNumber(num) {
    if (num === null || num === undefined) return '0';
    return new Intl.NumberFormat('en-US').format(Math.round(num));
}

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
    if (!grid) return;

    grid.innerHTML = dashboardCards.map(card => `
        <div class="card stat-card ${card.className}">
          <div class="stat-icon">
             <img src="${card.icon}" alt="${card.title}" class="icon-svg" />
          </div>
          <div class="stat-content">
            <h3>${card.title}</h3>
            <div class="value" id="${card.id}">--</div>
            <div class="label">${card.label}</div>
          </div>
        </div>
    `).join('');
}

function fetchGlobalHistoryData() {
    fetch(`/api/telemetry/global_history`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            const loading = document.getElementById('loading');
            const content = document.getElementById('content');
            if (loading) loading.style.display = 'none';
            if (content) content.style.display = 'block';

            // Populate Totals
            const t = data.total;
            
            // Formatters
            const fmtInt = (n) => formatNumber(Math.max(0, Math.round(n)));
            const fmtPct = (n) => (n * 100).toFixed(5) + "%";
            const fmtDec5 = (n) => n.toFixed(3);

            // Update cards
            if (document.getElementById('total-devices')) document.getElementById('total-devices').innerText = fmtInt(t.total_devices);
            if (document.getElementById('total-battles')) document.getElementById('total-battles').innerText = fmtInt(t.battle_count);
            if (document.getElementById('total-rounds')) document.getElementById('total-rounds').innerText = fmtInt(t.battle_rounds);
            if (document.getElementById('total-cost')) document.getElementById('total-cost').innerText = fmtInt(t.sortie_cost);
            if (document.getElementById('akashi-encounters')) document.getElementById('akashi-encounters').innerText = fmtInt(t.akashi_encounters);
            if (document.getElementById('akashi-prob-card')) document.getElementById('akashi-prob-card').innerText = fmtPct(t.akashi_probability);
            if (document.getElementById('avg-stamina-card')) document.getElementById('avg-stamina-card').innerText = fmtDec5(t.average_stamina);
            if (document.getElementById('cycle-efficiency')) document.getElementById('cycle-efficiency').innerText = fmtPct(t.cycle_efficiency);

            // Populate Monthly Table
            const tbody = document.getElementById('history-list');
            if (!tbody) return;
            tbody.innerHTML = '';

            if (!data.history || data.history.length === 0) {
                tbody.innerHTML = `<tr><td colspan="5" class="empty-state" style="border:none;">暂无月度记录</td></tr>`;
                return;
            }

            data.history.forEach((record, index) => {
                const tr = document.createElement('tr');
                tr.style.animationDelay = `${index * 0.1}s`;

                // Calculate Net AP color
                let netApColor = 'inherit';
                let netApPrefix = '';
                if (record.net_stamina_gain > 0) {
                    netApColor = '#10b981';
                    netApPrefix = '+';
                } else if (record.net_stamina_gain < 0) {
                    netApColor = '#ef4444';
                }

                // Format Month
                const monthParts = record.month.split('-');
                const formattedMonth = monthParts.length === 2 ? `${monthParts[0]}年${monthParts[1]}月` : record.month;

                tr.innerHTML = `
                    <td><span class="month-badge">${formattedMonth}</span></td>
                    <td class="numeric-cell">${formatNumber(record.battle_rounds)}</td>
                    <td class="numeric-cell" style="color: ${netApColor}; font-weight: bold;">
                        ${netApPrefix}${formatNumber(record.net_stamina_gain)}
                    </td>
                    <td class="numeric-cell"><span class="highlight-value">${formatNumber(record.akashi_encounters)}</span></td>
                    <td class="numeric-cell">${record.average_stamina.toFixed(1)}</td>
                `;
                tbody.appendChild(tr);
            });
        })
        .catch(error => {
            console.error('Error fetching global history:', error);
            const loading = document.getElementById('loading');
            if (loading) loading.innerHTML = '❌ 获取全网档案数据失败，请稍后重试。';
        });
}

document.addEventListener('DOMContentLoaded', () => {
    renderDashboard();
    fetchGlobalHistoryData();
});
