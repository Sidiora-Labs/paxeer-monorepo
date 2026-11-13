package holdings

import (
	"errors"
	"testing"
)

func TestTransferMutationRejectsInvalidAndOverflowingState(t *testing.T) {
	base := event(addr(1), addr(2), addr(3), "1", 1)
	for _, tc := range []struct {
		name, supply, from, to string
		edit                   func(*Transfer)
		integrity              bool
	}{
		{"invalid transfer", "1", "1", "0", func(e *Transfer) { e.Token = "invalid" }, false},
		{"invalid supply", "invalid", "1", "0", nil, false},
		{"invalid sender balance", "1", "invalid", "0", nil, false},
		{"invalid recipient balance", "1", "1", "invalid", nil, false},
		{"recipient overflow", maxUint.String(), "1", maxUint.String(), nil, true},
		{"mint overflow", maxUint.String(), "0", "0", func(e *Transfer) { e.From = Zero }, true},
		{"burn supply underflow", "0", "1", "0", func(e *Transfer) { e.To = Zero }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			if tc.edit != nil {
				tc.edit(&e)
			}
			s, f, to, err := Mutation(e, tc.supply, tc.from, tc.to)
			if err == nil || s != "" || f != "" || to != "" {
				t.Fatalf("invalid state mutated: %s %s %s %v", s, f, to, err)
			}
			if tc.integrity && !errors.Is(err, ErrIntegrity) {
				t.Fatalf("overflow classification: %v", err)
			}
		})
	}
}
