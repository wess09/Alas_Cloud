document.addEventListener('DOMContentLoaded', () => {
    const urlParams = new URLSearchParams(window.location.search);
    const deviceId = urlParams.get('device_id');

    if (!deviceId) {
        document.getElementById('loading').innerHTML = '❌ 错误：未提供指挥官 Device ID';
        return;
    }

    fetchHistoryData(deviceId);
});

function formatNumber(num) {
    if (num === null || num === undefined) return '0';
    return new Intl.NumberFormat('en-US').format(Math.round(num));
}

function fetchHistoryData(deviceId) {
    fetch(`/api/telemetry/history?device_id=${encodeURIComponent(deviceId)}`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            document.getElementById('loading').style.display = 'none';
            document.getElementById('content').style.display = 'block';

            // Populate Info
            document.getElementById('c-name').textContent = data.username || '未知指挥官';
            document.getElementById('c-id').textContent = `ID: ${data.device_id.substring(0, 8)}...`;

            // Populate Totals
            const t = data.total;
            document.getElementById('t-battles').textContent = formatNumber(t.battle_rounds);

            const staminaElem = document.getElementById('t-stamina');
            staminaElem.textContent = formatNumber(t.net_stamina_gain);
            if (t.net_stamina_gain > 0) {
                staminaElem.style.color = '#10b981'; // Green
                staminaElem.textContent = '+' + staminaElem.textContent;
            } else if (t.net_stamina_gain < 0) {
                staminaElem.style.color = '#ef4444'; // Red
            }

            document.getElementById('t-cost').textContent = formatNumber(t.sortie_cost);
            document.getElementById('t-akashi').textContent = formatNumber(t.akashi_encounters);

            // Populate Monthly Table
            const tbody = document.getElementById('history-list');
            tbody.innerHTML = '';

            if (!data.history || data.history.length === 0) {
                tbody.innerHTML = `<tr><td colspan="6" class="empty-state" style="border:none;">暂无月度记录</td></tr>`;
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
                    <td>
                        <div class="status-indicator">
                            <div class="status-dot"></div>
                            ${new Date(record.updated_at).toLocaleDateString()}
                        </div>
                    </td>
                `;
                tbody.appendChild(tr);
            });
        })
        .catch(error => {
            console.error('Error fetching history:', error);
            document.getElementById('loading').innerHTML = '❌ 获取档案数据失败，请稍后重试。';
        });
}
