package tasks

import (
	"alas-cloud/internal/handlers"
	"log"
	"time"
)

// StartStaminaAggregator 启动体力聚合定时任务
func StartStaminaAggregator() {
	go func() {
		log.Println("[STAMINA] Aggregator started, interval=60s")
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			minuteKey := now.Format("2006-01-02T15:04")

			// 执行分钟级聚合
			handlers.AggregateMinute(minuteKey)

			// 执行上级周期聚合
			handlers.AggregateHigherPeriods()

			// 通知 SSE 客户端
			handlers.NotifyDashboardUpdate()
		}
	}()
}
