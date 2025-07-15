package server

import (
	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/internal/queue"
	"gitlab-mr-conformity-bot/internal/storage"
	"gitlab-mr-conformity-bot/pkg/logger"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config       *config.Config
	gitlabClient *gitlab.Client
	checker      *conformity.Checker
	storage      storage.Storage
	logger       *logger.Logger
	queueManager *queue.QueueManager
}

func NewServer(cfg *config.Config, client *gitlab.Client, checker *conformity.Checker, store storage.Storage, log *logger.Logger, queueManager *queue.QueueManager) *Server {
	return &Server{
		config:       cfg,
		gitlabClient: client,
		checker:      checker,
		storage:      store,
		logger:       log,
		queueManager: queueManager,
	}
}

func (s *Server) Router() *gin.Engine {
	if gin.Mode() != gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Health check
	router.GET("/health", s.handleHealth)

	// Webhook endpoint

	if s.config.Queue.Enabled {
		router.POST("/webhook", s.HandleWebhook)
	} else {
		router.POST("/webhook", s.handleWebhookNoQueue)
	}

	// Status endpoint
	router.GET("/status/:project_id/:mr_id", s.handleStatus)

	return router
}
