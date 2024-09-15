package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/time/rate"
)

const (
	BodyMaxSize      = 256
	LinkMaxSize      = 256
	PubkeyMaxSize    = ed25519.PublicKeySize
	SignatureMaxSize = ed25519.SignatureSize
)

type StatusUpdate struct {
	ID        int    `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Body      string `json:"body"`
	Link      string `json:"link,omitempty"`
	Pubkey    []byte `json:"pubkey"`
	Signature []byte `json:"signature"`
}

type Statistics struct {
	TotalPosts                 int              `json:"total_posts"`
	UniquePubkeys              int              `json:"unique_pubkeys"`
	SuccessfulRequests         int              `json:"successful_requests"`
	FailedRequests             int              `json:"failed_requests"`
	TotalRequests              int              `json:"total_requests"`
	BodyMaxSize                int              `json:"body_max_size"`
	LinkMaxSize                int              `json:"link_max_size"`
	PubkeyMaxSize              int              `json:"pubkey_max_size"`
	SignatureMaxSize           int              `json:"signature_max_size"`
	TopProlificPubkeys         []ProlificPubkey `json:"top_prolific_pubkeys"`
	AveragePostsPerPubkey      float64          `json:"average_posts_per_pubkey"`
	MostRecentPostTimestamp    int64            `json:"most_recent_post_timestamp"`
	OldestPostTimestamp        int64            `json:"oldest_post_timestamp"`
	RateLimitRequestsPerSecond int              `json:"rate_limit_requests_per_second"`
}

type ProlificPubkey struct {
	Pubkey string `json:"pubkey"`
	Count  int    `json:"count"`
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
	r := mux.NewRouter()
	r.HandleFunc("/status", createStatusUpdate).Methods("POST")
	r.HandleFunc("/status/{pubkey}", getStatusUpdatesByPubkey).Methods("GET")
	r.HandleFunc("/status", getAllStatusUpdates).Methods("GET")
	r.HandleFunc("/stats", getStatistics).Methods("GET")

	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	http.ListenAndServe(":3495", handlers.CORS()(loggedRouter))
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
	pubkey := mux.Vars(r)["pubkey"]

	mu.Lock()
	defer mu.Unlock()

	var updates []StatusUpdate
	for _, update := range statusUpdates {
		if string(update.Pubkey) == pubkey {
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

func getStatistics(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	uniquePubkeys := len(pubkeyPostCounts)
	topProlificPubkeys := getTopProlificPubkeys()
	totalRequests := successfulRequests + failedRequests
	averagePostsPerPubkey := float64(len(statusUpdates)) / float64(uniquePubkeys)
	var mostRecentPostTimestamp, oldestPostTimestamp int64
	if len(statusUpdates) > 0 {
		mostRecentPostTimestamp = statusUpdates[len(statusUpdates)-1].Timestamp
		oldestPostTimestamp = statusUpdates[0].Timestamp
	}

	stats := Statistics{
		TotalPosts:                 len(statusUpdates),
		UniquePubkeys:              uniquePubkeys,
		SuccessfulRequests:         successfulRequests,
		FailedRequests:             failedRequests,
		TotalRequests:              totalRequests,
		BodyMaxSize:                BodyMaxSize,
		LinkMaxSize:                LinkMaxSize,
		PubkeyMaxSize:              PubkeyMaxSize,
		SignatureMaxSize:           SignatureMaxSize,
		TopProlificPubkeys:         topProlificPubkeys,
		AveragePostsPerPubkey:      averagePostsPerPubkey,
		MostRecentPostTimestamp:    mostRecentPostTimestamp,
		OldestPostTimestamp:        oldestPostTimestamp,
		RateLimitRequestsPerSecond: int(limiter.Limit()),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

func getTopProlificPubkeys() []ProlificPubkey {
	type kv struct {
		Key   string
		Value int
	}

	var ss []kv
	for k, v := range pubkeyPostCounts {
		ss = append(ss, kv{k, v})
	}

	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})

	var topProlificPubkeys []ProlificPubkey
	for i, kv := range ss {
		if i >= 10 {
			break
		}
		topProlificPubkeys = append(topProlificPubkeys, ProlificPubkey{Pubkey: kv.Key, Count: kv.Value})
	}

	return topProlificPubkeys
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
