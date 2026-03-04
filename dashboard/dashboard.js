// ============================================================
//  侵蚀体力大盘 – dashboard.js
//  ECharts K线图 + 技术指标 (MA/BOLL/MACD/KDJ)
// ============================================================

const API_BASE = 'https://alas-apiv2.nanoda.work';

// ---- 全局状态 ----
let currentRange = 'day';       // day | week | month
let currentPeriod = '1m';       // 1m | 5m | 1h | 1d
let currentChartType = 'candlestick'; // candlestick | line | area

const indicators = {
    ma: true,
    boll: false,
    macd: true,
    kdj: false,
};

let rawData = [];   // 原始 OHLCV 数据
let mainChart = null;
let volumeChart = null;
let indicatorChart = null;

// ---- 初始化 ----
document.addEventListener('DOMContentLoaded', () => {
    initCharts();
    // 首次加载：拉取 K 线 + 最新汇总
    fetchData();
    fetchLatest();
    // 每 30 秒自动刷新（数据聚合间隔 60 秒，30 秒轮询足够及时）
    setInterval(() => {
        fetchData();
        fetchLatest();
    }, 30000);
});

// ---- ECharts 实例 ----
function initCharts() {
    mainChart = echarts.init(document.getElementById('chart-main'), null, { renderer: 'canvas' });
    volumeChart = echarts.init(document.getElementById('chart-volume'), null, { renderer: 'canvas' });
    indicatorChart = echarts.init(document.getElementById('chart-indicator'), null, { renderer: 'canvas' });

    // 联动
    echarts.connect([mainChart, volumeChart, indicatorChart]);

    // 自适应
    window.addEventListener('resize', () => {
        mainChart.resize();
        volumeChart.resize();
        indicatorChart.resize();
    });
}

// ---- 数据拉取 ----
async function fetchData() {
    try {
        const res = await fetch(`${API_BASE}/api/stamina/kline?period=${currentPeriod}&range=${currentRange}`);
        const json = await res.json();
        rawData = json.data || [];
        renderAllCharts();
        updateHeaderFromData();
    } catch (e) {
        console.error('数据加载失败:', e);
    }
}

async function fetchLatest() {
    try {
        const res = await fetch(`${API_BASE}/api/stamina/latest`);
        const data = await res.json();
        updateHeader(data);
        updateSidebar(data);
    } catch (e) {
        console.error('最新数据获取失败:', e);
    }
}


// ---- 顶部信息栏更新 ----
function updateHeader(data) {
    if (!data || data.current_total === undefined) return;

    const total = data.current_total || 0;
    const change = data.change || 0;
    const changePct = data.change_percent || 0;

    document.getElementById('current-total').textContent = formatNum(total, 0);

    const badge = document.getElementById('change-badge');
    const arrow = document.getElementById('change-arrow');
    const pct = document.getElementById('change-pct');
    const abs = document.getElementById('change-abs');

    if (change >= 0) {
        badge.className = 'change-badge up';
        arrow.textContent = '▲';
        pct.textContent = '+' + changePct.toFixed(2) + '%';
        abs.textContent = '+' + formatNum(change, 0);
        abs.style.color = 'var(--green)';
    } else {
        badge.className = 'change-badge down';
        arrow.textContent = '▼';
        pct.textContent = changePct.toFixed(2) + '%';
        abs.textContent = formatNum(change, 0);
        abs.style.color = 'var(--red)';
    }

    if (data.minute_key) {
        document.getElementById('last-update-time').textContent = data.minute_key.replace('T', ' ');
    }

    // 数据质量
    if (data.reported_count !== undefined) {
        const reported = data.reported_count;
        const filled = data.filled_count || 0;
        const total_users = reported + filled;

        document.getElementById('quality-reported').textContent = reported + ' 人';
        document.getElementById('quality-filled').textContent = filled + ' 人';

        if (total_users > 0) {
            const coverage = (reported / total_users * 100).toFixed(1) + '%';
            document.getElementById('quality-coverage').textContent = coverage;
            document.getElementById('progress-reported').style.width = (reported / total_users * 100) + '%';
            document.getElementById('progress-filled').style.width = (filled / total_users * 100) + '%';
        }
    }
}

function updateHeaderFromData() {
    if (rawData.length === 0) return;
    const last = rawData[rawData.length - 1];
    const prev = rawData.length > 1 ? rawData[rawData.length - 2] : last;
    const change = last.close - prev.close;
    const changePct = prev.close > 0 ? (change / prev.close * 100) : 0;

    updateHeader({
        current_total: last.close,
        change: change,
        change_percent: changePct,
        minute_key: last.minute_key,
        reported_count: last.reported_count,
        filled_count: last.filled_count,
    });

    // 侧边栏数据概览
    document.getElementById('info-open').textContent = formatNum(last.open, 0);
    document.getElementById('info-close').textContent = formatNum(last.close, 0);
    document.getElementById('info-high').textContent = formatNum(last.high, 0);
    document.getElementById('info-low').textContent = formatNum(last.low, 0);
    document.getElementById('info-volume').textContent = formatNum(last.volume, 0);
    const amplitude = last.open > 0 ? ((last.high - last.low) / last.open * 100).toFixed(2) + '%' : '--';
    document.getElementById('info-amplitude').textContent = amplitude;

    fetchLatest();
}

function updateSidebar(data) {
    if (!data) return;

    // 排行榜
    const rankList = document.getElementById('rank-list');
    if (data.top_users && data.top_users.length > 0) {
        rankList.innerHTML = data.top_users.map((user, i) => `
      <div class="rank-item">
        <span class="rank-number">${i + 1}</span>
        <div class="rank-info">
          <div class="rank-name">${escapeHtml(user.username || '未知指挥官')}</div>
          <div class="rank-id">${user.device_id}...</div>
        </div>
        <span class="rank-value">${formatNum(user.stamina, 0)}</span>
      </div>
    `).join('');
    }
}

// ============================================================
//  技术指标计算
// ============================================================

function calcMA(data, period) {
    const result = [];
    for (let i = 0; i < data.length; i++) {
        if (i < period - 1) { result.push(null); continue; }
        let sum = 0;
        for (let j = 0; j < period; j++) sum += data[i - j].close;
        result.push(+(sum / period).toFixed(2));
    }
    return result;
}

function calcBOLL(data, period = 20, multiplier = 2) {
    const mid = [], upper = [], lower = [];
    for (let i = 0; i < data.length; i++) {
        if (i < period - 1) { mid.push(null); upper.push(null); lower.push(null); continue; }
        let sum = 0;
        for (let j = 0; j < period; j++) sum += data[i - j].close;
        const avg = sum / period;
        let sqSum = 0;
        for (let j = 0; j < period; j++) sqSum += Math.pow(data[i - j].close - avg, 2);
        const std = Math.sqrt(sqSum / period);
        mid.push(+avg.toFixed(2));
        upper.push(+(avg + multiplier * std).toFixed(2));
        lower.push(+(avg - multiplier * std).toFixed(2));
    }
    return { mid, upper, lower };
}

function calcMACD(data, fast = 12, slow = 26, signal = 9) {
    const closes = data.map(d => d.close);
    const emaFast = calcEMA(closes, fast);
    const emaSlow = calcEMA(closes, slow);
    const dif = [];
    for (let i = 0; i < closes.length; i++) {
        dif.push(emaFast[i] !== null && emaSlow[i] !== null ? +(emaFast[i] - emaSlow[i]).toFixed(2) : null);
    }
    const dea = calcEMA(dif, signal);
    const macd = [];
    for (let i = 0; i < closes.length; i++) {
        macd.push(dif[i] !== null && dea[i] !== null ? +((dif[i] - dea[i]) * 2).toFixed(2) : null);
    }
    return { dif, dea, macd };
}

function calcKDJ(data, period = 9, kSmooth = 3, dSmooth = 3) {
    const kValues = [], dValues = [], jValues = [];
    let prevK = 50, prevD = 50;
    for (let i = 0; i < data.length; i++) {
        if (i < period - 1) { kValues.push(null); dValues.push(null); jValues.push(null); continue; }
        let high = -Infinity, low = Infinity;
        for (let j = 0; j < period; j++) {
            high = Math.max(high, data[i - j].high);
            low = Math.min(low, data[i - j].low);
        }
        const rsv = high !== low ? (data[i].close - low) / (high - low) * 100 : 50;
        const k = (prevK * (kSmooth - 1) + rsv) / kSmooth;
        const d = (prevD * (dSmooth - 1) + k) / dSmooth;
        const j = 3 * k - 2 * d;
        kValues.push(+k.toFixed(2));
        dValues.push(+d.toFixed(2));
        jValues.push(+j.toFixed(2));
        prevK = k;
        prevD = d;
    }
    return { k: kValues, d: dValues, j: jValues };
}

function calcEMA(data, period) {
    const result = [];
    const multiplier = 2 / (period + 1);
    let prev = null;
    for (let i = 0; i < data.length; i++) {
        if (data[i] === null) { result.push(null); continue; }
        if (prev === null) {
            prev = data[i];
            result.push(+prev.toFixed(2));
        } else {
            prev = (data[i] - prev) * multiplier + prev;
            result.push(+prev.toFixed(2));
        }
    }
    return result;
}

// ============================================================
//  ECharts 渲染
// ============================================================

function renderAllCharts() {
    if (rawData.length === 0) {
        showNoData();
        return;
    }
    renderMainChart();
    renderVolumeChart();
    renderIndicatorChart();
}

function showNoData() {
    const opt = {
        graphic: [{
            type: 'text',
            left: 'center',
            top: 'center',
            style: { text: '暂无数据', fontSize: 16, fill: '#64748b' }
        }]
    };
    mainChart.setOption(opt, true);
    volumeChart.setOption(opt, true);
    indicatorChart.setOption(opt, true);
}

function renderMainChart() {
    const categories = rawData.map(d => d.minute_key.replace('T', '\n'));
    const ma5 = calcMA(rawData, 5);
    const ma10 = calcMA(rawData, 10);
    const ma20 = calcMA(rawData, 20);
    const boll = calcBOLL(rawData);

    const series = [];

    // 主数据系列
    if (currentChartType === 'candlestick') {
        series.push({
            name: '体力大盘',
            type: 'candlestick',
            data: rawData.map(d => [d.open, d.close, d.low, d.high]),
            itemStyle: {
                color: '#ef4444',    // 阳线（收>开）
                color0: '#22c55e',    // 阴线
                borderColor: '#ef4444',
                borderColor0: '#22c55e',
            },
        });
    } else if (currentChartType === 'line') {
        series.push({
            name: '收盘价',
            type: 'line',
            data: rawData.map(d => d.close),
            smooth: true,
            symbol: 'none',
            lineStyle: { color: '#6366f1', width: 2 },
            itemStyle: { color: '#6366f1' },
        });
    } else if (currentChartType === 'area') {
        series.push({
            name: '收盘价',
            type: 'line',
            data: rawData.map(d => d.close),
            smooth: true,
            symbol: 'none',
            lineStyle: { color: '#6366f1', width: 2 },
            areaStyle: {
                color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: 'rgba(99, 102, 241, 0.35)' },
                    { offset: 1, color: 'rgba(99, 102, 241, 0.02)' },
                ]),
            },
            itemStyle: { color: '#6366f1' },
        });
    }

    // MA 指标
    if (indicators.ma) {
        series.push(
            { name: 'MA5', type: 'line', data: ma5, smooth: true, symbol: 'none', lineStyle: { width: 1.2, color: '#eab308' } },
            { name: 'MA10', type: 'line', data: ma10, smooth: true, symbol: 'none', lineStyle: { width: 1.2, color: '#06b6d4' } },
            { name: 'MA20', type: 'line', data: ma20, smooth: true, symbol: 'none', lineStyle: { width: 1.2, color: '#a855f7' } },
        );
    }

    // BOLL 指标
    if (indicators.boll) {
        series.push(
            { name: 'BOLL中轨', type: 'line', data: boll.mid, smooth: true, symbol: 'none', lineStyle: { width: 1, color: '#f59e0b', type: 'dashed' } },
            { name: 'BOLL上轨', type: 'line', data: boll.upper, smooth: true, symbol: 'none', lineStyle: { width: 1, color: '#ef4444', type: 'dotted' } },
            { name: 'BOLL下轨', type: 'line', data: boll.lower, smooth: true, symbol: 'none', lineStyle: { width: 1, color: '#22c55e', type: 'dotted' } },
        );
    }

    const option = {
        animation: true,
        backgroundColor: 'transparent',
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'cross', crossStyle: { color: '#94a3b8' }, lineStyle: { color: 'rgba(148,163,184,0.3)' } },
            backgroundColor: 'rgba(17, 24, 39, 0.95)',
            borderColor: 'rgba(99, 102, 241, 0.3)',
            borderWidth: 1,
            textStyle: { color: '#f1f5f9', fontFamily: 'Outfit, sans-serif', fontSize: 12 },
            formatter: function (params) {
                if (!params || params.length === 0) return '';
                const idx = params[0].dataIndex;
                const d = rawData[idx];
                if (!d) return '';
                let html = `<div style="font-weight:600;margin-bottom:6px;color:#a5b4fc">${d.minute_key.replace('T', ' ')}</div>`;
                html += `<div style="display:grid;grid-template-columns:auto auto;gap:2px 12px;font-family:'JetBrains Mono',monospace;font-size:11px">`;
                html += `<span style="color:#94a3b8">开盘</span><span>${formatNum(d.open, 0)}</span>`;
                html += `<span style="color:#94a3b8">收盘</span><span>${formatNum(d.close, 0)}</span>`;
                html += `<span style="color:#94a3b8">最高</span><span style="color:#ef4444">${formatNum(d.high, 0)}</span>`;
                html += `<span style="color:#94a3b8">最低</span><span style="color:#22c55e">${formatNum(d.low, 0)}</span>`;
                html += `<span style="color:#94a3b8">总量</span><span>${formatNum(d.volume, 0)}</span>`;
                html += `<span style="color:#94a3b8">上报</span><span style="color:#22c55e">${d.reported_count} 人</span>`;
                html += `<span style="color:#94a3b8">补全</span><span style="color:#eab308">${d.filled_count} 人</span>`;
                html += `</div>`;
                return html;
            },
        },
        legend: {
            data: series.map(s => s.name),
            textStyle: { color: '#94a3b8', fontSize: 11 },
            top: 8,
            right: 16,
            itemWidth: 14,
            itemHeight: 8,
        },
        grid: {
            left: 72,
            right: 48,
            top: 42,
            bottom: 28,
        },
        xAxis: {
            type: 'category',
            data: categories,
            axisLine: { lineStyle: { color: 'rgba(148,163,184,0.15)' } },
            axisTick: { show: false },
            axisLabel: { color: '#64748b', fontSize: 10, fontFamily: 'JetBrains Mono' },
            splitLine: { show: false },
        },
        yAxis: {
            type: 'value',
            scale: true,
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: '#64748b', fontSize: 10, fontFamily: 'JetBrains Mono' },
            splitLine: { lineStyle: { color: 'rgba(148,163,184,0.06)' } },
        },
        dataZoom: [
            {
                type: 'inside',
                xAxisIndex: 0,
                start: rawData.length > 60 ? 100 - (60 / rawData.length * 100) : 0,
                end: 100,
            },
        ],
        series: series,
    };

    mainChart.setOption(option, true);
}

function renderVolumeChart() {
    const categories = rawData.map(d => d.minute_key.replace('T', '\n'));

    const volumeColors = rawData.map((d, i) => {
        if (i === 0) return 'rgba(99, 102, 241, 0.6)';
        return d.close >= rawData[i - 1].close ? 'rgba(239, 68, 68, 0.6)' : 'rgba(34, 197, 94, 0.6)';
    });

    const option = {
        animation: true,
        backgroundColor: 'transparent',
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            backgroundColor: 'rgba(17, 24, 39, 0.95)',
            borderColor: 'rgba(99, 102, 241, 0.3)',
            borderWidth: 1,
            textStyle: { color: '#f1f5f9', fontSize: 11 },
            formatter: function (params) {
                if (!params[0]) return '';
                return `<span style="color:#94a3b8">总量:</span> ${formatNum(params[0].value, 0)}`;
            },
        },
        grid: {
            left: 72,
            right: 48,
            top: 12,
            bottom: 24,
        },
        xAxis: {
            type: 'category',
            data: categories,
            axisLine: { lineStyle: { color: 'rgba(148,163,184,0.15)' } },
            axisTick: { show: false },
            axisLabel: { show: false },
            splitLine: { show: false },
        },
        yAxis: {
            type: 'value',
            scale: true,
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: '#64748b', fontSize: 10, fontFamily: 'JetBrains Mono' },
            splitLine: { lineStyle: { color: 'rgba(148,163,184,0.06)' } },
        },
        dataZoom: [
            {
                type: 'inside',
                xAxisIndex: 0,
                start: rawData.length > 60 ? 100 - (60 / rawData.length * 100) : 0,
                end: 100,
            },
        ],
        series: [{
            name: '总量',
            type: 'bar',
            data: rawData.map((d, i) => ({
                value: d.volume,
                itemStyle: { color: volumeColors[i] },
            })),
            barMaxWidth: 12,
        }],
    };

    volumeChart.setOption(option, true);
}

function renderIndicatorChart() {
    const categories = rawData.map(d => d.minute_key.replace('T', '\n'));
    const series = [];
    let yAxisConfig = {};

    if (indicators.macd) {
        const macdData = calcMACD(rawData);
        series.push(
            {
                name: 'MACD',
                type: 'bar',
                data: macdData.macd.map(v => ({
                    value: v,
                    itemStyle: { color: v !== null && v >= 0 ? 'rgba(239, 68, 68, 0.7)' : 'rgba(34, 197, 94, 0.7)' },
                })),
                barMaxWidth: 6,
            },
            {
                name: 'DIF',
                type: 'line',
                data: macdData.dif,
                smooth: true,
                symbol: 'none',
                lineStyle: { width: 1.2, color: '#6366f1' },
            },
            {
                name: 'DEA',
                type: 'line',
                data: macdData.dea,
                smooth: true,
                symbol: 'none',
                lineStyle: { width: 1.2, color: '#f59e0b' },
            },
        );
    }

    if (indicators.kdj) {
        const kdjData = calcKDJ(rawData);
        series.push(
            { name: 'K', type: 'line', data: kdjData.k, smooth: true, symbol: 'none', lineStyle: { width: 1.2, color: '#6366f1' } },
            { name: 'D', type: 'line', data: kdjData.d, smooth: true, symbol: 'none', lineStyle: { width: 1.2, color: '#f59e0b' } },
            { name: 'J', type: 'line', data: kdjData.j, smooth: true, symbol: 'none', lineStyle: { width: 1.2, color: '#a855f7' } },
        );
    }

    if (series.length === 0) {
        indicatorChart.setOption({
            graphic: [{ type: 'text', left: 'center', top: 'center', style: { text: '请选择技术指标 (MACD / KDJ)', fontSize: 13, fill: '#64748b' } }]
        }, true);
        return;
    }

    const option = {
        animation: true,
        backgroundColor: 'transparent',
        tooltip: {
            trigger: 'axis',
            backgroundColor: 'rgba(17, 24, 39, 0.95)',
            borderColor: 'rgba(99, 102, 241, 0.3)',
            borderWidth: 1,
            textStyle: { color: '#f1f5f9', fontSize: 11 },
        },
        legend: {
            data: series.map(s => s.name),
            textStyle: { color: '#94a3b8', fontSize: 11 },
            top: 4,
            right: 16,
            itemWidth: 14,
            itemHeight: 8,
        },
        grid: {
            left: 72,
            right: 48,
            top: 30,
            bottom: 24,
        },
        xAxis: {
            type: 'category',
            data: categories,
            axisLine: { lineStyle: { color: 'rgba(148,163,184,0.15)' } },
            axisTick: { show: false },
            axisLabel: { show: false },
            splitLine: { show: false },
        },
        yAxis: {
            type: 'value',
            scale: true,
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: '#64748b', fontSize: 10, fontFamily: 'JetBrains Mono' },
            splitLine: { lineStyle: { color: 'rgba(148,163,184,0.06)' } },
        },
        dataZoom: [
            {
                type: 'inside',
                xAxisIndex: 0,
                start: rawData.length > 60 ? 100 - (60 / rawData.length * 100) : 0,
                end: 100,
            },
        ],
        series: series,
    };

    indicatorChart.setOption(option, true);
}

// ============================================================
//  控制栏交互
// ============================================================

function switchRange(range) {
    currentRange = range;
    updateControlGroup('range-group', 'data-range', range);
    fetchData();
}

function switchPeriod(period) {
    currentPeriod = period;
    updateControlGroup('period-group', 'data-period', period);
    fetchData();
}

function switchChartType(type) {
    currentChartType = type;
    updateControlGroup('type-group', 'data-type', type);
    renderAllCharts();
}

function toggleIndicator(name) {
    indicators[name] = !indicators[name];
    const btn = document.getElementById('toggle-' + name);
    btn.classList.toggle('active', indicators[name]);
    renderAllCharts();
}

function updateControlGroup(groupId, attr, value) {
    const group = document.getElementById(groupId);
    group.querySelectorAll('.control-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute(attr) === value);
    });
}

// ============================================================
//  工具函数
// ============================================================

function formatNum(n, decimals = 0) {
    if (n === null || n === undefined) return '--';
    return new Intl.NumberFormat('en-US', {
        minimumFractionDigits: decimals,
        maximumFractionDigits: decimals,
    }).format(n);
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
