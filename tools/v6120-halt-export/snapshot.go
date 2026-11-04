package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	defaultChainID        = 125
	defaultRequestTimeout = 30 * time.Second
	defaultOverallTimeout = 30 * time.Minute
)

type snapshotBinding struct {
	Version       int         `json:"version"`
	Height        uint64      `json:"height"`
	ChainID       uint64      `json:"chainId"`
	GenesisChain  string      `json:"genesisChain"`
	BlockHash     common.Hash `json:"blockHash"`
	LayoutDigest  common.Hash `json:"layoutDigest"`
	ExportDigest  common.Hash `json:"exportDigest"`
	HeightsDigest common.Hash `json:"heightsDigest"`
}

func digest(raw []byte) common.Hash { return common.Hash(sha256.Sum256(raw)) }

func (b snapshotBinding) validate() error {
	if b.Version != 1 || b.Height == 0 || b.ChainID == 0 || b.GenesisChain == "" ||
		b.BlockHash == (common.Hash{}) || b.LayoutDigest == (common.Hash{}) ||
		b.ExportDigest == (common.Hash{}) || b.HeightsDigest == (common.Hash{}) {
		return fmt.Errorf("missing or unbound snapshot identity; legacy height records must be explicitly regenerated")
	}
	return nil
}

type fetchRecord struct {
	snapshotBinding
	Absent []absentEntry `json:"absent"`
}

type absentEntry struct {
	Address common.Address `json:"address"`
	Slot    *common.Hash   `json:"slot,omitempty"`
}

func readRecord(raw []byte, expected snapshotBinding) (fetchRecord, error) {
	rec := fetchRecord{snapshotBinding: expected, Absent: []absentEntry{}}
	if err := expected.validate(); err != nil {
		return rec, err
	}
	if len(raw) == 0 {
		return rec, nil
	}
	// Missing identity fields must never inherit the expected binding.
	var saved fetchRecord
	if err := json.Unmarshal(raw, &saved); err != nil {
		return rec, err
	}
	if err := saved.validate(); err != nil {
		return saved, err
	}
	if saved.snapshotBinding != expected {
		return saved, fmt.Errorf("fetch record does not match chain, block, layout, export or upgrade heights")
	}
	return saved, nil
}

func rpcCall(ctx context.Context, client *rpc.Client, timeout time.Duration, result interface{}, method string, args ...interface{}) error {
	if timeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	request, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.CallContext(request, result, method, args...); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return nil
}

func validateSnapshot(ctx context.Context, client *rpc.Client, b snapshotBinding, timeout time.Duration) error {
	if err := b.validate(); err != nil {
		return err
	}
	var chain hexutil.Uint64
	if err := rpcCall(ctx, client, timeout, &chain, "eth_chainId"); err != nil {
		return err
	}
	if uint64(chain) != b.ChainID {
		return fmt.Errorf("RPC chain %d, expected %d", chain, b.ChainID)
	}
	var head hexutil.Uint64
	if err := rpcCall(ctx, client, timeout, &head, "eth_blockNumber"); err != nil {
		return err
	}
	if uint64(head) < b.Height {
		return fmt.Errorf("eth_blockNumber %d is below layout height %d", head, b.Height)
	}
	var block *struct {
		Number hexutil.Uint64 `json:"number"`
		Hash   common.Hash    `json:"hash"`
	}
	if err := rpcCall(ctx, client, timeout, &block, "eth_getBlockByNumber", hexutil.EncodeUint64(b.Height), false); err != nil {
		return err
	}
	if block == nil || uint64(block.Number) != b.Height || block.Hash != b.BlockHash {
		return fmt.Errorf("RPC block does not match pinned snapshot at height %d (%s)", b.Height, b.BlockHash)
	}
	return nil
}

func (f *fetcher) record() fetchRecord {
	rec := fetchRecord{snapshotBinding: f.binding, Absent: []absentEntry{}}
	keys := make([]readKey, 0, len(f.done))
	for k, done := range f.done {
		if done {
			keys = append(keys, k)
		}
	}
	sortReadKeys(keys)
	for _, k := range keys {
		if k.code {
			if len(f.a.EvmKeeper.GetCode(f.base, k.addr)) == 0 {
				rec.Absent = append(rec.Absent, absentEntry{Address: k.addr})
			}
		} else if f.a.EvmKeeper.GetState(f.base, k.addr, k.slot) == (common.Hash{}) {
			slot := k.slot
			rec.Absent = append(rec.Absent, absentEntry{Address: k.addr, Slot: &slot})
		}
	}
	return rec
}

func (f *fetcher) save() error {
	out, err := encodeExport(f.raw, f.codes, f.states)
	if err != nil {
		return err
	}
	rec := f.record()
	rec.ExportDigest = digest(out)
	record, err := marshal(rec)
	if err != nil {
		return err
	}
	return f.publication.publish(out, append(record, '\n'), f.heights)
}
