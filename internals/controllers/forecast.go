package controllers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/db"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/models"
	"github.com/xuri/excelize/v2"
)

const forecastDateLayout = "2006-01-02"

type forecastImportRow struct {
	Type        string
	Amount      float64
	Category    string
	Description string
	Date        string
}

// CreateForecast creates a new forecast with optional initial entries
func CreateForecast(c *gin.Context) {
	userID := c.GetString("user_id")
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var input struct {
		Name         string   `json:"name" binding:"required"`
		StartingDate *string  `json:"starting_date"`
		StartingCash *float64 `json:"starting_cash"`
		Entries      []struct {
			Type        string  `json:"type" binding:"required,oneof=inflow outflow"`
			Amount      float64 `json:"amount" binding:"required"`
			Category    string  `json:"category"`
			Description string  `json:"description"`
			Date        string  `json:"date" binding:"required"`
		} `json:"entries"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startingDate, err := parseOptionalForecastDate(input.StartingDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startingCash := 0.0
	if input.StartingCash != nil {
		startingCash = *input.StartingCash
	}

	// Create forecast
	forecast := models.Forecast{
		UserID:       parsedUserID,
		Name:         input.Name,
		StartingDate: startingDate,
		StartingCash: startingCash,
	}

	if err := db.DB.Create(&forecast).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create forecast"})
		return
	}

	// Create entries if provided
	if len(input.Entries) > 0 {
		var entries []models.CashEntry
		for index, item := range input.Entries {
			normalizedDate, err := normalizeImportDate(item.Date)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("entries[%d].date has invalid format. Use YYYY-MM-DD", index)})
				return
			}

			entry := models.CashEntry{
				UserID:      parsedUserID,
				ForecastID:  forecast.ID,
				Type:        item.Type,
				Amount:      item.Amount,
				Category:    item.Category,
				Description: item.Description,
				Date:        normalizedDate,
			}
			entries = append(entries, entry)
		}

		if err := db.DB.Create(&entries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create entries"})
			return
		}
	}

	c.JSON(http.StatusCreated, forecast)
}

// GetAllForecasts retrieves all forecasts for the authenticated user with generated forecast data
func GetAllForecasts(c *gin.Context) {
	userID := c.GetString("user_id")
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var forecasts []models.Forecast
	if err := db.DB.Where("user_id = ?", parsedUserID).Find(&forecasts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve forecasts"})
		return
	}

	// Generate forecast data for each forecast
	var forecastResponses []models.ForecastResponse
	for _, forecast := range forecasts {
		var entries []models.CashEntry
		if err := db.DB.Where("forecast_id = ?", forecast.ID).Find(&entries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve entries"})
			return
		}

		weeks := generateForecastWeeks(forecast.StartingCash, forecast.StartingDate, entries)
		forecastResponse := models.ForecastResponse{
			ID:           forecast.ID,
			UserID:       forecast.UserID,
			Name:         forecast.Name,
			StartingDate: forecast.StartingDate,
			StartingCash: forecast.StartingCash,
			Weeks:        weeks,
			CreatedAt:    forecast.CreatedAt,
			UpdatedAt:    forecast.UpdatedAt,
		}
		forecastResponses = append(forecastResponses, forecastResponse)
	}

	c.JSON(http.StatusOK, forecastResponses)
}

// GetForecastByID retrieves a specific forecast with generated forecast data
func GetForecastByID(c *gin.Context) {
	userID := c.GetString("user_id")
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	forecastID := c.Param("id")
	parsedForecastID, err := uuid.Parse(forecastID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid forecast ID"})
		return
	}

	var forecast models.Forecast
	if err := db.DB.Where("id = ? AND user_id = ?", parsedForecastID, parsedUserID).First(&forecast).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Forecast not found"})
		return
	}

	// Fetch entries for the forecast
	var entries []models.CashEntry
	if err := db.DB.Where("forecast_id = ?", parsedForecastID).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve entries"})
		return
	}

	// Generate forecast data
	weeks := generateForecastWeeks(forecast.StartingCash, forecast.StartingDate, entries)
	forecastResponse := models.ForecastResponse{
		ID:           forecast.ID,
		UserID:       forecast.UserID,
		Name:         forecast.Name,
		StartingDate: forecast.StartingDate,
		StartingCash: forecast.StartingCash,
		Weeks:        weeks,
		CreatedAt:    forecast.CreatedAt,
		UpdatedAt:    forecast.UpdatedAt,
	}

	c.JSON(http.StatusOK, forecastResponse)
}

// UpdateForecast updates a forecast's name and/or starting cash
func UpdateForecast(c *gin.Context) {
	userID := c.GetString("user_id")
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	forecastID := c.Param("id")
	parsedForecastID, err := uuid.Parse(forecastID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid forecast ID"})
		return
	}

	var input struct {
		Name         string   `json:"name"`
		StartingDate *string  `json:"starting_date"`
		StartingCash *float64 `json:"starting_cash"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startingDate, err := parseOptionalForecastDate(input.StartingDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var forecast models.Forecast
	if err := db.DB.Where("id = ? AND user_id = ?", parsedForecastID, parsedUserID).First(&forecast).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Forecast not found"})
		return
	}

	if input.Name != "" {
		forecast.Name = input.Name
	}
	if input.StartingDate != nil {
		forecast.StartingDate = startingDate
	}
	if input.StartingCash != nil {
		forecast.StartingCash = *input.StartingCash
	}

	if err := db.DB.Save(&forecast).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update forecast"})
		return
	}

	c.JSON(http.StatusOK, forecast)
}

// DeleteForecast deletes a forecast and all its associated entries
func DeleteForecast(c *gin.Context) {
	userID := c.GetString("user_id")
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	forecastID := c.Param("id")
	parsedForecastID, err := uuid.Parse(forecastID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid forecast ID"})
		return
	}

	var forecast models.Forecast
	if err := db.DB.Where("id = ? AND user_id = ?", parsedForecastID, parsedUserID).First(&forecast).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Forecast not found"})
		return
	}

	// Delete all entries for this forecast
	if err := db.DB.Where("forecast_id = ?", parsedForecastID).Delete(&models.CashEntry{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete entries"})
		return
	}

	// Delete the forecast
	if err := db.DB.Delete(&forecast).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete forecast"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Forecast deleted successfully"})
}

// ImportForecastEntries uploads CSV or Excel rows into an existing forecast.
func ImportForecastEntries(c *gin.Context) {
	userID := c.GetString("user_id")
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	forecastID := c.Param("id")
	parsedForecastID, err := uuid.Parse(forecastID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid forecast ID"})
		return
	}

	var forecast models.Forecast
	if err := db.DB.Where("id = ? AND user_id = ?", parsedForecastID, parsedUserID).First(&forecast).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Forecast not found"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".csv" && ext != ".xlsx" && ext != ".xlsm" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only csv and excel files are supported"})
		return
	}

	tempFile, err := os.CreateTemp("", "forecast-import-*"+ext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare upload"})
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if err := c.SaveUploadedFile(fileHeader, tempFile.Name()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save uploaded file"})
		return
	}

	rows, err := readForecastImportRows(tempFile.Name(), ext)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entries := make([]models.CashEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, models.CashEntry{
			UserID:      parsedUserID,
			ForecastID:  parsedForecastID,
			Type:        row.Type,
			Amount:      row.Amount,
			Category:    row.Category,
			Description: row.Description,
			Date:        row.Date,
		})
	}

	if err := db.DB.Create(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import entries"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "entries imported successfully",
		"forecast_id":    parsedForecastID,
		"imported_count": len(entries),
		"entries":        entries,
	})
}

// generateForecastWeeks generates 13 forecast weeks based on starting cash and entries
func generateForecastWeeks(startingCash float64, startingDate *string, entries []models.CashEntry) []models.ForecastWeek {
	// Initialize 13 weeks with zero inflow/outflow
	weeks := make([]struct {
		inflow  float64
		outflow float64
	}, 13)

	anchorDate := time.Now()
	if startingDate != nil {
		parsedAnchorDate, err := time.ParseInLocation(forecastDateLayout, *startingDate, time.Local)
		if err == nil {
			anchorDate = parsedAnchorDate
		}
	}
	anchorDate = time.Date(anchorDate.Year(), anchorDate.Month(), anchorDate.Day(), 0, 0, 0, 0, anchorDate.Location())

	// Group entries by week from the optional starting date (or today when omitted)

	for _, entry := range entries {
		entryDate, err := parseProjectionDate(entry.Date, anchorDate.Location())
		if err != nil {
			// skip invalid dates
			continue
		}

		// only include entries dated on or after the forecast starting date
		if entryDate.Before(anchorDate) {
			continue
		}

		daysDiff := int(entryDate.Sub(anchorDate).Hours() / 24)
		weekIndex := daysDiff / 7

		if weekIndex >= 0 && weekIndex < 13 {
			if entry.Type == "inflow" {
				weeks[weekIndex].inflow += entry.Amount
			} else {
				weeks[weekIndex].outflow += entry.Amount
			}
		}
	}

	// Generate forecast weeks

	forecastWeeks := make([]models.ForecastWeek, 13)
	opening := startingCash

	for i := 0; i < 13; i++ {
		weekStart := anchorDate.AddDate(0, 0, i*7)
		weekEnd := weekStart.AddDate(0, 0, 6)
		closing := opening + weeks[i].inflow - weeks[i].outflow

		forecastWeeks[i] = models.ForecastWeek{
			Week:    i + 1,
			Opening: opening,
			Inflow:  weeks[i].inflow,
			Outflow: weeks[i].outflow,
			Closing: closing,
			EndDate: weekEnd.Format(forecastDateLayout),
			Warning: closing < 0,
		}

		opening = closing
	}

	return forecastWeeks
}

func parseOptionalForecastDate(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	parsedDate, err := time.ParseInLocation(forecastDateLayout, *raw, time.Local)
	if err != nil {
		return nil, fmt.Errorf("starting_date must use YYYY-MM-DD format")
	}

	normalized := parsedDate.Format(forecastDateLayout)
	return &normalized, nil
}

func parseProjectionDate(raw string, location *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}

	layouts := []string{
		forecastDateLayout,
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, location); err == nil {
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, parsed.Location()), nil
		}
	}

	if len(raw) >= len(forecastDateLayout) {
		if parsed, err := time.ParseInLocation(forecastDateLayout, raw[:len(forecastDateLayout)], location); err == nil {
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, parsed.Location()), nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid projection date format")
}

// getWeekIndex returns the week index (0-12) for a given date
// Week 0 = current week, Week 1 = next week, etc.
func getWeekIndex(dateStr string) int {
	// Parse the date string (format: YYYY-MM-DD)
	entryDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return -1
	}

	now := time.Now()
	currentWeekStart := getWeekStart(now)
	entryWeekStart := getWeekStart(entryDate)

	// Calculate the difference in weeks
	daysDiff := entryWeekStart.Sub(currentWeekStart).Hours() / 24
	weekDiff := int(daysDiff / 7)

	return weekDiff
}

// getWeekStart returns the Monday of the week for a given date
func getWeekStart(t time.Time) time.Time {
	// Get day of week (0 = Sunday, 1 = Monday, ..., 6 = Saturday)
	dayOfWeek := int(t.Weekday())
	// Convert Sunday=0 to Monday=1 for easier calculation
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}
	// Subtract (dayOfWeek - 1) days to get to Monday
	daysToSubtract := dayOfWeek - 1
	weekStart := t.AddDate(0, 0, -daysToSubtract)
	// Set to start of day (00:00:00)
	return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
}

func readForecastImportRows(path string, ext string) ([]forecastImportRow, error) {
	var rows [][]string
	var err error

	switch ext {
	case ".csv":
		rows, err = readCSVRows(path)
	case ".xlsx", ".xlsm":
		rows, err = readExcelRows(path)
	default:
		return nil, fmt.Errorf("unsupported file type")
	}

	if err != nil {
		return nil, err
	}

	return parseForecastImportRows(rows)
}

func readCSVRows(path string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	return reader.ReadAll()
}

func readExcelRows(path string) ([][]string, error) {
	workbook, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer workbook.Close()

	sheetNames := workbook.GetSheetList()
	if len(sheetNames) == 0 {
		return nil, fmt.Errorf("excel file does not contain any sheets")
	}

	return workbook.GetRows(sheetNames[0])
}

func parseForecastImportRows(rows [][]string) ([]forecastImportRow, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("file must contain a header row and at least one data row")
	}

	headerIndex := make(map[string]int)
	for index, header := range rows[0] {
		normalizedHeader := normalizeImportHeader(header)
		if normalizedHeader != "" {
			headerIndex[normalizedHeader] = index
		}
	}

	for _, required := range []string{"type", "amount", "date"} {
		if _, ok := headerIndex[required]; !ok {
			return nil, fmt.Errorf("missing required column: %s", required)
		}
	}

	var entries []forecastImportRow
	for rowIndex, row := range rows[1:] {
		if isEmptyImportRow(row) {
			continue
		}

		typeValue := strings.ToLower(strings.TrimSpace(getImportCell(row, headerIndex, "type")))
		if typeValue != "inflow" && typeValue != "outflow" {
			return nil, fmt.Errorf("row %d: type must be inflow or outflow", rowIndex+2)
		}

		amountRaw := strings.ReplaceAll(strings.TrimSpace(getImportCell(row, headerIndex, "amount")), ",", "")
		amount, err := strconv.ParseFloat(amountRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid amount", rowIndex+2)
		}

		dateValue, err := normalizeImportDate(getImportCell(row, headerIndex, "date"))
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowIndex+2, err)
		}

		entries = append(entries, forecastImportRow{
			Type:        typeValue,
			Amount:      amount,
			Category:    getImportCell(row, headerIndex, "category"),
			Description: getImportCell(row, headerIndex, "description"),
			Date:        dateValue,
		})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("file does not contain any valid entries")
	}

	return entries, nil
}

func getImportCell(row []string, headerIndex map[string]int, column string) string {
	index, ok := headerIndex[column]
	if !ok || index >= len(row) {
		return ""
	}

	return strings.TrimSpace(row[index])
}

func normalizeImportHeader(header string) string {
	switch strings.ToLower(strings.TrimSpace(header)) {
	case "type", "entry type":
		return "type"
	case "amount", "value":
		return "amount"
	case "date", "entry date":
		return "date"
	case "category":
		return "category"
	case "description", "notes":
		return "description"
	default:
		return ""
	}
}

func normalizeImportDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("date is required")
	}

	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"02/01/2006",
		"01/02/2006",
		"2/1/2006",
		"1/2/2006",
		"02-Jan-2006",
		"02 Jan 2006",
		"Jan 2, 2006",
	}

	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}

	if serial, err := strconv.ParseFloat(raw, 64); err == nil {
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		days := int(serial)
		fraction := serial - float64(days)
		parsed := excelEpoch.AddDate(0, 0, days).Add(time.Duration(fraction * float64(24*time.Hour)))
		return parsed.Format("2006-01-02"), nil
	}

	return "", fmt.Errorf("invalid date format")
}

func isEmptyImportRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}

	return true
}
