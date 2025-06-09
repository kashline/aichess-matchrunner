package botmatchresult

import (
	"gorm.io/datatypes"
)

type BotMatchResult struct {
	ID       uint           `gorm:"primaryKey"`
	BotOneID int            `gorm:"column:BotOneID"`
	BotTwoID int            `gorm:"column:BotTwoID"`
	History  datatypes.JSON `gorm:"column:History"`
	Score    int            `gorm:"column:Score"`
	PuzzleID int            `gorm:"column:PuzzleID"`
}

func (BotMatchResult) TableName() string {
	return "BotMatchResults"
}
