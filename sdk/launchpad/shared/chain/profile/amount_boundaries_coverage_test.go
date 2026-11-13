package profile

import (
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestAmountScalesAndManifestBindingsAreExact(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		decimals uint8
		want     string
	}{{"1", 6, "0.000001"}, {"1000000", 6, "1"}, {"42", 0, "42"}, {"0", 18, "0"}} {
		actual, err := FormatUnits(tc.raw, tc.decimals)
		if err != nil || actual != tc.want {
			t.Fatalf("formatted amount %q %v", actual, err)
		}
	}
	for _, tc := range []struct {
		raw      string
		decimals uint8
	}{{"", 6}, {"1", 78}, {"-1", 6}} {
		if _, err := FormatUnits(tc.raw, tc.decimals); err == nil {
			t.Fatal("invalid amount formatted")
		}
	}
	for _, tc := range []struct {
		display  string
		decimals uint8
	}{{"1", 78}, {strings.Repeat("1", 158), 0}, {"1.2.3", 6}, {".1", 6}, {"1.", 6}} {
		if _, err := ParseUnits(tc.display, tc.decimals); err == nil {
			t.Fatal("ambiguous scaled amount parsed")
		}
	}
	m := deployment.Current()
	if _, err := BindManifest(m, "endpoints", strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)); err == nil {
		t.Fatal("changed manifest hash bound")
	}
	hash, err := m.Hash()
	if err != nil {
		t.Fatal(err)
	}
	m.ChainID = 1
	if _, err := BindManifest(m, "endpoints", hash, strings.Repeat("b", 64), strings.Repeat("c", 64)); err == nil {
		t.Fatal("foreign chain manifest bound")
	}
}
