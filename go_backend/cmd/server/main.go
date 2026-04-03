package main

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/handlers"
	"alas-cloud/internal/middleware"
	"alas-cloud/internal/tasks"
	"alas-cloud/internal/utils"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化
	utils.InitJWT()
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 确保默认管理员存在
	handlers.EnsureDefaultAdmin()

	// 启动后台任务
	tasks.StartCleanupTask()
	tasks.StartUsernameGeneratorTask()
	tasks.StartStaminaAggregator()

	// 设置 Gin 模式
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 全局中间件
	// CORS
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 路由
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"service": "Alas API (Go)", "status": "running"})
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Public API
	handlers.InitStatsWorker() // 启动后台统计预计算协程
	r.GET("/api/get/announcement", handlers.GetLatestAnnouncement)
	r.GET("/api/updata", handlers.GetAutoUpdateStatus)
	r.POST("/api/telemetry", handlers.SubmitTelemetry)
	r.POST("/api/azurstat", handlers.SubmitAzurstat)
	r.GET("/api/azurstat/stats", handlers.GetAzurstatStats)
	r.GET("/api/azurstat/items", handlers.GetAzurstatItems)
	r.GET("/api/azurstat/history", handlers.GetAzurstatHistory)
	r.GET("/api/azurstat/filters", handlers.GetAzurstatFilters)
	r.GET("/api/telemetry/history", handlers.GetTelemetryHistory)
	r.GET("/api/telemetry/global_history", handlers.GetGlobalTelemetryHistory)
	r.GET("/api/telemetry/stats", handlers.GetTelemetryStats)
	r.GET("/api/telemetry/stats/stream", handlers.StreamTelemetryStats)
	r.POST("/api/post/bug", handlers.SubmitBug) // 假设 Bug 报告不需要鉴权，或者维持现状

	// Leaderboard API
	r.GET("/api/leaderboard", handlers.GetLeaderboard)
	r.POST("/api/user/profile", handlers.UpdateUserProfile)

	// Stamina Dashboard API
	r.POST("/api/stamina/report", handlers.ReportStamina)
	r.GET("/api/stamina/kline", handlers.GetStaminaKline)
	r.GET("/api/stamina/latest", handlers.GetStaminaLatest)
	r.GET("/api/stamina/stream", handlers.StreamStaminaDashboard)

	// Report & Ban API
	r.POST("/api/report", handlers.ReportUser)
	r.GET("/api/reports", handlers.GetReportedUsers)
	r.GET("/api/bans", handlers.GetBannedUsers)

	// Admin API
	r.POST("/api/admin/login", handlers.AdminLogin)

	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware())
	{
		admin.POST("/change-password", handlers.AdminChangePassword)
		admin.POST("/announcement", handlers.CreateAnnouncement)
		admin.GET("/announcements", handlers.ListAnnouncements)
		admin.DELETE("/announcement/:id", handlers.DeleteAnnouncement)
		admin.PATCH("/announcement/:id/toggle", handlers.ToggleAnnouncement)
		
		// System Config
		admin.GET("/config/auto_update", handlers.AdminGetAutoUpdateStatus)
		admin.PATCH("/config/auto_update", handlers.AdminToggleAutoUpdate)

		// User Management
		admin.POST("/ban", handlers.DirectBanUser)
		admin.POST("/unban", handlers.UnbanUser)
		admin.POST("/dismiss", handlers.DismissReport)
	}
	
	r.POST("/api/report/undo", handlers.UndoReport)

	// 静态文件 (前端)
	if _, err := os.Stat("frontend"); err == nil {
		r.Static("/admin", "./frontend")
	}

	// 启动服务器 (Graceful Shutdown)
	srv := &http.Server{
		Addr:    ":8000",
		Handler: r,
	}

	go func() {
		log.Println("🚀 Starting Alas API (Go) on :8000")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
