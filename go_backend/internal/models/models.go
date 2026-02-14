package models

import (
	"time"
)

// UserProfile 用户个人资料
type UserProfile struct {
	DeviceID  string    `gorm:"primaryKey;uniqueIndex;column:device_id" json:"device_id"`
	Username  string    `gorm:"column:username" json:"username"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (UserProfile) TableName() string {
	return "user_profiles"
}

// TelemetryData 遥测数据模型
type TelemetryData struct {
	ID                uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceID          string    `gorm:"uniqueIndex:uix_device_instance;not null;column:device_id" json:"device_id"`
	InstanceID        string    `gorm:"uniqueIndex:uix_device_instance;not null;column:instance_id" json:"instance_id"`
	IPAddress         string    `gorm:"index;column:ip_address" json:"ip_address"`
	Month             string    `gorm:"index;not null;column:month" json:"month"`
	BattleCount       int       `gorm:"not null;column:battle_count" json:"battle_count"`
	BattleRounds      int       `gorm:"not null;column:battle_rounds" json:"battle_rounds"`
	SortieCost        int       `gorm:"not null;column:sortie_cost" json:"sortie_cost"`
	AkashiEncounters  int       `gorm:"not null;column:akashi_encounters" json:"akashi_encounters"`
	AkashiProbability float64   `gorm:"not null;column:akashi_probability" json:"akashi_probability"`
	AverageStamina    float64   `gorm:"not null;column:average_stamina" json:"average_stamina"`
	NetStaminaGain    int       `gorm:"not null;column:net_stamina_gain" json:"net_stamina_gain"`
	CreatedAt         time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (TelemetryData) TableName() string {
	return "telemetry_data"
}

// Announcement 公告数据模型
type Announcement struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AnnouncementHash string    `gorm:"uniqueIndex;not null;size:32" json:"hash"`
	Title            string    `gorm:"not null" json:"title"`
	Content          string    `json:"content"`
	URL              string    `json:"url"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	IsActive         bool      `gorm:"default:true" json:"is_active"`
}

// TableName 指定表名
func (Announcement) TableName() string {
	return "announcements"
}

// AdminUser 管理员账户模型
type AdminUser struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"` // 不在 JSON 中返回
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (AdminUser) TableName() string {
	return "admin_users"
}

// Report 举报模型
type Report struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetID     string    `gorm:"index;not null;column:target_id" json:"target_id"`     // 被举报人 DeviceID
	ReporterID   string    `gorm:"index;not null;column:reporter_id" json:"reporter_id"` // 举报人 DeviceID (或 IP)
	Reason       string    `gorm:"column:reason" json:"reason"`
	CreatedAt    time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

// TableName 指定表名
func (Report) TableName() string {
	return "reports"
}

// BannedUser 封禁用户模型
type BannedUser struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DeviceID  string    `gorm:"uniqueIndex;column:device_id" json:"device_id"` // 被封禁的 DeviceID
	IPAddress string    `gorm:"index;column:ip_address" json:"ip_address"`     // 被封禁的 IP (最后一次已知 IP)
	Username  string    `gorm:"column:username" json:"username"`               // 封禁时的用户名 (备份用)
	Reason    string    `gorm:"column:reason" json:"reason"`
	BannedAt  time.Time `gorm:"autoCreateTime;column:banned_at" json:"banned_at"`
}

// TableName 指定表名
func (BannedUser) TableName() string {
	return "banned_users"
}

// API Request/Response Models

type TelemetryRequest struct {
	DeviceID          string  `json:"device_id" binding:"required"`
	InstanceID        string  `json:"instance_id" binding:"required"`
	Month             string  `json:"month" binding:"required"`
	BattleCount       int     `json:"battle_count" binding:"gte=0"`
	BattleRounds      int     `json:"battle_rounds" binding:"gte=0"`
	SortieCost        int     `json:"sortie_cost" binding:"gte=0"`
	AkashiEncounters  int     `json:"akashi_encounters" binding:"gte=0"`
	AkashiProbability float64 `json:"akashi_probability" binding:"gte=0,lte=1"`
	AverageStamina    float64 `json:"average_stamina" binding:"gte=0"`
	NetStaminaGain    int     `json:"net_stamina_gain"`
}

type BugReportRequest struct {
	DeviceID       string                 `json:"device_id"`
	LogType        string                 `json:"log_type" binding:"required"`
	LogContent     string                 `json:"log_content" binding:"required"`
	Timestamp      string                 `json:"timestamp"`
	AdditionalInfo map[string]interface{} `json:"additional_info"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AnnouncementRequest struct {
	Title   string `json:"title" binding:"required,min=1"`
	Content string `json:"content"`
	URL     string `json:"url"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}
