package store

import "testing"

func TestRESTPlanRequiresBoundedIndexedReads(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan map[string]any
		want bool
	}{
		{"index scan", map[string]any{"Node Type": "Index Scan", "Actual Rows": float64(50), "Actual Loops": float64(1)}, true},
		{"index only scan", map[string]any{"Node Type": "Index Only Scan", "Actual Rows": float64(2), "Actual Loops": float64(2)}, true},
		{"sequential scan", map[string]any{"Node Type": "Seq Scan", "Actual Rows": float64(1), "Actual Loops": float64(1)}, false},
		{"too many rows", map[string]any{"Node Type": "Index Scan", "Actual Rows": float64(52), "Actual Loops": float64(1)}, false},
		{"filtered rows", map[string]any{"Node Type": "Index Scan", "Actual Rows": float64(1), "Actual Loops": float64(1), "Rows Removed by Filter": float64(52)}, false},
		{"missing loops", map[string]any{"Node Type": "Index Scan", "Actual Rows": float64(1)}, false},
		{"nested bounded", map[string]any{"Node Type": "Limit", "Actual Rows": float64(2), "Actual Loops": float64(1), "Plans": []any{map[string]any{"Node Type": "Index Scan", "Actual Rows": float64(2), "Actual Loops": float64(1)}}}, true},
		{"nested unbounded", map[string]any{"Node Type": "Limit", "Actual Rows": float64(2), "Actual Loops": float64(1), "Plans": []any{map[string]any{"Node Type": "Seq Scan", "Actual Rows": float64(2), "Actual Loops": float64(1)}}}, false},
		{"no children", map[string]any{"Node Type": "Limit", "Actual Rows": float64(2), "Actual Loops": float64(1)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := boundedRESTPlan(tc.plan, 51); got != tc.want {
				t.Fatalf("bounded plan = %t, want %t", got, tc.want)
			}
		})
	}
}
