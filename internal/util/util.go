package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/signal"
	"sync"
	"syscall"

	"log"
	"net/http"

	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"aichess-matchrunner/internal/models/bot"
	"aichess-matchrunner/internal/models/botmatchresult"
	"aichess-matchrunner/internal/models/rating"

	"github.com/invopop/jsonschema"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Match struct {
	FirstBot       bot.Bot   `json:"firstBot"`
	SecondBot      bot.Bot   `json:"secondBot"`
	History        []History `json:"history"`
	FEN            string    `json:"fen"`
	RemainingTurns int       `json:"remainingTurns"`
	PuzzleID       int       `json:"puzzleId"`
}

type History struct {
	Move  string `json:"move"`
	Score int    `json:"score"`
}

type Score struct {
	BestMove        string `json:"best_move"`
	PlayerMove      string `json:"player_move"`
	Rating          string `json:"rating"`
	ScoreDifference int    `json:"score_difference"`
}

type BotResponse struct {
	Move int
}

// StartWorker starts the worker loop. Matches are read from a queue and processed, either writing back to redis or the database.  Spawns go routines for each found match.
func StartWorker(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	defer cancel()
	defer wg.Done()
	// Setup db connections
	rdb := redis.NewClient(GetRedisOptions())
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=host.docker.internal user=postgres password=postgres dbname=yourdb port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("failed to connect to db %v", err)
	}

	// Concurrency stuff
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	maxThreads := 2
	sem := make(chan struct{}, maxThreads)

	// Main worker loop
	for {
		// Get a game from the queue
		log.Println("Fetching a game")
		result, err := rdb.BRPop(ctx, 5*time.Second, "matchmaking_queue").Result()
		if err != nil {
			log.Print("Error while reading from queue:", err)
			if err.Error() == "redis: nil" {
				log.Print("No matches found. Exiting..")
				os.Exit(0)
			}
			time.Sleep(10 * time.Second)
			continue
		}
		// Update semaphore
		sem <- struct{}{}
		wg.Add(1)
		go func(matchData string) {
			// Concurrency
			defer func() { <-sem }()
			defer wg.Done()

			// Create match object
			var match Match
			if err := json.Unmarshal([]byte(matchData), &match); err != nil {
				log.Printf("failed to decode game: %v", err)
			}

			// Create list of valid moves from stockfish
			validMoves, err := GenerateValidMoves(match.FEN)
			if err != nil {
				log.Printf("failed to generate moves %v", err)
			}
			if len(validMoves) == 0 {
				log.Print("No valid moves! Game must be over...")
				time.Sleep(60 * time.Second)
			}

			// Get current turn
			turn, err := GetTurnFromFEN(match.FEN)
			if err != nil {
				log.Printf("failed getting turnfromfen")
			}

			// Set request
			var model openai.ChatModel
			var prompt string
			if turn == "white" {
				model = openai.ChatModel(match.FirstBot.Model)
				prompt = match.FirstBot.Prompt
			} else {
				model = openai.ChatModel(match.SecondBot.Model)
				prompt = match.SecondBot.Prompt
			}

			// Generate move
			log.Println("Generating move")
			move, err := GenerateMove(model, match.FEN, validMoves, prompt)
			if err != nil {
				log.Printf("error generating move %v", err)
			}

			// LLM responded with invalid index
			if move == "" {
				log.Print("Invalid move, retrying")
				CreateRedisMatch(match, rdb)
				return
			}

			// Make move
			newFen, score, err := MakeMove(match.FEN, move)
			if err != nil {
				log.Printf("failed to make move %v", err)
			}
			history := History{
				Move:  move,
				Score: score,
			}

			// Update match
			match.FEN = newFen
			match.History = append(match.History, history)
			match.RemainingTurns--

			// Determine if this was the last turn.
			// If so, write the result to the database and don't write to redis.

			// This was the last turn
			if match.RemainingTurns <= 0 {
				log.Println("game over turn limit reached")
				err = WriteResult(match, db)
				if err != nil {
					log.Printf("error writing result! %v", err)
				}
				return
			}
			// This was not the last turn, so write to redis
			err = CreateRedisMatch(match, rdb)
			if err != nil {
				log.Printf("failed to update redis match %v", err)
			}
			log.Printf("redis updated. turns remaining: %v", match.RemainingTurns)
		}(result[1])
	}
}

// GetRedisOptions returns redis client options based on environment
func GetRedisOptions() *redis.Options {
	url := os.Getenv("REDIS_URL")
	if url != "" {
		opt, err := redis.ParseURL(url)
		if err == nil {
			return opt
		}
		log.Printf("failed to parse REDIS_URL: %v, falling back to host.docker.internal", err)
	} else {
		log.Printf("REDIS_URL not set, defaulting to host.docker.internal")
	}
	return &redis.Options{
		Addr: "host.docker.internal:6379",
	}
}

func Healthz() bool {
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "host.docker.internal:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisUrl,
	})
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=host.docker.internal user=postgres password=postgres dbname=yourdb port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("failed to connect to db %v", err)
		return false
	}
	if rdb == nil || db == nil {
		log.Printf("Something went wrong checking downstream dependencies. rdb: %v, db: %v, err: %v", rdb, db, err)
		return false
	}
	return true
}

// GenerateMove generates the next chess move for the given bot parameters
func GenerateMove(model openai.ChatModel, fen string, moves []string, prompt string) (string, error) {
	type MoveResponse struct {
		Move int `json:"Move"`
	}
	client := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")))
	var BotResponseSchema = GenerateSchema[BotResponse]()
	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "chess_move",
		Description: openai.String("Chess move choice"),
		Schema:      BotResponseSchema,
		Strict:      openai.Bool(true),
	}
	chat, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(fmt.Sprintf("This fen represents a game of chess that we are currently playing: %v.  Here is an array of possible moves: [%v].  Select a move from this list and respond with the ZERO-BASED index the move has in the array.  Do not choose a number larger than %v.  Here are additional details: %v", fen, moves, len(moves)-1, prompt)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
		Model: model,
	})
	if err != nil {
		panic(err.Error())
	}
	var move MoveResponse
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &move)
	if err != nil {
		panic(err.Error())
	}
	if move.Move >= 0 && move.Move < len(moves) {
		return moves[move.Move], nil
	} else {
		return "", nil
	}
}

// GenerateSchema creates the schema for structured outputs
func GenerateSchema[T any]() any {
	// Structured Outputs uses a subset of JSON schema
	// These flags are necessary to comply with the subset
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

// GetTurnFromFEN returns the current turn for the given FEN
func GetTurnFromFEN(fen string) (string, error) {
	parts := strings.Split(fen, " ")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid FEN string")
	}

	switch parts[1] {
	case "w":
		return "white", nil
	case "b":
		return "black", nil
	default:
		return "", fmt.Errorf("unknown turn indicator: %s", parts[1])
	}
}

// GenerateValidMoves returns a list of valid moves for the provided FEN
func GenerateValidMoves(fen string) ([]string, error) {
	type ValidMoves struct {
		Legal_Moves []string `json:"legal_moves"`
	}
	var validMoves ValidMoves
	emptyReturn := []string{""}
	bodyData := map[string]string{
		"fen": fen,
	}
	jsonBody, err := json.Marshal(bodyData)
	if err != nil {
		log.Printf("failed to marshal JSON: %v", err)
		return emptyReturn, err
	}
	res, err := SendStockfishRequest("/legal_moves", jsonBody, "POST")
	if err != nil {
		log.Printf("failed to send stockfish request %v", err)
	}
	err = json.Unmarshal([]byte(res), &validMoves)
	if err != nil {
		return emptyReturn, err
	}
	return validMoves.Legal_Moves, nil
}

// MakeMove uses stockfish to execute and score the given move.
func MakeMove(fen string, move string) (string, int, error) {
	type response struct {
		FEN string `json:"fen"`
	}
	bodyData := map[string]string{
		"fen":  fen,
		"move": move,
	}
	defaultReturn := func(err error) (string, int, error) {
		return "", -1, err
	}
	jsonBody, err := json.Marshal(bodyData)
	if err != nil {
		log.Printf("failed to marshal JSON: %v", err)
		return defaultReturn(err)
	}
	var newFenParsed response
	newFen, err := SendStockfishRequest("/move", jsonBody, "POST")
	if err != nil {
		log.Printf("failed to send stockfish request %v", err)
		return defaultReturn(err)
	}
	err = json.Unmarshal([]byte(newFen), &newFenParsed)
	if err != nil {
		log.Println("failed to unmarshal new fen")
		return defaultReturn(err)
	}
	// score move
	score, err := SendStockfishRequest("/evaluatemove", jsonBody, "POST")
	if err != nil {
		log.Println("score not set for some reason")
		return newFenParsed.FEN, -1, nil
	}
	var scoreParsed Score
	err = json.Unmarshal([]byte(score), &scoreParsed)
	if err != nil {
		log.Printf("failed to convert score to int: %v", err)
		return newFenParsed.FEN, -1, nil
	}
	return newFenParsed.FEN, scoreParsed.ScoreDifference, nil
}

// SendStockfishRequest handles sending a request to the stockfish api
var SendStockfishRequest = func(stem string, payload []byte, method string) (string, error) {
	url := os.Getenv("STOCKFISH_URL") + stem
	if url == "" {
		url = "http://host.docker.internal:5001" + stem
	}
	// Use gcp auth in prod
	if os.Getenv("IS_PROD") == "true" {
		data, err := SendAuthenticatedRequest(method, url, payload, map[string]string{"Content-Type": "application/json"})
		if err != nil {
			log.Printf("failed to send authenticated request to stockfish: %v", err)
		}
		return string(data), err
	}
	// Not prod so no auth
	req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	return string(res), nil
}

// CreateRedisMatch creates a match in the redis queue
func CreateRedisMatch(match Match, rdb *redis.Client) error {
	ctx := context.Background()
	matchJSON, err := json.Marshal(match)
	if err != nil {
		return err
	}
	err = rdb.LPush(ctx, "matchmaking_queue", matchJSON).Err()
	if err != nil {
		return err
	}
	return nil
}

// WriteResult writes the match result and rating change to postgres
func WriteResult(match Match, db *gorm.DB) error {
	botmatchresultRepository := botmatchresult.NewBotMatchResultRepository(db)
	historyBytes, err := json.Marshal(match.History)
	if err != nil {
		log.Print("error marshaling history")
		return err
	}
	score := DetermineScore(match.History)
	jsonData := botmatchresult.BotMatchResult{
		BotOneID: int(match.FirstBot.ID),
		BotTwoID: int(match.SecondBot.ID),
		History:  historyBytes,
		Score:    score,
		PuzzleID: int(match.PuzzleID),
	}

	result, err := botmatchresultRepository.CreateResult(jsonData)
	if err != nil {
		log.Printf("error writing match result: %v", err)
		return err
	}
	ratingReposity := rating.NewRatingRepository(db)
	ratingChange, err := ratingReposity.DetermineRating(score, int(match.FirstBot.ID))
	if err != nil {
		log.Printf("issue determining rating: %v", err)
	}
	ratingData := rating.Rating{
		MatchID: int(result.ID),
		Rating:  float64(ratingChange),
		BotID:   int(match.FirstBot.ID),
	}
	_, err = ratingReposity.CreateRating(ratingData)
	if err != nil {
		log.Printf("error creating rating: %v", err)
		return err
	}
	return nil
}

// DetermineScore returns the score for a match. The overall score denotes who 'won' the game, positive=black, negative=white.
func DetermineScore(history []History) int {
	acc := 0
	for index, history := range history {
		if index%2 == 0 {
			acc += history.Score
		} else {
			acc -= history.Score
		}
	}
	return acc
}

// SendAuthenticatedRequest sends a request with a GCP ID token (retrieved from the metadata server).
// It works when running inside GCP (e.g., Cloud Run, GCE, GKE).
func SendAuthenticatedRequest(
	method string,
	url string,
	body []byte,
	headers map[string]string, // optional additional headers
) ([]byte, error) {
	// Step 1: Get ID token for target URL (audience must match the service)
	tokenURL := "http://metadata/computeMetadata/v1/instance/service-accounts/default/identity?audience=" + url
	tokenReq, _ := http.NewRequest("GET", tokenURL, nil)
	tokenReq.Header.Add("Metadata-Flavor", "Google")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	tokenResp, err := httpClient.Do(tokenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ID token: %w", err)
	}
	defer tokenResp.Body.Close()

	idTokenBytes, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ID token: %w", err)
	}
	idToken := string(idTokenBytes)

	// Step 2: Make the authenticated request
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("Content-Type", "application/json")

	// Optional extra headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authenticated request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
