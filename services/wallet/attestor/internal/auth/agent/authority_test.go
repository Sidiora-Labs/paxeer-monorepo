package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func sponsoredBindAuthority(t *testing.T, configure func(*AuthorityPrincipal)) (*Authority, string, string, evm.SponsoredBatch) {
	t.Helper()
	nodeKey := make([]byte, 32)
	if _, err := rand.Read(nodeKey); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir(), nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Error(err)
		}
	})
	walletKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	const keyID = "wallet:agent:secp256k1:0"
	if err := st.Put(store.ShareRecord{KeyID: keyID, Curve: store.CurveSecp256k1, PublicKey: crypto.FromECDSAPub(&walletKey.PublicKey), Participants: []string{"node-1"}}, crypto.FromECDSA(walletKey)); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	p := AuthorityPrincipal{
		DID:       "did:matrix:test:" + hex.EncodeToString(publicKey[:8]),
		PublicKey: hex.EncodeToString(publicKey), KeyIDs: []string{keyID},
		Policy:      AgentPolicy{Mode: "trade_only", MaxTx: "100", MaxDaily: "100", MaxApprove: "0", Rate: 10},
		LegacySpent: "0", LegacyWindow: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix(),
	}
	if configure != nil {
		configure(&p)
	}
	a := &Authority{config: AuthorityConfig{Store: st, ChainID: 125, Paymaster: common.HexToAddress("0x5555555555555555555555555555555555555555")}}
	if err := a.install(AuthoritySnapshot{SignedClaims: SignedClaims{ExpiresAt: now.Add(time.Hour).Unix()}, Principals: []AuthorityPrincipal{p}}); err != nil {
		t.Fatal(err)
	}
	bind, _ := evm.PrecompileAddress("addr")
	bindABI, err := evm.PrecompileABI("addr")
	if err != nil {
		t.Fatal(err)
	}
	var did [32]byte
	copy(did[:], publicKey)
	data, err := bindABI.Pack("bindLayerX", did, ed25519.Sign(privateKey, did[:]))
	if err != nil {
		t.Fatal(err)
	}
	batch := evm.SponsoredBatch{
		ChainID: big.NewInt(125), Account: crypto.PubkeyToAddress(walletKey.PublicKey), Nonce: big.NewInt(0),
		Calls: []evm.BatchCall{{To: bind, Value: big.NewInt(0), Data: data}, {To: bind, Value: big.NewInt(0), Data: append([]byte(nil), data...)}},
		Quote: evm.GasQuote{
			Sponsor:        common.HexToAddress("0x2222222222222222222222222222222222222222"),
			Token:          common.HexToAddress("0x3333333333333333333333333333333333333333"),
			MaxTokenAmount: big.NewInt(80), TokenAmount: big.NewInt(60), Deadline: big.NewInt(now.Add(time.Hour).Unix()),
			QuoteNonce: big.NewInt(7), GasCost: big.NewInt(10),
		},
	}
	return a, keyID, "agent:" + p.PublicKey, batch
}

func sponsoredBindClaim(t *testing.T, batch evm.SponsoredBatch) *evm.SponsoredBatchClaim {
	t.Helper()
	digest, err := evm.SponsoredBatchDigest(batch)
	if err != nil {
		t.Fatal(err)
	}
	return &evm.SponsoredBatchClaim{Batch: batch, ClaimedDigest: digest}
}

func TestEvaluateAdmitsSponsoredBatchBindForAgent(t *testing.T) {
	for _, mode := range []string{"trade_only", "full"} {
		t.Run(mode, func(t *testing.T) {
			a, keyID, subject, batch := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) { p.Policy.Mode = mode })
			claim := sponsoredBindClaim(t, batch)
			if err := a.Evaluate(keyID, subject, "bind-1", policy.KindSponsoredBatch, claim); err != nil {
				t.Fatalf("valid sponsored bind: %v", err)
			}
			if err := a.Evaluate(keyID, subject, "bind-1", policy.KindSponsoredBatch, claim); err != nil {
				t.Fatalf("same bind retry: %v", err)
			}
			p := a.byKey[keyID]
			if err := a.config.Store.WithRecord(store.RecordKind("custody-agent-spend"), p.PublicKey, func(raw []byte) error {
				var entries []policyReservation
				if err := strictJSON(raw, &entries); err != nil {
					return err
				}
				if len(entries) != 1 || entries[0].Value != "60" || entries[0].Digest != claim.ClaimedDigest.Hex() {
					t.Fatalf("fee must be counted once for the batch and retry: %+v", entries)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			batch.Nonce = big.NewInt(1)
			if err := a.Evaluate(keyID, subject, "bind-2", policy.KindSponsoredBatch, sponsoredBindClaim(t, batch)); !errors.Is(err, ErrAuthority) {
				t.Fatalf("second fee exceeds persistent daily cap: %v", err)
			}
		})
	}
	for name, configure := range map[string]func(*AuthorityPrincipal){
		"transaction cap": func(p *AuthorityPrincipal) { p.Policy.MaxTx = "59" },
		"daily cap":       func(p *AuthorityPrincipal) { p.LegacySpent = "50" },
		"rate cap":        func(p *AuthorityPrincipal) { p.LegacyRecent = p.Policy.Rate },
		"frozen":          func(p *AuthorityPrincipal) { p.Frozen = true },
		"read only":       func(p *AuthorityPrincipal) { p.Policy.Mode = "read_only" },
	} {
		t.Run(name, func(t *testing.T) {
			a, keyID, subject, batch := sponsoredBindAuthority(t, configure)
			err := a.Evaluate(keyID, subject, "bind", policy.KindSponsoredBatch, sponsoredBindClaim(t, batch))
			want := ErrAuthority
			if name == "frozen" || name == "read only" {
				want = ErrFrozen
			}
			if !errors.Is(err, want) {
				t.Fatalf("%s: got %v, want %v", name, err, want)
			}
		})
	}
}

func TestEvaluateAdmitsPaymasterAuthorizationForAgent(t *testing.T) {
	for _, name := range []string{"paymaster", "other delegate", "unset paymaster", "wrong chain", "wrong digest", "nil view", "wrong view", "other kind", "frozen", "read only"} {
		t.Run(name, func(t *testing.T) {
			a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) {
				if name == "frozen" {
					p.Frozen = true
				}
				if name == "read only" {
					p.Policy.Mode = "read_only"
				}
			})
			claim := &evm.AuthorizationClaim{ChainID: big.NewInt(125), Address: a.config.Paymaster, Nonce: 1}
			if name == "other delegate" {
				claim.Address = common.HexToAddress("0x4444444444444444444444444444444444444444")
			}
			if name == "unset paymaster" {
				a.config.Paymaster = common.Address{}
				claim.Address = common.Address{}
			}
			if name == "wrong chain" {
				claim.ChainID = big.NewInt(1)
			}
			digest, err := evm.AuthorizationDigest(claim.ChainID, claim.Address, claim.Nonce)
			if err != nil {
				t.Fatal(err)
			}
			claim.ClaimedDigest = digest
			if name == "wrong digest" {
				claim.ClaimedDigest[0] ^= 1
			}
			var view any = claim
			kind := policy.KindAuthorization
			if name == "nil view" {
				view = (*evm.AuthorizationClaim)(nil)
			}
			if name == "wrong view" {
				view = &evm.Transaction{}
			}
			if name == "other kind" {
				kind = policy.KindLXBind
			}
			err = a.Evaluate(keyID, subject, "authorization", kind, view)
			if name == "paymaster" {
				if err != nil {
					t.Fatalf("configured paymaster: %v", err)
				}
			} else {
				want := ErrAuthority
				if name == "frozen" || name == "read only" {
					want = ErrFrozen
				}
				if !errors.Is(err, want) {
					t.Fatalf("%s: got %v, want %v", name, err, want)
				}
			}
		})
	}
}

func TestEvaluateRefusesSponsoredBatchOutsideBindPrecompile(t *testing.T) {
	for name, mutate := range map[string]func(*evm.SponsoredBatch){
		"other target":              func(b *evm.SponsoredBatch) { b.Calls[0].To = b.Quote.Sponsor },
		"later other target":        func(b *evm.SponsoredBatch) { b.Calls[1].To = b.Quote.Sponsor },
		"other account":             func(b *evm.SponsoredBatch) { b.Account = b.Quote.Sponsor },
		"native value":              func(b *evm.SponsoredBatch) { b.Calls[0].Value = big.NewInt(1) },
		"later native value":        func(b *evm.SponsoredBatch) { b.Calls[1].Value = big.NewInt(1) },
		"other method":              func(b *evm.SponsoredBatch) { b.Calls[0].Data[0] ^= 1 },
		"later other method":        func(b *evm.SponsoredBatch) { b.Calls[1].Data[0] ^= 1 },
		"short selector":            func(b *evm.SponsoredBatch) { b.Calls[0].Data = b.Calls[0].Data[:3] },
		"missing bind arguments":    func(b *evm.SponsoredBatch) { b.Calls[0].Data = b.Calls[0].Data[:4] },
		"wrong chain":               func(b *evm.SponsoredBatch) { b.ChainID = big.NewInt(1) },
		"quote exceeds ceiling":     func(b *evm.SponsoredBatch) { b.Quote.TokenAmount = big.NewInt(81) },
		"quote ceiling exceeds cap": func(b *evm.SponsoredBatch) { b.Quote.MaxTokenAmount = big.NewInt(101) },
	} {
		t.Run(name, func(t *testing.T) {
			a, keyID, subject, batch := sponsoredBindAuthority(t, nil)
			mutate(&batch)
			if err := a.Evaluate(keyID, subject, "bind", policy.KindSponsoredBatch, sponsoredBindClaim(t, batch)); !errors.Is(err, ErrAuthority) {
				t.Fatalf("%s: got %v, want refusal", name, err)
			}
		})
	}
	for _, name := range []string{"empty calls", "missing nonce", "missing value", "missing quote amount", "wrong digest", "nil view", "wrong view", "missing wallet", "other subject"} {
		t.Run(name, func(t *testing.T) {
			a, keyID, subject, batch := sponsoredBindAuthority(t, nil)
			claim := sponsoredBindClaim(t, batch)
			switch name {
			case "empty calls":
				claim.Batch.Calls = nil
			case "missing nonce":
				claim.Batch.Nonce = nil
			case "missing value":
				claim.Batch.Calls[0].Value = nil
			case "missing quote amount":
				claim.Batch.Quote.TokenAmount = nil
			case "wrong digest":
				claim.ClaimedDigest[0] ^= 1
			case "missing wallet":
				if err := a.config.Store.Delete(keyID); err != nil {
					t.Fatal(err)
				}
			case "other subject":
				subject = "agent:another"
			}
			var view any = claim
			if name == "nil view" {
				view = (*evm.SponsoredBatchClaim)(nil)
			}
			if name == "wrong view" {
				view = &evm.Transaction{}
			}
			if err := a.Evaluate(keyID, subject, "bind", policy.KindSponsoredBatch, view); !errors.Is(err, ErrAuthority) {
				t.Fatalf("%s: got %v, want refusal", name, err)
			}
		})
	}
}
