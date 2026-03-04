package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ---- 大盘 SSE 广播器 ----

type DashboardBroadcaster struct {
	mu      sync.RWMutex
	clients map[chan struct{}]struct{}
}

var dashboardBroadcaster = &DashboardBroadcaster{
	clients: make(map[chan struct{}]struct{}),
}

func (b *DashboardBroadcaster) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *DashboardBroadcaster) Unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func (b *DashboardBroadcaster) NotifyUpdate() {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// NotifyDashboardUpdate 供外部（如聚合任务）调用
func NotifyDashboardUpdate() {
	dashboardBroadcaster.NotifyUpdate()
}

// ---- ReportStamina 接收单用户体力上报 ----

func ReportStamina(c *gin.Context) {
	var req models.StaminaReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	minuteKey := now.Format("2006-01-02T15:04")

	snapshot := models.StaminaSnapshot{
		DeviceID:  req.DeviceID,
		Stamina:   req.Stamina,
		MinuteKey: minuteKey,
	}

	if err := database.DB.Create(&snapshot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}

	// 通知 SSE 客户端
	dashboardBroadcaster.NotifyUpdate()

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "体力数据已上报",
		"device_id":  req.DeviceID,
		"stamina":    req.Stamina,
		"minute_key": minuteKey,
	})
}

// ---- GetStaminaKline 查询 K 线数据 ----

func GetStaminaKline(c *gin.Context) {
	period := c.DefaultQuery("period", "1m")    // 1m, 5m, 1h, 1d
	rangeStr := c.DefaultQuery("range", "day")   // day, week, month

	// 计算时间范围
	var since time.Time
	now := time.Now()
	switch rangeStr {
	case "day":
		since = now.Add(-24 * time.Hour)
	case "week":
		since = now.Add(-7 * 24 * time.Hour)
	case "month":
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = now.Add(-24 * time.Hour)
	}

	sinceKey := since.Format("2006-01-02T15:04")

	var data []models.StaminaOHLCV
	err := database.DB.
		Where("period = ? AND minute_key >= ?", period, sinceKey).
		Order("minute_key ASC").
		Find(&data).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   data,
		"period": period,
		"range":  rangeStr,
		"count":  len(data),
	})
}

// ---- GetStaminaLatest 获取当前最新大盘汇总 ----

func GetStaminaLatest(c *gin.Context) {
	// 获取最新一条 1m 级别 OHLCV
	var latest models.StaminaOHLCV
	err := database.DB.
		Where("period = ?", "1m").
		Order("minute_key DESC").
		First(&latest).Error

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"current_total":  0,
			"change":         0,
			"change_percent": 0,
			"reported_count": 0,
			"filled_count":   0,
			"minute_key":     "",
			"top_users":      []gin.H{},
		})
		return
	}

	// 获取前一条计算涨跌幅
	var prev models.StaminaOHLCV
	database.DB.
		Where("period = ? AND minute_key < ?", "1m", latest.MinuteKey).
		Order("minute_key DESC").
		First(&prev)

	change := latest.Close - prev.Close
	changePct := 0.0
	if prev.Close > 0 {
		changePct = change / prev.Close * 100
	}

	// 获取 Top 10 用户贡献排行
	type UserContribution struct {
		DeviceID string  `json:"device_id"`
		Username string  `json:"username"`
		Stamina  float64 `json:"stamina"`
	}

	var topUsers []UserContribution
	database.DB.Table("stamina_snapshots s").
		Select("SUBSTR(s.device_id, 1, 8) as device_id, COALESCE(u.username, '未知指挥官') as username, s.stamina").
		Joins("LEFT JOIN user_profiles u ON u.device_id = s.device_id").
		Where("s.minute_key = ?", latest.MinuteKey).
		Order("s.stamina DESC").
		Limit(10).
		Scan(&topUsers)

	c.JSON(http.StatusOK, gin.H{
		"current_total":  latest.Close,
		"open":           latest.Open,
		"high":           latest.High,
		"low":            latest.Low,
		"close":          latest.Close,
		"volume":         latest.Volume,
		"change":         change,
		"change_percent": changePct,
		"reported_count": latest.ReportedCount,
		"filled_count":   latest.FilledCount,
		"minute_key":     latest.MinuteKey,
		"top_users":      topUsers,
	})
}

// ---- StreamStaminaDashboard SSE 实时推送 ----

func StreamStaminaDashboard(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	updateCh := dashboardBroadcaster.Subscribe()
	defer dashboardBroadcaster.Unsubscribe(updateCh)

	clientGone := c.Request.Context().Done()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// 立即发送当前数据
	sendDashboardEvent(c)

	for {
		select {
		case <-clientGone:
			return
		case <-updateCh:
			sendDashboardEvent(c)
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		}
	}
}

func sendDashboardEvent(c *gin.Context) {
	// 获取最新汇总数据
	var latest models.StaminaOHLCV
	err := database.DB.
		Where("period = ?", "1m").
		Order("minute_key DESC").
		First(&latest).Error

	data := gin.H{}
	if err == nil {
		var prev models.StaminaOHLCV
		database.DB.
			Where("period = ? AND minute_key < ?", "1m", latest.MinuteKey).
			Order("minute_key DESC").
			First(&prev)

		change := latest.Close - prev.Close
		changePct := 0.0
		if prev.Close > 0 {
			changePct = change / prev.Close * 100
		}

		data = gin.H{
			"current_total":  latest.Close,
			"change":         change,
			"change_percent": changePct,
			"reported_count": latest.ReportedCount,
			"filled_count":   latest.FilledCount,
			"minute_key":     latest.MinuteKey,
			"volume":         latest.Volume,
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "data: %s\n\n", jsonData)
	c.Writer.Flush()
}

// ---- 聚合相关工具函数（供后台任务调用） ----

// AggregateMinute 执行一次分钟级聚合
func AggregateMinute(minuteKey string) {
	// 1. 获取所有曾经上报过的用户
	var allDeviceIDs []string
	database.DB.Model(&models.StaminaSnapshot{}).
		Distinct("device_id").
		Pluck("device_id", &allDeviceIDs)

	if len(allDeviceIDs) == 0 {
		return
	}

	// 2. 对每个用户获取当前分钟或最近的体力值
	totalStamina := 0.0
	reportedCount := 0
	filledCount := 0

	for _, deviceID := range allDeviceIDs {
		var snapshot models.StaminaSnapshot

		// 先查当前分钟是否有上报
		err := database.DB.
			Where("device_id = ? AND minute_key = ?", deviceID, minuteKey).
			Order("created_at DESC").
			First(&snapshot).Error

		if err == nil {
			// 有上报数据
			totalStamina += snapshot.Stamina
			reportedCount++
			continue
		}

		// 没有 → 向前填充 (Last Known Value)
		err = database.DB.
			Where("device_id = ? AND minute_key < ?", deviceID, minuteKey).
			Order("minute_key DESC, created_at DESC").
			First(&snapshot).Error

		if err == nil {
			totalStamina += snapshot.Stamina
			filledCount++
		}
		// 从未上报过的用户贡献 0，不参与求和
	}

	// 3. 查看该分钟是否已有 OHLCV 记录
	var existing models.StaminaOHLCV
	err := database.DB.Where("minute_key = ? AND period = ?", minuteKey, "1m").First(&existing).Error

	if err != nil {
		// 新建
		ohlcv := models.StaminaOHLCV{
			MinuteKey:     minuteKey,
			Period:        "1m",
			Open:          totalStamina,
			High:          totalStamina,
			Low:           totalStamina,
			Close:         totalStamina,
			Volume:        totalStamina,
			ReportedCount: reportedCount,
			FilledCount:   filledCount,
		}
		database.DB.Create(&ohlcv)
	} else {
		// 更新
		if totalStamina > existing.High {
			existing.High = totalStamina
		}
		if totalStamina < existing.Low {
			existing.Low = totalStamina
		}
		existing.Close = totalStamina
		existing.Volume = totalStamina
		existing.ReportedCount = reportedCount
		existing.FilledCount = filledCount
		database.DB.Save(&existing)
	}

	log.Printf("[STAMINA] Aggregated minute=%s total=%.0f reported=%d filled=%d",
		minuteKey, totalStamina, reportedCount, filledCount)
}

// AggregateHigherPeriods 聚合高级别周期（5m, 1h, 1d）
func AggregateHigherPeriods() {
	now := time.Now()

	// 5 分钟
	aggregatePeriod("5m", now, 5*time.Minute, "2006-01-02T15:04")
	// 1 小时
	aggregatePeriod("1h", now, 1*time.Hour, "2006-01-02T15")
	// 1 天
	aggregatePeriod("1d", now, 24*time.Hour, "2006-01-02")
}

func aggregatePeriod(period string, now time.Time, duration time.Duration, keyFormat string) {
	// 当前周期的 key
	var periodKey string
	switch period {
	case "5m":
		// 向下取整到 5 分钟
		minute := now.Minute()
		alignedMinute := (minute / 5) * 5
		aligned := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), alignedMinute, 0, 0, now.Location())
		periodKey = aligned.Format("2006-01-02T15:04")
	case "1h":
		periodKey = now.Format("2006-01-02T15")
	case "1d":
		periodKey = now.Format("2006-01-02")
	}

	// 查找该周期内所有 1m 级别数据
	var minuteData []models.StaminaOHLCV
	var startKey string

	switch period {
	case "5m":
		minute := now.Minute()
		alignedMinute := (minute / 5) * 5
		aligned := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), alignedMinute, 0, 0, now.Location())
		startKey = aligned.Format("2006-01-02T15:04")
	case "1h":
		startKey = now.Format("2006-01-02T15") + ":00"
	case "1d":
		startKey = now.Format("2006-01-02") + "T00:00"
	}

	database.DB.
		Where("period = ? AND minute_key >= ? AND minute_key <= ?", "1m", startKey, now.Format("2006-01-02T15:04")).
		Order("minute_key ASC").
		Find(&minuteData)

	if len(minuteData) == 0 {
		return
	}

	open := minuteData[0].Open
	closeVal := minuteData[len(minuteData)-1].Close
	high := minuteData[0].High
	low := minuteData[0].Low
	totalVolume := 0.0
	totalReported := 0
	totalFilled := 0

	for _, d := range minuteData {
		if d.High > high {
			high = d.High
		}
		if d.Low < low {
			low = d.Low
		}
		totalVolume += d.Volume
		totalReported += d.ReportedCount
		totalFilled += d.FilledCount
	}

	// Upsert
	var existing models.StaminaOHLCV
	err := database.DB.Where("minute_key = ? AND period = ?", periodKey, period).First(&existing).Error
	if err != nil {
		ohlcv := models.StaminaOHLCV{
			MinuteKey:     periodKey,
			Period:        period,
			Open:          open,
			High:          high,
			Low:           low,
			Close:         closeVal,
			Volume:        totalVolume,
			ReportedCount: totalReported,
			FilledCount:   totalFilled,
		}
		database.DB.Create(&ohlcv)
	} else {
		existing.Open = open
		existing.High = high
		existing.Low = low
		existing.Close = closeVal
		existing.Volume = totalVolume
		existing.ReportedCount = totalReported
		existing.FilledCount = totalFilled
		database.DB.Save(&existing)
	}
}
