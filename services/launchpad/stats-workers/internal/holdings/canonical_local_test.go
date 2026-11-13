package holdings

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

func canonicalLocalFixture(t *testing.T) (context.Context, *pgxpool.Pool, Transfer) {
	t.Helper()
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA indexer"); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"60000", "190001"} {
		if name == "190001" {
			ddl, err := delivery.PartitionSchemaDDL("indexer")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(filepath.Join(root, "migrations", name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	token, wallet := addr(81), addr(82)
	mint := event(token, Zero, wallet, "100", 1)
	poolAddress := addr(83)
	creation := fmt.Sprintf(`{"metadata":{"decimals":6,"totalSupply":"100"}}`)
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,1,10,$6,$7,0,0,100,'{}',$8)`, token, poolAddress, wallet, addr(84), addr(85), mint.BlockHash, mint.TransactionHash, creation); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'canonical','indexer',1,clock_timestamp()+interval '1 hour')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'canonical',10,10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'canonical',10,$1,2,true,true)`, mint.BlockHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,'canonical','first',9,10,$1,2,0)`, mint.BlockHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,10,0,$2,$3,'[]','\x')`, mint.TransactionHash, mint.BlockHash, addr(84)); err != nil {
		t.Fatal(err)
	}
	signature := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	topics, err := json.Marshal([]string{signature.Hex(), "0x" + strings.Repeat("0", 64), "0x" + strings.Repeat("0", 24) + wallet[2:]})
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(100).FillBytes(make([]byte, 32))
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,1,10,0,$2,$3,$4,$5)`, mint.TransactionHash, mint.BlockHash, token, topics, amount); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(mint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_token_transfers(chain_id,transaction_hash,log_index,token,from_address,to_address,amount_raw,height,transaction_index,block_hash,provenance,payload) VALUES(125,$1,1,$2,$3,$4,100,10,0,$5,'{}',$6)`, mint.TransactionHash, token, Zero, wallet, mint.BlockHash, payload); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, mint
}

func TestLocalCanonicalCatchupAndRebuild(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	for _, tc := range []struct {
		name   string
		replay bool
	}{{"catchup", false}, {"rebuild", true}} {
		t.Run(tc.name, func(t *testing.T) {
			schema := "holdings_" + tc.name
			ddl, err := SchemaSQL(schema)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			r, err := NewRepository(pool, schema)
			if err != nil {
				t.Fatal(err)
			}
			if tc.replay {
				checkpoint, err := r.RebuildFromCanonical(ctx, pool, "canonical", mint.Token, 6, "10", "10", "holdings_v1")
				if err != nil || checkpoint.Supply != "100" || checkpoint.Revision != 1 || checkpoint.Position == nil || checkpoint.Position.Identity() != mint.Identity() {
					t.Fatalf("rebuild checkpoint: %+v %v", checkpoint, err)
				}
			} else {
				if err := r.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
					t.Fatal(err)
				}
				batch, err := r.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 1)
				if err != nil || batch.Applied != 1 || batch.More || batch.Coverage != "complete" || batch.Revision != 2 {
					t.Fatalf("catchup: %+v %v", batch, err)
				}
				again, err := r.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 1)
				if err != nil || again.Applied != 0 || again.Revision != 2 {
					t.Fatalf("duplicate catchup: %+v %v", again, err)
				}
			}
			snapshot, err := r.Snapshot(ctx, mint.Token, 100)
			if err != nil || snapshot.Supply != "100" || len(snapshot.Balances) != 1 || snapshot.Balances[0].Raw != "100" {
				t.Fatalf("snapshot: %+v %v", snapshot, err)
			}
		})
	}
}

func TestLocalCanonicalReferenceMatchesPublicSnapshot(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	const schema = "holdings_reference"
	ddl, err := SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	r, err := NewRepository(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	batch, err := r.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 10)
	if err != nil || batch.Coverage != "complete" {
		t.Fatalf("canonical catchup: %+v %v", batch, err)
	}
	poolAddress := addr(83)
	cursor, err := r.Materialize(ctx, mint.Token, poolAddress, "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.ReadPage(ctx, mint.Token, poolAddress, cursor, 100)
	if err != nil || len(page.Snapshot.Balances) != 1 || page.Provenance == nil {
		t.Fatalf("materialized page: %+v %v", page, err)
	}
	wire := phase4HolderWire{Revision: strconv.FormatInt(page.Snapshot.Revision, 10), Quality: "complete"}
	wire.Watermark.Height = mint.Height
	wire.Watermark.BlockHash = mint.BlockHash
	wire.Watermark.ProjectionRevision = wire.Revision
	wire.Watermark.Integrity = "healthy"
	wire.Data.Token = mint.Token
	wire.Data.Pool = poolAddress
	wire.Data.Supply = page.Snapshot.Supply
	wire.Data.Coverage = page.Snapshot.Coverage
	wire.Data.SnapshotID = page.SnapshotID
	wire.Data.Position = page.Snapshot.Position
	wire.Data.Distribution = page.Distribution
	wire.Data.Decimals = int(page.Snapshot.Decimals)
	wire.Data.Provenance = page.Provenance
	for _, balance := range page.Snapshot.Balances {
		wire.Data.Items = append(wire.Data.Items, struct {
			Wallet string `json:"wallet"`
			Raw    string `json:"balanceRaw"`
		}{Wallet: balance.Address, Raw: balance.Raw})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(wire); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	p := Phase4Plan{Origin: server.URL, SourceStream: "canonical", HolderSchema: schema, Pools: []Phase4Pool{{Pool: poolAddress, Token: mint.Token}}}
	a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	if err := phase4Reconcile(ctx, pool, server.Client(), p, a); err != nil {
		t.Fatal(err)
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(a.directory, "arithmetic.ndjson"))
	if err != nil || !strings.Contains(string(raw), `"supply":"100"`) || !strings.Contains(string(raw), "independent_signed_integer_transfer_sums") {
		t.Fatalf("canonical arithmetic: %s %v", raw, err)
	}
}
