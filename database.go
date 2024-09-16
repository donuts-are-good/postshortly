package main

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const (
	dbFile = "postshortly.sqlite.db"
	schema = `
	-- Status Updates table
	CREATE TABLE IF NOT EXISTS status_updates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		body TEXT NOT NULL,
		link TEXT,
		pubkey TEXT NOT NULL CHECK(length(pubkey) = 64),
		signature TEXT NOT NULL CHECK(length(signature) = 128)
	);

	-- Statistics table
	CREATE TABLE IF NOT EXISTS statistics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		total_posts INTEGER NOT NULL,
		unique_pubkeys INTEGER NOT NULL,
		successful_requests INTEGER NOT NULL,
		failed_requests INTEGER NOT NULL,
		total_requests INTEGER NOT NULL,
		average_posts_per_pubkey REAL NOT NULL,
		most_recent_post_timestamp INTEGER NOT NULL,
		oldest_post_timestamp INTEGER NOT NULL,
		rate_limit_requests_per_second INTEGER NOT NULL
	);

	-- Index for faster queries on pubkey
	CREATE INDEX IF NOT EXISTS idx_status_updates_pubkey ON status_updates(pubkey);

	-- Index for faster timestamp-based queries
	CREATE INDEX IF NOT EXISTS idx_status_updates_timestamp ON status_updates(timestamp);
	`
)

var db *sqlx.DB

func initDB() error {
	var err error
	db, err = sqlx.Connect("sqlite3", dbFile)
	if err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}

	// Execute the schema
	_, err = db.Exec(schema)
	if err != nil {
		return fmt.Errorf("error creating schema: %v", err)
	}

	return nil
}

func addStatusUpdate(update *StatusUpdate) error {
	result, err := db.Exec(`
		INSERT INTO status_updates (timestamp, body, link, pubkey, signature)
		VALUES (?, ?, ?, ?, ?)
	`, update.Timestamp, update.Body, update.Link, update.Pubkey, update.Signature)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	update.ID = int(id)
	return nil
}

func getStatusUpdatesByPubkeyFromDB(pubkey string) ([]StatusUpdate, error) {
	var updates []StatusUpdate
	err := db.Select(&updates, "SELECT * FROM status_updates WHERE pubkey = ? ORDER BY timestamp DESC", pubkey)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func getAllStatusUpdatesFromDB() ([]StatusUpdate, error) {
	var updates []StatusUpdate
	err := db.Select(&updates, "SELECT * FROM status_updates ORDER BY timestamp DESC")
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func updateStatisticsInDB(stats Statistics) error {
	_, err := db.Exec(`
		INSERT INTO statistics (
			timestamp, total_posts, unique_pubkeys, successful_requests, 
			failed_requests, total_requests, average_posts_per_pubkey, 
			most_recent_post_timestamp, oldest_post_timestamp, 
			rate_limit_requests_per_second
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, time.Now().Unix(), stats.TotalPosts, stats.UniquePubkeys, stats.SuccessfulRequests,
		stats.FailedRequests, stats.TotalRequests, stats.AveragePostsPerPubkey,
		stats.MostRecentPostTimestamp, stats.OldestPostTimestamp,
		stats.RateLimitRequestsPerSecond)
	return err
}

func getLatestStatisticsFromDB() (Statistics, error) {
	var stats Statistics
	err := db.Get(&stats, "SELECT * FROM statistics ORDER BY timestamp DESC LIMIT 1")
	return stats, err
}
