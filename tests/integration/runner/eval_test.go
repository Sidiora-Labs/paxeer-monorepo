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

func TestEvalExprPrecedence(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		{"1 == 1 or 2 == 3 and 4 == 5", true},
		{"1 == 2 and 2 == 3 or 4 == 4", true},
		{"1 == 2 or 2 == 2 and 3 == 3", true},
		{"1 == 2 or 2 == 2 and 3 == 4", false},
		{"1 == 2 and 1 / 0 == 1 or 3 == 3", true},
		{"2 + 3 * 4 == 14", true},
		{"2 + 3 * 4 == 20", false},
		{"20 - 3 * 4 + 2 == 10", true},
		{"12 / 3 * 2 == 8", true},
		{"8 - 3 - 2 == 3", true},
		{"3 * -2 + 10 == 4", true},
		{"3 / 2 == 1.5", true},
		{"-3 / 2 == -1.5", true},
		{"0.1 + 0.2 == 0.3", false},
		{"999999999999999999999 + 3 * 4 == 1000000000000000000011", true},
		{`TEXT == "rock and roll or jazz"`, true},
		{`TEXT == 'rock and roll or jazz'`, true},
		{`"+" == "+"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := evalExpr(tt.expr, map[string]string{"TEXT": "rock and roll or jazz"})
			if err != nil || got != tt.want {
				t.Fatalf("evalExpr(%q) = %v, %v; want %v, nil", tt.expr, got, err, tt.want)
			}
		})
	}
}

func TestEvalExprRejectsDanglingOperators(t *testing.T) {
	for _, expr := range []string{
		"4 + == 4", "4 == 4 +", "4 * == 4", "4 == 4 /", "4 - == 4", "4 == 4 *",
		"4 + * 2 == 4", "4 ==", "== 4", "4 == 4 == 4", "4 ** 2 == 16",
		"1 == 1 or", "and 1 == 1", "1 == 1 and", "1 == 1 or or 2 == 2",
		"1 == 1 or 4 + == 4", "1 == 2 and 4 == 4 *", `TEXT == "unterminated`,
	} {
		t.Run(expr, func(t *testing.T) {
			if got, err := evalExpr(expr, nil); err == nil || got {
				t.Fatalf("evalExpr(%q) = %v, %v; want false and error", expr, got, err)
			}
		})
	}
}
