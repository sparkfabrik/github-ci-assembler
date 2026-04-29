package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/sparkfabrik/github-ci-assembler/internal/assembly"
	"github.com/sparkfabrik/github-ci-assembler/internal/render"
	"github.com/spf13/cobra"
)

// Version information set by goreleaser at build time via -ldflags.
// When not set (dev builds, go install without ldflags), these stay at their
// defaults and buildInfo() enriches them from the embedded VCS/module info.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionString returns the full version string, falling back to embedded
// runtime build info when the binary was not built by GoReleaser.
func versionString() string {
	// GoReleaser sets version to a real semver; skip the fallback.
	if version != "dev" {
		return fmt.Sprintf("gh-ci-assembler %s (commit: %s, built: %s)", version, commit, date)
	}

	// Try to read VCS / module info embedded by the Go toolchain.
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Sprintf("gh-ci-assembler %s (commit: %s, built: %s)", version, commit, date)
	}

	// When installed via "go install .../cmd/...@<ref>" the module version
	// reflects the ref (tag or pseudo-version), giving a meaningful string
	// even without ldflags.
	modVersion := info.Main.Version // e.g. "v0.3.1" or "(devel)"

	// Walk the VCS settings embedded by "go build" from a git checkout.
	var vcsRevision, vcsTime, vcsDirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 12 {
				vcsRevision = s.Value[:12]
			} else {
				vcsRevision = s.Value
			}
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				vcsDirty = "-dirty"
			}
		}
	}

	if vcsRevision != "" {
		return fmt.Sprintf("gh-ci-assembler %s (commit: %s%s, built: %s)", modVersion, vcsRevision, vcsDirty, vcsTime)
	}
	// Fallback: module version only (e.g. built from module cache without VCS).
	return fmt.Sprintf("gh-ci-assembler %s", modVersion)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "gh-ci-assembler",
		Short: "Composable CI/CD pipeline assembler for GitHub Actions",
		Long: `gh-ci-assembler assembles modular CI/CD pipeline packages into a standard
GitHub Actions workflow YAML file.`,
		Version: version,
	}

	// Customize version template
	rootCmd.SetVersionTemplate(versionString() + "\n")

	rootCmd.AddCommand(newGenerateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newGenerateCmd() *cobra.Command {
	var (
		confPath    string
		pkgPaths    []string
		projectPath string
		outputPath  string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Assemble packages into a GitHub Actions workflow",
		Long: `Load configuration, packages, and optional project customizations,
then generate a complete GitHub Actions workflow YAML file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(pkgPaths) == 0 {
				return fmt.Errorf("at least one --pkg is required")
			}

			// Check if project file exists when a path is given.
			actualProjectPath := projectPath
			if actualProjectPath != "" {
				if _, err := os.Stat(actualProjectPath); os.IsNotExist(err) {
					actualProjectPath = ""
				}
			}

			assembler := &assembly.Assembler{
				ConfigPath:  confPath,
				PkgPaths:    pkgPaths,
				ProjectPath: actualProjectPath,
			}

			result, err := assembler.Assemble()
			if err != nil {
				return err
			}

			output, err := render.Render(result)
			if err != nil {
				return fmt.Errorf("rendering output: %w", err)
			}

			if dryRun {
				fmt.Print(string(output))
				return nil
			}

			// Ensure output directory exists.
			if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}

			if err := os.WriteFile(outputPath, output, 0o644); err != nil {
				return fmt.Errorf("writing output file %q: %w", outputPath, err)
			}

			fmt.Fprintf(os.Stderr, "Generated %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&confPath, "conf", "c", "configuration.yml", "Path to configuration file")
	cmd.Flags().StringArrayVarP(&pkgPaths, "pkg", "p", nil, "Path to a package file (repeatable, order matters)")
	cmd.Flags().StringVar(&projectPath, "project", "", "Path to project configuration")
	cmd.Flags().StringVarP(&outputPath, "output", "o", ".github/workflows/gh-ci-assembler.yml", "Output path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print to stdout without writing")

	return cmd
}
