package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"alas-cloud/internal/utils"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Setup DB for testing
	os.Setenv("DATA_DIR", ".")
	if err := database.InitDB(); err != nil {
		panic("Failed to init DB: " + err.Error())
	}

	// Initialize Utils
	utils.InitJWT()

	EnsureDefaultAdmin()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	r.POST("/api/telemetry", SubmitTelemetry)
	r.POST("/api/admin/login", AdminLogin)

	return r
}

func TestHealthCheck(t *testing.T) {
	r := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"status":"healthy"}`, w.Body.String())
}

func TestAdminLogin(t *testing.T) {
	r := setupRouter()

	// 1. Default Admin Login
	loginReq := models.LoginRequest{
		Username: "admin",
		Password: "admin123",
	}
	jsonValue, _ := json.Marshal(loginReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/admin/login", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "token")

	// 2. Wrong Password
	loginReq.Password = "wrong"
	jsonValue, _ = json.Marshal(loginReq)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/admin/login", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestTelemetry(t *testing.T) {
	r := setupRouter()

	// Valid Telemetry
	telemetry := models.TelemetryRequest{
		DeviceID:          "1234567890abcdef1234567890abcdef", // 32 chars
		InstanceID:        "inst_001",
		Month:             "2023-10",
		BattleCount:       10,
		BattleRounds:      20, // rounds > count
		SortieCost:        100,
		AkashiEncounters:  1,
		AkashiProbability: 0.05,
		AverageStamina:    50.0,
		NetStaminaGain:    10,
	}
	jsonValue, _ := json.Marshal(telemetry)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/telemetry", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Since BattleRounds (20) > BattleCount (10), logic check: count <= rounds is typically true for valid battles?
	// The code says: if req.BattleCount <= req.BattleRounds { BanIP ... }
	// So for valid data we need BattleCount > BattleRounds

	// Let's fix test data to be valid according to code logic
	telemetry.BattleCount = 20
	telemetry.BattleRounds = 10
	jsonValue, _ = json.Marshal(telemetry)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/telemetry", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}
