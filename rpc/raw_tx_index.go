package evmrpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

// JSON-RPC names shared with metrics and the client parity contract.
const (
	eth_getRawTransactionByBlockNumberAndIndex = "eth_getRawTransactionByBlockNumberAndIndex"
	eth_getRawTransactionByBlockHashAndIndex   = "eth_getRawTransactionByBlockHashAndIndex"
)

func (r *RawTransactionAPI) GetRawTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (result hexutil.Bytes, returnErr error) {
	t := r.transactions
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, eth_getRawTransactionByBlockNumberAndIndex, t.connectionType, startTime, returnErr, recover())
		var overflowErr txUint32OverflowError
		if errors.As(returnErr, &overflowErr) {
			returnErr = nil
		}
	}()
	idx, err := txIndexToUint32(index)
	if err != nil {
		return nil, err
	}
	if blockNr == rpc.PendingBlockNumber && t.watermarks != nil && t.watermarks.pending != nil {
		snapshot, err := t.watermarks.pending.Snapshot()
		if err != nil {
			return nil, fmt.Errorf("pending block unavailable: %w", err)
		}
		hashes, err := pendingBlockTxHashes(snapshot)
		if err != nil {
			return nil, err
		}
		if uint64(idx) >= uint64(len(hashes)) {
			return nil, nil
		}
		if _, ok := t.watermarks.pending.PendingTx(hashes[idx]); !ok {
			return nil, fmt.Errorf("pending transaction %s unavailable", hashes[idx].Hex())
		}
		return r.GetRawTransactionByHash(ctx, hashes[idx])
	}
	blockNumber, err := getBlockNumber(ctx, t.tmClient, t.watermarks, blockNr)
	if err != nil {
		return nil, err
	}
	block, err := blockByNumberOrNullForJSONRPC(ctx, t.tmClient, t.watermarks, blockNumber, 1)
	if err != nil || block == nil {
		return nil, err
	}
	return r.rawTransactionWithBlock(ctx, block, idx)
}

func (r *RawTransactionAPI) GetRawTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (result hexutil.Bytes, returnErr error) {
	t := r.transactions
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, eth_getRawTransactionByBlockHashAndIndex, t.connectionType, startTime, returnErr, recover())
		var overflowErr txUint32OverflowError
		if errors.As(returnErr, &overflowErr) {
			returnErr = nil
		}
	}()
	block, err := blockByHashOrNullForJSONRPC(ctx, t.tmClient, t.watermarks, blockHash[:], 1)
	if err != nil || block == nil {
		return nil, err
	}
	idx, err := txIndexToUint32(index)
	if err != nil {
		return nil, err
	}
	return r.rawTransactionWithBlock(ctx, block, idx)
}

func (r *RawTransactionAPI) rawTransactionWithBlock(ctx context.Context, block *coretypes.ResultBlock, index uint32) (hexutil.Bytes, error) {
	msgs := r.transactions.getFilteredMsgs(block)
	if uint64(index) >= uint64(len(msgs)) {
		return nil, nil
	}
	msg, ok := msgs[index].msg.(*types.MsgEVMTransaction)
	if !ok {
		return nil, errors.New("transaction is not an EVM transaction and thus cannot be represented in _getTransaction* endpoints")
	}
	tx, _ := msg.AsTransaction()
	if tx == nil {
		return nil, errors.New("transaction contains malformed EVM message data")
	}
	if !domainAllowsTransaction(ctx, r.transactions.domainIndex, tx) {
		return nil, nil
	}
	return tx.MarshalBinary()
}
