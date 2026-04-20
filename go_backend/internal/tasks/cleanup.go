package tasks

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/handlers"
	"alas-cloud/internal/models"
	"log"
	"time"
)

// StartCleanupTask starts the background cleanup task
func StartCleanupTask() {
	// Run immediately on startup
	cleanup()

	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			cleanup()
		}
	}()
}

func cleanup() {
	log.Println("♻️ executing cleanup task...")
	cutoff := time.Now().Add(-24 * time.Hour)
	result := database.DB.Where("updated_at < ?", cutoff).Delete(&models.TelemetryData{})
	if result.Error != nil {
		log.Printf("⚠️ cleanup failed: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🧹 cleaned up %d inactive instances", result.RowsAffected)
		handlers.InvalidateTelemetryCache()
		handlers.RequestStatsRefresh()
	}
}
