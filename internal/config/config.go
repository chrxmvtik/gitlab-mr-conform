package config

import (
	"encoding/base64"
	"fmt"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/pkg/logger"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Port     int    `mapstructure:"port"`
		Host     string `mapstructure:"host"`
		LogLevel string `mapstructure:"log_level"`
	} `mapstructure:"server"`

	GitLab struct {
		Token       string `mapstructure:"token"`
		BaseURL     string `mapstructure:"base_url"`
		SecretToken string `mapstructure:"secret_token"`
		Insecure    bool   `mapstructure:"insecure"`
	} `mapstructure:"gitlab"`

	Rules RulesConfig `mapstructure:"rules"`
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
	Enabled        bool               `mapstructure:"enabled"`
	MinLength      int                `mapstructure:"min_length"`
	MaxLength      int                `mapstructure:"max_length"`
	Conventional   ConventionalConfig `mapstructure:"conventional"`
	ForbiddenWords []string           `mapstructure:"forbidden_words"`
	Jira           JiraConfig         `mapstructure:"jira"`
}

type DescriptionConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	Required        bool `mapstructure:"required"`
	MinLength       int  `mapstructure:"min_length"`
	RequireTemplate bool `mapstructure:"require_template"`
}

type BranchConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	AllowedPrefixes []string `mapstructure:"allowed_prefixes"`
	ForbiddenNames  []string `mapstructure:"forbidden_names"`
}

type CommitsConfig struct {
	Enabled      bool               `mapstructure:"enabled"`
	MaxLength    int                `mapstructure:"max_length"`
	Conventional ConventionalConfig `mapstructure:"conventional"`
	Jira         JiraConfig         `mapstructure:"jira"`
}

type ApprovalsConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	MinCount      int  `mapstructure:"min_count"`
	UseCodeowners bool `mapstructure:"use_codeowners"`
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

// ConfigLoader handles loading and merging configurations
type ConfigLoader struct {
	defaultConfig RulesConfig
	gitlabClient  *gitlab.Client
	logger        *logger.Logger
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("gitlab.base_url", "https://gitlab.com")
	viper.SetDefault("gitlab.insecure", false)
	viper.SetDefault("log_level", "INFO")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	// Environment variables
	viper.SetEnvPrefix("GITLAB_MR_BOT")
	viper.AutomaticEnv()

	_ = viper.BindEnv("gitlab.token")
	_ = viper.BindEnv("gitlab.secrettoken")
	_ = viper.BindEnv("gitlab.base_url")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(defaultConfig RulesConfig, client *gitlab.Client, log *logger.Logger) *ConfigLoader {
	return &ConfigLoader{
		defaultConfig: defaultConfig,
		gitlabClient:  client,
		logger:        log,
	}
}

// LoadConfig loads configuration for a project, trying repository config first, then falling back to default
func (cl *ConfigLoader) LoadConfig(projectID interface{}) (RulesConfig, error) {
	repoConfig, err := cl.loadRepositoryConfig(projectID)
	if err != nil {
		cl.logger.Debug("Using default configuration", "reason", err.Error())
	}

	return cl.selectConfig(repoConfig), nil
}

// loadRepositoryConfig attempts to load config from repository, returns nil if not found or invalid
func (cl *ConfigLoader) loadRepositoryConfig(projectID interface{}) (*RulesConfig, error) {
	// Try to get config file from repository
	cfg, err := cl.gitlabClient.GetConfigFile(projectID)
	if err != nil {
		cl.logger.Debug("No config file found in repository, using default config", "error", err)
		return nil, err
	}

	// Decode the base64 content
	decoded, err := base64.StdEncoding.DecodeString(cfg.Content)
	if err != nil {
		cl.logger.Warn("Failed to decode config file from repository, using default config", "error", err)
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Create a new viper instance to avoid global state conflicts
	v := viper.New()
	v.SetConfigType("yaml")

	err = v.ReadConfig(strings.NewReader(string(decoded)))
	if err != nil {
		cl.logger.Warn("Failed to parse config file from repository, using default config", "error", err)
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	var repoConfig Config
	err = v.Unmarshal(&repoConfig)
	if err != nil {
		cl.logger.Warn("Failed to unmarshal config file from repository, using default config", "error", err)
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cl.logger.Debug("Successfully loaded config from repository")
	return &repoConfig.Rules, nil
}

// selectConfig returns repository config if available, otherwise default config
func (cl *ConfigLoader) selectConfig(repoConfig *RulesConfig) RulesConfig {
	if repoConfig != nil {
		cl.logger.Debug("Using repository configuration")
		return *repoConfig
	}

	cl.logger.Info("Using default configuration")
	return cl.defaultConfig
}
