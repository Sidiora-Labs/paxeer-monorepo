package agent

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type repairM171Fixture struct {
	a         *Authority
	config    AuthorityConfig
	dir       string
	nodeKey   []byte
	signer    *ecdsa.PrivateKey
	principal AuthorityPrincipal
	sequence  uint64
}

const repairM171KeyID = "wallet:agent:secp256k1:0"

func newRepairM171Fixture(t *testing.T) *repairM171Fixture {
	t.Helper()
	dir := t.TempDir()
	signer, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&signer.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(dir, "authority.pem")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0600); err != nil {
		t.Fatal(err)
	}
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nodeKey := make([]byte, 32)
	if _, err := rand.Read(nodeKey); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	f := &repairM171Fixture{
		dir: filepath.Join(dir, "store"), nodeKey: nodeKey, signer: signer,
		config: AuthorityConfig{PublicKeyFile: keyFile, Issuer: "repair-m171", Tenant: "test", ChainID: 125},
		principal: AuthorityPrincipal{
			DID: "did:matrix:test:" + hex.EncodeToString(publicKey[:8]), PublicKey: hex.EncodeToString(publicKey),
			KeyIDs:      []string{repairM171KeyID},
			Policy:      AgentPolicy{Mode: "full", MaxTx: "100", MaxDaily: "100", MaxApprove: "0", Rate: 10, AllowNative: true},
			LegacySpent: "0", LegacyWindow: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix(),
		},
	}
	f.open(t)
	t.Cleanup(func() {
		if err := f.config.Store.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}

func (f *repairM171Fixture) open(t *testing.T) {
	t.Helper()
	st, err := store.Open(f.dir, f.nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	f.config.Store = st
	f.a, err = NewAuthority(f.config)
	if err != nil {
		t.Fatal(err)
	}
}

func (f *repairM171Fixture) reopen(t *testing.T) {
	t.Helper()
	if err := f.config.Store.Close(); err != nil {
		t.Fatal(err)
	}
	f.a = nil
	f.open(t)
}

func (f *repairM171Fixture) admitUntil(t *testing.T, expires time.Time) {
	t.Helper()
	f.sequence++
	now := time.Now()
	snapshot := AuthoritySnapshot{
		SignedClaims: SignedClaims{Version: 1, Issuer: f.config.Issuer, Audience: AuthorityAudience, Tenant: f.config.Tenant,
			Sequence: strconv.FormatUint(f.sequence, 10), IssuedAt: now.Unix(), ExpiresAt: expires.Unix()},
		Principals: []AuthorityPrincipal{f.principal},
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	header, err := json.Marshal(map[string]string{"alg": "ES256", "typ": AuthorityAudience + "+jwt"})
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, f.signer, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	sequence, err := f.a.Admit(input + "." + base64.RawURLEncoding.EncodeToString(signature))
	if err != nil || sequence != snapshot.Sequence {
		t.Fatalf("admit signed authority: sequence=%q error=%v", sequence, err)
	}
}

func (f *repairM171Fixture) admit(t *testing.T) {
	t.Helper()
	f.admitUntil(t, time.Now().Add(time.Minute))
}

func repairM171Transaction(t *testing.T, nonce uint64, value int64, to common.Address) *evm.Transaction {
	t.Helper()
	chainID := big.NewInt(125)
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: chainID, Nonce: nonce, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
		Gas: 21000, To: &to, Value: big.NewInt(value)})
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	view, err := evm.DecodeTransaction(raw, chainID)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func (f *repairM171Fixture) evaluate(requestID string, tx *evm.Transaction) error {
	return f.a.Evaluate(repairM171KeyID, "agent:"+f.principal.PublicKey, requestID, policy.KindEVMTransaction, tx)
}

func (f *repairM171Fixture) spendRecord(t *testing.T) []byte {
	t.Helper()
	var result []byte
	if err := f.config.Store.WithRecord(store.RecordKind("custody-agent-spend"), f.principal.PublicKey, func(raw []byte) error {
		result = append([]byte(nil), raw...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRepairM_171_RetryAfterReopenWithFullQuotas(t *testing.T) {
	for _, test := range []struct {
		name    string
		refresh func(*AuthorityPrincipal)
	}{
		{"later local requests fill rate", func(p *AuthorityPrincipal) { p.Policy.Rate = 1 }},
		{"later local spend fills value", func(p *AuthorityPrincipal) { p.Policy.MaxDaily = "40" }},
		{"producer requests fill rate", func(p *AuthorityPrincipal) { p.LegacyRecent = p.Policy.Rate }},
		{"producer spend fills value", func(p *AuthorityPrincipal) { p.LegacySpent = p.Policy.MaxDaily }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRepairM171Fixture(t)
			f.admit(t)
			to := common.HexToAddress("0x2222222222222222222222222222222222222222")
			original := repairM171Transaction(t, 0, 60, to)
			if err := f.evaluate("original", original); err != nil {
				t.Fatalf("first effect: %v", err)
			}
			if err := f.evaluate("later", repairM171Transaction(t, 1, 40, to)); err != nil {
				t.Fatalf("later effect: %v", err)
			}
			before := f.spendRecord(t)
			var entries []policyReservation
			if err := strictJSON(before, &entries); err != nil || len(entries) != 2 || entries[0].Value != "60" || entries[1].Value != "40" {
				t.Fatalf("producer effects not persisted: %s (%v)", before, err)
			}
			test.refresh(&f.principal)
			f.admit(t)
			f.reopen(t)
			if err := f.a.RequireSequence("2"); err != nil {
				t.Fatalf("recover refreshed signed authority: %v", err)
			}
			if !bytes.Equal(before, f.spendRecord(t)) {
				t.Fatal("reopening changed retained effects")
			}
			if err := f.evaluate("original", original); err != nil {
				t.Fatalf("exact retained retry charged exhausted quota after reopen: %v", err)
			}
			if err := f.evaluate("same-effect", original); err != nil {
				t.Fatalf("same retained economic effect under fresh request ID: %v", err)
			}
			if err := f.evaluate("new", repairM171Transaction(t, 2, 1, to)); !errors.Is(err, ErrAuthority) {
				t.Fatalf("new effect bypassed exhausted quota: %v", err)
			}
			if !bytes.Equal(before, f.spendRecord(t)) {
				t.Fatal("retry or refused effect changed retained accounting")
			}
			f.reopen(t)
			if !bytes.Equal(before, f.spendRecord(t)) {
				t.Fatal("retry changed durable accounting across another restart")
			}
		})
	}
}

func TestRepairM_171_RequestBindingSurvivesReopen(t *testing.T) {
	f := newRepairM171Fixture(t)
	f.principal.Policy.MaxDaily = "1000"
	f.admit(t)
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	original := repairM171Transaction(t, 0, 60, to)
	if err := f.evaluate("original", original); err != nil {
		t.Fatal(err)
	}
	if err := f.evaluate("later", repairM171Transaction(t, 1, 40, to)); err != nil {
		t.Fatal(err)
	}
	f.reopen(t)
	before := f.spendRecord(t)
	for _, test := range []struct {
		name    string
		request string
		tx      *evm.Transaction
	}{
		{"nonce changed", "original", repairM171Transaction(t, 2, 60, to)},
		{"value changed", "original", repairM171Transaction(t, 0, 61, to)},
		{"destination changed", "original", repairM171Transaction(t, 0, 60, common.HexToAddress("0x3333333333333333333333333333333333333333"))},
		{"matching digest precedes conflicting request", "later", original},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := f.evaluate(test.request, test.tx); !errors.Is(err, ErrAuthority) {
				t.Fatalf("altered request binding accepted: %v", err)
			}
			if !bytes.Equal(before, f.spendRecord(t)) {
				t.Fatal("refused request changed durable effects")
			}
		})
	}
	if err := f.evaluate("original", original); err != nil {
		t.Fatalf("unaltered binding refused: %v", err)
	}
}

func TestRepairM_171_RetryRequiresCurrentAuthority(t *testing.T) {
	for _, name := range []string{"frozen", "read only", "key removed", "deny destination", "wrong subject", "stale window", "expired"} {
		t.Run(name, func(t *testing.T) {
			f := newRepairM171Fixture(t)
			f.admit(t)
			to := common.HexToAddress("0x2222222222222222222222222222222222222222")
			original := repairM171Transaction(t, 0, 60, to)
			if err := f.evaluate("original", original); err != nil {
				t.Fatal(err)
			}
			before := f.spendRecord(t)
			want := ErrAuthority
			switch name {
			case "frozen":
				f.principal.Frozen = true
				want = ErrFrozen
			case "read only":
				f.principal.Policy.Mode = "read_only"
				want = ErrFrozen
			case "key removed":
				f.principal.KeyIDs = nil
			case "deny destination":
				f.principal.Rules = []AgentRule{{Effect: "deny", Subject: "contract", Value: strings.ToLower(to.Hex())}}
			case "stale window":
				f.principal.LegacyWindow -= int64((24 * time.Hour) / time.Second)
			}
			if name == "expired" {
				expires := time.Now().Add(2 * time.Second).Truncate(time.Second)
				f.admitUntil(t, expires)
				time.Sleep(time.Until(expires))
			} else {
				f.admit(t)
			}
			f.reopen(t)
			var err error
			if name == "wrong subject" {
				err = f.a.Evaluate(repairM171KeyID, "agent:other", "original", policy.KindEVMTransaction, original)
			} else {
				err = f.evaluate("original", original)
			}
			if !errors.Is(err, want) {
				t.Fatalf("retained effect bypassed %s authority: got %v, want %v", name, err, want)
			}
			if !bytes.Equal(before, f.spendRecord(t)) {
				t.Fatal("refused retry changed durable effects")
			}
		})
	}
}
