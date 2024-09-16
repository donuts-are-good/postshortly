package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

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

func getStatistics(mu *sync.Mutex, statusUpdates []StatusUpdate, pubkeyPostCounts map[string]int, successfulRequests, failedRequests int, limiter *rate.Limiter) Statistics {
	mu.Lock()
	defer mu.Unlock()

	uniquePubkeys := len(pubkeyPostCounts)
	topProlificPubkeys := getTopProlificPubkeys(pubkeyPostCounts)
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

	return stats
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

func getStatsForPrinting(mu *sync.Mutex, statusUpdates []StatusUpdate, pubkeyPostCounts map[string]int, successfulRequests, failedRequests int, limiter *rate.Limiter) Statistics {
	mu.Lock()
	defer mu.Unlock()

	uniquePubkeys := len(pubkeyPostCounts)
	topProlificPubkeys := getTopProlificPubkeys(pubkeyPostCounts)
	totalRequests := successfulRequests + failedRequests
	averagePostsPerPubkey := float64(len(statusUpdates)) / float64(uniquePubkeys)
	var mostRecentPostTimestamp, oldestPostTimestamp int64
	if len(statusUpdates) > 0 {
		mostRecentPostTimestamp = statusUpdates[len(statusUpdates)-1].Timestamp
		oldestPostTimestamp = statusUpdates[0].Timestamp
	}

	return Statistics{
		TotalPosts:                 len(statusUpdates),
		UniquePubkeys:              uniquePubkeys,
		SuccessfulRequests:         successfulRequests,
		FailedRequests:             failedRequests,
		TotalRequests:              totalRequests,
		TopProlificPubkeys:         topProlificPubkeys,
		AveragePostsPerPubkey:      averagePostsPerPubkey,
		MostRecentPostTimestamp:    mostRecentPostTimestamp,
		OldestPostTimestamp:        oldestPostTimestamp,
		RateLimitRequestsPerSecond: int(limiter.Limit()),
	}
}
