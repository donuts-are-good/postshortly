package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func setup() {
	// Reset the global state before each test
	successfulRequests = 0
	failedRequests = 0
	limiter = rate.NewLimiter(1, 1) // Reset to default rate limit

	// Initialize the test database
	if err := initDB(); err != nil {
		panic(err)
	}
}

func teardown() {
	// Close the database connection
	if db != nil {
		db.Close()
	}
	// Remove the test database file
	os.Remove(dbFile)
}

func TestCreateStatusUpdate(t *testing.T) {
	setup()
	defer teardown()

	// Generate a key pair for testing
	pubkey, privkey, _ := ed25519.GenerateKey(nil)

	// Create a valid status update
	update := StatusUpdate{
		Body:      "Test body",
		Link:      "http://example.com",
		Pubkey:    hex.EncodeToString(pubkey),
		Signature: hex.EncodeToString(ed25519.Sign(privkey, append(append(pubkey, []byte("Test body")...), []byte("http://example.com")...))),
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
	assert.NotZero(t, response.ID)
	assert.NotZero(t, response.Timestamp)
}

func TestCreateStatusUpdateRateLimit(t *testing.T) {
	setup()
	defer teardown()

	// Generate a key pair for testing
	pubkey, privkey, _ := ed25519.GenerateKey(nil)

	// Create a valid status update
	update := StatusUpdate{
		Body:      "Test body",
		Link:      "http://example.com",
		Pubkey:    hex.EncodeToString(pubkey),
		Signature: hex.EncodeToString(ed25519.Sign(privkey, append(append(pubkey, []byte("Test body")...), []byte("http://example.com")...))),
	}

	// Encode the update to JSON
	body, _ := json.Marshal(update)

	// Set a very low rate limit for testing
	limiter = rate.NewLimiter(rate.Every(1*time.Second), 1)

	// First request should pass
	req, _ := http.NewRequest("POST", "/status", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createStatusUpdate)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Second request should be rate limited
	req, _ = http.NewRequest("POST", "/status", bytes.NewBuffer(body))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestCreateStatusUpdateInvalidPayload(t *testing.T) {
	setup()
	defer teardown()

	req, err := http.NewRequest("POST", "/status", bytes.NewBuffer([]byte("invalid payload")))
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createStatusUpdate)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetStatusUpdatesByPubkeyInvalidKey(t *testing.T) {
	setup()
	defer teardown()

	req, err := http.NewRequest("GET", "/status/invalidpubkey", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	router := setupRouter()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid public key")
}

func TestGetStatusUpdatesByPubkey(t *testing.T) {
	setup()
	defer teardown()

	// Generate a key pair for testing
	pubkey, privkey, _ := ed25519.GenerateKey(nil)
	pubkeyStr := hex.EncodeToString(pubkey)

	// Add a status update to the database
	update := StatusUpdate{
		Timestamp: time.Now().UnixNano(),
		Body:      "Test body",
		Pubkey:    pubkeyStr,
		Signature: hex.EncodeToString(ed25519.Sign(privkey, append(pubkey, []byte("Test body")...))),
	}
	err := addStatusUpdate(&update)
	assert.NoError(t, err)

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
	defer teardown()

	// Add a status update to the database
	update := StatusUpdate{
		Timestamp: time.Now().UnixNano(),
		Body:      "Test body",
		Pubkey:    strings.Repeat("a", PubkeyMaxSize*2),
		Signature: strings.Repeat("b", SignatureMaxSize*2),
	}
	err := addStatusUpdate(&update)
	assert.NoError(t, err)

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
	defer teardown()

	// Add a status update to the database
	update := StatusUpdate{
		Timestamp: time.Now().UnixNano(),
		Body:      "Test body",
		Pubkey:    strings.Repeat("a", PubkeyMaxSize*2),
		Signature: strings.Repeat("b", SignatureMaxSize*2),
	}
	err := addStatusUpdate(&update)
	assert.NoError(t, err)

	// Insert initial statistics
	initialStats := Statistics{
		Timestamp:                  time.Now().Unix(),
		TotalPosts:                 1,
		UniquePubkeys:              1,
		SuccessfulRequests:         1,
		FailedRequests:             0,
		TotalRequests:              1,
		AveragePostsPerPubkey:      1.0,
		MostRecentPostTimestamp:    update.Timestamp,
		OldestPostTimestamp:        update.Timestamp,
		RateLimitRequestsPerSecond: 1,
	}
	err = updateStatisticsInDB(initialStats)
	assert.NoError(t, err)

	req, err := http.NewRequest("GET", "/stats", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getStatisticsHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var stats Statistics
	err = json.NewDecoder(rr.Body).Decode(&stats)
	assert.NoError(t, err)
	assert.Equal(t, 1, stats.TotalPosts)
	assert.Equal(t, 1, stats.UniquePubkeys)
	assert.Equal(t, 1, stats.SuccessfulRequests)
	assert.Equal(t, 0, stats.FailedRequests)
	assert.Equal(t, 1, stats.TotalRequests)
	assert.InDelta(t, 1.0, stats.AveragePostsPerPubkey, 0.001)
	assert.Equal(t, update.Timestamp, stats.MostRecentPostTimestamp)
	assert.Equal(t, update.Timestamp, stats.OldestPostTimestamp)
	assert.Equal(t, 1, stats.RateLimitRequestsPerSecond)
}

func TestPrintLiveStats(t *testing.T) {
	setup()
	defer teardown()

	// Redirect stdout to capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a context with cancel to stop printLiveStats
	ctx, cancel := context.WithCancel(context.Background())

	// Run printLiveStats in a goroutine
	done := make(chan bool)
	go func() {
		printLiveStats(ctx)
		done <- true
	}()

	// Wait for one tick of the stats refresh
	time.Sleep(StatsRefreshInterval + 100*time.Millisecond)

	// Stop printLiveStats
	cancel()
	<-done

	// Restore stdout
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	assert.NoError(t, err)

	output := buf.String()

	// Check for expected output
	assert.Contains(t, output, "Live Statistics:")
	assert.Contains(t, output, "Total Posts:")
	assert.Contains(t, output, "Unique Pubkeys:")
	assert.Contains(t, output, "Successful Requests:")
	assert.Contains(t, output, "Failed Requests:")
	assert.Contains(t, output, "Total Requests:")
	assert.Contains(t, output, "Avg. Per Pubkey:")
	assert.Contains(t, output, "Most Recent Post Time:")
	assert.Contains(t, output, "Oldest Post Time:")
	assert.Contains(t, output, "Limit (reqs/second):")
	assert.Contains(t, output, "Top Prolific Pubkeys:")
}

func TestUpdateStatisticsInDB(t *testing.T) {
	setup()
	defer teardown()

	stats := Statistics{
		Timestamp:                  time.Now().Unix(),
		TotalPosts:                 10,
		UniquePubkeys:              5,
		SuccessfulRequests:         15,
		FailedRequests:             2,
		TotalRequests:              17,
		AveragePostsPerPubkey:      2.0,
		MostRecentPostTimestamp:    time.Now().UnixNano(),
		OldestPostTimestamp:        time.Now().Add(-1 * time.Hour).UnixNano(),
		RateLimitRequestsPerSecond: 1,
	}

	err := updateStatisticsInDB(stats)
	assert.NoError(t, err)

	retrievedStats, err := getLatestStatisticsFromDB()
	assert.NoError(t, err)

	assert.Equal(t, stats.TotalPosts, retrievedStats.TotalPosts)
	assert.Equal(t, stats.UniquePubkeys, retrievedStats.UniquePubkeys)
	assert.Equal(t, stats.SuccessfulRequests, retrievedStats.SuccessfulRequests)
	assert.Equal(t, stats.FailedRequests, retrievedStats.FailedRequests)
	assert.Equal(t, stats.TotalRequests, retrievedStats.TotalRequests)
	assert.InDelta(t, stats.AveragePostsPerPubkey, retrievedStats.AveragePostsPerPubkey, 0.001)
	assert.Equal(t, stats.MostRecentPostTimestamp, retrievedStats.MostRecentPostTimestamp)
	assert.Equal(t, stats.OldestPostTimestamp, retrievedStats.OldestPostTimestamp)
	assert.Equal(t, stats.RateLimitRequestsPerSecond, retrievedStats.RateLimitRequestsPerSecond)
}
