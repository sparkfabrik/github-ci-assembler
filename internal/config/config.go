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

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing configuration file %q: %w", path, err)
	}

	cfg := Configuration{}

	if v, ok := raw["version"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return nil, fmt.Errorf("invalid \"version\" in %q: must be a string (e.g., version: \"1\"), got %T", path, v)
		}
		cfg.Version = s
	}

	if v, ok := raw["stages"]; ok {
		switch val := v.(type) {
		case []any:
			stages := make([]string, 0, len(val))
			for _, stage := range val {
				s, ok := stage.(string)
				if !ok {
					return nil, fmt.Errorf("invalid stage entry in %q: all stages must be strings", path)
				}
				stages = append(stages, s)
			}
			cfg.Stages = stages
		default:
			return nil, fmt.Errorf("invalid \"stages\" format in %q: expected a list of stage names", path)
		}
	}

	if v, ok := raw["name"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return nil, fmt.Errorf("invalid \"name\" in %q: must be a string, got %T", path, v)
		}
		cfg.Name = s
	}
	if v, ok := raw["on"]; ok {
		m := toMapStringAny(v)
		if m == nil && v != nil {
			return nil, fmt.Errorf("invalid \"on\" format in %q.\n       \"on\" must be a map (e.g., on: { push: { branches: [main] } }).\n       Shorthand forms like \"on: push\" or \"on: [push, pull_request]\" are not allowed", path)
		}
		cfg.On = m
	}
	if v, ok := raw["defaults"]; ok {
		m := toMapStringAny(v)
		if m == nil && v != nil {
			return nil, fmt.Errorf("invalid \"defaults\" format in %q: \"defaults\" must be a map", path)
		}
		cfg.Defaults = m
	}
	if v, ok := raw["env"]; ok {
		m := toMapStringAny(v)
		if m == nil && v != nil {
			return nil, fmt.Errorf("invalid \"env\" format in %q: \"env\" must be a map", path)
		}
		cfg.Env = m
	}
	if v, ok := raw["permissions"]; ok {
		m := toMapStringAny(v)
		if m == nil && v != nil {
			return nil, fmt.Errorf("invalid \"permissions\" format in %q: \"permissions\" must be a map", path)
		}
		cfg.Permissions = m
	}

	return &cfg, nil
}
