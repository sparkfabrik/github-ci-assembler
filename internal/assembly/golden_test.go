package assembly_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sparkfabrik/github-ci-assembler/internal/assembly"
	"github.com/sparkfabrik/github-ci-assembler/internal/render"
)

// fixedTime is a deterministic timestamp for golden file comparison.
var fixedTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func TestGolden_FullExample(t *testing.T) {
	root := findTestdataDir(t)
	dir := filepath.Join(root, "full-example")

	assembler := &assembly.Assembler{
		ConfigPath:  filepath.Join(dir, "configuration.yml"),
		PkgPaths: []string{
			filepath.Join(dir, "pkg_base.yml"),
			filepath.Join(dir, "pkg_drupal.yml"),
			filepath.Join(dir, "pkg_redis.yml"),
		},
		ProjectPath: filepath.Join(dir, "project.yml"),
	}

	result, err := assembler.Assemble()
	if err != nil {
		t.Fatalf("Assemble() failed: %v", err)
	}

	output, err := render.Render(result, render.RenderOptions{GeneratedTime: fixedTime})
	if err != nil {
		t.Fatalf("Render() failed: %v", err)
	}

	goldenPath := filepath.Join(dir, "golden", "expected.yml")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, output, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\nRun with UPDATE_GOLDEN=1 to create it", goldenPath, err)
	}

	if string(output) != string(expected) {
		// Find first differing line for helpful output.
		outputLines := splitLines(string(output))
		expectedLines := splitLines(string(expected))

		maxLines := len(outputLines)
		if len(expectedLines) > maxLines {
			maxLines = len(expectedLines)
		}

		for i := 0; i < maxLines; i++ {
			var got, want string
			if i < len(outputLines) {
				got = outputLines[i]
			} else {
				got = "<missing>"
			}
			if i < len(expectedLines) {
				want = expectedLines[i]
			} else {
				want = "<missing>"
			}
			if got != want {
				t.Errorf("first difference at line %d:\n  got:  %q\n  want: %q", i+1, got, want)
				break
			}
		}

		t.Errorf("output does not match golden file %s\nRun with UPDATE_GOLDEN=1 to update", goldenPath)

		// Write actual output for debugging.
		actualPath := goldenPath + ".actual"
		_ = os.WriteFile(actualPath, output, 0o644)
		t.Logf("Actual output written to: %s", actualPath)
	}
}

// TestGolden_PackagesOnly tests assembly without a project file.
func TestGolden_PackagesOnly(t *testing.T) {
	root := findTestdataDir(t)
	dir := filepath.Join(root, "full-example")

	assembler := &assembly.Assembler{
		ConfigPath: filepath.Join(dir, "configuration.yml"),
		PkgPaths: []string{
			filepath.Join(dir, "pkg_base.yml"),
			filepath.Join(dir, "pkg_drupal.yml"),
			filepath.Join(dir, "pkg_redis.yml"),
		},
		ProjectPath: "", // No project file.
	}

	result, err := assembler.Assemble()
	if err != nil {
		t.Fatalf("Assemble() failed: %v", err)
	}

	output, err := render.Render(result, render.RenderOptions{GeneratedTime: fixedTime})
	if err != nil {
		t.Fatalf("Render() failed: %v", err)
	}

	goldenPath := filepath.Join(dir, "golden", "packages-only.yml")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, output, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\nRun with UPDATE_GOLDEN=1 to create it", goldenPath, err)
	}

	if string(output) != string(expected) {
		outputLines := splitLines(string(output))
		expectedLines := splitLines(string(expected))

		maxLines := len(outputLines)
		if len(expectedLines) > maxLines {
			maxLines = len(expectedLines)
		}

		for i := 0; i < maxLines; i++ {
			var got, want string
			if i < len(outputLines) {
				got = outputLines[i]
			} else {
				got = "<missing>"
			}
			if i < len(expectedLines) {
				want = expectedLines[i]
			} else {
				want = "<missing>"
			}
			if got != want {
				t.Errorf("first difference at line %d:\n  got:  %q\n  want: %q", i+1, got, want)
				break
			}
		}

		t.Errorf("output does not match golden file %s\nRun with UPDATE_GOLDEN=1 to update", goldenPath)
		actualPath := goldenPath + ".actual"
		_ = os.WriteFile(actualPath, output, 0o644)
		t.Logf("Actual output written to: %s", actualPath)
	}
}

// TestGolden_SinglePackage tests assembly with only the base package.
func TestGolden_SinglePackage(t *testing.T) {
	root := findTestdataDir(t)
	dir := filepath.Join(root, "full-example")

	assembler := &assembly.Assembler{
		ConfigPath: filepath.Join(dir, "configuration.yml"),
		PkgPaths: []string{
			filepath.Join(dir, "pkg_base.yml"),
		},
		ProjectPath: "",
	}

	result, err := assembler.Assemble()
	if err != nil {
		t.Fatalf("Assemble() failed: %v", err)
	}

	output, err := render.Render(result, render.RenderOptions{GeneratedTime: fixedTime})
	if err != nil {
		t.Fatalf("Render() failed: %v", err)
	}

	goldenPath := filepath.Join(dir, "golden", "single-package.yml")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, output, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\nRun with UPDATE_GOLDEN=1 to create it", goldenPath, err)
	}

	if string(output) != string(expected) {
		outputLines := splitLines(string(output))
		expectedLines := splitLines(string(expected))

		maxLines := len(outputLines)
		if len(expectedLines) > maxLines {
			maxLines = len(expectedLines)
		}

		for i := 0; i < maxLines; i++ {
			var got, want string
			if i < len(outputLines) {
				got = outputLines[i]
			} else {
				got = "<missing>"
			}
			if i < len(expectedLines) {
				want = expectedLines[i]
			} else {
				want = "<missing>"
			}
			if got != want {
				t.Errorf("first difference at line %d:\n  got:  %q\n  want: %q", i+1, got, want)
				break
			}
		}

		t.Errorf("output does not match golden file %s\nRun with UPDATE_GOLDEN=1 to update", goldenPath)
		actualPath := goldenPath + ".actual"
		_ = os.WriteFile(actualPath, output, 0o644)
		t.Logf("Actual output written to: %s", actualPath)
	}
}

// splitLines splits a string into lines, preserving empty trailing lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// findTestdataDir walks up from the current working directory to find the testdata dir.
func findTestdataDir(t *testing.T) string {
	t.Helper()

	// The tests run from the package directory. testdata is at the repo root.
	// Walk up from the package dir.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}

	for {
		candidate := filepath.Join(dir, "testdata")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find testdata directory walking up from %s", dir)
		}
		dir = parent
	}
}
