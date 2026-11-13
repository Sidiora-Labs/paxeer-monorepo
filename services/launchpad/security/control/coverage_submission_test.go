package control

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPostgresSubmissionBindsWalletBountyAndAward(t *testing.T) {
	store, _ := fixture(t, nil)
	ctx := context.Background()
	actor := id()
	credentialID := id()
	credential := strings.Repeat("s", 32)
	hash := sha256.Sum256([]byte(credential))
	wallet := beneficiary
	schema := store.schema
	_, err := store.pool.Exec(ctx, "CREATE TABLE "+schema+".authority_credentials_v2(credential_id uuid,actor_id uuid,role text,scopes text[],token_hash bytea,session_id uuid,expires_at timestamptz,revoked_at timestamptz)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.pool.Exec(ctx, "INSERT INTO "+schema+".authority_credentials_v2 VALUES($1,$2,'user',ARRAY[$6]::text[],$3,$4,$5,NULL)", credentialID, actor, hash[:], id(), time.Now().Add(time.Hour), actor)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.pool.Exec(ctx, "CREATE TABLE "+schema+".identity_control_wallets_v2(account_id uuid,wallet text,chain_id bigint,state text)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.pool.Exec(ctx, "INSERT INTO "+schema+".identity_control_wallets_v2 VALUES($1,$2,125,'verified')", actor, wallet)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := NewPostgresIdentityV2(store.pool, strings.Trim(schema, `"`), strings.Trim(schema, `"`))
	if err != nil {
		t.Fatal(err)
	}
	store.identity = identity
	if principal, err := identity.Resolve(ctx, credential); err != nil || principal.ID != actor || !identity.Active(ctx, principal) {
		t.Fatalf("credential authority: %+v, %v", principal, err)
	}
	if err := identity.Wallet(ctx, actor, wallet); err != nil {
		t.Fatalf("verified wallet: %v", err)
	}
	if err := identity.Wallet(ctx, actor, custody); !errors.Is(err, Denied) {
		t.Fatalf("unverified wallet: %v", err)
	}
	bountyID := id()
	b := Bounty{ID: bountyID, Scope: "service", State: controlActive, Deadline: time.Now().Add(time.Hour)}
	body, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.pool.Exec(ctx, "INSERT INTO "+schema+".objects(kind,id,body) VALUES('bounty',$1,$2)", bountyID, body)
	if err != nil {
		t.Fatal(err)
	}
	submission := Submission{ID: id(), BountyID: bountyID, Author: actor, Wallet: wallet, Body: "reviewed evidence", EvidenceURLs: []string{"https://example.invalid/evidence"}}
	invalidURL := submission
	invalidURL.EvidenceURLs = []string{"http://example.invalid/evidence"}
	if err := store.SubmitBounty(ctx, credential, actor, "service", "submit evidence", invalidURL); !errors.Is(err, Invalid) {
		t.Fatalf("insecure evidence URL: %v", err)
	}
	missing := submission
	missing.BountyID = "missing"
	if err := store.SubmitBounty(ctx, credential, actor, "service", "submit evidence", missing); !errors.Is(err, Missing) {
		t.Fatalf("missing bounty: %v", err)
	}
	wrongAuthor := submission
	wrongAuthor.Author = id()
	if err := store.SubmitBounty(ctx, credential, actor, "service", "submit evidence", wrongAuthor); !errors.Is(err, Denied) {
		t.Fatalf("different author: %v", err)
	}
	if err := store.SubmitBounty(ctx, credential, actor, "service", "submit evidence", submission); err != nil {
		t.Fatal(err)
	}
	if err := store.SubmitBounty(ctx, credential, actor, "service", "retry submission", submission); err != nil {
		t.Fatalf("identical submission retry: %v", err)
	}
	var saved Submission
	if err := store.readObject(ctx, "submission", submission.ID, &saved); err != nil || saved.Author != actor || saved.Wallet != wallet || saved.SubmittedAt.IsZero() {
		t.Fatalf("stored submission: %+v, %v", saved, err)
	}
	if err := (StoredAwardAuthority{Store: store, Wallets: identity}).VerifyAward(ctx, b, submission.ID, wallet); err != nil {
		t.Fatalf("verified award: %v", err)
	}
	if err := (StoredAwardAuthority{Store: store, Wallets: identity}).VerifyAward(ctx, b, submission.ID, custody); !errors.Is(err, Denied) {
		t.Fatalf("different destination award: %v", err)
	}
	if err := (StoredAwardAuthority{Store: store, Wallets: identity}).VerifyAward(ctx, b, id(), wallet); !errors.Is(err, Unavailable) {
		t.Fatalf("missing submitted evidence: %v", err)
	}
	expiredAward := b
	expiredAward.Deadline = saved.SubmittedAt.Add(-time.Second)
	if err := (StoredAwardAuthority{Store: store, Wallets: identity}).VerifyAward(ctx, expiredAward, submission.ID, wallet); !errors.Is(err, Denied) {
		t.Fatalf("award after bounty deadline: %v", err)
	}
	changed := submission
	changed.Body = "different evidence"
	if err := store.SubmitBounty(ctx, credential, actor, "service", "changed retry", changed); !errors.Is(err, Conflict) {
		t.Fatalf("changed submission retry: %v", err)
	}
	var count int
	if err := store.pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".objects WHERE kind='submission' AND id=$1", submission.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("submission count: %d, %v", count, err)
	}
}
