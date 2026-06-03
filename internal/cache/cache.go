package cache

import (
	"context"
	"time"
)

// Cache is a generic key-value cache with TTL support.
type Cache interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
