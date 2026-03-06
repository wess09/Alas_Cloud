package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ---- SSE 广播器 ----

type StatsBroadcaster struct {
	mu      sync.RWMutex
	clients map[chan struct{}]struct{}
}

var statsBroadcaster = &StatsBroadcaster{
	clients: make(map[chan struct{}]struct{}),
}

// Subscribe 注册一个 SSE 客户端，返回通知 channel
func (b *StatsBroadcaster) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1) // buffer 1 防止阻塞
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe 移除一个 SSE 客户端
func (b *StatsBroadcaster) Unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// NotifyUpdate 通知所有 SSE 客户端有新数据
func (b *StatsBroadcaster) NotifyUpdate() {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- struct{}{}:
		default:
			// channel 已有未消费的通知，跳过（合并多次更新）
		}
	}
}

// ---- Telemetry Handlers ----

// SubmitTelemetry 提交遥测数据
func SubmitTelemetry(c *gin.Context) {
	var req models.TelemetryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip := c.ClientIP()

	// 0. 检查是否被封禁
	var banCount int64
	database.DB.Model(&models.BannedUser{}).
		Where("device_id = ? OR ip_address = ?", req.DeviceID, ip).
		Count(&banCount)
	
	if banCount > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "User is banned from the leaderboard."})
		return
	}

	// 1. 校验 Device ID 格式
	match, _ := regexp.MatchString("^[a-fA-F0-9]{32,64}$", req.DeviceID)
	if !match {
		log.Printf("[TELEMETRY] Rejected: invalid device_id format, ip=%s, device_id=%s", ip, req.DeviceID)
		c.JSON(http.StatusOK, gin.H{
			"status": "success", "message": "遥测数据已保存",
			"device_id": req.DeviceID, "instance_id": req.InstanceID,
		})
		return
	}

	// 2. 校验数值不能为 0
	if req.AkashiEncounters == 0 || req.AkashiProbability == 0 || req.AverageStamina == 0 || req.NetStaminaGain == 0 {
		log.Printf("[TELEMETRY] Rejected: zero value fields, ip=%s, device_id=%s", ip, req.DeviceID)
		c.JSON(http.StatusOK, gin.H{
			"status": "success", "message": "遥测数据已保存",
			"device_id": req.DeviceID, "instance_id": req.InstanceID,
		})
		return
	}

	// 3. 校验战斗逻辑
	if req.BattleCount <= req.BattleRounds {
		log.Printf("[TELEMETRY] Rejected: battle_count(%d) <= battle_rounds(%d), ip=%s, device_id=%s", req.BattleCount, req.BattleRounds, ip, req.DeviceID)
		c.JSON(http.StatusOK, gin.H{
			"status": "success", "message": "遥测数据已保存",
			"device_id": req.DeviceID, "instance_id": req.InstanceID,
		})
		return
	}

	// 入库
	data := models.TelemetryData{
		DeviceID:          req.DeviceID,
		InstanceID:        req.InstanceID,
		IPAddress:         ip,
		Month:             req.Month,
		BattleCount:       req.BattleCount,
		BattleRounds:      req.BattleRounds,
		SortieCost:        req.SortieCost,
		AkashiEncounters:  req.AkashiEncounters,
		AkashiProbability: req.AkashiProbability,
		AverageStamina:    req.AverageStamina,
		NetStaminaGain:    req.NetStaminaGain,
	}

	if err := database.DB.Where(&models.TelemetryData{DeviceID: req.DeviceID, InstanceID: req.InstanceID, Month: req.Month}).
		Assign(data).
		FirstOrCreate(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 通知所有 SSE 客户端
	statsBroadcaster.NotifyUpdate()

	c.JSON(http.StatusOK, gin.H{
		"status": "success", "message": "遥测数据已保存",
		"device_id": req.DeviceID, "instance_id": req.InstanceID,
	})
}

var (
	cachedStats     gin.H
	cachedStatsTime time.Time
	statsMutex      sync.RWMutex
)

// buildStatsResponse 构建统计数据响应（复用于 REST 和 SSE）
func buildStatsResponse() gin.H {
	statsMutex.RLock()
	if time.Since(cachedStatsTime) < 5*time.Second && cachedStats != nil {
		defer statsMutex.RUnlock()
		return cachedStats
	}
	statsMutex.RUnlock()

	statsMutex.Lock()
	defer statsMutex.Unlock()
	
	// Double-check after acquiring write lock
	if time.Since(cachedStatsTime) < 5*time.Second && cachedStats != nil {
		return cachedStats
	}
	type Result struct {
		TotalDevices          int64   `json:"total_devices"`
		TotalBattleCount      int64   `json:"total_battle_count"`
		TotalBattleRounds     int64   `json:"total_battle_rounds"`
		TotalAkashiEncounters int64   `json:"total_akashi_encounters"`
		TotalStaminaGain      float64 `json:"total_stamina_gain"`
	}

	var res Result
	// Dashboard 仅统计最近 24 小时的活跃数据
	cutoff := time.Now().Add(-24 * time.Hour)
	database.DB.Model(&models.TelemetryData{}).
		Where("updated_at > ?", cutoff).
		Select(
			"count(id) as total_devices",
			"sum(battle_count) as total_battle_count",
			"sum(battle_rounds) as total_battle_rounds",
			"sum(akashi_encounters) as total_akashi_encounters",
			"sum(akashi_encounters * average_stamina) as total_stamina_gain",
		).Scan(&res)

	// 出击消耗 = 总战斗轮次 × 5（侵蚀一）
	totalSortieCost := res.TotalBattleRounds * 5

	avgAkashiProbability := 0.0
	if res.TotalBattleRounds > 0 {
		avgAkashiProbability = float64(res.TotalAkashiEncounters) / float64(res.TotalBattleRounds)
	}

	avgStamina := 0.0
	if res.TotalAkashiEncounters > 0 {
		avgStamina = res.TotalStaminaGain / float64(res.TotalAkashiEncounters)
	}

	netStaminaGain := res.TotalStaminaGain - float64(res.TotalBattleRounds)*5

	cycleEfficiency := 0.0
	if totalSortieCost > 0 {
		cycleEfficiency = netStaminaGain / float64(totalSortieCost)
	}

	cachedStats = gin.H{
		"total_devices":           res.TotalDevices,
		"total_battle_count":      res.TotalBattleCount,
		"total_battle_rounds":     res.TotalBattleRounds,
		"total_sortie_cost":       totalSortieCost,
		"total_akashi_encounters": res.TotalAkashiEncounters,
		"avg_akashi_probability":  avgAkashiProbability,
		"avg_stamina":             avgStamina,
		"total_stamina_gain":      res.TotalStaminaGain,
		"net_stamina_gain":        netStaminaGain,
		"cycle_efficiency":        cycleEfficiency,
	}
	cachedStatsTime = time.Now()

	return cachedStats
}

// GetTelemetryStats 获取聚合统计（REST，保留兼容）
func GetTelemetryStats(c *gin.Context) {
	c.JSON(http.StatusOK, buildStatsResponse())
}

// StreamTelemetryStats SSE 实时推送统计数据
func StreamTelemetryStats(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// 订阅广播
	updateCh := statsBroadcaster.Subscribe()
	defer statsBroadcaster.Unsubscribe(updateCh)

	clientGone := c.Request.Context().Done()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// 使用节流器，最快 5 秒推送一次
	throttleTicker := time.NewTicker(5 * time.Second)
	defer throttleTicker.Stop()

	needsUpdate := false

	// 立即发送当前数据
	sendSSEEvent(c, buildStatsResponse())

	for {
		select {
		case <-clientGone:
			return
		case <-updateCh:
			// 标记有更新，但不立即推流（防洪泛）
			needsUpdate = true
		case <-throttleTicker.C:
			// 如果期间内收到更新信号，则推流一次
			if needsUpdate {
				sendSSEEvent(c, buildStatsResponse())
				needsUpdate = false
			}
		case <-heartbeat.C:
			// 心跳保活，防止代理/CDN 超时断开
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		}
	}
}

func sendSSEEvent(c *gin.Context, data gin.H) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "data: %s\n\n", jsonData)
}

// GetTelemetryHistory 获取指定设备的历史遥测数据并进行累计统计
func GetTelemetryHistory(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}

	// 解析完整的 Device ID
	fullID := resolveFullDeviceID(deviceID)

	type MonthlyAggr struct {
		Month            string    `json:"month"`
		BattleCount      int       `json:"battle_count"`
		BattleRounds     int       `json:"battle_rounds"`
		SortieCost       int       `json:"sortie_cost"`
		AkashiEncounters int       `json:"akashi_encounters"`
		NetStaminaGain   int       `json:"net_stamina_gain"`
		TotalStaminaSum  float64   `json:"-"`
		AverageStamina   float64   `json:"average_stamina"`
		UpdatedAt        time.Time `json:"updated_at"`
	}

	var monthlyAggrs []MonthlyAggr

	err := database.DB.Table("telemetry_data").
		Select(`
			month, 
			SUM(battle_count) as battle_count,
			SUM(battle_rounds) as battle_rounds,
			SUM(sortie_cost) as sortie_cost,
			SUM(akashi_encounters) as akashi_encounters,
			(SUM(net_stamina_gain) - SUM(battle_rounds * 5)) as net_stamina_gain,
			SUM(average_stamina * akashi_encounters) as total_stamina_sum,
			MAX(updated_at) as updated_at
		`).
		Where("device_id = ?", fullID).
		Group("month").
		Order("month DESC").
		Scan(&monthlyAggrs).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch history"})
		return
	}

	// 获取用户名（如果有）
	var profile models.UserProfile
	database.DB.Where("device_id = ?", fullID).First(&profile)
	username := profile.Username
	if username == "" {
		username = "未知指挥官"
	}

	// 累计聚合
	var totalBattleCount, totalBattleRounds, totalSortieCost int
	var totalAkashiEncounters, totalNetStaminaGain int
	var totalStaminaSum float64 // 用于计算加权平均

	for i := range monthlyAggrs {
		// 计算每月的平均体力
		if monthlyAggrs[i].AkashiEncounters > 0 {
			monthlyAggrs[i].AverageStamina = monthlyAggrs[i].TotalStaminaSum / float64(monthlyAggrs[i].AkashiEncounters)
		} else {
			monthlyAggrs[i].AverageStamina = 0
		}

		totalBattleCount += monthlyAggrs[i].BattleCount
		totalBattleRounds += monthlyAggrs[i].BattleRounds
		totalSortieCost += monthlyAggrs[i].SortieCost
		totalAkashiEncounters += monthlyAggrs[i].AkashiEncounters
		totalNetStaminaGain += monthlyAggrs[i].NetStaminaGain
		totalStaminaSum += monthlyAggrs[i].TotalStaminaSum
	}

	avgAkashiProbability := 0.0
	if totalBattleRounds > 0 {
		avgAkashiProbability = float64(totalAkashiEncounters) / float64(totalBattleRounds)
	}

	avgStamina := 0.0
	if totalAkashiEncounters > 0 {
		avgStamina = totalStaminaSum / float64(totalAkashiEncounters)
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id": fullID,
		"username":  username,
		"total": gin.H{
			"battle_count":       totalBattleCount,
			"battle_rounds":      totalBattleRounds,
			"sortie_cost":        totalSortieCost,
			"akashi_encounters":  totalAkashiEncounters,
			"akashi_probability": avgAkashiProbability,
			"average_stamina":    avgStamina,
			"net_stamina_gain":   totalNetStaminaGain,
		},
		"history": monthlyAggrs,
	})
}

// GetGlobalTelemetryHistory 获取所有用户的历史遥测聚合数据
func GetGlobalTelemetryHistory(c *gin.Context) {
	// 获取所有用户的按月分组数据
	type MonthlyAggr struct {
		Month            string  `json:"month"`
		BattleCount      int     `json:"battle_count"`
		BattleRounds     int     `json:"battle_rounds"`
		SortieCost       int     `json:"sortie_cost"`
		AkashiEncounters int     `json:"akashi_encounters"`
		NetStaminaGain   int     `json:"net_stamina_gain"`
		TotalStaminaSum  float64 `json:"-"`
		AverageStamina   float64 `json:"average_stamina"`
	}

	var monthlyAggrs []MonthlyAggr

	err := database.DB.Table("telemetry_data").
		Select(`
			month, 
			SUM(battle_count) as battle_count,
			SUM(battle_rounds) as battle_rounds,
			SUM(sortie_cost) as sortie_cost,
			SUM(akashi_encounters) as akashi_encounters,
			(SUM(net_stamina_gain) - SUM(battle_rounds * 5)) as net_stamina_gain,
			SUM(average_stamina * akashi_encounters) as total_stamina_sum
		`).
		Group("month").
		Order("month DESC").
		Scan(&monthlyAggrs).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch global history"})
		return
	}

	var totalBattleCount, totalBattleRounds, totalSortieCost int
	var totalAkashiEncounters, totalNetStaminaGain int
	var totalStaminaSum float64

	for i := range monthlyAggrs {
		// 计算每月的平均体力
		if monthlyAggrs[i].AkashiEncounters > 0 {
			monthlyAggrs[i].AverageStamina = monthlyAggrs[i].TotalStaminaSum / float64(monthlyAggrs[i].AkashiEncounters)
		} else {
			monthlyAggrs[i].AverageStamina = 0
		}

		// 累计总计数据
		totalBattleCount += monthlyAggrs[i].BattleCount
		totalBattleRounds += monthlyAggrs[i].BattleRounds
		totalSortieCost += monthlyAggrs[i].SortieCost
		totalAkashiEncounters += monthlyAggrs[i].AkashiEncounters
		totalNetStaminaGain += monthlyAggrs[i].NetStaminaGain
		totalStaminaSum += monthlyAggrs[i].TotalStaminaSum
	}

	avgAkashiProbability := 0.0
	if totalBattleRounds > 0 {
		avgAkashiProbability = float64(totalAkashiEncounters) / float64(totalBattleRounds)
	}

	avgStamina := 0.0
	if totalAkashiEncounters > 0 {
		avgStamina = totalStaminaSum / float64(totalAkashiEncounters)
	}

	c.JSON(http.StatusOK, gin.H{
		"total": gin.H{
			"battle_count":       totalBattleCount,
			"battle_rounds":      totalBattleRounds,
			"sortie_cost":        totalSortieCost,
			"akashi_encounters":  totalAkashiEncounters,
			"akashi_probability": avgAkashiProbability,
			"average_stamina":    avgStamina,
			"net_stamina_gain":   totalNetStaminaGain,
		},
		"history": monthlyAggrs,
	})
}
