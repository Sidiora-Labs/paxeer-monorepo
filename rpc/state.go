package evmrpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	gigacachekv "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/tracekv"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

var errNoProofCapableQueryableKVStore = errors.New("cannot find a proof-capable queryable KV store")

// ErrPendingProofRefused refuses eth_getProof with the pending tag on every
// node: the open block has no committed root to prove against.
var ErrPendingProofRefused error = &ErrEVMNotSupported{Msg: "pending state has no committed root"}

const MaxStorageKeysPerProof = 1024

type StateAPI struct {
	tmClient       client.LocalClient
	keeper         *keeper.Keeper
	ctxProvider    func(int64) sdk.Context
	connectionType ConnectionType
	watermarks     *WatermarkManager
	root           storetypes.Queryable
}

func NewStateAPI(tmClient client.LocalClient, k *keeper.Keeper, ctxProvider func(int64) sdk.Context, connectionType ConnectionType, watermarks *WatermarkManager, root storetypes.Queryable) *StateAPI {
	return &StateAPI{tmClient: tmClient, keeper: k, ctxProvider: ctxProvider, connectionType: connectionType, watermarks: watermarks, root: root}
}

func (a *StateAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Big, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getBalance", a.connectionType, startTime, returnErr, recover())
	}()
	sdkCtx, err := a.stateContext(ctx, blockNrOrHash)
	if err != nil {
		return nil, err
	}
	statedb := state.NewDBImpl(sdkCtx, a.keeper, true)
	return (*hexutil.Big)(statedb.GetBalance(address).ToBig()), nil
}

func (a *StateAPI) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getCode", a.connectionType, startTime, returnErr, recover())
	}()
	sdkCtx, err := a.stateContext(ctx, blockNrOrHash)
	if err != nil {
		return nil, err
	}
	code := a.keeper.GetCode(sdkCtx, address)
	return code, nil
}

func (a *StateAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getStorageAt", a.connectionType, startTime, returnErr, recover())
	}()
	sdkCtx, err := a.stateContext(ctx, blockNrOrHash)
	if err != nil {
		return nil, err
	}
	key, _, err := decodeHash(hexKey)
	if err != nil {
		return nil, fmt.Errorf("unable to decode storage key: %s", err)
	}
	state := a.keeper.GetState(sdkCtx, address, key)
	return state[:], nil
}

func (a *StateAPI) stateContext(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (sdk.Context, error) {
	if isPendingBlockNrOrHash(blockNrOrHash) {
		snapshot, height, open, err := a.watermarks.pendingSnapshot(ctx)
		if err != nil {
			return sdk.Context{}, err
		}
		if open {
			return snapshot, nil
		}
		blockNrOrHash = rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(height))
	}
	height, err := a.watermarks.ResolveHeight(ctx, blockNrOrHash)
	if err != nil {
		return sdk.Context{}, err
	}
	sdkCtx := a.ctxProvider(height)
	return sdkCtx, checkStateVersion(sdkCtx, height, a.keeper)
}

// checkStateVersion checks that the state stores exist at a committed height.
// The open block is built over the latest committed state and has no version
// of its own, so a pending read skips the check.
func checkStateVersion(sdkCtx sdk.Context, height int64, k *keeper.Keeper) error {
	if height == PendingCtxHeight {
		return nil
	}
	return CheckVersion(sdkCtx, k)
}

// Result structs for GetProof
// This differs from go-ethereum AccountResult in two ways:
// 1. Proof object is an ICS23 proof through the app root, not a trie proof
// 2. Per-account fields are excluded because there is no per-account root
type ProofResult struct {
	Address      common.Address     `json:"address"`
	HexValues    []string           `json:"hexValues"`
	StorageProof []*crypto.ProofOps `json:"storageProof"`
}

func (a *StateAPI) GetProof(ctx context.Context, address common.Address, storageKeys []string, blockNrOrHash rpc.BlockNumberOrHash) (result *ProofResult, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getProof", a.connectionType, startTime, returnErr, recover())
	}()
	if isPendingBlockNrOrHash(blockNrOrHash) {
		return nil, ErrPendingProofRefused
	}
	var block *coretypes.ResultBlock
	var err error
	if blockNr, ok := blockNrOrHash.Number(); ok {
		blockNumber, blockNumErr := getBlockNumber(ctx, a.tmClient, a.watermarks, blockNr)
		if blockNumErr != nil {
			return nil, blockNumErr
		}
		block, err = blockByNumberRespectingWatermarks(ctx, a.tmClient, a.watermarks, blockNumber, 1)
	} else {
		block, err = blockByHashRespectingWatermarks(ctx, a.tmClient, a.watermarks, blockNrOrHash.BlockHash[:], 1)
	}
	if err != nil {
		return nil, err
	}
	sdkCtx := a.ctxProvider(block.Block.Height)
	if err := CheckVersion(sdkCtx, a.keeper); err != nil {
		return nil, err
	}
	if a.root == nil {
		return nil, errors.New("cannot generate storage proof: missing proof root")
	}
	if len(storageKeys) > MaxStorageKeysPerProof {
		return nil, fmt.Errorf("too many storage keys: got %d, max %d", len(storageKeys), MaxStorageKeysPerProof)
	}
	paddedKeys := make([]common.Hash, len(storageKeys))
	for i, key := range storageKeys {
		paddedKey, _, err := decodeHash(key)
		if err != nil {
			return nil, fmt.Errorf("invalid storage key %q: %w", key, err)
		}
		paddedKeys[i] = paddedKey
	}
	proofResult := ProofResult{Address: address}
	for _, paddedKey := range paddedKeys {
		formattedKey := append(types.StateKey(address), paddedKey[:]...)
		qres := a.root.Query(ctx, abci.RequestQuery{
			Path:   "/" + a.keeper.GetStoreKey().Name() + "/key",
			Data:   formattedKey,
			Height: block.Block.Height,
			Prove:  true,
		})
		if qres.Code != 0 {
			return nil, fmt.Errorf("storage proof query failed (code %d): %s", qres.Code, qres.Log)
		}
		proofResult.HexValues = append(proofResult.HexValues, hex.EncodeToString(qres.Value))
		proofResult.StorageProof = append(proofResult.StorageProof, qres.ProofOps)
	}

	return &proofResult, nil
}

// findQueryableKVStore unwraps known KVStore wrappers until it reaches a types.Queryable
// (classic IAVL, store/v2 memiavl commitment, or future proof-capable roots).
// Go only allows `x := s.(type)` inside a type switch, not before it. Nil parents are
// handled by the `s == nil` check on the next iteration; nil *Store receivers are
// guarded in each pointer case so we never call methods on nil.
func findQueryableKVStore(s sdk.KVStore) (storetypes.Queryable, error) {
	const maxDepth = 64
	for range maxDepth {
		if s == nil {
			return nil, errNoProofCapableQueryableKVStore
		}
		switch cast := s.(type) {
		case *cachekv.Store:
			if cast == nil {
				return nil, errNoProofCapableQueryableKVStore
			}
			s = cast.GetParent()
			continue
		case *gigacachekv.Store:
			if cast == nil {
				return nil, errNoProofCapableQueryableKVStore
			}
			s = cast.GetParent()
			continue
		case *tracekv.Store:
			if cast == nil {
				return nil, errNoProofCapableQueryableKVStore
			}
			s = cast.Parent()
			continue
		case prefix.Store:
			s = cast.Parent()
			continue
		case *prefix.Store:
			if cast == nil {
				return nil, errNoProofCapableQueryableKVStore
			}
			s = cast.Parent()
			continue
		}
		if q, ok := s.(storetypes.Queryable); ok {
			return q, nil
		}
		return nil, errNoProofCapableQueryableKVStore
	}
	return nil, fmt.Errorf("%w: exceeded unwrap depth", errNoProofCapableQueryableKVStore)
}

func (a *StateAPI) GetNonce(ctx context.Context, address common.Address) uint64 {
	startTime := time.Now()
	defer recordMetrics(ctx, "eth_getNonce", a.connectionType, startTime)
	return a.keeper.GetNonce(a.ctxProvider(LatestCtxHeight), address)
}

// decodeHash parses a hex-encoded 32-byte hash. The input may optionally
// be prefixed by 0x and can have a byte length up to 32.
func decodeHash(s string) (h common.Hash, inputLength int, err error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}
	if (len(s) & 1) > 0 {
		s = "0" + s
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return common.Hash{}, 0, errors.New("hex string invalid")
	}
	if len(b) > 32 {
		return common.Hash{}, len(b), errors.New("hex string too long, want at most 32 bytes")
	}
	return common.BytesToHash(b), len(b), nil
}
