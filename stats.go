package main

import (
	"sort"
	"sync"

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
