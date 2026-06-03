package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestRedisCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := NewRedisCache(mr.Addr(), "", 0)
	return rc, mr
}

func TestMemoryCache_SetGet(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	if err := c.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("Get() = %q, %v, want value, true", got, ok)
	}
}

func TestMemoryCache_TTL_Expired(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	if err := c.Set(ctx, "key", []byte("value"), 20*time.Millisecond); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok || got != nil {
		t.Fatalf("Get() = %q, %v, want nil, false", got, ok)
	}
}

func TestMemoryCache_TTL_NotExpired(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	if err := c.Set(ctx, "key", []byte("value"), time.Second); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("Get() = %q, %v, want value, true", got, ok)
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	_ = c.Set(ctx, "key", []byte("value"), 0)
	if err := c.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestMemoryCache_Exists(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	_ = c.Set(ctx, "key", []byte("value"), 0)

	ok, err := c.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist")
	}
}

func TestRedisCache_SetGet(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("Get() = %q, %v, want value, true", got, ok)
	}
}

func TestRedisCache_TTL_Expired(t *testing.T) {
	c, mr := newTestRedisCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	mr.FastForward(2 * time.Minute)

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok || got != nil {
		t.Fatalf("Get() = %q, %v, want nil, false", got, ok)
	}
}

func TestRedisCache_Delete(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "key", []byte("value"), time.Minute)
	if err := c.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestRedisCache_Exists(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "key", []byte("value"), time.Minute)

	ok, err := c.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist")
	}
}

func TestTieredCache_L1Hit(t *testing.T) {
	ctx := context.Background()
	l1 := NewMemoryCache()
	l2 := NewMemoryCache()
	c := NewTieredCache(l1, l2)

	if err := l1.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("Get() = %q, %v, want value, true", got, ok)
	}
}

func TestTieredCache_L2Hit_PopulatesL1(t *testing.T) {
	ctx := context.Background()
	l1 := NewMemoryCache()
	l2 := NewMemoryCache()
	c := NewTieredCache(l1, l2)

	if err := l2.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("Get() = %q, %v, want value, true", got, ok)
	}

	cached, ok, err := l1.Get(ctx, "key")
	if err != nil {
		t.Fatalf("L1 Get() error = %v", err)
	}
	if !ok || string(cached) != "value" {
		t.Fatalf("L1 Get() = %q, %v, want value, true", cached, ok)
	}
}

func TestTieredCache_Set_WritesToBoth(t *testing.T) {
	ctx := context.Background()
	l1 := NewMemoryCache()
	l2 := NewMemoryCache()
	c := NewTieredCache(l1, l2)

	if err := c.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	for name, backend := range map[string]*MemoryCache{"l1": l1, "l2": l2} {
		got, ok, err := backend.Get(ctx, "key")
		if err != nil {
			t.Fatalf("%s Get() error = %v", name, err)
		}
		if !ok || string(got) != "value" {
			t.Fatalf("%s Get() = %q, %v, want value, true", name, got, ok)
		}
	}
}

func TestTieredCache_Delete_DeletesBoth(t *testing.T) {
	ctx := context.Background()
	l1 := NewMemoryCache()
	l2 := NewMemoryCache()
	c := NewTieredCache(l1, l2)

	_ = c.Set(ctx, "key", []byte("value"), time.Minute)
	if err := c.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	for name, backend := range map[string]*MemoryCache{"l1": l1, "l2": l2} {
		_, ok, err := backend.Get(ctx, "key")
		if err != nil {
			t.Fatalf("%s Get() error = %v", name, err)
		}
		if ok {
			t.Fatalf("expected %s key to be deleted", name)
		}
	}
}
