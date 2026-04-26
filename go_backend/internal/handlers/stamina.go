package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

	snapshot := staminaReport{
		DeviceID:  req.DeviceID,
		Stamina:   req.Stamina,
		MinuteKey: minuteKey,
	}

	EnqueueStaminaWrite(snapshot)

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "体力数据已进入写入缓冲队列",
		"device_id":  req.DeviceID,
		"stamina":    req.Stamina,
		"minute_key": minuteKey,
	})
}

// ---- GetStaminaKline 查询 K 线数据 ----

func GetStaminaKline(c *gin.Context) {
	period := c.DefaultQuery("period", "1m")   // 1m, 5m, 1h, 1d
	rangeStr := c.DefaultQuery("range", "day") // day, week, month

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
	database.DB.Table("stamina_current s").
		Select("SUBSTRING(s.device_id, 1, 8) as device_id, COALESCE(u.username, '未知指挥官') as username, s.stamina").
		Joins("LEFT JOIN user_profiles u ON u.device_id = s.device_id").
		Where("s.minute_key <= ?", latest.MinuteKey).
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
	type aggregateResult struct {
		TotalStamina  float64 `gorm:"column:total_stamina"`
		ReportedCount int     `gorm:"column:reported_count"`
		FilledCount   int     `gorm:"column:filled_count"`
	}

	var aggregate aggregateResult
	err := database.DB.Raw(`
		SELECT
			COALESCE(SUM(stamina), 0) AS total_stamina,
			COUNT(*) FILTER (WHERE minute_key = ?) AS reported_count,
			COUNT(*) FILTER (WHERE minute_key < ?) AS filled_count
		FROM stamina_current
		WHERE minute_key <= ?
	`, minuteKey, minuteKey, minuteKey).Scan(&aggregate).Error
	if err != nil {
		log.Printf("[STAMINA] aggregate minute query failed for %s: %v", minuteKey, err)
		return
	}

	if aggregate.ReportedCount == 0 && aggregate.FilledCount == 0 {
		return
	}

	totalStamina := aggregate.TotalStamina
	reportedCount := aggregate.ReportedCount
	filledCount := aggregate.FilledCount

	// 3. 获取前一分钟的 Close 值作为本分钟的 Open（形成连续K线）
	var prevOHLCV models.StaminaOHLCV
	prevClose := totalStamina // 默认：无历史数据时 Open = Close
	database.DB.
		Where("period = ? AND minute_key < ?", "1m", minuteKey).
		Order("minute_key DESC").
		First(&prevOHLCV)
	if prevOHLCV.ID > 0 {
		prevClose = prevOHLCV.Close
	}

	openVal := prevClose
	highVal := totalStamina
	lowVal := totalStamina
	if openVal > highVal {
		highVal = openVal
	}
	if openVal < lowVal {
		lowVal = openVal
	}

	ohlcv := models.StaminaOHLCV{
		MinuteKey:     minuteKey,
		Period:        "1m",
		Open:          openVal,
		High:          highVal,
		Low:           lowVal,
		Close:         totalStamina,
		Volume:        totalStamina,
		ReportedCount: reportedCount,
		FilledCount:   filledCount,
	}
	database.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "minute_key"}, {Name: "period"}},
		DoUpdates: clause.Assignments(map[string]any{
			"high":           gormExpr("GREATEST(stamina_kline.high, EXCLUDED.high)"),
			"low":            gormExpr("LEAST(stamina_kline.low, EXCLUDED.low)"),
			"close":          ohlcv.Close,
			"volume":         ohlcv.Volume,
			"reported_count": ohlcv.ReportedCount,
			"filled_count":   ohlcv.FilledCount,
		}),
	}).Create(&ohlcv)

	if os.Getenv("VERBOSE_BUFFER_LOGS") == "true" {
		log.Printf("[STAMINA] Aggregated minute=%s open=%.0f close=%.0f reported=%d filled=%d",
			minuteKey, openVal, totalStamina, reportedCount, filledCount)
	}
}

type staminaReport struct {
	DeviceID  string
	Stamina   float64
	MinuteKey string
}

type staminaBuffer struct {
	mu            sync.Mutex
	pending       map[string]staminaReport
	flushInterval time.Duration
	flushSize     int
	flushSignal   chan struct{}
	stopCh        chan struct{}
	doneCh        chan struct{}
}

var staminaWriter *staminaBuffer

func InitStaminaWriter() {
	if staminaWriter != nil {
		return
	}

	staminaWriter = &staminaBuffer{
		pending:       make(map[string]staminaReport),
		flushInterval: loadStaminaFlushInterval(),
		flushSize:     loadStaminaFlushSize(),
		flushSignal:   make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go staminaWriter.run()
	log.Printf("[STAMINA] buffered writer started interval=%s flush_size=%d", staminaWriter.flushInterval, staminaWriter.flushSize)
}

func ShutdownStaminaWriter(ctx context.Context) error {
	if staminaWriter == nil {
		return nil
	}

	close(staminaWriter.stopCh)
	select {
	case <-staminaWriter.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func EnqueueStaminaWrite(report staminaReport) {
	if staminaWriter == nil {
		writeStaminaBatch([]staminaReport{report})
		return
	}
	staminaWriter.enqueue(report)
}

func (b *staminaBuffer) run() {
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	defer close(b.doneCh)

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.flushSignal:
			b.flush()
		case <-b.stopCh:
			b.flush()
			return
		}
	}
}

func (b *staminaBuffer) enqueue(report staminaReport) {
	b.mu.Lock()
	b.pending[staminaBufferKey(report)] = report
	shouldFlush := len(b.pending) >= b.flushSize
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushSignal <- struct{}{}:
		default:
		}
	}
}

func (b *staminaBuffer) flush() {
	batch := b.drain()
	if len(batch) == 0 {
		return
	}
	if err := writeStaminaBatch(batch); err != nil {
		log.Printf("[STAMINA] buffered flush failed, requeueing %d rows: %v", len(batch), err)
		b.requeue(batch)
		return
	}
	if os.Getenv("VERBOSE_BUFFER_LOGS") == "true" {
		log.Printf("[STAMINA] flushed %d buffered rows", len(batch))
	}
}

func (b *staminaBuffer) drain() []staminaReport {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pending) == 0 {
		return nil
	}
	batch := make([]staminaReport, 0, len(b.pending))
	for _, item := range b.pending {
		batch = append(batch, item)
	}
	b.pending = make(map[string]staminaReport)
	return batch
}

func (b *staminaBuffer) requeue(batch []staminaReport) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range batch {
		b.pending[staminaBufferKey(item)] = item
	}
}

func writeStaminaBatch(batch []staminaReport) error {
	currents := make([]models.StaminaCurrent, 0, len(batch))
	now := time.Now()
	keepSnapshots := os.Getenv("STAMINA_KEEP_SNAPSHOTS") == "true"
	var snapshots []models.StaminaSnapshot
	if keepSnapshots {
		snapshots = make([]models.StaminaSnapshot, 0, len(batch))
	}
	for _, item := range batch {
		if keepSnapshots {
			snapshots = append(snapshots, models.StaminaSnapshot{
				DeviceID:  item.DeviceID,
				Stamina:   item.Stamina,
				MinuteKey: item.MinuteKey,
				CreatedAt: now,
			})
		}
		currents = append(currents, models.StaminaCurrent{
			DeviceID:  item.DeviceID,
			Stamina:   item.Stamina,
			MinuteKey: item.MinuteKey,
			UpdatedAt: now,
		})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if keepSnapshots && len(snapshots) > 0 {
			if err := tx.CreateInBatches(snapshots, 500).Error; err != nil {
				return err
			}
		}

		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "device_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"stamina",
				"minute_key",
				"updated_at",
			}),
		}).CreateInBatches(currents, 500).Error
	})
	if err != nil {
		return err
	}

	dashboardBroadcaster.NotifyUpdate()
	return nil
}

func staminaBufferKey(report staminaReport) string {
	return report.DeviceID + "|" + report.MinuteKey
}

func loadStaminaFlushInterval() time.Duration {
	raw := os.Getenv("STAMINA_FLUSH_INTERVAL_SECONDS")
	if raw == "" {
		return 5 * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func loadStaminaFlushSize() int {
	raw := os.Getenv("STAMINA_FLUSH_BATCH_SIZE")
	if raw == "" {
		return 500
	}
	size, err := strconv.Atoi(raw)
	if err != nil || size <= 0 {
		return 500
	}
	return size
}

func gormExpr(sql string) clause.Expr {
	return clause.Expr{SQL: sql}
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
		periodKey = now.Format("2006-01-02T15") + ":00"
	case "1d":
		periodKey = now.Format("2006-01-02") + "T00:00"
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
