package bot

import (
	"errors"

	"gorm.io/gorm"
)

type BotRepository struct {
	db *gorm.DB
}

func NewBotRepository(db *gorm.DB) *BotRepository {
	return &BotRepository{db: db}
}

func (r *BotRepository) FindOneActiveBot() (*Bot, error) {
	var bot Bot
	result := r.db.Where("remaining_games != 0").Order("RANDOM()").First(&bot)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &bot, nil
}

func (r *BotRepository) FindOneActiveBotExcluding(usedIDs []uint) (*Bot, error) {
	var bot Bot
	query := r.db.Where("remaining_games > 0").Order("RANDOM()")
	if len(usedIDs) > 0 {
		query = query.Where("id NOT IN ?", usedIDs)
	}
	result := query.First(&bot)
	if result.Error != nil {
		return nil, result.Error
	}
	return &bot, nil
}
