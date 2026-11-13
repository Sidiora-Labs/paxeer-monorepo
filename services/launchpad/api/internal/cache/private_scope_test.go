package cache

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

func TestPrivateAdapterSeparatesCommittedWalletSnapshots(t *testing.T) {
	dsn, db := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	const first = "0x1111111111111111111111111111111111111111"
	const second = "0x2222222222222222222222222222222222222222"
	if _, err := db.Exec(context.Background(), `CREATE SCHEMA pnl;
		CREATE TABLE pnl.portfolio_snapshots(id text NOT NULL,source_event text NOT NULL,digest text NOT NULL,wallet text NOT NULL,summary jsonb NOT NULL,revision bigint NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO pnl.portfolio_snapshots VALUES('snapshot-a','fill-a','digest-a',$1,'{"balance":"100"}',1),('snapshot-b','fill-b','digest-b',$2,'{"balance":"200"}',1)`, first, second); err != nil {
		t.Fatal(err)
	}
	t.Setenv("L09_DATABASE_URL", dsn)
	t.Setenv("L09_REDIS_URL", redisURL)
	t.Setenv("L09_NAMESPACE", "kd-421d7541932d2fd2136b3973_private")
	f := newAdversarialCacheFixture(t)
	a := f.adapter(t, 0)
	q := url.Values{"page": {"1"}}
	scopes := []VerifiedScope{
		{NetworkID: "127.0.0.1", Principal: "account-a:" + first, AuthorizationRevision: "1"},
		{NetworkID: "127.0.0.1", Principal: "account-b:" + second, AuthorizationRevision: "1"},
	}
	var bodies [2][]byte
	for i, scope := range scopes {
		projection, stale, err := a.Read(f.ctx, "portfolio_private", q, scope, f.privateFetch)
		if err != nil || stale || !strings.Contains(string(projection.Body), scope.Principal[10:]) || projection.Revision != "1" {
			t.Fatalf("wallet %d projection: %+v stale=%t err=%v", i, projection, stale, err)
		}
		bodies[i] = projection.Body
		cached, stale, err := a.Read(f.ctx, "portfolio_private", q, scope, f.privateFetch)
		if err != nil || stale || !bytes.Equal(cached.Body, bodies[i]) || cached.ContentDigest != projection.ContentDigest {
			t.Fatalf("wallet %d cached projection: %+v stale=%t err=%v", i, cached, stale, err)
		}
	}
	if bytes.Equal(bodies[0], bodies[1]) {
		t.Fatal("distinct wallets shared a private representation")
	}
	if _, err := db.Exec(f.ctx, `INSERT INTO pnl.portfolio_snapshots VALUES('snapshot-a3','fill-a3','digest-a3',$1,'{"balance":"300"}',3)`, first); err != nil {
		t.Fatal(err)
	}
	if err := a.Invalidate(f.ctx, "portfolio_private", q, scopes[0], "g1", "2"); err != nil {
		t.Fatal(err)
	}
	updated, stale, err := a.Read(f.ctx, "portfolio_private", q, scopes[0], f.privateFetch)
	if err != nil || stale || updated.Revision != "3" || !strings.Contains(string(updated.Body), `"balance":"300"`) {
		t.Fatalf("updated wallet projection: %+v stale=%t err=%v", updated, stale, err)
	}
	other, stale, err := a.Read(f.ctx, "portfolio_private", q, scopes[1], f.privateFetch)
	if err != nil || stale || other.Revision != "1" || !bytes.Equal(other.Body, bodies[1]) {
		t.Fatalf("other wallet changed: %+v stale=%t err=%v", other, stale, err)
	}
}
