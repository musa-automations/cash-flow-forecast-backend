package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ForecastWeek struct {
	Week    int     `json:"week"`
	Opening float64 `json:"opening"`
	Inflow  float64 `json:"inflow"`
	Outflow float64 `json:"outflow"`
	Closing float64 `json:"closing"`
	Warning bool    `json:"warning"`
}

type Forecast struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	Name         string    `gorm:"type:text;not null" json:"name"`
	StartingCash float64   `json:"starting_cash"`
	CreatedAt    int64     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    int64     `gorm:"autoUpdateTime" json:"updated_at"`
}

func (f *Forecast) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

type ForecastResponse struct {
	ID           uuid.UUID      `json:"id"`
	UserID       uuid.UUID      `json:"user_id"`
	Name         string         `json:"name"`
	StartingCash float64        `json:"starting_cash"`
	Weeks        []ForecastWeek `json:"weeks"`
	CreatedAt    int64          `json:"created_at"`
	UpdatedAt    int64          `json:"updated_at"`
}
