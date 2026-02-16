package assembly

import (
	"testing"
)

func TestExpandStages_AllPopulated(t *testing.T) {
	stages := []string{"build", "test", "deploy"}
	hasJobs := func(name string) bool { return true }

	expanded := ExpandStages(stages, hasJobs)

	expected := []string{"build", "test", "deploy"}

	if len(expanded) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(expanded))
	}
	for i, es := range expanded {
		if es.Name != expected[i] {
			t.Errorf("stage[%d] = %q, want %q", i, es.Name, expected[i])
		}
	}
}

func TestExpandStages_OnlyRegular(t *testing.T) {
	stages := []string{"build", "test", "deploy"}
	hasJobs := func(name string) bool {
		return name == "build" || name == "test" || name == "deploy"
	}

	expanded := ExpandStages(stages, hasJobs)

	expected := []string{"build", "test", "deploy"}
	if len(expanded) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(expanded))
	}
	for i, es := range expanded {
		if es.Name != expected[i] {
			t.Errorf("stage[%d] = %q, want %q", i, es.Name, expected[i])
		}
	}
}

func TestExpandStages_EmptyStagesSkipped(t *testing.T) {
	stages := []string{"build", "test", "deploy"}
	hasJobs := func(name string) bool {
		return name == "build" || name == "deploy"
	}

	expanded := ExpandStages(stages, hasJobs)

	expected := []string{"build", "deploy"}
	if len(expanded) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(expanded))
	}
}

func TestExpandStages_OnlyConfiguredStages(t *testing.T) {
	stages := []string{"build", "test"}
	hasJobs := func(name string) bool {
		return name == "build" || name == "test"
	}

	expanded := ExpandStages(stages, hasJobs)

	expected := []string{"build", "test"}
	if len(expanded) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(expanded))
	}
	for i, es := range expanded {
		if es.Name != expected[i] {
			t.Errorf("stage[%d] = %q, want %q", i, es.Name, expected[i])
		}
	}
}

func TestExpandStages_StageKinds(t *testing.T) {
	stages := []string{"build"}
	hasJobs := func(name string) bool { return name == "build" }

	expanded := ExpandStages(stages, hasJobs)

	if len(expanded) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(expanded))
	}
	if expanded[0].Kind != StageKindRegular {
		t.Error("build should be StageKindRegular")
	}
}
