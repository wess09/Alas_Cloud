package handlers

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type cachedJSONResponse struct {
	payload   []byte
	expiresAt time.Time
}

var (
	responseCache     sync.Map
	telemetryCacheGen atomic.Uint64
	azurstatCacheGen  atomic.Uint64
)

func telemetryCacheKey(name string, c *gin.Context) string {
	return name + "|v=" + uintToString(telemetryCacheGen.Load()) + "|q=" + c.Request.URL.RawQuery
}

func azurstatCacheKey(name string, c *gin.Context) string {
	return name + "|v=" + uintToString(azurstatCacheGen.Load()) + "|q=" + c.Request.URL.RawQuery
}

func invalidateTelemetryCache() {
	telemetryCacheGen.Add(1)
}

func invalidateAzurstatCache() {
	azurstatCacheGen.Add(1)
}

func InvalidateTelemetryCache() {
	invalidateTelemetryCache()
}

func InvalidateAzurstatCache() {
	invalidateAzurstatCache()
}

func serveCachedJSON(c *gin.Context, key string, ttl time.Duration, builder func() (any, error)) bool {
	now := time.Now()
	if cached, ok := responseCache.Load(key); ok {
		entry := cached.(cachedJSONResponse)
		if now.Before(entry.expiresAt) {
			c.Data(200, "application/json; charset=utf-8", entry.payload)
			return true
		}
		responseCache.Delete(key)
	}

	response, err := builder()
	if err != nil {
		return false
	}

	payload, err := json.Marshal(response)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to encode response"})
		return true
	}

	responseCache.Store(key, cachedJSONResponse{
		payload:   payload,
		expiresAt: now.Add(ttl),
	})
	c.Data(200, "application/json; charset=utf-8", payload)
	return true
}

func uintToString(v uint64) string {
	if v == 0 {
		return "0"
	}

	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(buf[i:])
}
