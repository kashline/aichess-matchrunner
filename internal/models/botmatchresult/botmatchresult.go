package botmatchresult

import (
	"time"

	"gorm.io/datatypes"
)

type BotMatchResult struct {
	ID        uint           `gorm:"primaryKey"`
	BotOneID  int            `gorm:"column:BotOneID"`
	BotTwoID  int            `gorm:"column:BotTwoID"`
	History   datatypes.JSON `gorm:"column:History"`
	Score     int            `gorm:"column:Score"`
	PuzzleID  int            `gorm:"column:PuzzleID"`
	CreatedAt time.Time      `gorm:"column:createdAt"`
	UpdatedAt time.Time      `gorm:"column:updatedAt"`
}

func (BotMatchResult) TableName() string {
	return "BotMatchResults"
}
