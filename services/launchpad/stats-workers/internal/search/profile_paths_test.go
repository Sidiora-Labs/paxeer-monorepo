package search

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestIntegratedProfileReadRespectsVisibilityAndTombstone(t *testing.T) {
	pool := integratedDatabase(t)
	ctx := context.Background()
	ddl := `CREATE SCHEMA users;
CREATE TABLE users.account_state(account_id uuid PRIMARY KEY,wallet text NOT NULL,profile jsonb NOT NULL,revision bigint NOT NULL,updated_at timestamptz NOT NULL,deleted boolean NOT NULL);
CREATE TABLE users.identity_wallet_links(account_id uuid NOT NULL,wallet text NOT NULL,chain_id integer NOT NULL,state text NOT NULL,visibility text NOT NULL);
CREATE VIEW users.account_public_search_v2 AS SELECT a.wallet,a.profile->>'displayName' AS display_name,a.revision,a.updated_at,EXISTS(SELECT 1 FROM users.identity_wallet_links l WHERE l.account_id=a.account_id AND l.wallet=a.wallet AND l.chain_id=125 AND l.state='verified' AND l.visibility='public_optin') AND NOT a.deleted AS public,a.deleted FROM users.account_state a;`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{501, 502} {
		id, wallet := uuid.New(), fixtureAddress(n)
		if _, err := pool.Exec(ctx, `INSERT INTO users.account_state VALUES($1,$2,'{"displayName":"Alice"}',1,clock_timestamp(),false)`, id, wallet); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO users.identity_wallet_links VALUES($1,$2,125,'verified','public_optin')`, id, wallet); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO stats_search.profile_documents_v2 VALUES($1,'Alice','alice',1,clock_timestamp())`, wallet); err != nil {
			t.Fatal(err)
		}
	}
	request := IntegratedRequest{Request: Request{Query: "alice", Kind: "profile", Limit: 1}, Sort: "address"}
	first, err := (Store{Pool: pool}).SearchIntegrated(ctx, request)
	if err != nil || len(first.Items) != 1 || first.Next == nil || *first.Items[0].Wallet != fixtureAddress(501) {
		t.Fatalf("first profile page: %+v %v", first, err)
	}
	request.Position = first.Next
	second, err := (Store{Pool: pool}).SearchIntegrated(ctx, request)
	if err != nil || len(second.Items) != 1 || second.Next != nil || *second.Items[0].Wallet != fixtureAddress(502) {
		t.Fatalf("second profile page: %+v %v", second, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users.identity_wallet_links SET visibility='private' WHERE wallet=$1`, fixtureAddress(501)); err != nil {
		t.Fatal(err)
	}
	request.Position = nil
	visible, err := (Store{Pool: pool}).SearchIntegrated(ctx, request)
	if err != nil || len(visible.Items) != 1 || *visible.Items[0].Wallet != fixtureAddress(502) {
		t.Fatalf("private profile visible: %+v %v", visible, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stats_search.profile_tombstones_v2 VALUES($1,2)`, fixtureAddress(502)); err != nil {
		t.Fatal(err)
	}
	request.Query = fixtureAddress(502)
	missing, err := (Store{Pool: pool}).SearchIntegrated(ctx, request)
	if err != nil || len(missing.Items) != 0 || missing.Status != "not-indexed" {
		t.Fatalf("deleted profile: %+v %v", missing, err)
	}
}

func TestIntegratedPlanUsesCurrentProjection(t *testing.T) {
	pool := integratedDatabase(t)
	doc := seedIndexed(t, pool, 71)
	r := IntegratedRequest{Request: Request{Query: doc.Name, Kind: "pool", Limit: 25}, Sort: "address"}
	plan, err := (Store{Pool: pool}).MeasurePlan(context.Background(), r)
	if err != nil || plan.Generation != "g1" || plan.Revision < 1 || len(plan.SQLDigest) != 64 || len(plan.Plan) == 0 {
		t.Fatalf("measured plan: %+v %v", plan, err)
	}
	var operations []map[string]any
	if err := json.Unmarshal(plan.Plan, &operations); err != nil || len(operations) != 1 {
		t.Fatalf("explain plan: %s %v", plan.Plan, err)
	}
	r.Kind = "profile"
	if _, err := (Store{Pool: pool}).MeasurePlan(context.Background(), r); err == nil {
		t.Fatal("profile plan accepted")
	}
}
