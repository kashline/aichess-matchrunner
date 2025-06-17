package util

import (
	"testing"
)

func TestGetTurnFromFEN(t *testing.T) {
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	result, err := GetTurnFromFEN(fen)
	if err != nil {
		t.Errorf("GetTurnFromFEN(%v) error: %v", fen, err)
	}

	expected := "white"

	if result != expected {
		t.Errorf("GetTurnFromFEN() = %+v; want %+v", result, expected)
	}
}
