package assembly

import (
	"testing"
)

func TestExpandStages_AllPopulated(t *testing.T) {
	stages := []string{"build", "test", "deploy"}
	hasJobs := func(name string) bool { return true }

	expanded := ExpandStages(stages, hasJobs)

	expected := []string{
		"pre-build", "build", "post-build",
		"pre-test", "test", "post-test",
		"pre-deploy", "deploy", "post-deploy",
	}

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

func TestExpandStages_PrePostOnly(t *testing.T) {
	stages := []string{"build", "test"}
	hasJobs := func(name string) bool {
		return name == "pre-build" || name == "build" || name == "post-test"
	}

	expanded := ExpandStages(stages, hasJobs)

	expected := []string{"pre-build", "build", "post-test"}
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
	hasJobs := func(name string) bool { return true }

	expanded := ExpandStages(stages, hasJobs)

	if expanded[0].Kind != StageKindPre {
		t.Error("pre-build should be StageKindPre")
	}
	if expanded[1].Kind != StageKindRegular {
		t.Error("build should be StageKindRegular")
	}
	if expanded[2].Kind != StageKindPost {
		t.Error("post-build should be StageKindPost")
	}
}

func TestParseVirtualStage(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantBase     string
		wantIsPre    bool
		wantIsPost   bool
	}{
		{"regular", "build", "build", false, false},
		{"pre", "pre-build", "build", true, false},
		{"post", "post-build", "build", false, true},
		{"pre with underscore", "pre-post_build", "post_build", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, isPre, isPost := ParseVirtualStage(tt.input)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if isPre != tt.wantIsPre {
				t.Errorf("isPre = %v, want %v", isPre, tt.wantIsPre)
			}
			if isPost != tt.wantIsPost {
				t.Errorf("isPost = %v, want %v", isPost, tt.wantIsPost)
			}
		})
	}
}
