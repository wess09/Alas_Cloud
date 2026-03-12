const API_BASE = 'https://alas-apiv2.nanoda.work';
let miniChart = null;

document.addEventListener('DOMContentLoaded', () => {
    miniChart = echarts.init(document.getElementById('mini-chart'), null, { renderer: 'canvas' });

    fetchLatest();
    fetchKlineDir();

    // 轮询 30 秒，确保大盘数据事实同步
    setInterval(() => {
        fetchLatest();
        fetchKlineDir();
    }, 30000);
});

async function fetchLatest() {
    try {
        const timestamp = new Date().getTime();
        const res = await fetch(`${API_BASE}/api/stamina/latest?_t=${timestamp}`);
        const data = await res.json();

        if (!data || data.current_total === undefined) return;

        const total = data.current_total || 0;
        const change = data.change || 0;
        const changePct = data.change_percent || 0;

        document.getElementById('current-total').textContent = new Intl.NumberFormat('en-US').format(total);

        const group = document.getElementById('change-group');
        const arrow = document.getElementById('change-arrow');
        const pct = document.getElementById('change-pct');
        const abs = document.getElementById('change-abs');

        // 红涨绿跌
        if (change >= 0) {
            group.className = 'change-group up';
            arrow.textContent = '▲';
            pct.textContent = '+' + changePct.toFixed(2) + '%';
            abs.textContent = '+' + change;
        } else {
            group.className = 'change-group down';
            arrow.textContent = '▼';
            pct.textContent = changePct.toFixed(2) + '%';
            abs.textContent = change;
        }

        if (data.minute_key) {
            document.getElementById('last-update').textContent = data.minute_key.split('T')[1];
        }
    } catch (e) {
        console.error('获取最新数据失败:', e);
    }
}

async function fetchKlineDir() {
    // 抓取近一小时的1分钟级别数据用于绘制背景波浪图
    try {
        const timestamp = new Date().getTime();
        const res = await fetch(`${API_BASE}/api/stamina/kline?period=1m&range=day&_t=${timestamp}`);
        const json = await res.json();
        let rawData = json.data || [];

        if (rawData.length === 0) return;

        // 截取最后60根K线（1小时）让视效更紧凑
        if (rawData.length > 60) {
            rawData = rawData.slice(-60);
        }

        // 判断涨跌决定线段颜色（红涨绿跌）
        const isUp = rawData[rawData.length - 1].close >= rawData[0].close;
        const lineColor = isUp ? '#ff453a' : '#32d74b';

        const option = {
            animation: false,
            grid: { left: -10, right: 0, top: 15, bottom: -10 }, // 留出顶部呼吸空间防止碰到文字
            xAxis: {
                type: 'category',
                data: rawData.map(d => d.minute_key),
                show: false
            },
            yAxis: {
                type: 'value',
                scale: true,
                show: false
            },
            series: [{
                type: 'line',
                data: rawData.map(d => d.close),
                smooth: true,
                symbol: 'none',
                lineStyle: { width: 3, color: lineColor },
                areaStyle: {
                    color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                        { offset: 0, color: hexToRgba(lineColor, 0.5) },
                        { offset: 1, color: hexToRgba(lineColor, 0.0) }
                    ])
                }
            }]
        };
        miniChart.setOption(option, true);
    } catch (e) {
        console.error('获取K线数据失败:', e);
    }
}

function hexToRgba(hex, alpha) {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}
