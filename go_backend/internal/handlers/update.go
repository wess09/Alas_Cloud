package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAutoUpdateStatus 获取当前自动更新的状态 (公开)
func GetAutoUpdateStatus(c *gin.Context) {
	var config models.SystemConfig
	result := database.DB.Where("`key` = ?", "auto_update").First(&config)
	if result.Error != nil {
		// 如果没有找到，默认返回 false
		c.JSON(http.StatusOK, false)
		return
	}

	c.JSON(http.StatusOK, config.Value == "true")
}

// AdminGetAutoUpdateStatus 获取当前自动更新的状态 (管理员)
func AdminGetAutoUpdateStatus(c *gin.Context) {
	var config models.SystemConfig
	result := database.DB.Where("`key` = ?", "auto_update").First(&config)
	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{"auto_update": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auto_update": config.Value == "true"})
}

// AdminToggleAutoUpdate 切换自动更新状态 (管理员)
func AdminToggleAutoUpdate(c *gin.Context) {
	isActive := c.Query("is_active") == "true"

	val := "false"
	if isActive {
		val = "true"
	}

	var config models.SystemConfig
	result := database.DB.Where("`key` = ?", "auto_update").First(&config)
	if result.Error != nil {
		// 不存在则创建
		config = models.SystemConfig{
			Key:   "auto_update",
			Value: val,
		}
		if err := database.DB.Create(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
			return
		}
	} else {
		// 存在则更新
		config.Value = val
		if err := database.DB.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
			return
		}
	}

	c.Status(http.StatusNoContent)
}
