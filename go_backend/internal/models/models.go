package models

import (
	"time"
)

// UserProfile 用户个人资料
type UserProfile struct {
	DeviceID  string    `gorm:"primaryKey;uniqueIndex;column:device_id;size:191" json:"device_id"`
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
	DeviceID          string    `gorm:"uniqueIndex:uix_dev_inst_month;not null;column:device_id;size:191;index:idx_telemetry_month_device,priority:2;index:idx_telemetry_device_created,priority:1" json:"device_id"`
	InstanceID        string    `gorm:"uniqueIndex:uix_dev_inst_month;not null;column:instance_id;size:191" json:"instance_id"`
	IPAddress         string    `gorm:"index;column:ip_address;size:191" json:"ip_address"`
	Month             string    `gorm:"uniqueIndex:uix_dev_inst_month;not null;column:month;size:191;index:idx_telemetry_month_device,priority:1" json:"month"`
	BattleCount       int       `gorm:"not null;column:battle_count" json:"battle_count"`
	BattleRounds      int       `gorm:"not null;column:battle_rounds" json:"battle_rounds"`
	SortieCost        int       `gorm:"not null;column:sortie_cost" json:"sortie_cost"`
	AkashiEncounters  int       `gorm:"not null;column:akashi_encounters" json:"akashi_encounters"`
	AkashiProbability float64   `gorm:"not null;column:akashi_probability" json:"akashi_probability"`
	AverageStamina    float64   `gorm:"not null;column:average_stamina" json:"average_stamina"`
	NetStaminaGain    int       `gorm:"not null;column:net_stamina_gain" json:"net_stamina_gain"`
	CreatedAt         time.Time `gorm:"autoCreateTime;column:created_at;index:idx_telemetry_device_created,priority:2,sort:desc" json:"created_at"`
	UpdatedAt         time.Time `gorm:"index:idx_updated_at;autoUpdateTime;column:updated_at" json:"updated_at"`
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

// SystemConfig 系统全局配置模型
type SystemConfig struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Key       string    `gorm:"uniqueIndex;not null;column:key;size:191" json:"key"`
	Value     string    `gorm:"not null;column:value" json:"value"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (SystemConfig) TableName() string {
	return "system_configs"
}

// AdminUser 管理员账户模型
type AdminUser struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null;size:191" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"` // 不在 JSON 中返回
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (AdminUser) TableName() string {
	return "admin_users"
}

// Report 举报模型
type Report struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetID   string    `gorm:"index;not null;column:target_id;size:191;index:idx_report_target_reporter,priority:1" json:"target_id"`     // 被举报人 DeviceID
	ReporterID string    `gorm:"index;not null;column:reporter_id;size:191;index:idx_report_target_reporter,priority:2" json:"reporter_id"` // 举报人 DeviceID (或 IP)
	Reason     string    `gorm:"column:reason" json:"reason"`
	CreatedAt  time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

// TableName 指定表名
func (Report) TableName() string {
	return "reports"
}

// BannedUser 封禁用户模型
type BannedUser struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DeviceID  string    `gorm:"uniqueIndex;column:device_id;size:191" json:"device_id"` // 被封禁的 DeviceID
	IPAddress string    `gorm:"index;column:ip_address;size:191" json:"ip_address"`     // 被封禁的 IP (最后一次已知 IP)
	Username  string    `gorm:"column:username" json:"username"`                        // 封禁时的用户名 (备份用)
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

// ---- 体力大盘相关模型 ----

// StaminaSnapshot 用户体力快照（每次上报原始数据）
type StaminaSnapshot struct {
	ID        uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceID  string    `gorm:"index;not null;column:device_id;size:191;index:idx_stamina_snapshot_device_minute_created,priority:1" json:"device_id"`
	Stamina   float64   `gorm:"not null;column:stamina;index:idx_stamina_snapshot_minute_stamina,priority:2,sort:desc" json:"stamina"`
	MinuteKey string    `gorm:"index;not null;column:minute_key;size:191;index:idx_stamina_snapshot_device_minute_created,priority:2,sort:desc;index:idx_stamina_snapshot_minute_stamina,priority:1" json:"minute_key"` // 格式: 2006-01-02T15:04
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at;index:idx_stamina_snapshot_device_minute_created,priority:3,sort:desc" json:"created_at"`
}

// TableName 指定表名
func (StaminaSnapshot) TableName() string {
	return "stamina_snapshots"
}

// StaminaOHLCV 体力大盘 K 线聚合数据
type StaminaOHLCV struct {
	ID            uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	MinuteKey     string    `gorm:"uniqueIndex:uix_stamina_kline_key_period;not null;column:minute_key;size:191;index:idx_stamina_kline_period_minute,priority:2,sort:desc" json:"minute_key"` // 格式: 2006-01-02T15:04
	Period        string    `gorm:"uniqueIndex:uix_stamina_kline_key_period;not null;column:period;size:191;index:idx_stamina_kline_period_minute,priority:1" json:"period"`                   // 1m, 5m, 1h, 1d
	Open          float64   `gorm:"not null;column:open" json:"open"`
	High          float64   `gorm:"not null;column:high" json:"high"`
	Low           float64   `gorm:"not null;column:low" json:"low"`
	Close         float64   `gorm:"not null;column:close" json:"close"`
	Volume        float64   `gorm:"not null;column:volume" json:"volume"` // 大盘总量
	ReportedCount int       `gorm:"not null;column:reported_count" json:"reported_count"`
	FilledCount   int       `gorm:"not null;column:filled_count" json:"filled_count"`
	CreatedAt     time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

// TableName 指定 StaminaOHLCV 的自定义表名（使用新表名绕过旧索引）
func (StaminaOHLCV) TableName() string {
	return "stamina_kline"
}

// StaminaReportRequest 体力上报请求
type StaminaReportRequest struct {
	DeviceID string  `json:"device_id" binding:"required"`
	Stamina  float64 `json:"stamina" binding:"gte=0"`
}

// ---- AzurStat 掉落统计相关模型 ----

// AzurstatReport 单次 AzurStat 原始上报
// 每次请求视为一条独立记录，不做幂等去重
// Zone/ZoneType/ZoneID 保留原始维度便于筛选和聚合
// CombatCount 用于总体战斗轮数与平均每战掉落计算
// Task 仅用于区分不同上报来源任务
// CreatedAt 作为统计历史主时间字段
// UpdatedAt 由 GORM 自动维护
// DeviceID 采用与 telemetry 相同的设备标识
// HazardLevel 保存危险等级 1-6
// TableName 见下方
//
// 注意：前端独立部署，不影响后端数据模型设计。
type AzurstatReport struct {
	ID          uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceID    string    `gorm:"index:idx_azurstat_device_id;not null;column:device_id;size:191" json:"device_id"`
	Task        string    `gorm:"index:idx_azurstat_task;not null;column:task;size:191" json:"task"`
	Zone        string    `gorm:"column:zone" json:"zone"`
	ZoneType    string    `gorm:"column:zone_type" json:"zone_type"`
	ZoneID      string    `gorm:"index:idx_azurstat_zone_id;column:zone_id;size:191" json:"zone_id"`
	HazardLevel int       `gorm:"index:idx_azurstat_hazard_level;not null;column:hazard_level" json:"hazard_level"`
	CombatCount int       `gorm:"not null;column:combat_count" json:"combat_count"`
	CreatedAt   time.Time `gorm:"index:idx_azurstat_created_at;autoCreateTime;column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
}

func (AzurstatReport) TableName() string {
	return "azurstat_reports"
}

// AzurstatItemDrop 单次上报中的单个物品掉落
type AzurstatItemDrop struct {
	ID        uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	ReportID  uint      `gorm:"index:idx_azurstat_item_report_id;not null;column:report_id" json:"report_id"`
	Item      string    `gorm:"index:idx_azurstat_item_name;not null;column:item;size:191" json:"item"`
	Amount    int       `gorm:"not null;column:amount" json:"amount"`
	IsMeow    bool      `gorm:"index:idx_azurstat_is_meow;not null;default:false;column:is_meow" json:"is_meow"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

func (AzurstatItemDrop) TableName() string {
	return "azurstat_item_drops"
}

// AzurStat API 请求结构
type AzurstatRequest struct {
	DeviceID string       `json:"device_id" binding:"required"`
	Task     string       `json:"task" binding:"required"`
	Body     AzurstatBody `json:"body" binding:"required"`
}

type AzurstatBody struct {
	Zone        string         `json:"zone"`
	ZoneType    string         `json:"zone_type"`
	ZoneID      int            `json:"zone_id"`
	HazardLevel int            `json:"hazard_level" binding:"required"`
	CombatCount int            `json:"combat_count" binding:"required"`
	Items       []AzurstatItem `json:"items" binding:"required"`
}

type AzurstatItem struct {
	Item   string `json:"item" binding:"required"`
	Amount int    `json:"amount" binding:"required"`
	IsMeow bool   `json:"is_meow"`
}
