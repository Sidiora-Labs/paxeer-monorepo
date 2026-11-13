package store

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMoneyAndPublicJSONContracts(t *testing.T) {
	const token = "0xABCDEF1111111111111111111111111111111111"
	fee, err := PoolCreationFee(token)
	if err != nil || fee.Raw != "1000000" || fee.Decimals != 6 || fee.AssetAddress == nil || *fee.AssetAddress != strings.ToLower(token) || fee.ChainID != "125" {
		t.Fatalf("creation fee: %+v, %v", fee, err)
	}
	if _, err := PoolCreationFee("0x0"); !errors.Is(err, ErrAddressInvalid) {
		t.Fatalf("invalid fee address: %v", err)
	}

	var amount UInt256
	if err := json.Unmarshal([]byte(`"1000000"`), &amount); err != nil || amount != "1000000" {
		t.Fatalf("uint256 wire value: %q, %v", amount, err)
	}
	encoded, err := json.Marshal(amount)
	if err != nil || string(encoded) != `"1000000"` {
		t.Fatalf("uint256 encode: %s, %v", encoded, err)
	}
	for _, raw := range []string{`"01"`, `"-1"`, `1`, `"` + strings.Repeat("9", 78) + `"`} {
		if err := json.Unmarshal([]byte(raw), &amount); !errors.Is(err, ErrDataIntegrity) {
			t.Errorf("accepted invalid uint256 %s: %v", raw, err)
		}
	}
	if _, err := json.Marshal(UInt256("01")); !errors.Is(err, ErrDataIntegrity) {
		t.Errorf("encoded invalid uint256: %v", err)
	}
	for _, raw := range [][]byte{nil, []byte("not-json"), []byte(strings.Repeat("x", PublicJSONMaxBytes+1))} {
		if err := ValidatePublicJSON(raw); !errors.Is(err, ErrDataIntegrity) {
			t.Errorf("accepted invalid public JSON of length %d: %v", len(raw), err)
		}
	}
	if err := ValidatePublicJSON([]byte(`{"poolAddress":"` + token + `"}`)); err != nil {
		t.Fatalf("valid public JSON: %v", err)
	}
}

func TestRESTPlanRequiresBoundedIndexReads(t *testing.T) {
	index := map[string]any{"Node Type": "Index Scan", "Actual Loops": float64(1), "Actual Rows": float64(50)}
	if !boundedRESTPlan(index, 51) {
		t.Fatal("bounded index scan refused")
	}
	appendPlan := map[string]any{
		"Node Type":    "Append",
		"Actual Loops": float64(1),
		"Actual Rows":  float64(50),
		"Plans":        []any{index},
	}
	if !boundedRESTPlan(appendPlan, 51) {
		t.Fatal("bounded child index scan refused")
	}
	for _, plan := range []map[string]any{
		{"Node Type": "Seq Scan", "Actual Loops": float64(1), "Actual Rows": float64(1)},
		{"Node Type": "Index Scan", "Actual Loops": float64(2), "Actual Rows": float64(26)},
		{"Node Type": "Index Scan", "Actual Loops": float64(1), "Actual Rows": float64(1), "Rows Removed by Filter": float64(52)},
		{"Node Type": "Index Scan", "Actual Loops": float64(-1), "Actual Rows": float64(1)},
		{"Node Type": "Append", "Actual Loops": float64(1), "Actual Rows": float64(1)},
		{"Node Type": "Append", "Actual Loops": float64(1), "Actual Rows": float64(1), "Plans": []any{map[string]any{"Node Type": "Seq Scan", "Actual Loops": float64(1), "Actual Rows": float64(1)}}},
	} {
		if boundedRESTPlan(plan, 51) {
			t.Errorf("accepted unbounded plan: %v", plan)
		}
	}
}
