package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/middleware"
	"alas-cloud/internal/models"
	"net/http"
	"regexp"

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

// GetTelemetryStats 获取聚合统计
func GetTelemetryStats(c *gin.Context) {
	type Result struct {
		TotalDevices          int64   `json:"total_devices"`
		TotalBattleCount      int64   `json:"total_battle_count"`
		TotalBattleRounds     int64   `json:"total_battle_rounds"`
		TotalSortieCost       int64   `json:"total_sortie_cost"`
		TotalAkashiEncounters int64   `json:"total_akashi_encounters"`
		AvgStamina            float64 `json:"avg_stamina"`
		TotalNetStaminaGain   int64   `json:"total_net_stamina_gain"`
	}

	var res Result
	// GORM 聚合查询
	database.DB.Model(&models.TelemetryData{}).Select(
		"count(id) as total_devices",
		"sum(battle_count) as total_battle_count",
		"sum(battle_rounds) as total_battle_rounds",
		"sum(sortie_cost) as total_sortie_cost",
		"sum(akashi_encounters) as total_akashi_encounters",
		"avg(average_stamina) as avg_stamina",
		"sum(net_stamina_gain) as total_net_stamina_gain",
	).Scan(&res)

	avgAkashiProbability := 0.0
	if res.TotalBattleRounds > 0 {
		avgAkashiProbability = float64(res.TotalAkashiEncounters) / float64(res.TotalBattleRounds)
	}

	response := gin.H{
		"total_devices":           res.TotalDevices,
		"total_battle_count":      res.TotalBattleCount,
		"total_battle_rounds":     res.TotalBattleRounds,
		"total_sortie_cost":       res.TotalSortieCost,
		"total_akashi_encounters": res.TotalAkashiEncounters,
		"avg_akashi_probability":  avgAkashiProbability,
		"avg_stamina":             res.AvgStamina,
		"total_net_stamina_gain":  res.TotalNetStaminaGain,
	}

	c.JSON(http.StatusOK, response)
}
