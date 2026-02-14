package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"alas-cloud/internal/utils"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// resolveFullDeviceID 将截断的 device_id 解析为完整的 device_id
// 前端排行榜只显示前 8 位，所以举报/封禁传入的可能是部分 ID
func resolveFullDeviceID(partialID string) string {
	var fullID string
	err := database.DB.Table("telemetry_data").
		Select("device_id").
		Where("device_id LIKE ?", partialID+"%").
		Limit(1).
		Row().Scan(&fullID)
	if err != nil {
		// 如果找不到，返回原始值（可能本身就是完整 ID）
		return partialID
	}
	return fullID
}

// ReportUserRequest 举报请求
type ReportUserRequest struct {
	TargetID string `json:"target_id" binding:"required"`
	Reason   string `json:"reason"`
}

// ReportUser 处理举报请求
func ReportUser(c *gin.Context) {
	var req ReportUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解析完整 DeviceID (前端传来的可能是截断的前 8 位)
	fullTargetID := resolveFullDeviceID(req.TargetID)
	log.Printf("[REPORT] Resolved target_id: %s -> %s", req.TargetID, fullTargetID)

	// Check if reporter is Admin (One Vote Veto)
	isAdmin := false
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if _, err := utils.ParseToken(tokenStr); err == nil {
			isAdmin = true
		}
	}

	reporterID := c.ClientIP()

	// 检查是否已经举报过 (用完整 ID 检查)
	// 如果是管理员，允许重复举报（或者直接忽略此检查进入封禁流程？）
	// 为了简单，我们仍然检查数据库，但如果管理员想强制封禁，通常会用 Direct Ban
	// 这里的一票否决更多是方便管理员在排行榜上顺手点一下
	var count int64
	database.DB.Model(&models.Report{}).
		Where("target_id = ? AND reporter_id = ?", fullTargetID, reporterID).
		Count(&count)

	if count > 0 && !isAdmin {
		c.JSON(http.StatusConflict, gin.H{"error": "You have already reported this user."})
		return
	}

	// 创建举报记录 (存储完整 ID)
	report := models.Report{
		TargetID:   fullTargetID,
		ReporterID: reporterID,
		Reason:     req.Reason,
	}

	// 如果管理员重复举报，可能导致主键冲突或者无意义数据，这里 `Create` 应该没问题(除非有唯一索引)
	// UserProfile/Telemetry 都没有唯一索引限制 target_id+reporter_id
	if err := database.DB.Create(&report).Error; err != nil {
		// 忽略重复创建错误
		log.Printf("Failed to create report record: %v", err)
	}

	// 检查是否达到封禁阈值 (5票) 或 管理员
	var totalReports int64
	database.DB.Model(&models.Report{}).Where("target_id = ?", fullTargetID).Count(&totalReports)

	if totalReports > 5 || isAdmin {
		reason := "System: Automatic ban due to excessive reports"
		if isAdmin {
			reason = "Admin: Immediate ban via report (One Vote Veto)"
		}

		if err := banUser(fullTargetID, reason); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "Report submitted. User has been banned.",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Report submitted."})
}

// banUser 执行封禁逻辑 (事务)
func banUser(targetID, reason string) error {
	// 确保使用完整 ID
	fullID := resolveFullDeviceID(targetID)
	log.Printf("[BAN] Banning user: input=%s, resolved=%s", targetID, fullID)

	return database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 获取用户信息 (为了获取 IP 和 Username)
		var telem models.TelemetryData
		var profile models.UserProfile

		tx.Where("device_id = ?", fullID).Order("updated_at desc").First(&telem)
		tx.Where("device_id = ?", fullID).First(&profile)

		username := profile.Username
		if username == "" {
			username = fullID
		}

		ip := telem.IPAddress

		// 2. 创建封禁记录 (存储完整 ID)
		bannedUser := models.BannedUser{
			DeviceID:  fullID,
			IPAddress: ip,
			Username:  username,
			Reason:    reason,
		}
		if err := tx.Create(&bannedUser).Error; err != nil {
			return err
		}

		// 3. 删除用户数据 (使用原生 SQL 确保删除所有匹配行)
		if result := tx.Exec("DELETE FROM user_profiles WHERE device_id = ?", fullID); result.Error != nil {
			return result.Error
		} else {
			log.Printf("[BAN] Deleted %d rows from user_profiles for device_id=%s", result.RowsAffected, fullID)
		}

		if result := tx.Exec("DELETE FROM telemetry_data WHERE device_id = ?", fullID); result.Error != nil {
			return result.Error
		} else {
			log.Printf("[BAN] Deleted %d rows from telemetry_data for device_id=%s", result.RowsAffected, fullID)
		}

		// 4. 删除举报记录
		if result := tx.Exec("DELETE FROM reports WHERE target_id = ?", fullID); result.Error != nil {
			return result.Error
		} else {
			log.Printf("[BAN] Deleted %d rows from reports for target_id=%s", result.RowsAffected, fullID)
		}

		return nil
	})
}

// GetReportedUsersResponse 响应结构
type GetReportedUsersResponse struct {
	TargetID         string `json:"target_id"`
	Username         string `json:"username"`
	ReportCount      int64  `json:"report_count"`
	LatestReason     string `json:"latest_reason"`
	BattleRounds     int    `json:"battle_rounds"`
	NetStaminaGain   int    `json:"net_stamina_gain"`
	AkashiEncounters int    `json:"akashi_encounters"`
}

// GetReportedUsers 获取被举报用户列表
func GetReportedUsers(c *gin.Context) {
	var results []GetReportedUsersResponse

	// 使用子查询避免 JOIN 笛卡尔积导致数据膨胀，与排行榜逻辑一致
	err := database.DB.Table("reports").
		Select(
			"reports.target_id, "+
			"count(reports.id) as report_count, "+
			"MAX(reports.reason) as latest_reason, "+
			"COALESCE((SELECT username FROM user_profiles WHERE device_id = reports.target_id), 'Unknown') as username, "+
			"COALESCE((SELECT SUM(battle_rounds) FROM telemetry_data WHERE device_id = reports.target_id), 0) as battle_rounds, "+
			"COALESCE((SELECT SUM(net_stamina_gain) - SUM(battle_rounds * 5) FROM telemetry_data WHERE device_id = reports.target_id), 0) as net_stamina_gain, "+
			"COALESCE((SELECT SUM(akashi_encounters) FROM telemetry_data WHERE device_id = reports.target_id), 0) as akashi_encounters",
		).
		Group("reports.target_id").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reported users"})
		return
	}

	c.JSON(http.StatusOK, results)
}

// GetBannedUsers 获取封禁用户列表
func GetBannedUsers(c *gin.Context) {
	var users []models.BannedUser
	if err := database.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch banned users"})
		return
	}
	c.JSON(http.StatusOK, users)
}

// UnbanRequest 解封请求
type UnbanRequest struct {
	TargetID string `json:"target_id" binding:"required"`
}

// UnbanUser 解封用户
func UnbanUser(c *gin.Context) {
	var req UnbanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 执行解封 (banned_users 中已经存储了完整 ID，直接精确匹配)
	result := database.DB.Exec("DELETE FROM banned_users WHERE device_id = ?", req.TargetID)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unban user"})
		return
	}

	log.Printf("[UNBAN] Deleted %d rows from banned_users for device_id=%s", result.RowsAffected, req.TargetID)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found in ban list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "User unbanned"})
}

// BanRequest 封禁请求 (管理员)
type BanRequest struct {
	TargetID string `json:"target_id" binding:"required"`
	Reason   string `json:"reason"`
}

// DirectBanUser 直接封禁用户 (管理员)
func DirectBanUser(c *gin.Context) {
	var req BanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "Admin: Manual ban"
	}

	// 执行封禁 (banUser 内部会自动解析完整 ID)
	if err := banUser(req.TargetID, reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ban user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "User banned successfully"})
}

// DismissRequest 撤销举报/清空举报请求 (管理员)
type DismissRequest struct {
	TargetID string `json:"target_id" binding:"required"`
}

// DismissReport 管理员一票否决：驳回举报（清空对该用户的所有举报）
func DismissReport(c *gin.Context) {
	var req DismissRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fullID := resolveFullDeviceID(req.TargetID)

	// Admin only: Delete all reports for this target
	// 使用原生 SQL 删除
	result := database.DB.Exec("DELETE FROM reports WHERE target_id = ?", fullID)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dismiss reports"})
		return
	}

	log.Printf("[DISMISS] Admin dismissed %d reports for target %s", result.RowsAffected, fullID)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Reports dismissed"})
}

// UndoReportRequest 用户撤销自己的举报
type UndoReportRequest struct {
	TargetID string `json:"target_id" binding:"required"`
}

// UndoReport 用户撤销自己的举报
func UndoReport(c *gin.Context) {
	var req UndoReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fullID := resolveFullDeviceID(req.TargetID)
	reporterID := c.ClientIP()

	// Delete only reports by this reporter
	result := database.DB.Exec("DELETE FROM reports WHERE target_id = ? AND reporter_id = ?", fullID, reporterID)
	
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to undo report"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Report undone"})
}
