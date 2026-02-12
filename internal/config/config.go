package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfiguration parses a configuration.yml file.
func LoadConfiguration(path string) (*Configuration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading configuration file %q: %w", path, err)
	}

	var cfg Configuration
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing configuration file %q: %w", path, err)
	}

	return &cfg, nil
}
