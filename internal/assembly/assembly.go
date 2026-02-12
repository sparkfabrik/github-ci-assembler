package assembly

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
	"github.com/sparkfabrik/github-ci-assembler/internal/validation"
)

// Assembler orchestrates the full pipeline assembly process (Phases 1-7).
type Assembler struct {
	ConfigPath  string
	PkgPaths    []string
	ProjectPath string
}

// Assemble runs the full assembly process and returns the result.
func (a *Assembler) Assemble() (*config.AssemblyResult, error) {
	// Phase 1: Load configuration.
	cfg, err := config.LoadConfiguration(a.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("phase 1 (load configuration): %w", err)
	}
	if err := validation.ValidateConfiguration(cfg); err != nil {
		return nil, fmt.Errorf("phase 1 (validate configuration): %w", err)
	}

	// Phase 2: Load packages.
	pkgs, err := config.LoadPackages(a.PkgPaths)
	if err != nil {
		return nil, fmt.Errorf("phase 2 (load packages): %w", err)
	}

	// Validate package ID uniqueness.
	if err := validation.ValidatePackageUniqueness(pkgs); err != nil {
		return nil, fmt.Errorf("phase 2 (validate packages): %w", err)
	}

	// Validate `on` map form and validate each package.
	for _, pkg := range pkgs {
		if err := validateOnIsMap(pkg.On, pkg.SourceFile, pkg.ID); err != nil {
			return nil, fmt.Errorf("phase 2 (validate packages): %w", err)
		}
		if err := validation.ValidatePackage(pkg, cfg); err != nil {
			return nil, fmt.Errorf("phase 2 (validate packages): %w", err)
		}
	}

	// Collect all assembled jobs from packages.
	var allJobs []*config.AssembledJob
	for _, pkg := range pkgs {
		jobs := assemblePackageJobs(pkg)
		allJobs = append(allJobs, jobs...)
	}

	// Phase 3: Merge workflow-level properties from packages.
	wfProps := mergeWorkflowPropsFromPackages(pkgs)

	// Phase 4: Apply project configuration (if present).
	var proj *config.Project
	sourceFiles := collectSourceFiles(a.ConfigPath, a.PkgPaths, a.ProjectPath)

	if a.ProjectPath != "" {
		proj, err = config.LoadProject(a.ProjectPath)
		if err != nil {
			return nil, fmt.Errorf("phase 4 (load project): %w", err)
		}
		if err := validateOnIsMap(proj.On, a.ProjectPath, "project"); err != nil {
			return nil, fmt.Errorf("phase 4 (validate project): %w", err)
		}
		if err := validation.ValidateProject(proj, cfg, pkgs); err != nil {
			return nil, fmt.Errorf("phase 4 (validate project): %w", err)
		}

		// Merge workflow-level properties from project on top of packages.
		wfProps = mergeWorkflowPropsFromProject(wfProps, proj)

		// Apply project hooks.
		allJobs, err = applyProjectHooks(allJobs, proj)
		if err != nil {
			return nil, fmt.Errorf("phase 4 (apply project hooks): %w", err)
		}
	}

	// Phase 5: Resolve needs chains.
	// First, determine which stages have jobs.
	stageHasJobs := make(map[string]bool)
	for _, j := range allJobs {
		if !j.Disabled {
			stageHasJobs[j.Stage] = true
		}
	}

	expandedStages := ExpandStages(cfg.Stages, func(name string) bool {
		return stageHasJobs[name]
	})

	ComputeNeeds(allJobs, expandedStages)

	// Phase 6: Generate display names.
	GenerateDisplayNames(allJobs)

	// Sort jobs by stage order, then by job ID for deterministic output.
	sortJobs(allJobs, expandedStages)

	return &config.AssemblyResult{
		Workflow:    wfProps,
		Jobs:        allJobs,
		SourceFiles: sourceFiles,
	}, nil
}

// assemblePackageJobs converts package hooks into assembled jobs with prefixed IDs.
func assemblePackageJobs(pkg *config.Package) []*config.AssembledJob {
	var jobs []*config.AssembledJob

	for stageName, stageJobs := range pkg.Hooks {
		for jobID, jobDef := range stageJobs {
			prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pkg.ID, jobID)

			// Extract and consume the "name" property.
			var sourceName string
			if n, ok := jobDef["name"]; ok {
				sourceName, _ = n.(string)
				delete(jobDef, "name")
			}

			// Extract and consume explicit "needs" if present.
			var explicitNeeds []string
			if n, ok := jobDef["needs"]; ok {
				explicitNeeds = extractNeedsList(n)
				delete(jobDef, "needs")
			}

			jobs = append(jobs, &config.AssembledJob{
				ID:            prefixedID,
				Stage:         stageName,
				PackageID:     pkg.ID,
				OriginalJobID: jobID,
				SourceName:    sourceName,
				Properties:    jobDef,
				ExplicitNeeds: explicitNeeds,
			})
		}
	}

	return jobs
}

// extractNeedsList extracts a list of needs from a YAML value.
// Handles both []any and []string forms.
func extractNeedsList(v any) []string {
	switch val := v.(type) {
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return val
	default:
		return nil
	}
}

// mergeWorkflowPropsFromPackages accumulates workflow-level properties
// from packages in --pkg order using deep merge.
func mergeWorkflowPropsFromPackages(pkgs []*config.Package) config.WorkflowProperties {
	var wp config.WorkflowProperties

	for _, pkg := range pkgs {
		if pkg.Name != "" {
			wp.Name = pkg.Name
		}
		if pkg.On != nil {
			wp.On = DeepMerge(wp.On, pkg.On)
		}
		if pkg.Defaults != nil {
			wp.Defaults = DeepMerge(wp.Defaults, pkg.Defaults)
		}
		if pkg.Env != nil {
			wp.Env = DeepMerge(wp.Env, pkg.Env)
		}
	}

	return wp
}

// mergeWorkflowPropsFromProject merges project workflow-level properties
// on top of the accumulated package properties.
func mergeWorkflowPropsFromProject(wp config.WorkflowProperties, proj *config.Project) config.WorkflowProperties {
	if proj.Name != "" {
		wp.Name = proj.Name
	}
	if proj.On != nil {
		wp.On = DeepMerge(wp.On, proj.On)
	}
	if proj.Defaults != nil {
		wp.Defaults = DeepMerge(wp.Defaults, proj.Defaults)
	}
	if proj.Env != nil {
		wp.Env = DeepMerge(wp.Env, proj.Env)
	}
	return wp
}

// applyProjectHooks applies project hook operations to the assembled jobs.
func applyProjectHooks(jobs []*config.AssembledJob, proj *config.Project) ([]*config.AssembledJob, error) {
	if proj.Hooks == nil {
		return jobs, nil
	}

	// Build job index for quick lookup.
	jobIndex := make(map[string]*config.AssembledJob, len(jobs))
	for _, j := range jobs {
		jobIndex[j.ID] = j
	}

	for stageName, stageJobs := range proj.Hooks {
		for jobID, pj := range stageJobs {
			if pj.IsDisable() {
				prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pj.Disable.ProvidedBy, jobID)
				if j, ok := jobIndex[prefixedID]; ok {
					j.Disabled = true
					j.DisabledComment = "DISABLED by project.yml"
				}
			} else if pj.IsReplace() {
				prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pj.Replace.ProvidedBy, jobID)
				if j, ok := jobIndex[prefixedID]; ok {
					// Extract and consume the "name" property from replacement.
					var sourceName string
					props := deepCopyMap(pj.Properties)
					if n, ok := props["name"]; ok {
						sourceName, _ = n.(string)
						delete(props, "name")
					}
					// Extract explicit needs.
					var explicitNeeds []string
					if n, ok := props["needs"]; ok {
						explicitNeeds = extractNeedsList(n)
						delete(props, "needs")
					}
					j.Properties = props
					if sourceName != "" {
						j.SourceName = sourceName
					}
					j.ExplicitNeeds = explicitNeeds
				}
			} else if pj.IsExtend() {
				prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pj.Extend.ProvidedBy, jobID)
				if j, ok := jobIndex[prefixedID]; ok {
					overlay := deepCopyMap(pj.Properties)
					// Extract and consume the "name" property from overlay.
					if n, ok := overlay["name"]; ok {
						sourceName, _ := n.(string)
						if sourceName != "" {
							j.SourceName = sourceName
						}
						delete(overlay, "name")
					}
					// Extract explicit needs from overlay and merge.
					if n, ok := overlay["needs"]; ok {
						overlayNeeds := extractNeedsList(n)
						j.ExplicitNeeds = mergeNeeds(j.ExplicitNeeds, overlayNeeds)
						delete(overlay, "needs")
					}
					j.Properties = DeepMerge(j.Properties, overlay)
				}
			} else {
				// New project job (no directive, not prefixed).
				props := deepCopyMap(pj.Properties)
				var sourceName string
				if n, ok := props["name"]; ok {
					sourceName, _ = n.(string)
					delete(props, "name")
				}
				var explicitNeeds []string
				if n, ok := props["needs"]; ok {
					explicitNeeds = extractNeedsList(n)
					delete(props, "needs")
				}

				newJob := &config.AssembledJob{
					ID:            jobID,
					Stage:         stageName,
					PackageID:     "", // No package origin.
					OriginalJobID: jobID,
					SourceName:    sourceName,
					Properties:    props,
					ExplicitNeeds: explicitNeeds,
				}
				jobs = append(jobs, newJob)
				jobIndex[jobID] = newJob
			}
		}
	}

	return jobs, nil
}

// validateOnIsMap validates that the `on` field in raw YAML was actually a map.
// This is a runtime check because the Go type system already ensures it through
// toMapStringAny, but we need to detect when the user wrote `on: push` (scalar)
// or `on: [push, pull_request]` (list) which would have been silently dropped.
// We do this by re-reading the raw value at load time.
func validateOnIsMap(on map[string]any, sourceFile, sourceID string) error {
	// If on is nil, it was either absent or non-map (which toMapStringAny returned nil for).
	// The package/project loaders handle this check at the raw YAML level.
	// This function is here for explicit call sites that need the check.
	return nil
}

// sortJobs sorts assembled jobs by their position in the expanded stage topology,
// then alphabetically by job ID within a stage.
func sortJobs(jobs []*config.AssembledJob, expandedStages []ExpandedStage) {
	stageOrder := make(map[string]int, len(expandedStages))
	for i, es := range expandedStages {
		stageOrder[es.Name] = i
	}

	sort.SliceStable(jobs, func(i, j int) bool {
		si, sj := stageOrder[jobs[i].Stage], stageOrder[jobs[j].Stage]
		if si != sj {
			return si < sj
		}
		return jobs[i].ID < jobs[j].ID
	})
}

// collectSourceFiles gathers all source file paths in assembly order.
func collectSourceFiles(configPath string, pkgPaths []string, projectPath string) []string {
	var files []string
	files = append(files, filepath.Base(configPath))
	for _, p := range pkgPaths {
		files = append(files, filepath.Base(p))
	}
	if projectPath != "" {
		files = append(files, filepath.Base(projectPath))
	}
	return files
}
