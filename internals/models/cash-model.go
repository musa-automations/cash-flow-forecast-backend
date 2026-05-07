package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CashEntry struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null"`
	ForecastID  uuid.UUID `gorm:"type:uuid;not null"`
	Type        string    `gorm:"type:text;not null;check:type IN ('inflow', 'outflow')"`
	Amount      float64   `gorm:"not null"`
	Category    string    `gorm:"type:text"`
	Description string    `gorm:"type:text"`
	Date        string    `gorm:"type:date;not null"`
	CreatedAt   int64     `gorm:"autoCreateTime"`
	UpdatedAt   int64     `gorm:"autoUpdateTime"`
}

func (c *CashEntry) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}

	return nil
}
