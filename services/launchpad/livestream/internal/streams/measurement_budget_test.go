package streams

import (
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestMeasurementBudgetThresholdsPreserveFractionalSpend(t *testing.T) {
	for _, test := range []struct {
		spent string
		want  []int
	}{
		{"0.399999999999999999", []int{}},
		{"0.4", []int{50}},
		{"0.64", []int{50, 80}},
		{"0.8", []int{50, 80, 100}},
	} {
		t.Run(test.spent, func(t *testing.T) {
			cost, ok := new(big.Rat).SetString(test.spent)
			if !ok {
				t.Fatal("invalid spend")
			}
			got, err := measurementBudgetThresholds(cost, "0.8")
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("thresholds: %v %v, want %v", got, err, test.want)
			}
		})
	}
	for _, value := range []string{"-1", "1/3", "0.0000000000000000001"} {
		cost, ok := new(big.Rat).SetString(value)
		if !ok {
			t.Fatal("invalid spend")
		}
		if _, err := measurementBudgetThresholds(cost, "1"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("unrepresentable spend %s: %v", value, err)
		}
	}
}
