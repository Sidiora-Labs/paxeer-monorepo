package lxwire_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	policylx "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/lx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func TestRepairM_145_AccountParity(t *testing.T) {
	lxwire.TestAccountIDsMatchRustAndChain(t)
}

func repairM145Transport(t *testing.T, send *lxwire.Send, key ed25519.PrivateKey, registry *lxwire.Registry) *lxwire.Activity {
	t.Helper()
	payload, err := send.Encode()
	if err != nil {
		t.Fatal(err)
	}
	activity := &lxwire.Activity{
		ProtocolVersion: send.ProtocolVersion, NetworkID: send.NetworkID,
		Type: policylx.OpAssetTransfer, ActorDID: []byte(lxwire.DIDFromKey(send.PublicKey)),
		Authority: send.PublicKey[:], AccountSequence: send.SourceSequence,
		NotBefore: 1000, NotAfter: send.ExpiresAt, IdempotencyKey: send.IdempotencyKey,
		Payload: payload, PayloadHash: lxwire.PayloadHash(payload), Signed: true,
	}
	preimage, err := lxwire.SignaturePreimage(activity)
	if err != nil {
		t.Fatal(err)
	}
	activity.Signature = ed25519.Sign(key, preimage[:])
	envelope, err := lxwire.EncodeActivity(activity)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := lxwire.DecodeActivity(envelope, registry)
	if err != nil {
		t.Fatal(err)
	}
	transportedPreimage, err := lxwire.SignaturePreimage(decoded)
	if err != nil || !decoded.PayloadHashMatches() || !ed25519.Verify(ed25519.PublicKey(send.PublicKey[:]), transportedPreimage[:], decoded.Signature) {
		t.Fatalf("transport changed signed activity: %v", err)
	}
	return decoded
}

func TestRepairM_145_ProducerTransportPolicy(t *testing.T) {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x45}, ed25519.SeedSize))
	var publicKey [32]byte
	copy(publicKey[:], key.Public().(ed25519.PublicKey))
	from, err := codec.DeriveAccountID([]byte(lxwire.MainAccountName(lxwire.DIDFromKey(publicKey))))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := lxwire.NewRegistry(policylx.OpAssetTransfer)
	if err != nil {
		t.Fatal(err)
	}
	seen := [15]bool{}
	for _, v := range lxwire.AccountParityVectors() {
		if v.Kind == 0 {
			continue
		}
		t.Run(v.Name, func(t *testing.T) {
			to, err := codec.DeriveAccountID([]byte(v.Name))
			if err != nil {
				t.Fatal(err)
			}
			send := &lxwire.Send{
				From: from, To: to, Asset: [32]byte{0x17}, Amount: lxwire.Uint128{Hi: 1, Lo: 17},
				SourceSequence: 3, IdempotencyKey: [32]byte{1}, ExpiresAt: 2000,
				AuthorizationKind: lxwire.OwnerAuthorization, Controller: from, PublicKey: publicKey,
				NetworkID: 125, ProtocolVersion: lxwire.StateCommitmentProtocolVersion,
			}
			digest, err := send.AuthorizationDigest()
			if err != nil {
				t.Fatal(err)
			}
			copy(send.Signature[:], ed25519.Sign(key, digest[:]))
			decoded := repairM145Transport(t, send, key, registry)
			effect, err := policylx.DecodeEffect(decoded)
			if err != nil {
				t.Fatalf("canonical account refused by policy: %v", err)
			}
			recipient, err := lxwire.AccountID([]byte(v.Name))
			if err != nil {
				t.Fatal(err)
			}
			amount := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(17))
			disclosure := policylx.Disclosure{
				Account: policylx.ID(from), Module: "asset", Operation: 5,
				Amounts:      []policylx.Amount{{Asset: policylx.ID(send.Asset), Amount: amount}},
				Destinations: []policylx.ID{recipient}, Sequence: 3, NotBefore: 1000, NotAfter: 2000,
			}
			raw, err := json.Marshal(disclosure)
			if err != nil {
				t.Fatal(err)
			}
			var transported policylx.Disclosure
			if err := json.Unmarshal(raw, &transported); err != nil {
				t.Fatal(err)
			}
			if effect.Account != policylx.ID(from) || len(effect.Destinations) != 1 || effect.Destinations[0] != policylx.ID(to) ||
				len(effect.Legs) != 1 || effect.Legs[0].From != policylx.ID(from) || effect.Legs[0].To != policylx.ID(to) ||
				effect.Legs[0].Asset != policylx.ID(send.Asset) || effect.Legs[0].Amount.Cmp(amount) != 0 ||
				len(effect.Amounts) != 1 || effect.Amounts[0].Asset != policylx.ID(send.Asset) || effect.Amounts[0].Amount.Cmp(amount) != 0 {
				t.Fatalf("policy changed account identifiers or transfer: %+v", effect)
			}
			if err := policylx.MatchDisclosure(decoded, effect, &transported); err != nil {
				t.Fatalf("canonical account disclosure refused: %v", err)
			}
			transported.Destinations[0][0] ^= 1
			if err := policylx.MatchDisclosure(decoded, effect, &transported); err == nil {
				t.Fatal("policy accepted a disclosure naming another recipient")
			}
			send.To[0] ^= 1
			changed := repairM145Transport(t, send, key, registry)
			if _, err := policylx.DecodeEffect(changed); err == nil {
				t.Fatal("policy accepted a send whose recipient changed after authorization")
			}
			seen[v.Kind] = true
		})
	}
	for kind := 1; kind < len(seen); kind++ {
		if !seen[kind] {
			t.Errorf("account kind %d has no successful policy-consumer proof", kind)
		}
	}
}
