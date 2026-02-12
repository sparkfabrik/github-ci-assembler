# GitHub Actions Setup Guide

This document describes the GitHub Actions workflows configured for this project.

## Workflows

### 1. Test Workflow (`.github/workflows/test.yml`)

**Triggers:**
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop` branches

**What it does:**
- Runs tests on Go 1.25.6 and 1.26.x (matrix strategy)
- Checks code coverage
- Verifies the CLI builds
- Optionally uploads coverage to Codecov

**Note:** The `go test` command includes integration tests with golden files, so no additional CLI testing is needed.

**Local equivalent:**
```bash
go test -v -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### 2. Lint Workflow (`.github/workflows/lint.yml`)

**Triggers:**
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop` branches

**What it does:**
- Runs golangci-lint with multiple linters enabled
- Checks code quality, formatting, and potential issues

**Why separate from tests?**
- Test and lint jobs run in **parallel** (faster CI)
- You can see test results even if lint fails (and vice versa)
- Can configure different branch protection requirements

**Local equivalent:**
```bash
# Install golangci-lint first: https://golangci-lint.run/usage/install/
golangci-lint run --timeout=5m
```

**Configuration:** See `.golangci.yml` for enabled linters and settings.

### 3. Release Workflow (`.github/workflows/release.yml`)

**Triggers:**
- Push of version tags (e.g., `v1.0.0`, `v1.2.3`)

**What it does:**
- Runs tests to ensure quality
- Builds binaries for multiple platforms (Linux, macOS, Windows; amd64, arm64)
- Creates GitHub release with changelog
- Uploads binary artifacts

**Configuration:** See `.goreleaser.yml` for build settings.

**To create a release:**
```bash
# 1. Tag a commit
git tag -a v1.0.0 -m "Release v1.0.0"

# 2. Push the tag
git push origin v1.0.0

# The release workflow will automatically trigger
```

## Release Process

### Semantic Versioning

This project follows [Semantic Versioning 2.0.0](https://semver.org/):
- **MAJOR** version (v2.0.0): Breaking changes
- **MINOR** version (v1.1.0): New features, backwards compatible
- **PATCH** version (v1.0.1): Bug fixes, backwards compatible

### Changelog Grouping

The release workflow automatically generates changelogs by grouping commits:
- `feat:` → Features
- `fix:` → Bug fixes
- `perf:` → Performance improvements
- Others → Others section

**Example commit messages:**
```bash
git commit -m "feat: add support for parallel job execution"
git commit -m "fix: correct stage dependency calculation"
git commit -m "docs: update README with new examples"
```

### Creating a Release

1. **Ensure all tests pass:**
   ```bash
   go test ./...
   ```

2. **Update documentation if needed:**
   - Update `README.md` if CLI flags changed
   - Update `AGENTS.md` if architecture changed
   - Update `specs/gh-ci-assembler.md` if behavior changed

3. **Create and push tag:**
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0: Initial stable release"
   git push origin v1.0.0
   ```

4. **Monitor release workflow:**
   - Go to GitHub Actions tab
   - Watch the "Release" workflow
   - Verify binaries are built and uploaded

5. **Edit release notes (optional):**
   - Go to GitHub Releases
   - Edit the auto-generated release
   - Add migration notes or breaking changes if needed

## Configuration Files

### `.goreleaser.yml`

GoReleaser configuration that defines:
- Build targets (OS/architecture combinations)
- Binary naming conventions
- Archive formats
- Changelog generation rules
- Release metadata

**Key features:**
- Cross-platform builds (Linux, macOS, Windows)
- Multi-architecture support (amd64, arm64)
- Automatic changelog generation
- Version information embedded via ldflags

### `.golangci.yml`

Golangci-lint configuration that enables:
- **errcheck**: Unchecked error detection
- **gosimple**: Code simplification suggestions
- **govet**: Suspicious construct detection
- **staticcheck**: Advanced static analysis
- **gosec**: Security issue detection
- **gofmt/goimports**: Formatting checks
- **misspell**: Typo detection
- And more...

**To run locally:**
```bash
golangci-lint run
```

## Version Information

The CLI includes version information embedded at build time:

```bash
gh-ci-assembler --version
# Output: gh-ci-assembler v1.0.0 (commit: abc123, built: 2026-02-12T12:00:00Z)
```

This is implemented via:
- Build-time ldflags in `.goreleaser.yml`
- Version variables in `cmd/gh-ci-assembler/main.go`

## Continuous Integration Best Practices

### Branch Protection Rules (Recommended)

Configure branch protection for `main`:
1. Go to Settings → Branches → Add rule
2. Enable:
   - Require pull request reviews
   - Require status checks to pass (select "Test" and "Lint")
   - Require branches to be up to date

### Required Status Checks

Recommended required checks:
- `Test` workflow (all matrix variations)
- `Lint` workflow

### Codecov Integration (Optional)

To enable coverage tracking:
1. Sign up at https://codecov.io
2. Add repository
3. No token needed for public repos
4. View coverage reports in PRs

## Troubleshooting

### Test workflow fails on Go 1.26.x

This is expected if Go 1.26 introduces breaking changes. Either:
- Fix compatibility issues
- Update matrix to only test supported versions

### Release workflow fails

Common issues:
1. **Tag format wrong**: Must be `vX.Y.Z` (e.g., `v1.0.0`)
2. **Tests fail**: Fix tests before releasing
3. **Permission issues**: Check repository settings → Actions → Workflow permissions

### Lint workflow has too many issues

Run locally first:
```bash
golangci-lint run --fix
```

Some linters can auto-fix issues. Review and commit fixes.

## Future Enhancements

Potential additions:
1. **Dependabot**: Automated dependency updates
2. **Security scanning**: CodeQL or Snyk integration
3. **Benchmark tracking**: Performance regression detection
4. **Docker images**: Container releases in addition to binaries
5. **Homebrew tap**: Automated formula updates

## Support

For issues with the GitHub Actions workflows, contact the Platform Team or create an issue in the repository.
