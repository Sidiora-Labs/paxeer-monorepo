package referral

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

func productionReferralFixture(t *testing.T, inspect ...func(*rpc.Client, internaltest.ChainFixture)) (*ProductionService, auth.HTTPIdentityAuthority, string, string) {
	t.Helper()
	ctx := context.Background()
	client, chain := internaltest.NewAnvilFixture(t, ctx, coordinatortest.RepositoryRoot(t))
	for _, observe := range inspect {
		observe(client, chain)
	}
	var head struct {
		Number hexutil.Uint64
		Hash   string
	}
	if err := client.CallContext(ctx, &head, "eth_getBlockByNumber", "latest", false); err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for _, target := range []string{l18.NFT, l18.Accumulator, l18.FeesRouter, l18.USDL} {
		var code hexutil.Bytes
		if err := client.CallContext(ctx, &code, "eth_getCode", target, map[string]any{"blockHash": head.Hash, "requireCanonical": true}); err != nil {
			t.Fatal(err)
		}
		hashes[target] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		var slot hexutil.Bytes
		if err := client.CallContext(ctx, &slot, "eth_getStorageAt", target, "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc", map[string]any{"blockHash": head.Hash, "requireCanonical": true}); err != nil {
			t.Fatal(err)
		}
		implementation := common.BytesToAddress(slot)
		if implementation != (common.Address{}) {
			if err := client.CallContext(ctx, &code, "eth_getCode", implementation.Hex(), map[string]any{"blockHash": head.Hash, "requireCanonical": true}); err != nil {
				t.Fatal(err)
			}
			hashes[strings.ToLower(implementation.Hex())] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		}
	}
	creator, err := l18.NewCommittedCreator(ctx, client, time.Now, func(context.Context) (l18.Checkpoint, error) {
		return l18.Checkpoint{Number: uint64(head.Number), Hash: head.Hash}, nil
	}, l18.ManifestDigest, hashes)
	if err != nil {
		t.Fatal(err)
	}
	db := coordinatortest.Bootstrap(t, os.Getenv("PNL_TEST_POSTGRES_DSN"), "indexer_runtime", "pnl_runtime")
	pool, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	program := l18.Program{ID: "campaign", Version: "v1", DeploymentID: "fixture", Asset: l18.USDL, ReferralRaw: "1000", CashbackBPS: 100, CashbackStartHeight: "1", AwardCapRaw: "1000000", DailyCapRaw: "1000000", BeneficiaryCapRaw: "1000000"}
	authority, err := l18.NewCanonicalAuthority(pool, "pnl", creator, []l18.Program{program})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := l18.NewRuntimeLedger(pool, "pnl", "pnl", authority, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		t.Fatal(err)
	}
	attribution, err := l18.NewAttribution(ledger, key, time.Now, program.ID, program.Version)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewProductionService(pool, ledger, authority, attribution, program.ID, program.Version)
	if err != nil {
		t.Fatal(err)
	}
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input auth.IdentityAuthenticationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Authorization != "Bearer owner-access" {
			http.Error(w, "authentication required", 401)
			return
		}
		principal := auth.IdentityPrincipal{Wallet: chain.Alice, AccountID: uuid.NewString(), SessionID: uuid.NewString(), AuthEpoch: 1, ProofAt: time.Now().UTC(), AccessExpiresAt: time.Now().Add(time.Hour).UTC()}
		if err := json.NewEncoder(w).Encode(principal); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(identityServer.Close)
	identity := auth.HTTPIdentityAuthority{Endpoint: identityServer.URL, ServiceToken: strings.Repeat("s", 32), Client: identityServer.Client()}
	return service, identity, strings.ToLower(chain.Alice), strings.ToLower(chain.Bob)
}

func TestProductionReferralHTTPPersistsCodesConsentAndBinding(t *testing.T) {
	s, identity, alice, bob := productionReferralFixture(t)
	ctx := context.Background()
	authenticate := func(r *http.Request) (l18.Actor, error) {
		principal, err := identity.AuthenticateRequest(r.Context(), r)
		return l18.Actor{Wallet: principal.Wallet}, err
	}
	h, err := NewHTTP([]*ProductionService{s}, authenticate)
	if err != nil {
		t.Fatal(err)
	}
	request := func(handler http.HandlerFunc, method, query, body, key, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/referrals"+query, strings.NewReader(body))
		r.Header.Set("Authorization", bearer)
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		out := httptest.NewRecorder()
		handler(out, r)
		return out
	}
	codeKey := uuid.NewString()
	var code l18.ReferralCode
	for i := 0; i < 2; i++ {
		out := request(h.IssueCode, "POST", "", "{}", codeKey, "Bearer owner-access")
		if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &code) != nil || code.Code == "" {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	token, err := s.IssueClick(ctx, l18.Actor{Wallet: bob}, true, code.Code, uuid.NewString())
	if err != nil || token == "" {
		t.Fatal(token, err)
	}
	binding, err := s.Bind(ctx, l18.Actor{Wallet: bob}, uuid.NewString(), true, []string{token})
	if err != nil || binding.Referrer != alice {
		t.Fatal(binding, err)
	}
	for _, tc := range []struct {
		handler http.HandlerFunc
		body    string
	}{{h.Consent, `{"consent":true}`}, {h.Bind, `{"consent":true,"tokens":[]}`}} {
		out := request(tc.handler, "POST", "", tc.body, uuid.NewString(), "Bearer owner-access")
		if out.Code != 200 {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	out := request(h.Dashboard, "GET", "", "", "", "Bearer owner-access")
	var dashboard l18.ReferralDashboard
	if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &dashboard) != nil || dashboard.Code == nil || dashboard.Code.Code != code.Code || !dashboard.Consent.Consent || dashboard.Clicks != 1 || dashboard.BoundAccounts != 1 {
		t.Fatal(out.Code, out.Body.String())
	}
	out = request(h.Click, "POST", "", fmt.Sprintf(`{"code":%q,"consent":true}`, code.Code), uuid.NewString(), "Bearer owner-access")
	if out.Code != 403 {
		t.Fatal(out.Code, out.Body.String())
	}
	for _, tc := range []struct {
		handler                          http.HandlerFunc
		method, query, body, key, bearer string
		status                           int
	}{
		{h.Dashboard, "POST", "", "", "", "Bearer owner-access", 405},
		{h.IssueCode, "GET", "", "", "", "Bearer owner-access", 405},
		{h.Dashboard, "GET", "", "", "", "", 403},
		{h.Dashboard, "GET", "?other=1", "", "", "Bearer owner-access", 422},
		{h.Dashboard, "GET", "?campaign=x&campaign=y", "", "", "Bearer owner-access", 422},
		{h.Dashboard, "GET", "?campaign=missing&policyVersion=v1", "", "", "Bearer owner-access", 503},
		{h.IssueCode, "POST", "", "{}", "", "Bearer owner-access", 422},
		{h.IssueCode, "POST", "", "{}", "invalid", "Bearer owner-access", 422},
		{h.IssueCode, "POST", "", "{", uuid.NewString(), "Bearer owner-access", 422},
		{h.IssueCode, "POST", "", strings.Repeat("x", 16385), uuid.NewString(), "Bearer owner-access", 422},
		{h.IssueCode, "POST", "", "[]", uuid.NewString(), "Bearer owner-access", 422},
		{h.IssueCode, "POST", "", `{"extra":true}`, uuid.NewString(), "Bearer owner-access", 422},
		{h.Consent, "POST", "", "{}", uuid.NewString(), "Bearer owner-access", 422},
		{h.Click, "POST", "", `{"code":"absent","consent":true}`, uuid.NewString(), "Bearer owner-access", 404},
		{h.Bind, "POST", "", `{"consent":false,"tokens":[]}`, uuid.NewString(), "Bearer owner-access", 403},
	} {
		out = request(tc.handler, tc.method, tc.query, tc.body, tc.key, tc.bearer)
		if out.Code != tc.status {
			t.Fatal(tc.method, tc.query, tc.body, out.Code, out.Body.String())
		}
	}
	if err = s.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.CashbackRunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = s.RunOnce(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, actor := range []string{"invalid", "0x" + strings.Repeat("0", 40), "0x" + strings.Repeat("g", 40)} {
		if _, err = s.Convert(ctx, l18.Actor{Wallet: actor}, "invalid"); !errors.Is(err, l18.ErrUnauthorized) {
			t.Fatal(err)
		}
		if _, err = s.Cashback(ctx, l18.Actor{Wallet: actor}, "invalid"); !errors.Is(err, l18.ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if _, err = s.Convert(ctx, l18.Actor{Wallet: alice}, "invalid"); !errors.Is(err, l18.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = s.Cashback(ctx, l18.Actor{Wallet: alice}, "invalid"); !errors.Is(err, l18.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = s.IssueClick(ctx, l18.Actor{Wallet: alice}, false, code.Code, uuid.NewString()); !errors.Is(err, l18.ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err = NewHTTP(nil, nil); !errors.Is(err, l18.ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err = NewHTTP([]*ProductionService{nil}, authenticate); !errors.Is(err, l18.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = NewHTTP([]*ProductionService{s, s}, authenticate); !errors.Is(err, l18.ErrConflict) {
		t.Fatal(err)
	}
	if _, err = NewProductionService(nil, s.ledger, s.authority, s.attribution, "campaign", "v1"); !errors.Is(err, l18.ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err = NewProductionService(s.pool, s.ledger, s.authority, s.attribution, "other", "v1"); !errors.Is(err, l18.ErrInvalid) {
		t.Fatal(err)
	}
	for _, e := range []error{l18.ErrInvalid, l18.ErrUnauthorized, l18.ErrConflict, ErrUnknownReferral, l18.ErrUnavailable} {
		out = httptest.NewRecorder()
		referralError(out, e)
		var body struct {
			Code, RequestID string
			Retryable       bool
		}
		if err = json.Unmarshal(out.Body.Bytes(), &body); err != nil || body.Code == "" || uuid.Validate(body.RequestID) != nil || body.Retryable != (out.Code == 503) {
			t.Fatal(out.Body.String(), err)
		}
	}
}
