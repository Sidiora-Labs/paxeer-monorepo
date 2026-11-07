package main

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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// The peer serves reads from a real keeper over loopback HTTP. Identity fields
// are a fixed transcript; gates never contact a live node.
type repairRPC867 struct {
	source  *fetcher
	chain   uint64
	head    uint64
	hash    common.Hash
	number  uint64
	stall   string
	release chan struct{}
	reads   atomic.Int64
}

func (s *repairRPC867) wait(ctx context.Context, method string) error {
	if s.stall != method {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return fmt.Errorf("peer stopped")
	}
}
func (s *repairRPC867) ChainId(ctx context.Context) (hexutil.Uint64, error) {
	if err := s.wait(ctx, "eth_chainId"); err != nil {
		return 0, err
	}
	return hexutil.Uint64(s.chain), nil
}
func (s *repairRPC867) BlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	if err := s.wait(ctx, "eth_blockNumber"); err != nil {
		return 0, err
	}
	return hexutil.Uint64(s.head), nil
}
func (s *repairRPC867) GetBlockByNumber(ctx context.Context, number string, full bool) (map[string]interface{}, error) {
	if err := s.wait(ctx, "eth_getBlockByNumber"); err != nil {
		return nil, err
	}
	if number != hexutil.EncodeUint64(s.number) || full {
		return nil, fmt.Errorf("unexpected block query")
	}
	return map[string]interface{}{"number": hexutil.EncodeUint64(s.number), "hash": s.hash}, nil
}
func (s *repairRPC867) checkBlock(block rpc.BlockNumberOrHash) error {
	hash, ok := block.Hash()
	if !ok || hash != s.hash || !block.RequireCanonical {
		return fmt.Errorf("unbound read: %v", block)
	}
	s.reads.Add(1)
	return nil
}
func (s *repairRPC867) GetCode(ctx context.Context, addr common.Address, block rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	if err := s.checkBlock(block); err != nil {
		return nil, err
	}
	return s.source.a.EvmKeeper.GetCode(s.source.base, addr), nil
}
func (s *repairRPC867) GetStorageAt(ctx context.Context, addr common.Address, slot common.Hash, block rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	if err := s.checkBlock(block); err != nil {
		return nil, err
	}
	return s.source.a.EvmKeeper.GetState(s.source.base, addr, slot).Bytes(), nil
}

func repairPeer867(t *testing.T, peer *repairRPC867, stallBatch int, badBatch bool) *rpc.Client {
	t.Helper()
	peer.release = make(chan struct{})
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", peer))
	var batches atomic.Int64
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("RPC used path %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Preserve the exact JSON body for the real RPC server after inspecting
		// whether this is the batch whose transport should be interrupted.
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Error(err)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if len(raw) > 0 && raw[0] == '[' {
			n := int(batches.Add(1))
			if stallBatch == n {
				select {
				case <-r.Context().Done():
				case <-peer.release:
				}
				return
			}
			if badBatch {
				var elems []struct {
					ID json.RawMessage `json:"id"`
				}
				if err := json.Unmarshal(raw, &elems); err != nil {
					t.Error(err)
					return
				}
				results := make([]map[string]interface{}, len(elems))
				for i, e := range elems {
					result := "0x" + strings.Repeat("00", 32)
					if i == len(elems)-1 {
						result = "0x01"
					}
					results[i] = map[string]interface{}{"jsonrpc": "2.0", "id": e.ID, "result": result}
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(results); err != nil {
					t.Error(err)
				}
				return
			}
		}
		server.ServeHTTP(w, r)
	}))
	client, err := rpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close(); close(peer.release); httpServer.Close(); server.Stop() })
	return client
}

func repairFixture867(t *testing.T) ([]byte, []byte, snapshotBinding) {
	t.Helper()
	codec := app.MakeEncodingConfig().Marshaler
	genesis := app.NewDefaultGenesisState(codec)
	var evm map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(genesis["evm"], &evm))
	evm["codes"] = json.RawMessage(`[]`)
	evm["states"] = json.RawMessage(`[]`)
	evmRaw, err := marshal(evm)
	require.NoError(t, err)
	genesis["evm"] = evmRaw
	state, err := marshal(genesis)
	require.NoError(t, err)
	doc := tmtypes.GenesisDoc{ChainID: "pax-test", InitialHeight: 43, GenesisTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), AppState: state}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	heights, err := marshal(map[string]int64{app.ActivationUpgrade: 2, app.V610Upgrade: 3})
	require.NoError(t, err)
	heights = append(heights, '\n')
	binding := snapshotBinding{Version: 1, Height: 42, ChainID: defaultChainID, GenesisChain: doc.ChainID, BlockHash: common.HexToHash("0x867"), LayoutDigest: digest([]byte(`{"height":42}`)), ExportDigest: digest(raw), HeightsDigest: digest(heights)}
	return raw, heights, binding
}

func repairLoad867(t *testing.T, path string, expected snapshotBinding, client *rpc.Client) *fetcher {
	t.Helper()
	pub := newPublication(path)
	raw, recordRaw, heights, err := pub.open()
	require.NoError(t, err)
	if len(heights) == 0 {
		_, heights, _ = repairFixture867(t)
	}
	expected.ExportDigest = digest(raw)
	rec, err := readRecord(recordRaw, expected)
	require.NoError(t, err)
	doc, err := tmtypes.GenesisDocFromJSON(raw)
	require.NoError(t, err)
	a := app.Setup(t, true, false, false)
	// Both first boot and restart import the published export through the real
	// application codec and InitChain, exactly as the command does.
	params := doc.ConsensusParams.ToProto()
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{Time: doc.GenesisTime, ChainId: doc.ChainID, ConsensusParams: &params, AppStateBytes: doc.AppState, InitialHeight: doc.InitialHeight})
	require.NoError(t, err)
	base := a.NewContext(false, tmproto.Header{ChainID: doc.ChainID, Height: doc.InitialHeight - 1, Time: doc.GenesisTime})
	f := &fetcher{a: a, base: base, client: client, block: rpc.BlockNumberOrHashWithHash(rec.BlockHash, true), done: map[readKey]bool{}, requestTimeout: time.Second, binding: expected, publication: pub, raw: raw, heights: heights}
	for _, e := range rec.Absent {
		if e.Slot == nil {
			f.done[readKey{addr: e.Address, code: true}] = true
		} else {
			f.done[readKey{addr: e.Address, slot: *e.Slot}] = true
		}
	}
	return f
}

func repairStart867(t *testing.T, raw []byte, binding snapshotBinding, client *rpc.Client) *fetcher {
	t.Helper()
	path := filepath.Join(t.TempDir(), "halt.json")
	require.NoError(t, os.WriteFile(path, raw, 0o644))
	return repairLoad867(t, path, binding, client)
}

func repairRecordBytes867(t *testing.T, raw, heights []byte, binding snapshotBinding) []byte {
	t.Helper()
	binding.ExportDigest = digest(raw)
	binding.HeightsDigest = digest(heights)
	record, err := marshal(fetchRecord{snapshotBinding: binding, Absent: []absentEntry{}})
	require.NoError(t, err)
	return append(record, '\n')
}

func TestRepairM_867_RPCPathAndPinnedSnapshot(t *testing.T) {
	endpoint, err := url.Parse(defaultRPC)
	require.NoError(t, err)
	require.Equal(t, "https://api-mainnet-beta.paxeer.network", defaultRPC)
	require.Empty(t, endpoint.Path)
	require.Equal(t, uint64(125), uint64(defaultChainID))
	raw, _, binding := repairFixture867(t)
	for _, tc := range []struct {
		name                string
		chain, head, number uint64
		hash                common.Hash
		wantError           bool
	}{
		{"valid", 125, 45, 42, binding.BlockHash, false},
		{"wrong-chain", 126, 45, 42, binding.BlockHash, true},
		{"behind", 125, 41, 42, binding.BlockHash, true},
		{"wrong-block", 125, 45, 42, common.HexToHash("0xbad"), true},
		{"wrong-number", 125, 45, 43, binding.BlockHash, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := &repairRPC867{chain: tc.chain, head: tc.head, number: tc.number, hash: tc.hash}
			client := repairPeer867(t, peer, 0, false)
			err := validateSnapshot(context.Background(), client, binding, time.Second)
			if tc.wantError {
				require.Error(t, err)
				require.Zero(t, peer.reads.Load())
				return
			}
			require.NoError(t, err)
			source := repairStart867(t, raw, binding, nil)
			defer source.a.Close()
			peer.source = source
			target := repairStart867(t, raw, binding, client)
			defer target.a.Close()
			addr := common.HexToAddress("0x8671")
			slot := common.HexToHash("0x03")
			value := common.HexToHash("0x1234")
			code := []byte{0x60, 0x01, 0x00}
			source.a.EvmKeeper.SetCode(source.base, addr, code)
			source.a.EvmKeeper.SetState(source.base, addr, slot, value)
			require.NoError(t, target.fetch(context.Background(), []readKey{{addr: addr, code: true}, {addr: addr, slot: slot}}))
			require.Equal(t, code, target.a.EvmKeeper.GetCode(target.base, addr))
			require.Equal(t, value, target.a.EvmKeeper.GetState(target.base, addr, slot))
			require.Equal(t, int64(2), peer.reads.Load())
		})
	}
}

func TestRepairM_867_SnapshotBindingRestart(t *testing.T) {
	raw, _, binding := repairFixture867(t)
	source := repairStart867(t, raw, binding, nil)
	defer source.a.Close()
	peer := &repairRPC867{source: source, chain: 125, head: 42, number: 42, hash: binding.BlockHash}
	client := repairPeer867(t, peer, 0, false)
	target := repairStart867(t, raw, binding, client)
	addr := common.HexToAddress("0x8672")
	slot := common.HexToHash("0x02")
	absent := common.HexToAddress("0x8673")
	source.a.EvmKeeper.SetCode(source.base, addr, []byte{0x60, 0x02, 0x00})
	source.a.EvmKeeper.SetState(source.base, addr, slot, common.HexToHash("0x34"))
	keys := []readKey{{addr: addr, code: true}, {addr: addr, slot: slot}, {addr: absent, code: true}, {addr: absent, slot: slot}}
	require.NoError(t, target.fetch(context.Background(), keys))
	path := target.publication.paths[0]
	require.NoError(t, target.a.Close())
	reopened := repairLoad867(t, path, binding, client)
	defer reopened.a.Close()
	require.Equal(t, []byte{0x60, 0x02, 0x00}, reopened.a.EvmKeeper.GetCode(reopened.base, addr))
	require.Equal(t, common.HexToHash("0x34"), reopened.a.EvmKeeper.GetState(reopened.base, addr, slot))
	require.True(t, reopened.done[readKey{addr: absent, code: true}])
	require.True(t, reopened.done[readKey{addr: absent, slot: slot}])
	before := peer.reads.Load()
	require.NoError(t, reopened.fetch(context.Background(), keys))
	require.Equal(t, before, peer.reads.Load(), "recovered zero reads must not be refetched")
	out, recRaw, heights, err := newPublication(path).open()
	require.NoError(t, err)
	expected := binding
	expected.ExportDigest = digest(out)
	expected.HeightsDigest = digest(heights)
	_, err = readRecord(recRaw, expected)
	require.NoError(t, err)
	mutations := map[string]func(*snapshotBinding){
		"height": func(b *snapshotBinding) { b.Height++ }, "chain": func(b *snapshotBinding) { b.ChainID++ },
		"genesis-chain": func(b *snapshotBinding) { b.GenesisChain = "other" }, "block-hash": func(b *snapshotBinding) { b.BlockHash = common.HexToHash("0xbad") },
		"layout": func(b *snapshotBinding) { b.LayoutDigest = digest([]byte("other layout")) }, "export": func(b *snapshotBinding) { b.ExportDigest = digest([]byte("other export")) },
		"heights": func(b *snapshotBinding) { b.HeightsDigest = digest([]byte("other heights")) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := expected
			mutate(&changed)
			_, err := readRecord(recRaw, changed)
			require.Error(t, err)
		})
	}
	_, err = readRecord([]byte(`{"height":42,"absent":[]}`), expected)
	require.ErrorContains(t, err, "unbound")
}

func TestRepairM_867_NetworkDeadlinesAndRecovery(t *testing.T) {
	raw, _, binding := repairFixture867(t)
	for _, method := range []string{"eth_chainId", "eth_blockNumber", "eth_getBlockByNumber"} {
		t.Run(method, func(t *testing.T) {
			peer := &repairRPC867{chain: 125, head: 42, number: 42, hash: binding.BlockHash, stall: method}
			client := repairPeer867(t, peer, 0, false)
			start := time.Now()
			err := validateSnapshot(context.Background(), client, binding, 40*time.Millisecond)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Less(t, time.Since(start), 2*time.Second)
		})
	}
	for _, overall := range []bool{false, true} {
		t.Run(fmt.Sprintf("batch-overall-%v", overall), func(t *testing.T) {
			source := repairStart867(t, raw, binding, nil)
			defer source.a.Close()
			peer := &repairRPC867{source: source, chain: 125, head: 42, number: 42, hash: binding.BlockHash}
			client := repairPeer867(t, peer, 2, false)
			target := repairStart867(t, raw, binding, client)
			var keys []readKey
			// First batch contains both durable positive values and negative reads;
			// the second batch stalls before it can mutate the keeper or record.
			for i := 1; i <= batchSize+1; i++ {
				addr := common.BigToAddress(big.NewInt(int64(i)))
				key := readKey{addr: addr, code: true}
				if i%2 == 0 {
					key.code = false
					key.slot = common.HexToHash("0x01")
				}
				if i%3 == 0 {
					if key.code {
						source.a.EvmKeeper.SetCode(source.base, addr, []byte{0x60, byte(i), 0x00})
					} else {
						source.a.EvmKeeper.SetState(source.base, addr, key.slot, common.BigToHash(big.NewInt(int64(i))))
					}
				}
				keys = append(keys, key)
			}
			target.requestTimeout = 200 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if overall {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
				target.requestTimeout = 2 * time.Second
			}
			defer cancel()
			started := time.Now()
			err := target.fetch(ctx, keys)
			require.True(t, errors.Is(err, context.DeadlineExceeded), "%v", err)
			require.Less(t, time.Since(started), 2*time.Second)
			require.Len(t, target.done, batchSize)
			require.False(t, target.done[keys[batchSize]])
			path := target.publication.paths[0]
			require.NoError(t, target.a.Close())
			resumedPeer := &repairRPC867{source: source, chain: 125, head: 42, number: 42, hash: binding.BlockHash}
			resumedClient := repairPeer867(t, resumedPeer, 0, false)
			reopened := repairLoad867(t, path, binding, resumedClient)
			defer reopened.a.Close()
			require.Len(t, reopened.pending(keys), 1)
			require.NoError(t, validateSnapshot(context.Background(), resumedClient, reopened.binding, time.Second))
			require.NoError(t, reopened.fetch(context.Background(), keys))
			require.Equal(t, int64(1), resumedPeer.reads.Load())
			for _, k := range keys {
				if k.code {
					require.Equal(t, source.a.EvmKeeper.GetCode(source.base, k.addr), reopened.a.EvmKeeper.GetCode(reopened.base, k.addr))
				} else {
					require.Equal(t, source.a.EvmKeeper.GetState(source.base, k.addr, k.slot), reopened.a.EvmKeeper.GetState(reopened.base, k.addr, k.slot))
				}
			}
		})
	}
}

func TestRepairM_867_RejectInvalidBatchWithoutProgress(t *testing.T) {
	raw, _, binding := repairFixture867(t)
	peer := &repairRPC867{chain: 125, head: 42, number: 42, hash: binding.BlockHash}
	client := repairPeer867(t, peer, 0, true)
	target := repairStart867(t, raw, binding, client)
	defer target.a.Close()
	require.NoError(t, target.save())
	before, err := os.Readlink(target.publication.current)
	require.NoError(t, err)
	keys := []readKey{{addr: common.HexToAddress("0x01"), slot: common.HexToHash("0x01")}, {addr: common.HexToAddress("0x02"), slot: common.HexToHash("0x02")}}
	require.ErrorContains(t, target.fetch(context.Background(), keys), "1 bytes")
	require.Empty(t, target.done)
	require.Len(t, target.pending(keys), 2)
	after, err := os.Readlink(target.publication.current)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestRepairM_867_AtomicPublicationInterruption(t *testing.T) {
	raw, heights, binding := repairFixture867(t)
	source := repairStart867(t, raw, binding, nil)
	defer source.a.Close()
	source.a.EvmKeeper.SetCode(source.base, common.HexToAddress("0x8674"), []byte{0x60, 0x04, 0x00})
	peer := &repairRPC867{source: source, chain: 125, head: 42, number: 42, hash: binding.BlockHash}
	client := repairPeer867(t, peer, 0, false)
	producer := repairStart867(t, raw, binding, client)
	require.NoError(t, producer.fetch(context.Background(), []readKey{{addr: common.HexToAddress("0x8674"), code: true}}))
	nextExport, nextRecord, nextHeights, err := producer.publication.open()
	require.NoError(t, err)
	require.NoError(t, producer.a.Close())
	firstRecord := repairRecordBytes867(t, raw, heights, binding)
	for _, phase := range []string{"staged-before-commit", "bootstrap-after-pointer", "update-after-pointer"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "halt.json")
			require.NoError(t, os.WriteFile(path, raw, 0o644))
			p := newPublication(path)
			if phase != "bootstrap-after-pointer" {
				require.NoError(t, p.publish(raw, firstRecord, heights))
			}
			oldTarget, _ := os.Readlink(p.current)
			staged, err := p.stage(nextExport, nextRecord, nextHeights)
			require.NoError(t, err)
			if phase != "staged-before-commit" {
				require.NoError(t, staged.commitPointer())
			}
			// Discard all publication objects and reopen only from the on-disk pointer.
			p = nil
			staged = nil
			recovered := newPublication(path)
			got, record, gotHeights, err := recovered.open()
			require.NoError(t, err)
			if phase == "staged-before-commit" {
				require.Equal(t, raw, got)
				require.Equal(t, firstRecord, record)
				require.Equal(t, heights, gotHeights)
				current, err := os.Readlink(recovered.current)
				require.NoError(t, err)
				require.Equal(t, oldTarget, current)
			} else {
				require.Equal(t, nextExport, got)
				require.Equal(t, nextRecord, record)
				require.Equal(t, nextHeights, gotHeights)
				reopened := repairLoad867(t, path, binding, client)
				require.Equal(t, []byte{0x60, 0x04, 0x00}, reopened.a.EvmKeeper.GetCode(reopened.base, common.HexToAddress("0x8674")))
				require.NoError(t, reopened.a.Close())
			}
			for i, path := range recovered.paths {
				contents, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, [][]byte{got, record, gotHeights}[i], contents)
			}
		})
	}
	t.Run("reject-snapshot-replacement", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "halt.json")
		require.NoError(t, os.WriteFile(path, raw, 0o644))
		p := newPublication(path)
		require.NoError(t, p.publish(raw, firstRecord, heights))
		changed := binding
		changed.BlockHash = common.HexToHash("0xbad")
		different := repairRecordBytes867(t, nextExport, nextHeights, changed)
		_, err := p.stage(nextExport, different, nextHeights)
		require.ErrorContains(t, err, "different snapshot")
		got, record, _, err := newPublication(path).open()
		require.NoError(t, err)
		require.Equal(t, raw, got)
		require.Equal(t, firstRecord, record)
	})
	t.Run("corrupt-generation", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "halt.json")
		require.NoError(t, os.WriteFile(path, raw, 0o644))
		p := newPublication(path)
		require.NoError(t, p.publish(raw, firstRecord, heights))
		require.NoError(t, os.WriteFile(p.paths[1], nextRecord, 0o644))
		_, _, _, err := newPublication(path).open()
		require.ErrorContains(t, err, "digest mismatch")
	})
	t.Run("mixed-bootstrap-sidecar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "halt.json")
		require.NoError(t, os.WriteFile(path, raw, 0o644))
		p := newPublication(path)
		staged, err := p.stage(nextExport, nextRecord, nextHeights)
		require.NoError(t, err)
		require.NoError(t, staged.commitPointer())
		require.NoError(t, os.WriteFile(p.paths[1], firstRecord, 0o644))
		_, _, _, err = newPublication(path).open()
		require.ErrorContains(t, err, "unexpected snapshot sidecar")
	})
}
