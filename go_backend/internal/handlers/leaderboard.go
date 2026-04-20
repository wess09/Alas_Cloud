package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// LeaderboardEntry 排行榜条目
type LeaderboardEntry struct {
	DeviceID         string `json:"device_id"`
	Username         string `json:"username"`
	BattleRounds     int    `json:"battle_rounds"`
	NetStaminaGain   int    `json:"net_stamina_gain"`
	AkashiEncounters int    `json:"akashi_encounters"`
	LastActive       string `json:"last_active"` // 最近一次上传数据的时间
}

// GetLeaderboard 获取排行榜数据 (支持分页)
func GetLeaderboard(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 100 {
		size = 100
	}

	offset := (page - 1) * size

	// 排序逻辑
	orderBy := "battle_rounds DESC"              // 默认
	sortType := c.DefaultQuery("sort", "rounds") // 默认改为 rounds
	if sortType == "stamina" {
		orderBy = "net_stamina_gain DESC"
	}

	// 筛选月份，默认当前月
	month := c.Query("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	serveCachedJSON(c, telemetryCacheKey("leaderboard", c), 15*time.Second, func() (any, error) {
		var results []LeaderboardEntry

		// 联合查询: 聚合 telemetry_data 并关联 user_profiles
		// 注意: SQLite 的 Group By 行为
		// 我们需要按 device_id 分组统计
		// fix: 隐藏 device_id，只返回前 8 位
		query := database.DB.Table("telemetry_data").
			Select("SUBSTRING(telemetry_data.device_id, 1, 8) as device_id, " +
				"COALESCE(user_profiles.username, '') as username, " +
				"SUM(telemetry_data.battle_rounds) as battle_rounds, " +
				"(SUM(telemetry_data.net_stamina_gain) - SUM(telemetry_data.battle_rounds * 5)) as net_stamina_gain, " +
				"SUM(telemetry_data.akashi_encounters) as akashi_encounters, " +
				"MAX(telemetry_data.updated_at) as last_active").
			Joins("LEFT JOIN user_profiles ON user_profiles.device_id = telemetry_data.device_id")

		if month != "all" {
			query = query.Where("telemetry_data.month = ?", month)
		}

		err := query.Group("telemetry_data.device_id, user_profiles.username").
			Order(orderBy).
			Limit(size).
			Offset(offset).
			Scan(&results).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard"})
			return nil, err
		}

		// 统计总数用于前端分页
		var total int64
		countQuery := database.DB.Table("telemetry_data")
		if month != "all" {
			countQuery = countQuery.Where("month = ?", month)
		}
		if err := countQuery.Distinct("device_id").Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count leaderboard"})
			return nil, err
		}

		return gin.H{
			"data":  results,
			"page":  page,
			"size":  size,
			"total": total,
		}, nil
	})
}

// UpdateUserProfileRequest 更新用户信息请求
type UpdateUserProfileRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
	Username string `json:"username" binding:"required,max=32"` // 限制用户名长度
}

// UpdateUserProfile 更新用户昵称
func UpdateUserProfile(c *gin.Context) {
	var req UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 简单的验证：确保 DeviceID 存在于 telemetry_data 中 (可选，防止垃圾数据)
	var count int64
	database.DB.Model(&models.TelemetryData{}).Where("device_id = ?", req.DeviceID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device ID not found in telemetry records"})
		return
	}

	// Upsert UserProfile using Save (which performs upsert based on primary key)
	// 防止 XSS: 移除 script 标签和危险字符
	safeUsername := strings.TrimSpace(req.Username)
	scriptRe := regexp.MustCompile(`(?i)<\s*/?script[^>]*>`)
	safeUsername = scriptRe.ReplaceAllString(safeUsername, "")
	dangerChars := strings.NewReplacer("&", "", `"`, "", "'", "", "<", "", ">", "")
	safeUsername = dangerChars.Replace(safeUsername)

	if safeUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名不能为空或仅包含非法字符"})
		return
	}

	profile := models.UserProfile{
		DeviceID: req.DeviceID,
		Username: safeUsername,
	}

	// 使用 Clauses 处理 MySQL 的 Upsert，显式指定只更新用户名和时间戳，避免覆盖 CreatedAt
	if err := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"username", "updated_at"}),
	}).Create(&profile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	invalidateTelemetryCache()

	c.JSON(http.StatusOK, gin.H{"status": "success", "username": req.Username})
}
