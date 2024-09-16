package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"golang.org/x/time/rate"
)

type Statistics struct {
	ID                         int              `json:"id" db:"id"`
	Timestamp                  int64            `json:"timestamp" db:"timestamp"`
	TotalPosts                 int              `json:"total_posts" db:"total_posts"`
	UniquePubkeys              int              `json:"unique_pubkeys" db:"unique_pubkeys"`
	SuccessfulRequests         int              `json:"successful_requests" db:"successful_requests"`
	FailedRequests             int              `json:"failed_requests" db:"failed_requests"`
	TotalRequests              int              `json:"total_requests" db:"total_requests"`
	AveragePostsPerPubkey      float64          `json:"average_posts_per_pubkey" db:"average_posts_per_pubkey"`
	MostRecentPostTimestamp    int64            `json:"most_recent_post_timestamp" db:"most_recent_post_timestamp"`
	OldestPostTimestamp        int64            `json:"oldest_post_timestamp" db:"oldest_post_timestamp"`
	RateLimitRequestsPerSecond int              `json:"rate_limit_requests_per_second" db:"rate_limit_requests_per_second"`
	BodyMaxSize                int              `json:"body_max_size"`
	LinkMaxSize                int              `json:"link_max_size"`
	PubkeyMaxSize              int              `json:"pubkey_max_size"`
	SignatureMaxSize           int              `json:"signature_max_size"`
	TopProlificPubkeys         []ProlificPubkey `json:"top_prolific_pubkeys"`
}

type ProlificPubkey struct {
	Pubkey string `json:"pubkey"`
	Count  int    `json:"count"`
}

func getStatistics(successfulRequests, failedRequests int, limiter *rate.Limiter) (Statistics, error) {
	allUpdates, err := getAllStatusUpdatesFromDB()
	if err != nil {
		return Statistics{}, err
	}

	pubkeyPostCounts := make(map[string]int)
	for _, update := range allUpdates {
		pubkeyPostCounts[string(update.Pubkey)]++
	}

	uniquePubkeys := len(pubkeyPostCounts)
	topProlificPubkeys := getTopProlificPubkeys(pubkeyPostCounts)
	totalRequests := successfulRequests + failedRequests
	averagePostsPerPubkey := float64(len(allUpdates)) / float64(uniquePubkeys)
	var mostRecentPostTimestamp, oldestPostTimestamp int64
	if len(allUpdates) > 0 {
		mostRecentPostTimestamp = allUpdates[0].Timestamp
		oldestPostTimestamp = allUpdates[len(allUpdates)-1].Timestamp
	}

	stats := Statistics{
		TotalPosts:                 len(allUpdates),
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

	return stats, nil
}

func printLiveStats(ctx context.Context) {
	ticker := time.NewTicker(StatsRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := getStatistics(successfulRequests, failedRequests, limiter)
			if err != nil {
				fmt.Printf("Error getting statistics: %v\n", err)
				continue
			}

			// Update statistics in the database
			err = updateStatisticsInDB(stats)
			if err != nil {
				fmt.Printf("Error updating statistics in database: %v\n", err)
			}

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

func getTopProlificPubkeys(pubkeyPostCounts map[string]int) []ProlificPubkey {
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
