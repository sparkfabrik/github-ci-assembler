package assembly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

func TestAssemblePackageJobs_FileEnvAndExplicitNeeds(t *testing.T) {
	pkg := &config.Package{
		ID:         "drupal",
		SourceFile: "pkg_drupal.yml",
		Env: map[string]any{
			"BASE":   "1",
			"SHARED": "from-file",
		},
		Hooks: map[string]config.JobMap{
			"test": {
				"behat": {
					"runs-on": "ubuntu-latest",
				},
				"phpunit": {
					"runs-on": "ubuntu-latest",
					"env": map[string]any{
						"SHARED": "from-job",
						"JOB":    "1",
					},
					"needs": []any{"test--drupal--behat"},
				},
			},
		},
	}

	jobs, err := assemblePackageJobs(pkg)
	if err != nil {
		t.Fatalf("assemblePackageJobs() error: %v", err)
	}

	phpunit := findJobByID(t, jobs, "test--drupal--phpunit")
	if len(phpunit.ExplicitNeeds) != 1 || phpunit.ExplicitNeeds[0] != "test--drupal--behat" {
		t.Fatalf("unexpected explicit needs: %v", phpunit.ExplicitNeeds)
	}

	phpunitEnv, ok := phpunit.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("phpunit env is not a map: %T", phpunit.Properties["env"])
	}
	if phpunitEnv["BASE"] != "1" || phpunitEnv["SHARED"] != "from-job" || phpunitEnv["JOB"] != "1" {
		t.Fatalf("unexpected phpunit env: %v", phpunitEnv)
	}

	behat := findJobByID(t, jobs, "test--drupal--behat")
	behatEnv, ok := behat.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("behat env is not a map: %T", behat.Properties["env"])
	}
	if behatEnv["BASE"] != "1" || behatEnv["SHARED"] != "from-file" {
		t.Fatalf("unexpected behat env: %v", behatEnv)
	}
}

func TestAssemblePackageJobs_PreservesUnknownNeedsForPhaseValidation(t *testing.T) {
	pkg := &config.Package{
		ID:         "drupal",
		SourceFile: "pkg_drupal.yml",
		Hooks: map[string]config.JobMap{
			"test": {
				"phpunit": {
					"runs-on": "ubuntu-latest",
					"needs":   []any{"unknown-job"},
				},
			},
		},
	}

	jobs, err := assemblePackageJobs(pkg)
	if err != nil {
		t.Fatalf("assemblePackageJobs() error: %v", err)
	}
	phpunit := findJobByID(t, jobs, "test--drupal--phpunit")
	if len(phpunit.ExplicitNeeds) != 1 || phpunit.ExplicitNeeds[0] != "unknown-job" {
		t.Fatalf("unexpected explicit needs: %v", phpunit.ExplicitNeeds)
	}
}

func TestApplyProjectHooks_FileEnvAndExplicitNeeds(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			Properties: map[string]any{
				"env": map[string]any{"BASE": "1"},
			},
		},
	}

	proj := &config.Project{
		Env: map[string]any{
			"GLOBAL": "yes",
		},
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-php": {
					Extend: &config.ProvidedByRef{ProvidedBy: "drupal"},
					Properties: map[string]any{
						"needs": []any{"custom-lint"},
					},
				},
				"custom-lint": {
					Properties: map[string]any{
						"runs-on": "ubuntu-latest",
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	extended := findJobByID(t, updatedJobs, "build--drupal--docker-php")
	if len(extended.ExplicitNeeds) != 1 || extended.ExplicitNeeds[0] != "custom-lint" {
		t.Fatalf("unexpected extended explicit needs: %v", extended.ExplicitNeeds)
	}

	extendedEnv, ok := extended.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("extended env is not a map: %T", extended.Properties["env"])
	}
	if extendedEnv["BASE"] != "1" || extendedEnv["GLOBAL"] != "yes" {
		t.Fatalf("unexpected extended env: %v", extendedEnv)
	}

	customLint := findJobByID(t, updatedJobs, "custom-lint")
	customLintEnv, ok := customLint.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("custom-lint env is not a map: %T", customLint.Properties["env"])
	}
	if customLintEnv["GLOBAL"] != "yes" {
		t.Fatalf("unexpected custom-lint env: %v", customLintEnv)
	}
}

func TestApplyProjectHooks_PreservesUnknownNeedsForPhaseValidation(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			Properties:    map[string]any{"runs-on": "ubuntu-latest"},
		},
	}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-php": {
					Extend: &config.ProvidedByRef{ProvidedBy: "drupal"},
					Properties: map[string]any{
						"needs": []any{"docker-nginx"},
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	extended := findJobByID(t, updatedJobs, "build--drupal--docker-php")
	if len(extended.ExplicitNeeds) != 1 || extended.ExplicitNeeds[0] != "docker-nginx" {
		t.Fatalf("unexpected explicit needs: %v", extended.ExplicitNeeds)
	}
}

func TestValidateExplicitNeeds_InvalidReference(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--drupal--docker-php", Stage: "build"},
		{ID: "custom-lint", Stage: "build"},
		{ID: "test--drupal--phpunit", Stage: "test", ExplicitNeeds: []string{"docker-nginx"}},
	}

	err := validateExplicitNeeds(jobs)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid needs reference \"docker-nginx\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflowPropsFromConfiguration_IncludesRootEnv(t *testing.T) {
	cfg := &config.Configuration{
		Name:     "CI",
		On:       map[string]any{"push": map[string]any{}},
		Defaults: map[string]any{"run": map[string]any{"shell": "bash"}},
		Env: map[string]any{
			"GLOBAL_FLAG": "1",
		},
		Permissions: map[string]any{"contents": "read"},
	}

	props := workflowPropsFromConfiguration(cfg)

	if props.Env == nil {
		t.Fatal("expected workflow env to be set, got nil")
	}
	if props.Env["GLOBAL_FLAG"] != "1" {
		t.Fatalf("unexpected GLOBAL_FLAG value: %v", props.Env["GLOBAL_FLAG"])
	}
}

// --- Additional assembly gap tests (5 cases) ---

func TestApplyProjectHooks_Replace(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			SourceName:    "Build PHP container",
			Properties: map[string]any{
				"runs-on":         "ubuntu-latest",
				"timeout-minutes": 30,
			},
		},
	}

	proj := &config.Project{
		Env: map[string]any{"REPLACED_ENV": "yes"},
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-php": {
					Replace: &config.ProvidedByRef{ProvidedBy: "drupal"},
					Properties: map[string]any{
						"name":    "Replaced PHP build",
						"runs-on": "self-hosted",
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	replaced := findJobByID(t, updatedJobs, "build--drupal--docker-php")

	// Properties should be fully replaced (not merged).
	if replaced.Properties["runs-on"] != "self-hosted" {
		t.Errorf("runs-on = %v, want %q", replaced.Properties["runs-on"], "self-hosted")
	}
	if _, ok := replaced.Properties["timeout-minutes"]; ok {
		t.Error("expected timeout-minutes to be absent after replace, but it is present")
	}

	// SourceName should be updated from replacement.
	if replaced.SourceName != "Replaced PHP build" {
		t.Errorf("SourceName = %q, want %q", replaced.SourceName, "Replaced PHP build")
	}

	// name should be consumed (removed from Properties).
	if _, ok := replaced.Properties["name"]; ok {
		t.Error("expected 'name' to be consumed from Properties, but it is present")
	}

	// Project-level env should be merged into replaced job.
	replacedEnv, ok := replaced.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("replaced env is not a map: %T", replaced.Properties["env"])
	}
	if replacedEnv["REPLACED_ENV"] != "yes" {
		t.Errorf("unexpected replaced env: %v", replacedEnv)
	}
}

func TestApplyProjectHooks_NilHooks(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:         "build--drupal--docker-php",
			Stage:      "build",
			Properties: map[string]any{"runs-on": "ubuntu-latest"},
		},
	}

	proj := &config.Project{
		Hooks: nil,
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	if len(updatedJobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(updatedJobs))
	}
	if updatedJobs[0].ID != "build--drupal--docker-php" {
		t.Errorf("unexpected job ID: %v", updatedJobs[0].ID)
	}
	if updatedJobs[0].Properties["runs-on"] != "ubuntu-latest" {
		t.Errorf("unexpected runs-on: %v", updatedJobs[0].Properties["runs-on"])
	}
}

func TestExtractNeedsList_NonStringEntry(t *testing.T) {
	_, err := extractNeedsList([]any{"valid", 42}, "test.yml", "build", "j1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid needs entry") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractNeedsList_NonList(t *testing.T) {
	_, err := extractNeedsList("not-a-list", "test.yml", "build", "j1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid needs format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeNeeds_Deduplication(t *testing.T) {
	automatic := []string{"a", "b"}
	explicit := []string{"b", "c"}

	result := mergeNeeds(automatic, explicit)

	// Explicit first, then automatic, duplicates removed.
	expected := []string{"b", "c", "a"}
	if len(result) != len(expected) {
		t.Fatalf("len(result) = %d, want %d: %v", len(result), len(expected), result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, want %q (full: %v)", i, result[i], v, result)
		}
	}
}

func TestValidateExplicitNeeds_ReferencesDisabledJob(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--drupal--docker-php", Stage: "build"},
		{ID: "build--drupal--docker-nginx", Stage: "build", Disabled: true},
		{ID: "test--drupal--phpunit", Stage: "test", ExplicitNeeds: []string{"build--drupal--docker-nginx"}},
	}

	err := validateExplicitNeeds(jobs)
	if err == nil {
		t.Fatal("expected validation error when referencing disabled job, got nil")
	}
	if !strings.Contains(err.Error(), "invalid needs reference") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "build--drupal--docker-nginx") {
		t.Fatalf("error should mention the disabled job ID: %v", err)
	}
}

func TestApplyProjectHooks_ExtendUpdatesSourceName(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			SourceName:    "Original PHP build",
			Properties: map[string]any{
				"runs-on":         "ubuntu-latest",
				"timeout-minutes": 30,
			},
		},
	}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-php": {
					Extend: &config.ProvidedByRef{ProvidedBy: "drupal"},
					Properties: map[string]any{
						"name":            "Updated PHP build",
						"timeout-minutes": 60,
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	extended := findJobByID(t, updatedJobs, "build--drupal--docker-php")

	// SourceName should be updated from extend overlay.
	if extended.SourceName != "Updated PHP build" {
		t.Errorf("SourceName = %q, want %q", extended.SourceName, "Updated PHP build")
	}

	// "name" should be consumed (removed from Properties).
	if _, ok := extended.Properties["name"]; ok {
		t.Error("expected 'name' to be consumed from Properties, but it is present")
	}

	// Other properties should be merged (not replaced).
	if extended.Properties["runs-on"] != "ubuntu-latest" {
		t.Errorf("runs-on should be preserved from original, got %v", extended.Properties["runs-on"])
	}
	if extended.Properties["timeout-minutes"] != 60 {
		t.Errorf("timeout-minutes should be updated to 60, got %v", extended.Properties["timeout-minutes"])
	}
}

func TestApplyProjectHooks_NewJobEnvWinsOverProjectFileEnv(t *testing.T) {
	jobs := []*config.AssembledJob{}

	proj := &config.Project{
		Env: map[string]any{
			"SHARED":    "from-project-file",
			"FILE_ONLY": "yes",
		},
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"custom-lint": {
					Properties: map[string]any{
						"runs-on": "ubuntu-latest",
						"env": map[string]any{
							"SHARED":   "from-job",
							"JOB_ONLY": "yes",
						},
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	lint := findJobByID(t, updatedJobs, "custom-lint")
	lintEnv, ok := lint.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("custom-lint env is not a map: %T", lint.Properties["env"])
	}

	// Job-level env wins on conflict.
	if lintEnv["SHARED"] != "from-job" {
		t.Errorf("SHARED = %v, want %q (job env should win)", lintEnv["SHARED"], "from-job")
	}
	// File-level env is merged in.
	if lintEnv["FILE_ONLY"] != "yes" {
		t.Errorf("FILE_ONLY = %v, want %q (project file env should be merged)", lintEnv["FILE_ONLY"], "yes")
	}
	// Job-level env is preserved.
	if lintEnv["JOB_ONLY"] != "yes" {
		t.Errorf("JOB_ONLY = %v, want %q", lintEnv["JOB_ONLY"], "yes")
	}
}

func TestApplyProjectHooks_ReplaceWithoutNamePreservesOriginal(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			SourceName:    "Original PHP build",
			Properties: map[string]any{
				"runs-on": "ubuntu-latest",
			},
		},
	}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-php": {
					Replace: &config.ProvidedByRef{ProvidedBy: "drupal"},
					Properties: map[string]any{
						"runs-on": "self-hosted",
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	replaced := findJobByID(t, updatedJobs, "build--drupal--docker-php")

	// When replacement has no "name", the original SourceName should survive.
	if replaced.SourceName != "Original PHP build" {
		t.Errorf("SourceName = %q, want %q (original should be preserved when replacement has no name)",
			replaced.SourceName, "Original PHP build")
	}

	// Properties should be fully replaced.
	if replaced.Properties["runs-on"] != "self-hosted" {
		t.Errorf("runs-on = %v, want %q", replaced.Properties["runs-on"], "self-hosted")
	}
}

func TestApplyProjectHooks_NewJobWithExplicitNeeds(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			Properties:    map[string]any{"runs-on": "ubuntu-latest"},
		},
	}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"custom-lint": {
					Properties: map[string]any{
						"runs-on": "ubuntu-latest",
						"needs":   []any{"build--drupal--docker-php"},
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	lint := findJobByID(t, updatedJobs, "custom-lint")
	if len(lint.ExplicitNeeds) != 1 || lint.ExplicitNeeds[0] != "build--drupal--docker-php" {
		t.Fatalf("unexpected explicit needs for new job: %v", lint.ExplicitNeeds)
	}

	// "needs" should be consumed from Properties.
	if _, ok := lint.Properties["needs"]; ok {
		t.Error("expected 'needs' to be consumed from Properties, but it is present")
	}
}

func TestAssemblePackageJobs_EnvIsolationBetweenPackages(t *testing.T) {
	pkgA := &config.Package{
		ID:         "drupal",
		SourceFile: "pkg_drupal.yml",
		Env: map[string]any{
			"DRUPAL_ENV": "yes",
		},
		Hooks: map[string]config.JobMap{
			"build": {
				"docker-php": {"runs-on": "ubuntu-latest"},
			},
		},
	}

	pkgB := &config.Package{
		ID:         "redis",
		SourceFile: "pkg_redis.yml",
		Env: map[string]any{
			"REDIS_ENV": "yes",
		},
		Hooks: map[string]config.JobMap{
			"build": {
				"docker-redis": {"runs-on": "ubuntu-latest"},
			},
		},
	}

	jobsA, err := assemblePackageJobs(pkgA)
	if err != nil {
		t.Fatalf("assemblePackageJobs(pkgA) error: %v", err)
	}
	jobsB, err := assemblePackageJobs(pkgB)
	if err != nil {
		t.Fatalf("assemblePackageJobs(pkgB) error: %v", err)
	}

	// Drupal job should have DRUPAL_ENV but NOT REDIS_ENV.
	drupalJob := findJobByID(t, jobsA, "build--drupal--docker-php")
	drupalEnv, ok := drupalJob.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("drupal job env is not a map: %T", drupalJob.Properties["env"])
	}
	if drupalEnv["DRUPAL_ENV"] != "yes" {
		t.Errorf("DRUPAL_ENV missing from drupal job env")
	}
	if _, leaked := drupalEnv["REDIS_ENV"]; leaked {
		t.Error("REDIS_ENV should NOT be present in drupal job (env leaked between packages)")
	}

	// Redis job should have REDIS_ENV but NOT DRUPAL_ENV.
	redisJob := findJobByID(t, jobsB, "build--redis--docker-redis")
	redisEnv, ok := redisJob.Properties["env"].(map[string]any)
	if !ok {
		t.Fatalf("redis job env is not a map: %T", redisJob.Properties["env"])
	}
	if redisEnv["REDIS_ENV"] != "yes" {
		t.Errorf("REDIS_ENV missing from redis job env")
	}
	if _, leaked := redisEnv["DRUPAL_ENV"]; leaked {
		t.Error("DRUPAL_ENV should NOT be present in redis job (env leaked between packages)")
	}
}

func TestApplyProjectHooks_Disable(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			SourceName:    "Build PHP image",
			Properties: map[string]any{
				"runs-on": "ubuntu-latest",
				"steps":   []any{map[string]any{"run": "echo build"}},
			},
		},
		{
			ID:            "build--drupal--docker-nginx",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-nginx",
			Properties: map[string]any{
				"runs-on": "ubuntu-latest",
			},
		},
	}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-php": {
					Disable:    &config.ProvidedByRef{ProvidedBy: "drupal"},
					Properties: map[string]any{},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	disabled := findJobByID(t, updatedJobs, "build--drupal--docker-php")

	// Disabled flag must be set.
	if !disabled.Disabled {
		t.Error("expected Disabled=true, got false")
	}

	// DisabledComment must be set.
	if disabled.DisabledComment == "" {
		t.Error("expected DisabledComment to be set, got empty string")
	}
	if !strings.Contains(disabled.DisabledComment, "DISABLED") {
		t.Errorf("DisabledComment should contain 'DISABLED', got %q", disabled.DisabledComment)
	}

	// Original properties should be untouched.
	if disabled.Properties["runs-on"] != "ubuntu-latest" {
		t.Errorf("runs-on should be preserved after disable, got %v", disabled.Properties["runs-on"])
	}
	if disabled.SourceName != "Build PHP image" {
		t.Errorf("SourceName should be preserved after disable, got %q", disabled.SourceName)
	}

	// The other job should not be affected.
	nginx := findJobByID(t, updatedJobs, "build--drupal--docker-nginx")
	if nginx.Disabled {
		t.Error("docker-nginx should NOT be disabled")
	}
}

func TestApplyProjectHooks_ReplaceWithExplicitNeeds(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			Properties:    map[string]any{"runs-on": "ubuntu-latest"},
		},
		{
			ID:            "build--redis--docker-redis",
			Stage:         "build",
			PackageID:     "redis",
			OriginalJobID: "docker-redis",
			Properties:    map[string]any{"runs-on": "ubuntu-latest"},
		},
	}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-redis": {
					Replace: &config.ProvidedByRef{ProvidedBy: "redis"},
					Properties: map[string]any{
						"runs-on": "self-hosted",
						"needs":   []any{"build--drupal--docker-php"},
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	replaced := findJobByID(t, updatedJobs, "build--redis--docker-redis")

	// Explicit needs should be extracted from replacement.
	if len(replaced.ExplicitNeeds) != 1 || replaced.ExplicitNeeds[0] != "build--drupal--docker-php" {
		t.Fatalf("unexpected explicit needs for replaced job: %v", replaced.ExplicitNeeds)
	}

	// "needs" should be consumed from Properties.
	if _, ok := replaced.Properties["needs"]; ok {
		t.Error("expected 'needs' to be consumed from Properties, but it is present")
	}

	// Properties should be the replacement (not merged).
	if replaced.Properties["runs-on"] != "self-hosted" {
		t.Errorf("runs-on = %v, want %q", replaced.Properties["runs-on"], "self-hosted")
	}
}

func TestApplyProjectHooks_ExtendMergesExplicitNeedsFromBoth(t *testing.T) {
	jobs := []*config.AssembledJob{
		{
			ID:            "build--drupal--docker-php",
			Stage:         "build",
			PackageID:     "drupal",
			OriginalJobID: "docker-php",
			Properties:    map[string]any{"runs-on": "ubuntu-latest"},
			ExplicitNeeds: []string{"build--base--setup"},
		},
	}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"docker-php": {
					Extend: &config.ProvidedByRef{ProvidedBy: "drupal"},
					Properties: map[string]any{
						"needs": []any{"build--redis--docker-redis"},
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	extended := findJobByID(t, updatedJobs, "build--drupal--docker-php")

	// Both the original explicit needs and the overlay needs should be merged.
	// mergeNeeds(j.ExplicitNeeds, localNeeds) puts localNeeds (overlay) first,
	// then j.ExplicitNeeds (original) second, per the "explicit first" convention.
	if len(extended.ExplicitNeeds) != 2 {
		t.Fatalf("expected 2 explicit needs (overlay + original), got %d: %v",
			len(extended.ExplicitNeeds), extended.ExplicitNeeds)
	}
	// Overlay (localNeeds) comes first per mergeNeeds semantics.
	if extended.ExplicitNeeds[0] != "build--redis--docker-redis" {
		t.Errorf("first explicit need should be overlay 'build--redis--docker-redis', got %q", extended.ExplicitNeeds[0])
	}
	if extended.ExplicitNeeds[1] != "build--base--setup" {
		t.Errorf("second explicit need should be original 'build--base--setup', got %q", extended.ExplicitNeeds[1])
	}

	// "needs" should be consumed from Properties.
	if _, ok := extended.Properties["needs"]; ok {
		t.Error("expected 'needs' to be consumed from Properties, but it is present")
	}
}

func TestSortJobs_DeterministicOrder(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "test--b--j1", Stage: "test"},
		{ID: "build--c--z-job", Stage: "build"},
		{ID: "build--a--a-job", Stage: "build"},
		{ID: "build--b--m-job", Stage: "build"},
		{ID: "test--a--j2", Stage: "test"},
	}

	expanded := []ExpandedStage{
		{Name: "build", Kind: StageKindRegular, BaseName: "build"},
		{Name: "test", Kind: StageKindRegular, BaseName: "test"},
	}

	sortJobs(jobs, expanded)

	expected := []string{
		"build--a--a-job",
		"build--b--m-job",
		"build--c--z-job",
		"test--a--j2",
		"test--b--j1",
	}

	if len(jobs) != len(expected) {
		t.Fatalf("expected %d jobs, got %d", len(expected), len(jobs))
	}
	for i, want := range expected {
		if jobs[i].ID != want {
			t.Errorf("jobs[%d].ID = %q, want %q", i, jobs[i].ID, want)
		}
	}
}

// --- Final gap tests (gaps #1, #2, #3, #5, #7) ---

func TestMergeFileEnvIntoJob_NonMapJobEnv(t *testing.T) {
	// When the job's env is not a map[string]any, mergeFileEnvIntoJob
	// should silently return without modifying the job (assembly.go:207-209).
	jobDef := map[string]any{
		"runs-on": "ubuntu-latest",
		"env":     "not-a-map",
	}
	fileEnv := map[string]any{
		"FILE_VAR": "should-not-appear",
	}

	mergeFileEnvIntoJob(jobDef, fileEnv)

	// The non-map env value should be preserved unchanged.
	if jobDef["env"] != "not-a-map" {
		t.Errorf("env = %v, want %q (non-map env should be left untouched)", jobDef["env"], "not-a-map")
	}
}

func TestApplyProjectHooks_NewJobWithNameProperty(t *testing.T) {
	// A new project job with a "name" key should have it extracted into
	// SourceName and removed from Properties (assembly.go:282-286).
	jobs := []*config.AssembledJob{}

	proj := &config.Project{
		Hooks: map[string]map[string]config.ProjectJob{
			"build": {
				"custom-lint": {
					Properties: map[string]any{
						"name":    "Custom Lint Check",
						"runs-on": "ubuntu-latest",
					},
				},
			},
		},
	}

	updatedJobs, err := applyProjectHooks(jobs, proj)
	if err != nil {
		t.Fatalf("applyProjectHooks() error: %v", err)
	}

	lint := findJobByID(t, updatedJobs, "custom-lint")

	// SourceName should be extracted from the "name" property.
	if lint.SourceName != "Custom Lint Check" {
		t.Errorf("SourceName = %q, want %q", lint.SourceName, "Custom Lint Check")
	}

	// "name" should be consumed (removed from Properties).
	if _, ok := lint.Properties["name"]; ok {
		t.Error("expected 'name' to be consumed from Properties, but it is present")
	}

	// Other properties should remain.
	if lint.Properties["runs-on"] != "ubuntu-latest" {
		t.Errorf("runs-on = %v, want %q", lint.Properties["runs-on"], "ubuntu-latest")
	}
}

func TestAssemble_ErrorPhases(t *testing.T) {
	// Integration tests verifying the Assembler.Assemble() error-wrapping paths
	// (assembly.go:23-87). Each subtest triggers a different phase error.

	// Helper: write a temp file with the given content.
	writeTemp := func(t *testing.T, name, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
		return path
	}

	validConfig := `version: "1"
stages: [build, test]
name: "Test CI"
on:
  push: {}
`

	validPkg := `id: base
hooks:
  build:
    setup:
      runs-on: ubuntu-latest
`

	t.Run("phase1_load_configuration_error", func(t *testing.T) {
		a := &Assembler{
			ConfigPath: "/nonexistent/configuration.yml",
			PkgPaths:   []string{},
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 1 (load configuration)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase1_validate_configuration_error", func(t *testing.T) {
		// version: "99" triggers validation error.
		cfgPath := writeTemp(t, "configuration.yml", `version: "99"
stages: [build]
`)
		a := &Assembler{
			ConfigPath: cfgPath,
			PkgPaths:   []string{},
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 1 (validate configuration)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase2_load_packages_error", func(t *testing.T) {
		cfgPath := writeTemp(t, "configuration.yml", validConfig)
		a := &Assembler{
			ConfigPath: cfgPath,
			PkgPaths:   []string{"/nonexistent/pkg_missing.yml"},
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 2 (load packages)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase4_load_project_error", func(t *testing.T) {
		cfgPath := writeTemp(t, "configuration.yml", validConfig)
		pkgPath := writeTemp(t, "pkg_base.yml", validPkg)
		a := &Assembler{
			ConfigPath:  cfgPath,
			PkgPaths:    []string{pkgPath},
			ProjectPath: "/nonexistent/project.yml",
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 4 (load project)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase5_validate_explicit_needs_error", func(t *testing.T) {
		cfgPath := writeTemp(t, "configuration.yml", validConfig)
		// Package with an explicit needs reference to a non-existent job.
		pkgPath := writeTemp(t, "pkg_bad_needs.yml", `id: badneeds
hooks:
  build:
    setup:
      runs-on: ubuntu-latest
      needs: [nonexistent-job]
`)
		a := &Assembler{
			ConfigPath: cfgPath,
			PkgPaths:   []string{pkgPath},
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 5 (validate explicit needs)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase2_validate_package_uniqueness_error", func(t *testing.T) {
		cfgPath := writeTemp(t, "configuration.yml", validConfig)
		// Two packages with the same ID.
		pkg1Path := writeTemp(t, "pkg_dup1.yml", validPkg)
		pkg2Path := writeTemp(t, "pkg_dup2.yml", validPkg)
		a := &Assembler{
			ConfigPath: cfgPath,
			PkgPaths:   []string{pkg1Path, pkg2Path},
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 2 (validate packages)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase2_validate_package_error", func(t *testing.T) {
		cfgPath := writeTemp(t, "configuration.yml", validConfig)
		// Package with missing hooks.
		pkgPath := writeTemp(t, "pkg_no_hooks.yml", `id: nohooks
`)
		a := &Assembler{
			ConfigPath: cfgPath,
			PkgPaths:   []string{pkgPath},
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 2 (validate packages)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase4_validate_project_error", func(t *testing.T) {
		cfgPath := writeTemp(t, "configuration.yml", validConfig)
		pkgPath := writeTemp(t, "pkg_base.yml", validPkg)
		// Project referencing unknown stage.
		projPath := writeTemp(t, "project.yml", `hooks:
  nonexistent-stage:
    some-job:
      runs-on: ubuntu-latest
`)
		a := &Assembler{
			ConfigPath:  cfgPath,
			PkgPaths:    []string{pkgPath},
			ProjectPath: projPath,
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 4 (validate project)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("phase4_apply_project_hooks_error", func(t *testing.T) {
		cfgPath := writeTemp(t, "configuration.yml", validConfig)
		pkgPath := writeTemp(t, "pkg_base.yml", validPkg)
		// Project with invalid needs format (string instead of list) for a new job.
		projPath := writeTemp(t, "project.yml", `hooks:
  build:
    custom-job:
      runs-on: ubuntu-latest
      needs: "not-a-list"
`)
		a := &Assembler{
			ConfigPath:  cfgPath,
			PkgPaths:    []string{pkgPath},
			ProjectPath: projPath,
		}
		_, err := a.Assemble()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "phase 4 (apply project hooks)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestExtractNeedsList_StringSlice(t *testing.T) {
	// Exercise the []string branch in extractNeedsList (assembly.go:189-190).
	// YAML v3 never produces []string (it uses []any), but the branch exists
	// as a defensive guard and should be tested.
	result, err := extractNeedsList([]string{"job-a", "job-b"}, "test.yml", "build", "j1")
	if err != nil {
		t.Fatalf("extractNeedsList([]string) error: %v", err)
	}
	if len(result) != 2 || result[0] != "job-a" || result[1] != "job-b" {
		t.Fatalf("unexpected result: %v, want [job-a job-b]", result)
	}
}

func TestApplyProjectHooks_ConsumeNeedsListErrorPropagation(t *testing.T) {
	// When "needs" in a project job is invalid (not a list), consumeNeedsList
	// returns an error. This error should propagate from each directive branch
	// (extend, replace, new) in applyProjectHooks.

	baseJobs := func() []*config.AssembledJob {
		return []*config.AssembledJob{
			{
				ID:            "build--drupal--docker-php",
				Stage:         "build",
				PackageID:     "drupal",
				OriginalJobID: "docker-php",
				Properties:    map[string]any{"runs-on": "ubuntu-latest"},
			},
		}
	}

	t.Run("extend_invalid_needs", func(t *testing.T) {
		proj := &config.Project{
			Hooks: map[string]map[string]config.ProjectJob{
				"build": {
					"docker-php": {
						Extend: &config.ProvidedByRef{ProvidedBy: "drupal"},
						Properties: map[string]any{
							"needs": "not-a-list",
						},
					},
				},
			},
		}
		_, err := applyProjectHooks(baseJobs(), proj)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid needs format") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("replace_invalid_needs", func(t *testing.T) {
		proj := &config.Project{
			Hooks: map[string]map[string]config.ProjectJob{
				"build": {
					"docker-php": {
						Replace: &config.ProvidedByRef{ProvidedBy: "drupal"},
						Properties: map[string]any{
							"needs": "not-a-list",
						},
					},
				},
			},
		}
		_, err := applyProjectHooks(baseJobs(), proj)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid needs format") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("new_job_invalid_needs", func(t *testing.T) {
		proj := &config.Project{
			Hooks: map[string]map[string]config.ProjectJob{
				"build": {
					"custom-lint": {
						Properties: map[string]any{
							"runs-on": "ubuntu-latest",
							"needs":   "not-a-list",
						},
					},
				},
			},
		}
		_, err := applyProjectHooks([]*config.AssembledJob{}, proj)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid needs format") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestAssemblePackageJobs_ConsumeNeedsListError(t *testing.T) {
	// When a package job has an invalid needs value (not a list),
	// assemblePackageJobs should propagate the error from consumeNeedsList.
	pkg := &config.Package{
		ID:         "drupal",
		SourceFile: "pkg_drupal.yml",
		Hooks: map[string]config.JobMap{
			"build": {
				"docker-php": {
					"runs-on": "ubuntu-latest",
					"needs":   42, // Neither a list nor a string.
				},
			},
		},
	}

	_, err := assemblePackageJobs(pkg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid needs format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateExplicitNeeds_AllValid(t *testing.T) {
	// Verify that validateExplicitNeeds returns nil when all explicit needs
	// reference existing, non-disabled jobs.
	jobs := []*config.AssembledJob{
		{ID: "build--drupal--docker-php", Stage: "build"},
		{ID: "build--drupal--docker-nginx", Stage: "build"},
		{ID: "test--drupal--phpunit", Stage: "test", ExplicitNeeds: []string{"build--drupal--docker-php"}},
		{ID: "test--drupal--behat", Stage: "test", ExplicitNeeds: []string{"build--drupal--docker-php", "build--drupal--docker-nginx"}},
	}

	err := validateExplicitNeeds(jobs)
	if err != nil {
		t.Fatalf("validateExplicitNeeds() returned unexpected error: %v", err)
	}
}

func findJobByID(t *testing.T, jobs []*config.AssembledJob, id string) *config.AssembledJob {
	t.Helper()
	for _, j := range jobs {
		if j.ID == id {
			return j
		}
	}
	t.Fatalf("job %q not found", id)
	return nil
}
