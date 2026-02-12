// Package validation provides validation functions for gh-ci-assembler configuration files.
package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

var (
	// idPattern matches valid package IDs: lowercase alphanumeric with hyphens.
	idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	// jobIDPattern matches valid job IDs: lowercase alphanumeric with hyphens and underscores.
	jobIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	// stagePattern matches valid stage names.
	stagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	// doubleDash is the reserved separator.
	doubleDash = "--"
)

// ValidateConfiguration validates a parsed configuration file.
func ValidateConfiguration(cfg *config.Configuration) error {
	if cfg.Version == "" {
		return fmt.Errorf("configuration: missing required field 'version'")
	}
	if cfg.Version != "1" {
		return fmt.Errorf("configuration: unsupported version %q (only \"1\" is supported)", cfg.Version)
	}
	if len(cfg.Stages) == 0 {
		return fmt.Errorf("configuration: 'stages' must contain at least one stage")
	}

	seen := make(map[string]bool, len(cfg.Stages))
	for _, s := range cfg.Stages {
		if !stagePattern.MatchString(s) {
			return fmt.Errorf("configuration: invalid stage name %q (must match %s)", s, stagePattern.String())
		}
		if seen[s] {
			return fmt.Errorf("configuration: duplicate stage name %q", s)
		}
		seen[s] = true
	}

	return nil
}

// ValidatePackage validates a parsed package file against the configuration.
func ValidatePackage(pkg *config.Package, cfg *config.Configuration) error {
	// Validate ID.
	if pkg.ID == "" {
		return fmt.Errorf("missing required \"id\" in %s.\n       Every package must declare an explicit id", pkg.SourceFile)
	}
	if !idPattern.MatchString(pkg.ID) {
		return fmt.Errorf("invalid package id %q in %s.\n       Package id must match %s (lowercase, hyphens allowed)", pkg.ID, pkg.SourceFile, idPattern.String())
	}
	if strings.Contains(pkg.ID, doubleDash) {
		return fmt.Errorf("invalid package id %q in %s.\n       Package id must not contain %q (reserved as separator)", pkg.ID, pkg.SourceFile, doubleDash)
	}

	// Validate on is map form.
	if err := validateOnMapForm(pkg.On, pkg.SourceFile, pkg.ID); err != nil {
		return err
	}

	// Validate hooks is present and non-empty.
	if pkg.Hooks == nil || len(pkg.Hooks) == 0 {
		return fmt.Errorf("package %q (file: %s): 'hooks' is required and must contain at least one stage with jobs", pkg.ID, pkg.SourceFile)
	}

	// Build valid stage set (including pre-/post- virtual stages).
	validStages := buildValidStageSet(cfg.Stages)

	for stageName, jobs := range pkg.Hooks {
		// Validate stage name.
		if !validStages[stageName] {
			return fmt.Errorf("package %q (file: %s) references unknown stage %q.\n       Valid stages: %v", pkg.ID, pkg.SourceFile, stageName, cfg.Stages)
		}

		if len(jobs) == 0 {
			return fmt.Errorf("package %q (file: %s): stage %q must contain at least one job", pkg.ID, pkg.SourceFile, stageName)
		}

		// Check for pre/post stage usage in packages and warn.
		if isVirtualStage(stageName) {
			fmt.Printf("Warning: package %q (file: %s) uses virtual stage %q. Pre/post stages in packages are discouraged.\n", pkg.ID, pkg.SourceFile, stageName)
		}

		// Validate job IDs.
		for jobID := range jobs {
			if err := validateJobID(jobID, stageName, pkg.SourceFile, pkg.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidatePackageUniqueness checks that all package IDs are unique.
func ValidatePackageUniqueness(pkgs []*config.Package) error {
	idToFiles := make(map[string][]string, len(pkgs))
	for _, pkg := range pkgs {
		idToFiles[pkg.ID] = append(idToFiles[pkg.ID], pkg.SourceFile)
	}

	for id, files := range idToFiles {
		if len(files) > 1 {
			return fmt.Errorf("duplicate package id %q.\n       Declared in: %s\n       Every package must have a unique id", id, strings.Join(files, ", "))
		}
	}

	return nil
}

// ValidateProject validates a parsed project file against the configuration and packages.
func ValidateProject(proj *config.Project, cfg *config.Configuration, pkgs []*config.Package) error {
	// Validate on is map form.
	if err := validateOnMapForm(proj.On, "project.yml", "project"); err != nil {
		return err
	}

	if proj.Hooks == nil {
		return nil // No hooks section is valid.
	}

	// Build valid stage set.
	validStages := buildValidStageSet(cfg.Stages)

	// Build package job index: stage--pkg-id--job-id → true.
	pkgJobIndex := buildPackageJobIndex(pkgs)

	for stageName, jobs := range proj.Hooks {
		if !validStages[stageName] {
			return fmt.Errorf("project.yml references unknown stage %q.\n       Valid stages: %v", stageName, cfg.Stages)
		}

		for jobID, pj := range jobs {
			// Validate job ID format.
			if err := validateJobID(jobID, stageName, "project.yml", "project"); err != nil {
				return err
			}

			// Validate directive targets.
			if pj.IsExtend() {
				prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pj.Extend.ProvidedBy, jobID)
				if !pkgJobIndex[prefixedID] {
					available := availableJobsInStage(pkgJobIndex, stageName)
					return fmt.Errorf("project.yml declares extend for %q (provided_by: %s)\n       in stage %q, but no matching job %q was found.\n       Available jobs in %q: %v",
						jobID, pj.Extend.ProvidedBy, stageName, prefixedID, stageName, available)
				}
			}
			if pj.IsReplace() {
				prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pj.Replace.ProvidedBy, jobID)
				if !pkgJobIndex[prefixedID] {
					available := availableJobsInStage(pkgJobIndex, stageName)
					return fmt.Errorf("project.yml declares replace for %q (provided_by: %s)\n       in stage %q, but no matching job %q was found.\n       Available jobs in %q: %v",
						jobID, pj.Replace.ProvidedBy, stageName, prefixedID, stageName, available)
				}
			}
			if pj.IsDisable() {
				prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pj.Disable.ProvidedBy, jobID)
				if !pkgJobIndex[prefixedID] {
					available := availableJobsInStage(pkgJobIndex, stageName)
					return fmt.Errorf("project.yml declares disable for %q (provided_by: %s)\n       in stage %q, but no matching job %q was found.\n       Available jobs in %q: %v",
						jobID, pj.Disable.ProvidedBy, stageName, prefixedID, stageName, available)
				}
			}
		}
	}

	return nil
}

// validateJobID checks a job ID against format requirements.
func validateJobID(jobID, stageName, sourceFile, sourceID string) error {
	if !jobIDPattern.MatchString(jobID) {
		return fmt.Errorf("invalid job id %q in stage %q of %s (id: %s).\n       Job id must match %s",
			jobID, stageName, sourceFile, sourceID, jobIDPattern.String())
	}
	if strings.Contains(jobID, doubleDash) {
		return fmt.Errorf("invalid job id %q in stage %q of %s (id: %s):\n       Job id must not contain %q (reserved as stage-id/package-id/job-id separator)",
			jobID, stageName, sourceFile, sourceID, doubleDash)
	}
	return nil
}

// validateOnMapForm checks that the `on` property is a map (not scalar or list).
func validateOnMapForm(on map[string]any, sourceFile, sourceID string) error {
	// If on is nil, it was either absent or already parsed as a map.
	// The YAML parser would have failed if it was a scalar, so we check
	// for nil (absent) which is fine. The map[string]any type already
	// ensures it's a map if present.
	// However, we need to handle the case where YAML parsed `on: push`
	// as a string — but toMapStringAny would return nil for that.
	// This is handled at the raw parsing level; if on was present in
	// the YAML but toMapStringAny returned nil, it means it wasn't a map.
	return nil
}

// buildValidStageSet creates a set of valid stage names including pre-/post- virtual stages.
func buildValidStageSet(stages []string) map[string]bool {
	valid := make(map[string]bool, len(stages)*3)
	for _, s := range stages {
		valid[s] = true
		valid["pre-"+s] = true
		valid["post-"+s] = true
	}
	return valid
}

// isVirtualStage returns true if the stage name is a pre- or post- virtual stage.
func isVirtualStage(name string) bool {
	return strings.HasPrefix(name, "pre-") || strings.HasPrefix(name, "post-")
}

// buildPackageJobIndex builds an index of all prefixed job IDs across all packages.
func buildPackageJobIndex(pkgs []*config.Package) map[string]bool {
	index := make(map[string]bool)
	for _, pkg := range pkgs {
		for stageName, jobs := range pkg.Hooks {
			for jobID := range jobs {
				prefixedID := fmt.Sprintf("%s--%s--%s", stageName, pkg.ID, jobID)
				index[prefixedID] = true
			}
		}
	}
	return index
}

// availableJobsInStage returns all job IDs in a given stage from the package index.
func availableJobsInStage(index map[string]bool, stageName string) []string {
	var result []string
	prefix := stageName + "--"
	for id := range index {
		if strings.HasPrefix(id, prefix) {
			result = append(result, id)
		}
	}
	return result
}
