package graphql

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestRetiredSpotRoots(t *testing.T) {
	schema := newTestSchema(t)
	for _, root := range []string{
		"spotBatchClearings",
		"spotEpochSettlements",
		"spotFastSettlements",
		"spotMarketModeChanges",
		"spotPofqUpdates",
		"spotPLVs",
		"spotKeeperCycles",
		"spotBatchClearFailures",
	} {
		t.Run(root, func(t *testing.T) {
			errors := schema.Validate("{ " + root + " { __typename } }")
			if len(errors) != 1 {
				t.Fatalf("validation errors = %v, want one unknown-field error", errors)
			}
			want := fmt.Sprintf("Cannot query field %q on type %q.", root, "Query")
			if !strings.HasPrefix(errors[0].Message, want) {
				t.Fatalf("validation error = %q, want %q", errors[0].Message, want)
			}
		})
	}

	t.Run("remaining_operations", func(t *testing.T) {
		documents, order := loadOperations(t)
		if want := append(slices.Clone(opOperations), opPerfdb...); !slices.Equal(order, want) {
			t.Fatalf("testdata operations = %v, want %v", order, want)
		}
		cases := operationCases("validation")
		for _, name := range order {
			t.Run(name, func(t *testing.T) {
				tc, ok := cases[name]
				if !ok {
					t.Fatalf("no variables for operation %s", name)
				}
				if errors := schema.ValidateWithVariables(documents[name], tc.vars); len(errors) != 0 {
					t.Fatalf("operation %s rejected: %v", name, errors)
				}
			})
		}
	})
}
