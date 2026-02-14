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

	// 设置 Gin 模式
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 全局中间件
	r.Use(middleware.BlacklistMiddleware())
	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "*")

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
	r.GET("/api/get/announcement", handlers.GetLatestAnnouncement)
	r.POST("/api/telemetry", handlers.SubmitTelemetry)
	r.GET("/api/telemetry/stats", handlers.GetTelemetryStats)
	r.GET("/api/telemetry/stats/stream", handlers.StreamTelemetryStats)
	r.POST("/api/post/bug", handlers.SubmitBug) // 假设 Bug 报告不需要鉴权，或者维持现状

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
	}

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
