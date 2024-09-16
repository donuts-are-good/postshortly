package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"golang.org/x/time/rate"
)

const (
	BodyMaxSize          = 256
	LinkMaxSize          = 256
	PubkeyMaxSize        = ed25519.PublicKeySize
	SignatureMaxSize     = ed25519.SignatureSize
	StatsRefreshInterval = 500 * time.Millisecond
	Port                 = 3495
)

type StatusUpdate struct {
	ID        int    `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Body      string `json:"body"`
	Link      string `json:"link,omitempty"`
	Pubkey    string `json:"pubkey"`
	Signature string `json:"signature"`
}

var (
	limiter            = rate.NewLimiter(1, 1)
	successfulRequests int
	failedRequests     int
)

func main() {
	if err := initDB(); err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go printLiveStats(ctx)
	r := setupRouter()
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	fmt.Printf("Started on port: %d\n", Port)
	http.ListenAndServe(fmt.Sprintf(":%d", Port), handlers.CORS()(loggedRouter))
}
