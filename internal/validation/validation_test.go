package validation

import (
	"strings"
	"testing"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

// ---------------------------------------------------------------------------
// TestValidateConfiguration
// ---------------------------------------------------------------------------

func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.Configuration
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid minimal config",
			cfg: &config.Configuration{
				Version: "1",
				Stages:  []string{"build"},
			},
			wantErr: false,
		},
		{
			name: "valid multi-stage config",
			cfg: &config.Configuration{
				Version: "1",
				Stages:  []string{"build", "test", "deploy"},
			},
			wantErr: false,
		},
		{
			name: "valid stage name with underscore",
			cfg: &config.Configuration{
				Version: "1",
				Stages:  []string{"pre_build", "build", "post_deploy"},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			cfg: &config.Configuration{
				Version: "",
				Stages:  []string{"build"},
			},
			wantErr:   true,
			errSubstr: "missing required field 'version'",
		},
		{
			name: "unsupported version",
			cfg: &config.Configuration{
				Version: "2",
				Stages:  []string{"build"},
			},
			wantErr:   true,
			errSubstr: "unsupported version",
		},
		{
			name: "empty stages",
			cfg: &config.Configuration{
				Version: "1",
				Stages:  []string{},
			},
			wantErr:   true,
			errSubstr: "at least one stage",
		},
		{
			name: "invalid stage name uppercase",
			cfg: &config.Configuration{
				Version: "1",
				Stages:  []string{"Build"},
			},
			wantErr:   true,
			errSubstr: "invalid stage name",
		},
		{
			name: "invalid stage name starts with underscore",
			cfg: &config.Configuration{
				Version: "1",
				Stages:  []string{"_build"},
			},
			wantErr:   true,
			errSubstr: "invalid stage name",
		},
		{
			name: "duplicate stage name",
			cfg: &config.Configuration{
				Version: "1",
				Stages:  []string{"build", "build"},
			},
			wantErr:   true,
			errSubstr: "duplicate stage name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfiguration(tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tc.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestValidatePackage
// ---------------------------------------------------------------------------

func TestValidatePackage(t *testing.T) {
	validCfg := &config.Configuration{
		Version: "1",
		Stages:  []string{"build", "test"},
	}

	validHooks := map[string]config.JobMap{
		"build": {
			"docker-php": {"runs-on": "ubuntu-latest"},
		},
	}

	tests := []struct {
		name      string
		pkg       *config.Package
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid package",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks:      validHooks,
			},
			wantErr: false,
		},
		{
			name: "valid package multiple stages",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks: map[string]config.JobMap{
					"build": {
						"docker-php": {"runs-on": "ubuntu-latest"},
					},
					"test": {
						"phpunit": {"runs-on": "ubuntu-latest"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing id",
			pkg: &config.Package{
				ID:         "",
				SourceFile: "pkg_noid.yml",
				Hooks:      validHooks,
			},
			wantErr:   true,
			errSubstr: "missing required \"id\"",
		},
		{
			name: "invalid id uppercase",
			pkg: &config.Package{
				ID:         "Drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks:      validHooks,
			},
			wantErr:   true,
			errSubstr: "invalid package id",
		},
		{
			name: "invalid id starts with hyphen",
			pkg: &config.Package{
				ID:         "-drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks:      validHooks,
			},
			wantErr:   true,
			errSubstr: "invalid package id",
		},
		{
			name: "id contains double dash",
			pkg: &config.Package{
				ID:         "my--pkg",
				SourceFile: "pkg_bad.yml",
				Hooks:      validHooks,
			},
			wantErr:   true,
			errSubstr: "must not contain \"--\"",
		},
		{
			name: "empty hooks",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks:      map[string]config.JobMap{},
			},
			wantErr:   true,
			errSubstr: "'hooks' is required",
		},
		{
			name: "nil hooks",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks:      nil,
			},
			wantErr:   true,
			errSubstr: "'hooks' is required",
		},
		{
			name: "unknown stage reference",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks: map[string]config.JobMap{
					"deploy": {
						"push": {"runs-on": "ubuntu-latest"},
					},
				},
			},
			wantErr:   true,
			errSubstr: "references unknown stage",
		},
		{
			name: "empty stage zero jobs",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks: map[string]config.JobMap{
					"build": {},
				},
			},
			wantErr:   true,
			errSubstr: "must contain at least one job",
		},
		{
			name: "invalid job id uppercase",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks: map[string]config.JobMap{
					"build": {
						"Docker-PHP": {"runs-on": "ubuntu-latest"},
					},
				},
			},
			wantErr:   true,
			errSubstr: "invalid job id",
		},
		{
			name: "job id contains double dash",
			pkg: &config.Package{
				ID:         "drupal",
				SourceFile: "pkg_drupal.yml",
				Hooks: map[string]config.JobMap{
					"build": {
						"docker--php": {"runs-on": "ubuntu-latest"},
					},
				},
			},
			wantErr:   true,
			errSubstr: "must not contain \"--\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePackage(tc.pkg, validCfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tc.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestValidatePackageUniqueness — 3 cases
// ---------------------------------------------------------------------------

func TestValidatePackageUniqueness(t *testing.T) {
	tests := []struct {
		name      string
		pkgs      []*config.Package
		wantErr   bool
		errSubstr string
	}{
		{
			name: "unique ids",
			pkgs: []*config.Package{
				{ID: "drupal", SourceFile: "pkg_drupal.yml"},
				{ID: "redis", SourceFile: "pkg_redis.yml"},
			},
			wantErr: false,
		},
		{
			name: "single package",
			pkgs: []*config.Package{
				{ID: "drupal", SourceFile: "pkg_drupal.yml"},
			},
			wantErr: false,
		},
		{
			name: "duplicate ids",
			pkgs: []*config.Package{
				{ID: "drupal", SourceFile: "pkg_drupal.yml"},
				{ID: "drupal", SourceFile: "pkg_drupal2.yml"},
			},
			wantErr:   true,
			errSubstr: "duplicate package id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePackageUniqueness(tc.pkgs)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tc.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestValidateProject — 12 cases
// ---------------------------------------------------------------------------

func TestValidateProject(t *testing.T) {
	validCfg := &config.Configuration{
		Version: "1",
		Stages:  []string{"build", "test"},
	}

	// A package with one job in "build" for directive target resolution.
	validPkgs := []*config.Package{
		{
			ID:         "drupal",
			SourceFile: "pkg_drupal.yml",
			Hooks: map[string]config.JobMap{
				"build": {
					"docker-php": {"runs-on": "ubuntu-latest"},
				},
			},
		},
	}

	tests := []struct {
		name      string
		proj      *config.Project
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "nil hooks valid",
			proj:    &config.Project{Hooks: nil},
			wantErr: false,
		},
		{
			name: "valid extend",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"docker-php": {
							Extend:     &config.ProvidedByRef{ProvidedBy: "drupal"},
							Properties: map[string]any{"timeout-minutes": 30},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid replace",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"docker-php": {
							Replace:    &config.ProvidedByRef{ProvidedBy: "drupal"},
							Properties: map[string]any{"runs-on": "self-hosted"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid disable",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"docker-php": {
							Disable:    &config.ProvidedByRef{ProvidedBy: "drupal"},
							Properties: map[string]any{},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid new job",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"custom-lint": {
							Properties: map[string]any{"runs-on": "ubuntu-latest"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "unknown stage",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"deploy": {
						"push": {
							Properties: map[string]any{"runs-on": "ubuntu-latest"},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "references unknown stage",
		},
		{
			name: "invalid job id in project",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"Docker-PHP": {
							Properties: map[string]any{"runs-on": "ubuntu-latest"},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "invalid job id",
		},
		{
			name: "job id contains double dash in project",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"docker--php": {
							Properties: map[string]any{"runs-on": "ubuntu-latest"},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "must not contain \"--\"",
		},
		{
			name: "extend targets nonexistent job",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"nonexistent": {
							Extend:     &config.ProvidedByRef{ProvidedBy: "drupal"},
							Properties: map[string]any{},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "declares extend for",
		},
		{
			name: "extend targets nonexistent package",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"docker-php": {
							Extend:     &config.ProvidedByRef{ProvidedBy: "redis"},
							Properties: map[string]any{},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "declares extend for",
		},
		{
			name: "replace targets nonexistent job",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"nonexistent": {
							Replace:    &config.ProvidedByRef{ProvidedBy: "drupal"},
							Properties: map[string]any{"runs-on": "self-hosted"},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "declares replace for",
		},
		{
			name: "disable targets nonexistent job",
			proj: &config.Project{
				Hooks: map[string]map[string]config.ProjectJob{
					"build": {
						"nonexistent": {
							Disable:    &config.ProvidedByRef{ProvidedBy: "drupal"},
							Properties: map[string]any{},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "declares disable for",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProject(tc.proj, validCfg, validPkgs)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tc.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
