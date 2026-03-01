package middleware

import (
	"alas-cloud/internal/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "缺少认证头"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		username, err := utils.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的 Token"})
			return
		}

		c.Set("username", username)
		c.Next()
	}
}
