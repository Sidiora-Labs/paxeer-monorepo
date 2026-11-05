package types

import (
	"bytes"
	"math"
	"testing"
)

func TestParamsDefaultAndValidate(t *testing.T) {
	params := DefaultParams()
	if params.Paused {
		t.Fatal("default params are paused")
	}
	if params.IdempotencyRetention != 0 {
		t.Fatalf("default retention = %d, want 0", params.IdempotencyRetention)
	}
	if err := params.Validate(); err != nil {
		t.Fatalf("default params: %v", err)
	}
	for _, p := range []Params{
		{Paused: true},
		{IdempotencyRetention: 1},
		{Paused: true, IdempotencyRetention: math.MaxUint64},
	} {
		if err := p.Validate(); err != nil {
			t.Fatalf("params %+v: %v", p, err)
		}
	}
}

func TestParamsStoreKeyLayout(t *testing.T) {
	if ModuleName != "layerxauth" || StoreKey != ModuleName || RouterKey != ModuleName || QuerierRoute != ModuleName {
		t.Fatal("module names differ from layerxauth")
	}
	if !bytes.Equal(ParamsKey, []byte{0x01}) {
		t.Fatalf("ParamsKey = %x, want 01", ParamsKey)
	}
}
