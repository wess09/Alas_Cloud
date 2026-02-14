package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"log"
	"net/http"

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

	reporterID := c.ClientIP()

	// 检查是否已经举报过 (用完整 ID 检查)
	var count int64
	database.DB.Model(&models.Report{}).
		Where("target_id = ? AND reporter_id = ?", fullTargetID, reporterID).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "You have already reported this user."})
		return
	}

	// 创建举报记录 (存储完整 ID)
	report := models.Report{
		TargetID:   fullTargetID,
		ReporterID: reporterID,
		Reason:     req.Reason,
	}

	if err := database.DB.Create(&report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit report"})
		return
	}

	// 检查是否达到封禁阈值 (5票)
	var totalReports int64
	database.DB.Model(&models.Report{}).Where("target_id = ?", fullTargetID).Count(&totalReports)

	if totalReports > 5 {
		if err := banUser(fullTargetID, "System: Automatic ban due to excessive reports"); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "Report submitted. User has been banned due to excessive reports.",
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
	TargetID     string `json:"target_id"`
	Username     string `json:"username"`
	ReportCount  int64  `json:"report_count"`
	LatestReason string `json:"latest_reason"`
}

// GetReportedUsers 获取被举报用户列表
func GetReportedUsers(c *gin.Context) {
	var results []GetReportedUsersResponse

	err := database.DB.Table("reports").
		Select("reports.target_id, count(reports.id) as report_count, MAX(reports.reason) as latest_reason, COALESCE(user_profiles.username, 'Unknown') as username").
		Joins("LEFT JOIN user_profiles ON user_profiles.device_id = reports.target_id").
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
