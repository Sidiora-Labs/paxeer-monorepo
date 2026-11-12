package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIdentitySigningKeyBoundaries(t *testing.T) {
	const reference = "GATEWAY_BOUNDARY_SIGNING_KEY"
	t.Setenv(reference, strings.Repeat("s", 48))
	now := time.Now().UTC().Truncate(time.Second)
	key := SigningKey{ID: "owned", SecretReference: reference, ActiveAt: now.Add(-time.Minute), RetireAt: now.Add(20 * time.Minute)}
	for _, tc := range []struct {
		name, issuer, active string
		keys                 []SigningKey
	}{
		{"missing issuer", "", "owned", []SigningKey{key}},
		{"invalid key", "issuer", "owned", []SigningKey{{ID: "", SecretReference: reference, ActiveAt: key.ActiveAt, RetireAt: key.RetireAt}}},
		{"duplicate key", "issuer", "owned", []SigningKey{key, key}},
		{"missing active key", "issuer", "absent", []SigningKey{key}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewIdentityTokens(tc.issuer, "audience", tc.active, tc.keys, EnvironmentIdentitySecrets{}, func() time.Time { return now })
			requireCode(t, err, "FIELD_INVALID")
		})
	}
	tokens, err := NewIdentityTokens("issuer", "audience", "owned", []SigningKey{key}, EnvironmentIdentitySecrets{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	principal := Principal{accountID: uuid.NewString(), sessionID: uuid.NewString()}
	if _, err := tokens.Mint(context.Background(), Principal{}); err == nil {
		t.Fatal("unbound principal received an access token")
	}
	access, err := tokens.Mint(context.Background(), principal)
	if err != nil || access == "" {
		t.Fatalf("bound access token: token=%q err=%v", access, err)
	}
	if _, err := tokens.Verify(context.Background(), strings.Repeat("x", 2049)); err == nil {
		t.Fatal("oversized access token was accepted")
	}
	if _, err := tokens.Verify(context.Background(), "!"+access[1:]); err == nil {
		t.Fatal("invalid token header was accepted")
	}
	claims, err := tokens.Verify(context.Background(), access)
	if err != nil {
		t.Fatal(err)
	}
	claims.AccessID = strings.Repeat("A", 64)
	changed, err := tokens.signClaims(context.Background(), "owned", claims)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Verify(context.Background(), changed); err == nil {
		t.Fatal("noncanonical access identity was accepted")
	}
	short := key
	short.RetireAt = now.Add(30 * time.Second)
	shortLived, err := NewIdentityTokens("issuer", "audience", "owned", []SigningKey{short}, EnvironmentIdentitySecrets{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := shortLived.key(context.Background(), "owned"); err != nil || !shortLived.cache["owned"].until.Equal(short.RetireAt) {
		t.Fatalf("bounded key cache: until=%v err=%v", shortLived.cache["owned"].until, err)
	}
	if _, err := shortLived.Mint(context.Background(), principal); err == nil {
		t.Fatal("access token outlived its signing key")
	}
	future := key
	future.ActiveAt = now.Add(time.Minute)
	future.RetireAt = now.Add(time.Hour)
	inactive, err := NewIdentityTokens("issuer", "audience", "owned", []SigningKey{future}, EnvironmentIdentitySecrets{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	observed := ""
	inactive.SetObserver(func(category string) { observed = category })
	if _, err := inactive.key(context.Background(), "owned"); err == nil || observed != "key_not_active" {
		t.Fatalf("inactive key: observed=%q err=%v", observed, err)
	}
}

func TestIdentityEnvironmentKeyAndJSONBoundaries(t *testing.T) {
	resolver := EnvironmentIdentitySecrets{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver.ResolveIdentityKey(ctx, "GATEWAY_BOUNDARY_SIGNING_KEY"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled key resolution: %v", err)
	}
	if _, err := resolver.ResolveIdentityKey(context.Background(), "lowercase"); err == nil {
		t.Fatal("unbound key reference was accepted")
	}
	t.Setenv("GATEWAY_BOUNDARY_SHORT_KEY", "short")
	if _, err := resolver.ResolveIdentityKey(context.Background(), "GATEWAY_BOUNDARY_SHORT_KEY"); err == nil {
		t.Fatal("short signing key was accepted")
	}
	var shape struct {
		Value string `json:"value"`
	}
	if err := strictJSON([]byte(`["value"]`), &shape); err == nil {
		t.Fatal("array accepted as closed JSON object")
	}
	var mapping map[string]string
	if err := strictJSON([]byte(`{"value":"owned"}`), &mapping); err == nil {
		t.Fatal("untyped destination accepted as closed claims")
	}
}

func TestIdentityRequestShapesPreserveOwnerBounds(t *testing.T) {
	wallet := "0x" + strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, raw string
		dest      any
	}{
		{"origin", `{"wallet":"` + wallet + `","origin":"http://kindle.test"}`, &ChallengeRequest{}},
		{"proof object", `{"challengeId":"` + uuid.NewString() + `","proof":[],"deviceLabel":"owned"}`, &LoginRequest{}},
		{"chain", `{"wallet":"` + wallet + `","chainId":"126","visibility":"private"}`, &WalletLinkChallengeRequest{}},
		{"proof length", `{"linkId":"` + uuid.NewString() + `","reauthProof":"` + strings.Repeat("a", 257) + `"}`, &WalletLinkRemoveRequest{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireCode(t, DecodeIdentityRequest(strings.NewReader(tc.raw), tc.dest), "FIELD_INVALID")
		})
	}
	if err := DecodeIdentityRequest(strings.NewReader(`{}`), &RefreshRequest{}); err != nil {
		t.Fatalf("empty cookie refresh body: %v", err)
	}
	if err := validateWalletProof(WalletProof{Wallet: wallet, Kind: eoaEIP191Method, Signature: "0xzz"}); err == nil {
		t.Fatal("nonhex wallet proof was accepted")
	}
}
