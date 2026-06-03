package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gitlab-mr-conformity-bot/internal/cache"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/pkg/logger"

	"github.com/spf13/viper"
	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type Config struct {
	Server struct {
		Port     int    `mapstructure:"port"`
		Host     string `mapstructure:"host"`
		LogLevel string `mapstructure:"log_level"`
	} `mapstructure:"server"`

	GitLab struct {
		Token                 string `mapstructure:"token"`
		BaseURL               string `mapstructure:"base_url"`
		SecretToken           string `mapstructure:"secret_token"`
		SystemHookSecretToken string `mapstructure:"system_hook_secret_token"`
		Insecure              bool   `mapstructure:"insecure"`
	} `mapstructure:"gitlab"`

	Rules RulesConfig `mapstructure:"rules"`

	Queue QueueConfig `mapstructure:"queue"`

	Integrations IntegrationsConfig `mapstructure:"integrations"`
}

// QueueConfig holds Redis queue configuration.
type QueueConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Queue   QueueSettings `mapstructure:"queue"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// QueueSettings holds queue behavior settings.
type QueueSettings struct {
	ProcessingInterval time.Duration `mapstructure:"processing_interval"`
	MaxRetries         int           `mapstructure:"max_retries"`
	LockTTL            time.Duration `mapstructure:"lock_ttl"`
	WorkerPoolSize     int           `mapstructure:"worker_pool_size"`
}

// Integrations settings.
type IntegrationsConfig struct {
	Asana AsanaConfig `mapstructure:"asana"`
}

// AsanaConfig holds Asana integration settings.
type AsanaConfig struct {
	APIToken string `mapstructure:"api_token"`
}

type RulesConfig struct {
	Title       TitleConfig       `mapstructure:"title"`
	Description DescriptionConfig `mapstructure:"description"`
	Branch      BranchConfig      `mapstructure:"branch"`
	Commits     CommitsConfig     `mapstructure:"commits"`
	Approvals   ApprovalsConfig   `mapstructure:"approvals"`
	Squash      SquashConfig      `mapstructure:"squash"`
}

type TitleConfig struct {
	Enabled        bool                 `mapstructure:"enabled"`
	MinLength      int                  `mapstructure:"min_length"`
	MaxLength      int                  `mapstructure:"max_length"`
	Conventional   ConventionalConfig   `mapstructure:"conventional"`
	ForbiddenWords []string             `mapstructure:"forbidden_words"`
	Jira           JiraConfig           `mapstructure:"jira"`
	Asana          AsanaValidatorConfig `mapstructure:"asana"`
}

type DescriptionConfig struct {
	Enabled         bool                 `mapstructure:"enabled"`
	Required        bool                 `mapstructure:"required"`
	MinLength       int                  `mapstructure:"min_length"`
	RequireTemplate bool                 `mapstructure:"require_template"`
	Jira            JiraConfig           `mapstructure:"jira"`
	Asana           AsanaValidatorConfig `mapstructure:"asana"`
}

type BranchConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	AllowedPrefixes []string `mapstructure:"allowed_prefixes"`
	ForbiddenNames  []string `mapstructure:"forbidden_names"`
}

type CommitsConfig struct {
	Enabled      bool                 `mapstructure:"enabled"`
	MaxLength    int                  `mapstructure:"max_length"`
	Conventional ConventionalConfig   `mapstructure:"conventional"`
	Jira         JiraConfig           `mapstructure:"jira"`
	Asana        AsanaValidatorConfig `mapstructure:"asana"`
}

type ApprovalsConfig struct {
	Enabled                 bool `mapstructure:"enabled"`
	MinCount                int  `mapstructure:"min_count"`
	UseCodeowners           bool `mapstructure:"use_codeowners"`
	ExcludeCreatorFromCount bool `mapstructure:"exclude_creator_from_count"`
}

type SquashConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	EnforceBranches  []string `mapstructure:"enforce_branches"`
	DisallowBranches []string `mapstructure:"disallow_branches"`
}

type ConventionalConfig struct {
	Types  []string `mapstructure:"types"`
	Scopes []string `mapstructure:"scopes"`
}

type JiraConfig struct {
	Keys []string `mapstructure:"keys"`
}

type AsanaValidatorConfig struct {
	Keys              []string `mapstructure:"keys"`
	ValidateExistence bool     `mapstructure:"validate_existence"`
}

// ConfigPresence describes whether a .mr-conform.yaml file was found in the repository.
type ConfigPresence int

const (
	// ConfigNotFound means the file does not exist; the repo should be skipped entirely.
	ConfigNotFound ConfigPresence = iota
	// ConfigEmpty means the file exists but is empty or whitespace-only; use the global default config.
	ConfigEmpty
	// ConfigPopulated means the file exists and contains configuration; use the repository config.
	ConfigPopulated
)

const (
	defaultConfigCacheTTL = 5 * time.Minute
	defaultSkipCacheTTL   = time.Hour
)

type configFetcher interface {
	GetConfigFile(projectID interface{}) (*gitlabapi.File, error)
}

// ConfigLoader handles loading and merging configurations.
type ConfigLoader struct {
	defaultConfig RulesConfig
	gitlabClient  configFetcher
	logger        *logger.Logger
	cache         cache.Cache
	configTTL     time.Duration
	skipTTL       time.Duration
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.log_level", "INFO")
	viper.SetDefault("gitlab.base_url", "https://gitlab.com")
	viper.SetDefault("gitlab.insecure", false)
	viper.SetDefault("queue.enabled", false)
	viper.SetDefault("queue.queue.lock_ttl", "10s")
	viper.SetDefault("queue.queue.max_retries", 3)
	viper.SetDefault("queue.queue.processing_interval", "100ms")
	viper.SetDefault("queue.queue.worker_pool_size", 10)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	viper.SetEnvPrefix("GITLAB_MR_BOT")
	viper.AutomaticEnv()

	_ = viper.BindEnv("gitlab.token")
	_ = viper.BindEnv("gitlab.secrettoken")
	_ = viper.BindEnv("gitlab.base_url")
	_ = viper.BindEnv("queue.redis.password")
	_ = viper.BindEnv("integrations.asana.api_token")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// NewConfigLoader creates a new configuration loader.
func NewConfigLoader(defaultConfig RulesConfig, client *gitlab.Client, log *logger.Logger) *ConfigLoader {
	return NewConfigLoaderWithCache(defaultConfig, client, log, nil)
}

func NewConfigLoaderWithCache(defaultConfig RulesConfig, client *gitlab.Client, log *logger.Logger, c cache.Cache) *ConfigLoader {
	return &ConfigLoader{
		defaultConfig: defaultConfig,
		gitlabClient:  client,
		logger:        log,
		cache:         c,
		configTTL:     defaultConfigCacheTTL,
		skipTTL:       defaultSkipCacheTTL,
	}
}

// LoadConfig loads configuration for a project, trying repository config first, then falling back to default.
// It also returns a ConfigPresence value so callers can decide whether to process the repository at all.
func (cl *ConfigLoader) LoadConfig(projectID interface{}) (RulesConfig, ConfigPresence, error) {
	ctx := context.Background()

	if cl.cache != nil {
		if ok, err := cl.cache.Exists(ctx, cl.skipCacheKey(projectID)); err == nil && ok {
			cl.logger.Debug("Skip cache hit for repository", "project_id", projectID)
			return cl.defaultConfig, ConfigNotFound, nil
		}

		if data, ok, err := cl.cache.Get(ctx, cl.configCacheKey(projectID)); err == nil && ok {
			if string(data) == "empty" {
				cl.logger.Debug("Config cache hit for empty repository config", "project_id", projectID)
				return cl.defaultConfig, ConfigEmpty, nil
			}

			var cached RulesConfig
			if err := json.Unmarshal(data, &cached); err == nil {
				cl.logger.Debug("Config cache hit for repository", "project_id", projectID)
				return cached, ConfigPopulated, nil
			}

			cl.logger.Warn("Failed to decode cached repository config", "project_id", projectID)
			_ = cl.cache.Delete(ctx, cl.configCacheKey(projectID))
		}
	}

	repoConfig, presence, err := cl.loadRepositoryConfig(projectID)
	if err != nil {
		return cl.defaultConfig, ConfigNotFound, fmt.Errorf("failed to load repository config: %w", err)
	}

	switch presence {
	case ConfigNotFound:
		cl.logger.Debug("No .mr-conform.yaml found, skipping repository")
		cl.cacheSkipResult(ctx, projectID)
		return cl.defaultConfig, ConfigNotFound, nil
	case ConfigEmpty:
		cl.logger.Info("Empty .mr-conform.yaml found, using default configuration")
		cl.cacheEmptyConfig(ctx, projectID)
		return cl.defaultConfig, ConfigEmpty, nil
	default:
		cl.logger.Debug("Using repository configuration from .mr-conform.yaml")
		cl.cachePopulatedConfig(ctx, projectID, *repoConfig)
		return *repoConfig, ConfigPopulated, nil
	}
}

// loadRepositoryConfig attempts to load config from repository.
// Returns (nil, ConfigNotFound, nil) if the file is absent,
// (nil, ConfigEmpty, nil) if the file exists but is empty,
// or (*RulesConfig, ConfigPopulated, nil) if the file has content.
func (cl *ConfigLoader) loadRepositoryConfig(projectID interface{}) (*RulesConfig, ConfigPresence, error) {
	cfg, err := cl.gitlabClient.GetConfigFile(projectID)
	if err != nil {
		cl.logger.Debug("No .mr-conform.yaml found in repository", "error", err)
		return nil, ConfigNotFound, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(cfg.Content)
	if err != nil {
		cl.logger.Warn("Failed to decode .mr-conform.yaml, using default config", "error", err)
		return nil, ConfigNotFound, fmt.Errorf("failed to decode config: %w", err)
	}

	if len(strings.TrimSpace(string(decoded))) == 0 {
		cl.logger.Debug(".mr-conform.yaml is empty, using default configuration")
		return nil, ConfigEmpty, nil
	}

	v := viper.New()
	v.SetConfigType("yaml")

	if err = v.ReadConfig(strings.NewReader(string(decoded))); err != nil {
		cl.logger.Warn("Failed to parse .mr-conform.yaml, using default config", "error", err)
		return nil, ConfigNotFound, fmt.Errorf("failed to parse config: %w", err)
	}

	var repoConfig Config
	if err = v.Unmarshal(&repoConfig); err != nil {
		cl.logger.Warn("Failed to unmarshal .mr-conform.yaml, using default config", "error", err)
		return nil, ConfigNotFound, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cl.logger.Debug("Successfully loaded config from repository")
	return &repoConfig.Rules, ConfigPopulated, nil
}

func (cl *ConfigLoader) skipCacheKey(projectID interface{}) string {
	return fmt.Sprintf("gitlab:cache:skip:%v", projectID)
}

func (cl *ConfigLoader) configCacheKey(projectID interface{}) string {
	return fmt.Sprintf("gitlab:cache:config:%v", projectID)
}

func (cl *ConfigLoader) cacheSkipResult(ctx context.Context, projectID interface{}) {
	if cl.cache == nil {
		return
	}
	_ = cl.cache.Delete(ctx, cl.configCacheKey(projectID))
	_ = cl.cache.Set(ctx, cl.skipCacheKey(projectID), []byte("1"), cl.skipTTL)
}

func (cl *ConfigLoader) cacheEmptyConfig(ctx context.Context, projectID interface{}) {
	if cl.cache == nil {
		return
	}
	_ = cl.cache.Delete(ctx, cl.skipCacheKey(projectID))
	_ = cl.cache.Set(ctx, cl.configCacheKey(projectID), []byte("empty"), cl.configTTL)
}

func (cl *ConfigLoader) cachePopulatedConfig(ctx context.Context, projectID interface{}, cfg RulesConfig) {
	if cl.cache == nil {
		return
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		cl.logger.Warn("Failed to marshal repository config for cache", "project_id", projectID, "error", err)
		return
	}

	_ = cl.cache.Delete(ctx, cl.skipCacheKey(projectID))
	_ = cl.cache.Set(ctx, cl.configCacheKey(projectID), data, cl.configTTL)
}
