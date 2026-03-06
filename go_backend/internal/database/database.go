package database

import (
	"alas-cloud/internal/models"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() error {
	// 获取数据目录，默认为当前目录
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "telemetry.db")

	// 配置 GORM 日志
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	var err error
	// 启用 WAL 模式和 busy_timeout，允许读写并发，避免聚合任务锁死读取
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 尝试删除旧的唯一索引，以便按月保存数据
	// 忽略错误，因为如果索引不存在或已经删除，它会报错
	DB.Exec("DROP INDEX IF EXISTS uix_device_instance")

	// 自动迁移模式
	// 注意：GORM 的 AutoMigrate 会自动创建表、缺少列、索引等
	// 但不会删除未使用的列，这通常是安全的
	err = DB.AutoMigrate(
		&models.TelemetryData{},
		&models.Announcement{},
		&models.SystemConfig{},
		&models.AdminUser{},
		&models.UserProfile{},
		&models.Report{},
		&models.BannedUser{},
		&models.StaminaSnapshot{},
		&models.StaminaOHLCV{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}
