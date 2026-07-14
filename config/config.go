// Package config provides configuration loading for the Azure Exemption CLI.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	// BlockedPolicyDefinitionIDs is a list of policy definition IDs that cannot be exempted.
	// These definitions will appear greyed out and be non-selectable in the UI.
	BlockedPolicyDefinitionIDs []string `yaml:"blocked_policy_definition_ids"`
}

// DefaultConfigPaths returns the list of paths to search for the config file.
func DefaultConfigPaths() []string {
	paths := []string{
		"config.yaml",
		"config.yml",
	}

	// Add user home directory config
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".azexempt", "config.yaml"),
			filepath.Join(home, ".azexempt", "config.yml"),
			filepath.Join(home, ".azure-exemption-cli", "config.yaml"),
			filepath.Join(home, ".azure-exemption-cli", "config.yml"),
		)
	}

	// Add XDG config directory
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		paths = append(paths,
			filepath.Join(xdgConfig, "azexempt", "config.yaml"),
			filepath.Join(xdgConfig, "azexempt", "config.yml"),
			filepath.Join(xdgConfig, "azure-exemption-cli", "config.yaml"),
			filepath.Join(xdgConfig, "azure-exemption-cli", "config.yml"),
		)
	} else if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "azexempt", "config.yaml"),
			filepath.Join(home, ".config", "azexempt", "config.yml"),
			filepath.Join(home, ".config", "azure-exemption-cli", "config.yaml"),
			filepath.Join(home, ".config", "azure-exemption-cli", "config.yml"),
		)
	}

	return paths
}

// Load attempts to load configuration from the default paths.
// Returns an empty Config if no config file is found (not an error).
func Load() (*Config, error) {
	return LoadFromPaths(DefaultConfigPaths())
}

// LoadFromPaths attempts to load configuration from the given paths.
// The first path that exists and is readable will be used.
// Returns an empty Config if no config file is found (not an error).
func LoadFromPaths(paths []string) (*Config, error) {
	for _, path := range paths {
		cfg, err := LoadFromFile(path)
		if err == nil {
			return cfg, nil
		}
		if !os.IsNotExist(err) {
			// Return error for non-missing file errors (e.g., parse errors)
			return nil, err
		}
	}

	// No config file found, return empty config
	return &Config{}, nil
}

// LoadFromFile loads configuration from a specific file path.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// BlockedDefinitionsMap converts the blocked policy definition IDs slice to a map
// for O(1) lookup. The map keys are normalized to lowercase for case-insensitive matching.
func (c *Config) BlockedDefinitionsMap() map[string]bool {
	blocked := make(map[string]bool, len(c.BlockedPolicyDefinitionIDs))
	for _, id := range c.BlockedPolicyDefinitionIDs {
		blocked[strings.ToLower(id)] = true
	}
	return blocked
}
