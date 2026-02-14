package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/middleware"
	"alas-cloud/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

// SubmitTelemetry 提交遥测数据
func SubmitTelemetry(c *gin.Context) {
	var req models.TelemetryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip := c.ClientIP()

	// 1. 校验 Device ID 格式
	match, _ := regexp.MatchString("^[a-fA-F0-9]{32,64}$", req.DeviceID)
	if !match {
		middleware.BanIP(ip)
		// 假装成功
		c.JSON(http.StatusOK, gin.H{
			"status": "success", "message": "遥测数据已保存",
			"device_id": req.DeviceID, "instance_id": req.InstanceID,
		})
		return
	}

	// 2. 校验数值不能为 0
	if req.AkashiEncounters == 0 || req.AkashiProbability == 0 || req.AverageStamina == 0 || req.NetStaminaGain == 0 {
		middleware.BanIP(ip)
		c.JSON(http.StatusOK, gin.H{
			"status": "success", "message": "遥测数据已保存",
			"device_id": req.DeviceID, "instance_id": req.InstanceID,
		})
		return
	}

	// 3. 校验战斗逻辑
	if req.BattleCount <= req.BattleRounds {
		middleware.BanIP(ip)
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

	// 使用 Upsert (Clause.OnConflict)
	// SQLite 支持 ON CONFLICT
	if err := database.DB.Where(&models.TelemetryData{DeviceID: req.DeviceID, InstanceID: req.InstanceID}).
		Assign(data).
		FirstOrCreate(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success", "message": "遥测数据已保存",
		"device_id": req.DeviceID, "instance_id": req.InstanceID,
	})
}

// buildStatsResponse 构建统计数据响应（复用于 REST 和 SSE）
func buildStatsResponse() gin.H {
	type Result struct {
		TotalDevices          int64   `json:"total_devices"`
		TotalBattleCount      int64   `json:"total_battle_count"`
		TotalBattleRounds     int64   `json:"total_battle_rounds"`
		TotalSortieCost       int64   `json:"total_sortie_cost"`
		TotalAkashiEncounters int64   `json:"total_akashi_encounters"`
		TotalStaminaGain      float64 `json:"total_stamina_gain"`
	}

	var res Result
	// GORM 聚合查询
	// 总获取体力 = Σ(遇见明石次数_i × 平均获取体力_i)
	database.DB.Model(&models.TelemetryData{}).Select(
		"count(id) as total_devices",
		"sum(battle_count) as total_battle_count",
		"sum(battle_rounds) as total_battle_rounds",
		"sum(sortie_cost) as total_sortie_cost",
		"sum(akashi_encounters) as total_akashi_encounters",
		"sum(akashi_encounters * average_stamina) as total_stamina_gain",
	).Scan(&res)

	// 遇见明石概率 = 总遇见明石次数 / 总战斗轮次
	avgAkashiProbability := 0.0
	if res.TotalBattleRounds > 0 {
		avgAkashiProbability = float64(res.TotalAkashiEncounters) / float64(res.TotalBattleRounds)
	}

	// 平均体力 = 总获取体力 / 总遇见明石次数
	avgStamina := 0.0
	if res.TotalAkashiEncounters > 0 {
		avgStamina = res.TotalStaminaGain / float64(res.TotalAkashiEncounters)
	}

	// 净赚体力 = 总获取体力 - 总战斗轮次 × 5（侵蚀一）
	netStaminaGain := res.TotalStaminaGain - float64(res.TotalBattleRounds)*5

	// 循环效率 = 净赚体力 / 出击消耗
	cycleEfficiency := 0.0
	if res.TotalSortieCost > 0 {
		cycleEfficiency = netStaminaGain / float64(res.TotalSortieCost)
	}

	return gin.H{
		"total_devices":           res.TotalDevices,
		"total_battle_count":      res.TotalBattleCount,
		"total_battle_rounds":     res.TotalBattleRounds,
		"total_sortie_cost":       res.TotalSortieCost,
		"total_akashi_encounters": res.TotalAkashiEncounters,
		"avg_akashi_probability":  avgAkashiProbability,
		"avg_stamina":             avgStamina,
		"total_stamina_gain":      res.TotalStaminaGain,
		"net_stamina_gain":        netStaminaGain,
		"cycle_efficiency":        cycleEfficiency,
	}
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
	c.Writer.Header().Set("X-Accel-Buffering", "no") // nginx 禁用缓冲

	clientGone := c.Request.Context().Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// 立即发送一次
	sendSSEEvent(c, buildStatsResponse())

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			sendSSEEvent(c, buildStatsResponse())
		}
	}
}

func sendSSEEvent(c *gin.Context, data gin.H) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "data: %s\n\n", jsonData)
	c.Writer.Flush()
}
