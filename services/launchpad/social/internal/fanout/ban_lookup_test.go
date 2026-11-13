package fanout

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/google/uuid"
)

func TestHubBanLookupUsesCommittedGlobalAndPoolActions(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	const wallet = "0x1111111111111111111111111111111111111111"
	const pool = "0x2222222222222222222222222222222222222222"
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS chat.chat_bans (id text PRIMARY KEY,wallet varchar(42) NOT NULL,pool_address varchar(42),reason text,banned_by varchar(42) NOT NULL,expires_at bigint,created_at bigint NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	hub := &Hub{q: sqlcdb.New(db), clock: time.Now}
	if hub.isBanned(ctx, wallet, pool) {
		t.Fatal("unbanned wallet denied")
	}
	if _, err := db.Exec(ctx, `INSERT INTO chat.chat_bans(id,wallet,pool_address,banned_by,created_at) VALUES($1,$2,$3,$4,$5)`, uuid.NewString(), wallet, pool, wallet, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if !hub.isBanned(ctx, wallet, pool) || hub.isBanned(ctx, wallet, "") {
		t.Fatal("pool ban did not remain scoped")
	}
	if _, err := db.Exec(ctx, `INSERT INTO chat.chat_bans(id,wallet,banned_by,created_at) VALUES($1,$2,$3,$4)`, uuid.NewString(), wallet, wallet, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if !hub.isBanned(ctx, wallet, "") {
		t.Fatal("global ban did not deny the wallet")
	}
	db.Close()
	if !hub.isBanned(ctx, wallet, pool) {
		t.Fatal("unavailable ban relation admitted the wallet")
	}
}
