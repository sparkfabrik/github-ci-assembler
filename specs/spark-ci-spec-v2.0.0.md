# Spark CI Extensibility Specification

**Version:** 2.0.0-draft  
**Author:** Platform Team  
**Status:** Draft  
**Last Updated:** 2026-02-10

---

## 1. Executive Summary

This specification defines a composable CI/CD pipeline system for SparkFabrik's GitHub Actions workflows. Instead of a monolithic workflow per project, the pipeline is assembled from multiple **packages** (`pkg_*.yml`), each contributing jobs to predefined stages. A **base configuration** defines the stage topology, and an optional **project configuration** allows per-project customizations through three explicit operations: extend, replace, and disable.

The system generates a standard GitHub Actions workflow YAML file where everything below the job level is native GitHub Actions syntax — no custom DSL, no intermediary abstractions.

---

## 2. Problem Statement

### 2.1 Current Challenge

SparkFabrik provides managed CI/CD workflows (e.g., for Drupal projects) distributed as packages. Teams need the ability to add project-specific jobs (custom linters, security scans, Lighthouse audits, notifications) without:

- Forking or modifying the master workflow
- Breaking future updates from the platform team
- Requiring deep knowledge of GitHub Actions internals

Additionally, when multiple technology packages are composed (e.g., Drupal + Redis + Elasticsearch), each should contribute its pipeline fragment independently without central coordination.

### 2.2 GitLab CI Comparison

In GitLab CI, stages act as logical containers where jobs can be registered declaratively. The `extends` keyword allows deep merging of job definitions, following a well-understood convention shared by Kubernetes strategic merge patches.

GitHub Actions lacks both abstractions. Jobs must explicitly declare dependencies via `needs`, and there is no native merge mechanism for job definitions.

### 2.3 Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| R1 | Multiple packages can contribute jobs independently | Must |
| R2 | Platform team controls available stages (extension points) | Must |
| R3 | Jobs within a stage run in parallel | Must |
| R4 | Each job retains full GitHub Actions expressiveness (runs-on, services, container, permissions) | Must |
| R5 | Configuration is declarative and validatable | Must |
| R6 | Projects can customize package-provided jobs without forking | Must |
| R7 | Projects can remove package-provided jobs they don't need | Must |
| R8 | No collision risk when composing multiple packages | Must |
| R9 | Generated output is a readable, diffable GitHub Actions workflow | Should |
| R10 | Minimal boilerplate for project teams | Must |

---

## 3. Architecture

### 3.1 Assembly Chain

The pipeline is assembled from three layers, processed in order:

```
base.yml          Defines stage topology, defaults, and workflow triggers
    │
    ▼
pkg_*.yml         Each package contributes jobs to stages (native GHA syntax)
    │
    ▼
project.yml       Per-project customizations: extend, replace, disable, new jobs
    │
    ▼
spark-ci          CLI tool assembles all layers and generates:
generate
    │
    ▼
.github/workflows/spark-ci.yml    Standard GitHub Actions workflow (committed)
```

### 3.2 Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| Base Config | `base.yml` | Stage topology, defaults, workflow triggers |
| Packages | `pkg_*.yml` | Technology-specific job contributions |
| Project Config | `project.yml` | Per-project customizations (optional) |
| CLI Tool | `sparkfabrik/spark-ci-loader` | Assembles layers, generates workflow YAML |
| JSON Schema | `sparkfabrik/spark-ci-schema` | Validates all configuration files |

### 3.3 Key Design Principles

**Native GitHub Actions below job level.** The `hooks.<stage>.<job_id>` boundary is the contract line. Everything above it (`hooks`, stage names) is our orchestration layer. Everything below it (`runs-on`, `steps`, `services`, `env`, `container`, `permissions`, etc.) is passed through as-is to the generated workflow. The tool never interprets or transforms job-level properties.

**Explicit package identity.** Every package declares an `id` field — a short, stable identifier that serves as the job prefix and as the reference for `provided_by` in the project configuration. The `id` is the package's contract: renaming the file does not change the package's identity. The loader validates that all package ids are unique across the composition and fails fast with an actionable error if duplicates are detected.

**Automatic job prefixing.** Every job id from a package is automatically prefixed with the package `id` using `.` as separator. `docker-php` declared in a package with `id: drupal` becomes `drupal.docker-php` in the generated workflow. This eliminates collision risk by construction — no coordination between package authors required.

**Project jobs are not prefixed.** Jobs contributed directly by `project.yml` (new jobs, not extending or replacing) appear with their original id. They are first-class citizens in the output, visually distinct from package-provided jobs.

**Automatic job display name.** The tool generates a `name` property for every job to provide clear, hierarchical display in the GitHub Actions UI. The format uses bracket notation to show the stage, package origin, and job purpose at a glance. See section 6.2 for the complete naming rules.

**Linear stage topology.** Stages are processed sequentially. Every job in stage N depends on all jobs in stage N-1. Empty stages are skipped transparently. This follows GitLab CI's model and hides GitHub Actions' DAG complexity from the end user.

---

## 4. Configuration Format

### 4.1 base.yml

Defines the pipeline skeleton: stage order, default job properties, and workflow triggers.

```yaml
version: "1"
name: Spark CI

stages:
  - build
  - post_build
  - test
  - post_test
  - deploy
  - post_deploy

defaults:
  runs-on: ubuntu-latest
  timeout-minutes: 30

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]
```

| Field | Required | Description |
|-------|----------|-------------|
| `version` | Yes | Schema version (currently `"1"`) |
| `name` | No | Workflow display name (default: `Spark CI`) |
| `stages` | Yes | Ordered list of stage names |
| `defaults` | No | Default job-level properties applied when not specified by a package |
| `on` | No | Workflow triggers (default: push to main/develop, PRs to main) |

**Defaults behavior:** Properties in `defaults` are applied to any job that does not explicitly declare them. Package-level and project-level values always take precedence.

### 4.2 pkg_*.yml (Packages)

Each package file contributes jobs to one or more stages. The filename pattern `pkg_*.yml` is used only for **discovery** — the loader scans the directory for files matching this pattern and loads them. The package's identity is determined exclusively by the `id` field declared inside the file.

#### 4.2.1 Package Identity

Every package must declare an `id` at the top level. This `id`:

- Is used as the prefix for all job ids contributed by the package
- Is the value referenced by `provided_by` in `project.yml`
- Must be unique across all packages in the composition
- Must match the pattern `[a-z0-9][a-z0-9-]*` (lowercase alphanumeric with hyphens)

The filename has no bearing on the package's identity. Renaming `pkg_drupal.yml` to `pkg_drupal-cms.yml` has no effect on the package id, the generated job ids, or any `project.yml` references.

**Uniqueness validation.** Immediately after discovery, the loader collects all `id` values and verifies they are unique. If two or more files declare the same `id`, the loader fails with an error listing all conflicting files. This check runs before any hook or job validation.

| `id` | Filename (example) | Job prefix |
|------|--------------------|------------|
| `drupal` | `pkg_drupal.yml` | `drupal.` |
| `redis` | `pkg_redis.yml` | `redis.` |
| `my-cache` | `pkg_cache.yml` | `my-cache.` |

#### 4.2.2 Package Format

```yaml
id: <package-id>

hooks:
  <stage>:
    <job-id>:
      # Everything here is native GitHub Actions job syntax
      name: <display-name>           # optional, used in generated job display name
      runs-on: <runner>
      env: { ... }
      services: { ... }
      container: { ... }
      permissions: { ... }
      timeout-minutes: <int>
      continue-on-error: <bool>
      if: <expression>
      steps:
        - uses: <action>
        - name: <n>
          run: <script>
```

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique package identifier, used as job prefix and `provided_by` target |
| `hooks` | Yes | Map of stage → job_id → job definition (native GHA syntax) |

#### 4.2.3 Package Examples

**pkg_drupal.yml:**

```yaml
id: drupal

hooks:
  build:
    docker-php:
      name: Build PHP image
      runs-on: ubuntu-latest
      env:
        PHP_VERSION: "8.2"
      steps:
        - uses: actions/checkout@v4
        - name: Setup PHP
          uses: shivammathur/setup-php@v2
          with:
            php-version: "8.2"
        - name: Install dependencies
          run: composer install --no-interaction

    docker-nginx:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Build nginx configuration
          run: ./scripts/build-nginx.sh

  test:
    phpunit:
      name: PHPUnit test suite
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Run PHPUnit
          run: vendor/bin/phpunit --coverage-text

  post_build:
    notify:
      runs-on: ubuntu-latest
      steps:
        - name: Notify Drupal build complete
          run: echo "Drupal build finished."
```

**pkg_redis.yml:**

```yaml
id: redis

hooks:
  build:
    docker-redis:
      name: Build Redis image
      runs-on: self-hosted
      services:
        redis:
          image: redis:7-alpine
          ports:
            - "6379:6379"
      steps:
        - uses: actions/checkout@v4
        - name: Build Redis image
          run: docker build -t myapp-redis ./docker/redis

  test:
    job-test:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Run Redis integration tests
          run: ./run-tests.sh

  post_build:
    notify:
      runs-on: ubuntu-latest
      continue-on-error: true
      steps:
        - name: Notify Redis build complete
          run: echo "Redis image built."

  deploy:
    push-redis-image:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Push Redis image
          uses: docker/build-push-action@v5
          with:
            context: ./docker/redis
            push: "true"
            tags: "ghcr.io/myorg/myapp-redis:latest"
```

**Validation rules for packages:**

- `id` is required and must match `[a-z0-9][a-z0-9-]*`
- `id` must be unique across all packages (fail-fast check)
- Stage names must exist in `base.yml`
- Job ids must match `[a-z0-9][a-z0-9_-]*`
- Every job must declare `steps` as a non-empty list
- All properties below `<job-id>` must be valid GitHub Actions job syntax

### 4.3 project.yml (Project Configuration)

The project configuration is the **last element in the assembly chain** and has the exclusive privilege of customizing package-provided jobs. It supports four types of job declarations:

| Type | Directive | Behavior |
|------|-----------|----------|
| **New job** | *(none)* | Contributes a new job, identical to a package contribution |
| **Extend** | `extend` | Deep merges project properties into the target package job |
| **Replace** | `replace` | Fully substitutes the target package job |
| **Disable** | `disable` | Removes the target package job from the pipeline |

All three directives (`extend`, `replace`, `disable`) require `provided_by` to explicitly identify the target package. This is mandatory to prevent breakage when new packages are added to the composition.

**Semantic distinction between extend and replace.** The three directives have deliberately different names because they express different intent:

- `extend` says "I'm customizing" — the original job is the foundation, the project adds or modifies specific properties on top of it.
- `replace` says "I'm redefining" — the original job is discarded entirely, the project provides a complete new definition.
- `disable` says "I'm removing" — the job is excluded from the pipeline.

While `extend` with all properties redeclared is functionally similar to `replace`, the semantic signal matters for readability and code review. A reviewer seeing `replace` knows immediately that nothing from the original package survives.

#### 4.3.1 New Jobs

Jobs without any directive are contributed as-is, like a regular package. They are **not prefixed** — their id appears as declared in the generated workflow.

```yaml
hooks:
  test:
    custom-lint:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Run custom linter
          run: ./scripts/lint.sh
```

#### 4.3.2 Extend (Deep Merge)

The `extend` directive performs a deep merge of the project's properties into the target package job, following Kubernetes / GitLab CI conventions:

| Property type | Merge behavior |
|---------------|----------------|
| Maps (env, services, with) | Recursive merge; project keys win on conflict |
| Sequential arrays (steps) | Full replacement; project array replaces package array |
| Scalars (runs-on, timeout-minutes) | Project value replaces package value |

Properties not declared in the extend are preserved from the package.

```yaml
hooks:
  build:
    # Add environment variables to pkg_drupal's docker-php job.
    # Steps, runner, and all other properties remain untouched.
    docker-php:
      extend:
        provided_by: drupal
      env:
        DATABASE_URL: postgres://db:5432/myapp
        REDIS_HOST: redis

    # Add a service sidecar and change the timeout.
    # Steps remain untouched.
    docker-php-alt:
      extend:
        provided_by: drupal
      services:
        postgres:
          image: postgres:16
          ports:
            - "5432:5432"
      timeout-minutes: 45
```

**Extend with steps declared:** Since `steps` is a sequential array, declaring it in an extend replaces the entire steps list from the package. All other properties still merge normally. This is the expected behavior when the project needs to change *how* the job executes while preserving its context (env, services, runner).

```yaml
hooks:
  build:
    docker-php:
      extend:
        provided_by: drupal
      steps:
        # These steps fully replace drupal's steps.
        # env, services, runs-on etc. from drupal are preserved.
        - uses: actions/checkout@v4
        - name: Custom build process
          run: ./my-build.sh
```

#### 4.3.3 Replace (Full Substitution)

The `replace` directive discards the target job's entire definition and substitutes it with the project's definition. Nothing from the package survives.

Use `replace` when the project's needs are fundamentally different from what the package provides, and partial merging would be more confusing than a clean redefinition.

```yaml
hooks:
  build:
    docker-redis:
      replace:
        provided_by: redis
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Custom Redis build
          run: ./my-redis-build.sh
```

**When to use extend vs replace:**

| Scenario | Use extend | Use replace |
|----------|-----------|-------------|
| Add env variables | ✓ | |
| Add a service sidecar | ✓ | |
| Change timeout or runner | ✓ | |
| Change steps but keep env/services | ✓ | |
| Completely redefine the job from scratch | | ✓ |

#### 4.3.4 Disable (Removal)

The `disable` directive removes a package-provided job from the generated pipeline entirely. The job will not appear in the output workflow, and subsequent stages' `needs` chains are recomputed accordingly.

```yaml
hooks:
  test:
    job-test:
      disable:
        provided_by: redis
```

**Error handling:** Disabling a job that does not exist in the target package is a fatal error. This prevents stale disable directives from silently hiding misconfiguration.

#### 4.3.5 Complete project.yml Example

```yaml
# project.yml
hooks:
  build:
    # Extend: add project-specific env to Drupal build
    docker-php:
      extend:
        provided_by: drupal
      env:
        DATABASE_URL: postgres://db:5432/myapp
        REDIS_HOST: redis
      services:
        postgres:
          image: postgres:16
          ports:
            - "5432:5432"

    # Replace: completely redefine Redis build
    docker-redis:
      replace:
        provided_by: redis
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Build custom Redis image
          run: ./scripts/build-redis.sh

  test:
    # Disable: remove Redis integration tests (not needed in this project)
    job-test:
      disable:
        provided_by: redis

    # New job: project-specific linting
    custom-lint:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - name: Run project linter
          run: ./scripts/lint.sh

  post_build:
    # Extend: replace notification steps, keep other properties
    notify:
      extend:
        provided_by: drupal
      steps:
        - name: Custom notification
          run: |
            curl -X POST $SLACK_WEBHOOK \
              -d '{"text": "Build complete for ${{ github.repository }}"}'
```

---

## 5. Assembly Process

### 5.1 Pipeline Assembly Steps

The `spark-ci-loader` CLI tool processes the configuration in the following order:

```
Phase 1: Load base configuration
  → Parse base.yml
  → Extract stage topology and defaults

Phase 2: Discover and load packages
  → Find all pkg_*.yml files (sorted alphabetically by filename)
  → Parse each file and extract the id field
  → Validate id uniqueness across all packages (fail fast)
  → Validate hooks and job definitions against base stages
  → Prefix all job ids with package id

Phase 3: Apply project configuration
  → Load project.yml (if present)
  → Apply disable operations (remove target jobs)
  → Apply replace operations (substitute target jobs)
  → Apply extend operations (deep merge into target jobs)
  → Add new project jobs (unprefixed)

Phase 4: Resolve needs chains
  → Identify active stages (stages with at least one job)
  → For each job in stage N, set needs = [all job ids in stage N-1]
  → Skip empty stages transparently

Phase 5: Generate display names
  → Compute the name property for each job (see section 6.2)

Phase 6: Render
  → Generate valid GitHub Actions workflow YAML
  → Write to output path
```

### 5.2 Automatic `needs` Computation

The `needs` chain is derived mechanically from the stage topology. Given stages `[build, post_build, test, deploy]` and the following jobs:

```
build:       drupal.docker-php, drupal.docker-nginx, redis.docker-redis
post_build:  drupal.notify, redis.notify
test:        (empty — no package contributes here)
deploy:      redis.push-redis-image
```

The computed needs are:

| Job | needs |
|-----|-------|
| `drupal.docker-php` | *(none — first stage)* |
| `drupal.docker-nginx` | *(none)* |
| `redis.docker-redis` | *(none)* |
| `drupal.notify` | `[drupal.docker-php, drupal.docker-nginx, redis.docker-redis]` |
| `redis.notify` | `[drupal.docker-php, drupal.docker-nginx, redis.docker-redis]` |
| `redis.push-redis-image` | `[drupal.notify, redis.notify]` |

Note that `test` is empty, so `deploy` depends on `post_build` directly.

### 5.3 Deep Merge Algorithm

The deep merge follows Kubernetes strategic merge patch semantics:

```
function deepMerge(base, overlay):
    result = copy(base)
    for each key in overlay:
        if key not in result:
            result[key] = overlay[key]
        else if both result[key] and overlay[key] are associative arrays (maps):
            result[key] = deepMerge(result[key], overlay[key])
        else:
            result[key] = overlay[key]    // scalars and sequential arrays: right side wins
    return result
```

**Examples:**

Merging env (associative array → recursive merge):
```
base:    { PHP_VERSION: "8.2", APP_ENV: "test" }
overlay: { DATABASE_URL: "postgres://...", APP_ENV: "prod" }
result:  { PHP_VERSION: "8.2", APP_ENV: "prod", DATABASE_URL: "postgres://..." }
```

Merging steps (sequential array → full replacement):
```
base:    [{ run: "echo step1" }, { run: "echo step2" }]
overlay: [{ run: "echo new-step" }]
result:  [{ run: "echo new-step" }]
```

Merging services (associative array → recursive merge):
```
base:    { mysql: { image: "mysql:8" } }
overlay: { redis: { image: "redis:7" } }
result:  { mysql: { image: "mysql:8" }, redis: { image: "redis:7" } }
```

---

## 6. Generated Output

### 6.1 Output Format

The generated file is a standard GitHub Actions workflow with an auto-generated header:

```yaml
# ┌──────────────────────────────────────────────────────────────────────┐
# │ AUTO-GENERATED by spark-ci-loader — do not edit manually            │
# │ Source: base.yml, pkg_drupal.yml, pkg_redis.yml, project.yml        │
# │ Generated: 2026-02-10T14:32:00+01:00                                │
# └──────────────────────────────────────────────────────────────────────┘

name: Spark CI
on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  # ── Stage: build ────────────────────────────────────────────

  drupal.docker-php:
    name: "[build] drupal · Build PHP image"
    runs-on: ubuntu-latest
    env:
      PHP_VERSION: "8.2"
      DATABASE_URL: postgres://db:5432/myapp
      REDIS_HOST: redis
    services:
      postgres:
        image: postgres:16
        ports:
          - "5432:5432"
    steps:
      - uses: actions/checkout@v4
      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: "8.2"
      - name: Install dependencies
        run: composer install --no-interaction

  drupal.docker-nginx:
    name: "[build] drupal · docker-nginx"
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build nginx configuration
        run: ./scripts/build-nginx.sh

  redis.docker-redis:
    name: "[build] redis · Build Redis image"
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build custom Redis image
        run: ./scripts/build-redis.sh

  # ── Stage: post_build ───────────────────────────────────────

  drupal.notify:
    name: "[post_build] drupal · notify"
    needs: [drupal.docker-php, drupal.docker-nginx, redis.docker-redis]
    runs-on: ubuntu-latest
    steps:
      - name: Custom notification
        run: |
          curl -X POST $SLACK_WEBHOOK \
            -d '{"text": "Build complete for ${{ github.repository }}"}'

  redis.notify:
    name: "[post_build] redis · notify"
    needs: [drupal.docker-php, drupal.docker-nginx, redis.docker-redis]
    runs-on: ubuntu-latest
    continue-on-error: true
    steps:
      - name: Notify Redis build complete
        run: echo "Redis image built."

  # ── Stage: test ─────────────────────────────────────────────
  # redis.job-test: DISABLED by project.yml

  custom-lint:
    name: "[test] custom-lint"
    needs: [drupal.notify, redis.notify]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run project linter
        run: ./scripts/lint.sh

  # ── Stage: deploy ───────────────────────────────────────────

  redis.push-redis-image:
    name: "[deploy] redis · push-redis-image"
    needs: [custom-lint]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Push Redis image
        uses: docker/build-push-action@v5
        with:
          context: ./docker/redis
          push: "true"
          tags: "ghcr.io/myorg/myapp-redis:latest"
```

### 6.2 Job Display Names

The tool generates a `name` property for every job in the output workflow. The display name provides hierarchical context visible in the GitHub Actions UI (checks tab, sidebar, PR status).

**Format:**

| Job origin | Has `name` in source | Generated display name |
|------------|---------------------|----------------------|
| Package | Yes | `[stage] package-id · name` |
| Package | No | `[stage] package-id · job-id` |
| Project (new) | Yes | `[stage] name` |
| Project (new) | No | `[stage] job-id` |
| Project (extend/replace) | — | Same rules as package (keeps package origin) |

**Examples with the bracket notation:**

```
[build] drupal · Build PHP image         ← package job with name declared
[build] drupal · docker-nginx            ← package job without name (falls back to job id)
[build] redis · Build Redis image        ← package job with name declared
[post_build] drupal · notify             ← package job without name
[test] custom-lint                       ← project job without name (no package prefix)
[test] Security scan                     ← project job with name declared
[deploy] redis · push-redis-image        ← package job without name
```

**Rules:**

- The `name` declared in the source file is never passed through as-is. The tool always wraps it in the bracket notation.
- If a job declares `name` in the source, the tool uses that as the human-readable portion. If not, it uses the `job-id`.
- The `name` property in the source file is consumed by the tool and replaced in the output. It is not a native GitHub Actions passthrough — it is the only exception to the "everything below job-id is native GHA" rule.
- Project jobs that extend or replace a package job retain the package origin in their display name (they appear as `[stage] package-id · ...` since the job id in the output is still prefixed).

### 6.3 Lifecycle Management

The generated workflow file is committed to the repository. Two strategies are supported:

**Strategy A — CLI generation with CI check (recommended).**

Developers run the CLI locally (or via a pre-commit hook) and commit the generated file alongside the source configuration. A CI step validates that the committed file is in sync:

```yaml
# In an existing CI workflow
- name: Verify Spark CI workflow is up to date
  run: |
    bin/spark-ci generate --diff
```

If the file is stale, the check fails with a diff showing what changed.

**Strategy B — Automated generation via CI.**

A meta-workflow detects changes to configuration files, regenerates the workflow, and auto-commits:

```yaml
name: Regenerate Spark CI
on:
  push:
    paths:
      - "base.yml"
      - "pkg_*.yml"
      - "project.yml"
jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Generate workflow
        run: bin/spark-ci generate
      - name: Commit if changed
        run: |
          git config user.name "spark-ci-bot"
          git config user.email "bot@sparkfabrik.com"
          git add .github/workflows/spark-ci.yml
          git diff --cached --quiet || git commit -m "chore: regenerate spark-ci workflow"
          git push
```

Strategy A is recommended for its simplicity and transparency.

---

## 7. CLI Tool

### 7.1 Commands

**Generate:**
```bash
bin/spark-ci generate \
  --base=base.yml \
  --pkg-dir=. \
  --project=project.yml \
  --output=.github/workflows/spark-ci.yml
```

| Option | Default | Description |
|--------|---------|-------------|
| `--base`, `-b` | `base.yml` | Path to base configuration |
| `--pkg-dir`, `-p` | `.` | Directory containing `pkg_*.yml` files |
| `--project` | `project.yml` | Path to project configuration |
| `--output`, `-o` | `.github/workflows/spark-ci.yml` | Output path |
| `--dry-run` | | Print to stdout without writing |
| `--diff` | | Compare generated vs existing (for CI) |

**Validate:**
```bash
bin/spark-ci validate \
  --base=base.yml \
  --pkg-dir=. \
  --project=project.yml
```

Validates all configuration files without generating output. Checks:

- YAML syntax
- Schema compliance
- Package id format and uniqueness
- Stage references
- Job id format
- Extend/replace/disable target resolution
- No contradictory project directives (extend + disable on same target)

---

## 8. Error Handling

All errors are fatal and produce actionable messages identifying the file, stage, job, and specific issue.

### 8.1 Package Validation Errors

```
Error: Missing required "id" in pkg_drupal.yml.
       Every package must declare an explicit id.
```

```
Error: Duplicate package id "redis".
       Declared in: pkg_redis.yml, pkg_custom_redis.yml
       Every package must have a unique id.
```

```
Error: Invalid package id "My-Package" in pkg_bad.yml.
       Package id must match [a-z0-9][a-z0-9-]* (lowercase, hyphens allowed).
```

```
Error: Package "pkg_bad.yml" (id: bad) references unknown stage "unknown_stage".
       Valid stages: [build, post_build, test, deploy]
```

```
Error: Invalid job "compile" in stage "build" of pkg_bad.yml (id: bad):
       Job must define "steps" as a non-empty list.
```

### 8.2 Project Validation Errors

```
Error: project.yml declares extend for "docker-php" (provided_by: drupal)
       in stage "build", but no matching job "drupal.docker-php" was found.
       Available jobs in "build": [redis.docker-redis, redis.docker-nginx]
```

```
Error: project.yml declares disable for "job-test" (provided_by: redis)
       in stage "test", but job "redis.job-test" belongs to stage "qa", not "test".
```

```
Error: Job "docker-php" in stage "build" of project.yml cannot declare
       both "extend" and "replace".
```

```
Error: Job "docker-php" in stage "build" of project.yml cannot declare
       both "extend" and "disable".
```

---

## 9. Security Considerations

### 9.1 Secrets

Jobs can reference secrets using standard GitHub Actions `${{ secrets.* }}` expressions within their `env` or `with` blocks. Secrets must be declared at the workflow level (in the `on.workflow_call.secrets` block if using reusable workflows, or available at the repository/organization level).

The tool does not inspect or validate secret references — they are treated as opaque strings in the passthrough.

### 9.2 Action Pinning

Third-party actions referenced in `uses` should be pinned to a SHA, not a mutable tag. This is a best practice enforced by code review, not by the tool. A future enhancement may introduce an allowlist of approved actions.

### 9.3 Code Injection

The `run` field in steps is passed through as-is. The tool performs no sanitization. Security relies on:

- Repository branch protection rules
- Required PR reviews for changes to `pkg_*.yml`, `base.yml`, and `project.yml`
- Audit logging

---

## 10. Limitations and Trade-offs

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| Max 256 jobs per workflow (GitHub Actions limit) | Large compositions may hit the limit | Consolidate jobs within packages |
| Stage topology is linear only | Cannot express arbitrary DAG dependencies | Sufficient for the vast majority of CI pipelines; DAG support is a possible future enhancement |
| Only `project.yml` can extend/replace/disable | Packages cannot customize other packages | Prevents non-deterministic load order issues; if needed, compose in a third package |
| Generated file must be committed | Risk of drift between source and output | Mitigated by `--diff` CI check |
| Deep merge follows fixed rules | Cannot customize merge behavior per-property | Covers 95% of cases; use `replace` for the remaining 5% |
| `name` property is consumed by the tool | Cannot use raw `name` passthrough for package jobs | Acceptable trade-off for consistent UI display |

---

## 11. Comparison with Alternatives

| Approach | Pros | Cons |
|----------|------|------|
| **This proposal** | Composable, native GHA syntax, readable output, deep merge | Generated file to manage, new concepts to learn |
| Matrix-based (v1 spec) | No generated file, runtime assembly | All jobs share runner/services, opaque at read time |
| Composite Actions | GitHub-native | Step-level only, no job-level composition |
| Workflow templates | GitHub-native | No runtime injection, full workflow copy required |
| Probot / GitHub Apps | Maximum flexibility | Separate infrastructure, operational overhead |

---

## 12. Rollout Plan

### Phase 1: Foundation (Week 1-2)
- [ ] Implement `spark-ci-loader` CLI (PHP / Symfony)
- [ ] Implement base config and package loader with id validation
- [ ] Implement pipeline assembler with deep merge
- [ ] Implement project config with extend/replace/disable
- [ ] Implement display name generation
- [ ] Create JSON Schema for all configuration files
- [ ] Write unit test suite

### Phase 2: Pilot (Week 3-4)
- [ ] Create `pkg_drupal.yml` for existing Drupal pipeline
- [ ] Deploy to 2-3 internal projects
- [ ] Gather feedback on DX and naming conventions
- [ ] Iterate on error messages

### Phase 3: Documentation (Week 5)
- [ ] User-facing documentation with examples
- [ ] Migration guide from GitLab CI
- [ ] Migration guide from v1 spark-ci.yml format

### Phase 4: General Availability (Week 6)
- [ ] Announce to all teams
- [ ] Provide migration support
- [ ] Monitor adoption metrics

---

## 13. Open Questions

1. **Should packages be able to declare dependencies on other packages?** (e.g., `pkg_redis` requires `pkg_drupal` to be present). Currently, packages are independent.
2. **Should the tool support a `--watch` mode** for development that regenerates on file change?
3. **Should we support environment-specific stage overrides?** (e.g., different deploy hooks for staging vs production)
4. **Should the base be able to declare "fixed" jobs** (not overridable by packages or project)?
5. **Should we introduce an `extensible` marker on package jobs** as a contract for which jobs are safe to extend/replace?

---

## 14. References

- [GitHub Actions: Workflow Syntax](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions)
- [GitHub Actions: Reusable Workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows)
- [GitLab CI: extends keyword](https://docs.gitlab.com/ee/ci/yaml/#extends)
- [Kubernetes: Strategic Merge Patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/#use-a-strategic-merge-patch-to-update-a-deployment)
- [JSON Schema Specification](https://json-schema.org/specification.html)

---

## Appendix A: Quick Reference

### Project Operations Summary

```yaml
# project.yml

hooks:
  <stage>:
    # NEW — contribute a job (no directive, no prefix in output)
    my-job:
      runs-on: ubuntu-latest
      steps:
        - run: echo "new job"

    # EXTEND — deep merge into a package job
    <job-id>:
      extend:
        provided_by: <package-id>
      env:                            # ← merged with package env
        MY_VAR: value
      services:                       # ← merged with package services
        redis: { image: redis:7 }
      timeout-minutes: 60             # ← replaces package scalar
      # steps:                        # ← if declared, replaces package steps entirely

    # REPLACE — full substitution of a package job
    <job-id>:
      replace:
        provided_by: <package-id>
      runs-on: ubuntu-latest          # ← nothing from the package survives
      steps:
        - run: echo "replacement"

    # DISABLE — remove a package job
    <job-id>:
      disable:
        provided_by: <package-id>
```

### Deep Merge Rules

| Type | Behavior | Example |
|------|----------|---------|
| Map (env, services, with) | Recursive merge, right wins on conflict | `{A: 1, B: 2}` + `{B: 3, C: 4}` → `{A: 1, B: 3, C: 4}` |
| Sequential array (steps) | Full replacement | `[a, b]` + `[c]` → `[c]` |
| Scalar (runs-on, timeout) | Replacement | `30` + `60` → `60` |

### Display Name Format

| Origin | Format | Example |
|--------|--------|---------|
| Package job with `name` | `[stage] pkg-id · name` | `[build] drupal · Build PHP image` |
| Package job without `name` | `[stage] pkg-id · job-id` | `[build] drupal · docker-nginx` |
| Project job with `name` | `[stage] name` | `[test] Security scan` |
| Project job without `name` | `[stage] job-id` | `[test] custom-lint` |

### Assembly Chain Priority

```
base.yml defaults  →  pkg_*.yml  →  project.yml (extend/replace)
     (lowest)                           (highest)
```

---

*Document end.*