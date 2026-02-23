package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadProject parses a project.yml file.
// The project file has a complex structure: the hooks section contains
// jobs that may have extend/replace/disable directives mixed with
// native GHA properties. We parse these manually.
func LoadProject(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading project file %q: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing project file %q: %w", path, err)
	}

	proj := &Project{}

	if _, ok := raw["name"]; ok {
		return nil, fmt.Errorf("invalid top-level key \"name\" in project file %q.\n       \"name\" is only allowed in configuration.yml", path)
	}
	if _, ok := raw["on"]; ok {
		return nil, fmt.Errorf("invalid top-level key \"on\" in project file %q.\n       \"on\" is only allowed in configuration.yml", path)
	}
	if _, ok := raw["defaults"]; ok {
		return nil, fmt.Errorf("invalid top-level key \"defaults\" in project file %q.\n       \"defaults\" is only allowed in configuration.yml", path)
	}
	if v, ok := raw["env"]; ok {
		m := toMapStringAny(v)
		if m == nil && v != nil {
			return nil, fmt.Errorf("invalid \"env\" format in project file %q: \"env\" must be a map", path)
		}
		proj.Env = m
	}
	if v, ok := raw["permissions"]; ok {
		m := toMapStringAny(v)
		if m == nil && v != nil {
			return nil, fmt.Errorf("invalid \"permissions\" format in project file %q: \"permissions\" must be a map", path)
		}
		proj.Permissions = m
	}

	// Parse hooks with directive detection.
	if hooksRaw, ok := raw["hooks"]; ok {
		hooksMap := toMapStringAny(hooksRaw)
		proj.Hooks = make(map[string]map[string]ProjectJob, len(hooksMap))
		for stageName, stageVal := range hooksMap {
			stageJobs := toMapStringAny(stageVal)
			jobs := make(map[string]ProjectJob, len(stageJobs))
			for jobID, jobVal := range stageJobs {
				jobMap := toMapStringAny(jobVal)
				pj, err := parseProjectJob(jobMap)
				if err != nil {
					return nil, fmt.Errorf("project file %q, stage %q, job %q: %w", path, stageName, jobID, err)
				}
				jobs[jobID] = pj
			}
			proj.Hooks[stageName] = jobs
		}
	}

	return proj, nil
}

// parseProjectJob parses a single job entry from project.yml,
// extracting any extend/replace/disable directive and separating
// it from the native GHA properties.
func parseProjectJob(jobMap map[string]any) (ProjectJob, error) {
	pj := ProjectJob{
		Properties: make(map[string]any),
	}

	directiveCount := 0

	for key, val := range jobMap {
		switch key {
		case "extend":
			ref, err := parseProvidedByRef(val)
			if err != nil {
				return pj, fmt.Errorf("invalid extend directive: %w", err)
			}
			pj.Extend = ref
			directiveCount++
		case "replace":
			ref, err := parseProvidedByRef(val)
			if err != nil {
				return pj, fmt.Errorf("invalid replace directive: %w", err)
			}
			pj.Replace = ref
			directiveCount++
		case "disable":
			ref, err := parseProvidedByRef(val)
			if err != nil {
				return pj, fmt.Errorf("invalid disable directive: %w", err)
			}
			pj.Disable = ref
			directiveCount++
		default:
			pj.Properties[key] = val
		}
	}

	if directiveCount > 1 {
		return pj, fmt.Errorf("cannot declare multiple directives (extend, replace, disable) on the same job")
	}

	return pj, nil
}

// parseProvidedByRef parses the provided_by reference from a directive value.
func parseProvidedByRef(val any) (*ProvidedByRef, error) {
	m := toMapStringAny(val)
	if m == nil {
		return nil, fmt.Errorf("expected a map with 'provided_by' key")
	}
	pb, ok := m["provided_by"]
	if !ok {
		return nil, fmt.Errorf("missing 'provided_by' key")
	}
	s, ok := pb.(string)
	if !ok {
		return nil, fmt.Errorf("'provided_by' must be a string")
	}
	return &ProvidedByRef{ProvidedBy: s}, nil
}
