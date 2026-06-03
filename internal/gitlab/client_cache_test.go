package gitlab

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitlab-mr-conformity-bot/internal/cache"
)

type branchCacheServer struct {
	server       *httptest.Server
	projectCalls int32
	fileCalls    int32
}

func newBranchCacheTestClient(t *testing.T, c cache.Cache) (*Client, *branchCacheServer) {
	t.Helper()

	ts := &branchCacheServer{}
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case r.Method == http.MethodGet && strings.Contains(path, "/repository/files/"):
			atomic.AddInt32(&ts.fileCalls, 1)
			content := base64.StdEncoding.EncodeToString([]byte("test"))
			fmt.Fprintf(w, `{"file_name":"file","content":"%s","encoding":"base64"}`, content)
		case r.Method == http.MethodGet && strings.Contains(path, "/api/v4/projects/"):
			atomic.AddInt32(&ts.projectCalls, 1)
			fmt.Fprint(w, `{"id":1,"name":"Test","default_branch":"main","path_with_namespace":"group/test"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message":"unhandled %s"}`, path)
		}
	}))
	t.Cleanup(ts.server.Close)

	client, err := NewClientWithCache("token", ts.server.URL, false, c)
	if err != nil {
		t.Fatalf("NewClientWithCache() error = %v", err)
	}
	return client, ts
}

func TestGetDefaultBranch_CacheHit(t *testing.T) {
	c := cache.NewMemoryCache()
	if err := c.Set(context.Background(), "gitlab:cache:branch:1", []byte("main"), time.Hour); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	client, calls := newBranchCacheTestClient(t, c)

	if _, err := client.GetConfigFile("1"); err != nil {
		t.Fatalf("GetConfigFile() error = %v", err)
	}
	if atomic.LoadInt32(&calls.projectCalls) != 0 {
		t.Fatalf("project calls = %d, want 0", calls.projectCalls)
	}
	if atomic.LoadInt32(&calls.fileCalls) != 1 {
		t.Fatalf("file calls = %d, want 1", calls.fileCalls)
	}
}

func TestGetDefaultBranch_CacheMiss_ThenCached(t *testing.T) {
	client, calls := newBranchCacheTestClient(t, cache.NewMemoryCache())

	for i := 0; i < 2; i++ {
		if _, err := client.GetConfigFile("1"); err != nil {
			t.Fatalf("GetConfigFile() error = %v", err)
		}
	}

	if atomic.LoadInt32(&calls.projectCalls) != 1 {
		t.Fatalf("project calls = %d, want 1", calls.projectCalls)
	}
	if atomic.LoadInt32(&calls.fileCalls) != 2 {
		t.Fatalf("file calls = %d, want 2", calls.fileCalls)
	}
}

func TestGetConfigFile_AndGetCodeownersFile_ShareBranchCache(t *testing.T) {
	client, calls := newBranchCacheTestClient(t, cache.NewMemoryCache())

	if _, err := client.GetConfigFile("1"); err != nil {
		t.Fatalf("GetConfigFile() error = %v", err)
	}
	if _, err := client.GetCodeownersFile("1"); err != nil {
		t.Fatalf("GetCodeownersFile() error = %v", err)
	}

	if atomic.LoadInt32(&calls.projectCalls) != 1 {
		t.Fatalf("project calls = %d, want 1", calls.projectCalls)
	}
	if atomic.LoadInt32(&calls.fileCalls) != 2 {
		t.Fatalf("file calls = %d, want 2", calls.fileCalls)
	}
}

func TestGetDefaultBranch_NilCache_Works(t *testing.T) {
	client, calls := newBranchCacheTestClient(t, nil)

	for i := 0; i < 2; i++ {
		if _, err := client.GetConfigFile("1"); err != nil {
			t.Fatalf("GetConfigFile() error = %v", err)
		}
	}

	if atomic.LoadInt32(&calls.projectCalls) != 2 {
		t.Fatalf("project calls = %d, want 2", calls.projectCalls)
	}
	if atomic.LoadInt32(&calls.fileCalls) != 2 {
		t.Fatalf("file calls = %d, want 2", calls.fileCalls)
	}
}
