// Package config provides types and parsers for gh-ci-assembler configuration files.
package config

// JobMap is an ordered map of job-id → job definition.
// Job definitions are untyped maps because everything below the job-id level
// is native GitHub Actions passthrough.
type JobMap = map[string]map[string]any

// Configuration represents a parsed configuration.yml file.
// It defines the pipeline skeleton and workflow root properties.
type Configuration struct {
	Version     string         `yaml:"version"`
	Stages      []string       `yaml:"stages"`
	Name        string         `yaml:"name,omitempty"`
	On          map[string]any `yaml:"on,omitempty"`
	Defaults    map[string]any `yaml:"defaults,omitempty"`
	Env         map[string]any `yaml:"env,omitempty"`
	Permissions map[string]any `yaml:"permissions,omitempty"`
}

// Package represents a parsed pkg_*.yml file.
// Each package contributes jobs to one or more stages and may define
// a file-scoped env map merged into each job's env.
type Package struct {
	ID          string            `yaml:"id"`
	Env         map[string]any    `yaml:"env,omitempty"`
	Permissions map[string]any    `yaml:"permissions,omitempty"`
	Hooks       map[string]JobMap `yaml:"hooks"`

	// SourceFile is the path to the file this package was loaded from.
	// Not parsed from YAML; set by the loader.
	SourceFile string `yaml:"-"`
}

// ProvidedByRef identifies the target package for extend/replace/disable operations.
type ProvidedByRef struct {
	ProvidedBy string `yaml:"provided_by"`
}

// ProjectJob represents a single job declaration in project.yml.
// It can be one of four types: new, extend, replace, or disable.
type ProjectJob struct {
	Extend  *ProvidedByRef `yaml:"extend,omitempty"`
	Replace *ProvidedByRef `yaml:"replace,omitempty"`
	Disable *ProvidedByRef `yaml:"disable,omitempty"`

	// Properties holds all remaining key-value pairs (native GHA job syntax).
	// For new/extend/replace jobs, this contains the job definition.
	// For disable jobs, this should be empty.
	Properties map[string]any `yaml:"-"`
}

// Project represents a parsed project.yml file.
type Project struct {
	Env         map[string]any                   `yaml:"env,omitempty"`
	Permissions map[string]any                   `yaml:"permissions,omitempty"`
	Hooks       map[string]map[string]ProjectJob `yaml:"-"`
}

// IsNew returns true if the project job is a new job (no directive).
func (pj *ProjectJob) IsNew() bool {
	return pj.Extend == nil && pj.Replace == nil && pj.Disable == nil
}

// IsExtend returns true if the project job extends a package job.
func (pj *ProjectJob) IsExtend() bool {
	return pj.Extend != nil
}

// IsReplace returns true if the project job replaces a package job.
func (pj *ProjectJob) IsReplace() bool {
	return pj.Replace != nil
}

// IsDisable returns true if the project job disables a package job.
func (pj *ProjectJob) IsDisable() bool {
	return pj.Disable != nil
}

// AssembledJob represents a job in the final assembled workflow.
type AssembledJob struct {
	// ID is the final job ID in the output (e.g., "build--drupal--docker-php" or "custom-lint").
	ID string

	// Stage is the stage this job belongs to.
	Stage string

	// PackageID is the source package ID, or empty for project-contributed jobs.
	PackageID string

	// OriginalJobID is the job ID as declared in the source file (before prefixing).
	OriginalJobID string

	// SourceName is the "name" property from the source definition (consumed by the tool).
	SourceName string

	// Properties contains the job definition (native GHA syntax).
	Properties map[string]any

	// ExplicitNeeds are needs declared in the source definition (before assembly).
	ExplicitNeeds []string

	// ComputedNeeds is the final needs array after assembly.
	ComputedNeeds []string

	// DisplayName is the generated name property for the output.
	DisplayName string

	// Disabled is true if this job was disabled by the project file.
	Disabled bool

	// DisabledComment records the reason for disabling (for output comments).
	DisabledComment string

	// Directive is the project-level operation applied to this job:
	// "extend", "replace", "disable", "new", or "" for pure package jobs.
	Directive string
}

// WorkflowProperties holds the accumulated workflow-level properties.
type WorkflowProperties struct {
	Name        string
	On          map[string]any
	Defaults    map[string]any
	Env         map[string]any
	Permissions map[string]any
}

// SourceFile represents a single input file and its role in the assembly.
type SourceFile struct {
	Kind string // " config", "package", or "project"
	Path string
}

// AssemblyResult holds the complete result of the assembly process.
type AssemblyResult struct {
	Workflow    WorkflowProperties
	Jobs        []*AssembledJob
	SourceFiles []SourceFile // ordered list of all source files with their roles
}
