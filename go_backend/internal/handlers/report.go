package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

	// 获取举报人标识 (可以是 DeviceID 也可以是 IP，这里优先用 IP 防止刷票，或者结合 DeviceID)
	// 为了简单且有效，使用 ClientIP 作为主要判断依据，防止同一 IP 刷票。
	// 但如果前端能传 DeviceID 更好。这里假设 API 调用者是受信任的客户端，
	// 实际场景中，应该从 Context 中获取认证信息。
	// 由于目前没有严格的鉴权，我们使用 IP + UserAgent 生成一个指纹，或者简单点，直接用 IP。
	// 还可以要求请求头带上 reporter_device_id
	reporterID := c.ClientIP()
	
	// 检查是否已经举报过
	var count int64
	database.DB.Model(&models.Report{}).
		Where("target_id = ? AND reporter_id = ?", req.TargetID, reporterID).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "You have already reported this user."})
		return
	}

	// 创建举报记录
	report := models.Report{
		TargetID:   req.TargetID,
		ReporterID: reporterID,
		Reason:     req.Reason,
	}

	if err := database.DB.Create(&report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit report"})
		return
	}

	// 检查是否达到封禁阈值 (5票)
	var totalReports int64
	database.DB.Model(&models.Report{}).Where("target_id = ?", req.TargetID).Count(&totalReports)

	if totalReports > 5 {
		// 触发封禁
		if err := banUser(req.TargetID, "System: Automatic ban due to excessive reports"); err != nil {
			// 记录错误但不需要告诉前端失败，因为举报本身成功了
			// 实际生产中应该打 Log
			c.JSON(http.StatusOK, gin.H{
				"status": "success", 
				"message": "Report submitted. User has been banned due to excessive reports.",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Report submitted."})
}

// banUser 执行封禁逻辑 (事务)
func banUser(targetID, reason string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 获取用户信息 (为了获取 IP 和 Username)
		var telem models.TelemetryData
		var profile models.UserProfile
		
		// 尝试获取最新的一条遥测数据以获取 IP
		tx.Where("device_id = ?", targetID).Order("updated_at desc").First(&telem)
		
		// 尝试获取用户资料
		tx.Where("device_id = ?", targetID).First(&profile)

		username := profile.Username
		if username == "" {
			username = targetID // fallback
		}
		
		ip := telem.IPAddress // 可能是空，如果没有遥测数据

		// 2. 创建封禁记录
		bannedUser := models.BannedUser{
			DeviceID:  targetID,
			IPAddress: ip,
			Username:  username,
			Reason:    reason,
		}
		if err := tx.Create(&bannedUser).Error; err != nil {
			return err
		}

		// 3. 删除用户数据
		if err := tx.Where("device_id = ?", targetID).Delete(&models.UserProfile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", targetID).Delete(&models.TelemetryData{}).Error; err != nil {
			return err
		}

		// 4. (可选) 删除举报记录，或者保留作为历史
		// 为了保持表干净，这里选择删除
		if err := tx.Where("target_id = ?", targetID).Delete(&models.Report{}).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetReportedUsersResponse 响应结构
type GetReportedUsersResponse struct {
	TargetID    string `json:"target_id"`
	Username    string `json:"username"`
	ReportCount int64  `json:"report_count"`
	LatestReason string `json:"latest_reason"`
}

// GetReportedUsers 获取被举报用户列表
func GetReportedUsers(c *gin.Context) {
	var results []GetReportedUsersResponse

	// 使用原生 SQL 或 GORM 聚合查询
	// 选出 reports 表中 target_id，count(*)，以及 join user_profiles 获取 username
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

	// 执行解封
	if err := database.DB.Where("device_id = ?", req.TargetID).Delete(&models.BannedUser{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unban user"})
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

	// 执行封禁
	if err := banUser(req.TargetID, reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ban user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "User banned successfully"})
}
