package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

type BatchRequest struct {
	CustomID string      `json:"custom_id"`
	Method   string      `json:"method"` // usually "POST"
	URL      string      `json:"url"`    // e.g. "/v1/chat/completions"`
	Body     RequestBody `json:"body"`
}

type RequestBody struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	MaxTokens      int           `json:"max_tokens"`
	ResponseFormat string        `json:"response_format,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", etc.
	Content string `json:"content"` // message content
}

type FileResponse struct {
	ID string `json:"id"`
}

func GenerateBatchMove(model openai.ChatModel, fen string, moves []string, prompt string, fileLength *int) (string, error) {
	maxRequests := 500
	// Write to file
	err := WriteToJsonl(model, fen, moves, prompt)
	if err != nil {
		log.Printf("error writing to file: %v", err)
	}
	// Length limit reached, send file
	if *fileLength >= maxRequests {
		res, err := SendBatchRequest()
		if err != nil {
			log.Printf("failed to send batch request: %v", err)
		}
		if res {
			log.Print("batch request sent")
			// reset fileLength
			*fileLength = 0
		}
	}

	return "", nil
}

func WriteToJsonl(model openai.ChatModel, fen string, moves []string, prompt string) error {
	customID := uuid.New().String()
	// Generate jsonl file
	req := BatchRequest{
		CustomID: customID,
		Method:   "POST",
		URL:      "/v1/chat/completions",
		Body: RequestBody{
			Model: model,
			Messages: []ChatMessage{
				{Role: "system", Content: fmt.Sprintf("This fen represents a game of chess that we are currently playing: %v.  Here is an array of possible moves: [%v].  Select a move from this list and respond with the ZERO-BASED index the move has in the array.  Do not choose a number larger than %v.  Here are additional details: %v", fen, moves, len(moves)-1, prompt)},
			},
			MaxTokens: 1000,
		},
	}
	file, err := os.OpenFile("data.jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	jsonBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("error marshalling match: %v", err)
	}
	_, err = file.WriteString(string(jsonBytes) + "\n")
	if err != nil {
		log.Printf("error writing match file: %v", err)
	}
	return nil
}

func SendBatchRequest() (bool, error) {
	file, err := UploadFile()
	if err != nil {
		log.Printf("error uploading file: %v", err)
	}
	err = CreateBatch(file.ID)
	if err != nil {
		log.Printf("error creating batch: %v", err)
	}
	return true, nil
}

func UploadFile() (FileResponse, error) {
	// Set form fields
	file, err := os.Open("data.jsonl")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	// Add file field
	fw, err := w.CreateFormFile("file", "batch_input.jsonl")
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(fw, file)
	if err != nil {
		panic(err)
	}
	// Add purpose field
	err = w.WriteField("purpose", "batch")
	if err != nil {
		panic(err)
	}
	w.Close()

	// Send file
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/files", &b)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", os.Getenv("OPENAI_API_KEY")))
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res FileResponse
	err = json.Unmarshal([]byte(string(body)), &res)
	if err != nil {
		return res, err
	}
	return res, nil
}

func CreateBatch(fileID string) error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY not set")
	}

	batchRequest := map[string]interface{}{
		"input_file_id":     fileID, // <- your real file ID
		"endpoint":          "/v1/chat/completions",
		"completion_window": "24h",
		"metadata": map[string]string{
			"description": "aichess-matches",
		},
	}

	payload, err := json.Marshal(batchRequest)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/batches", bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		fmt.Println("Batch creation failed with status:", resp.Status)
		os.Exit(1)
	}

	var responseBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		panic(err)
	}

	pretty, _ := json.MarshalIndent(responseBody, "", "  ")
	fmt.Println("Batch created:\n", string(pretty))
	return nil
}
