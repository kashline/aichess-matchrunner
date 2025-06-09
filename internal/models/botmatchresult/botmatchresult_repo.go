package botmatchresult

import (
	"errors"

	"gorm.io/gorm"
)

type BotMatchResultRepository struct {
	db *gorm.DB
}

func NewBotMatchResultRepository(db *gorm.DB) *BotMatchResultRepository {
	return &BotMatchResultRepository{db: db}
}

func (r *BotMatchResultRepository) CreateResult(data BotMatchResult) (*BotMatchResult, error) {
	botMatchResult := data
	result := r.db.Create(&botMatchResult)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &botMatchResult, nil
}
