package main

import (
	"aichess-matchrunner/internal/util"
	"context"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go util.StartWorker(ctx, cancel)
}
