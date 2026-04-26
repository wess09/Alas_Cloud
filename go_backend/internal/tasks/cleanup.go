package tasks

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/handlers"
	"alas-cloud/internal/models"
	"log"
	"os"
	"strconv"
	"time"
)

// StartCleanupTask starts the background cleanup task
func StartCleanupTask() {
	if os.Getenv("DISABLE_CLEANUP_TASK") == "true" {
		log.Println("[CLEANUP] disabled by DISABLE_CLEANUP_TASK=true")
		return
	}

	interval := loadCleanupInterval()
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()
	log.Printf("[CLEANUP] started interval=%s batch_size=%d", interval, loadCleanupBatchSize())
}

func cleanup() {
	cutoff := time.Now().Add(-24 * time.Hour)
	batchSize := loadCleanupBatchSize()
	var total int64

	for {
		var rows []models.TelemetryData
		if err := database.DB.
			Select("id").
			Where("updated_at < ?", cutoff).
			Order("updated_at ASC").
			Limit(batchSize).
			Find(&rows).Error; err != nil {
			log.Printf("[CLEANUP] select failed: %v", err)
			return
		}
		if len(rows) == 0 {
			break
		}

		result := database.DB.Delete(&rows)
		if result.Error != nil {
			log.Printf("[CLEANUP] delete failed: %v", result.Error)
			return
		}
		total += result.RowsAffected
		if len(rows) < batchSize {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if total > 0 {
		log.Printf("[CLEANUP] cleaned up %d inactive instances", total)
		handlers.InvalidateTelemetryCache()
		handlers.RequestStatsRefresh()
	}
}

func loadCleanupInterval() time.Duration {
	raw := os.Getenv("CLEANUP_INTERVAL_MINUTES")
	if raw == "" {
		return 6 * time.Hour
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil || minutes <= 0 {
		return 6 * time.Hour
	}
	return time.Duration(minutes) * time.Minute
}

func loadCleanupBatchSize() int {
	raw := os.Getenv("CLEANUP_BATCH_SIZE")
	if raw == "" {
		return 500
	}
	size, err := strconv.Atoi(raw)
	if err != nil || size <= 0 {
		return 500
	}
	return size
}
