package assembly

import (
	"strings"
)

// StageKind represents the kind of a stage in the expanded topology.
type StageKind int

const (
	StageKindPre     StageKind = iota // Virtual pre-stage
	StageKindRegular                  // Regular stage from configuration
	StageKindPost                     // Virtual post-stage
)

// ExpandedStage represents a stage in the fully expanded linear topology.
type ExpandedStage struct {
	// Name is the stage name (e.g., "pre-build", "build", "post-build").
	Name string
	// Kind indicates whether this is a pre, regular, or post stage.
	Kind StageKind
	// BaseName is the underlying real stage name (e.g., "build" for all three).
	BaseName string
}

// ExpandStages takes the ordered list of stages from configuration.yml and
// returns the fully expanded topology including pre-/post- virtual stages.
// Only stages that have at least one job are included.
//
// For each real stage S, the expansion produces: pre-S, S, post-S.
// The hasJobs function determines which stages to include.
func ExpandStages(stages []string, hasJobs func(stageName string) bool) []ExpandedStage {
	var result []ExpandedStage

	for _, s := range stages {
		preName := "pre-" + s
		postName := "post-" + s

		if hasJobs(preName) {
			result = append(result, ExpandedStage{
				Name:     preName,
				Kind:     StageKindPre,
				BaseName: s,
			})
		}
		if hasJobs(s) {
			result = append(result, ExpandedStage{
				Name:     s,
				Kind:     StageKindRegular,
				BaseName: s,
			})
		}
		if hasJobs(postName) {
			result = append(result, ExpandedStage{
				Name:     postName,
				Kind:     StageKindPost,
				BaseName: s,
			})
		}
	}

	return result
}

// ParseVirtualStage checks if a stage name is a virtual pre-/post- stage
// and returns the base stage name and a boolean indicating if it's virtual.
func ParseVirtualStage(name string) (baseName string, isPre bool, isPost bool) {
	if strings.HasPrefix(name, "pre-") {
		return strings.TrimPrefix(name, "pre-"), true, false
	}
	if strings.HasPrefix(name, "post-") {
		return strings.TrimPrefix(name, "post-"), false, true
	}
	return name, false, false
}
