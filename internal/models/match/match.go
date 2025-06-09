package match

import "aichess-matchrunner/internal/models/bot"

type Match struct {
	FirstBot       bot.Bot   `json:"firstBot"`
	SecondBot      bot.Bot   `json:"secondBot"`
	History        []History `json:"history"`
	FEN            string    `json:"fen"`
	RemainingTurns int       `json:"remainingTurns"`
}

type History struct {
	Move  string `json:"move"`
	Score int    `json:"score"`
}
