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

	// Validate each package.
	for _, pkg := range pkgs {
		if err := validation.ValidatePackage(pkg, cfg); err != nil {
			return nil, fmt.Errorf("phase 2 (validate packages): %w", err)
		}
	}

	// Phase 3: Initialize workflow-level properties from configuration.yml.
	wfProps := workflowPropsFromConfiguration(cfg)

	// Collect all assembled jobs from packages.
	var allJobs []*config.AssembledJob
	for _, pkg := range pkgs {
		jobs, err := assemblePackageJobs(pkg)
		if err != nil {
			return nil, fmt.Errorf("phase 2 (assemble package jobs): %w", err)
		}
		allJobs = append(allJobs, jobs...)
	}

	// Phase 4: Apply project configuration (if present).
	var proj *config.Project
	sourceFiles := collectSourceFiles(a.ConfigPath, a.PkgPaths, a.ProjectPath)

	if a.ProjectPath != "" {
		proj, err = config.LoadProject(a.ProjectPath)
		if err != nil {
			return nil, fmt.Errorf("phase 4 (load project): %w", err)
		}
		if err := validation.ValidateProject(proj, cfg, pkgs); err != nil {
			return nil, fmt.Errorf("phase 4 (validate project): %w", err)
		}

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
func assemblePackageJobs(pkg *config.Package) ([]*config.AssembledJob, error) {
	var jobs []*config.AssembledJob

	for stageName, stageJobs := range pkg.Hooks {
		stageJobIndex := make(map[string]string, len(stageJobs))
		for jobID := range stageJobs {
			stageJobIndex[jobID] = fmt.Sprintf("%s--%s--%s", stageName, pkg.ID, jobID)
		}

		for jobID, rawJobDef := range stageJobs {
			jobDef := deepCopyMap(rawJobDef)
			prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pkg.ID, jobID)

			// Merge file-level env into job env (job-level env wins).
			mergeFileEnvIntoJob(jobDef, pkg.Env)

			// Extract and consume the "name" property.
			var sourceName string
			if n, ok := jobDef["name"]; ok {
				sourceName, _ = n.(string)
				delete(jobDef, "name")
			}

			// Extract and consume explicit "needs" if present.
			localNeeds, err := consumeNeedsList(jobDef, pkg.SourceFile, stageName, jobID)
			if err != nil {
				return nil, err
			}
			explicitNeeds, err := resolveLocalNeeds(localNeeds, stageJobIndex, pkg.SourceFile, stageName, jobID, "package stage")
			if err != nil {
				return nil, err
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

	return jobs, nil
}

func workflowPropsFromConfiguration(cfg *config.Configuration) config.WorkflowProperties {
	return config.WorkflowProperties{
		Name:     cfg.Name,
		On:       cfg.On,
		Defaults: cfg.Defaults,
	}
}

// consumeNeedsList extracts and consumes a "needs" list from a job definition.
func consumeNeedsList(props map[string]any, sourceFile, stageName, jobID string) ([]string, error) {
	rawNeeds, ok := props["needs"]
	if !ok {
		return nil, nil
	}
	delete(props, "needs")
	return extractNeedsList(rawNeeds, sourceFile, stageName, jobID)
}

// extractNeedsList extracts a list of needs from a YAML value.
func extractNeedsList(v any, sourceFile, stageName, jobID string) ([]string, error) {
	switch val := v.(type) {
	case []any:
		result := make([]string, 0, len(val))
		for i, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
				continue
			}
			return nil, fmt.Errorf("%s, stage %q, job %q: invalid needs entry at index %d (must be a string)", sourceFile, stageName, jobID, i)
		}
		return result, nil
	case []string:
		return val, nil
	default:
		return nil, fmt.Errorf("%s, stage %q, job %q: invalid needs format (must be a list of job IDs)", sourceFile, stageName, jobID)
	}
}

func resolveLocalNeeds(localNeeds []string, stageIndex map[string]string, sourceFile, stageName, jobID, scope string) ([]string, error) {
	if len(localNeeds) == 0 {
		return nil, nil
	}

	resolved := make([]string, 0, len(localNeeds))
	for _, need := range localNeeds {
		resolvedID, ok := stageIndex[need]
		if !ok {
			available := make([]string, 0, len(stageIndex))
			for id := range stageIndex {
				available = append(available, id)
			}
			sort.Strings(available)
			return nil, fmt.Errorf("%s, stage %q, job %q: invalid needs reference %q.\n       needs entries must reference non-prefixed job IDs in the same stage and %s.\n       Available job IDs: %v",
				sourceFile, stageName, jobID, need, scope, available)
		}
		resolved = append(resolved, resolvedID)
	}
	return resolved, nil
}

func mergeFileEnvIntoJob(jobDef map[string]any, fileEnv map[string]any) {
	if len(fileEnv) == 0 {
		return
	}

	jobEnvRaw, hasJobEnv := jobDef["env"]
	if !hasJobEnv {
		jobDef["env"] = deepCopyMap(fileEnv)
		return
	}

	jobEnv, ok := jobEnvRaw.(map[string]any)
	if !ok {
		return
	}

	jobDef["env"] = DeepMerge(fileEnv, jobEnv)
}

// applyProjectHooks applies project hook operations to the assembled jobs.
func applyProjectHooks(jobs []*config.AssembledJob, proj *config.Project) ([]*config.AssembledJob, error) {
	if proj.Hooks == nil {
		return jobs, nil
	}

	projectNeedsIndex := buildProjectNeedsIndex(proj)

	// Build job index for quick lookup.
	jobIndex := make(map[string]*config.AssembledJob, len(jobs))
	for _, j := range jobs {
		jobIndex[j.ID] = j
	}

	for stageName, stageJobs := range proj.Hooks {
		for jobID, pj := range stageJobs {
			stageNeedsIndex := projectNeedsIndex[stageName]

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
					mergeFileEnvIntoJob(props, proj.Env)
					if n, ok := props["name"]; ok {
						sourceName, _ = n.(string)
						delete(props, "name")
					}
					// Extract and resolve explicit needs.
					localNeeds, err := consumeNeedsList(props, "project.yml", stageName, jobID)
					if err != nil {
						return nil, err
					}
					explicitNeeds, err := resolveLocalNeeds(localNeeds, stageNeedsIndex, "project.yml", stageName, jobID, "project.yml")
					if err != nil {
						return nil, err
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
					mergeFileEnvIntoJob(overlay, proj.Env)
					// Extract and consume the "name" property from overlay.
					if n, ok := overlay["name"]; ok {
						sourceName, _ := n.(string)
						if sourceName != "" {
							j.SourceName = sourceName
						}
						delete(overlay, "name")
					}
					// Extract explicit needs from overlay and merge.
					localNeeds, err := consumeNeedsList(overlay, "project.yml", stageName, jobID)
					if err != nil {
						return nil, err
					}
					overlayNeeds, err := resolveLocalNeeds(localNeeds, stageNeedsIndex, "project.yml", stageName, jobID, "project.yml")
					if err != nil {
						return nil, err
					}
					j.ExplicitNeeds = mergeNeeds(j.ExplicitNeeds, overlayNeeds)
					j.Properties = DeepMerge(j.Properties, overlay)
				}
			} else {
				// New project job (no directive, not prefixed).
				props := deepCopyMap(pj.Properties)
				mergeFileEnvIntoJob(props, proj.Env)
				var sourceName string
				if n, ok := props["name"]; ok {
					sourceName, _ = n.(string)
					delete(props, "name")
				}
				localNeeds, err := consumeNeedsList(props, "project.yml", stageName, jobID)
				if err != nil {
					return nil, err
				}
				explicitNeeds, err := resolveLocalNeeds(localNeeds, stageNeedsIndex, "project.yml", stageName, jobID, "project.yml")
				if err != nil {
					return nil, err
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

func buildProjectNeedsIndex(proj *config.Project) map[string]map[string]string {
	index := make(map[string]map[string]string, len(proj.Hooks))
	for stageName, stageJobs := range proj.Hooks {
		stageIndex := make(map[string]string, len(stageJobs))
		for jobID, pj := range stageJobs {
			switch {
			case pj.IsDisable():
				// Disabled declarations do not produce output jobs.
				continue
			case pj.IsExtend():
				stageIndex[jobID] = fmt.Sprintf("%s--%s--%s", stageName, pj.Extend.ProvidedBy, jobID)
			case pj.IsReplace():
				stageIndex[jobID] = fmt.Sprintf("%s--%s--%s", stageName, pj.Replace.ProvidedBy, jobID)
			default:
				stageIndex[jobID] = jobID
			}
		}
		index[stageName] = stageIndex
	}
	return index
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
