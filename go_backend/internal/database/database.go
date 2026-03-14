package database

import (
	"alas-cloud/internal/models"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	// 1. 自动创建数据库逻辑
	parts := strings.Split(dsn, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid DSN format")
	}
	
	serverDSN := parts[0] + "/"
	if strings.Contains(parts[1], "?") {
		serverDSN += "?" + strings.Split(parts[1], "?")[1]
	}

	dbName := strings.Split(parts[1], "?")[0]

	tempDB, err := sql.Open("mysql", serverDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to mysql server for setup: %w", err)
	}
	defer tempDB.Close()

	_, err = tempDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName))
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// 2. 正式初始化 GORM 连接
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 自动迁移模式
	err = DB.AutoMigrate(
		&models.TelemetryData{},
		&models.AzurstatReport{},
		&models.AzurstatItemDrop{},
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

	// 3. 执行一次性数据迁移 (SQLite -> MySQL)
	migrateFromSQLite()

	return nil
}

func migrateFromSQLite() {
	sqlitePath := os.Getenv("SQLITE_DB_PATH")
	if sqlitePath == "" {
		// 默认检测路径
		sqlitePath = "data/alas_cloud.db"
	}

	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		return // SQLite 文件不存在，跳过
	}

	// 检查是否已经迁移过
	var config models.SystemConfig
	err := DB.Where("`key` = ?", "sqlite_migrated").First(&config).Error
	if err == nil && config.Value == "true" {
		return // 已完成迁移，跳过
	}

	log.Printf("🚀 Detected SQLite database at %s, starting automatic migration...", sqlitePath)

	src, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		log.Printf("⚠️ Failed to open SQLite for migration: %v", err)
		return
	}

	// 开始同步数据
	copyTable[models.UserProfile](src, DB, "UserProfiles")
	copyTable[models.AdminUser](src, DB, "AdminUsers")
	copyTable[models.Announcement](src, DB, "Announcements")
	copyTable[models.SystemConfig](src, DB, "SystemConfigs")
	copyTable[models.BannedUser](src, DB, "BannedUsers")
	copyTable[models.TelemetryData](src, DB, "TelemetryData")
	copyTable[models.AzurstatReport](src, DB, "AzurstatReports")
	copyTable[models.AzurstatItemDrop](src, DB, "AzurstatItemDrops")
	copyTable[models.Report](src, DB, "Reports")
	copyTable[models.StaminaSnapshot](src, DB, "StaminaSnapshots")
	copyTable[models.StaminaOHLCV](src, DB, "StaminaOHLCVs")

	// 记录完成标记
	mark := models.SystemConfig{Key: "sqlite_migrated", Value: "true"}
	DB.Save(&mark)
	log.Println("✅ Automatic migration from SQLite to MySQL completed.")
}

func copyTable[T any](src *gorm.DB, dst *gorm.DB, tableName string) {
	var items []T
	if err := src.Find(&items).Error; err != nil {
		log.Printf("  ⚠️ Error reading %s: %v", tableName, err)
		return
	}
	if len(items) == 0 {
		return
	}

	if err := dst.CreateInBatches(items, 100).Error; err != nil {
		log.Printf("  ❌ Error writing %s: %v", tableName, err)
	} else {
		log.Printf("  ✅ Copied %d rows to %s", len(items), tableName)
	}
}
