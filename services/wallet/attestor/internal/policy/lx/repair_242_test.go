package lx

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
)

func repair242Complete(t *testing.T, req *SendAuthorizationRequest, change func(*lxwire.Activity, *lxwire.Send)) []byte {
	t.Helper()
	a, err := lxwire.DecodeUnsignedActivity(req.Envelope, mustOperations(t))
	if err != nil {
		t.Fatal(err)
	}
	send, _, err := DecodeSendAuthorization(a)
	if err != nil {
		t.Fatal(err)
	}
	if change != nil {
		change(a, send)
	}
	digest, err := send.AuthorizationDigest()
	if err != nil {
		t.Fatal(err)
	}
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	if !bytes.Equal(key.Public().(ed25519.PublicKey), req.PublicKey[:]) {
		t.Fatal("send producer does not hold the authorization key")
	}
	copy(send.Signature[:], ed25519.Sign(key, digest[:]))
	a.Payload, err = send.Encode()
	if err != nil {
		t.Fatal(err)
	}
	a.PayloadHash = lxwire.PayloadHash(a.Payload)
	envelope, err := lxwire.EncodeUnsignedActivity(a)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func repair242Binding(req *SendAuthorizationRequest) SendDebitBinding {
	return SendDebitBinding{KeyID: "held-key", Account: common.HexToAddress(walletAddr),
		PublicKey: req.PublicKey, NetworkID: 125, AuthorizationDigest: req.Digest}
}

func TestRepair_242_AuthorizationAndSignatureDoNotDebit(t *testing.T) {
	h := newHarness(t)
	req := h.nativeAuthorization(t, 5_000_000)
	decision := h.evaluator.EvaluateSendAuthorization(common.HexToAddress(walletAddr), req, h.request())
	expect(t, decision, policy.CodeAllowed)
	if decision.Spends == nil || len(decision.Spends) != 0 {
		t.Fatalf("authorization must return an empty spend list: %+v", decision.Spends)
	}
	completed := repair242Complete(t, req, nil)
	debit, err := DecodeAuthorizedSendDebit(repair242Binding(req), completed)
	if err != nil {
		t.Fatal(err)
	}
	native := "lx:" + strings.Repeat("00", 32)
	if len(debit.Spends) != 1 || debit.Spends[0].Asset != native || debit.Spends[0].Amount.Cmp(big.NewInt(5_000_000)) != 0 {
		t.Fatalf("decoded debit does not match the producer: %+v", debit.Spends)
	}
	if debit.Binding != repair242Binding(req) || debit.EnvelopeDigest != sha256.Sum256(completed) || debit.ReplayID == "" {
		t.Fatalf("debit lost its held authorization or canonical envelope binding: %+v", debit)
	}
	if got := h.spent(t, walletAddr, native); got.Sign() != 0 {
		t.Fatalf("authorization or decoding a signed send recorded %s before commitment", got)
	}
	h.restart(t)
	if got := h.spent(t, walletAddr, native); got.Sign() != 0 {
		t.Fatalf("restart recovered a debit without committed ingestion: %s", got)
	}
}

func TestRepair_242_RejectsUnboundOrInvalidSends(t *testing.T) {
	h := newHarness(t)
	req := h.nativeAuthorization(t, 5_000_000)
	completed := repair242Complete(t, req, nil)
	base := repair242Binding(req)
	for name, change := range map[string]func(*SendDebitBinding){
		"missing key":     func(b *SendDebitBinding) { b.KeyID = "" },
		"missing account": func(b *SendDebitBinding) { b.Account = common.Address{} },
		"missing network": func(b *SendDebitBinding) { b.NetworkID = 0 },
		"other network":   func(b *SendDebitBinding) { b.NetworkID++ },
		"other key":       func(b *SendDebitBinding) { b.PublicKey[0] ^= 1 },
		"other digest":    func(b *SendDebitBinding) { b.AuthorizationDigest[0] ^= 1 },
	} {
		t.Run(name, func(t *testing.T) {
			binding := base
			change(&binding)
			if debit, err := DecodeAuthorizedSendDebit(binding, completed); err == nil || debit != nil {
				t.Fatalf("accepted unbound signed debit: %+v, %v", debit, err)
			}
		})
	}
	for name, envelope := range map[string][]byte{
		"unsigned authorization": req.Envelope,
		"truncated":              completed[:len(completed)-1],
		"trailing bytes":         append(append([]byte(nil), completed...), 0),
		"different signed amount": repair242Complete(t, req, func(_ *lxwire.Activity, s *lxwire.Send) {
			s.Amount.Lo++
		}),
		"different actor": repair242Complete(t, req, func(a *lxwire.Activity, _ *lxwire.Send) {
			a.ActorDID = []byte(lxwire.DIDFromKey([32]byte{1}))
		}),
		"unowned source": repair242Complete(t, req, func(_ *lxwire.Activity, s *lxwire.Send) {
			s.From[0] ^= 1
			s.Controller = s.From
		}),
		"different envelope network": repair242Complete(t, req, func(a *lxwire.Activity, _ *lxwire.Send) {
			a.NetworkID++
		}),
		"different signed network": repair242Complete(t, req, func(_ *lxwire.Activity, s *lxwire.Send) {
			s.NetworkID++
		}),
		"different context": repair242Complete(t, req, func(_ *lxwire.Activity, s *lxwire.Send) {
			s.SignedContextHash[0] ^= 1
		}),
		"different idempotency key": repair242Complete(t, req, func(_ *lxwire.Activity, s *lxwire.Send) {
			s.IdempotencyKey[0] ^= 1
		}),
		"non-owner authorization": repair242Complete(t, req, func(_ *lxwire.Activity, s *lxwire.Send) {
			s.AuthorizationKind = 2
		}),
	} {
		t.Run(name, func(t *testing.T) {
			if debit, err := DecodeAuthorizedSendDebit(base, envelope); err == nil || debit != nil {
				t.Fatalf("accepted invalid send: %+v, %v", debit, err)
			}
		})
	}
	// Corrupt the authorization while retaining a canonical envelope and payload hash.
	a, err := lxwire.DecodeUnsignedActivity(completed, mustOperations(t))
	if err != nil {
		t.Fatal(err)
	}
	send, err := lxwire.DecodeSend(a.Payload)
	if err != nil {
		t.Fatal(err)
	}
	send.Signature[0] ^= 1
	a.Payload, err = send.Encode()
	if err != nil {
		t.Fatal(err)
	}
	a.PayloadHash = lxwire.PayloadHash(a.Payload)
	corrupted, err := lxwire.EncodeUnsignedActivity(a)
	if err != nil {
		t.Fatal(err)
	}
	if debit, err := DecodeAuthorizedSendDebit(base, corrupted); err == nil || debit != nil {
		t.Fatalf("accepted a corrupt authorization: %+v, %v", debit, err)
	}
	// A valid signature and matching expected digest still cannot debit an
	// account that the key does not own.
	unowned := repair242Complete(t, req, func(_ *lxwire.Activity, s *lxwire.Send) {
		s.From[0] ^= 1
		s.Controller = s.From
	})
	a, err = lxwire.DecodeUnsignedActivity(unowned, mustOperations(t))
	if err != nil {
		t.Fatal(err)
	}
	send, err = lxwire.DecodeSend(a.Payload)
	if err != nil {
		t.Fatal(err)
	}
	base.AuthorizationDigest, err = send.AuthorizationDigest()
	if err != nil {
		t.Fatal(err)
	}
	if debit, err := DecodeAuthorizedSendDebit(base, unowned); err == nil || debit != nil {
		t.Fatalf("accepted a debit from an unowned source: %+v, %v", debit, err)
	}
}

func TestRepair_242_DebitIdentityAndDurableLedgerReplay(t *testing.T) {
	producer := newHarness(t)
	req := producer.nativeAuthorization(t, 5_000_000)
	expect(t, producer.evaluator.EvaluateSendAuthorization(common.HexToAddress(walletAddr), req, producer.request()), policy.CodeAllowed)
	completed := repair242Complete(t, req, nil)
	binding := repair242Binding(req)
	debit, err := DecodeAuthorizedSendDebit(binding, completed)
	if err != nil {
		t.Fatal(err)
	}
	// The send signature does not cover this outer metadata. It cannot create a
	// second rolling debit for the same authorization when a caller resubmits it.
	rewrapped := repair242Complete(t, req, func(a *lxwire.Activity, _ *lxwire.Send) { a.AccountSequence++ })
	other, err := DecodeAuthorizedSendDebit(binding, rewrapped)
	if err != nil {
		t.Fatal(err)
	}
	if other.ReplayID != debit.ReplayID || other.EnvelopeDigest == debit.EnvelopeDigest {
		t.Fatal("resubmission must retain debit identity while binding the distinct envelope")
	}
	nextReq := producer.nativeAuthorization(t, 1)
	next, err := DecodeAuthorizedSendDebit(repair242Binding(nextReq), repair242Complete(t, nextReq, nil))
	if err != nil {
		t.Fatal(err)
	}
	if next.ReplayID == debit.ReplayID {
		t.Fatal("distinct signed intents share a debit identity")
	}
	// These are real independent encrypted ledgers. Applying a debit here proves
	// the storage contract, not the still-required production commitment check.
	for member := 0; member < 3; member++ {
		h := newHarness(t)
		at := h.now
		originalSession := h.ledger.ForRequest("held-key/original-signing-session")
		expect(t, h.evaluator.EvaluateSendAuthorization(binding.Account, req, originalSession), policy.CodeAllowed)
		for _, raw := range [][]byte{completed, completed, rewrapped} {
			received, err := DecodeAuthorizedSendDebit(binding, raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := h.ledger.Apply(policy.AccountKey(binding.Account.Hex()), received.ReplayID, received.Spends, at); err != nil {
				t.Fatal(err)
			}
		}
		h.restart(t)
		originalSession = h.ledger.ForRequest("held-key/original-signing-session")
		recovered, err := DecodeAuthorizedSendDebit(binding, completed)
		if err != nil {
			t.Fatal(err)
		}
		if err := h.ledger.Apply(policy.AccountKey(binding.Account.Hex()), recovered.ReplayID, recovered.Spends, at); err != nil {
			t.Fatal(err)
		}
		native := "lx:" + strings.Repeat("00", 32)
		if got := h.spent(t, walletAddr, native); got.Cmp(big.NewInt(5_000_000)) != 0 {
			t.Fatalf("member %d recovered %s after duplicate delivery", member, got)
		}
		expect(t, h.evaluator.EvaluateSendAuthorization(binding.Account, req, h.request()), policy.CodeDailyCap)
		expect(t, h.evaluator.EvaluateSendAuthorization(binding.Account, req, originalSession), policy.CodeDailyCap)
		h.now = at.Add(policy.SpendWindow + time.Second)
		if err := h.ledger.Apply(policy.AccountKey(binding.Account.Hex()), recovered.ReplayID, recovered.Spends, at); err != nil {
			t.Fatal(err)
		}
		if got := h.spent(t, walletAddr, native); got.Sign() != 0 {
			t.Fatalf("member %d retained %s after its rolling window", member, got)
		}
	}
}
