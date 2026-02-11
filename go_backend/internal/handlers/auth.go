package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"alas-cloud/internal/utils"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminLogin 管理员登录
func AdminLogin(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.AdminUser
	result := database.DB.Where("username = ?", req.Username).First(&user)

	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "用户名或密码错误"})
		return
	}

	if user.PasswordHash != utils.HashPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "用户名或密码错误"})
		return
	}

	token, err := utils.GenerateToken(user.Username, 24)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "登录服务异常"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"token":      token,
		"expires_in": 24 * 3600,
	})
}

// AdminChangePassword 修改密码
func AdminChangePassword(c *gin.Context) {
	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	usernameStr := username.(string)

	var user models.AdminUser
	if err := database.DB.Where("username = ?", usernameStr).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "用户不存在"})
		return
	}

	if user.PasswordHash != utils.HashPassword(req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "旧密码错误"})
		return
	}

	user.PasswordHash = utils.HashPassword(req.NewPassword)
	database.DB.Save(&user)

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "密码已更新"})
}

// EnsureDefaultAdmin 确保管理员存在，且默认密码为 admin123
func EnsureDefaultAdmin() {
	var user models.AdminUser
	result := database.DB.Where("username = ?", "admin").First(&user)

	defaultPassword := "admin123"
	passwordHash := utils.HashPassword(defaultPassword)

	if result.Error != nil {
		// 不存在则创建
		user = models.AdminUser{
			Username:     "admin",
			PasswordHash: passwordHash,
		}
		database.DB.Create(&user)
		log.Printf("🔐 Created default admin user: admin / %s", defaultPassword)
	}
}
