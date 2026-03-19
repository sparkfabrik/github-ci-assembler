package assembly

import (
	"testing"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

func TestComputeNeeds_LinearTopology(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--a--j1", Stage: "build"},
		{ID: "build--a--j2", Stage: "build"},
		{ID: "test--b--j1", Stage: "test"},
		{ID: "deploy--c--j1", Stage: "deploy"},
	}

	expanded := []ExpandedStage{
		{Name: "build", Kind: StageKindRegular, BaseName: "build"},
		{Name: "test", Kind: StageKindRegular, BaseName: "test"},
		{Name: "deploy", Kind: StageKindRegular, BaseName: "deploy"},
	}

	ComputeNeeds(jobs, expanded)

	// build jobs have no needs (first stage).
	if len(jobs[0].ComputedNeeds) != 0 {
		t.Errorf("build job should have no needs, got %v", jobs[0].ComputedNeeds)
	}
	if len(jobs[1].ComputedNeeds) != 0 {
		t.Errorf("build job should have no needs, got %v", jobs[1].ComputedNeeds)
	}

	// test jobs depend on all build jobs.
	if len(jobs[2].ComputedNeeds) != 2 {
		t.Fatalf("test job should have 2 needs, got %d: %v", len(jobs[2].ComputedNeeds), jobs[2].ComputedNeeds)
	}

	// deploy jobs depend on all test jobs.
	if len(jobs[3].ComputedNeeds) != 1 {
		t.Fatalf("deploy job should have 1 need, got %d: %v", len(jobs[3].ComputedNeeds), jobs[3].ComputedNeeds)
	}
	if jobs[3].ComputedNeeds[0] != "test--b--j1" {
		t.Errorf("deploy job should need test--b--j1, got %v", jobs[3].ComputedNeeds[0])
	}
}

func TestComputeNeeds_SkipsEmptyStages(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--a--j1", Stage: "build"},
		// test stage is empty
		{ID: "deploy--c--j1", Stage: "deploy"},
	}

	expanded := []ExpandedStage{
		{Name: "build", Kind: StageKindRegular, BaseName: "build"},
		// test is not in expanded because it has no jobs
		{Name: "deploy", Kind: StageKindRegular, BaseName: "deploy"},
	}

	ComputeNeeds(jobs, expanded)

	// deploy should depend directly on build (test skipped).
	if len(jobs[1].ComputedNeeds) != 1 {
		t.Fatalf("deploy job should have 1 need, got %d: %v", len(jobs[1].ComputedNeeds), jobs[1].ComputedNeeds)
	}
	if jobs[1].ComputedNeeds[0] != "build--a--j1" {
		t.Errorf("deploy should need build--a--j1, got %v", jobs[1].ComputedNeeds[0])
	}
}

func TestComputeNeeds_SkipsDisabledJobs(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--a--j1", Stage: "build"},
		{ID: "build--a--j2", Stage: "build", Disabled: true},
		{ID: "test--b--j1", Stage: "test"},
	}

	expanded := []ExpandedStage{
		{Name: "build", Kind: StageKindRegular, BaseName: "build"},
		{Name: "test", Kind: StageKindRegular, BaseName: "test"},
	}

	ComputeNeeds(jobs, expanded)

	// test should only depend on the non-disabled build job.
	if len(jobs[2].ComputedNeeds) != 1 {
		t.Fatalf("test job should have 1 need (disabled excluded), got %d: %v", len(jobs[2].ComputedNeeds), jobs[2].ComputedNeeds)
	}
	if jobs[2].ComputedNeeds[0] != "build--a--j1" {
		t.Errorf("test should need build--a--j1, got %v", jobs[2].ComputedNeeds[0])
	}
}

func TestComputeNeeds_ExplicitNeedsMerged(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--a--j1", Stage: "build"},
		{ID: "build--a--j2", Stage: "build"},
		{ID: "test--a--j1", Stage: "test", ExplicitNeeds: []string{"test--a--j2"}},
		{ID: "test--a--j2", Stage: "test"},
	}

	expanded := []ExpandedStage{
		{Name: "build", Kind: StageKindRegular, BaseName: "build"},
		{Name: "test", Kind: StageKindRegular, BaseName: "test"},
	}

	ComputeNeeds(jobs, expanded)

	// test--a--j1 should have explicit + automatic needs.
	needs := jobs[2].ComputedNeeds
	if len(needs) != 3 {
		t.Fatalf("expected 3 needs, got %d: %v", len(needs), needs)
	}
	// Explicit comes first.
	if needs[0] != "test--a--j2" {
		t.Errorf("first need should be explicit test--a--j2, got %v", needs[0])
	}
}

func TestComputeNeeds_CrossStageExplicitNeeds(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--a--j1", Stage: "build"},
		{ID: "test--b--j1", Stage: "test"},
		{ID: "deploy--c--j1", Stage: "deploy", ExplicitNeeds: []string{"build--a--j1"}},
	}

	expanded := []ExpandedStage{
		{Name: "build", Kind: StageKindRegular, BaseName: "build"},
		{Name: "test", Kind: StageKindRegular, BaseName: "test"},
		{Name: "deploy", Kind: StageKindRegular, BaseName: "deploy"},
	}

	ComputeNeeds(jobs, expanded)

	// deploy should have explicit need (build--a--j1) first, then automatic (test--b--j1).
	needs := jobs[2].ComputedNeeds
	if len(needs) != 2 {
		t.Fatalf("expected 2 needs for deploy job, got %d: %v", len(needs), needs)
	}
	// Explicit comes first.
	if needs[0] != "build--a--j1" {
		t.Errorf("first need should be explicit build--a--j1, got %v", needs[0])
	}
	// Automatic comes second (from the previous stage, test).
	if needs[1] != "test--b--j1" {
		t.Errorf("second need should be automatic test--b--j1, got %v", needs[1])
	}
}

func TestComputeNeeds_SingleStage(t *testing.T) {
	jobs := []*config.AssembledJob{
		{ID: "build--a--j1", Stage: "build"},
		{ID: "build--a--j2", Stage: "build"},
	}

	expanded := []ExpandedStage{
		{Name: "build", Kind: StageKindRegular, BaseName: "build"},
	}

	ComputeNeeds(jobs, expanded)

	for _, j := range jobs {
		if len(j.ComputedNeeds) != 0 {
			t.Errorf("job %q in single-stage pipeline should have no needs, got %v", j.ID, j.ComputedNeeds)
		}
	}
}

func TestMergeNeeds_NilInputs(t *testing.T) {
	// Both nil returns nil.
	if result := mergeNeeds(nil, nil); result != nil {
		t.Errorf("mergeNeeds(nil, nil) = %v, want nil", result)
	}

	// Both empty returns nil.
	if result := mergeNeeds([]string{}, []string{}); result != nil {
		t.Errorf("mergeNeeds([], []) = %v, want nil", result)
	}

	// Only automatic returns automatic.
	result := mergeNeeds([]string{"a", "b"}, nil)
	if len(result) != 2 || result[0] != "a" || result[1] != "b" {
		t.Errorf("mergeNeeds(auto, nil) = %v, want [a b]", result)
	}

	// Only explicit returns explicit.
	result = mergeNeeds(nil, []string{"x", "y"})
	if len(result) != 2 || result[0] != "x" || result[1] != "y" {
		t.Errorf("mergeNeeds(nil, explicit) = %v, want [x y]", result)
	}
}
