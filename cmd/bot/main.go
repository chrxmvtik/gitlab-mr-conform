package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/internal/queue"
	"gitlab-mr-conformity-bot/internal/server"
	"gitlab-mr-conformity-bot/internal/storage"
	"gitlab-mr-conformity-bot/pkg/logger"
)

func main() {
	// Initialize logger
	log := logger.New()

	log.Info("Starting bot", "version", Version)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration", "error", err)
	}

	log.SetLevel(cfg.Server.LogLevel)

	// Initialize Redis queue manager
	queueConfig := &queue.Config{
		RedisHost:          cfg.Queue.Redis.Host,
		RedisPassword:      cfg.Queue.Redis.Password,
		RedisDB:            cfg.Queue.Redis.DB,
		QueuePrefix:        "gitlab:mr:queue",
		LockPrefix:         "gitlab:mr:lock",
		ProcessingPrefix:   "gitlab:mr:processing",
		DefaultLockTTL:     cfg.Queue.Queue.LockTTL,            //10 * time.Second,
		MaxRetries:         cfg.Queue.Queue.MaxRetries,         //3,
		ProcessingInterval: cfg.Queue.Queue.ProcessingInterval, // 100 * time.Milisecond,
	}

	queueManager := queue.NewQueueManager(queueConfig, log)

	// Initialize GitLab client
	gitlabClient, err := gitlab.NewClient(cfg.GitLab.Token, cfg.GitLab.BaseURL, cfg.GitLab.Insecure)
	if err != nil {
		log.Fatal("Failed to create GitLab client", "error", err)
	}

	log.Info("Connected to GitLab server", "server", cfg.GitLab.BaseURL)

	// Initialize storage
	store := storage.NewMemoryStorage()

	// Initialize conformity checker
	checker := conformity.NewChecker(cfg.Rules, gitlabClient, log, cfg.Integrations)

	// Initialize HTTP server
	srv := server.NewServer(cfg, gitlabClient, checker, store, log, queueManager)

	// Create context for graceful shutdown
	c, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the background job processor only if queue is enabled
	if cfg.Queue.Enabled {
		go srv.StartProcessor(c)
	} else {
		log.Info("Queue processing disabled, webhooks will be processed synchronously")
	}

	// Start server
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

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", "error", err)
	}

	log.Info("Server exited")
}
