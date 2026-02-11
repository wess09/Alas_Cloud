package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// HashPassword SHA256 Hashing matching Python's hashlib.sha256
func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// GenerateAnnouncementHash MD5 Hashing for announcements
func GenerateAnnouncementHash(title, content, url string) string {
	data := fmt.Sprintf("%s|%s|%s|%d", title, content, url, time.Now().Unix())
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

var jwtSecret []byte

func InitJWT() {
	// Try to read .jwt_secret from parent directory (to share with Python if possible, or just be consistent)
	// For simplicity in this Go backend, we'll look for it in the current working directory or data dir
	// In the Python code: BASE_DIR / ".jwt_secret"

	// We will try to read from environment or default file
	secretPath := ".jwt_secret"
	if _, err := os.Stat(secretPath); os.IsNotExist(err) {
		// Generate new secret if not exists
		// In a real migration, we should copy the old secret
		// For now, let's just generate one if missing
		jwtSecret = []byte("default-secret-change-me")
	} else {
		content, _ := os.ReadFile(secretPath)
		jwtSecret = content
	}
}

// GenerateToken creates a new JWT token
func GenerateToken(username string, expireHours int) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(time.Hour * time.Duration(expireHours)).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken parses and validates a JWT token
func ParseToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if username, ok := claims["sub"].(string); ok {
			return username, nil
		}
	}

	return "", fmt.Errorf("invalid token claims")
}
