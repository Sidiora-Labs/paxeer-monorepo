package store

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTradeCursorRejectsInvalidKeysAndRepresentations(t *testing.T) {
	key := []byte(strings.Repeat("a", 32))
	for _, id := range []string{"", "with.dot", "with space", "with\tseparator", strings.Repeat("a", 33)} {
		if keys, err := NewCursorKeys(id, map[string][]byte{id: key}); keys != nil || !errors.Is(err, ErrCursorInvalid) {
			t.Fatalf("key identity %q: keys=%v err=%v", id, keys, err)
		}
	}
	for _, keys := range []map[string][]byte{nil, {"other": key}, {"active": key[:31]}} {
		if got, err := NewCursorKeys("active", keys); got != nil || !errors.Is(err, ErrCursorInvalid) {
			t.Fatalf("active key validation: keys=%v err=%v", got, err)
		}
	}
	keys, err := NewCursorKeys("active", map[string][]byte{"active": key})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	for _, token := range []string{"", "active", "active.a.b", "unknown.a", "active.!", "active." + base64.RawURLEncoding.EncodeToString([]byte("short")), "active." + base64.RawURLEncoding.EncodeToString(make([]byte, 64)), strings.Repeat("a", 2049)} {
		if _, err := keys.open(token, fixtureAddress, "filter", 50, now); !errors.Is(err, ErrCursorInvalid) {
			t.Fatalf("invalid cursor %q: %v", token, err)
		}
	}
	valid := cursorPayload{Version: 1, Chain: "125", Pool: fixtureAddress, FilterDigest: "filter", Size: 50, Issued: now.Unix(), Expiry: now.Unix() + 900, Order: ChainOrder{Height: "1", TransactionIndex: "0", LogIndex: "0"}, Watermark: ReadWatermark{Height: "1", ProjectionRevision: "1", AppliedEvent: "1"}}
	for _, change := range []func(*cursorPayload){
		func(p *cursorPayload) { p.Version = 2 },
		func(p *cursorPayload) { p.Chain = "1" },
		func(p *cursorPayload) { p.Expiry++ },
		func(p *cursorPayload) { p.Issued++; p.Expiry++ },
	} {
		p := valid
		change(&p)
		token, err := keys.seal(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := keys.open(token, fixtureAddress, "filter", 50, now); !errors.Is(err, ErrCursorInvalid) {
			t.Fatalf("invalid cursor claims admitted: %+v err=%v", p, err)
		}
	}
	oversized := valid
	oversized.Pool = strings.Repeat("a", 2048)
	if token, err := keys.seal(oversized); token != "" || !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("oversized cursor: %q %v", token, err)
	}
	invalid := valid
	invalid.Order.Height = "01"
	if token, err := keys.seal(invalid); token != "" || !errors.Is(err, ErrDataIntegrity) {
		t.Fatalf("noncanonical order: %q %v", token, err)
	}
}
