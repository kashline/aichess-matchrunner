package rating

import (
	"errors"
	"log"
	"math"

	"gorm.io/gorm"
)

type RatingRepository struct {
	db *gorm.DB
}

func NewRatingRepository(db *gorm.DB) *RatingRepository {
	return &RatingRepository{db: db}
}

func (r *RatingRepository) CreateRating(data Rating) (*Rating, error) {
	Rating := data
	result := r.db.Create(&Rating)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &Rating, nil
}

func (r *RatingRepository) DetermineRating(matchScore int, botID int, firstPlayer bool) (int, error) {
	// Get last rating
	var latestRating Rating
	err := r.db.Where(`"BotID" = ?`, botID).Order(`"createdAt" DESC`).First(&latestRating).Error
	if err != nil {
		if err.Error() == "record not found" {
			var newRating = float64(1500)
			log.Printf("No current rating for bot %v, creating new rating at %v", botID, newRating)
			latestRating.Rating = newRating
		} else {
			return 0, err
		}
	}

	// Determine rating adjustment
	absScore := math.Abs(float64(matchScore))
	var score int
	switch true {
	case absScore >= 500:
		score = 25
	case absScore >= 300:
		score = 15
	case absScore >= 200:
		score = 10
	default:
		score = 5
	}

	// Return the new rating
	if (matchScore < 0 && firstPlayer) || (matchScore > 0 && !firstPlayer) {
		return int(latestRating.Rating) + score, nil
	} else {
		return int(latestRating.Rating) + score*-1, nil
	}
}
