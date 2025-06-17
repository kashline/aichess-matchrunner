package util

import (
	"encoding/json"
	"errors"
	"testing"
)

// backup original SendStockfishRequest
var originalSendStockfishRequest = SendStockfishRequest

func teardown() {
	SendStockfishRequest = originalSendStockfishRequest
}

func TestMakeMove_Success(t *testing.T) {
	defer teardown()

	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	move := "e2e4"
	expectedNewFEN := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	expectedScore := 32

	callCount := 0
	SendStockfishRequest = func(path string, jsonBody []byte, method string) (string, error) {
		callCount++
		if path == "/move" {
			resp := map[string]string{"fen": expectedNewFEN}
			out, _ := json.Marshal(resp)
			return string(out), nil
		} else if path == "/evaluatemove" {
			score := map[string]int{"score_difference": expectedScore}
			out, _ := json.Marshal(score)
			return string(out), nil
		}
		return "", errors.New("unknown path")
	}

	newFEN, score, err := MakeMove(fen, move)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newFEN != expectedNewFEN {
		t.Errorf("expected new FEN %s, got %s", expectedNewFEN, newFEN)
	}
	if score != expectedScore {
		t.Errorf("expected score %d, got %d", expectedScore, score)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls to SendStockfishRequest, got %d", callCount)
	}
}

func TestMakeMove_MoveAPIFailure(t *testing.T) {
	defer teardown()

	SendStockfishRequest = func(path string, jsonBody []byte, method string) (string, error) {
		if path == "/move" {
			return "", errors.New("move failed")
		}
		return "", nil
	}

	fen, score, err := MakeMove("invalid fen", "e2e4")
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if fen != "" || score != -1 {
		t.Errorf("expected default return, got fen: %s, score: %d", fen, score)
	}
}

func TestMakeMove_EvaluationFails(t *testing.T) {
	defer teardown()

	SendStockfishRequest = func(path string, jsonBody []byte, method string) (string, error) {
		if path == "/move" {
			resp := map[string]string{"fen": "updated FEN"}
			out, _ := json.Marshal(resp)
			return string(out), nil
		}
		if path == "/evaluatemove" {
			return "", errors.New("evaluation failed")
		}
		return "", nil
	}

	fen, score, err := MakeMove("some fen", "e2e4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fen != "updated FEN" {
		t.Errorf("expected updated FEN, got %s", fen)
	}
	if score != -1 {
		t.Errorf("expected fallback score -1, got %d", score)
	}
}
