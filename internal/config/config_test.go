package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// --- Existing tests (2 cases) ---

func TestLoadConfiguration_AllowsRootEnvMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
env:
  GLOBAL_FLAG: "1"
  DEPLOY_ENV: staging
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("LoadConfiguration() error: %v", err)
	}

	if cfg.Env == nil {
		t.Fatal("expected env to be loaded, got nil")
	}
	if cfg.Env["GLOBAL_FLAG"] != "1" {
		t.Fatalf("unexpected GLOBAL_FLAG value: %v", cfg.Env["GLOBAL_FLAG"])
	}
	if cfg.Env["DEPLOY_ENV"] != "staging" {
		t.Fatalf("unexpected DEPLOY_ENV value: %v", cfg.Env["DEPLOY_ENV"])
	}
}

func TestLoadConfiguration_RejectsInvalidRootEnvFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
env: not-a-map
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid \"env\" format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LoadConfiguration tests (7 cases) ---

func TestLoadConfiguration_HappyPath_AllFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build, test, deploy]
name: "My CI"
on:
  push:
    branches: [main]
  pull_request: {}
defaults:
  run:
    shell: bash
env:
  CI: "true"
permissions:
  contents: read
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("LoadConfiguration() error: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if !reflect.DeepEqual(cfg.Stages, []string{"build", "test", "deploy"}) {
		t.Errorf("Stages = %v, want [build test deploy]", cfg.Stages)
	}
	if cfg.Name != "My CI" {
		t.Errorf("Name = %q, want %q", cfg.Name, "My CI")
	}
	if cfg.On == nil {
		t.Fatal("expected On to be populated, got nil")
	}
	if _, ok := cfg.On["push"]; !ok {
		t.Error("expected On to contain 'push' key")
	}
	if _, ok := cfg.On["pull_request"]; !ok {
		t.Error("expected On to contain 'pull_request' key")
	}
	if cfg.Defaults == nil {
		t.Fatal("expected Defaults to be populated, got nil")
	}
	if cfg.Env == nil || cfg.Env["CI"] != "true" {
		t.Errorf("unexpected Env: %v", cfg.Env)
	}
	if cfg.Permissions == nil || cfg.Permissions["contents"] != "read" {
		t.Errorf("unexpected Permissions: %v", cfg.Permissions)
	}
}

func TestLoadConfiguration_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	if err := os.WriteFile(path, []byte(`{{{broken`), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing configuration file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfiguration_OnAsScalarShorthand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
on: push
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "on" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfiguration_DefaultsAsNonMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
defaults: invalid
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "defaults" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfiguration_PermissionsAsNonMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
permissions: invalid
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "permissions" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfiguration_StagesAsNonList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: build
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "stages" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfiguration_NonStringStageEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [1, 2]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "all stages must be strings") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LoadPackage tests (8 cases) ---

func TestLoadPackage_HappyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg_drupal.yml")
	content := `id: drupal
env:
  DB_HOST: localhost
permissions:
  contents: read
hooks:
  build:
    docker-php:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
  test:
    phpunit:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test package: %v", err)
	}

	pkg, err := LoadPackage(path)
	if err != nil {
		t.Fatalf("LoadPackage() error: %v", err)
	}

	if pkg.ID != "drupal" {
		t.Errorf("ID = %q, want %q", pkg.ID, "drupal")
	}
	if pkg.SourceFile != path {
		t.Errorf("SourceFile = %q, want %q", pkg.SourceFile, path)
	}
	if pkg.Env == nil || pkg.Env["DB_HOST"] != "localhost" {
		t.Errorf("unexpected Env: %v", pkg.Env)
	}
	if pkg.Permissions == nil || pkg.Permissions["contents"] != "read" {
		t.Errorf("unexpected Permissions: %v", pkg.Permissions)
	}
	if pkg.Hooks == nil {
		t.Fatal("expected Hooks to be populated, got nil")
	}
	if _, ok := pkg.Hooks["build"]; !ok {
		t.Error("expected Hooks to contain 'build' stage")
	}
	if _, ok := pkg.Hooks["build"]["docker-php"]; !ok {
		t.Error("expected build stage to contain 'docker-php' job")
	}
	if _, ok := pkg.Hooks["test"]; !ok {
		t.Error("expected Hooks to contain 'test' stage")
	}
	if _, ok := pkg.Hooks["test"]["phpunit"]; !ok {
		t.Error("expected test stage to contain 'phpunit' job")
	}
}

func TestLoadPackage_FileNotFound(t *testing.T) {
	_, err := LoadPackage("/nonexistent/pkg_missing.yml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "reading package file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPackage_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg_broken.yml")
	if err := os.WriteFile(path, []byte(`{{{broken`), 0o644); err != nil {
		t.Fatalf("writing test package: %v", err)
	}

	_, err := LoadPackage(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing package file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPackage_DisallowedName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg_bad.yml")
	content := `id: drupal
name: "My Package"
hooks:
  build:
    j1:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test package: %v", err)
	}

	_, err := LoadPackage(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid top-level key "name"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPackage_DisallowedOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg_bad.yml")
	content := `id: drupal
on:
  push: {}
hooks:
  build:
    j1:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test package: %v", err)
	}

	_, err := LoadPackage(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid top-level key "on"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPackage_DisallowedDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg_bad.yml")
	content := `id: drupal
defaults:
  run:
    shell: bash
hooks:
  build:
    j1:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test package: %v", err)
	}

	_, err := LoadPackage(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid top-level key "defaults"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPackage_InvalidEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg_bad.yml")
	content := `id: drupal
env: not-a-map
hooks:
  build:
    j1:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test package: %v", err)
	}

	_, err := LoadPackage(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "env" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPackage_InvalidPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg_bad.yml")
	content := `id: drupal
permissions: not-a-map
hooks:
  build:
    j1:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test package: %v", err)
	}

	_, err := LoadPackage(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "permissions" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LoadProject tests (7 cases) ---

func TestLoadProject_HappyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `env:
  DEPLOY_TARGET: staging
permissions:
  contents: write
hooks:
  build:
    docker-php:
      extend:
        provided_by: drupal
      timeout-minutes: 30
  test:
    lighthouse:
      runs-on: ubuntu-latest
      steps:
        - run: echo "audit"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	proj, err := LoadProject(path)
	if err != nil {
		t.Fatalf("LoadProject() error: %v", err)
	}

	if proj.Env == nil || proj.Env["DEPLOY_TARGET"] != "staging" {
		t.Errorf("unexpected Env: %v", proj.Env)
	}
	if proj.Permissions == nil || proj.Permissions["contents"] != "write" {
		t.Errorf("unexpected Permissions: %v", proj.Permissions)
	}
	if proj.Hooks == nil {
		t.Fatal("expected Hooks to be populated, got nil")
	}

	// Check extend directive.
	buildJobs, ok := proj.Hooks["build"]
	if !ok {
		t.Fatal("expected Hooks to contain 'build' stage")
	}
	dockerPHP, ok := buildJobs["docker-php"]
	if !ok {
		t.Fatal("expected build stage to contain 'docker-php' job")
	}
	if dockerPHP.Extend == nil {
		t.Fatal("expected docker-php to have Extend directive")
	}
	if dockerPHP.Extend.ProvidedBy != "drupal" {
		t.Errorf("Extend.ProvidedBy = %q, want %q", dockerPHP.Extend.ProvidedBy, "drupal")
	}
	if dockerPHP.Properties["timeout-minutes"] == nil {
		t.Error("expected docker-php Properties to contain 'timeout-minutes'")
	}

	// Check new job (no directive).
	testJobs, ok := proj.Hooks["test"]
	if !ok {
		t.Fatal("expected Hooks to contain 'test' stage")
	}
	lighthouse, ok := testJobs["lighthouse"]
	if !ok {
		t.Fatal("expected test stage to contain 'lighthouse' job")
	}
	if !lighthouse.IsNew() {
		t.Error("expected lighthouse to be a new job (no directive)")
	}
	if lighthouse.Properties["runs-on"] != "ubuntu-latest" {
		t.Errorf("unexpected runs-on: %v", lighthouse.Properties["runs-on"])
	}
}

func TestLoadProject_FileNotFound(t *testing.T) {
	_, err := LoadProject("/nonexistent/project.yml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "reading project file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_DisallowedName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `name: "My Project"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid top-level key "name"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_DisallowedOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `on:
  push: {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid top-level key "on"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_DisallowedDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `defaults:
  run:
    shell: bash
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid top-level key "defaults"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_InvalidEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `env: not-a-map
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "env" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_InvalidPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `permissions: not-a-map
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "permissions" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- parseProjectJob / parseProvidedByRef tests via LoadProject (4 cases) ---

func TestLoadProject_MultipleDirectives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `hooks:
  build:
    docker-php:
      extend:
        provided_by: drupal
      replace:
        provided_by: redis
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot declare multiple directives") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_ExtendNotAMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `hooks:
  build:
    docker-php:
      extend: "invalid"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "expected a map") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_MissingProvidedBy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `hooks:
  build:
    docker-php:
      extend:
        something: else
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing 'provided_by'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_ProvidedByNotAString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `hooks:
  build:
    docker-php:
      extend:
        provided_by: 42
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "'provided_by' must be a string") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfiguration_OnAsListShorthand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
on: [push, pull_request]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `invalid "on" format`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	if err := os.WriteFile(path, []byte(`{{{broken`), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing project file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProject_ReplaceNotAMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `hooks:
  build:
    docker-php:
      replace: "invalid"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "expected a map") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfiguration_FileNotFound(t *testing.T) {
	_, err := LoadConfiguration("/nonexistent/configuration.yml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "reading configuration file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LoadPackages tests ---

func TestLoadPackages_HappyPath(t *testing.T) {
	dir := t.TempDir()
	pkg1Path := filepath.Join(dir, "pkg_alpha.yml")
	pkg1Content := `id: alpha
hooks:
  build:
    setup:
      runs-on: ubuntu-latest
`
	pkg2Path := filepath.Join(dir, "pkg_beta.yml")
	pkg2Content := `id: beta
hooks:
  test:
    lint:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(pkg1Path, []byte(pkg1Content), 0o644); err != nil {
		t.Fatalf("writing pkg_alpha.yml: %v", err)
	}
	if err := os.WriteFile(pkg2Path, []byte(pkg2Content), 0o644); err != nil {
		t.Fatalf("writing pkg_beta.yml: %v", err)
	}

	pkgs, err := LoadPackages([]string{pkg1Path, pkg2Path})
	if err != nil {
		t.Fatalf("LoadPackages() error: %v", err)
	}

	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
	if pkgs[0].ID != "alpha" {
		t.Errorf("pkgs[0].ID = %q, want %q", pkgs[0].ID, "alpha")
	}
	if pkgs[1].ID != "beta" {
		t.Errorf("pkgs[1].ID = %q, want %q", pkgs[1].ID, "beta")
	}
}

func TestLoadPackages_ErrorMidIteration(t *testing.T) {
	dir := t.TempDir()
	pkg1Path := filepath.Join(dir, "pkg_good.yml")
	pkg1Content := `id: good
hooks:
  build:
    j1:
      runs-on: ubuntu-latest
`
	if err := os.WriteFile(pkg1Path, []byte(pkg1Content), 0o644); err != nil {
		t.Fatalf("writing pkg_good.yml: %v", err)
	}

	// Second path is nonexistent to trigger an error mid-iteration.
	pkgs, err := LoadPackages([]string{pkg1Path, "/nonexistent/pkg_missing.yml"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if pkgs != nil {
		t.Errorf("expected nil result on error, got %v", pkgs)
	}
	if !strings.Contains(err.Error(), "reading package file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPackages_EmptyList(t *testing.T) {
	pkgs, err := LoadPackages([]string{})
	if err != nil {
		t.Fatalf("LoadPackages([]) error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d", len(pkgs))
	}
}

// --- LoadProject happy path with all directives (including disable) ---

func TestLoadProject_HappyPath_AllDirectives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `env:
  DEPLOY_TARGET: staging
permissions:
  contents: write
hooks:
  build:
    docker-php:
      extend:
        provided_by: drupal
      timeout-minutes: 30
  test:
    phpunit:
      replace:
        provided_by: drupal
      runs-on: self-hosted
    redis-check:
      disable:
        provided_by: redis
    lighthouse:
      runs-on: ubuntu-latest
      steps:
        - run: echo "audit"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	proj, err := LoadProject(path)
	if err != nil {
		t.Fatalf("LoadProject() error: %v", err)
	}

	// Verify extend directive.
	dockerPHP := proj.Hooks["build"]["docker-php"]
	if !dockerPHP.IsExtend() {
		t.Error("expected docker-php to have Extend directive")
	}
	if dockerPHP.Extend.ProvidedBy != "drupal" {
		t.Errorf("Extend.ProvidedBy = %q, want %q", dockerPHP.Extend.ProvidedBy, "drupal")
	}

	// Verify replace directive.
	phpunit := proj.Hooks["test"]["phpunit"]
	if !phpunit.IsReplace() {
		t.Error("expected phpunit to have Replace directive")
	}
	if phpunit.Replace.ProvidedBy != "drupal" {
		t.Errorf("Replace.ProvidedBy = %q, want %q", phpunit.Replace.ProvidedBy, "drupal")
	}

	// Verify disable directive (the gap we're covering).
	redisCheck := proj.Hooks["test"]["redis-check"]
	if !redisCheck.IsDisable() {
		t.Error("expected redis-check to have Disable directive")
	}
	if redisCheck.Disable.ProvidedBy != "redis" {
		t.Errorf("Disable.ProvidedBy = %q, want %q", redisCheck.Disable.ProvidedBy, "redis")
	}

	// Verify new job (no directive).
	lighthouse := proj.Hooks["test"]["lighthouse"]
	if lighthouse.IsExtend() || lighthouse.IsReplace() || lighthouse.IsDisable() {
		t.Error("expected lighthouse to have no directive (new job)")
	}
	if lighthouse.Properties["runs-on"] != "ubuntu-latest" {
		t.Errorf("lighthouse runs-on = %v, want %q", lighthouse.Properties["runs-on"], "ubuntu-latest")
	}
}

func TestLoadProject_DisableNotAMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	content := `hooks:
  build:
    docker-php:
      disable: "invalid"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test project: %v", err)
	}

	_, err := LoadProject(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "expected a map") {
		t.Fatalf("unexpected error: %v", err)
	}
}
