# github-ci-assembler

**Composable CI/CD pipeline system for GitHub Actions**

`github-ci-assembler` is a CLI tool that assembles modular GitHub Actions workflows from reusable packages. Instead of maintaining monolithic workflow files, you compose pipelines from multiple packages, each contributing jobs to predefined stages.

## Features

- **Package-based composition** — Combine multiple technology packages (Drupal, Redis, etc.) without conflicts
- **Stage-based topology** — Define pipeline stages once; packages contribute jobs to any stage
- **Project customizations** — Extend, replace, or disable package jobs without forking
- **Native GitHub Actions syntax** — Everything below the job level is standard GitHub Actions (GHA) YAML
- **Automatic dependency management** — Jobs within stages run in parallel; stages run sequentially
- **Collision-free by design** — Automatic job prefixing eliminates ID conflicts

## Quick Start

### Installation

```bash
go install github.com/sparkfabrik/github-ci-assembler/cmd/gh-ci-assembler@latest
```

### Basic Usage

```bash
# Generate workflow from packages
gh-ci-assembler generate \
  --conf configuration.yml \
  --pkg pkg_base.yml \
  --pkg pkg_drupal.yml \
  --output .github/workflows/gh-ci-assembler.yml

# With project customizations
gh-ci-assembler generate \
  --conf configuration.yml \
  --pkg pkg_base.yml \
  --pkg pkg_drupal.yml \
  --project project.yml \
  --output .github/workflows/gh-ci-assembler.yml

# Dry run (print to stdout)
gh-ci-assembler generate \
  --conf configuration.yml \
  --pkg pkg_base.yml \
  --dry-run
```

## Configuration Files

### configuration.yml

Defines the stage topology:

```yaml
version: "1"

stages:
  - build
  - test
  - deploy
```

### pkg_*.yml (Packages)

Each package contributes jobs and workflow-level properties:

```yaml
id: drupal

name: "Drupal CI"
on:
  push: {}
  pull_request: {}

env:
  PHP_VERSION: "8.2"

hooks:
  build:
    docker-php:
      name: Build PHP container
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Build
          run: docker build -t app:latest .
  
  test:
    phpunit:
      name: PHPUnit tests
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Run tests
          run: vendor/bin/phpunit
```

**Key points:**
- `id` is required and must be unique across all packages
- `hooks` maps stages to job definitions (native GHA syntax)
- Workflow properties (`name`, `on`, `defaults`, `env`) are merged across packages
- `on` must use map form only (no shorthand scalars or arrays)

### project.yml (Optional)

Customize package jobs per-project:

```yaml
# Extend a package job (deep merge)
hooks:
  build:
    - job_id: docker-php
      provided_by: drupal
      extend:
        env:
          CUSTOM_VAR: "value"
        steps:
          - name: Additional step
            run: echo "custom logic"

# Replace a package job entirely
    - job_id: phpunit
      provided_by: drupal
      replace:
        runs-on: ubuntu-latest
        steps:
          - name: Custom test
            run: npm test

# Disable a package job
    - job_id: deploy-staging
      provided_by: drupal
      disable: true

# Add new project-specific job
    - job_id: lighthouse
      new:
        name: Lighthouse audit
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
          - name: Run Lighthouse
            run: npm run lighthouse

# Override workflow-level properties
name: "My Project CI"
env:
  PROJECT_NAME: "acme"
```

## How It Works

1. **Load configuration** — Read stage topology from `configuration.yml`
2. **Load packages** — Parse each `--pkg` file in order
3. **Load project** — Parse `project.yml` if provided
4. **Validate** — Check IDs, stage references, directive targets
5. **Merge jobs** — Apply extend/replace/disable operations
6. **Expand stages** — Build stage topology with pre-/post- virtual stages
7. **Compute dependencies** — Generate `needs` chains for sequential stages
8. **Generate names** — Create display names: `[stage] pkg-id · job-name`
9. **Render YAML** — Write GitHub Actions workflow file

## Job Naming and Prefixing

**Package jobs** are automatically prefixed to prevent collisions:

```
Original:        docker-php (in stage build, package drupal)
Generated ID:    build--drupal--docker-php
Display name:    [build] drupal · Build PHP container
```

**Project jobs** (using `new` directive) are not prefixed:

```
Original:        lighthouse (in stage test)
Generated ID:    test--lighthouse
Display name:    [test] Lighthouse audit
```

## Command Reference

### generate

Generate a GitHub Actions workflow from configuration files.

```bash
gh-ci-assembler generate [flags]
```

**Flags:**
- `--conf <file>` — Configuration file (required)
- `--pkg <file>` — Package file (repeatable, order matters)
- `--project <file>` — Project customization file (optional)
- `--output <file>` — Output workflow file (required unless --dry-run)
- `--dry-run` — Print to stdout instead of writing file

**Exit codes:**
- `0` — Success
- `1` — Validation or assembly error

## Examples

See `testdata/full-example/` for complete working examples:

- `configuration.yml` — 4-stage pipeline
- `pkg_base.yml` — Base workflow properties + placeholder job
- `pkg_drupal.yml` — Drupal build/test/notify jobs
- `pkg_redis.yml` — Redis build/test/notify/deploy jobs
- `project.yml` — All customization operations (extend/replace/disable/new)
- `golden/expected.yml` — Generated workflow output

## Specification

Full specification: `specs/gh-ci-assembler.md` (version 2.1.0-draft)

Key design principles:

- **Native GHA below job level** — No custom DSL; job properties are passed through as-is
- **Explicit package identity** — `id` field defines package contract, not filename
- **Linear stage topology** — Jobs in stage N depend on all jobs in stage N-1
- **Deep merge semantics** — Kubernetes strategic merge patch rules for `extend` operations

## Development

**Build:**
```bash
go build ./...
```

**Test:**
```bash
go test ./...
```

**Update golden files:**
```bash
UPDATE_GOLDEN=1 go test ./...
```

## Requirements

- Go 1.25.6 or later
- Dependencies managed via `go.mod`

## License

Copyright SparkFabrik. All rights reserved.

## See Also

- Full specification: `specs/gh-ci-assembler.md`
- JSON schemas: `schemas/gh-ci-assembler-schemas.json`
- AI context: `AGENTS.md`
