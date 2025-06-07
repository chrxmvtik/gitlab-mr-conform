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
	"gitlab-mr-conformity-bot/internal/server"
	"gitlab-mr-conformity-bot/internal/storage"
	"gitlab-mr-conformity-bot/pkg/logger"
)

func main() {
	// Initialize logger
	log := logger.New()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration", "error", err)
	}

	// Initialize GitLab client
	gitlabClient, err := gitlab.NewClient(cfg.GitLab.Token, cfg.GitLab.BaseURL, cfg.GitLab.Insecure)
	if err != nil {
		log.Fatal("Failed to create GitLab client", "error", err)
	}

	log.Info("Connected to GitLab server", "server", cfg.GitLab.BaseURL)

	// Initialize storage
	store := storage.NewMemoryStorage()

	// Initialize conformity checker
	checker := conformity.NewChecker(cfg.Rules, gitlabClient, log)

	// Initialize HTTP server
	srv := server.NewServer(cfg, gitlabClient, checker, store, log)

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
