package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab-mr-conformity-bot/internal/cache"
	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/internal/queue"
	"gitlab-mr-conformity-bot/internal/server"
	"gitlab-mr-conformity-bot/pkg/logger"
)

func main() {
	log := logger.New()
	log.Info("Starting bot", "version", Version)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration", "error", err)
	}

	log.SetLevel(cfg.Server.LogLevel)

	queueConfig := &queue.Config{
		RedisHost:          cfg.Queue.Redis.Host,
		RedisPassword:      cfg.Queue.Redis.Password,
		RedisDB:            cfg.Queue.Redis.DB,
		QueuePrefix:        "gitlab:mr:queue",
		LockPrefix:         "gitlab:mr:lock",
		ProcessingPrefix:   "gitlab:mr:processing",
		DefaultLockTTL:     cfg.Queue.Queue.LockTTL,
		MaxRetries:         cfg.Queue.Queue.MaxRetries,
		ProcessingInterval: cfg.Queue.Queue.ProcessingInterval,
		WorkerPoolSize:     cfg.Queue.Queue.WorkerPoolSize,
	}
	queueManager := queue.NewQueueManager(queueConfig, log)

	var appCache cache.Cache
	if cfg.Queue.Enabled {
		appCache = cache.NewTieredCache(
			cache.NewMemoryCache(),
			cache.NewRedisCache(cfg.Queue.Redis.Host, cfg.Queue.Redis.Password, cfg.Queue.Redis.DB),
		)
	} else {
		appCache = cache.NewMemoryCache()
	}

	gitlabClient, err := gitlab.NewClientWithCache(cfg.GitLab.Token, cfg.GitLab.BaseURL, cfg.GitLab.Insecure, appCache)
	if err != nil {
		log.Fatal("Failed to create GitLab client", "error", err)
	}

	log.Info("Connected to GitLab server", "server", cfg.GitLab.BaseURL)

	checker := conformity.NewCheckerWithCache(cfg.Rules, gitlabClient, log, cfg.Integrations, appCache)
	srv := server.NewServer(cfg, gitlabClient, checker, nil, log, queueManager)

	c, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Queue.Enabled {
		go srv.StartProcessor(c)
	} else {
		log.Info("Queue processing disabled, webhooks will be processed synchronously")
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: srv.Router(),
	}

	go func() {
		log.Info("Starting server", "port", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", "error", err)
	}

	log.Info("Server exited")
}
