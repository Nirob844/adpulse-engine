package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/nirob844/adpulse-engine/internal/model"
)

// Client wraps the redis.Client with domain-specific caching and counter operations.
type Client struct {
	rdb *redis.Client
}

// Connect parses the Redis URL and establishes a connection.
func Connect(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}

	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Ping checks Redis connectivity.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return errors.New("redis client not configured")
	}
	return c.rdb.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// GetCandidates retrieves cached active ad candidates for a given slot size.
func (c *Client) GetCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, bool, error) {
	if c == nil || c.rdb == nil {
		return nil, false, nil
	}

	key := fmt.Sprintf("ad:candidates:%dx%d", width, height)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil // Cache miss
		}
		return nil, false, err
	}

	var candidates []*model.CreativeCandidate
	if err := json.Unmarshal([]byte(val), &candidates); err != nil {
		return nil, false, err
	}

	return candidates, true, nil
}

// SetCandidates caches active ad candidates for a slot size with TTL.
func (c *Client) SetCandidates(ctx context.Context, width, height int, candidates []*model.CreativeCandidate, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}

	key := fmt.Sprintf("ad:candidates:%dx%d", width, height)
	data, err := json.Marshal(candidates)
	if err != nil {
		return fmt.Errorf("failed to marshal candidates: %w", err)
	}

	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// GetFrequencyCount returns the number of impressions served to an IP for a specific creative today.
func (c *Client) GetFrequencyCount(ctx context.Context, ip string, creativeID int64, date string) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}

	key := fmt.Sprintf("fcap:%s:%d:%s", ip, creativeID, date)
	val, err := c.rdb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// GetCampaignDailyImpressions gets the cached daily impression count for a campaign.
func (c *Client) GetCampaignDailyImpressions(ctx context.Context, campaignID int64, date string) (int64, bool, error) {
	if c == nil || c.rdb == nil {
		return 0, false, nil
	}

	key := fmt.Sprintf("camp:daily:%d:%s", campaignID, date)
	val, err := c.rdb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false, nil // Not yet cached
		}
		return 0, false, err
	}
	return val, true, nil
}

// SetCampaignDailyImpressions primes the daily impression count for a campaign.
func (c *Client) SetCampaignDailyImpressions(ctx context.Context, campaignID int64, date string, count int64) error {
	if c == nil || c.rdb == nil {
		return nil
	}

	key := fmt.Sprintf("camp:daily:%d:%s", campaignID, date)
	return c.rdb.Set(ctx, key, count, 26*time.Hour).Err()
}

// IncrImpression atomically increments both the IP frequency cap counter and campaign daily count.
func (c *Client) IncrImpression(ctx context.Context, ip string, creativeID int64, campaignID int64, date string) error {
	if c == nil || c.rdb == nil {
		return nil
	}

	fcapKey := fmt.Sprintf("fcap:%s:%d:%s", ip, creativeID, date)
	campKey := fmt.Sprintf("camp:daily:%d:%s", campaignID, date)

	pipe := c.rdb.Pipeline()
	fcapCmd := pipe.Incr(ctx, fcapKey)
	pipe.Expire(ctx, fcapKey, 26*time.Hour)

	pipe.Incr(ctx, campKey)
	pipe.Expire(ctx, campKey, 26*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to increment impression counters: %w", err)
	}

	_ = fcapCmd.Val()
	return nil
}

