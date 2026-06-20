package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const lobehubSSOCodePrefix = "lobehub:sso:code:"

type lobehubSSOCache struct {
	rdb *redis.Client
}

func NewLobeHubSSOCache(rdb *redis.Client) service.LobeHubSSOCodeStore {
	return &lobehubSSOCache{rdb: rdb}
}

func lobehubSSOCodeKey(code string) string {
	return fmt.Sprintf("%s%s", lobehubSSOCodePrefix, code)
}

func (c *lobehubSSOCache) Store(ctx context.Context, code string, payload service.LobeHubSSOCodePayload, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return errors.New("lobehub sso cache unavailable")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, lobehubSSOCodeKey(code), data, ttl).Err()
}

func (c *lobehubSSOCache) Take(ctx context.Context, code string) (*service.LobeHubSSOCodePayload, error) {
	if c == nil || c.rdb == nil {
		return nil, errors.New("lobehub sso cache unavailable")
	}
	key := lobehubSSOCodeKey(code)
	data, err := c.rdb.GetDel(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, service.ErrLobeHubSSOCodeInvalid
	}
	if err != nil {
		return nil, err
	}
	var payload service.LobeHubSSOCodePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
