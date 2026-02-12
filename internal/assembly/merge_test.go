package assembly

import (
	"testing"
)

func TestDeepMerge_NilInputs(t *testing.T) {
	if result := DeepMerge(nil, nil); result != nil {
		t.Errorf("DeepMerge(nil, nil) = %v, want nil", result)
	}

	overlay := map[string]any{"a": 1}
	result := DeepMerge(nil, overlay)
	if result["a"] != 1 {
		t.Errorf("DeepMerge(nil, overlay) missing key 'a'")
	}

	base := map[string]any{"b": 2}
	result = DeepMerge(base, nil)
	if result["b"] != 2 {
		t.Errorf("DeepMerge(base, nil) missing key 'b'")
	}
}

func TestDeepMerge_ScalarReplacement(t *testing.T) {
	base := map[string]any{
		"PHP_VERSION": "8.2",
		"APP_ENV":     "test",
	}
	overlay := map[string]any{
		"DATABASE_URL": "postgres://...",
		"APP_ENV":      "prod",
	}

	result := DeepMerge(base, overlay)

	if result["PHP_VERSION"] != "8.2" {
		t.Errorf("expected PHP_VERSION=8.2, got %v", result["PHP_VERSION"])
	}
	if result["APP_ENV"] != "prod" {
		t.Errorf("expected APP_ENV=prod, got %v", result["APP_ENV"])
	}
	if result["DATABASE_URL"] != "postgres://..." {
		t.Errorf("expected DATABASE_URL=postgres://..., got %v", result["DATABASE_URL"])
	}
}

func TestDeepMerge_RecursiveMapMerge(t *testing.T) {
	base := map[string]any{
		"services": map[string]any{
			"mysql": map[string]any{
				"image": "mysql:8",
			},
		},
	}
	overlay := map[string]any{
		"services": map[string]any{
			"redis": map[string]any{
				"image": "redis:7",
			},
		},
	}

	result := DeepMerge(base, overlay)

	services, ok := result["services"].(map[string]any)
	if !ok {
		t.Fatalf("services should be a map")
	}
	if _, ok := services["mysql"]; !ok {
		t.Error("mysql service should be preserved")
	}
	if _, ok := services["redis"]; !ok {
		t.Error("redis service should be added")
	}
}

func TestDeepMerge_ArrayReplacement(t *testing.T) {
	base := map[string]any{
		"steps": []any{
			map[string]any{"run": "echo step1"},
			map[string]any{"run": "echo step2"},
		},
	}
	overlay := map[string]any{
		"steps": []any{
			map[string]any{"run": "echo new-step"},
		},
	}

	result := DeepMerge(base, overlay)

	steps, ok := result["steps"].([]any)
	if !ok {
		t.Fatalf("steps should be a slice")
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step after replacement, got %d", len(steps))
	}
}

func TestDeepMerge_DoesNotMutateInputs(t *testing.T) {
	base := map[string]any{
		"env": map[string]any{
			"A": "1",
		},
	}
	overlay := map[string]any{
		"env": map[string]any{
			"B": "2",
		},
	}

	_ = DeepMerge(base, overlay)

	baseEnv := base["env"].(map[string]any)
	if _, ok := baseEnv["B"]; ok {
		t.Error("DeepMerge should not mutate base")
	}
	overlayEnv := overlay["env"].(map[string]any)
	if _, ok := overlayEnv["A"]; ok {
		t.Error("DeepMerge should not mutate overlay")
	}
}

func TestDeepMerge_NestedConflict(t *testing.T) {
	base := map[string]any{
		"services": map[string]any{
			"postgres": map[string]any{
				"image": "postgres:15",
				"ports": []any{"5432:5432"},
			},
		},
	}
	overlay := map[string]any{
		"services": map[string]any{
			"postgres": map[string]any{
				"image": "postgres:16",
			},
		},
	}

	result := DeepMerge(base, overlay)

	services := result["services"].(map[string]any)
	postgres := services["postgres"].(map[string]any)

	if postgres["image"] != "postgres:16" {
		t.Errorf("expected postgres image=16, got %v", postgres["image"])
	}
	// ports should be preserved from base (overlay didn't specify it).
	if _, ok := postgres["ports"]; !ok {
		t.Error("ports should be preserved from base")
	}
}
