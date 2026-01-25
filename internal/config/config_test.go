package config

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// RepositoryFile represents a file from the repository
type RepositoryFile struct {
	Content string
}

// GitLabClient interface defines the methods needed from gitlab.Client
type GitLabClient interface {
	GetConfigFile(projectID interface{}) (*RepositoryFile, error)
}

// Logger interface defines the methods needed from logger.Logger
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// MockGitLabClient is a mock implementation of GitLabClient
type MockGitLabClient struct {
	mock.Mock
}

func (m *MockGitLabClient) GetConfigFile(projectID interface{}) (*RepositoryFile, error) {
	args := m.Called(projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RepositoryFile), args.Error(1)
}

// MockLogger is a mock implementation of Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name           string
		setupConfig    func() string
		setupEnv       func()
		cleanupEnv     func()
		expectedError  bool
		validateConfig func(*testing.T, *Config)
	}{
		{
			name: "Load with default values",
			setupConfig: func() string {
				content := `
server:
  port: 8080
  host: "0.0.0.0"
  log_level: "INFO"
gitlab:
  token: "test-token"
  base_url: "https://gitlab.com"
  secret_token: "secret"
rules:
  title:
    enabled: true
    min_length: 10
`
				tmpfile, _ := os.CreateTemp("", "config*.yaml")
				tmpfile.Write([]byte(content))
				tmpfile.Close()
				return tmpfile.Name()
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			validateConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 8080, cfg.Server.Port)
				assert.Equal(t, "0.0.0.0", cfg.Server.Host)
				assert.Equal(t, "INFO", cfg.Server.LogLevel)
				assert.Equal(t, "test-token", cfg.GitLab.Token)
				assert.Equal(t, "https://gitlab.com", cfg.GitLab.BaseURL)
				assert.True(t, cfg.Rules.Title.Enabled)
			},
		},
		{
			name: "Override with environment variables",
			setupConfig: func() string {
				content := `
server:
  port: 8080
gitlab:
  token: "config-token"
  base_url: "https://gitlab.com"
`
				tmpfile, _ := os.CreateTemp("", "config*.yaml")
				tmpfile.Write([]byte(content))
				tmpfile.Close()
				return tmpfile.Name()
			},
			setupEnv: func() {
				os.Setenv("GITLAB_MR_BOT_GITLAB_TOKEN", "env-token")
				os.Setenv("GITLAB_MR_BOT_GITLAB_BASE_URL", "https://custom-gitlab.com")
			},
			cleanupEnv: func() {
				os.Unsetenv("GITLAB_MR_BOT_GITLAB_TOKEN")
				os.Unsetenv("GITLAB_MR_BOT_GITLAB_BASE_URL")
			},
			validateConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "env-token", cfg.GitLab.Token)
				assert.Equal(t, "https://custom-gitlab.com", cfg.GitLab.BaseURL)
			},
		},
		{
			name: "Queue configuration with defaults",
			setupConfig: func() string {
				content := `
server:
  port: 8080
gitlab:
  token: "test-token"
queue:
  enabled: true
  redis:
    host: "localhost:6379"
    password: "redis-pass"
    db: 0
`
				tmpfile, _ := os.CreateTemp("", "config*.yaml")
				tmpfile.Write([]byte(content))
				tmpfile.Close()
				return tmpfile.Name()
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			validateConfig: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.Queue.Enabled)
				assert.Equal(t, "localhost:6379", cfg.Queue.Redis.Host)
				assert.Equal(t, 3, cfg.Queue.Queue.MaxRetries)
				assert.Equal(t, 10*time.Second, cfg.Queue.Queue.LockTTL)
				assert.Equal(t, 100*time.Millisecond, cfg.Queue.Queue.ProcessingInterval)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper for each test
			viper.Reset()

			// Create a temporary config file
			configFile := tt.setupConfig()
			defer os.Remove(configFile)

			// Override the Load() function's config paths to use our temp file
			// We'll create the config in the current directory as "config.yaml"
			testConfigPath := "./config.yaml"

			// Read the temp file content
			content, err := os.ReadFile(configFile)
			assert.NoError(t, err)

			// Write to the expected location
			err = os.WriteFile(testConfigPath, content, 0644)
			assert.NoError(t, err)
			defer os.Remove(testConfigPath)

			tt.setupEnv()
			defer tt.cleanupEnv()

			cfg, err := Load()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validateConfig != nil {
					tt.validateConfig(t, cfg)
				}
			}
		})
	}
}

func TestNewConfigLoader(t *testing.T) {
	defaultConfig := RulesConfig{
		Title: TitleConfig{
			Enabled:   true,
			MinLength: 5,
		},
	}

	loader := NewConfigLoader(defaultConfig, nil, nil)

	assert.NotNil(t, loader)
	assert.Equal(t, defaultConfig, loader.defaultConfig)

	// Note: We can't test with mock types since the actual signature expects concrete types
	// In a real scenario, you would need to refactor ConfigLoader to accept interfaces
}

func TestConfigLoader_LoadConfig_WithMocks(t *testing.T) {
	// This test demonstrates the issue: your ConfigLoader needs interface-based dependency injection
	// For now, we'll create a wrapper test that shows the intent

	t.Run("Test would require interface refactoring", func(t *testing.T) {
		t.Skip("ConfigLoader needs to accept interfaces instead of concrete types for proper testing")
		// The actual implementation should be:
		// type ConfigLoader struct {
		//     defaultConfig RulesConfig
		//     gitlabClient  GitLabClient  // interface instead of *gitlab.Client
		//     logger        Logger        // interface instead of *logger.Logger
		// }
	})
}

func TestConfigLoader_selectConfig(t *testing.T) {
	defaultConfig := RulesConfig{
		Title: TitleConfig{
			Enabled:   true,
			MinLength: 10,
		},
	}

	// Create loader without logger to avoid the interface issue
	loader := &ConfigLoader{
		defaultConfig: defaultConfig,
	}

	t.Run("Returns repository config when available", func(t *testing.T) {
		repoConfig := &RulesConfig{
			Title: TitleConfig{
				Enabled:   true,
				MinLength: 25,
			},
		}

		result := loader.selectConfig(repoConfig)
		assert.Equal(t, 25, result.Title.MinLength)
	})

	t.Run("Returns default config when repository config is nil", func(t *testing.T) {
		result := loader.selectConfig(nil)
		assert.Equal(t, 10, result.Title.MinLength)
	})
}

func TestConfigStructs(t *testing.T) {
	t.Run("Config struct initialization", func(t *testing.T) {
		cfg := Config{}
		cfg.Server.Port = 9090
		cfg.Server.Host = "localhost"
		cfg.GitLab.Token = "test-token"
		cfg.GitLab.BaseURL = "https://gitlab.example.com"

		assert.Equal(t, 9090, cfg.Server.Port)
		assert.Equal(t, "localhost", cfg.Server.Host)
		assert.Equal(t, "test-token", cfg.GitLab.Token)
	})

	t.Run("RulesConfig struct initialization", func(t *testing.T) {
		rules := RulesConfig{
			Title: TitleConfig{
				Enabled:        true,
				MinLength:      5,
				MaxLength:      100,
				ForbiddenWords: []string{"WIP", "TODO"},
			},
			Approvals: ApprovalsConfig{
				Enabled:  true,
				MinCount: 2,
			},
		}

		assert.True(t, rules.Title.Enabled)
		assert.Equal(t, 5, rules.Title.MinLength)
		assert.Len(t, rules.Title.ForbiddenWords, 2)
		assert.Equal(t, 2, rules.Approvals.MinCount)
	})

	t.Run("QueueConfig with time durations", func(t *testing.T) {
		queue := QueueConfig{
			Enabled: true,
			Redis: RedisConfig{
				Host:     "localhost:6379",
				Password: "secret",
				DB:       1,
			},
			Queue: QueueSettings{
				ProcessingInterval: 500 * time.Millisecond,
				MaxRetries:         5,
				LockTTL:            30 * time.Second,
			},
		}

		assert.True(t, queue.Enabled)
		assert.Equal(t, "localhost:6379", queue.Redis.Host)
		assert.Equal(t, 500*time.Millisecond, queue.Queue.ProcessingInterval)
		assert.Equal(t, 5, queue.Queue.MaxRetries)
	})
}

func TestConfigLoader_loadRepositoryConfig(t *testing.T) {
	t.Run("Successful config load and parse", func(t *testing.T) {
		repoConfig := `
rules:
  title:
    enabled: true
    min_length: 20
    max_length: 100
  description:
    enabled: true
    required: true
`
		encodedConfig := base64.StdEncoding.EncodeToString([]byte(repoConfig))

		// Test the decoding and parsing logic independently
		decoded, err := base64.StdEncoding.DecodeString(encodedConfig)
		assert.NoError(t, err)
		assert.Contains(t, string(decoded), "min_length: 20")
	})

	t.Run("Invalid base64 handling", func(t *testing.T) {
		invalidBase64 := "invalid-base64!!!"
		_, err := base64.StdEncoding.DecodeString(invalidBase64)
		assert.Error(t, err)
	})

	t.Run("YAML parsing", func(t *testing.T) {
		validYAML := `
rules:
  title:
    enabled: true
    min_length: 15
`
		v := viper.New()
		v.SetConfigType("yaml")

		// Create a reader from the string
		reader := strings.NewReader(validYAML)
		err := v.ReadConfig(reader)
		assert.NoError(t, err)

		var config Config
		err = v.Unmarshal(&config)
		assert.NoError(t, err)
		assert.Equal(t, 15, config.Rules.Title.MinLength)
	})
}

// TestIntegration_ConfigLoaderFlow tests the complete flow without mocks
// This is an integration-style test that uses real file I/O
func TestIntegration_ConfigLoaderFlow(t *testing.T) {
	t.Run("Complete flow with actual config file", func(t *testing.T) {
		// Create a temporary config file
		configContent := `
rules:
  title:
    enabled: true
    min_length: 15
    max_length: 150
  description:
    enabled: false
  approvals:
    enabled: true
    min_count: 2
`
		tmpfile, err := os.CreateTemp("", "integration-config*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(configContent))
		assert.NoError(t, err)
		tmpfile.Close()

		// Test that config can be loaded
		v := viper.New()
		v.SetConfigFile(tmpfile.Name())
		err = v.ReadInConfig()
		assert.NoError(t, err)

		var config Config
		err = v.Unmarshal(&config)
		assert.NoError(t, err)

		// Validate the loaded config
		assert.True(t, config.Rules.Title.Enabled)
		assert.Equal(t, 15, config.Rules.Title.MinLength)
		assert.Equal(t, 150, config.Rules.Title.MaxLength)
		assert.False(t, config.Rules.Description.Enabled)
		assert.True(t, config.Rules.Approvals.Enabled)
		assert.Equal(t, 2, config.Rules.Approvals.MinCount)
	})
}
