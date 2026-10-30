package agent

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var ErrAuthority = errors.New("custody authority unavailable or refused")

const AuthorityAudience = "wallet-custody-authority"
const InventoryAudience = "wallet-custody-inventory"
const maxAuthorityBytes = 2 << 20

type AuthorityConfig struct {
	PublicKeyFile string
	Issuer        string
	Tenant        string
	Store         *store.Store
	ChainID       uint64
	Paymaster     common.Address
}

type SignedClaims struct {
	Version   int    `json:"version"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Tenant    string `json:"tenant"`
	Sequence  string `json:"sequence"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type AgentPolicy struct {
	Mode           string `json:"mode"`
	MaxTx          string `json:"max_tx_value_wei"`
	MaxDaily       string `json:"max_daily_value_wei"`
	Rate           uint32 `json:"rate_limit_per_min"`
	MaxApprove     string `json:"max_approve_wei"`
	AllowNative    bool   `json:"allow_native_transfer"`
	WithdrawalOnly bool   `json:"withdrawal_allowlist_only"`
	ResetHour      int    `json:"daily_reset_utc_hour"`
}

type AgentRule struct {
	Effect   string  `json:"effect"`
	Subject  string  `json:"subject"`
	Value    string  `json:"value"`
	MaxValue *string `json:"max_value_wei"`
}

type AgentBudget struct {
	ID        string  `json:"id"`
	Target    *string `json:"target_contract"`
	Token     *string `json:"token"`
	Cap       string  `json:"cap_wei"`
	Spent     string  `json:"spent_wei"`
	ExpiresAt int64   `json:"expires_at"`
}

type BudgetReservation struct {
	ID          string `json:"id"`
	BudgetID    string `json:"budget_id"`
	Transaction string `json:"transaction"`
	Value       string `json:"value"`
}

type AuthorityPrincipal struct {
	DID          string              `json:"did"`
	PublicKey    string              `json:"public_key"`
	OwnerSubject string              `json:"owner_subject"`
	Frozen       bool                `json:"frozen"`
	KeyIDs       []string            `json:"key_ids"`
	Policy       AgentPolicy         `json:"policy"`
	Rules        []AgentRule         `json:"rules"`
	Budgets      []AgentBudget       `json:"budgets"`
	Reservations []BudgetReservation `json:"reservations"`
	LegacySpent  string              `json:"legacy_spent"`
	LegacyWindow int64               `json:"legacy_window"`
	LegacyRecent uint32              `json:"legacy_recent_requests"`
}

type AuthoritySnapshot struct {
	SignedClaims
	Principals []AuthorityPrincipal `json:"principals"`
}

type SignedVerifier struct {
	Key    *ecdsa.PublicKey
	Issuer string
	Tenant string
}

func NewSignedVerifier(path, issuer, tenant string) (*SignedVerifier, error) {
	if path == "" || issuer == "" || tenant == "" {
		return nil, ErrAuthority
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 16384 {
		return nil, ErrAuthority
	}
	block, rest := pem.Decode(raw)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 || block.Type != "PUBLIC KEY" {
		return nil, ErrAuthority
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrAuthority
	}
	ec, ok := key.(*ecdsa.PublicKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, ErrAuthority
	}
	return &SignedVerifier{Key: ec, Issuer: issuer, Tenant: tenant}, nil
}

func strictJSON(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return ErrAuthority
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return ErrAuthority
	}
	return nil
}

func (v *SignedVerifier) Decode(token, audience string, maxAge time.Duration, out any) error {
	if v == nil || len(token) > maxAuthorityBytes {
		return ErrAuthority
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ErrAuthority
	}
	header, err := base64.RawURLEncoding.Strict().DecodeString(parts[0])
	if err != nil {
		return ErrAuthority
	}
	var h struct {
		Alg  string `json:"alg"`
		Type string `json:"typ"`
	}
	if strictJSON(header, &h) != nil || h.Alg != "ES256" || h.Type != audience+"+jwt" {
		return ErrAuthority
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		return ErrAuthority
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(v.Key, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		return ErrAuthority
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(parts[1])
	if err != nil {
		return ErrAuthority
	}
	if err := strictJSON(payload, out); err != nil {
		return err
	}
	var claims SignedClaims
	if json.Unmarshal(payload, &claims) != nil {
		return ErrAuthority
	}
	now := time.Now().Unix()
	if claims.Version != 1 || claims.Issuer != v.Issuer || claims.Tenant != v.Tenant || claims.Audience != audience || claims.IssuedAt < 0 || claims.IssuedAt > now || claims.ExpiresAt <= now || claims.ExpiresAt <= claims.IssuedAt || claims.ExpiresAt-claims.IssuedAt > int64(maxAge/time.Second) {
		return ErrAuthority
	}
	n, err := strconv.ParseUint(claims.Sequence, 10, 64)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != claims.Sequence {
		return ErrAuthority
	}
	return nil
}

type storedAuthority struct {
	Sequence uint64 `json:"sequence"`
	Token    string `json:"token"`
}

func PersistSigned(st *store.Store, purpose, sequence, token string) error {
	n, err := strconv.ParseUint(sequence, 10, 64)
	if err != nil || st == nil {
		return ErrAuthority
	}
	return st.UpdateRecord(store.RecordKind("custody-authority"), purpose, func(raw []byte) ([]byte, error) {
		var prior storedAuthority
		if raw != nil && strictJSON(raw, &prior) != nil {
			return nil, ErrAuthority
		}
		if prior.Sequence > n || (prior.Sequence == n && prior.Token != token) {
			return nil, ErrAuthority
		}
		return json.Marshal(storedAuthority{Sequence: n, Token: token})
	})
}

type Authority struct {
	mu         sync.RWMutex
	config     AuthorityConfig
	verifier   *SignedVerifier
	snapshot   AuthoritySnapshot
	principals *StaticPrincipals
	byKey      map[string]AuthorityPrincipal
}

func NewAuthority(cfg AuthorityConfig) (*Authority, error) {
	verifier, err := NewSignedVerifier(cfg.PublicKeyFile, cfg.Issuer, cfg.Tenant)
	if err != nil || cfg.Store == nil || cfg.ChainID == 0 {
		return nil, ErrAuthority
	}
	a := &Authority{config: cfg, verifier: verifier}
	err = cfg.Store.WithRecord(store.RecordKind("custody-authority"), AuthorityAudience, func(raw []byte) error {
		var saved storedAuthority
		if strictJSON(raw, &saved) != nil {
			return ErrAuthority
		}
		var snap AuthoritySnapshot
		if verifier.Decode(saved.Token, AuthorityAudience, time.Minute, &snap) != nil {
			return nil
		}
		return a.install(snap)
	})
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	return a, nil
}

func (a *Authority) install(snap AuthoritySnapshot) error {
	if len(snap.Principals) > 65536 {
		return ErrAuthority
	}
	entries := make([]principalJSON, 0, len(snap.Principals))
	byKey := map[string]AuthorityPrincipal{}
	for _, p := range snap.Principals {
		if p.DID == "" || p.PublicKey != strings.ToLower(p.PublicKey) || strings.HasPrefix(p.PublicKey, "0x") {
			return ErrAuthority
		}
		if p.Policy.Mode != "read_only" && p.Policy.Mode != "trade_only" && p.Policy.Mode != "full" {
			return ErrAuthority
		}
		if p.Policy.ResetHour < 0 || p.Policy.ResetHour > 23 || p.Policy.Rate == 0 {
			return ErrAuthority
		}
		for _, amount := range []string{p.Policy.MaxTx, p.Policy.MaxDaily, p.Policy.MaxApprove, p.LegacySpent} {
			if _, err := amount256(amount); err != nil {
				return err
			}
		}
		for _, rule := range p.Rules {
			if rule.Effect != "allow" && rule.Effect != "deny" {
				return ErrAuthority
			}
			if rule.Subject != "contract" && rule.Subject != "selector" && rule.Subject != "token" && rule.Subject != "address" && rule.Subject != "withdrawal" {
				return ErrAuthority
			}
			if rule.Value != strings.ToLower(rule.Value) {
				return ErrAuthority
			}
			if rule.Subject == "selector" {
				b, e := hex.DecodeString(strings.TrimPrefix(rule.Value, "0x"))
				if e != nil || len(b) != 4 || !strings.HasPrefix(rule.Value, "0x") {
					return ErrAuthority
				}
			} else if !common.IsHexAddress(rule.Value) {
				return ErrAuthority
			}
			if rule.MaxValue != nil {
				if _, e := amount256(*rule.MaxValue); e != nil {
					return e
				}
			}
		}
		seenBudgets := map[string]bool{}
		for _, b := range p.Budgets {
			if b.ID == "" || seenBudgets[b.ID] || b.ExpiresAt <= 0 {
				return ErrAuthority
			}
			seenBudgets[b.ID] = true
			cap, e := amount256(b.Cap)
			if e != nil {
				return e
			}
			spent, e := amount256(b.Spent)
			if e != nil || spent.Cmp(cap) > 0 {
				return ErrAuthority
			}
			if b.Target != nil && !common.IsHexAddress(*b.Target) {
				return ErrAuthority
			}
			if b.Token != nil && !common.IsHexAddress(*b.Token) {
				return ErrAuthority
			}
		}
		reservations := map[string]bool{}
		for _, reservation := range p.Reservations {
			if reservation.ID == "" || reservations[reservation.ID] || !seenBudgets[reservation.BudgetID] || !strings.HasPrefix(reservation.Transaction, "0x") {
				return ErrAuthority
			}
			reservations[reservation.ID] = true
			value, err := amount256(reservation.Value)
			if err != nil || value.Sign() <= 0 {
				return ErrAuthority
			}
			raw, err := hex.DecodeString(strings.TrimPrefix(reservation.Transaction, "0x"))
			if err != nil {
				return ErrAuthority
			}
			tx, err := evm.DecodeTransaction(raw, new(big.Int).SetUint64(a.config.ChainID))
			if err != nil || tx.Value.Cmp(value) != 0 {
				return ErrAuthority
			}
		}
		entries = append(entries, principalJSON{DID: p.DID, PublicKey: p.PublicKey, OwnerSubject: p.OwnerSubject, Frozen: p.Frozen, KeyIDs: p.KeyIDs})
		for _, id := range p.KeyIDs {
			if _, duplicate := byKey[id]; duplicate {
				return ErrAuthority
			}
			byKey[id] = p
		}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	principals, err := parsePrincipals(raw, true)
	if err != nil {
		return err
	}
	a.snapshot = snap
	a.principals = principals
	a.byKey = byKey
	return nil
}

func (a *Authority) Admit(token string) (string, error) {
	var snap AuthoritySnapshot
	if err := a.verifier.Decode(token, AuthorityAudience, time.Minute, &snap); err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	candidate := &Authority{config: a.config}
	if err := candidate.install(snap); err != nil {
		return "", err
	}
	if err := PersistSigned(a.config.Store, AuthorityAudience, snap.Sequence, token); err != nil {
		return "", err
	}
	a.snapshot = candidate.snapshot
	a.principals = candidate.principals
	a.byKey = candidate.byKey
	return snap.Sequence, nil
}

func (a *Authority) Lookup(pub [ed25519.PublicKeySize]byte) (Principal, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.principals == nil || time.Now().Unix() >= a.snapshot.ExpiresAt {
		return Principal{}, false
	}
	return a.principals.Lookup(pub)
}

func amount256(text string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(text, 10)
	if !ok || n.Sign() < 0 || n.BitLen() > 256 || n.String() != text {
		return nil, ErrAuthority
	}
	return n, nil
}

type policyReservation struct {
	Request     string `json:"request"`
	Digest      string `json:"digest"`
	At          int64  `json:"at"`
	Value       string `json:"value"`
	Budget      string `json:"budget"`
	Reservation string `json:"reservation"`
}

func (a *Authority) Evaluate(keyID, subject, requestID, kind string, view any) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.principals == nil || time.Now().Unix() >= a.snapshot.ExpiresAt {
		return ErrAuthority
	}
	p, found := a.byKey[keyID]
	if !found {
		if strings.HasPrefix(subject, "agent:") {
			return ErrAuthority
		}
		return nil
	}
	if subject != "agent:"+p.PublicKey {
		if subject == p.OwnerSubject && subject != "" {
			return nil
		}
		return ErrAuthority
	}
	if p.Frozen || p.Policy.Mode == "read_only" {
		return ErrFrozen
	}
	if kind == policy.KindPersonalMessage || kind == policy.KindTypedData {
		return nil
	}
	var value *big.Int
	var destination *common.Address
	var data []byte
	var chainID *big.Int
	var signingDigest common.Hash
	switch kind {
	case policy.KindSponsoredBatch:
		claim, ok := view.(*evm.SponsoredBatchClaim)
		if !ok || claim == nil || a.config.Store == nil {
			return ErrAuthority
		}
		record, err := a.config.Store.Get(keyID)
		if err != nil || record.Curve != store.CurveSecp256k1 {
			return ErrAuthority
		}
		var publicKey *ecdsa.PublicKey
		if len(record.PublicKey) == 33 {
			publicKey, err = crypto.DecompressPubkey(record.PublicKey)
		} else {
			publicKey, err = crypto.UnmarshalPubkey(record.PublicKey)
		}
		if err != nil || claim.Batch.Account != crypto.PubkeyToAddress(*publicKey) {
			return ErrAuthority
		}
		bind, _ := evm.PrecompileAddress("addr")
		bindABI, err := evm.PrecompileABI("addr")
		if err != nil {
			return ErrAuthority
		}
		selector := bindABI.Methods["bindLayerX"].ID
		for _, call := range claim.Batch.Calls {
			if call.To != bind || call.Value == nil || call.Value.Sign() != 0 || len(call.Data) < 4 || !bytes.Equal(call.Data[:4], selector) {
				return ErrAuthority
			}
		}
		quote := claim.Batch.Quote
		maxTx, err := amount256(p.Policy.MaxTx)
		if err != nil || quote.MaxTokenAmount == nil || quote.TokenAmount == nil || quote.TokenAmount.Cmp(quote.MaxTokenAmount) > 0 || quote.MaxTokenAmount.Cmp(maxTx) > 0 {
			return ErrAuthority
		}
		batchPolicy := policy.New(&policy.Document{Version: policy.Version, Defaults: policy.Rules{
			ChainID:       &a.config.ChainID,
			Kinds:         []string{policy.KindSponsoredBatch},
			RatePerMinute: &p.Policy.Rate,
			Selectors:     map[string][]string{bind.Hex(): {"0x" + hex.EncodeToString(selector)}},
			Caps:          map[string]policy.Cap{policy.TokenAsset(quote.Token): {PerTransaction: p.Policy.MaxTx, Daily: p.Policy.MaxDaily}},
		}})
		if !batchPolicy.Evaluate(claim.Batch.Account.Hex(), policy.Request{Kind: kind, View: claim}, policy.NewMemoryLedger(time.Now)).Allowed {
			return ErrAuthority
		}
		value, destination, data = quote.TokenAmount, &bind, claim.Batch.Calls[0].Data
		chainID, signingDigest = claim.Batch.ChainID, claim.ClaimedDigest
	case policy.KindAuthorization:
		claim, ok := view.(*evm.AuthorizationClaim)
		if !ok || claim == nil || a.config.Paymaster == (common.Address{}) || claim.Address != a.config.Paymaster || claim.ChainID == nil || claim.ChainID.Cmp(new(big.Int).SetUint64(a.config.ChainID)) != 0 {
			return ErrAuthority
		}
		if _, err := claim.Verify(); err != nil {
			return ErrAuthority
		}
		return nil
	case policy.KindEVMTransaction:
		tx, ok := view.(*evm.Transaction)
		if !ok || tx == nil || len(tx.Authorizations) != 0 {
			return ErrAuthority
		}
		value, destination, data = tx.Value, tx.To, tx.Data
		chainID, signingDigest = tx.ChainID, tx.SigningDigest
	default:
		return ErrAuthority
	}
	if value == nil || value.Sign() < 0 {
		return ErrAuthority
	}
	var to, selector, token, recipient string
	var tokenAmount *big.Int
	approve := false
	if destination == nil {
		if p.Policy.Mode != "full" {
			return ErrAuthority
		}
	} else {
		to = strings.ToLower(destination.Hex())
		if len(data) > 0 && len(data) < 4 {
			return ErrAuthority
		}
		if len(data) >= 4 {
			selector = "0x" + hex.EncodeToString(data[:4])
		}
		if selector == "0x095ea7b3" || selector == "0xa9059cbb" || selector == "0x23b872dd" {
			call, err := evm.DecodeCalldata(*destination, data)
			if err != nil || call.Kind != evm.CallERC20 {
				return ErrAuthority
			}
			arg := "to"
			if selector == "0x095ea7b3" {
				arg = "spender"
				approve = true
			}
			address, ok := call.Args[arg].(common.Address)
			if !ok {
				return ErrAuthority
			}
			recipient = strings.ToLower(address.Hex())
			tokenAmount, ok = call.Args["value"].(*big.Int)
			if !ok {
				return ErrAuthority
			}
			token = to
		}
	}
	allowedContract := false
	hasAllowedContract := false
	allowedWithdrawal := false
	withdrawal := recipient
	if value.Sign() > 0 && len(data) == 0 {
		withdrawal = to
	}
	for _, r := range p.Rules {
		if r.Effect == "deny" && ((r.Subject == "contract" && r.Value == to) || (r.Subject == "selector" && r.Value == selector) || (r.Subject == "token" && r.Value == token) || (r.Subject == "address" && (r.Value == recipient || r.Value == withdrawal))) {
			return ErrAuthority
		}
		if r.Effect == "allow" && r.Subject == "contract" {
			hasAllowedContract = true
			allowedContract = allowedContract || r.Value == to
		}
		if r.Effect == "allow" && r.Subject == "withdrawal" && r.Value == withdrawal {
			allowedWithdrawal = true
		}
		if r.Effect == "allow" && r.Subject == "token" && r.Value == token && r.MaxValue != nil && tokenAmount != nil {
			cap, _ := amount256(*r.MaxValue)
			if tokenAmount.Cmp(cap) > 0 {
				return ErrAuthority
			}
		}
	}
	if p.Policy.Mode == "trade_only" && hasAllowedContract && !allowedContract {
		return ErrAuthority
	}
	if p.Policy.WithdrawalOnly && withdrawal != "" && !allowedWithdrawal {
		return ErrAuthority
	}
	if value.Sign() > 0 && len(data) == 0 && !p.Policy.AllowNative {
		return ErrAuthority
	}
	if approve {
		cap, _ := amount256(p.Policy.MaxApprove)
		if tokenAmount.Cmp(cap) > 0 {
			return ErrAuthority
		}
	}
	digest := signingDigest.Hex()
	now := time.Now()
	start := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), p.Policy.ResetHour, 0, 0, 0, time.UTC)
	if start.After(now) {
		start = start.Add(-24 * time.Hour)
	}
	if p.LegacyWindow != start.Unix() {
		return ErrAuthority
	}
	bindings := map[string]bool{}
	var binding *BudgetReservation
	for n, reservation := range p.Reservations {
		bindings[reservation.ID] = true
		raw, err := hex.DecodeString(strings.TrimPrefix(reservation.Transaction, "0x"))
		if err != nil {
			return ErrAuthority
		}
		granted, err := evm.DecodeTransaction(raw, chainID)
		if err != nil {
			return ErrAuthority
		}
		if granted.SigningDigest == signingDigest && reservation.Value == value.String() {
			binding = &p.Reservations[n]
		}
	}
	return a.config.Store.UpdateRecord(store.RecordKind("custody-agent-spend"), p.PublicKey, func(raw []byte) ([]byte, error) {
		var entries []policyReservation
		if raw != nil && strictJSON(raw, &entries) != nil {
			return nil, ErrAuthority
		}
		kept := make([]policyReservation, 0, len(entries)+1)
		spent, _ := amount256(p.LegacySpent)
		requests := p.LegacyRecent
		unaccounted := map[string]*big.Int{}
		already := false
		for _, e := range entries {
			if e.Request == requestID && e.Digest != digest {
				return nil, ErrAuthority
			}
			if e.Digest == digest {
				already = true
				kept = append(kept, e)
				continue
			}
			if e.At < now.Add(-24*time.Hour).Unix() {
				continue
			}
			kept = append(kept, e)
			amount, err := amount256(e.Value)
			if err != nil {
				return nil, err
			}
			if e.At >= start.Unix() {
				spent.Add(spent, amount)
			}
			if e.At > now.Add(-time.Minute).Unix() {
				requests++
			}
			if e.Budget != "" && !bindings[e.Reservation] {
				if unaccounted[e.Budget] == nil {
					unaccounted[e.Budget] = new(big.Int)
				}
				unaccounted[e.Budget].Add(unaccounted[e.Budget], amount)
			}
		}
		if requests >= p.Policy.Rate {
			return nil, ErrAuthority
		}
		maxTx, _ := amount256(p.Policy.MaxTx)
		maxDaily, _ := amount256(p.Policy.MaxDaily)
		budgetID := ""
		reservationID := ""
		if value.Cmp(maxTx) > 0 || new(big.Int).Add(spent, value).Cmp(maxDaily) > 0 {
			if binding == nil {
				return nil, ErrAuthority
			}
			for _, b := range p.Budgets {
				if b.ID != binding.BudgetID || b.ExpiresAt <= now.Unix() || b.Token != nil || (b.Target != nil && !strings.EqualFold(*b.Target, to)) {
					continue
				}
				cap, _ := amount256(b.Cap)
				used, _ := amount256(b.Spent)
				if used.Cmp(value) < 0 {
					return nil, ErrAuthority
				}
				if extra := unaccounted[b.ID]; extra != nil {
					used.Add(used, extra)
				}
				if used.Cmp(cap) <= 0 {
					budgetID = b.ID
					reservationID = binding.ID
					break
				}
			}
			if budgetID == "" {
				return nil, ErrAuthority
			}
		}
		if already {
			return append([]byte(nil), raw...), nil
		}
		kept = append(kept, policyReservation{Request: requestID, Digest: digest, At: now.Unix(), Value: value.String(), Budget: budgetID, Reservation: reservationID})
		return json.Marshal(kept)
	})
}

func (a *Authority) Ready() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.principals == nil || time.Now().Unix() >= a.snapshot.ExpiresAt {
		return fmt.Errorf("%w: snapshot missing or expired", ErrAuthority)
	}
	return nil
}

func (a *Authority) RequireSequence(sequence string) error {
	if a == nil {
		return ErrAuthority
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.principals == nil || time.Now().Unix() >= a.snapshot.ExpiresAt || sequence != a.snapshot.Sequence {
		return ErrAuthority
	}
	return nil
}
