package account

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCompactEndpointProofBindsOwnerTokenAndExpiry(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	now := time.Unix(1760000000, 0)
	token := "owned-push-endpoint"
	digest := sha256.Sum256([]byte(token))
	wire := make([]byte, 73)
	wire[0] = 1
	copy(wire[1:17], id[:])
	copy(wire[17:49], digest[:])
	wire[49] = 1
	binary.BigEndian.PutUint64(wire[65:73], uint64(now.Add(time.Minute).Unix()))
	proof := func(body []byte) string {
		signed := append(append([]byte(nil), body...), ed25519.Sign(private, body)...)
		return base64.RawURLEncoding.EncodeToString(signed)
	}
	verify := CompactEndpointProofVerifier(public)
	if err := verify(context.Background(), id, token, proof(wire), now); err != nil {
		t.Fatalf("bound endpoint proof rejected: %v", err)
	}
	if err := verify(context.Background(), uuid.New(), token, proof(wire), now); !errors.Is(err, ErrDenied) {
		t.Fatalf("proof accepted for another owner: %v", err)
	}
	if err := verify(context.Background(), id, "another-endpoint", proof(wire), now); !errors.Is(err, ErrDenied) {
		t.Fatalf("proof accepted for another token: %v", err)
	}
	if err := verify(context.Background(), id, token, proof(wire), now.Add(2*time.Minute)); !errors.Is(err, ErrDenied) {
		t.Fatalf("expired endpoint proof accepted: %v", err)
	}
	withoutNonce := append([]byte(nil), wire...)
	withoutNonce[49] = 0
	if err := verify(context.Background(), id, token, proof(withoutNonce), now); !errors.Is(err, ErrDenied) {
		t.Fatalf("proof without a nonce accepted: %v", err)
	}
	if err := verify(context.Background(), id, token, proof(wire)[:20], now); !errors.Is(err, ErrDenied) {
		t.Fatalf("truncated endpoint proof accepted: %v", err)
	}
}
