package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func setup() {
	// Reset the global state before each test
	statusUpdates = []StatusUpdate{}
	idCounter = 0
	successfulRequests = 0
	failedRequests = 0
	pubkeyPostCounts = make(map[string]int)
	limiter = rate.NewLimiter(1, 1)
}

func TestCreateStatusUpdate(t *testing.T) {
	setup()

	// Generate a key pair for testing
	pubkey, privkey, _ := ed25519.GenerateKey(nil)

	// Create a valid status update
	update := StatusUpdate{
		Body:      "Test body",
		Link:      "http://example.com",
		Pubkey:    pubkey,
		Signature: ed25519.Sign(privkey, append(append(pubkey, []byte("Test body")...), []byte("http://example.com")...)),
	}

	// Encode the update to JSON
	body, _ := json.Marshal(update)

	req, err := http.NewRequest("POST", "/status", bytes.NewBuffer(body))
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createStatusUpdate)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response StatusUpdate
	err = json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, update.Body, response.Body)
	assert.Equal(t, update.Link, response.Link)
	assert.Equal(t, update.Pubkey, response.Pubkey)
}

func TestCreateStatusUpdateRateLimit(t *testing.T) {
	setup()

	// Generate a key pair for testing
	pubkey, privkey, _ := ed25519.GenerateKey(nil)

	// Create a valid status update
	update := StatusUpdate{
		Body:      "Test body",
		Link:      "http://example.com",
		Pubkey:    pubkey,
		Signature: ed25519.Sign(privkey, append(append(pubkey, []byte("Test body")...), []byte("http://example.com")...)),
	}

	// Encode the update to JSON
	body, _ := json.Marshal(update)

	req, err := http.NewRequest("POST", "/status", bytes.NewBuffer(body))
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createStatusUpdate)

	// First request should pass
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Second request should be rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestCreateStatusUpdateInvalidPayload(t *testing.T) {
	setup()

	req, err := http.NewRequest("POST", "/status", bytes.NewBuffer([]byte("invalid payload")))
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createStatusUpdate)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetStatusUpdatesByPubkeyInvalidKey(t *testing.T) {
	setup()

	req, err := http.NewRequest("GET", "/status/invalid_pubkey", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	router := setupRouter()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid public key")
}

func TestGetStatusUpdatesByPubkey(t *testing.T) {
	setup()

	// Generate a key pair for testing
	pubkey, _, _ := ed25519.GenerateKey(nil)
	pubkeyStr := hex.EncodeToString(pubkey)

	// Add a status update to the global slice
	mu.Lock()
	statusUpdates = append(statusUpdates, StatusUpdate{
		ID:        1,
		Timestamp: time.Now().UnixNano(),
		Body:      "Test body",
		Pubkey:    pubkey,
	})
	mu.Unlock()

	req, err := http.NewRequest("GET", "/status/"+pubkeyStr, nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	router := setupRouter()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var updates []StatusUpdate
	err = json.NewDecoder(rr.Body).Decode(&updates)
	assert.NoError(t, err)
	assert.Len(t, updates, 1)
	assert.Equal(t, "Test body", updates[0].Body)
}

func TestGetAllStatusUpdates(t *testing.T) {
	setup()

	// Ensure the statusUpdates slice is populated
	mu.Lock()
	statusUpdates = append(statusUpdates, StatusUpdate{
		ID:        1,
		Timestamp: time.Now().UnixNano(),
		Body:      "Test body",
		Pubkey:    []byte("test_pubkey"),
	})
	mu.Unlock()

	req, err := http.NewRequest("GET", "/status", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getAllStatusUpdates)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var updates []StatusUpdate
	err = json.NewDecoder(rr.Body).Decode(&updates)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(updates), 1)
}

func TestGetStatistics(t *testing.T) {
	setup()

	// Ensure the statusUpdates slice is populated
	mu.Lock()
	statusUpdates = append(statusUpdates, StatusUpdate{
		ID:        1,
		Timestamp: time.Now().UnixNano(),
		Body:      "Test body",
		Pubkey:    []byte("test_pubkey"),
	})
	successfulRequests = 1
	pubkeyPostCounts["test_pubkey"] = 1
	mu.Unlock()

	req, err := http.NewRequest("GET", "/stats", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getStatistics)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var stats Statistics
	err = json.NewDecoder(rr.Body).Decode(&stats)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalPosts, 1)
	assert.GreaterOrEqual(t, stats.UniquePubkeys, 1)
}

func TestValidateStatusUpdate(t *testing.T) {
	setup()

	// Generate a key pair for testing
	pubkey, privkey, _ := ed25519.GenerateKey(nil)

	// Create a valid status update
	update := StatusUpdate{
		Body:      "Test body",
		Link:      "http://example.com",
		Pubkey:    pubkey,
		Signature: ed25519.Sign(privkey, append(append(pubkey, []byte("Test body")...), []byte("http://example.com")...)),
	}

	err := validateStatusUpdate(update)
	assert.NoError(t, err)

	// Create an invalid status update with a wrong signature
	update.Signature = []byte("invalid_signature")
	err = validateStatusUpdate(update)
	assert.Error(t, err)
}

func TestValidateStatusUpdateEdgeCases(t *testing.T) {
	setup()

	// Generate a key pair for testing
	pubkey, privkey, _ := ed25519.GenerateKey(nil)

	// Test maximum body size
	maxBody := make([]byte, BodyMaxSize)
	for i := range maxBody {
		maxBody[i] = 'a'
	}
	update := StatusUpdate{
		Body:      string(maxBody),
		Pubkey:    pubkey,
		Signature: ed25519.Sign(privkey, append(pubkey, maxBody...)),
	}
	err := validateStatusUpdate(update)
	assert.NoError(t, err)

	// Test maximum link size
	maxLink := make([]byte, LinkMaxSize)
	for i := range maxLink {
		maxLink[i] = 'a'
	}
	update = StatusUpdate{
		Body:      "Test body",
		Link:      string(maxLink),
		Pubkey:    pubkey,
		Signature: ed25519.Sign(privkey, append(append(pubkey, []byte("Test body")...), maxLink...)),
	}
	err = validateStatusUpdate(update)
	assert.NoError(t, err)

	// Test invalid pubkey size
	update = StatusUpdate{
		Body:      "Test body",
		Pubkey:    []byte("invalid_pubkey"),
		Signature: ed25519.Sign(privkey, append([]byte("invalid_pubkey"), []byte("Test body")...)),
	}
	err = validateStatusUpdate(update)
	assert.Error(t, err)

	// Test invalid signature size
	update = StatusUpdate{
		Body:      "Test body",
		Pubkey:    pubkey,
		Signature: []byte("invalid_signature"),
	}
	err = validateStatusUpdate(update)
	assert.Error(t, err)
}

func TestHandleError(t *testing.T) {
	setup()

	rr := httptest.NewRecorder()
	handleError(rr, "Test error", http.StatusBadRequest)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Test error")
}
