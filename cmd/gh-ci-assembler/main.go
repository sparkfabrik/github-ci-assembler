package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sparkfabrik/github-ci-assembler/internal/assembly"
	"github.com/sparkfabrik/github-ci-assembler/internal/render"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gh-ci-assembler",
		Short: "Composable CI/CD pipeline assembler for GitHub Actions",
		Long: `gh-ci-assembler assembles modular CI/CD pipeline packages into a standard
GitHub Actions workflow YAML file.`,
	}

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
			if err := os.MkdirAll(outputDir(outputPath), 0o755); err != nil {
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

// outputDir extracts the directory part of a file path.
func outputDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
