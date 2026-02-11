package middleware

import (
	"alas-cloud/internal/utils"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// IP Blacklist with concurrency safety
var (
	BlacklistedIPs = make(map[string]time.Time)
	mutex          sync.RWMutex
	BlacklistFile  = "data/blacklist.txt" // Adjust path as needed
)

func LoadBlacklist() {
	// Implementation to load from file (simplified for now)
}

func BanIP(ip string) {
	mutex.Lock()
	defer mutex.Unlock()
	BlacklistedIPs[ip] = time.Now().Add(8 * time.Hour)
	// Also append to file...
}

// BlacklistMiddleware checks if the IP is banned
func BlacklistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		mutex.RLock()
		expireTime, exists := BlacklistedIPs[ip]
		mutex.RUnlock()

		if exists {
			if time.Now().After(expireTime) {
				// Expired, remove from map
				mutex.Lock()
				delete(BlacklistedIPs, ip)
				mutex.Unlock()
			} else {
				// Banned
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": "None"})
				return
			}
		}
		c.Next()
	}
}

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
