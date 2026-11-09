package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

// InitGenesis validates the complete genesis envelope before writing params or
// records. Kernel records belong to the shared host store, not the pay store.
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := ValidateOwnedGenesis(gs); err != nil {
		return err
	}
	branch, commit := ctx.CacheContext()
	k.SetParams(branch, gs.Params)
	store := branch.KVStore(k.hostKey)
	for _, record := range gs.KernelRecords {
		store.Set(record.Key, append([]byte{}, record.Value...))
	}
	for _, record := range gs.EscrowReceipts {
		store.Set(record.Key, bytes.Clone(record.Value))
	}
	payStore := branch.KVStore(k.storeKey)
	for _, record := range gs.AgentStreamMetadata {
		payStore.Set(record.Key, bytes.Clone(record.Value))
	}
	if err := k.RebuildServiceIndexes(branch); err != nil {
		return err
	}
	commit()
	return nil
}

// ValidateOwnedGenesis validates keeper-owned records before any import writes.
func ValidateOwnedGenesis(gs types.GenesisState) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	if err := validateEscrowRecords(gs.KernelRecords); err != nil {
		return types.ErrInvalidGenesis.Wrap(err.Error())
	}
	if err := validateServiceRecords(gs.KernelRecords); err != nil {
		return types.ErrInvalidGenesis.Wrap(err.Error())
	}
	return nil
}

// ExportGenesis returns a detached snapshot of pay params, kernel families and
// agent stream metadata for the application's JSON genesis boundary.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	gs := &types.GenesisState{Params: k.GetParams(ctx)}
	store := ctx.KVStore(k.hostKey)
	for _, prefix := range [][]byte{
		types.EscrowKVPrefix, types.BudgetKVPrefix, types.StreamKVPrefix,
		types.ServiceKVPrefix, types.GovernanceKVPrefix,
	} {
		iterator := sdk.KVStorePrefixIterator(store, prefix)
		for ; iterator.Valid(); iterator.Next() {
			gs.KernelRecords = append(gs.KernelRecords, kv.Pair{
				Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value()),
			})
		}
		iterator.Close()
	}
	iterator := sdk.KVStorePrefixIterator(store, []byte(types.EscrowReceiptPrefix))
	for ; iterator.Valid(); iterator.Next() {
		gs.EscrowReceipts = append(gs.EscrowReceipts, kv.Pair{
			Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value()),
		})
	}
	iterator.Close()
	iterator = sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), []byte(types.AgentStreamMetadataPrefix))
	for ; iterator.Valid(); iterator.Next() {
		gs.AgentStreamMetadata = append(gs.AgentStreamMetadata, kv.Pair{
			Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value()),
		})
	}
	iterator.Close()
	return gs
}

// RecoveryGenesis is the versioned, complete recovery envelope with bounded
// chunks. The application's types.GenesisState also accepts the full record set.
type RecoveryGenesis struct {
	Version             uint32          `json:"version"`
	Params              types.Params    `json:"params"`
	KernelRecordCount   uint64          `json:"kernel_record_count"`
	EscrowReceiptCount  uint64          `json:"escrow_receipt_count"`
	MetadataRecordCount uint64          `json:"metadata_record_count"`
	Chunks              []RecoveryChunk `json:"chunks"`
	Checksum            [32]byte        `json:"checksum"`
}

type RecoveryChunk struct {
	EscrowReceipts      []kv.Pair `json:"escrow_receipts"`
	KernelRecords       []kv.Pair `json:"kernel_records"`
	AgentStreamMetadata []kv.Pair `json:"agent_stream_metadata"`
}

func (g RecoveryGenesis) checksum() [32]byte {
	h := sha256.New()
	encoder := json.NewEncoder(h)
	// JSON encodes byte slices canonically; length and chunk boundaries are bound
	// as well as the counts, params and every owned byte.
	_ = encoder.Encode("layerxpay/recovery/v1")
	_ = encoder.Encode(g.Version)
	_ = encoder.Encode(g.Params)
	_ = encoder.Encode(g.KernelRecordCount)
	_ = encoder.Encode(g.MetadataRecordCount)
	_ = encoder.Encode(g.EscrowReceiptCount)
	for _, chunk := range g.Chunks {
		_ = encoder.Encode(chunk)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func (g RecoveryGenesis) Validate() error {
	if g.Version != 1 {
		return types.ErrInvalidGenesis.Wrap("unknown recovery version")
	}
	if err := g.Params.Validate(); err != nil {
		return err
	}
	var kernels, metadata, receipts uint64
	var lastKernel, lastMetadata, lastReceipt []byte
	var gates []kv.Pair
	var owned []kv.Pair
	for i, chunk := range g.Chunks {
		size := len(chunk.KernelRecords) + len(chunk.AgentStreamMetadata) + len(chunk.EscrowReceipts)
		if size == 0 || size > types.MaxKernelRecords {
			return types.ErrInvalidGenesis.Wrapf("recovery chunk %d has invalid count", i)
		}
		// Reuse the existing key, value and gate validation within each bounded
		// chunk; the full envelope is checked below for global consistency.
		bounded := types.GenesisState{Params: types.DefaultParams(), KernelRecords: chunk.KernelRecords, AgentStreamMetadata: chunk.AgentStreamMetadata, EscrowReceipts: chunk.EscrowReceipts}
		if err := bounded.Validate(); err != nil {
			return err
		}
		for _, record := range chunk.KernelRecords {
			owned = append(owned, record)
			if lastKernel != nil && bytes.Compare(lastKernel, record.Key) >= 0 {
				return types.ErrInvalidGenesis.Wrap("recovery kernel keys must be strictly ordered")
			}
			lastKernel = record.Key
			kernels++
			for _, id := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
				key, err := types.KernelModuleEnableKey(id)
				if err != nil {
					return err
				}
				if bytes.Equal(key, record.Key) {
					gates = append(gates, record)
				}
			}
		}
		for _, record := range chunk.EscrowReceipts {
			if lastReceipt != nil && bytes.Compare(lastReceipt, record.Key) >= 0 {
				return types.ErrInvalidGenesis.Wrap("recovery receipt keys must be strictly ordered")
			}
			lastReceipt = record.Key
			receipts++
		}
		for _, record := range chunk.AgentStreamMetadata {
			if lastMetadata != nil && bytes.Compare(lastMetadata, record.Key) >= 0 {
				return types.ErrInvalidGenesis.Wrap("recovery metadata keys must be strictly ordered")
			}
			lastMetadata = record.Key
			metadata++
		}
	}
	if kernels != g.KernelRecordCount || metadata != g.MetadataRecordCount || receipts != g.EscrowReceiptCount || g.Checksum != g.checksum() {
		return types.ErrInvalidGenesis.Wrap("incomplete or altered recovery envelope")
	}
	if metadata > types.MaxKernelRecords {
		return types.ErrInvalidGenesis.Wrapf("agent stream metadata count %d exceeds %d", metadata, types.MaxKernelRecords)
	}
	if err := validateEscrowRecords(owned); err != nil {
		return types.ErrInvalidGenesis.Wrap(err.Error())
	}
	if err := validateServiceRecords(owned); err != nil {
		return types.ErrInvalidGenesis.Wrap(err.Error())
	}
	return (types.GenesisState{Params: g.Params, KernelRecords: gates}).Validate()
}

func (k Keeper) ExportRecoveryGenesis(ctx sdk.Context) *RecoveryGenesis {
	g := &RecoveryGenesis{Version: 1, Params: k.GetParams(ctx)}
	appendRecord := func(record kv.Pair, collection string) {
		if len(g.Chunks) == 0 || len(g.Chunks[len(g.Chunks)-1].KernelRecords)+len(g.Chunks[len(g.Chunks)-1].AgentStreamMetadata)+len(g.Chunks[len(g.Chunks)-1].EscrowReceipts) == types.MaxKernelRecords {
			g.Chunks = append(g.Chunks, RecoveryChunk{})
		}
		chunk := &g.Chunks[len(g.Chunks)-1]
		if collection == "receipts" {
			chunk.EscrowReceipts = append(chunk.EscrowReceipts, record)
			g.EscrowReceiptCount++
		} else if collection == "metadata" {
			chunk.AgentStreamMetadata = append(chunk.AgentStreamMetadata, record)
			g.MetadataRecordCount++
		} else {
			chunk.KernelRecords = append(chunk.KernelRecords, record)
			g.KernelRecordCount++
		}
	}
	for _, prefix := range [][]byte{types.EscrowKVPrefix, types.BudgetKVPrefix, types.StreamKVPrefix, types.ServiceKVPrefix, types.GovernanceKVPrefix} {
		iterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.hostKey), prefix)
		for ; iterator.Valid(); iterator.Next() {
			appendRecord(kv.Pair{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())}, "kernel")
		}
		iterator.Close()
	}
	iterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.hostKey), []byte(types.EscrowReceiptPrefix))
	for ; iterator.Valid(); iterator.Next() {
		appendRecord(kv.Pair{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())}, "receipts")
	}
	iterator.Close()
	iterator = sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), []byte(types.AgentStreamMetadataPrefix))
	for ; iterator.Valid(); iterator.Next() {
		appendRecord(kv.Pair{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())}, "metadata")
	}
	iterator.Close()
	g.Checksum = g.checksum()
	return g
}

func (k Keeper) InitRecoveryGenesis(ctx sdk.Context, g RecoveryGenesis) error {
	if err := g.Validate(); err != nil {
		return err
	}
	// All chunks, global ordering, completeness and gates are checked before the
	// first write. Publish the complete recovery through a single store branch.
	branch, commit := ctx.CacheContext()
	k.SetParams(branch, g.Params)
	for _, chunk := range g.Chunks {
		for _, record := range chunk.KernelRecords {
			branch.KVStore(k.hostKey).Set(record.Key, append([]byte{}, record.Value...))
		}
		for _, record := range chunk.EscrowReceipts {
			branch.KVStore(k.hostKey).Set(record.Key, bytes.Clone(record.Value))
		}
		for _, record := range chunk.AgentStreamMetadata {
			branch.KVStore(k.storeKey).Set(record.Key, bytes.Clone(record.Value))
		}
	}
	if err := k.RebuildServiceIndexes(branch); err != nil {
		return err
	}
	commit()
	return nil
}
