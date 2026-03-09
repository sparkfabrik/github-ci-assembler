package assembly

import (
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
