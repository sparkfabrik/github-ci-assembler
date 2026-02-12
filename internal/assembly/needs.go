package assembly

import (
	"sort"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

// ComputeNeeds calculates the needs chains for all assembled jobs based on
// the linear stage topology. Each job in stage N depends on all jobs in stage N-1.
// Explicit needs from source definitions are preserved and merged.
func ComputeNeeds(jobs []*config.AssembledJob, expandedStages []ExpandedStage) {
	// Build index: stage name → list of job IDs in that stage.
	stageJobs := make(map[string][]string)
	for _, j := range jobs {
		if j.Disabled {
			continue
		}
		stageJobs[j.Stage] = append(stageJobs[j.Stage], j.ID)
	}

	// For each stage, find the previous stage that has jobs.
	// Sort job IDs within each stage for deterministic needs arrays.
	for name := range stageJobs {
		sort.Strings(stageJobs[name])
	}

	prevStageJobs := make(map[string][]string)
	var prevJobs []string
	for _, es := range expandedStages {
		if stageJobIDs, ok := stageJobs[es.Name]; ok && len(stageJobIDs) > 0 {
			prevStageJobs[es.Name] = prevJobs
			prevJobs = stageJobIDs
		}
	}

	// Assign needs to each job.
	for _, j := range jobs {
		if j.Disabled {
			continue
		}

		automatic := prevStageJobs[j.Stage]

		// Merge explicit needs with automatic needs, removing duplicates.
		j.ComputedNeeds = mergeNeeds(automatic, j.ExplicitNeeds)
	}
}

// mergeNeeds combines automatic and explicit needs, removing duplicates.
// Explicit needs come first, then automatic needs (order matters for readability).
func mergeNeeds(automatic, explicit []string) []string {
	if len(automatic) == 0 && len(explicit) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(automatic)+len(explicit))
	var result []string

	// Explicit needs first.
	for _, n := range explicit {
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}

	// Automatic needs.
	for _, n := range automatic {
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}

	return result
}
