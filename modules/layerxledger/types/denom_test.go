package types

import (
	"errors"
	"strings"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestMirrorDenom(t *testing.T) {
	if MirrorDenomPrefix != "layerx/" {
		t.Fatalf("mirror denom prefix = %q, want layerx/", MirrorDenomPrefix)
	}
	var ascending, full [32]byte
	for i := range ascending {
		ascending[i] = byte(i)
		full[i] = 0xff
	}
	for _, tc := range []struct {
		name string
		id   [32]byte
		want string
	}{
		{"zero", [32]byte{}, "layerx/" + strings.Repeat("0", 64)},
		{"ascending", ascending, "layerx/000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
		{"full", full, "layerx/" + strings.Repeat("f", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			denom := MirrorDenom(tc.id)
			if denom != tc.want {
				t.Fatalf("mirror denom = %q, want %q", denom, tc.want)
			}
			if len(denom) != 71 {
				t.Fatalf("mirror denom length = %d, want 71", len(denom))
			}
			if err := sdk.ValidateDenom(denom); err != nil {
				t.Fatalf("SDK rejected mirror denom: %v", err)
			}
			got, err := ParseMirrorDenom(denom)
			if err != nil || got != tc.id {
				t.Fatalf("parsed asset = %x, error = %v, want %x", got, err, tc.id)
			}
		})
	}

	for _, tc := range []struct {
		name  string
		denom string
	}{
		{"empty", ""},
		{"missing_prefix", strings.Repeat("a", 64)},
		{"other_prefix", "otherx/" + strings.Repeat("a", 64)},
		{"uppercase_prefix", "LAYERX/" + strings.Repeat("a", 64)},
		{"tokenfactory", "factory/creator/" + strings.Repeat("a", 64)},
		{"63_digits", "layerx/" + strings.Repeat("a", 63)},
		{"65_digits", "layerx/" + strings.Repeat("a", 65)},
		{"uppercase_hex", "layerx/" + strings.Repeat("A", 64)},
		{"mixed_case_hex", "layerx/" + strings.Repeat("a", 63) + "F"},
		{"invalid_hex", "layerx/" + strings.Repeat("a", 63) + "g"},
		{"embedded_slash", "layerx/" + strings.Repeat("a", 63) + "/"},
		{"trailing_newline", "layerx/" + strings.Repeat("a", 63) + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMirrorDenom(tc.denom)
			if !errors.Is(err, ErrNonCanonical) {
				t.Fatalf("parse error = %v, want ErrNonCanonical", err)
			}
			if got != ([32]byte{}) {
				t.Fatalf("invalid denom returned asset ID %x", got)
			}
		})
	}
}
