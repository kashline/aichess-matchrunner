package rating

import "time"

type Rating struct {
	ID        uint      `gorm:"primaryKey"`
	MatchID   int       `gorm:"column:MatchID"`
	Rating    float64   `gorm:"column:Rating"`
	BotID     int       `gorm:"column:BotID"`
	CreatedAt time.Time `gorm:"column:createdAt"`
}

func (Rating) TableName() string {
	return "Ratings"
}
