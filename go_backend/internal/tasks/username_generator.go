package tasks

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// StartUsernameGeneratorTask 启动后台用户名生成任务
func StartUsernameGeneratorTask() {
	if os.Getenv("DISABLE_USERNAME_GENERATOR") == "true" {
		log.Println("[USERNAME] generator disabled by DISABLE_USERNAME_GENERATOR=true")
		return
	}

	interval := loadUsernameGeneratorInterval()
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			processNextUser()
		}
	}()
	log.Printf("[USERNAME] generator started interval=%s", interval)
}

type ipApiResponse struct {
	Status     string `json:"status"`
	Country    string `json:"country"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	Message    string `json:"message"`
}

func processNextUser() {
	var results []struct {
		DeviceID  string
		IPAddress string
	}

	// PostgreSQL 上用 DISTINCT ON 取每个设备的最新记录，避免 GROUP BY + MAX 全表聚合排序。
	err := database.DB.Raw(`
		SELECT latest.device_id, latest.ip_address
		FROM (
			SELECT DISTINCT ON (t.device_id)
				t.device_id,
				t.ip_address,
				t.created_at
			FROM telemetry_data t
			LEFT JOIN user_profiles u ON t.device_id = u.device_id
			WHERE u.device_id IS NULL
			ORDER BY t.device_id, t.created_at DESC, t.id DESC
		) AS latest
		ORDER BY latest.created_at DESC
		LIMIT 1
	`).Scan(&results).Error

	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Printf("❌ Failed to query unnamed users: %v", err)
		}
		return
	}

	if len(results) == 0 {
		return // 没有需要处理的用户
	}

	target := results[0]
	if target.IPAddress == "" || target.IPAddress == "::1" || target.IPAddress == "127.0.0.1" {
		// 本地 IP 无法获取地理位置，给个默认名
		saveProfile(target.DeviceID, "来自本地的猫娘小萝莉")
		return
	}

	// 调用 IP API
	name := generateNameFromIP(target.IPAddress)
	saveProfile(target.DeviceID, name)
}

func generateNameFromIP(ip string) string {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip))
	if err != nil {
		log.Printf("⚠️ Failed to fetch IP info for %s: %v", ip, err)
		return "神秘的猫娘小萝莉"
	}
	defer resp.Body.Close()

	var data ipApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "神秘的猫娘小萝莉"
	}

	if data.Status != "success" {
		log.Printf("⚠️ IP API error for %s: %s", ip, data.Message)
		return "神秘的猫娘小萝莉"
	}

	region := data.City
	if region == "" {
		region = data.RegionName
	}
	if region == "" {
		region = data.Country
	}
	if region == "" {
		return "神秘的猫娘小萝莉"
	}

	return fmt.Sprintf("来自%s的猫娘小萝莉", region)
}

func saveProfile(deviceID, username string) {
	profile := models.UserProfile{
		DeviceID: deviceID,
		Username: username,
	}
	if err := database.DB.Save(&profile).Error; err != nil {
		log.Printf("❌ Failed to save auto-generated profile for %s: %v", deviceID, err)
	} else {
		log.Printf("✨ Auto-generated username for %s: %s", deviceID, username)
	}
}

func loadUsernameGeneratorInterval() time.Duration {
	raw := os.Getenv("USERNAME_GENERATOR_INTERVAL_SECONDS")
	if raw == "" {
		return 5 * time.Minute
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 5 * time.Minute
	}
	if seconds < 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}
