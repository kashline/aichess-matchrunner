package bot

import "time"

type Bot struct {
	ID             uint      `gorm:"primaryKey"`
	UserID         uint      `gorm:"column:UserID"`
	Name           string    `gorm:"column:Name"`
	Model          string    `gorm:"column:Model"`
	Prompt         string    `gorm:"column:Prompt"`
	RemainingGames int       `gorm:"column:RemainingGames"`
	CreatedAt      time.Time `gorm:"column:createdAt;type:timestamptz"`
	UpdatedAt      time.Time `gorm:"column:updatedAt;type:timestamptz"`
}

func (Bot) TableName() string {
	return "Bots"
}
