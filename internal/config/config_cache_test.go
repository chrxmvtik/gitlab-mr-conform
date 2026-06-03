package config

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
	internalgitlab "gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/pkg/logger"
)

type configTestServer struct {
	server       *httptest.Server
	projectCalls int32
	fileCalls    int32
}

func newConfigTestClient(t *testing.T, statusCode int, content string) (*internalgitlab.Client, *configTestServer) {
	t.Helper()

	ts := &configTestServer{}
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case r.Method == http.MethodGet && strings.Contains(path, "/repository/files/"):
			atomic.AddInt32(&ts.fileCalls, 1)
			if statusCode == http.StatusNotFound {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"message":"404 Not Found"}`)
				return
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(content))
			fmt.Fprintf(w, `{"file_name":".mr-conform.yaml","content":"%s","encoding":"base64"}`, encoded)
		case r.Method == http.MethodGet && strings.Contains(path, "/api/v4/projects/"):
			atomic.AddInt32(&ts.projectCalls, 1)
			fmt.Fprint(w, `{"id":1,"name":"Test","default_branch":"main","path_with_namespace":"group/test"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message":"unhandled %s"}`, path)
		}
	}))
	t.Cleanup(ts.server.Close)

	client, err := internalgitlab.NewClient("token", ts.server.URL, false)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client, ts
}

func defaultRulesConfig() RulesConfig {
	return RulesConfig{Title: TitleConfig{Enabled: true, MinLength: 5}}
}

func TestConfigLoader_SkipCache_Hit(t *testing.T) {
	client, calls := newConfigTestClient(t, http.StatusNotFound, "")
	c := cache.NewMemoryCache()
	loader := NewConfigLoaderWithCache(defaultRulesConfig(), client, logger.New(), c)

	if err := c.Set(context.Background(), "gitlab:cache:skip:1", []byte("1"), time.Hour); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	cfg, presence, err := loader.LoadConfig("1")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if presence != ConfigNotFound || cfg.Title.Enabled != true {
		t.Fatalf("presence = %v, cfg = %+v", presence, cfg)
	}
	if atomic.LoadInt32(&calls.projectCalls) != 0 || atomic.LoadInt32(&calls.fileCalls) != 0 {
		t.Fatalf("expected no GitLab calls, got project=%d file=%d", calls.projectCalls, calls.fileCalls)
	}
}

func TestConfigLoader_SkipCache_Miss_ThenCached(t *testing.T) {
	client, calls := newConfigTestClient(t, http.StatusNotFound, "")
	loader := NewConfigLoaderWithCache(defaultRulesConfig(), client, logger.New(), cache.NewMemoryCache())

	for i := 0; i < 2; i++ {
		_, presence, err := loader.LoadConfig("1")
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if presence != ConfigNotFound {
			t.Fatalf("presence = %v, want ConfigNotFound", presence)
		}
	}

	if atomic.LoadInt32(&calls.projectCalls) != 1 || atomic.LoadInt32(&calls.fileCalls) != 1 {
		t.Fatalf("calls = project:%d file:%d, want 1 each", calls.projectCalls, calls.fileCalls)
	}
}

func TestConfigLoader_ConfigCache_Hit(t *testing.T) {
	content := "rules:\n  title:\n    enabled: false\n    min_length: 2\n"
	client, calls := newConfigTestClient(t, http.StatusOK, content)
	loader := NewConfigLoaderWithCache(defaultRulesConfig(), client, logger.New(), cache.NewMemoryCache())

	for i := 0; i < 2; i++ {
		cfg, presence, err := loader.LoadConfig("1")
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if presence != ConfigPopulated {
			t.Fatalf("presence = %v, want ConfigPopulated", presence)
		}
		if cfg.Title.Enabled {
			t.Fatalf("cfg.Title.Enabled = true, want false")
		}
	}

	if atomic.LoadInt32(&calls.projectCalls) != 1 || atomic.LoadInt32(&calls.fileCalls) != 1 {
		t.Fatalf("calls = project:%d file:%d, want 1 each", calls.projectCalls, calls.fileCalls)
	}
}

func TestConfigLoader_ConfigEmpty_Cached(t *testing.T) {
	client, calls := newConfigTestClient(t, http.StatusOK, "")
	loader := NewConfigLoaderWithCache(defaultRulesConfig(), client, logger.New(), cache.NewMemoryCache())

	for i := 0; i < 2; i++ {
		cfg, presence, err := loader.LoadConfig("1")
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if presence != ConfigEmpty {
			t.Fatalf("presence = %v, want ConfigEmpty", presence)
		}
		if !cfg.Title.Enabled {
			t.Fatalf("cfg.Title.Enabled = false, want true")
		}
	}

	if atomic.LoadInt32(&calls.projectCalls) != 1 || atomic.LoadInt32(&calls.fileCalls) != 1 {
		t.Fatalf("calls = project:%d file:%d, want 1 each", calls.projectCalls, calls.fileCalls)
	}
}

func TestConfigLoader_Cache_Nil_Works(t *testing.T) {
	content := "rules:\n  title:\n    enabled: false\n"
	client, calls := newConfigTestClient(t, http.StatusOK, content)
	loader := NewConfigLoader(defaultRulesConfig(), client, logger.New())

	for i := 0; i < 2; i++ {
		cfg, presence, err := loader.LoadConfig("1")
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if presence != ConfigPopulated {
			t.Fatalf("presence = %v, want ConfigPopulated", presence)
		}
		if cfg.Title.Enabled {
			t.Fatalf("cfg.Title.Enabled = true, want false")
		}
	}

	if atomic.LoadInt32(&calls.projectCalls) != 2 || atomic.LoadInt32(&calls.fileCalls) != 2 {
		t.Fatalf("calls = project:%d file:%d, want 2 each", calls.projectCalls, calls.fileCalls)
	}
}

func TestConfigLoader_Cache_TTL_Expired(t *testing.T) {
	content := "rules:\n  title:\n    enabled: false\n"
	client, calls := newConfigTestClient(t, http.StatusOK, content)
	loader := NewConfigLoaderWithCache(defaultRulesConfig(), client, logger.New(), cache.NewMemoryCache())
	loader.configTTL = 25 * time.Millisecond

	if _, _, err := loader.LoadConfig("1"); err != nil {
		t.Fatalf("first LoadConfig() error = %v", err)
	}
	if atomic.LoadInt32(&calls.projectCalls) != 1 || atomic.LoadInt32(&calls.fileCalls) != 1 {
		t.Fatalf("calls after first load = project:%d file:%d, want 1 each", calls.projectCalls, calls.fileCalls)
	}

	time.Sleep(50 * time.Millisecond)

	if _, _, err := loader.LoadConfig("1"); err != nil {
		t.Fatalf("second LoadConfig() error = %v", err)
	}
	if atomic.LoadInt32(&calls.projectCalls) != 2 || atomic.LoadInt32(&calls.fileCalls) != 2 {
		t.Fatalf("calls after expiry = project:%d file:%d, want 2 each", calls.projectCalls, calls.fileCalls)
	}
}
