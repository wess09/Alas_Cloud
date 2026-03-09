const API_BASE_URL = window.AZURSTAT_API_BASE_URL || '';
const NUMBER_FORMAT = new Intl.NumberFormat('en-US');
const DEFAULT_INTERVAL = 'day';

const state = {
    filtersLoaded: false,
    currentInterval: DEFAULT_INTERVAL,
};

document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('filter-form');
    const resetButton = document.getElementById('reset-filters');

    form.addEventListener('submit', event => {
        event.preventDefault();
        loadDashboard();
    });

    resetButton.addEventListener('click', () => {
        form.reset();
        state.currentInterval = DEFAULT_INTERVAL;
        updateIntervalBadge();
        loadDashboard();
    });

    initializePage();
});

async function initializePage() {
    showLoading();

    try {
        await loadFilters();
        await loadDashboard();
    } catch (error) {
        console.error('Failed to initialize AzurStat page:', error);
        showError('加载筛选器失败，请检查 API 地址或稍后重试。');
    }
}

async function loadFilters() {
    const data = await fetchJson('/api/azurstat/filters');
    populateSelect('task', data.tasks || [], '全部任务');
    populateSelect('zone_id', data.zones || [], '全部 Zone');
    populateSelect('hazard_level', data.hazard_levels || [], '全部等级');
    state.filtersLoaded = true;
}

async function loadDashboard() {
    if (!state.filtersLoaded) {
        return;
    }

    showLoading();
    clearError();

    const params = buildQueryParams();
    updateIntervalBadge();

    try {
        const [stats, items, history] = await Promise.all([
            fetchJson(`/api/azurstat/stats?${params.toString()}`),
            fetchJson(`/api/azurstat/items?${params.toString()}`),
            fetchJson(`/api/azurstat/history?${buildHistoryQueryParams(params).toString()}`),
        ]);

        renderStats(stats || {});
        renderItems(items);
        renderHistory(history);
        showContent();
    } catch (error) {
        console.error('Failed to load AzurStat dashboard:', error);
        showError('获取统计数据失败，请确认前端可访问对应 AzurStat API。');
    }
}

function buildQueryParams() {
    const params = new URLSearchParams();
    const formData = new FormData(document.getElementById('filter-form'));

    for (const [key, value] of formData.entries()) {
        if (!value) {
            continue;
        }
        params.set(key, value);
    }

    return params;
}

function buildHistoryQueryParams(baseParams) {
    const params = new URLSearchParams(baseParams.toString());
    params.set('interval', state.currentInterval);
    return params;
}

async function fetchJson(path) {
    const response = await fetch(`${API_BASE_URL}${path}`);
    if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
    }
    return response.json();
}

function populateSelect(id, values, defaultLabel) {
    const select = document.getElementById(id);
    const currentValue = select.value;
    const options = [`<option value="">${defaultLabel}</option>`];

    values.forEach(value => {
        const normalized = String(value);
        options.push(`<option value="${escapeHtml(normalized)}">${escapeHtml(formatFilterValue(id, normalized))}</option>`);
    });

    select.innerHTML = options.join('');
    if (values.map(String).includes(currentValue)) {
        select.value = currentValue;
    }
}

function renderStats(stats) {
    setText('stat-total-reports', formatNumber(stats.total_reports));
    setText('stat-total-devices', formatNumber(stats.total_devices));
    setText('stat-total-combats', formatNumber(stats.total_combat_count));
    setText('stat-total-items', formatNumber(stats.total_item_amount));
    setText('stat-item-types', formatNumber(stats.total_item_types));
}

function renderItems(payload) {
    const tbody = document.getElementById('items-body');
    tbody.innerHTML = '';

    const items = Array.isArray(payload) ? payload : (payload.items || []);
    const total = Array.isArray(payload) ? items.length : (payload.total || items.length);
    setText('items-summary', `共 ${formatNumber(total)} 个物品条目，按总掉落数量排序`);

    if (items.length === 0) {
        tbody.innerHTML = `<tr class="table-empty"><td colspan="7">暂无数据</td></tr>`;
        return;
    }

    items.forEach((item, index) => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${index + 1}</td>
            <td>${escapeHtml(item.item || '-')}</td>
            <td class="numeric-cell">${formatNumber(item.total_amount)}</td>
            <td class="numeric-cell">${formatNumber(item.drop_reports)}</td>
            <td class="numeric-cell">${formatNumber(item.meow_amount)}</td>
            <td class="numeric-cell">${formatNumber(item.normal_amount)}</td>
            <td class="numeric-cell">${formatDecimal(item.avg_per_combat)}</td>
        `;
        tbody.appendChild(tr);
    });
}

function renderHistory(payload) {
    const tbody = document.getElementById('history-body');
    tbody.innerHTML = '';

    const history = Array.isArray(payload) ? payload : (payload.history || []);

    if (history.length === 0) {
        tbody.innerHTML = `<tr class="table-empty"><td colspan="5">暂无数据</td></tr>`;
        return;
    }

    history.forEach(record => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${escapeHtml(record.date || record.month || '-')}</td>
            <td class="numeric-cell">${formatNumber(record.report_count)}</td>
            <td class="numeric-cell">${formatNumber(record.combat_count)}</td>
            <td class="numeric-cell">${formatNumber(record.item_amount)}</td>
            <td class="numeric-cell">${formatNumber(record.device_count)}</td>
        `;
        tbody.appendChild(tr);
    });
}

function updateIntervalBadge() {
    setText('history-interval-badge', state.currentInterval === 'month' ? '按月统计' : '按日统计');
}

function showLoading() {
    document.getElementById('loading').style.display = 'block';
    document.getElementById('content').style.display = 'none';
    document.getElementById('error-state').style.display = 'none';
}

function showContent() {
    document.getElementById('loading').style.display = 'none';
    document.getElementById('content').style.display = 'block';
    document.getElementById('error-state').style.display = 'none';
}

function showError(message) {
    document.getElementById('loading').style.display = 'none';
    document.getElementById('content').style.display = 'none';
    const errorState = document.getElementById('error-state');
    errorState.textContent = message;
    errorState.style.display = 'block';
}

function clearError() {
    document.getElementById('error-state').style.display = 'none';
}

function setText(id, value) {
    document.getElementById(id).textContent = value;
}

function formatNumber(value) {
    return NUMBER_FORMAT.format(Math.round(Number(value) || 0));
}

function formatDecimal(value) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) {
        return '0.00';
    }
    return numeric.toFixed(2);
}

function formatFilterValue(id, value) {
    if (id === 'hazard_level') {
        return `等级 ${value}`;
    }
    return value;
}

function escapeHtml(value) {
    return String(value)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}
