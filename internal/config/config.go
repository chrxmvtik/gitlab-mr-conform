package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
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
	Enabled  bool `mapstructure:"enabled"`
	Required bool `mapstructure:"required"`
	MinCount int  `mapstructure:"min_count"`
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
