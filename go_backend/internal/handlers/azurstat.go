package handlers

import (
	"alas-cloud/internal/database"
	"alas-cloud/internal/models"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var azurstatAllowedTasks = map[string]struct{}{
	"opsi_hazard1_leveling":   {},
	"opsi_meowfficer_farming": {},
}

func SubmitAzurstat(c *gin.Context) {
	var req models.AzurstatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !isValidAzurstatDeviceID(req.DeviceID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device_id"})
		return
	}

	if _, ok := azurstatAllowedTasks[req.Task]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task"})
		return
	}

	if req.Body.HazardLevel < 1 || req.Body.HazardLevel > 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hazard_level must be between 1 and 6"})
		return
	}

	if req.Body.CombatCount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "combat_count must be greater than 0"})
		return
	}

	if len(req.Body.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items must contain at least one item"})
		return
	}

	itemDrops := make([]models.AzurstatItemDrop, 0, len(req.Body.Items))
	for _, item := range req.Body.Items {
		if strings.TrimSpace(item.Item) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "item name is required"})
			return
		}
		if item.Amount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "item amount must be greater than 0"})
			return
		}
		itemDrops = append(itemDrops, models.AzurstatItemDrop{
			Item:   strings.TrimSpace(item.Item),
			Amount: item.Amount,
			IsMeow: item.IsMeow,
		})
	}

	report := models.AzurstatReport{
		DeviceID:    req.DeviceID,
		Task:        req.Task,
		Zone:        strings.TrimSpace(req.Body.Zone),
		ZoneType:    strings.TrimSpace(req.Body.ZoneType),
		ZoneID:      strings.TrimSpace(req.Body.ZoneID),
		HazardLevel: req.Body.HazardLevel,
		CombatCount: req.Body.CombatCount,
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&report).Error; err != nil {
			return err
		}
		for i := range itemDrops {
			itemDrops[i].ReportID = report.ID
		}
		return tx.Create(&itemDrops).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"message":      "AzurStat report saved",
		"report_id":    report.ID,
		"device_id":    report.DeviceID,
		"item_count":   len(itemDrops),
		"created_at":   report.CreatedAt,
		"combat_count": report.CombatCount,
	})
}

func GetAzurstatStats(c *gin.Context) {
	reportQuery, itemQuery, err := buildAzurstatQueries(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type statsResult struct {
		TotalReports     int64 `json:"total_reports"`
		TotalDevices     int64 `json:"total_devices"`
		TotalCombatCount int64 `json:"total_combat_count"`
	}

	type itemResult struct {
		TotalItemAmount int64 `json:"total_item_amount"`
		TotalItemTypes  int64 `json:"total_item_types"`
	}

	var stats statsResult
	if err := reportQuery.Select(`
		COUNT(azurstat_reports.id) as total_reports,
		COUNT(DISTINCT azurstat_reports.device_id) as total_devices,
		COALESCE(SUM(azurstat_reports.combat_count), 0) as total_combat_count
	`).Scan(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
		return
	}

	var itemStats itemResult
	if err := itemQuery.Select(`
		COALESCE(SUM(azurstat_item_drops.amount), 0) as total_item_amount,
		COUNT(DISTINCT azurstat_item_drops.item) as total_item_types
	`).Scan(&itemStats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch item stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_reports":      stats.TotalReports,
		"total_devices":      stats.TotalDevices,
		"total_combat_count": stats.TotalCombatCount,
		"total_item_amount":  itemStats.TotalItemAmount,
		"total_item_types":   itemStats.TotalItemTypes,
	})
}

func GetAzurstatItems(c *gin.Context) {
	reportQuery, itemQuery, err := buildAzurstatQueries(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var totalCombat struct {
		TotalCombatCount int64
	}
	if err := reportQuery.Select("COALESCE(SUM(azurstat_reports.combat_count), 0) as total_combat_count").Scan(&totalCombat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch combat totals"})
		return
	}

	limit := 50
	if rawLimit := c.DefaultQuery("limit", "50"); rawLimit != "" {
		parsedLimit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsedLimit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		if parsedLimit > 200 {
			parsedLimit = 200
		}
		limit = parsedLimit
	}

	type itemRow struct {
		Item         string `json:"item"`
		TotalAmount  int64  `json:"total_amount"`
		DropReports  int64  `json:"drop_reports"`
		MeowAmount   int64  `json:"meow_amount"`
		NormalAmount int64  `json:"normal_amount"`
	}

	var rows []itemRow
	if err := itemQuery.Select(`
		azurstat_item_drops.item as item,
		COALESCE(SUM(azurstat_item_drops.amount), 0) as total_amount,
		COUNT(DISTINCT azurstat_item_drops.report_id) as drop_reports,
		COALESCE(SUM(CASE WHEN azurstat_item_drops.is_meow THEN azurstat_item_drops.amount ELSE 0 END), 0) as meow_amount,
		COALESCE(SUM(CASE WHEN azurstat_item_drops.is_meow THEN 0 ELSE azurstat_item_drops.amount END), 0) as normal_amount
	`).Group("azurstat_item_drops.item").Order("total_amount DESC, azurstat_item_drops.item ASC").Limit(limit).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch items"})
		return
	}

	response := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		avgPerCombat := 0.0
		if totalCombat.TotalCombatCount > 0 {
			avgPerCombat = float64(row.TotalAmount) / float64(totalCombat.TotalCombatCount)
		}
		response = append(response, gin.H{
			"item":           row.Item,
			"total_amount":   row.TotalAmount,
			"drop_reports":   row.DropReports,
			"meow_amount":    row.MeowAmount,
			"normal_amount":  row.NormalAmount,
			"avg_per_combat": avgPerCombat,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items": response,
		"total": len(response),
	})
}

func GetAzurstatHistory(c *gin.Context) {
	reportQuery, _, err := buildAzurstatQueries(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	interval := c.DefaultQuery("interval", "day")
	dateExpr, err := azurstatDateExpression(interval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	itemTotalsQuery := database.DB.Table("azurstat_item_drops").
		Select("report_id, COALESCE(SUM(amount), 0) as item_amount").
		Group("report_id")

	if isMeowRaw := strings.TrimSpace(c.Query("is_meow")); isMeowRaw != "" {
		isMeow, parseErr := strconv.ParseBool(isMeowRaw)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid is_meow"})
			return
		}
		itemTotalsQuery = itemTotalsQuery.Where("is_meow = ?", isMeow)
	}

	baseQuery := reportQuery.Joins("LEFT JOIN (?) as item_totals ON item_totals.report_id = azurstat_reports.id", itemTotalsQuery)

	type historyRow struct {
		Date        string `json:"date"`
		ReportCount int64  `json:"report_count"`
		CombatCount int64  `json:"combat_count"`
		ItemAmount  int64  `json:"item_amount"`
		DeviceCount int64  `json:"device_count"`
	}

	var rows []historyRow
	selectClause := fmt.Sprintf(`%s as date,
		COUNT(azurstat_reports.id) as report_count,
		COALESCE(SUM(azurstat_reports.combat_count), 0) as combat_count,
		COALESCE(SUM(COALESCE(item_totals.item_amount, 0)), 0) as item_amount,
		COUNT(DISTINCT azurstat_reports.device_id) as device_count`, dateExpr)

	if err := baseQuery.Select(selectClause).
		Group("date").
		Order("date DESC").
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"interval": interval,
		"history":  rows,
	})
}

func GetAzurstatFilters(c *gin.Context) {
	var tasks []string
	if err := database.DB.Model(&models.AzurstatReport{}).Distinct().Order("task ASC").Pluck("task", &tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tasks"})
		return
	}

	var zones []string
	if err := database.DB.Model(&models.AzurstatReport{}).
		Where("zone_id <> ''").Distinct().Order("zone_id ASC").Pluck("zone_id", &zones).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch zones"})
		return
	}

	var hazardLevels []int
	if err := database.DB.Model(&models.AzurstatReport{}).
		Distinct().Order("hazard_level ASC").Pluck("hazard_level", &hazardLevels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hazard levels"})
		return
	}

	hazardLevelSet := make(map[int]struct{}, len(hazardLevels))
	uniqueHazardLevels := make([]int, 0, len(hazardLevels))
	for _, level := range hazardLevels {
		if _, exists := hazardLevelSet[level]; exists {
			continue
		}
		hazardLevelSet[level] = struct{}{}
		uniqueHazardLevels = append(uniqueHazardLevels, level)
	}
	sort.Ints(uniqueHazardLevels)

	c.JSON(http.StatusOK, gin.H{
		"tasks":         tasks,
		"zones":         zones,
		"hazard_levels": uniqueHazardLevels,
	})
}

func buildAzurstatQueries(c *gin.Context) (*gorm.DB, *gorm.DB, error) {
	reportQuery := database.DB.Model(&models.AzurstatReport{})
	itemQuery := database.DB.Table("azurstat_item_drops").Joins("JOIN azurstat_reports ON azurstat_reports.id = azurstat_item_drops.report_id")

	if task := strings.TrimSpace(c.Query("task")); task != "" {
		reportQuery = reportQuery.Where("azurstat_reports.task = ?", task)
		itemQuery = itemQuery.Where("azurstat_reports.task = ?", task)
	}

	if zoneID := strings.TrimSpace(c.Query("zone_id")); zoneID != "" {
		reportQuery = reportQuery.Where("azurstat_reports.zone_id = ?", zoneID)
		itemQuery = itemQuery.Where("azurstat_reports.zone_id = ?", zoneID)
	}

	if hazardLevelRaw := strings.TrimSpace(c.Query("hazard_level")); hazardLevelRaw != "" {
		hazardLevel, err := strconv.Atoi(hazardLevelRaw)
		if err != nil || hazardLevel < 1 || hazardLevel > 6 {
			return nil, nil, errInvalidAzurstatFilter("invalid hazard_level")
		}
		reportQuery = reportQuery.Where("azurstat_reports.hazard_level = ?", hazardLevel)
		itemQuery = itemQuery.Where("azurstat_reports.hazard_level = ?", hazardLevel)
	}

	if isMeowRaw := strings.TrimSpace(c.Query("is_meow")); isMeowRaw != "" {
		isMeow, err := strconv.ParseBool(isMeowRaw)
		if err != nil {
			return nil, nil, errInvalidAzurstatFilter("invalid is_meow")
		}
		itemQuery = itemQuery.Where("azurstat_item_drops.is_meow = ?", isMeow)
	}

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		parsed, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			return nil, nil, errInvalidAzurstatFilter("invalid start_date")
		}
		reportQuery = reportQuery.Where("azurstat_reports.created_at >= ?", parsed)
		itemQuery = itemQuery.Where("azurstat_reports.created_at >= ?", parsed)
	}

	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		parsed, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			return nil, nil, errInvalidAzurstatFilter("invalid end_date")
		}
		endOfDay := parsed.Add(24*time.Hour - time.Nanosecond)
		reportQuery = reportQuery.Where("azurstat_reports.created_at <= ?", endOfDay)
		itemQuery = itemQuery.Where("azurstat_reports.created_at <= ?", endOfDay)
	}

	return reportQuery, itemQuery, nil
}

func azurstatDateExpression(interval string) (string, error) {
	switch interval {
	case "day":
		return "strftime('%Y-%m-%d', azurstat_reports.created_at)", nil
	case "month":
		return "strftime('%Y-%m', azurstat_reports.created_at)", nil
	default:
		return "", errInvalidAzurstatFilter("invalid interval")
	}
}

func isValidAzurstatDeviceID(deviceID string) bool {
	match, _ := regexp.MatchString("^[a-fA-F0-9]{32,64}$", deviceID)
	return match
}

type azurstatFilterError string

func (e azurstatFilterError) Error() string {
	return string(e)
}

func errInvalidAzurstatFilter(message string) error {
	return azurstatFilterError(message)
}
