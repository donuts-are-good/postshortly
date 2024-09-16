package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/time/rate"
)

const (
	BodyMaxSize          = 256
	LinkMaxSize          = 256
	PubkeyMaxSize        = ed25519.PublicKeySize
	SignatureMaxSize     = ed25519.SignatureSize
	StatsRefreshInterval = 1 * time.Second
	Port                 = 3495
)

type StatusUpdate struct {
	ID        int    `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Body      string `json:"body"`
	Link      string `json:"link,omitempty"`
	Pubkey    []byte `json:"pubkey"`
	Signature []byte `json:"signature"`
}

var (
	statusUpdates      []StatusUpdate
	idCounter          int
	mu                 sync.Mutex
	limiter            = rate.NewLimiter(1, 1)
	successfulRequests int
	failedRequests     int
	pubkeyPostCounts   = make(map[string]int)
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go printLiveStats(ctx)
	r := setupRouter()
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	fmt.Printf("Started on port: %d\n", Port)
	http.ListenAndServe(fmt.Sprintf(":%d", Port), handlers.CORS()(loggedRouter))
}

func setupRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/status", createStatusUpdate).Methods("POST")
	r.HandleFunc("/status/{pubkey}", getStatusUpdatesByPubkey).Methods("GET")
	r.HandleFunc("/status", getAllStatusUpdates).Methods("GET")
	r.HandleFunc("/stats", getStatisticsHandler).Methods("GET")
	return r
}

func createStatusUpdate(w http.ResponseWriter, r *http.Request) {
	if !limiter.Allow() {
		handleError(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	var update StatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		handleError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := validateStatusUpdate(update); err != nil {
		handleError(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	idCounter++
	update.ID = idCounter
	update.Timestamp = time.Now().UnixNano()
	statusUpdates = append(statusUpdates, update)
	successfulRequests++

	pubkeyStr := string(update.Pubkey)
	pubkeyPostCounts[pubkeyStr]++

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(update)
}

func getStatusUpdatesByPubkey(w http.ResponseWriter, r *http.Request) {
	pubkeyStr := mux.Vars(r)["pubkey"]
	pubkey, err := hex.DecodeString(pubkeyStr)
	if err != nil {
		handleError(w, "Invalid public key", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var updates []StatusUpdate
	for _, update := range statusUpdates {
		if bytes.Equal(update.Pubkey, pubkey) {
			updates = append(updates, update)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updates)
}

func getAllStatusUpdates(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(statusUpdates)
}

func getStatisticsHandler(w http.ResponseWriter, r *http.Request) {
	stats := getStatistics(&mu, statusUpdates, pubkeyPostCounts, successfulRequests, failedRequests, limiter)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

func validateStatusUpdate(update StatusUpdate) error {
	p := bluemonday.UGCPolicy()
	update.Body = p.Sanitize(update.Body)
	update.Link = p.Sanitize(update.Link)

	if len(update.Body) > BodyMaxSize || (update.Link != "" && len(update.Link) > LinkMaxSize) || len(update.Pubkey) != PubkeyMaxSize || len(update.Signature) != SignatureMaxSize {
		return fmt.Errorf("invalid field sizes")
	}

	dataToVerify := append(update.Pubkey, []byte(update.Body)...)
	dataToVerify = append(dataToVerify, []byte(update.Link)...)

	if !ed25519.Verify(update.Pubkey, dataToVerify, update.Signature) {
		return fmt.Errorf("Unauthorized")
	}

	return nil
}

func handleError(w http.ResponseWriter, message string, statusCode int) {
	failedRequests++
	http.Error(w, message, statusCode)
	log.Printf("Error: %s, StatusCode: %d", message, statusCode)
}

func printLiveStats(ctx context.Context) {
	ticker := time.NewTicker(StatsRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := getStatsForPrinting(&mu, statusUpdates, pubkeyPostCounts, successfulRequests, failedRequests, limiter)

			// Clear the screen and move cursor to top-left
			fmt.Print("\033[2J\033[H")

			// Print underlined "Live Statistics:"
			fmt.Println("\033[4mLive Statistics:\033[0m")
			fmt.Printf("-> Total Posts:           %d\n", stats.TotalPosts)
			fmt.Printf("-> Unique Pubkeys:        %d\n", stats.UniquePubkeys)
			fmt.Printf("-> Successful Requests:   %d\n", stats.SuccessfulRequests)
			fmt.Printf("-> Failed Requests:       %d\n", stats.FailedRequests)
			fmt.Printf("-> Total Requests:        %d\n", stats.TotalRequests)
			fmt.Printf("-> Avg. Per Pubkey:       %.2f\n", stats.AveragePostsPerPubkey)
			fmt.Printf("-> Most Recent Post Time: %s\n", time.Unix(0, stats.MostRecentPostTimestamp).Format("2006-01-02 03:04:05 PM"))
			fmt.Printf("-> Oldest Post Time:      %s\n", time.Unix(0, stats.OldestPostTimestamp).Format("2006-01-02 03:04:05 PM"))
			fmt.Printf("-> Limit (reqs/second):   %d\n", stats.RateLimitRequestsPerSecond)

			fmt.Println("\nTop Prolific Pubkeys:")
			for i, pubkey := range stats.TopProlificPubkeys {
				fmt.Printf("%d. %s: %d posts\n", i+1, pubkey.Pubkey, pubkey.Count)
			}
		}
	}
}
