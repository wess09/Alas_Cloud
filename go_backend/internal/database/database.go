package database

import (
	"alas-cloud/internal/models"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	// DSN 格式: user:password@tcp(host:port)/dbname?params
	parts := strings.Split(dsn, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid DSN format")
	}
	
	// 提取不带数据库名的 DSN 用于初始连接
	// 取最后一个 / 之前的部分和之后的参数部分（如果有）
	serverDSN := parts[0] + "/"
	if strings.Contains(parts[1], "?") {
		serverDSN += "?" + strings.Split(parts[1], "?")[1]
	}

	// 提取数据库名
	dbName := strings.Split(parts[1], "?")[0]

	// 使用标准库尝试连接服务器并创建数据库
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

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 自动迁移模式
	// 注意：GORM 的 AutoMigrate 会自动创建表、缺少列、索引等
	// 但不会删除未使用的列，这通常是安全的
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

	return nil
}
