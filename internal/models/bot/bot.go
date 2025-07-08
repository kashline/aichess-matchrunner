package bot

type Bot struct {
	ID             uint `gorm:"primaryKey"`
	UserID         uint
	Name           string
	Model          string
	Prompt         string
	RemainingGames int
}

func (Bot) TableName() string {
	return "Bots"
}
