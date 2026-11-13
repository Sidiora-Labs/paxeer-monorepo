package source

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
)

func TestLegacyExplorerRejectsUnavailableQueriesAndMalformedRows(t *testing.T) {
	ctx := context.Background()
	dsn, pool := internaltest.NewPostgres(t)
	if _, err := pool.Exec(ctx, blockscoutSchema); err != nil {
		t.Fatal(err)
	}
	emitter := common.HexToAddress("0xee")
	txHash := common.HexToHash("0xa1")
	if _, err := pool.Exec(ctx, `INSERT INTO logs(address_hash,first_topic,data,transaction_hash,block_number,index) VALUES($1,$2,$3,$4,16,0);`, emitter.Bytes(), swapTopic(t).Bytes(), []byte{1}, txHash.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO blocks VALUES(16,to_timestamp(1700000000),true); INSERT INTO transactions VALUES(decode(lpad('a1',64,'0'),'hex'),decode(lpad('ee',40,'0'),'hex'))`); err != nil {
		t.Fatal(err)
	}
	s, err := NewPaxscan(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	opts := FetchOptions{FromBlock: 16, ToBlock: 16, MonitoredAddresses: []string{emitter.Hex()}, Historical: true}
	for _, tc := range []struct{ name, sql, want string }{
		{"logs query", `ALTER TABLE logs RENAME TO unavailable_logs`, "fetch logs"},
		{"log index", `UPDATE logs SET index=NULL`, "scan log"},
		{"blocks query", `ALTER TABLE blocks RENAME TO unavailable_blocks`, "block ts"},
		{"block timestamp", `UPDATE blocks SET timestamp=NULL`, "scan ts"},
		{"transactions query", `ALTER TABLE transactions RENAME TO unavailable_transactions`, "tx from"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			// Commit the schema state so the source's independent read pool observes it.
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			_, err = s.FetchLogs(ctx, opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("explorer admission %v", err)
			}
			restore := map[string]string{"logs query": `ALTER TABLE unavailable_logs RENAME TO logs`, "log index": `UPDATE logs SET index=0`, "blocks query": `ALTER TABLE unavailable_blocks RENAME TO blocks`, "block timestamp": `UPDATE blocks SET timestamp=to_timestamp(1700000000)`, "transactions query": `ALTER TABLE unavailable_transactions RENAME TO transactions`}[tc.name]
			if _, err = pool.Exec(ctx, restore); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE blocks RENAME TO unavailable_blocks`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Head(ctx); err == nil {
		t.Fatal("unavailable explorer head admitted")
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE unavailable_blocks RENAME TO blocks`); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Head(ctx); err == nil {
		t.Fatal("closed explorer head admitted")
	}
	if _, err = s.FetchLogs(ctx, opts); err == nil {
		t.Fatal("closed explorer logs admitted")
	}
}

func TestCommittedExplorerConfigurationMatchesApprovedEpochsAndReader(t *testing.T) {
	ctx := context.Background()
	node := newCapacityChain(t)
	live := repair997Open(t, fopts(node.server.URL))
	dsn, pool := canonicaltest.New(t)
	explorer := capacityExplorer(t, dsn, pool, live, 100)
	for _, tc := range []struct {
		name string
		edit func(*CommittedPaxscanOptions)
		dsn  string
	}{
		{name: "invalid epochs", edit: func(o *CommittedPaxscanOptions) { o.Epochs[0].ABIRevision = "" }},
		{name: "missing epoch", edit: func(o *CommittedPaxscanOptions) { o.Epochs = nil }},
		{name: "different epoch", edit: func(o *CommittedPaxscanOptions) { o.Epochs[0].Role = "Other" }},
		{name: "connection bound", edit: func(o *CommittedPaxscanOptions) { o.MaxConnections = 9 }},
		{name: "dsn", dsn: "://invalid"},
		{name: "reader identity", edit: func(o *CommittedPaxscanOptions) { o.ExpectedRole = "another_reader" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := explorer.options
			o.Epochs = cloneEpochs(o.Epochs)
			if tc.edit != nil {
				tc.edit(&o)
			}
			endpoint := explorer.pool.Config().ConnString()
			if tc.dsn != "" {
				endpoint = tc.dsn
			}
			s, err := NewCommittedPaxscan(ctx, endpoint, o)
			if err == nil {
				s.Close()
				t.Fatal("unbound explorer configuration admitted")
			}
		})
	}
	role := explorer.options.ExpectedRole
	if _, err := pool.Exec(ctx, "REVOKE SELECT ON public.logs FROM "+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if err := explorer.validateSchema(ctx); err == nil {
		t.Fatal("unreadable schema admitted")
	}
	if _, err := pool.Exec(ctx, "GRANT SELECT ON public.logs TO "+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.blocks RENAME TO unavailable_blocks`); err != nil {
		t.Fatal(err)
	}
	if _, err := explorer.Head(ctx); err == nil {
		t.Fatal("unavailable committed head admitted")
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.unavailable_blocks RENAME TO blocks`); err != nil {
		t.Fatal(err)
	}
	if err := explorer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := explorer.Head(ctx); err == nil {
		t.Fatal("closed committed head admitted")
	}
	if _, err := explorer.FetchLogs(ctx, FetchOptions{FromBlock: 1, ToBlock: 2, Historical: true}); err == nil {
		t.Fatal("closed committed range admitted")
	}
}
