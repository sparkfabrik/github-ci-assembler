# Contributing

## Commit and PR conventions

This repository uses [Conventional Commits](https://www.conventionalcommits.org/) to automate changelog generation and releases via [release-please](https://github.com/googleapis/release-please).

**PR titles must follow the conventional commits format.** Since we squash-merge PRs, the PR title becomes the merge commit message. CI will reject PRs with non-conforming titles.

### Format

```
<type>: <description>
```

### Types

- `feat` — A new feature or capability (triggers a **minor** version bump)
- `fix` — A bug fix (triggers a **patch** version bump)
- `docs` — Documentation only changes
- `chore` — Maintenance tasks (dependency updates, CI changes, etc.)
- `refactor` — Code changes that neither fix a bug nor add a feature
- `test` — Adding or updating tests

### Breaking changes

Append `!` after the type to signal a breaking change (triggers a **major** version bump):

```
feat!: redesign assembly pipeline inputs
```

### Examples

```
feat: add validate CLI command
fix: correct needs computation for single-stage pipelines
docs: update AGENTS.md with new design decisions
chore: bump Go version to 1.26
test: add golden file test for permissions merge
```

## Release process

Releases are fully automated via [release-please](https://github.com/googleapis/release-please):

1. Merge PRs to `main` using conventional commit titles
2. release-please automatically creates/updates a **Release PR** that bumps the version and updates `CHANGELOG.md`
3. When the Release PR is merged, release-please creates a **GitHub Release** with a git tag
4. The release triggers **GoReleaser**, which builds cross-platform binaries and attaches them to the release

You do **not** need to manually create tags or releases.
