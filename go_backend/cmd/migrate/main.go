package main

import (
	"alas-cloud/internal/models"
	"fmt"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. 配置路径和连接
	sqlitePath := "data/alas_cloud.db" // 假设默认路径
	if envPath := os.Getenv("SQLITE_PATH"); envPath != "" {
		sqlitePath = envPath
	}
	
	mysqlDSN := os.Getenv("DATABASE_URL")
	if mysqlDSN == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	fmt.Printf("🚀 Starting migration from %s to MySQL...\n", sqlitePath)

	// 2. 连接 SQLite
	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}

	// 3. 连接 MySQL
	mysqlDB, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// 4. 重建表结构 (确保 MySQL 端表结构是最新的)
	fmt.Println("📦 Ensuring table structures...")
	err = mysqlDB.AutoMigrate(
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
		log.Fatalf("Failed to migrate MySQL schema: %v", err)
	}

	// 5. 迁移数据
	migrateData(sqliteDB, mysqlDB)

	fmt.Println("✅ Migration completed successfully!")
}

func migrateData(src *gorm.DB, dst *gorm.DB) {
	// 列表顺序：先迁基础数据，再迁关联数据
	
	// UserProfiles
	copyTable[models.UserProfile](src, dst, "UserProfiles")
	// AdminUsers
	copyTable[models.AdminUser](src, dst, "AdminUsers")
	// Announcements
	copyTable[models.Announcement](src, dst, "Announcements")
	// SystemConfigs
	copyTable[models.SystemConfig](src, dst, "SystemConfigs")
	// BannedUsers
	copyTable[models.BannedUser](src, dst, "BannedUsers")
	// TelemetryData
	copyTable[models.TelemetryData](src, dst, "TelemetryData")
	// AzurstatReports
	copyTable[models.AzurstatReport](src, dst, "AzurstatReports")
	// AzurstatItemDrops
	copyTable[models.AzurstatItemDrop](src, dst, "AzurstatItemDrops")
	// Reports
	copyTable[models.Report](src, dst, "Reports")
	// StaminaSnapshots
	copyTable[models.StaminaSnapshot](src, dst, "StaminaSnapshots")
	// StaminaOHLCVs
	copyTable[models.StaminaOHLCV](src, dst, "StaminaOHLCVs")
}

func copyTable[T any](src *gorm.DB, dst *gorm.DB, tableName string) {
	fmt.Printf("  - Copying %s...\n", tableName)
	var items []T
	result := src.Find(&items)
	if result.Error != nil {
		fmt.Printf("    ⚠️ Error reading %s: %v\n", tableName, result.Error)
		return
	}
	
	if len(items) == 0 {
		fmt.Printf("    ℹ️ No data in %s, skipping.\n", tableName)
		return
	}

	// 批量插入
	err := dst.CreateInBatches(items, 100).Error
	if err != nil {
		fmt.Printf("    ❌ Error writing %s: %v\n", tableName, err)
	} else {
		fmt.Printf("    ✅ Copied %d rows.\n", len(items))
	}
}
