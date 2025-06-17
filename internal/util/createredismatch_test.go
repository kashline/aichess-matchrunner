package util

import (
	"aichess-matchrunner/internal/models/bot"
	"context"
	"encoding/json"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestCreateRedisMatch(t *testing.T) {
	// Connect to local Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Clean up the queue before test
	ctx := context.Background()
	rdb.Del(ctx, "matchmaking_queue")

	// Prepare a sample match
	match := Match{
		FirstBot: bot.Bot{
			ID:             1,
			UserID:         100,
			Name:           "BotOne",
			Model:          "gpt-3.5",
			Prompt:         "Aggressive",
			RemainingGames: 10,
		},
		SecondBot: bot.Bot{
			ID:             2,
			UserID:         101,
			Name:           "BotTwo",
			Model:          "gpt-4",
			Prompt:         "Defensive",
			RemainingGames: 5,
		},
		History: []History{
			{Move: "e2e4"},
			{Move: "e7e5"},
		},
		FEN:            "some fen string",
		RemainingTurns: 20,
		PuzzleID:       12345,
	}
	err := CreateRedisMatch(match, rdb)
	if err != nil {
		t.Fatalf("CreateRedisMatch error: %v", err)
	}

	result, err := rdb.LPop(ctx, "matchmaking_queue").Result()
	if err != nil {
		t.Fatalf("Failed to LPop from Redis: %v", err)
	}

	var poppedMatch Match
	if err := json.Unmarshal([]byte(result), &poppedMatch); err != nil {
		t.Fatalf("Failed to unmarshal JSON from Redis: %v", err)
	}

	// Compare key fields to confirm correct data stored
	if poppedMatch.FirstBot.Name != match.FirstBot.Name ||
		poppedMatch.SecondBot.Model != match.SecondBot.Model ||
		len(poppedMatch.History) != len(match.History) ||
		poppedMatch.FEN != match.FEN ||
		poppedMatch.RemainingTurns != match.RemainingTurns ||
		poppedMatch.PuzzleID != match.PuzzleID {
		t.Errorf("Mismatch between pushed and popped match:\nPushed: %+v\nPopped: %+v", match, poppedMatch)
	}
}
