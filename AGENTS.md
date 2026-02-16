# AGENTS.md — AI Context for GitHub CI Assembler

This document provides context for AI coding assistants working on the `gh-ci-assembler` project. It captures design decisions, implementation details, and development conventions to maintain consistency across conversations.

## Project Overview

**gh-ci-assembler** is a Go CLI tool that assembles modular GitHub Actions workflows from reusable packages. It implements a composable CI/CD pipeline system based on the specification in `specs/gh-ci-assembler.md`

**Module path:** `github.com/sparkfabrik/github-ci-assembler`  

**Go version:** 1.25.6 (triggers LSP warnings but builds successfully)

## Architecture

### File Structure

```
cmd/gh-ci-assembler/main.go       CLI entry point (Cobra)
internal/
  config/
    types.go                       Core data structures
    config.go                      configuration.yml parser
    package.go                     pkg_*.yml parser
    project.go                     project.yml parser
  assembly/
    assembly.go                    Main assembly orchestrator (7 phases)
    merge.go                       Deep merge algorithm
    stages.go                      Stage topology expansion
    needs.go                       Dependency chain computation
    names.go                       Display name generation
    *_test.go                      Unit tests + golden file tests
  validation/
    validation.go                  All validation logic
  render/
    render.go                      YAML output with controlled ordering
testdata/full-example/             Test fixtures
  configuration.yml                4 stages: build, notify, test, deploy
  pkg_base.yml                     Base workflow properties
  pkg_drupal.yml                   Drupal jobs
  pkg_redis.yml                    Redis jobs
  project.yml                      All customization operations
  golden/                          Expected output files
specs/gh-ci-assembler.md           Full specification (~1165 lines)
schemas/gh-ci-assembler-schemas.json      JSON schemas
```

### Core Data Structures (internal/config/types.go)

- **Configuration** — Stage topology (`version`, `stages`)
- **Package** — Package file data (`id`, `name`, `on`, `defaults`, `env`, `hooks`)
- **Project** — Project customizations (`name`, `on`, `defaults`, `env`, `hooks`)
- **ProjectJob** — Job customization directive (extend/replace/disable/new + `provided_by`)
- **AssembledJob** — Job after merge/assembly (ID, stage, package ID, job def, disabled flag)
- **WorkflowProperties** — Workflow-level data (name, on, defaults, env)
- **AssemblyResult** — Final output (jobs + workflow properties)

### Assembly Pipeline (7 Phases)

Implemented in `internal/assembly/assembly.go`:

1. **Load configuration** — Parse stage list
2. **Load packages** — Parse each `--pkg` file in order
3. **Load project** — Parse `project.yml` (optional)
4. **Validate** — Check IDs, stage refs, directive targets, `on` map-form
5. **Merge jobs** — Apply extend/replace/disable/new operations
6. **Expand stages** — Build topology with pre-/post- virtual stages
7. **Compute dependencies** — Generate `needs` arrays
8. **Generate display names** — Format: `[stage] pkg-id · name`
9. **Render YAML** — Output with controlled key order and comments

## Key Design Decisions

### Completed Design Choices

1. **CLI structure:** Single `generate` command only (no `validate` yet — deferred)
2. **YAML library:** `gopkg.in/yaml.v3` for parsing/rendering
3. **CLI framework:** `github.com/spf13/cobra`
4. **Project structure:** `cmd/` + `internal/` layout
5. **Hooks validation:** `hooks` must be present and non-empty in packages
6. **Pre/post stages in packages:** Allowed but emit warning
7. **Project file:** Optional — skip gracefully if `--project` not provided
8. **Test strategy:** Both unit tests (merge, stages, needs, names) and golden file tests
9. **Stage name validation:** Underscores allowed in user-defined stage names
10. **No virtual stage validation:** Pre/post-prefixed stage names in `configuration.yml` not validated (trust users won't do this)
11. **Job ID sort in needs arrays:** Sort job IDs within each stage to ensure deterministic `needs` arrays despite Go map iteration randomness. This does NOT affect merge priority — only the cosmetic order of `needs` entries (which GHA treats as a set).

### Merge Semantics

Deep merge follows Kubernetes strategic merge patch rules (see `internal/assembly/merge.go`):

- **Maps:** Merge recursively; later package wins on scalar conflicts
- **Arrays with `name` key:** Merge by matching `name` (for `steps`)
- **Other arrays:** Replace entirely (no append)
- **Scalars:** Replace

Merge priority (lowest to highest):
1. First package
2. Second package
3. ...
4. Last package
5. Project file (highest)

### Job Naming Rules

Implemented in `internal/assembly/names.go`:

**Package jobs:**
```
ID:          build--drupal--docker-php
Display:     [build] drupal · Build PHP container
```

**Project jobs (new directive):**
```
ID:          test--lighthouse
Display:     [test] Lighthouse audit
```

Format: `[stage] <pkg-id> · <job-name>` for packages, `[stage] <job-name>` for project jobs.

### Job Prefixing

- **Package jobs:** `{stage}--{package-id}--{original-job-id}`
- **Project new jobs:** `{stage}--{original-job-id}` (no package prefix)
- **Separator:** `--` (double dash, because `.` not allowed in GHA job IDs)

### Stage Topology

Linear topology: jobs in stage N depend on all jobs in stage N-1.

**Virtual stages:** Any configured stage `foo` automatically gets `pre-foo` and `post-foo` virtual stages. Jobs can be inserted into these stages from packages or project file.

**Stage expansion order:** For stages `[build, test]`:
```
pre-build → build → post-build → pre-test → test → post-test
```

### Validation Rules

Implemented in `internal/validation/validation.go`:

**Configuration:**
- `version` must be `"1"`
- `stages` must be non-empty array of strings

**Package:**
- `id` required, must match `[a-z0-9][a-z0-9-]*`
- `id` must be unique across all packages
- `--` forbidden in job IDs (reserved separator)
- Stage references must exist in configuration
- Job IDs must match `[a-z0-9_-]+`
- `on` must be map form only (no scalar/list shorthand)
- `hooks` must be non-empty

**Project:**
- All directives (extend/replace/disable/new) must target valid stages
- `provided_by` references must resolve to existing package IDs
- Job IDs must exist in target package (for extend/replace/disable)
- `on` must be map form only
- Each `hooks.<stage>` entry must have exactly one directive

### YAML Output

Implemented in `internal/render/render.go`:

- **Controlled key order:** `name`, `on`, `defaults`, `env`, `jobs`
- **Stage separator comments:** `# ===== Stage: {stage} =====` between stages
- **Disabled job comments:** `# Disabled by project: {job-id}`
- **Auto-generated header:** Warns against manual editing, includes timestamp
- **RenderOptions:** Accepts optional `GeneratedTime` for deterministic golden file tests
- **Empty stage skipping:** Stages with zero jobs omitted (except in comments)

## Testing

### Unit Tests

Located in `internal/assembly/*_test.go`:

- `merge_test.go` — 6 tests for deep merge algorithm
- `stages_test.go` — 5 tests for stage expansion + ParseVirtualStage
- `needs_test.go` — 5 tests for dependency computation
- `names_test.go` — 5 tests for display name generation

**Total: 21 tests** (all passing as of 2026-02-12)

### Golden File Tests

Located in `internal/assembly/golden_test.go`:

- **TestGolden_FullExample** — Full pipeline with all packages + project
- **TestGolden_PackagesOnly** — Packages only, no project file
- **TestGolden_SinglePackage** — Single base package

**Golden files:** `testdata/full-example/golden/*.yml`  
**Update command:** `UPDATE_GOLDEN=1 go test ./...`

### Test Fixtures

All fixtures in `testdata/full-example/`:

- **configuration.yml:** 4 stages `[build, notify, test, deploy]`
- **pkg_base.yml:** Workflow props + placeholder job in build
- **pkg_drupal.yml:** Build/test/notify jobs + workflow env
- **pkg_redis.yml:** Build/test/notify/deploy jobs + workflow env
- **project.yml:** All operations (extend/replace/disable/new) + workflow overrides

**Important:** Stage names in testdata must NOT look like virtual stage prefixes. Use names like `notify` instead of `post_build` to avoid confusion.

## Common Development Tasks

### Adding a New Feature

1. Update `specs/gh-ci-assembler.md` if changing behavior
2. Add/update types in `internal/config/types.go` if needed
3. Update parser(s) in `internal/config/*.go`
4. Add validation in `internal/validation/validation.go`
5. Update assembly logic in `internal/assembly/*.go`
6. Add unit tests for new logic
7. Update golden files if output changes: `UPDATE_GOLDEN=1 go test ./...`
8. Update `README.md` and this file

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/assembly

# With verbose output
go test -v ./...

# Update golden files
UPDATE_GOLDEN=1 go test ./internal/assembly
```

### Building

```bash
# Build all packages
go build ./...

# Build CLI binary
go build -o gh-ci-assembler ./cmd/gh-ci-assembler

# Install globally
go install ./cmd/gh-ci-assembler
```

### Manual Testing

```bash
# Generate from testdata
./gh-ci-assembler generate \
  --conf testdata/full-example/configuration.yml \
  --pkg testdata/full-example/pkg_base.yml \
  --pkg testdata/full-example/pkg_drupal.yml \
  --pkg testdata/full-example/pkg_redis.yml \
  --project testdata/full-example/project.yml \
  --dry-run
```

## Known Issues and Gotchas

1. **Go version 1.25.6:** Triggers LSP warnings in `go.mod` but builds and runs correctly
2. **Map iteration order:** Go randomizes map iteration. `needs.go` includes explicit sort on job IDs to ensure deterministic output.
3. **`on` map form:** GitHub Actions allows `on: push` and `on: [push, pull_request]`, but we require `on: {push: {}, pull_request: {}}` for well-defined merge behavior.

## Deferred Features

These are explicitly deferred and NOT implemented:

- `validate` CLI command (mentioned in spec but not implemented yet)
- `--diff` mode (mentioned in spec but not implemented yet)

## Working with AI Assistants

When continuing work on this project:

1. **Read this file first** to understand context and decisions
2. **Check `specs/gh-ci-assembler.md`** for authoritative behavior specifications
3. **Run tests after changes:** `go test ./...`
4. **Run formatting** `go fmt ./...` or `gofmt` (with the options you deem necessary)
5. **Run linting:** `golangci-lint run`
6. **Update golden files** if output format changes
7. **Update this file** when making new design decisions
8. **Verify build** after any import path changes: `go build ./...`

## Contact and Feedback

This is an internal SparkFabrik tool. For questions or issues, contact the Platform Team.

---

**Last Updated:** 2026-02-12
**Document Version:** 1.0
