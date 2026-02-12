package assembly

import (
	"testing"

	"github.com/sparkfabrik/github-ci-assembler/internal/config"
)

func TestGenerateDisplayNames_PackageJobWithName(t *testing.T) {
	jobs := []*config.AssembledJob{
		{Stage: "build", PackageID: "drupal", OriginalJobID: "docker-php", SourceName: "Build PHP image"},
	}

	GenerateDisplayNames(jobs)

	expected := "[build] drupal · Build PHP image"
	if jobs[0].DisplayName != expected {
		t.Errorf("got %q, want %q", jobs[0].DisplayName, expected)
	}
}

func TestGenerateDisplayNames_PackageJobWithoutName(t *testing.T) {
	jobs := []*config.AssembledJob{
		{Stage: "build", PackageID: "drupal", OriginalJobID: "docker-nginx"},
	}

	GenerateDisplayNames(jobs)

	expected := "[build] drupal · docker-nginx"
	if jobs[0].DisplayName != expected {
		t.Errorf("got %q, want %q", jobs[0].DisplayName, expected)
	}
}

func TestGenerateDisplayNames_ProjectJobWithName(t *testing.T) {
	jobs := []*config.AssembledJob{
		{Stage: "test", PackageID: "", OriginalJobID: "security-scan", SourceName: "Security scan"},
	}

	GenerateDisplayNames(jobs)

	expected := "[test] Security scan"
	if jobs[0].DisplayName != expected {
		t.Errorf("got %q, want %q", jobs[0].DisplayName, expected)
	}
}

func TestGenerateDisplayNames_ProjectJobWithoutName(t *testing.T) {
	jobs := []*config.AssembledJob{
		{Stage: "test", PackageID: "", OriginalJobID: "custom-lint"},
	}

	GenerateDisplayNames(jobs)

	expected := "[test] custom-lint"
	if jobs[0].DisplayName != expected {
		t.Errorf("got %q, want %q", jobs[0].DisplayName, expected)
	}
}

func TestGenerateDisplayNames_SkipsDisabled(t *testing.T) {
	jobs := []*config.AssembledJob{
		{Stage: "test", PackageID: "redis", OriginalJobID: "job-test", Disabled: true},
	}

	GenerateDisplayNames(jobs)

	if jobs[0].DisplayName != "" {
		t.Errorf("disabled job should have empty display name, got %q", jobs[0].DisplayName)
	}
}
