package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/db"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/models"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf(".env file not found, using environment variables")
	}

	db.Connect()

	db.DB.AutoMigrate(&models.User{}, &models.Forecast{}, &models.CashEntry{})
}
