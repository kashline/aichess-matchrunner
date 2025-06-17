package main

import (
	"aichess-matchrunner/internal/util"
	"context"
	"fmt"
	"log"
	"net/http"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go util.StartWorker(ctx, cancel)
	// Set up the HTTP health endpoint
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if util.Healthz() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "healthy")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "unhealthy")
		}
	})
	port := "8080"
	log.Printf("Starting HTTP server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
