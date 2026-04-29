package assembly

// ExpandedStage represents a stage in the linear topology.
type ExpandedStage struct {
	// Name is the stage name from configuration.yml.
	Name string
}

// ExpandStages takes the ordered list of stages from configuration.yml and
// returns the ordered active stages (stages with at least one job).
func ExpandStages(stages []string, hasJobs func(stageName string) bool) []ExpandedStage {
	var result []ExpandedStage

	for _, s := range stages {
		if hasJobs(s) {
			result = append(result, ExpandedStage{
				Name: s,
			})
		}
	}

	return result
}
