// Package assembly implements the gh-ci-assembler pipeline assembly process.
package assembly

// DeepMerge performs a recursive merge of two maps following
// Kubernetes strategic merge patch semantics:
//   - Maps merge recursively (overlay keys win on conflict)
//   - Scalars and sequential arrays: overlay replaces base entirely
//
// Returns a new map; neither base nor overlay are modified.
func DeepMerge(base, overlay map[string]any) map[string]any {
	if base == nil && overlay == nil {
		return nil
	}
	if base == nil {
		return copyMap(overlay)
	}
	if overlay == nil {
		return copyMap(base)
	}

	result := copyMap(base)
	for key, overlayVal := range overlay {
		baseVal, exists := result[key]
		if !exists {
			result[key] = deepCopyValue(overlayVal)
			continue
		}

		baseMap, baseIsMap := baseVal.(map[string]any)
		overlayMap, overlayIsMap := overlayVal.(map[string]any)

		if baseIsMap && overlayIsMap {
			result[key] = DeepMerge(baseMap, overlayMap)
		} else {
			// Scalars and sequential arrays: overlay wins.
			result[key] = deepCopyValue(overlayVal)
		}
	}
	return result
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// deepCopyValue creates a deep copy of a value.
func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		return deepCopySlice(val)
	default:
		return v
	}
}

// deepCopyMap creates a deep copy of a map.
func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

// deepCopySlice creates a deep copy of a slice.
func deepCopySlice(s []any) []any {
	if s == nil {
		return nil
	}
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = deepCopyValue(v)
	}
	return result
}
