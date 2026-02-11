package handlers

import (
	"alas-cloud/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

// SubmitBug 提交 Bug 报告
func SubmitBug(c *gin.Context) {
	var req models.BugReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Timestamp == "" {
		req.Timestamp = time.Now().Format(time.RFC3339)
	}

	rawDeviceID := req.DeviceID
	if rawDeviceID == "" {
		rawDeviceID = "anonymous"
	}
	// Sanitize Device ID
	reg, _ := regexp.Compile("[^a-zA-Z0-9_-]+")
	safeDeviceID := reg.ReplaceAllString(rawDeviceID, "")
	if safeDeviceID == "" {
		safeDeviceID = "anonymous"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}
	bugLogDir := filepath.Join(dataDir, "bug_logs", safeDeviceID)
	os.MkdirAll(bugLogDir, 0755)

	logFile := filepath.Join(bugLogDir, fmt.Sprintf("%s.log", safeDeviceID))

	ip := c.ClientIP()

	additionalInfo := "None"
	if req.AdditionalInfo != nil {
		bytes, _ := json.Marshal(req.AdditionalInfo)
		additionalInfo = string(bytes)
	}

	logEntry := fmt.Sprintf("[%s] [%s] %s\n  IP: %s\n  Additional Info: %s\n\n",
		req.Timestamp, req.LogType, req.LogContent, ip, additionalInfo)

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	defer f.Close()

	if _, err := f.WriteString(logEntry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
