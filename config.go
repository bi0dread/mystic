package mystic

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	LogLevel string `env:"LOG_LEVEL, required"`
	Facility string `env:"FACILITY, required"`
}

// ConfigProfile represents different environment configurations
type ConfigProfile struct {
	Name          string
	Config        Config
	GraylogConfig GraylogSenderConfig
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() (*Config, error) {
	config := &Config{}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	} else {
		config.LogLevel = "info" // default
	}

	if facility := os.Getenv("FACILITY"); facility != "" {
		config.Facility = facility
	} else {
		return nil, fmt.Errorf("FACILITY environment variable is required")
	}

	return config, nil
}

// LoadGraylogConfigFromEnv loads Graylog configuration from environment variables
func LoadGraylogConfigFromEnv() (*GraylogSenderConfig, error) {
	config := &GraylogSenderConfig{}

	if graylogAddr := os.Getenv("GRAYLOG_ADDR"); graylogAddr != "" {
		config.GrayLogAddr = graylogAddr
	} else {
		config.GrayLogAddr = "0.0.0.0:12201" // default
	}

	if facility := os.Getenv("FACILITY"); facility != "" {
		config.Facility = facility
	} else {
		return nil, fmt.Errorf("FACILITY environment variable is required")
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.LogLevel == "" {
		return fmt.Errorf("LogLevel is required")
	}

	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"panic": true,
	}

	if !validLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("invalid LogLevel: %s, must be one of: debug, info, warn, error, panic", c.LogLevel)
	}

	if c.Facility == "" {
		return fmt.Errorf("Facility is required")
	}

	return nil
}

// Validate validates the Graylog configuration
func (c *GraylogSenderConfig) Validate() error {
	if c.Facility == "" {
		return fmt.Errorf("Facility is required")
	}

	// GrayLogAddr can be empty (will use fallback)
	return nil
}

// SetDefaults sets default values for the configuration
func (c *Config) SetDefaults() {
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}

// SetDefaults sets default values for the Graylog configuration
func (c *GraylogSenderConfig) SetDefaults() {
	if c.GrayLogAddr == "" {
		c.GrayLogAddr = "0.0.0.0:12201"
	}
}

// LoadProfile loads a specific configuration profile
func LoadProfile(profile string) (*ConfigProfile, error) {
	switch strings.ToLower(profile) {
	case "development":
		return &ConfigProfile{
			Name: "development",
			Config: Config{
				LogLevel: "debug",
				Facility: "mystic-dev",
			},
			GraylogConfig: GraylogSenderConfig{
				GrayLogAddr: "localhost:12201",
				Facility:    "mystic-dev",
			},
		}, nil
	case "staging":
		return &ConfigProfile{
			Name: "staging",
			Config: Config{
				LogLevel: "info",
				Facility: "mystic-staging",
			},
			GraylogConfig: GraylogSenderConfig{
				GrayLogAddr: "staging-graylog:12201",
				Facility:    "mystic-staging",
			},
		}, nil
	case "production":
		return &ConfigProfile{
			Name: "production",
			Config: Config{
				LogLevel: "warn",
				Facility: "mystic-prod",
			},
			GraylogConfig: GraylogSenderConfig{
				GrayLogAddr: "prod-graylog:12201",
				Facility:    "mystic-prod",
			},
		}, nil
	case "testing":
		return &ConfigProfile{
			Name: "testing",
			Config: Config{
				LogLevel: "debug",
				Facility: "mystic-test",
			},
			GraylogConfig: GraylogSenderConfig{
				GrayLogAddr: "",
				Facility:    "mystic-test",
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown profile: %s", profile)
	}
}
