package main

import (
	"log"
	"math"
	"os"

	"aichess-matchrunner/internal/models/botmatchresult"
	"aichess-matchrunner/internal/models/rating"
	"aichess-matchrunner/internal/util"

	"context"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// UpdateRatings()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go util.StartWorker(ctx, cancel, &wg)
	wg.Wait()
}

func UpdateRatings() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=host.docker.internal user=postgres password=postgres dbname=yourdb port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("failed to connect to db %v", err)
	}
	var ratingModel rating.Rating
	err = db.AutoMigrate(&ratingModel)
	if err != nil {
		log.Printf("err: %v", err)
	}

	var botMatchResults []botmatchresult.BotMatchResult
	result := db.Find(&botMatchResults)
	if result.Error != nil {
		log.Printf("Error finding bots: %v", result.Error)
	} else {
		log.Printf("Found %d bots\n", len(botMatchResults))
		ratings := make(map[int]int)
		for _, botMatchResult := range botMatchResults {
			var latestRating int
			// var latestScore float64
			// Get last rating
			// err := db.Where(`"BotID" = ?`, botMatchResult.BotOneID).Order(`"createdAt" DESC`).First(&latestRating).Error
			// if err != nil {
			// 	if err.Error() == "record not found" {
			// 		log.Printf("no current rating, creating..")
			// 		latestScore = 1500
			// 	} else {
			// 		log.Printf("error finding latest rating: %v", err)
			// 	}
			// } else {
			// 	latestScore = latestRating.Rating
			// }

			latestRating = ratings[botMatchResult.BotOneID]
			if latestRating == 0 {
				latestRating = 1500
			}

			// Determine score
			var score int
			absScore := math.Abs(float64(botMatchResult.Score))
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
			var newRating int
			if botMatchResult.Score < 0 {
				newRating = latestRating - score
			} else {
				newRating = latestRating + score
			}
			ratings[botMatchResult.BotOneID] = newRating
			log.Printf("Old rating: %v Score: %v New Rating: %v", latestRating, score, newRating)
		}
		for botID, newRating := range ratings {
			writeRating := rating.Rating{
				MatchID: 0,
				Rating:  float64(newRating),
				BotID:   botID,
			}
			db.Create(&writeRating)
		}
	}
}
