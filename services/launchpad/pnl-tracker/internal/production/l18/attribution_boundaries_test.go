package l18

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAttributionTokenEnvelopeAndPolicyBindings(t *testing.T) {
	ctx := context.Background()
	l := storage(t)
	now := time.Unix(1791590400, 0)
	key := []byte(strings.Repeat("k", 32))
	clock := func() time.Time { return now }
	a, err := NewAttribution(l, key, clock, "campaign", "v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ledger            *Ledger
		key               []byte
		clock             func() time.Time
		campaign, version string
	}{
		{nil, key, clock, "campaign", "v1"}, {l, nil, clock, "campaign", "v1"}, {l, key, nil, "campaign", "v1"}, {l, key, clock, "invalid campaign", "v1"}, {l, key, clock, "campaign", "invalid version"},
	} {
		if _, err = NewAttribution(tc.ledger, tc.key, tc.clock, tc.campaign, tc.version); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		keys   map[string][]byte
		active string
	}{
		{map[string][]byte{"v1": key}, "bad key"}, {nil, "v1"}, {map[string][]byte{"v1": key, "old": nil}, "v1"}, {map[string][]byte{"v1": key, "bad key": key}, "v1"},
	} {
		if _, err = NewRotatingAttribution(l, tc.keys, tc.active, clock, "campaign", "v1"); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if !a.Matches(l, "campaign", "v1") || a.Matches(l, "campaign", "v2") || (*Attribution)(nil).Matches(l, "campaign", "v1") {
		t.Fatal("attribution identity mismatch")
	}
	for _, tc := range [][2]string{{"invalid", "nonce"}, {actorA, "bad nonce"}} {
		if _, err = a.IssueClick(tc[0], tc[1]); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	sign := func(raw []byte) string {
		mac := hmac.New(sha256.New, key)
		mac.Write(raw)
		return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	valid := Click{Campaign: "campaign", Version: "v1", Referrer: actorA, IssuedAt: now.Unix(), Nonce: "nonce"}
	for _, token := range []string{strings.Repeat("x", 1025), "invalid", "!.YQ", "YQ.!", sign([]byte(`{`))} {
		if _, err = a.decode(token); !errors.Is(err, ErrInvalid) {
			t.Fatal(token, err)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*Click)
		want   error
	}{
		{"campaign", func(c *Click) { c.Campaign = "other" }, ErrInvalid}, {"version", func(c *Click) { c.Version = "other" }, ErrInvalid}, {"nonce", func(c *Click) { c.Nonce = "bad nonce" }, ErrInvalid}, {"wallet", func(c *Click) { c.Referrer = "invalid" }, ErrInvalid}, {"key version", func(c *Click) { c.KeyVersion = "v1" }, ErrUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.change(&c)
			raw, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = a.decode(sign(raw)); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	if _, err = ReferralAwardIdentity("invalid campaign", "v1", actorB); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = a.Bind(ctx, "request", "invalid", true, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = a.Bind(ctx, "request", actorB, true, make([]string, 17)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	future, err := a.IssueClick(actorA, "future")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(-time.Hour)
	if _, err = a.Bind(ctx, "request", actorB, true, []string{future}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	first, err := a.IssueClick(actorA, "a-first")
	if err != nil {
		t.Fatal(err)
	}
	last, err := a.IssueClick(actorC, "z-last")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := a.Bind(ctx, "ordered", actorB, true, []string{last, first})
	if err != nil || binding.Referrer != actorA || binding.Nonce != "a-first" {
		t.Fatal(binding, err)
	}
}

func TestIssuedAttributionRejectsUnboundClicksAndConsent(t *testing.T) {
	ctx := context.Background()
	l := storage(t)
	now := time.Unix(1791590400, 0)
	a, err := NewAttribution(l, []byte(strings.Repeat("k", 32)), func() time.Time { return now }, "campaign", "v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range []string{"invalid", actorA} {
		if _, err = a.BindIssued(ctx, actor, uuid.NewString(), false, nil); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if _, err = a.BindIssued(ctx, actorB, uuid.NewString(), true, make([]string, 17)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = a.SetConsent(ctx, "invalid", uuid.NewString(), true); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err = a.SetConsent(ctx, actorA, "invalid", true); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = a.IssueCode(ctx, "invalid", uuid.NewString()); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err = a.Dashboard(ctx, "invalid"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err = a.ClickCode(ctx, "invalid", uuid.NewString(), "code", true); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	for _, code := range []string{"", " code ", strings.Repeat("x", 17)} {
		if _, err = a.ClickCode(ctx, actorB, uuid.NewString(), code, true); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	raw, err := a.IssueCode(ctx, actorA, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	var code ReferralCode
	if err = json.Unmarshal(raw, &code); err != nil {
		t.Fatal(err)
	}
	if _, err = a.ClickCode(ctx, actorA, uuid.NewString(), code.Code, true); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		token string
	}{
		{"malformed", "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err = a.BindIssued(ctx, actorB, uuid.NewString(), true, []string{tc.token}); !errors.Is(err, ErrInvalid) {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct {
		referrer string
		future   bool
		want     error
	}{{actorB, false, ErrUnauthorized}, {actorA, true, ErrUnauthorized}, {actorA, false, ErrUnauthorized}} {
		if tc.future {
			now = now.Add(time.Hour)
		}
		token, err := a.IssueClick(tc.referrer, "unissued")
		if err != nil {
			t.Fatal(err)
		}
		if tc.future {
			now = now.Add(-time.Hour)
		}
		if _, err = a.BindIssued(ctx, actorB, uuid.NewString(), true, []string{token}); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
	var count int
	if err = l.pool.QueryRow(ctx, "SELECT count(*) FROM "+l.schema+".l18_bindings").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
