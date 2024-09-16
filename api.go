package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
)

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
