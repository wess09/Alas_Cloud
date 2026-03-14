package tasks

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm"
)

// StartUsernameGeneratorTask 启动后台用户名生成任务
func StartUsernameGeneratorTask() {
	// 启动时先运行一次 (可选)
	// go processNextUser()

	// 限制速率: ip-api.com 免费版限制 45 req/min -> 1.33s/req
	// 我们设置为 3 秒一次，非常安全
	ticker := time.NewTicker(3 * time.Second)
	go func() {
		for range ticker.C {
			processNextUser()
		}
	}()
}

type ipApiResponse struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Message     string  `json:"message"`
}

func processNextUser() {
	var results []struct {
		DeviceID  string
		IPAddress string
	}

	// 查找没有 Profile 的 DeviceID，并获取其最新的 IP
	// 使用子查询排除已存在的 Profile
	// 注意：这里需要去重，取最新的 IP
	err := database.DB.Raw(`
		SELECT t.device_id, ANY_VALUE(t.ip_address) as ip_address
		FROM telemetry_data t
		LEFT JOIN user_profiles u ON t.device_id = u.device_id
		WHERE u.device_id IS NULL
		GROUP BY t.device_id
		ORDER BY MAX(t.created_at) DESC
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
