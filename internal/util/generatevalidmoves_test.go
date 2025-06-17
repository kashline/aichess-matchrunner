package util

import (
	"encoding/json"
	"errors"
	"testing"
)

// 👇 Injected mock of SendStockfishRequest for testing
var mockSendStockfishRequest func(path string, jsonBody []byte, method string) (string, error)

func init() {
	// Replace the real SendStockfishRequest with the mock
	SendStockfishRequest = func(path string, jsonBody []byte, method string) (string, error) {
		return mockSendStockfishRequest(path, jsonBody, method)
	}
}

func TestGenerateValidMoves(t *testing.T) {
	// Example valid FEN: starting position
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

	expectedMoves := []string{"a2a3", "a2a4", "b2b3", "b2b4"}

	// Set up the mock response
	mockSendStockfishRequest = func(path string, jsonBody []byte, method string) (string, error) {
		data := map[string][]string{"legal_moves": expectedMoves}
		out, _ := json.Marshal(data)
		return string(out), nil
	}

	moves, err := GenerateValidMoves(fen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(moves) != len(expectedMoves) {
		t.Errorf("expected %d moves, got %d", len(expectedMoves), len(moves))
	}

	for i, move := range expectedMoves {
		if moves[i] != move {
			t.Errorf("expected move %s, got %s", move, moves[i])
		}
	}
}

func TestGenerateValidMoves_ErrorCase(t *testing.T) {
	badFEN := "not a real fen"

	mockSendStockfishRequest = func(path string, jsonBody []byte, method string) (string, error) {
		return "", errors.New("invalid FEN")
	}

	moves, err := GenerateValidMoves(badFEN)
	if err == nil {
		t.Error("expected error for invalid FEN, got nil")
	}
	if len(moves) != 1 || moves[0] != "" {
		t.Errorf("expected fallback return [\"\"], got %v", moves)
	}
}
