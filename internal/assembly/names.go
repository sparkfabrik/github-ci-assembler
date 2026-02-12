package assembly

import (
	"fmt"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

// GenerateDisplayNames computes the name property for each job in the output workflow.
//
// Format:
//   - Package job with name:    [stage] pkg-id · name
//   - Package job without name: [stage] pkg-id · job-id
//   - Project job with name:    [stage] name
//   - Project job without name: [stage] job-id
func GenerateDisplayNames(jobs []*config.AssembledJob) {
	for _, j := range jobs {
		if j.Disabled {
			continue
		}

		if j.PackageID != "" {
			// Package job (or project extend/replace — they keep the package origin).
			humanPart := j.OriginalJobID
			if j.SourceName != "" {
				humanPart = j.SourceName
			}
			j.DisplayName = fmt.Sprintf("[%s] %s · %s", j.Stage, j.PackageID, humanPart)
		} else {
			// Project new job.
			humanPart := j.OriginalJobID
			if j.SourceName != "" {
				humanPart = j.SourceName
			}
			j.DisplayName = fmt.Sprintf("[%s] %s", j.Stage, humanPart)
		}
	}
}
