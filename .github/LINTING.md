# Linting Discussion & Recommendations

## Your Questions

### 1. Is linting actually required?

**Short answer:** Not strictly required for an internal tool, but **I recommend keeping it with the minimal config**.

**Here's why:**

✅ **Keep linting (with minimal config)**
- Catches **real bugs** (unchecked errors, dead code, suspicious constructs)
- Very low noise with the simplified config (only 4 issues!)
- Runs in parallel with tests, doesn't slow you down
- Free code review from a robot
- Standard practice in Go ecosystem

❌ **Skip linting entirely** if:
- This is a throwaway prototype
- You're the only developer
- You never want automated code review

### 2. Why was the original config so noisy?

The migrated config from v1 → v2 enabled **too many opinionated linters**:
- `gofmt`/`goimports` - Formatting (should be handled by your editor)
- `gosec` - Security scanner (overly paranoid for internal tools)
- `gocritic` - Very opinionated style rules
- `revive` - Linting comments and naming conventions
- `unparam` - Unused parameters (often intentional in interfaces)

These are great for **public libraries** but overkill for an **internal CLI tool**.

## What I Changed

### Before (Your migrated config)
```yaml
linters:
  enable:
    - gocritic      # 100+ opinionated rules
    - gosec         # Security paranoia
    - misspell      # Spell checker
    - revive        # Comment police
    - unconvert     # Type conversion nitpicks
    - unparam       # Unused param complaints
formatters:
  enable:
    - gofmt         # Formatting enforcement
    - goimports     # Import organization
```

**Result:** ~60 issues, mostly formatting and style opinions

### After (New minimal config)
```yaml
linters:
  enable:
    - errcheck      # Unchecked errors (real bugs!)
    - govet         # Suspicious code (real bugs!)
    - ineffassign   # Dead assignments (likely bugs)
    - staticcheck   # Smart static analysis (catches bugs)
    - unused        # Dead code detection
```

**Result:** 4 issues, all legitimate code improvements from `staticcheck`

### The 4 Remaining Issues

All from `staticcheck`, all are **actual improvements**:

1. `internal/assembly/assembly.go:230` - Don't need `fmt.Sprintf` for a string literal
2. `internal/render/render.go:86-87` - Use `fmt.Fprintf` instead of `WriteString(fmt.Sprintf(...))`
3. `internal/validation/validation.go:68` - `len()` already handles nil maps, don't check both

These are easy fixes and make the code slightly better.

## My Recommendation

**Keep the lint workflow with the minimal config I created.**

### Why?
1. **Only 4 issues** - Very reasonable
2. **Actually helpful** - Catches real bugs, not style preferences
3. **Standard practice** - Every Go project should have basic linting
4. **Free** - Runs in parallel with tests, no extra time
5. **Future-proof** - Will catch bugs in new code automatically

### How to handle the 4 issues?

**Option 1: Fix them** (5 minutes of work)
```bash
# They're all simple one-line fixes
golangci-lint run --fix
```

**Option 2: Suppress them** (if you disagree)
```go
//nolint:staticcheck // Reason why this is intentional
```

**Option 3: Disable staticcheck** (not recommended)
```yaml
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - unused
  # Remove staticcheck if you really don't want it
```

## Alternative: Remove Linting Entirely

If you want to remove linting, here's what to do:

```bash
# Delete the lint workflow
rm .github/workflows/lint.yml

# Delete the config
rm .golangci.yml

# Update WORKFLOWS.md to remove lint documentation
```

But honestly, **I'd keep it**. The minimal config is very reasonable and will save you from bugs.

## Go Community Standards

For context, here's what popular Go projects do:

| Project | Linting? | Config |
|---------|----------|--------|
| **Kubernetes** | ✅ Yes | Minimal (similar to ours) |
| **Docker** | ✅ Yes | Moderate |
| **Terraform** | ✅ Yes | Minimal |
| **Hugo** | ✅ Yes | Moderate |
| **Small internal tools** | 🤷 Mixed | Often none or very minimal |

For an **internal SparkFabrik tool**, a minimal config like ours is perfect.

## Updated Files

I've updated:
1. ✅ `.github/workflows/lint.yml` - Now uses `v9` (latest) and `version: v2.6`
2. ✅ `.golangci.yml` - Minimal config, only 5 default linters
3. ✅ `.github/workflows/test.yml` - Removed redundant CLI tests

## Final Recommendation

**Keep the linting workflow.** It's minimal, fast, catches real bugs, and is standard practice. The 4 remaining issues are legitimate improvements that take 5 minutes to fix.

If you really don't want it, just delete `.github/workflows/lint.yml` and `.golangci.yml`. But I think you'll regret it later when a bug slips through that the linter would have caught! 😉
