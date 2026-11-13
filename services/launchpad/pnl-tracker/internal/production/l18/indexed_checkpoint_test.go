package l18

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

func TestIndexedCreatorRequiresCompleteCanonicalWatermark(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(localChainFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var chain struct{ RPC string }
	if err = json.Unmarshal(raw, &chain); err != nil {
		t.Fatal(err)
	}
	rpc, err := newFixtureRPC(chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	latest := NewCreator(rpc, time.Now)
	block, err := latest.checkpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for _, target := range []string{NFT, Accumulator, FeesRouter, USDL} {
		var code, slot hexutil.Bytes
		bound := map[string]any{fieldBlockHash: block.Hash, fieldRequireCanonical: true}
		if err = rpc.CallContext(ctx, &code, "eth_getCode", target, bound); err != nil {
			t.Fatal(err)
		}
		hashes[target] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		if err = rpc.CallContext(ctx, &slot, "eth_getStorageAt", target, "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc", bound); err != nil {
			t.Fatal(err)
		}
		if implementation := common.BytesToAddress(slot); implementation != (common.Address{}) {
			if err = rpc.CallContext(ctx, &code, "eth_getCode", implementation.Hex(), bound); err != nil {
				t.Fatal(err)
			}
			hashes[strings.ToLower(implementation.Hex())] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		}
	}
	ledger := storage(t)
	pool := ledger.pool
	if _, err = pool.Exec(ctx, `CREATE SCHEMA indexer; CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id bigint,stream_id text,raw_complete_height bigint,typed_complete_height bigint);CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint,stream_id text,height bigint,block_hash text,fetch_complete boolean,required_decode_complete boolean)`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,'canonical',$1,$1)`, int64(block.Number)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'canonical',$1,$2,true,true)`, int64(block.Number), block.Hash); err != nil {
		t.Fatal(err)
	}
	creator, err := NewIndexedCreator(ctx, rpc, pool, "canonical", time.Now, ManifestDigest, hashes)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := creator.checkpoint(ctx); err != nil || got != block {
		t.Fatal(got, block, err)
	}
	for _, tc := range []struct{ name, sql string }{
		{"raw frontier", `UPDATE indexer.indexer_ingest_watermarks_v1 SET raw_complete_height=0`},
		{"fetch completeness", `UPDATE indexer.indexer_block_completeness_v1 SET fetch_complete=false`},
		{"decode completeness", `UPDATE indexer.indexer_block_completeness_v1 SET required_decode_complete=false`},
		{"block hash", `UPDATE indexer.indexer_block_completeness_v1 SET block_hash='invalid'`},
		{"missing completeness", `DELETE FROM indexer.indexer_block_completeness_v1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if _, err := creator.checkpoint(ctx); !errors.Is(err, ErrUnavailable) {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks_v1 SET raw_complete_height=typed_complete_height`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `DELETE FROM indexer.indexer_block_completeness_v1`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'canonical',$1,$2,true,true)`, int64(block.Number), block.Hash); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err = NewIndexedCreator(ctx, rpc, nil, "canonical", time.Now, ManifestDigest, hashes); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err = NewIndexedCreator(ctx, rpc, pool, "invalid stream", time.Now, ManifestDigest, hashes); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestRewardPreparationRejectsUnavailableGasAndChangedBinding(t *testing.T) {
	ctx := context.Background()
	f := repair62Chain(t)
	origin := httptest.NewServer(http.HandlerFunc(f.db.rpcResponse))
	defer origin.Close()
	for _, tc := range []struct {
		name, method string
		result       any
		unavailable  bool
		want         error
	}{
		{name: "gas unavailable", method: "eth_estimateGas", unavailable: true},
		{name: "zero gas", method: "eth_estimateGas", result: "0x0", want: ErrUnavailable},
		{name: "price unavailable", method: "eth_gasPrice", unavailable: true},
		{name: "zero price", method: "eth_gasPrice", result: "0x0", want: ErrUnavailable},
		{name: "cost overflow", method: "eth_gasPrice", result: "0x" + strings.Repeat("f", 64), want: ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			variant := rewardReceiptVariant(t, origin.URL, func(method string, _ []json.RawMessage, out map[string]any) {
				if method == tc.method {
					if tc.unavailable {
						delete(out, "result")
						out["error"] = map[string]any{"code": -32000, "message": "gas unavailable"}
					} else {
						out["result"] = tc.result
					}
				}
			})
			creator := *f.creator
			creator.rpc = variant.rpc
			journey := *f.journey
			journey.creator = &creator
			_, err := journey.Prepare(ctx, Actor{Wallet: f.chain.Alice}, uuid.NewString(), iface.RewardClaimBody{RewardIds: []string{"creator:1"}, Recipient: f.chain.Alice}, uuid.NewString())
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
}
