package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level configuration file at ~/.config/druid-mcp/config.yaml
type Config struct {
	Environments map[string]Environment `yaml:"environments"`
}

// Environment represents a single Druid sandbox environment.
type Environment struct {
	URL  string `yaml:"url"`
	Auth string `yaml:"auth"` // Full Authorization header value, e.g. "Basic dXNlcjpwYXNz"
}

// configPath returns the path to the config file.
func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "druid-mcp", "config.yaml")
}

// Load reads and parses the config file.
func Load() (*Config, error) {
	path := configPath()
	if path == "" {
		return nil, fmt.Errorf("could not determine home directory")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"config file not found at %s\n\nCreate it with:\n\n"+
					"  mkdir -p ~/.config/druid-mcp\n"+
					"  cat > ~/.config/druid-mcp/config.yaml << 'EOF'\n"+
					"  environments:\n"+
					"    sb5:\n"+
					"      url: https://druid.sb5.example.com\n"+
					"      auth: \"Basic dXNlcjpwYXNz\"\n"+
					"  EOF", path)
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.Environments == nil || len(cfg.Environments) == 0 {
		return nil, fmt.Errorf("config file has no environments defined")
	}

	return &cfg, nil
}

// GetEnvironment returns the environment config for the given name.
func (c *Config) GetEnvironment(name string) (*Environment, error) {
	env, ok := c.Environments[name]
	if !ok {
		available := make([]string, 0, len(c.Environments))
		for k := range c.Environments {
			available = append(available, k)
		}
		return nil, fmt.Errorf("environment %q not found in config. Available: %v", name, available)
	}

	if env.URL == "" {
		return nil, fmt.Errorf("environment %q has no URL configured", name)
	}
	if env.Auth == "" {
		return nil, fmt.Errorf("environment %q has no auth configured", name)
	}

	return &env, nil
}

// ListEnvironments returns all configured environment names and their URLs.
func (c *Config) ListEnvironments() []EnvironmentInfo {
	result := make([]EnvironmentInfo, 0, len(c.Environments))
	for name, env := range c.Environments {
		result = append(result, EnvironmentInfo{
			Name:           name,
			URL:            env.URL,
			AuthConfigured: env.Auth != "",
		})
	}
	return result
}

// EnvironmentInfo is a summary of an environment for listing purposes.
type EnvironmentInfo struct {
	Name           string
	URL            string
	AuthConfigured bool
}
