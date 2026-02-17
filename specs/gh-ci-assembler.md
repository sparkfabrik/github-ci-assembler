# GitHub CI Assembler Extensibility Specification

**Version:** 2.1.1-draft
**Author:** Platform Team
**Status:** Draft
**Last Updated:** 2026-02-11

---

## 1. Executive Summary

This specification defines a composable CI/CD pipeline system for SparkFabrik's GitHub Actions workflows. Instead of a monolithic workflow per project, the pipeline is assembled from multiple **packages** (`pkg_*.yml`), each contributing jobs to predefined stages and optionally defining workflow-level properties (name, triggers, defaults, environment variables). A **configuration file** defines the stage topology, and an optional **project configuration** allows per-project customizations through three explicit operations: extend, replace, and disable.

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
| ---- | ------------- | ---------- |
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
configuration.yml          Defines stage topology (structural only)
    │
    ▼
pkg_*.yml         Each package contributes jobs
    │             and optional file-scoped env merged into package jobs
    ▼
project.yml       Per-project customizations: extend, replace, disable, new jobs,
    │             and optional file-scoped env merged into project-defined jobs
    ▼
gh-ci-assembler          CLI tool assembles all layers and generates:
generate
    │
    ▼
.github/workflows/gh-ci-assembler.yml    Standard GitHub Actions workflow (committed)
```

### 3.2 Components

| Component | Location | Responsibility |
| ----------- | ---------- | ---------------- |
| Configuration | `configuration.yml` | Stage topology, schema version, and workflow root keys (`name`, `on`, `defaults`) |
| Packages | `pkg_*.yml` | Technology-specific jobs and optional file-scoped job `env` defaults |
| Project Config | `project.yml` | Per-project customizations and optional file-scoped job `env` defaults (optional) |
| CLI Tool | `sparkfabrik/github-ci-assembler` | Assembles layers, generates workflow YAML |

### 3.3 Key Design Principles

**Native GitHub Actions below job level.** The `hooks.<stage>.<job_id>` boundary is the contract line. Everything above it (`hooks`, stage names) is our orchestration layer. Everything below it (`runs-on`, `steps`, `services`, `env`, `container`, `permissions`, etc.) is passed through as-is to the generated workflow. The tool never interprets or transforms job-level properties.

**Explicit package identity.** Every package declares an `id` field — a short, stable identifier that serves as the job prefix and as the reference for `provided_by` in the project configuration. The `id` is the package's contract: renaming the file does not change the package's identity. The loader validates that all package ids are unique across the composition and fails fast with an actionable error if duplicates are detected.

**Automatic job prefixing.** Every job id from a package is automatically prefixed with both the stage ID and the package ID using `--` as separator. `docker-php` declared in stage `build` in a package with `id: drupal` becomes `build--drupal--docker-php` in the generated workflow. This eliminates collision risk by construction — no coordination between package authors required. The `--` separator is used instead of `.` because dots are not allowed in GitHub Actions job identifiers.

**Project jobs are not prefixed.** Jobs contributed directly by `project.yml` (new jobs, not extending or replacing) appear with their original id. They are first-class citizens in the output, visually distinct from package-provided jobs.

**Automatic job display name.** The tool generates a `name` property for every job to provide clear, hierarchical display in the GitHub Actions UI. The format uses bracket notation to show the stage, package origin, and job purpose at a glance. See section 6.2 for the complete naming rules.

**Linear stage topology.** Stages are processed sequentially in the exact order declared in `configuration.yml`. Every job in stage N depends on all jobs in stage N-1. Empty stages are skipped transparently.

---

## 4. Configuration Format

### 4.1 configuration.yml

Defines the pipeline skeleton and workflow root keys.

```yaml
version: "1"

name: <workflow-display-name>         # optional
on:                                   # optional, map form only
  <event>: { ... }
defaults:                             # optional
  run:
    shell: <shell>
    working-directory: <dir>

stages:
  - build
  - test
  - deploy
```

| Field | Required | Description |
| ------- | ---------- | ------------- |
| `version` | Yes | Schema version (currently `"1"`) |
| `name` | No | Workflow display name |
| `on` | No | Workflow triggers (map form only) |
| `defaults` | No | Workflow-level defaults |
| `stages` | Yes | Ordered list of stage names |

`configuration.yml` is the only file allowed to define workflow root keys (`name`, `on`, `defaults`). Root-level `env` is not supported.

### 4.2 pkg_*.yml (Packages)

Each package file contributes jobs to one or more stages. Package files are specified explicitly via repeated `--pkg` command-line switches, which also determines their loading order. The filename pattern `pkg_*.yml` is a naming convention, not a discovery mechanism. The package's identity is determined exclusively by the `id` field declared inside the file.

#### 4.2.1 Package Identity

Every package must declare an `id` at the top level. This `id`:

- Is used as the prefix for all job ids contributed by the package
- Is the value referenced by `provided_by` in `project.yml`
- Must be unique across all packages in the composition
- Must match the pattern `[a-z0-9][a-z0-9_-]*` (lowercase alphanumeric with hyphens)

The filename has no bearing on the package's identity. Renaming `pkg_drupal.yml` to `pkg_drupal-cms.yml` has no effect on the package id, the generated job ids, or any `project.yml` references.

**Uniqueness validation.** Immediately after discovery, the loader collects all `id` values and verifies they are unique. If two or more files declare the same `id`, the loader fails with an error listing all conflicting files. This check runs before any hook or job validation.

| `id` | Filename (example) | Job prefix (in stage `build`) |
| ------ | -------------------- | ------------------------------- |
| `drupal` | `pkg_drupal.yml` | `build--drupal--` |
| `redis` | `pkg_redis.yml` | `build--redis--` |
| `my-cache` | `pkg_cache.yml` | `build--my-cache--` |

#### 4.2.2 Package Format

```yaml
id: <package-id>

# File-scoped env defaults (optional, merged into each package job env)
env:
  <KEY>: <value>

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
| ------- | ---------- | ------------- |
| `id` | Yes | Unique package identifier, used as job prefix and `provided_by` target |
| `env` | No | File-scoped env defaults merged into each job in the package (job `env` wins on conflict) |
| `hooks` | Yes | Map of stage → job_id → job definition (native GHA syntax) |

**Forbidden root keys in packages:** `name`, `on`, and `defaults` are invalid in package files and must raise a validation error.

#### 4.2.3 Package Examples

**pkg_base.yml** (base package — defines workflow triggers, defaults, name, and a placeholder job):

```yaml
id: base

hooks:
  build:
    placeholder:
      runs-on: ubuntu-latest
      steps:
        - name: Placeholder
          run: echo "Base package placeholder"
```

**pkg_drupal.yml:**

```yaml
id: drupal

env:
  PHP_VERSION: "8.2"

hooks:
  build:
    docker-php:
      name: Build PHP image
      runs-on: ubuntu-latest
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

```

**pkg_redis.yml:**

```yaml
id: redis

env:
  REDIS_VERSION: "7"

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

- `id` is required and must match `[a-z0-9][a-z0-9_-]*`
- `id` must be unique across all packages (fail-fast check)
- `id` must not contain `--` (the double-dash sequence is reserved as a separator)
- Root-level `name`, `on`, and `defaults` are forbidden in package files (they are only valid in `configuration.yml`)
- `env`, if present, must be a map
- Stage names must exist in `configuration.yml`
- Job ids must match `[a-z0-9][a-z0-9_-]*` and **must not contain `--`** (the double-dash sequence is reserved as the stage-id/package-id/job-id separator in generated job identifiers)
- All properties below `<job-id>` must be valid GitHub Actions job syntax

### 4.3 project.yml (Project Configuration)

The project configuration is the **last element in the assembly chain**. It serves two purposes:

1. **File-scoped env defaults:** The project file may declare top-level `env`, which is merged into the `env` map of every project-defined job (`new`, `extend`, `replace`) with lower priority than that job's own `env`.

2. **Job customizations:** The project file can customize package-provided jobs through the `hooks` section, using four types of declarations:

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

**Validation rules for new project jobs:**

- Job ids must match `[a-z0-9][a-z0-9_-]*` and **must not contain `--`** (the double-dash sequence is reserved as a separator)
- All properties below `<job-id>` must be valid GitHub Actions job syntax

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
| --------------- | ---------------- |
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
| ---------- | ----------- | ------------- |
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

#### 4.3.5 Stage References (Explicit Only)

There are no virtual stages. A stage name used in `pkg_*.yml` or `project.yml` must match a stage explicitly declared in `configuration.yml`.

If you want pre/post behavior, declare those stages explicitly in `configuration.yml`, for example:

```yaml
stages:
  - pre-build
  - build
  - post-build
  - test
  - deploy
```

#### 4.3.6 Complete project.yml Example

```yaml
# project.yml
env:
  DEPLOY_ENV: staging
  SLACK_WEBHOOK: "https://hooks.slack.com/services/..."

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

  notify:
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

The `gh-ci-assembler` CLI tool processes the configuration in the following order:

```
Phase 1: Load configuration
  → Parse configuration.yml
  → Extract workflow root keys (name, on, defaults)
  → Extract stage topology

Phase 2: Load packages
  → Load package files in the order specified by --pkg switches
  → Parse each file and extract the id field
  → Validate id uniqueness across all packages (fail fast)
  → Validate hooks and job definitions against configuration stages
  → Validate forbidden package root keys (name, on, defaults)
  → Merge package-level env into each package job env (job env wins)
  → Prefix all job ids with stage ID and package ID using -- separator
  → Resolve explicit needs (output job IDs)

Phase 3: Initialize workflow properties
  → Use configuration.yml root keys only (name, on, defaults)

Phase 4: Apply project configuration
  → Load project.yml (if present)
  → Validate forbidden project root keys (name, on, defaults)
  → Merge project-level env into each project-defined job env (job env wins)
  → Apply disable operations (remove target jobs)
  → Apply replace operations (substitute target jobs)
  → Apply extend operations (deep merge into target jobs)
  → Add new project jobs (unprefixed)
  → Resolve explicit needs (output job IDs)

Phase 5: Resolve needs chains
  → Identify active stages (stages with at least one job)
  → For each job in stage N, set needs = [all job ids in stage N-1]
  → Merge computed needs with any explicit needs declared in source definitions
  → Skip empty stages transparently

Phase 6: Generate display names
  → Compute the name property for each job

Phase 7: Render
  → Emit workflow-level properties (name, on, defaults) as root-level keys
  → Generate valid GitHub Actions workflow YAML
  → Write to output path
```

### 5.2 Automatic `needs` Computation

The `needs` chain is derived mechanically from the stage topology. Given stages `[build, qa, test, deploy]` and the following pkg files:

```yaml
id: drupal

hooks:
  build:
    docker-php:
      # ...
    docker-nginx:
      # ...

  qa:
    notify:
      # ...
```

```yaml
id: redis

hooks:
  build:
    docker-redis:
      # ...
  qa:
    notify:
      # ...
  deploy:
    push-redis-image:
      # ...
```

The computed needs are:

| Job ID | needs |
| ----- | ------- |
| `build--drupal--docker-php` | *(none — first stage)* |
| `build--drupal--docker-nginx` | *(none — first stage)* |
| `build--redis--docker-redis` | *(none — first stage)* |
| `qa--drupal--notify` | `[build--drupal--docker-php, build--drupal--docker-nginx, build--redis--docker-redis]` |
| `qa--redis--notify` | `[build--drupal--docker-php, build--drupal--docker-nginx, build--redis--docker-redis]` |
| `deploy--redis--push-redis-image` | `[qa--drupal--notify, qa--redis--notify]` |

Note that `test` is empty, so `deploy` depends on `qa` directly.

### 5.3 Preserving Explicit Dependencies

Jobs may explicitly declare a `needs` keyword in their source definition (in either package files or project files). When present, these explicit dependencies are **preserved and merged** with the automatic stage-based dependencies.

The final `needs` array in the generated workflow contains:

1. **Automatic dependencies:** All computed job IDs from the previous stage (based on linear stage topology)
2. **Explicit dependencies:** All job IDs from source `needs`

**Example:**

```yaml
# pkg_drupal.yml
id: drupal

hooks:
  build:
    docker-php:
      # ...
    docker-nginx:
      # ...
  
  test:
    behat:
      # ...
    phpunit:
      needs: [test--drupal--behat]  # ← output job IDs are preserved as declared
      # ...
```

After assembly, `test--drupal--phpunit` will have:

- Automatic: `[build--drupal--docker-php, build--drupal--docker-nginx, build--redis--docker-redis]` (all jobs from previous stage)
- Explicit: `[test--drupal--behat]` (preserved from source `needs`)
- **Final needs:** `[test--drupal--behat, build--drupal--docker-php, build--drupal--docker-nginx, build--redis--docker-redis]` (merged, duplicates removed)

**Validation rules:**

- Invalid `needs` format (not a string array) is a fatal error.
- Referencing unknown job IDs is a fatal error with the list of allowed job IDs.

### 5.4 Job Name Generation

The tool generates a `name` property for every job in the output workflow. The display name provides hierarchical context visible in the GitHub Actions UI (checks tab, sidebar, PR status).

**Format:**

| Job origin | Has `name` in source | Generated display name |
| ------------ | --------------------- | ---------------------- |
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
[post-build] my-extra-job                ← project job without name (no package prefix)
[test] Security scan                     ← project job with name declared
[deploy] redis · push-redis-image        ← package job without name
```

**Rules:**

- The `name` declared in the source file is never passed through as-is. The tool always wraps it in the bracket notation.
- If a job declares `name` in the source, the tool uses that as the human-readable portion. If not, it uses the `job-id`.
- The `name` property in the source file is consumed by the tool and replaced in the output. It is not a native GitHub Actions passthrough — it is the only exception to the "everything below job-id is native GHA" rule.
- Project jobs that extend or replace a package job retain the package origin in their display name (they appear as `[stage] package-id · ...` since the job id in the output is still prefixed with `stage--package-id--`).

### 5.5 Deep Merge Algorithm

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
# │ AUTO-GENERATED by gh-ci-assembler — do not edit manually                    │
# │ Source: configuration.yml, pkg_base.yml, pkg_drupal.yml, pkg_redis.yml, project.yml│
# │ Generated: 2026-02-10T14:32:00+01:00                                  │
# └──────────────────────────────────────────────────────────────────────┘

name: GitHub CI Assembler
on:
  pull_request:
    branches:
      - main
  push:
    branches:
      - main
      - develop
defaults:
  run:
    shell: bash
jobs:
  #  ── Stage: build ─────────────────────────────────────
  build--base--placeholder:
    name: '[build] base · placeholder'
    runs-on: ubuntu-latest
    steps:
      - name: Placeholder
        run: echo "Base package placeholder"
  build--drupal--docker-nginx:
    name: '[build] drupal · docker-nginx'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build nginx configuration
        run: ./scripts/build-nginx.sh
  build--drupal--docker-php:
    name: '[build] drupal · Build PHP image'
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        ports:
          - 5432:5432
    env:
      DATABASE_URL: postgres://db:5432/myapp
      REDIS_HOST: redis
    steps:
      - uses: actions/checkout@v4
      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: "8.2"
      - name: Install dependencies
        run: composer install --no-interaction
  build--redis--docker-redis:
    name: '[build] redis · Build Redis image'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build custom Redis image
        run: ./scripts/build-redis.sh
  #  ── Stage: notify ────────────────────────────────────
  notify--drupal--notify:
    name: '[notify] drupal · notify'
    needs: [build--base--placeholder, build--drupal--docker-nginx, build--drupal--docker-php, build--redis--docker-redis]
    runs-on: ubuntu-latest
    steps:
      - name: Custom notification
        run: |
          curl -X POST $SLACK_WEBHOOK \
            -d '{"text": "Build complete for ${{ github.repository }}"}'
  notify--redis--notify:
    name: '[notify] redis · notify'
    needs: [build--base--placeholder, build--drupal--docker-nginx, build--drupal--docker-php, build--redis--docker-redis]
    runs-on: ubuntu-latest
    continue-on-error: true
    steps:
      - name: Notify Redis build complete
        run: echo "Redis image built."
  #  ── Stage: test ──────────────────────────────────────
  #  test--redis--job-test: DISABLED by project.yml
  custom-lint:
    name: '[test] custom-lint'
    needs: [notify--drupal--notify, notify--redis--notify]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run project linter
        run: ./scripts/lint.sh
  test--drupal--phpunit:
    name: '[test] drupal · PHPUnit test suite'
    needs: [notify--drupal--notify, notify--redis--notify]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run PHPUnit
        run: vendor/bin/phpunit --coverage-text
  #  ── Stage: deploy ────────────────────────────────────
  deploy--redis--push-redis-image:
    name: '[deploy] redis · push-redis-image'
    needs: [custom-lint, test--drupal--phpunit]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Push Redis image
        uses: docker/build-push-action@v5
        with:
          context: ./docker/redis
          push: "true"
          tags: ghcr.io/myorg/myapp-redis:latest
```

### 6.2 Lifecycle Management

This is somewhat out of scope: the generated workflow file is committed to the repository. The generation will happen by invoking this CLI tool in the same circumstances that happen today with the generation of Gitlab CI files.

---

## 7. CLI Tool

### 7.1 Commands

**Generate:**
```bash
gh-ci-assembler generate \
  --conf=configuration.yml \
  --pkg=pkg_drupal.yml \
  --pkg=pkg_redis.yml \
  --project=project.yml \
  --output=.github/workflows/gh-ci-assembler.yml
```

| Option | Default | Description |
|--------|---------|-------------|
| `--conf`, `-c` | `configuration.yml` | Path to configuration file |
| `--pkg`, `-p` | *(none)* | Path to a package file (repeatable, order matters) |
| `--project` | `project.yml` | Path to project configuration |
| `--output`, `-o` | `.github/workflows/gh-ci-assembler.yml` | Output path |
| `--dry-run` | | Print to stdout without writing |

Packages are loaded in the order they are specified on the command line. This explicit ordering avoids reliance on filesystem or alphabetical ordering and makes the composition deterministic. At least one `--pkg` switch is required.

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
       Package id must match [a-z0-9][a-z0-9_-]* (lowercase, hyphens allowed).
```

```
Error: Package "pkg_bad.yml" (id: bad) references unknown stage "unknown_stage".
       Valid stages: [build, test, deploy]
```

```
Error: Invalid top-level key "on" in pkg_bad.yml (id: bad).
       "on" is only allowed in configuration.yml.
```

```
Error: Invalid job id "my--job" in stage "build" of pkg_bad.yml (id: bad):
       Job id must not contain "--" (reserved as stage-id/package-id/job-id separator).
```

### 8.2 Project Validation Errors

```
Error: project.yml declares extend for "docker-php" (provided_by: drupal)
       in stage "build", but no matching job "build--drupal--docker-php" was found.
       Available jobs in "build": [build--redis--docker-redis, build--drupal--docker-nginx]
```

```
Error: project.yml declares disable for "job-test" (provided_by: redis)
       in stage "test", but job "test--redis--job-test" belongs to stage "qa", not "test".
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

## 9. Limitations and Trade-offs

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| Max 256 jobs per workflow (GitHub Actions limit) | Large compositions may hit the limit | Consolidate jobs within packages |
| Stage topology is linear only | Cannot express arbitrary DAG dependencies | Sufficient for the vast majority of CI pipelines; DAG support is a possible future enhancement |
| Only `project.yml` can extend/replace/disable | Packages cannot customize other packages | Prevents non-deterministic load order issues; if needed, compose in a third package |
| Generated file must be committed | Risk of drift between source and output | Mitigated by regenerating and validating output in CI |
| Deep merge follows fixed rules | Cannot customize merge behavior per-property | Covers 95% of cases; use `replace` for the remaining 5% |
| `name` property is consumed by the tool | Cannot use raw `name` passthrough for package jobs | Acceptable trade-off for consistent UI display |

---

## 10. Comparison with Alternatives

| Approach | Pros | Cons |
|----------|------|------|
| **This proposal** | Composable, native GHA syntax, readable output, deep merge | Generated file to manage, new concepts to learn |
| Matrix-based (v1 spec) | No generated file, runtime assembly | All jobs share runner/services, opaque at read time |
| Composite Actions | GitHub-native | Step-level only, no job-level composition |
| Workflow templates | GitHub-native | No runtime injection, full workflow copy required |
| Probot / GitHub Apps | Maximum flexibility | Separate infrastructure, operational overhead |

---

---

## 11. Open Questions

1. **Should packages be able to declare dependencies on other packages?** (e.g., `pkg_redis` requires `pkg_drupal` to be present). Currently, packages are independent.
2. **Should the tool support a `--watch` mode** for development that regenerates on file change?
3. **Should we support environment-specific stage overrides?** (e.g., different deploy hooks for staging vs production)
4. **Should the configuration file be able to declare "fixed" jobs** (not overridable by packages or project)?
5. **Should we introduce an `extensible` marker on package jobs** as a contract for which jobs are safe to extend/replace?

---

## 12. References

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

# File-scoped env defaults (optional, merged into project-defined job env maps)
env:                                # ← lower priority than each job's own env
  MY_VAR: value

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
      env:                            # ← merged over project-file env and package env
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

    # needs values must be output job IDs (package jobs are prefixed)
    another-job:
      needs: [build--drupal--docker-php]
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
Workflow-level (name, on, defaults):
  configuration.yml only

Job-level (hooks):
  pkg_*.yml  →  project.yml (extend/replace/disable)
  (base)       (highest)
```

---
