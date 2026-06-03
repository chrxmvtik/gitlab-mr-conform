package cache

import (
	"context"
	"time"
)

const defaultTieredL1TTL = time.Minute

// TieredCache combines a fast local cache with a shared backing cache.
type TieredCache struct {
	l1    *MemoryCache
	l2    Cache
	l1TTL time.Duration
}

func NewTieredCache(l1 *MemoryCache, l2 Cache) *TieredCache {
	if l1 == nil {
		l1 = NewMemoryCache()
	}

	return &TieredCache{
		l1:    l1,
		l2:    l2,
		l1TTL: defaultTieredL1TTL,
	}
}

func (c *TieredCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.l1.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	if c.l2 == nil {
		return nil
	}
	return c.l2.Set(ctx, key, value, ttl)
}

func (c *TieredCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if value, ok, err := c.l1.Get(ctx, key); err != nil || ok {
		return value, ok, err
	}
	if c.l2 == nil {
		return nil, false, nil
	}

	value, ok, err := c.l2.Get(ctx, key)
	if err != nil || !ok {
		return value, ok, err
	}

	_ = c.l1.Set(ctx, key, value, c.l1TTL)
	return value, true, nil
}

func (c *TieredCache) Delete(ctx context.Context, key string) error {
	if err := c.l1.Delete(ctx, key); err != nil {
		return err
	}
	if c.l2 == nil {
		return nil
	}
	return c.l2.Delete(ctx, key)
}

func (c *TieredCache) Exists(ctx context.Context, key string) (bool, error) {
	if ok, err := c.l1.Exists(ctx, key); err != nil || ok {
		return ok, err
	}
	if c.l2 == nil {
		return false, nil
	}

	ok, err := c.l2.Exists(ctx, key)
	if err != nil || !ok {
		return ok, err
	}

	if value, hit, getErr := c.l2.Get(ctx, key); getErr == nil && hit {
		_ = c.l1.Set(ctx, key, value, c.l1TTL)
	}
	return true, nil
}
