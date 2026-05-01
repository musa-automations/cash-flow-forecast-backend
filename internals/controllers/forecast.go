package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/db"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/models"
)

// GetForecast generates a 13-week cash flow forecast
func GetForecast(c *gin.Context) {
	userID := c.GetString("user_id")
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get starting cash from query parameter
	startingCashStr := c.DefaultQuery("startingCash", "0")
	startingCash, err := strconv.ParseFloat(startingCashStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid starting cash value"})
		return
	}

	// Fetch all cash entries for the user
	var entries []models.CashEntry
	if err := db.DB.Where("user_id = ?", parsedUserID).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve entries"})
		return
	}

	// Generate forecast
	forecast := generateForecast(startingCash, entries)

	c.JSON(http.StatusOK, forecast)
}

// generateForecast creates a 13-week forecast based on starting cash and entries
func generateForecast(startingCash float64, entries []models.CashEntry) models.Forecast {
	// Initialize 13 weeks with zero inflow/outflow
	weeks := make([]struct {
		inflow  float64
		outflow float64
	}, 13)

	// Group entries by week (only future-dated entries: entry.date >= today)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	currentWeekStart := getWeekStart(now)

	for _, entry := range entries {
		entryDate, err := time.ParseInLocation("2006-01-02", entry.Date, now.Location())
		if err != nil {
			// skip invalid dates
			continue
		}

		// only include entries dated today or in the future
		if entryDate.Before(today) {
			continue
		}

		entryWeekStart := getWeekStart(entryDate)
		daysDiff := entryWeekStart.Sub(currentWeekStart).Hours() / 24
		weekIndex := int(daysDiff / 7)

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
		closing := opening + weeks[i].inflow - weeks[i].outflow

		forecastWeeks[i] = models.ForecastWeek{
			Week:    i + 1,
			Opening: opening,
			Inflow:  weeks[i].inflow,
			Outflow: weeks[i].outflow,
			Closing: closing,
			Warning: closing < 0,
		}

		opening = closing
	}

	return models.Forecast{
		StartingCash: startingCash,
		Weeks:        forecastWeeks,
	}
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
