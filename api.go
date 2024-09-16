package main

import (
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

	update.Timestamp = time.Now().UnixNano()
	if err := addStatusUpdate(&update); err != nil {
		handleError(w, "Error adding status update", http.StatusInternalServerError)
		return
	}

	successfulRequests++

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(update)
}

func getStatusUpdatesByPubkey(w http.ResponseWriter, r *http.Request) {
	pubkeyStr := mux.Vars(r)["pubkey"]
	if len(pubkeyStr) != PubkeyMaxSize*2 {
		handleError(w, "Invalid public key", http.StatusBadRequest)
		return
	}

	updates, err := getStatusUpdatesByPubkeyFromDB(pubkeyStr)
	if err != nil {
		handleError(w, "Error retrieving status updates", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updates)
}

func getAllStatusUpdates(w http.ResponseWriter, r *http.Request) {
	updates, err := getAllStatusUpdatesFromDB()
	if err != nil {
		handleError(w, "Error retrieving status updates", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updates)
}

func getStatisticsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := getLatestStatisticsFromDB()
	if err != nil {
		handleError(w, "Error retrieving statistics", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

func validateStatusUpdate(update StatusUpdate) error {
	p := bluemonday.UGCPolicy()
	update.Body = p.Sanitize(update.Body)
	update.Link = p.Sanitize(update.Link)

	if update.Body == "" {
		return fmt.Errorf("body cannot be empty")
	}

	if len(update.Body) > BodyMaxSize {
		return fmt.Errorf("body exceeds maximum size of %d characters", BodyMaxSize)
	}

	if update.Link != "" && len(update.Link) > LinkMaxSize {
		return fmt.Errorf("link exceeds maximum size of %d characters", LinkMaxSize)
	}

	if len(update.Pubkey) != PubkeyMaxSize*2 {
		return fmt.Errorf("invalid pubkey length")
	}

	if len(update.Signature) != SignatureMaxSize*2 {
		return fmt.Errorf("invalid signature length")
	}

	pubkey, err := hex.DecodeString(update.Pubkey)
	if err != nil {
		return fmt.Errorf("invalid pubkey format")
	}

	signature, err := hex.DecodeString(update.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature format")
	}

	dataToVerify := append(pubkey, []byte(update.Body)...)
	dataToVerify = append(dataToVerify, []byte(update.Link)...)

	if !ed25519.Verify(pubkey, dataToVerify, signature) {
		return fmt.Errorf("unauthorized: signature verification failed")
	}

	return nil
}

func handleError(w http.ResponseWriter, message string, statusCode int) {
	failedRequests++
	http.Error(w, message, statusCode)
	log.Printf("Error: %s, StatusCode: %d", message, statusCode)
}
