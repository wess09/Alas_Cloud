package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupForceUpdateRouter(t *testing.T) *gin.Engine {
	t.Helper()

	previousDB := database.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SystemConfig{}))
	database.DB = db
	t.Cleanup(func() {
		database.DB = previousDB
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/force_update", GetForceUpdateStatus)
	r.GET("/api/admin/config/force_update", AdminGetForceUpdateStatus)
	r.PATCH("/api/admin/config/force_update", AdminToggleForceUpdate)
	return r
}

func TestForceUpdateConfigAPI(t *testing.T) {
	r := setupForceUpdateRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/force_update", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "false", w.Body.String())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/api/admin/config/force_update?is_active=true", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/force_update", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Body.String())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/config/force_update", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"force_update":true}`, w.Body.String())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/api/admin/config/force_update?is_active=false", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/force_update", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "false", w.Body.String())
}
