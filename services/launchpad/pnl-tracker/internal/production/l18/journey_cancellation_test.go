package l18

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRewardJourneyDatabaseCancellation(t *testing.T) {
	ctx := context.Background()
	for _, action := range []string{"rewards", "preview", "prepare", "prepare replay", "community", "community replay", "watch first", "watch unchanged", "source"} {
		t.Run(action, func(t *testing.T) {
			fixture := func(t *testing.T) (func() error, *internaltest.QueryCancellation, string) {
				f := repair62Chain(t)
				boundLedger := f.ledger
				if action == "community replay" {
					boundLedger = storage(t)
				}
				cfg := boundLedger.pool.Config()
				trace := new(internaltest.QueryCancellation)
				cfg.ConnConfig.Tracer = trace
				pool, err := pgxpool.NewWithConfig(ctx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(pool.Close)
				ledger := *boundLedger
				ledger.pool = pool
				journey := *f.journey
				journey.ledger = &ledger
				watcher, err := NewCommunityWatcher(&journey, nil)
				if err != nil {
					t.Fatal(err)
				}
				key, request := uuid.NewString(), uuid.NewString()
				body := iface.RewardClaimBody{RewardIds: []string{"creator:1"}, Recipient: f.chain.Alice}
				if action == "prepare replay" {
					raw, err := journey.Prepare(ctx, Actor{Wallet: f.chain.Alice}, key, body, request)
					if err != nil || len(raw) == 0 {
						t.Fatal(string(raw), err)
					}
				}
				if action == "community replay" {
					result, err := f.journey.CommunityAuthority(ctx, "1", nil)
					if err != nil {
						t.Fatal(err)
					}
					raw, err := json.Marshal(result)
					if err != nil {
						t.Fatal(err)
					}
					tx, err := ledger.begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = inbox(ctx, tx, "community:"+key, struct{ NFT, Policy string }{"1", watcher.policyDigest}); err != nil {
						t.Fatal(err)
					}
					if _, err = tx.Exec(ctx, `INSERT INTO l18_community_history(request_id,nft_id,state,owner,policy_digest,service_identity,block_number,block_hash,payload) VALUES($1,1,$2,$3,$4,$5,$6,$7,$8)`, key, result.State, result.Owner, watcher.policyDigest, ledger.service, result.Block.Number, result.Block.Hash, raw); err != nil {
						t.Fatal(err)
					}
					if err = tx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if action == "watch unchanged" {
					if err = watcher.RunOnce(ctx); err != nil {
						t.Fatal(err)
					}
				}
				return func() error {
					switch action {
					case "rewards":
						result, err := journey.Rewards(ctx, Actor{Wallet: f.chain.Alice}, "", "", 1)
						if err == nil && (len(result.Items) != 1 || result.Items[0].ClaimableRaw != "10000000") {
							t.Fatal(result)
						}
						return err
					case "preview":
						result, err := journey.Preview(ctx, Actor{Wallet: f.chain.Alice}, "1")
						if err == nil && result.Owner != f.chain.Alice {
							t.Fatal(result)
						}
						return err
					case "prepare", "prepare replay":
						raw, err := journey.Prepare(ctx, Actor{Wallet: f.chain.Alice}, key, body, request)
						if err == nil && len(raw) == 0 {
							t.Fatal("missing prepared reward")
						}
						return err
					case "community", "community replay":
						result, err := watcher.Observe(ctx, key, "1")
						if err == nil && result.Owner != f.chain.Alice {
							t.Fatal(result)
						}
						return err
					case "watch first", "watch unchanged":
						return watcher.RunOnce(ctx)
					default:
						result, err := journey.sourceCheckpoint(ctx)
						if err == nil && result.Number == 0 {
							t.Fatal(result)
						}
						return err
					}
				}, trace, ledger.schema
			}
			operation, trace, _ := fixture(t)
			trace.Start(0)
			err := operation()
			queries := trace.Stop()
			if err != nil || len(queries) == 0 {
				t.Fatal(queries, err)
			}
			for i, query := range queries {
				if strings.EqualFold(query, "rollback") {
					continue
				}
				t.Run(fmt.Sprintf("query_%03d", i+1), func(t *testing.T) {
					operation, cancellation, schema := fixture(t)
					expected := query
					if strings.HasPrefix(query, "SET LOCAL search_path TO ") {
						expected = "SET LOCAL search_path TO " + pgx.Identifier{schema}.Sanitize()
					}
					cancellation.Start(i + 1)
					err := operation()
					observed := cancellation.Stop()
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("query %q: error=%v", query, err)
					}
					if len(observed) != i+1 || observed[i] != expected {
						t.Fatal(observed, query)
					}
				})
			}
		})
	}
}
