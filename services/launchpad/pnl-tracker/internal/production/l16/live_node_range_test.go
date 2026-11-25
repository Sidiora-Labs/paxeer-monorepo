package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

type liveNodeRequest struct {
	ID     json.RawMessage   `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func liveNodeProxy(t *testing.T, origin *ethclient.Client, intercept func(liveNodeRequest) (any, error, bool)) *ethclient.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var request liveNodeRequest
		if err = json.Unmarshal(raw, &request); err != nil {
			t.Error(err)
			return
		}
		result, fault, handled := intercept(request)
		if handled {
			envelope := map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}
			if fault != nil {
				delete(envelope, "result")
				envelope["error"] = map[string]any{"code": -32000, "message": fault.Error()}
			}
			if err = json.NewEncoder(w).Encode(envelope); err != nil {
				t.Error(err)
			}
			return
		}
		params := make([]any, len(request.Params))
		for i, param := range request.Params {
			params[i] = param
		}
		var answer json.RawMessage
		err = origin.Client().CallContext(r.Context(), &answer, request.Method, params...)
		envelope := map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": answer}
		if err != nil {
			delete(envelope, "result")
			envelope["error"] = map[string]any{"code": -32000, "message": err.Error()}
		}
		if err = json.NewEncoder(w).Encode(envelope); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := ethclient.DialContext(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func liveNodeFilter(t *testing.T, request liveNodeRequest) (int64, int64) {
	t.Helper()
	var filter struct {
		From hexutil.Big `json:"fromBlock"`
		To   hexutil.Big `json:"toBlock"`
	}
	if err := json.Unmarshal(request.Params[0], &filter); err != nil {
		t.Error(err)
	}
	return (*big.Int)(&filter.From).Int64(), (*big.Int)(&filter.To).Int64()
}

func TestLiveNodeRangeRPC(t *testing.T) {
	fixture := newLocalCanonicalFixture(t, "incremental")
	ctx, a := fixture.ctx, fixture.authority
	admin, err := pgxpool.New(ctx, fixture.db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var creation int64
	if err = admin.QueryRow(ctx, `SELECT height FROM indexer.indexer_canonical_creations_v1 WHERE token=$1`, fixture.chain.Token).Scan(&creation); err != nil {
		t.Fatal(err)
	}
	end := creation + 9999
	incrementalEmptyCoverage(t, ctx, admin, fixture.chain.Stream, int64(fixture.chain.Height)+1, end)
	incrementalHead(t, ctx, admin, fixture.chain.Stream, end, incrementalHash(end))
	origin := newIncrementalNode(t, ctx, admin, fixture.chain.RPC, fixture.chain.Stream, end, int64(fixture.chain.Height), int64(fixture.chain.Height), false)
	query := ethereum.FilterQuery{FromBlock: big.NewInt(creation), ToBlock: big.NewInt(end), Addresses: []common.Address{common.HexToAddress(fixture.chain.Token)}, Topics: [][]common.Hash{{transferTopic}}}
	expected, err := origin.client.FilterLogs(ctx, query)
	if err != nil || len(expected) == 0 {
		t.Fatal(expected, err)
	}

	t.Run("accepted ranges preserve logs and page proof", func(t *testing.T) {
		var mu sync.Mutex
		var ranges [][2]int64
		refused := 0
		client := liveNodeProxy(t, origin.client, func(request liveNodeRequest) (any, error, bool) {
			if request.Method != "eth_getLogs" {
				return nil, nil, false
			}
			from, to := liveNodeFilter(t, request)
			mu.Lock()
			defer mu.Unlock()
			ranges = append(ranges, [2]int64{from, to})
			if to-from+1 > 2000 {
				refused++
				return nil, fmt.Errorf("block range too large (%d), maximum allowed is 2000 blocks", to-from+1), true
			}
			return nil, nil, false
		})
		limited := *a
		limited.rpc = client
		actual, err := limited.liveTransferLogs(ctx, fixture.chain.Token, creation, end)
		if err != nil || !reflect.DeepEqual(actual, expected) {
			t.Fatal(actual, expected, err)
		}
		mu.Lock()
		if refused != 0 || len(ranges) != 5 {
			t.Errorf("ranges=%v refused=%d", ranges, refused)
		}
		for i, r := range ranges {
			if r != [2]int64{creation + int64(i)*2000, creation + int64(i+1)*2000 - 1} {
				t.Errorf("range %d: %v", i, r)
			}
		}
		mu.Unlock()
		tx, err := admin.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer inventoryRollback(tx)
		moves, digest, hash, err := limited.liveMovementPages(ctx, tx, fixture.chain.Token, creation, end)
		if err != nil {
			t.Fatal(err)
		}
		reference := make([]movement, len(expected))
		for i, log := range expected {
			reference[i], err = decodeTransfer(log)
			if err != nil {
				t.Fatal(err)
			}
		}
		raw, err := json.Marshal(reference)
		if err != nil {
			t.Fatal(err)
		}
		proof, err := json.Marshal([]string{delivery.Hash(raw)})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(moves, reference) || digest != delivery.Hash(proof) || hash != incrementalHash(end) {
			t.Fatal(moves, reference, digest, delivery.Hash(proof), hash)
		}
	})

	t.Run("historical ranges use the second chain", func(t *testing.T) {
		var mu sync.Mutex
		var ranges [][2]int64
		primary := liveNodeProxy(t, origin.client, func(request liveNodeRequest) (any, error, bool) {
			if request.Method != "eth_getLogs" {
				return nil, nil, false
			}
			from, to := liveNodeFilter(t, request)
			mu.Lock()
			ranges = append(ranges, [2]int64{from, to})
			mu.Unlock()
			return nil, errors.New("requested fromBlock 48110154 is before earliest available block 48113965"), true
		})
		authority := *a
		authority.rpc = primary
		authority.liveFallback = origin.client
		logs, err := authority.liveTransferLogs(ctx, fixture.chain.Token, creation, end)
		if err != nil || !reflect.DeepEqual(logs, expected) {
			t.Fatal(logs, err)
		}
		mu.Lock()
		if len(ranges) != 5 {
			t.Errorf("primary ranges=%v", ranges)
		}
		for i, r := range ranges {
			if r != [2]int64{creation + int64(i)*2000, creation + int64(i+1)*2000 - 1} {
				t.Errorf("range %d: %v", i, r)
			}
		}
		mu.Unlock()
		var wrongReads int
		wrong := liveNodeProxy(t, origin.client, func(request liveNodeRequest) (any, error, bool) {
			if request.Method == "eth_chainId" {
				return "0x1", nil, true
			}
			if request.Method == "eth_getLogs" {
				wrongReads++
			}
			return nil, nil, false
		})
		authority.liveFallback = wrong
		if _, err = authority.liveTransferLogs(ctx, fixture.chain.Token, creation, creation); !errors.Is(err, delivery.ErrIntegrity) {
			t.Fatal(err)
		}
		if wrongReads != 0 {
			t.Fatal("read on another chain", wrongReads)
		}
	})

	t.Run("missing receipts and blocks use the second node", func(t *testing.T) {
		missing := liveNodeProxy(t, a.rpc, func(request liveNodeRequest) (any, error, bool) {
			return nil, nil, request.Method == "eth_getTransactionReceipt" || request.Method == "eth_getBlockByHash"
		})
		authority := *a
		authority.rpc = missing
		authority.liveFallback = a.rpc
		log := expected[0]
		want, err := a.rpc.TransactionReceipt(ctx, log.TxHash)
		if err != nil {
			t.Fatal(err)
		}
		got, err := liveRead(ctx, &authority, func(client *ethclient.Client) (*types.Receipt, error) {
			return client.TransactionReceipt(ctx, log.TxHash)
		})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal(got, want, err)
		}
		wantHeader, err := a.rpc.HeaderByHash(ctx, log.BlockHash)
		if err != nil {
			t.Fatal(err)
		}
		gotHeader, err := authority.liveHeaderByHash(ctx, log.BlockHash)
		if err != nil || !reflect.DeepEqual(gotHeader, wantHeader) {
			t.Fatal(gotHeader, wantHeader, err)
		}
		wantBlock, err := a.rpc.BlockByHash(ctx, log.BlockHash)
		if err != nil {
			t.Fatal(err)
		}
		gotBlock, err := liveRead(ctx, &authority, func(client *ethclient.Client) (*types.Block, error) { return client.BlockByHash(ctx, log.BlockHash) })
		if err != nil || gotBlock.Hash() != wantBlock.Hash() || len(gotBlock.Transactions()) != len(wantBlock.Transactions()) {
			t.Fatal(gotBlock, wantBlock, err)
		}
		for i, transaction := range gotBlock.Transactions() {
			actual, err := transaction.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			expected, err := wantBlock.Transactions()[i].MarshalBinary()
			if err != nil || !bytes.Equal(actual, expected) {
				t.Fatal(i, actual, expected, err)
			}
		}
		var raw []byte
		if err = admin.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 ORDER BY (payload->'position'->>'height')::bigint LIMIT 1`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		fill, err := accounting.DecodeFillV1(raw)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := admin.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer inventoryRollback(tx)
		wantSettlement, err := a.liveReceiptSettlement(ctx, tx, fill)
		if err != nil {
			t.Fatal(err)
		}
		gotSettlement, err := authority.liveReceiptSettlement(ctx, tx, fill)
		if err != nil || !reflect.DeepEqual(gotSettlement, wantSettlement) {
			t.Fatal(gotSettlement, wantSettlement, err)
		}
		authority.liveFallback = missing
		if _, err = authority.liveHeaderByHash(ctx, log.BlockHash); !errors.Is(err, ethereum.NotFound) {
			t.Fatal(err)
		}
		if _, err = liveRead(ctx, &authority, func(client *ethclient.Client) (*types.Block, error) { return client.BlockByHash(ctx, log.BlockHash) }); !errors.Is(err, ethereum.NotFound) {
			t.Fatal(err)
		}
		if _, err = authority.liveReceiptSettlement(ctx, tx, fill); !errors.Is(err, ethereum.NotFound) {
			t.Fatal(err)
		}
		if _, err = liveRead(ctx, &authority, func(client *ethclient.Client) (*types.Receipt, error) {
			return client.TransactionReceipt(ctx, log.TxHash)
		}); !errors.Is(err, ethereum.NotFound) {
			t.Fatal(err)
		}
	})

	t.Run("hash retention answers preserve reserve checks", func(t *testing.T) {
		var raw []byte
		if err := admin.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 ORDER BY (payload->'position'->>'height')::bigint LIMIT 1`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		fill, err := accounting.DecodeFillV1(raw)
		if err != nil {
			t.Fatal(err)
		}
		block := common.HexToHash(fill.Position.BlockHash)
		pool := common.HexToAddress(fill.PoolAddress)
		positionTx, ok := new(big.Int).SetString(fill.Position.TransactionIndex, 10)
		if !ok {
			t.Fatal(fill.Position)
		}
		positionLog, ok := new(big.Int).SetString(fill.Position.LogIndex, 10)
		if !ok {
			t.Fatal(fill.Position)
		}
		want, err := a.reservesSettledAfter(ctx, block, pool, fill, uint(positionTx.Uint64()), uint(positionLog.Uint64()))
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range []string{"unknown block", "block not found by hash"} {
			t.Run(message, func(t *testing.T) {
				primary := liveNodeProxy(t, a.rpc, func(request liveNodeRequest) (any, error, bool) {
					return nil, errors.New(message), request.Method == "eth_getLogs" || request.Method == "eth_getBlockByHash"
				})
				authority := *a
				authority.rpc = primary
				authority.liveFallback = a.rpc
				got, err := authority.reservesSettledAfter(ctx, block, pool, fill, uint(positionTx.Uint64()), uint(positionLog.Uint64()))
				if err != nil || got != want {
					t.Fatal(got, want, err)
				}
				if _, err = authority.liveHeaderByHash(ctx, block); err != nil {
					t.Fatal(err)
				}
			})
		}
	})

	t.Run("retention matching ignores case and preserves other errors", func(t *testing.T) {
		for _, message := range []string{"requested height 48110154 has been pruned; earliest available is 48113963", "requested fromBlock 48110154 is before earliest available block 48113965", "unknown block", "block not found by hash", "connection refused"} {
			for _, text := range []string{message, strings.ToUpper(message)} {
				t.Run(text, func(t *testing.T) {
					primary := liveNodeProxy(t, a.rpc, func(request liveNodeRequest) (any, error, bool) {
						return nil, errors.New(text), request.Method == "eth_getBlockByHash"
					})
					reads := 0
					second := liveNodeProxy(t, a.rpc, func(request liveNodeRequest) (any, error, bool) { reads++; return nil, nil, false })
					authority := *a
					authority.rpc = primary
					authority.liveFallback = second
					_, err := authority.liveHeaderByHash(ctx, expected[0].BlockHash)
					if message == "connection refused" {
						if err == nil || err.Error() != text || reads != 0 {
							t.Fatal(err, reads)
						}
					} else if err != nil || reads != 2 {
						t.Fatal(err, reads)
					}
				})
			}
		}
	})
}

func TestLiveNodeRangeContinuation(t *testing.T) {
	fixture := newLocalCanonicalFixture(t, "incremental")
	ctx, a := fixture.ctx, fixture.authority
	admin, err := pgxpool.New(ctx, fixture.db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var creation int64
	if err = admin.QueryRow(ctx, `SELECT height FROM indexer.indexer_canonical_creations_v1 WHERE token=$1`, fixture.chain.Token).Scan(&creation); err != nil {
		t.Fatal(err)
	}
	end := creation + 29999
	incrementalEmptyCoverage(t, ctx, admin, fixture.chain.Stream, int64(fixture.chain.Height)+1, end)
	incrementalHead(t, ctx, admin, fixture.chain.Stream, end, incrementalHash(end))
	primary := newIncrementalNode(t, ctx, admin, fixture.chain.RPC, fixture.chain.Stream, end, int64(fixture.chain.Height), int64(fixture.chain.Height), false)
	second := newIncrementalNode(t, ctx, admin, fixture.chain.RPC, fixture.chain.Stream, end, int64(fixture.chain.Height), int64(fixture.chain.Height), false)
	a.rpc, a.liveFallback = primary.client, second.client
	outbox, err := delivery.NewStoreWithEpochGuard(a.pool, "pnl", delivery.Policy{Subjects: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}, EventTypes: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}}, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(a, second.client, [2]string{"primary", "archive"}, strings.Repeat("c", 64), outbox)
	if err != nil {
		t.Fatal(err)
	}
	originalBudget := liveEnrollmentPageTime
	t.Cleanup(func() { liveEnrollmentPageTime = originalBudget })
	if originalBudget != 30*time.Second {
		t.Fatal(originalBudget)
	}
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	var referenceHistory liveReplayHistory
	reference, err := service.liveContinueTx(ctx, tx, fixture.chain.Wallet, fixture.chain.Token, end, nil, &referenceHistory)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	liveEnrollmentPageTime = -time.Nanosecond
	for page := int64(1); page <= 3; page++ {
		before := len(primary.pages)
		tx, err = a.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer inventoryRollback(tx)
		var history liveReplayHistory
		snapshot, foldErr := service.liveContinueTx(ctx, tx, fixture.chain.Wallet, fixture.chain.Token, end, nil, &history)
		if page < 3 && !errors.Is(foldErr, errLiveEnrollmentPending) || page == 3 && foldErr != nil {
			t.Fatal(page, foldErr)
		}
		if snapshot.Height != creation+page*10000-1 {
			t.Fatal(page, snapshot.Height)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if len(primary.pages)-before != 5 || primary.pages[before][0] != creation+(page-1)*10000 {
			t.Fatal(page, primary.pages[before:])
		}
		var persisted int64
		if err = admin.QueryRow(ctx, `SELECT max(height) FROM pnl.inventory_l16_active_v2 WHERE partition LIKE 'live-enrollment:%'`).Scan(&persisted); err != nil || persisted != snapshot.Height {
			t.Fatal(persisted, snapshot.Height, err)
		}
		if page == 3 && (!reflect.DeepEqual(snapshot, reference) || !reflect.DeepEqual(history, referenceHistory)) {
			t.Fatal(snapshot, reference, history, referenceHistory)
		}
		restarted := *service
		authority := *a
		restarted.authority = &authority
		service = &restarted
	}
}
