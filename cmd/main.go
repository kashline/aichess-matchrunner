package main

import (
	"aichess-matchrunner/internal/util"
	"context"
	"sync"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go util.StartWorker(ctx, cancel, &wg)
	wg.Wait()
}
