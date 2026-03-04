# 侵蚀体力大盘 – 客户端 API 文档

**Base URL:** `https://alas-apiv2.nanoda.work`

---

## 1. 上报体力数据

客户端应在每次获取到用户当前体力值时调用此接口。

```
POST /api/stamina/report
Content-Type: application/json
```

### 请求体

```json
{
    "device_id": "a1b2c3d4e5f6...",
    "stamina": 1520
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `device_id` | string | ✅ | 设备唯一标识（与遥测数据中的 device_id 一致） |
| `stamina` | float | ✅ | 当前侵蚀体力值（≥ 0） |

### 成功响应 (200)

```json
{
    "status": "success",
    "message": "体力数据已上报",
    "device_id": "a1b2c3d4e5f6...",
    "stamina": 1520,
    "minute_key": "2026-03-04T21:30"
}
```

### 错误响应

| 状态码 | 说明 |
|--------|------|
| 400 | 参数缺失或格式错误 |
| 500 | 服务端写入失败 |

### 调用建议

- **频率**：建议每 **1~2 分钟** 上报一次，与服务端聚合间隔（60 秒）对齐
- **时机**：每次刷图循环结束后、体力发生变化时上报
- **幂等性**：同一分钟内多次上报不会冲突，服务端会取最新值参与聚合

### Python 示例

```python
import requests

API_BASE = "https://alas-apiv2.nanoda.work"

def report_stamina(device_id: str, stamina: float):
    """上报当前体力到大盘"""
    try:
        resp = requests.post(
            f"{API_BASE}/api/stamina/report",
            json={
                "device_id": device_id,
                "stamina": stamina,
            },
            timeout=5,
        )
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        print(f"[STAMINA] 上报失败: {e}")
        return None
```

---

## 2. 查询 K 线数据（可选，前端已用）

```
GET /api/stamina/kline?period=1m&range=day
```

| 参数 | 类型 | 默认值 | 可选值 | 说明 |
|------|------|--------|--------|------|
| `period` | string | `1m` | `1m`, `5m`, `1h`, `1d` | 时间粒度 |
| `range` | string | `day` | `day`, `week`, `month` | 时间范围 |

### 响应

```json
{
    "data": [
        {
            "minute_key": "2026-03-04T21:30",
            "period": "1m",
            "open": 450,
            "high": 480,
            "low": 440,
            "close": 470,
            "volume": 470,
            "reported_count": 3,
            "filled_count": 2
        }
    ],
    "period": "1m",
    "range": "day",
    "count": 120
}
```

---

## 3. 查询最新汇总

```
GET /api/stamina/latest
```

### 响应

```json
{
    "current_total": 470,
    "open": 450,
    "high": 480,
    "low": 440,
    "close": 470,
    "volume": 470,
    "change": 20,
    "change_percent": 4.44,
    "reported_count": 3,
    "filled_count": 2,
    "minute_key": "2026-03-04T21:30",
    "top_users": [
        { "device_id": "a1b2c3d4", "username": "指挥官A", "stamina": 200 },
        { "device_id": "e5f6g7h8", "username": "指挥官B", "stamina": 150 }
    ]
}
```

---

## 数据补全逻辑说明

服务端每 60 秒执行一次聚合，逻辑如下：

1. 遍历所有曾上报过体力的用户
2. 若该用户**本分钟有上报** → 使用最新值，计入 `reported_count`
3. 若该用户**本分钟无上报** → 自动沿用其最近一次有效数据（Last Known Value），计入 `filled_count`
4. 若该用户**从未上报过** → 贡献 0，不参与求和
5. 所有用户体力之和 = 该分钟的大盘总量
