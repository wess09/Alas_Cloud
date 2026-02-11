package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"alas-cloud/internal/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetLatestAnnouncement 获取最新公告
func GetLatestAnnouncement(c *gin.Context) {
	idStr := c.Query("id")

	var announcement models.Announcement
	result := database.DB.Where("is_active = ?", true).Order("id desc").First(&announcement)

	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	// 增量更新检查
	if idStr != "" && idStr == announcement.AnnouncementHash {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"announcementId": announcement.AnnouncementHash,
		"title":          announcement.Title,
		"content":        announcement.Content,
		"url":            announcement.URL,
	})
}

// CreateAnnouncement 创建公告
func CreateAnnouncement(c *gin.Context) {
	var req models.AnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash := utils.GenerateAnnouncementHash(req.Title, req.Content, req.URL)
	announcement := models.Announcement{
		AnnouncementHash: hash,
		Title:            req.Title,
		Content:          req.Content,
		URL:              req.URL,
		IsActive:         true,
	}

	if err := database.DB.Create(&announcement).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "announcement": announcement})
}

// ListAnnouncements 列出所有公告
func ListAnnouncements(c *gin.Context) {
	var announcements []models.Announcement
	database.DB.Order("id desc").Limit(20).Find(&announcements)

	// Convert to response format
	var response []gin.H
	for _, a := range announcements {
		response = append(response, gin.H{
			"id":         a.ID,
			"hash":       a.AnnouncementHash,
			"title":      a.Title,
			"content":    a.Content,
			"url":        a.URL,
			"is_active":  a.IsActive,
			"created_at": a.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// DeleteAnnouncement 删除公告
func DeleteAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid ID"})
		return
	}

	result := database.DB.Delete(&models.Announcement{}, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "公告不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ToggleAnnouncement 切换公告状态
func ToggleAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid ID"})
		return
	}

	isActiveStr := c.Query("is_active")
	isActive := isActiveStr == "true"

	result := database.DB.Model(&models.Announcement{}).Where("id = ?", id).Update("is_active", isActive)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "公告不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "is_active": isActive})
}
