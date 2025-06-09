package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"log"
	"net/http"

	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"aichess-matchrunner/internal/models/bot"
	"aichess-matchrunner/internal/models/botmatchresult"

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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rdb := redis.NewClient(&redis.Options{
		Addr: "host.docker.internal:6379",
	})
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=host.docker.internal user=postgres password=postgres dbname=yourdb port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("failed to connect to db %v", err)
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	// Concurrency semaphore
	maxThreads := runtime.NumCPU()
	sem := make(chan struct{}, maxThreads)
	var activeMatches sync.Map
	go func() {
		<-sigChan
		log.Println("Signal received, shutting down...")
		cancel()
	}()
mainLoop:
	for {
		var lastMatch *Match
		select {
		// Shutdown cleanup
		case <-ctx.Done():
			log.Println("Cleanup during shutdown...")
			log.Println("Shutting down active matches...")
			activeMatches.Range(func(_, value any) bool {
				match := value.(Match)
				log.Printf("Cleaning up match FEN: %s\n", match.FEN)
				createRedisMatch(match, rdb)
				return true
			})
			break mainLoop
		default:
			// Get a game from the queue
			log.Println("Fetching a game")
			result, err := rdb.BRPop(ctx, 5*time.Second, "matchmaking_queue").Result()
			if err != nil {
				fmt.Println("Error while reading from queue:", err)
				if err.Error() == "redis: nil" && !(lastMatch == nil) {
					lastMatch = nil
				}
				time.Sleep(10 * time.Second)
				continue
			}
			sem <- struct{}{}
			log.Printf("Slot acquired, executing... Current processes: %v", len(sem))
			go func(matchData string) {
				defer func() { <-sem }()
				var match Match
				if err := json.Unmarshal([]byte(matchData), &match); err != nil {
					log.Printf("failed to decode game: %v", err)
				}
				matchID := uuid.New().String()
				activeMatches.Store(matchID, match)

				defer func() {
					<-sem
					activeMatches.Delete(matchID)
				}()
				validMoves, err := generateValidMoves(match.FEN)
				if err != nil {
					log.Printf("failed to generate moves %v", err)
				}
				if len(validMoves) == 0 {
					log.Print("No valid moves! Game must be over...")
					time.Sleep(60 * time.Second)
				}
				// Generate move
				log.Println("Generating move")
				turn, err := getTurnFromFEN(match.FEN)
				if err != nil {
					log.Printf("failed getting turnfromfen")
				}
				var model openai.ChatModel
				var prompt string
				if turn == "white" {
					model = openai.ChatModel(match.FirstBot.Model)
					prompt = match.FirstBot.Prompt
				} else {
					model = openai.ChatModel(match.SecondBot.Model)
					prompt = match.SecondBot.Prompt
				}
				move, err := generateMove(model, match.FEN, validMoves, prompt)
				if err != nil {
					log.Printf("error generating move %v", err)
				}
				// LLM responded with invalid index
				if move == "" {
					log.Print("Invalid move, retrying")
					onShutdown(*lastMatch, rdb)
					return
				}
				// Make move
				newFen, score, err := makeMove(match.FEN, move)
				if err != nil {
					log.Printf("failed to make move %v", err)
				}
				history := History{
					Move:  move,
					Score: score,
				}
				match.FEN = newFen
				match.History = append(match.History, history)
				match.RemainingTurns--
				// Determine if this was the last turn.
				// If so, write the result to the database and don't write to redis.

				// This was the last turn
				if match.RemainingTurns <= 0 {
					log.Println("game over turn limit reached")
					// TODO write to db
					err = writeResult(match, db)
					if err != nil {
						log.Printf("error writing result! %v", err)
					}
					return
				}
				// This was not the last turn, so write to redis
				err = createRedisMatch(match, rdb)
				if err != nil {
					log.Printf("failed to update redis match %v", err)
				}
				log.Printf("redis updated. turns remaining: %v", match.RemainingTurns)
			}(result[1])
		}
	}
}

func onShutdown(match Match, rdb *redis.Client) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println("\nReceived signal:", sig)
		log.Println("Shutting down gracefully...")
		createRedisMatch(match, rdb)
		log.Printf("Reset in flight match: %v", match)
		log.Println("Exiting")
		os.Exit(0)
	}()
}

func generateMove(model openai.ChatModel, fen string, moves []string, prompt string) (string, error) {
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

func GenerateSchema[T any]() interface{} {
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

func getTurnFromFEN(fen string) (string, error) {
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

func generateValidMoves(fen string) ([]string, error) {
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
	res, err := sendStockfishRequest("/legal_moves", jsonBody, "POST")
	if err != nil {
		log.Printf("failed to send stockfish request %v", err)
	}
	err = json.Unmarshal([]byte(res), &validMoves)
	if err != nil {
		return emptyReturn, err
	}
	return validMoves.Legal_Moves, nil
}

func makeMove(fen string, move string) (string, int, error) {
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
	newFen, err := sendStockfishRequest("/move", jsonBody, "POST")
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
	score, err := sendStockfishRequest("/evaluatemove", jsonBody, "POST")
	if err != nil {
		log.Println("score not set for some reason")
		return newFenParsed.FEN, -1, nil
	}
	var scoreParsed Score
	err = json.Unmarshal([]byte(score), &scoreParsed)
	if err != nil {
		log.Println("failed to convert score to int")
		return newFenParsed.FEN, -1, nil
	}
	return newFenParsed.FEN, scoreParsed.ScoreDifference, nil
}

func sendStockfishRequest(stem string, payload []byte, method string) (string, error) {
	url := os.Getenv("STOCKFISH_URL") + stem
	if url == "" {
		url = "http://host.docker.internal:5001" + stem
	}
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

func createRedisMatch(match Match, rdb *redis.Client) error {
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

func writeResult(match Match, db *gorm.DB) error {
	botmatchresultRepository := botmatchresult.NewBotMatchResultRepository(db)
	historyBytes, err := json.Marshal(match.History)
	if err != nil {
		log.Print("error marshaling history")
		return err
	}
	score := determineScore(match.History)
	jsonData := botmatchresult.BotMatchResult{
		BotOneID: int(match.FirstBot.ID),
		BotTwoID: int(match.SecondBot.ID),
		History:  historyBytes,
		Score:    score,
		PuzzleID: int(match.PuzzleID),
	}
	botmatchresultRepository.CreateResult(jsonData)
	return nil
}

func determineScore(history []History) int {
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
