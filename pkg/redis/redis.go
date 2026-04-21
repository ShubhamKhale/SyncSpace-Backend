// Package redis initialises and exposes the shared Redis client.
package redis

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// Client is the process-wide Redis connection pool.
var Client *redis.Client

// Connect initialises the Redis client and verifies connectivity with PING.
// url must be a Redis URL: redis://:password@host:port/db
func Connect(ctx context.Context, url string) error {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return err
	}

	Client = redis.NewClient(opts)

	if err := Client.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}

// Close shuts down the Redis client gracefully.
func Close() {
	if Client != nil {
		if err := Client.Close(); err != nil {
			log.Printf("[REDIS] close error: %v", err)
		}
	}
}
