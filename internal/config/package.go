package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadPackage parses a single pkg_*.yml file.
func LoadPackage(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading package file %q: %w", path, err)
	}

	// First, do a raw parse to handle the untyped job definitions.
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing package file %q: %w", path, err)
	}

	pkg := &Package{
		SourceFile: path,
	}

	// Extract typed top-level fields.
	if v, ok := raw["id"]; ok {
		pkg.ID, _ = v.(string)
	}
	if v, ok := raw["name"]; ok {
		pkg.Name, _ = v.(string)
	}
	if v, ok := raw["on"]; ok {
		m := toMapStringAny(v)
		if m == nil && v != nil {
			return nil, fmt.Errorf("invalid \"on\" format in %s (id: %s).\n       \"on\" must be a map (e.g., on: { push: { branches: [main] } }).\n       Shorthand forms like \"on: push\" or \"on: [push, pull_request]\" are not allowed", path, pkg.ID)
		}
		pkg.On = m
	}
	if v, ok := raw["defaults"]; ok {
		pkg.Defaults = toMapStringAny(v)
	}
	if v, ok := raw["env"]; ok {
		pkg.Env = toMapStringAny(v)
	}

	// Parse hooks: map[stage] → map[job-id] → map[string]any
	if hooksRaw, ok := raw["hooks"]; ok {
		hooksMap := toMapStringAny(hooksRaw)
		pkg.Hooks = make(map[string]JobMap, len(hooksMap))
		for stageName, stageVal := range hooksMap {
			stageJobs := toMapStringAny(stageVal)
			jobs := make(JobMap, len(stageJobs))
			for jobID, jobVal := range stageJobs {
				jobs[jobID] = toMapStringAny(jobVal)
			}
			pkg.Hooks[stageName] = jobs
		}
	}

	return pkg, nil
}

// LoadPackages loads multiple package files in order.
func LoadPackages(paths []string) ([]*Package, error) {
	pkgs := make([]*Package, 0, len(paths))
	for _, p := range paths {
		pkg, err := LoadPackage(p)
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// toMapStringAny converts an interface{} to map[string]any.
// YAML maps decode as map[string]any with yaml.v3.
func toMapStringAny(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
