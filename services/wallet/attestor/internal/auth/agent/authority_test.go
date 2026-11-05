package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
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

func agentPermitData(t *testing.T, primary string, amounts ...string) *evm.TypedData {
	t.Helper()
	field := func(name, typ string) map[string]string { return map[string]string{"name": name, "type": typ} }
	types := map[string]any{
		"EIP712Domain":     []any{field("chainId", "uint256"), field("verifyingContract", "address")},
		"PermitDetails":    []any{field("token", "address"), field("amount", "uint160"), field("expiration", "uint48"), field("nonce", "uint48")},
		"TokenPermissions": []any{field("token", "address"), field("amount", "uint256")},
	}
	const token = "0x3333333333333333333333333333333333333333"
	const spender = "0x2222222222222222222222222222222222222222"
	const permit2 = "0x4444444444444444444444444444444444444444"
	message := map[string]any{"spender": spender}
	verifier := permit2
	switch primary {
	case "Permit":
		verifier = token
		types[primary] = []any{field("spender", "address"), field("value", "uint256")}
		message["value"] = amounts[0]
	case "PermitSingle", "PermitBatch":
		details := make([]any, len(amounts))
		for i, amount := range amounts {
			details[i] = map[string]any{"token": token, "amount": amount, "expiration": "4102444800", "nonce": "0"}
		}
		typ := "PermitDetails"
		message["details"] = details[0]
		if primary == "PermitBatch" {
			typ += "[]"
			message["details"] = details
		}
		types[primary] = []any{field("details", typ), field("spender", "address"), field("sigDeadline", "uint256")}
		message["sigDeadline"] = "4102444800"
	case "PermitTransferFrom", "PermitBatchTransferFrom":
		details := make([]any, len(amounts))
		for i, amount := range amounts {
			details[i] = map[string]any{"token": token, "amount": amount}
		}
		typ := "TokenPermissions"
		message["permitted"] = details[0]
		if primary == "PermitBatchTransferFrom" {
			typ += "[]"
			message["permitted"] = details
		}
		types[primary] = []any{field("permitted", typ), field("spender", "address"), field("nonce", "uint256"), field("deadline", "uint256")}
		message["nonce"], message["deadline"] = "0", "4102444800"
	default:
		t.Fatal("unsupported test permit type")
	}
	raw, err := json.Marshal(map[string]any{"types": types, "primaryType": primary, "domain": map[string]any{"chainId": 125, "verifyingContract": verifier}, "message": message})
	if err != nil {
		t.Fatal(err)
	}
	typed, err := evm.DecodeTypedData(raw)
	if err != nil {
		t.Fatal(err)
	}
	return typed
}

func TestEvaluateTypedDataPermitCountsAgainstTokenCap(t *testing.T) {
	const token = "0x3333333333333333333333333333333333333333"
	const spender = "0x2222222222222222222222222222222222222222"
	for _, primary := range []string{"Permit", "PermitSingle", "PermitBatch", "PermitTransferFrom", "PermitBatchTransferFrom"} {
		for _, amount := range []string{"101", "100", "99"} {
			t.Run(primary+"/token cap/"+amount, func(t *testing.T) {
				a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) {
					cap := "100"
					p.Policy.MaxApprove = "1000"
					p.Rules = []AgentRule{{Effect: "allow", Subject: "token", Value: token, MaxValue: &cap}}
				})
				err := a.Evaluate(keyID, subject, "permit", policy.KindTypedData, agentPermitData(t, primary, amount))
				if amount == "101" {
					if !errors.Is(err, ErrAuthority) {
						t.Fatalf("over-cap %s admitted: %v", primary, err)
					}
				} else if err != nil {
					t.Fatalf("within-cap %s refused: %v", primary, err)
				}
			})
		}
		for _, test := range []struct {
			name      string
			configure func(*AuthorityPrincipal)
		}{
			{"approval cap", func(p *AuthorityPrincipal) { p.Policy.MaxApprove = "98" }},
			{"deny token", func(p *AuthorityPrincipal) { p.Rules = []AgentRule{{Effect: "deny", Subject: "token", Value: token}} }},
			{"deny withdrawal", func(p *AuthorityPrincipal) {
				p.Rules = []AgentRule{{Effect: "deny", Subject: "withdrawal", Value: spender}}
			}},
			{"deny spender", func(p *AuthorityPrincipal) {
				p.Rules = []AgentRule{{Effect: "deny", Subject: "address", Value: spender}}
			}},
			{"deny approval selector", func(p *AuthorityPrincipal) {
				p.Rules = []AgentRule{{Effect: "deny", Subject: "selector", Value: "0x095ea7b3"}}
			}},
			{"deny verifier", func(p *AuthorityPrincipal) {
				verifier := "0x4444444444444444444444444444444444444444"
				if primary == "Permit" {
					verifier = token
				}
				p.Rules = []AgentRule{{Effect: "deny", Subject: "contract", Value: verifier}}
			}},
			{"trade allowlist", func(p *AuthorityPrincipal) {
				p.Rules = []AgentRule{{Effect: "allow", Subject: "contract", Value: spender}}
			}},
			{"withdrawal allowlist", func(p *AuthorityPrincipal) { p.Policy.WithdrawalOnly = true }},
			{"read only", func(p *AuthorityPrincipal) { p.Policy.Mode = "read_only" }},
		} {
			t.Run(primary+"/"+test.name, func(t *testing.T) {
				a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) {
					p.Policy.MaxApprove = "1000"
					test.configure(p)
				})
				if err := a.Evaluate(keyID, subject, "permit", policy.KindTypedData, agentPermitData(t, primary, "99")); err == nil {
					t.Fatalf("%s bypassed %s", primary, test.name)
				}
			})
		}
	}
	for _, primary := range []string{"Permit", "PermitSingle", "PermitBatch", "PermitTransferFrom", "PermitBatchTransferFrom"} {
		t.Run(primary+"/matching allowlists", func(t *testing.T) {
			typed := agentPermitData(t, primary, "99")
			a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) {
				p.Policy.MaxApprove = "100"
				p.Policy.WithdrawalOnly = true
				p.Rules = []AgentRule{
					{Effect: "allow", Subject: "contract", Value: strings.ToLower(typed.Data.Domain.VerifyingContract)},
					{Effect: "allow", Subject: "withdrawal", Value: spender},
					{Effect: "deny", Subject: "address", Value: "0x5555555555555555555555555555555555555555"},
				}
			})
			if err := a.Evaluate(keyID, subject, "permit", policy.KindTypedData, typed); err != nil {
				t.Fatalf("permit matching trade and withdrawal allowlists refused: %v", err)
			}
		})
	}

	for _, primary := range []string{"PermitBatch", "PermitBatchTransferFrom"} {
		for _, capKind := range []string{"token", "approval"} {
			t.Run(primary+"/aggregate/"+capKind, func(t *testing.T) {
				a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) {
					p.Policy.MaxApprove = "100"
					if capKind == "token" {
						cap := "100"
						p.Policy.MaxApprove = "1000"
						p.Rules = []AgentRule{{Effect: "allow", Subject: "token", Value: token, MaxValue: &cap}}
					}
				})
				if err := a.Evaluate(keyID, subject, "batch", policy.KindTypedData, agentPermitData(t, primary, "60", "41")); !errors.Is(err, ErrAuthority) {
					t.Fatalf("split permit bypassed %s cap: %v", capKind, err)
				}
			})
		}
	}
	t.Run("digest and chain validated", func(t *testing.T) {
		a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) { p.Policy.MaxApprove = "100" })
		for _, mutation := range []string{"digest", "chain", "unknown type"} {
			typed := agentPermitData(t, "PermitSingle", "1")
			switch mutation {
			case "digest":
				typed.Digest[0] ^= 1
			case "chain":
				(*big.Int)(typed.Data.Domain.ChainId).SetInt64(1)
				var err error
				typed.Digest, err = evm.TypedDataDigest(typed.Data)
				if err != nil {
					t.Fatal(err)
				}
			case "unknown type":
				typed.Data.Types["Unknown"] = typed.Data.Types[typed.Data.PrimaryType]
				typed.Data.PrimaryType = "Unknown"
				var err error
				typed.Digest, err = evm.TypedDataDigest(typed.Data)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := a.Evaluate(keyID, subject, mutation, policy.KindTypedData, typed); !errors.Is(err, ErrAuthority) {
				t.Fatalf("%s admitted: %v", mutation, err)
			}
		}
	})
}

func TestEvaluatePersonalMessageHonoursDenyRules(t *testing.T) {
	for _, ruleSubject := range []string{"contract", "token", "address", "withdrawal", "selector"} {
		t.Run(ruleSubject, func(t *testing.T) {
			value := "0x2222222222222222222222222222222222222222"
			if ruleSubject == "selector" {
				value = "0x095ea7b3"
			}
			a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) {
				p.Policy.Mode = "full"
				p.Rules = []AgentRule{{Effect: "deny", Subject: ruleSubject, Value: value}}
			})
			message := evm.DecodePersonalMessage([]byte("Authorize " + strings.ToUpper(value)))
			if err := a.Evaluate(keyID, subject, "message", policy.KindPersonalMessage, message); !errors.Is(err, ErrAuthority) {
				t.Fatalf("personal message bypassed %s deny: %v", ruleSubject, err)
			}
		})
	}
	for _, mode := range []string{"full", "trade_only", "read_only", "withdrawal_only", "token cap"} {
		t.Run(mode, func(t *testing.T) {
			a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) {
				p.Policy.Mode = mode
				if mode == "withdrawal_only" {
					p.Policy.Mode = "full"
					p.Policy.WithdrawalOnly = true
				}
				if mode == "token cap" {
					p.Policy.Mode = "full"
					cap := "100"
					p.Rules = []AgentRule{{Effect: "allow", Subject: "token", Value: "0x3333333333333333333333333333333333333333", MaxValue: &cap}}
				}
			})
			message := evm.DecodePersonalMessage([]byte("hello"))
			err := a.Evaluate(keyID, subject, "message", policy.KindPersonalMessage, message)
			if mode == "full" {
				if err != nil {
					t.Fatalf("unrestricted personal message refused: %v", err)
				}
				message.Digest[0] ^= 1
				if err := a.Evaluate(keyID, subject, "tampered", policy.KindPersonalMessage, message); !errors.Is(err, ErrAuthority) {
					t.Fatalf("invalid message digest admitted: %v", err)
				}
			} else if err == nil {
				t.Fatalf("personal message bypassed %s", mode)
			}
		})
	}
	for i, view := range []any{nil, (*evm.PersonalMessage)(nil), &evm.Transaction{}} {
		t.Run(fmt.Sprintf("invalid view/%d", i), func(t *testing.T) {
			a, keyID, subject, _ := sponsoredBindAuthority(t, func(p *AuthorityPrincipal) { p.Policy.Mode = "full" })
			if err := a.Evaluate(keyID, subject, "message", policy.KindPersonalMessage, view); !errors.Is(err, ErrAuthority) {
				t.Fatalf("invalid personal message admitted: %v", err)
			}
		})
	}
}
