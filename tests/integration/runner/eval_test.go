package runner

import "testing"

func TestEvalExprOrPropagatesEvaluationError(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    bool
		wantErr string
	}{
		{
			name:    "error before false branch",
			expr:    "missing or 1 == 2",
			wantErr: `no operator in "missing"`,
		},
		{
			name:    "error after false branch",
			expr:    "1 == 2 or missing",
			wantErr: `no operator in "missing"`,
		},
		{
			name:    "first of multiple errors",
			expr:    "missing or 4 / 0 == 1 or later",
			wantErr: `no operator in "missing"`,
		},
		{
			name:    "arithmetic error before syntax error",
			expr:    "1 == 2 or 4 / 0 == 1 or missing",
			wantErr: "division by zero in [4 / 0]",
		},
		{
			name: "true branch after error",
			expr: "missing or 1 == 1",
			want: true,
		},
		{
			name: "true branch short circuits",
			expr: "1 == 1 or missing",
			want: true,
		},
		{
			name: "all branches false without errors",
			expr: "1 == 2 or 2 == 3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalExpr(tt.expr, nil)
			if got != tt.want {
				t.Errorf("evalExpr(%q) = %v, want %v", tt.expr, got, tt.want)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("evalExpr(%q) error = %v, want nil", tt.expr, err)
				}
			} else if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("evalExpr(%q) error = %v, want %q", tt.expr, err, tt.wantErr)
			}
		})
	}
}
