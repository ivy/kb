package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level configuration for the KB application.
type Config struct {
	Database  Database  `mapstructure:"database"`
	Server    Server    `mapstructure:"server"`
	Tailscale Tailscale `mapstructure:"tailscale"`
	GitLab    GitLab    `mapstructure:"gitlab"`
	Embedding Embedding `mapstructure:"embedding"`
	Agent     Agent     `mapstructure:"agent"`
	Sessions  Sessions  `mapstructure:"sessions"`
	Workers   Workers   `mapstructure:"workers"`
	Log       Log       `mapstructure:"log"`
}

// Server holds HTTP server configuration.
type Server struct {
	Listen string `mapstructure:"listen"`
}

// Database holds database connection configuration.
type Database struct {
	URL string `mapstructure:"url"`
}

// Tailscale holds Tailscale networking configuration.
type Tailscale struct {
	Enabled  bool   `mapstructure:"enabled"`
	Hostname string `mapstructure:"hostname"`
	StateDir string `mapstructure:"state_dir"`
	AuthZ    bool   `mapstructure:"authz"`
	AskCap   string `mapstructure:"ask_cap"`
	AdminCap string `mapstructure:"admin_cap"`
}

// GitLab holds GitLab integration configuration.
type GitLab struct {
	Token string `mapstructure:"token"`
}

// Embedding holds embedding provider configuration.
type Embedding struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
	APIKey   string `mapstructure:"api_key"`
}

// Agent holds agent configuration.
type Agent struct {
	DefaultModel string `mapstructure:"default_model"`
}

// Sessions holds session management configuration.
type Sessions struct {
	Dir string `mapstructure:"dir"`
	Max int    `mapstructure:"max"`
}

// Workers holds worker pool configuration.
type Workers struct {
	Reindex int `mapstructure:"reindex"`
	Embed   int `mapstructure:"embed"`
}

// Log holds logging configuration.
type Log struct {
	Format string `mapstructure:"format"`
	Level  string `mapstructure:"level"`
}

// Load reads configuration from a YAML file and environment variables.
// Environment variables use the KB_ prefix and override file values.
// If configPath is empty, it looks for kb.yaml in the current directory.
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("kb")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("KB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}
