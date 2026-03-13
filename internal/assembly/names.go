package assembly

import (
	"fmt"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

// GenerateDisplayNames computes the name property for each job in the output workflow.
//
// Format:
//   - Package job with name:    name - pkg-id [stage]
//   - Package job without name: job-id - pkg-id [stage]
//   - Project job with name:    name [stage]
//   - Project job without name: job-id [stage]
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
			j.DisplayName = fmt.Sprintf("%s - %s [%s]", humanPart, j.PackageID, j.Stage)
		} else {
			// Project new job.
			humanPart := j.OriginalJobID
			if j.SourceName != "" {
				humanPart = j.SourceName
			}
			j.DisplayName = fmt.Sprintf("%s [%s]", humanPart, j.Stage)
		}
	}
}
