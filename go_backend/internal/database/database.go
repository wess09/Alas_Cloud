package database

import (
	"alas-cloud/internal/models"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	// 初始化 GORM 连接 (PostgreSQL)
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
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:      newLogger,
		PrepareStmt: true, // 开启全局预编译语句缓存，提高性能
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 性能调优：配置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	// 设置最大的空闲连接数
	sqlDB.SetMaxIdleConns(20)
	// 设置数据库的最大打开连接数
	sqlDB.SetMaxOpenConns(100)
	// 设置连接可复用的最大时间
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

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
	log.Println("🔍 Checking for SQLite to MySQL migration...")
	migrateFromSQLite()

	return nil
}

func migrateFromSQLite() {
	sqlitePath := os.Getenv("SQLITE_DB_PATH")
	log.Printf("🔍 SQLITE_DB_PATH from env: %s", sqlitePath)
	if sqlitePath == "" {
		// 默认检测路径
		sqlitePath = "data/alas_cloud.db"
	}

	absPath := sqlitePath
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Printf("ℹ️ SQLite file not found at %s, skipping migration.", absPath)
		return // SQLite 文件不存在，跳过
	}

	log.Printf("🚀 Found SQLite database at %s, checking migration status...", absPath)

	// 检查是否已经迁移过
	var config models.SystemConfig
	err := DB.Where("key = ?", "sqlite_migrated").First(&config).Error
	if err == nil && config.Value == "true" {
		log.Println("ℹ️ Migration already completed (flag found in DB), skipping.")
		return // 已完成迁移，跳过
	}

	log.Printf("🚀 Detected SQLite database at %s, starting automatic migration...", sqlitePath)

	src, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		log.Printf("⚠️ Failed to open SQLite for migration: %v", err)
		return
	}

	// 开始同步数据
	shouldTruncate := os.Getenv("FORCE_MIGRATION_REDO") == "true"
	if shouldTruncate {
		log.Println("⚠️ FORCE_MIGRATION_REDO is true, truncating PostgreSQL tables before migration...")
		// PostgreSQL 的级联清空
		DB.Exec("TRUNCATE TABLE stamina_kline CASCADE")
		DB.Exec("TRUNCATE TABLE stamina_snapshots CASCADE")
		DB.Exec("TRUNCATE TABLE telemetry_data CASCADE")
		DB.Exec("TRUNCATE TABLE azurstat_reports CASCADE")
		DB.Exec("TRUNCATE TABLE azurstat_item_drops CASCADE")
		
		// 清除迁移标记
		DB.Where("key = ?", "sqlite_migrated").Delete(&models.SystemConfig{})
	}

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
	log.Println("✅ Automatic migration from SQLite to PostgreSQL completed.")
}

func copyTable[T any](src *gorm.DB, dst *gorm.DB, tableName string) {
	var count int64
	if err := src.Model(new(T)).Count(&count).Error; err != nil {
		log.Printf("  ⚠️ Error counting %s: %v", tableName, err)
		return
	}
	if count == 0 {
		return
	}

	log.Printf("  📦 Copying %d rows from %s...", count, tableName)
	
	var processed int64
	var items []T
	// 使用 FindInBatches 避免一次性加载百万级数据到内存
	err := src.FindInBatches(&items, 5000, func(tx *gorm.DB, batch int) error {
		// 使用 DoNothing 忽略主键冲突 (避免因重复主键导致迁移中断)
		if err := dst.Clauses(clause.OnConflict{DoNothing: true}).Create(&items).Error; err != nil {
			return err
		}

		processed += int64(len(items))
		if batch%10 == 0 || processed == count {
			log.Printf("    - Progress [%s]: %d/%d (%.1f%%)", tableName, processed, count, float64(processed)*100/float64(count))
		}
		return nil
	}).Error

	if err != nil {
		log.Printf("  ❌ Error migrating %s: %v", tableName, err)
	} else {
		log.Printf("  ✅ Finished copying %s", tableName)
	}
}
