package evmrpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	receiptpkg "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// JSON-RPC names shared with metrics and the client parity contract.
const (
	eth_getRawTransactionByHash = "eth_getRawTransactionByHash"
)

type RawTransactionAPI struct {
	transactions *TransactionAPI
}

func NewRawTransactionAPI(
	tmClient client.LocalClient,
	k *keeper.Keeper,
	ctxProvider func(int64) sdk.Context,
	txConfigProvider func(int64) client.TxConfig,
	homeDir string,
	connectionType ConnectionType,
	methodTimeout utils.Option[time.Duration],
	watermarks *WatermarkManager,
	globalBlockCache BlockCache,
	cacheCreationMutex *sync.Mutex,
	enableKeyring ...bool,
) *RawTransactionAPI {
	return &RawTransactionAPI{
		transactions: NewTransactionAPI(tmClient, k, ctxProvider, txConfigProvider, homeDir, connectionType, methodTimeout, watermarks, globalBlockCache, cacheCreationMutex, enableKeyring...),
	}
}

func (r *RawTransactionAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (result hexutil.Bytes, returnErr error) {
	t := r.transactions
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, eth_getRawTransactionByHash, t.connectionType, startTime, returnErr, recover())
	}()
	if t.watermarks != nil && t.watermarks.pending != nil {
		if ptx, ok := t.watermarks.pending.PendingTx(hash); ok {
			if ptx.Height < 1 {
				return nil, fmt.Errorf("pending transaction height %d is not positive", ptx.Height)
			}
			if ptx.TxIndex < 0 {
				return nil, fmt.Errorf("pending transaction index %d is negative", ptx.TxIndex)
			}
			etx := getEthTxForTxBz(ptx.TxBytes, t.txConfigProvider(LatestCtxHeight).TxDecoder())
			if etx == nil {
				return nil, errors.New("transaction is not an EVM transaction and thus cannot be represented in _getTransaction* endpoints")
			}
			if !domainAllowsTransaction(ctx, t.domainIndex, etx) {
				return nil, nil
			}
			return rawEnvelopeForHash(etx, hash)
		}
	}
	if tx, ok := t.tmClient.EvmTxByHash(hash); ok {
		if etx := getEthTxForTxBz(tx, t.txConfigProvider(LatestCtxHeight).TxDecoder()); etx != nil {
			if !domainAllowsTransaction(ctx, t.domainIndex, etx) {
				return nil, nil
			}
			return rawEnvelopeForHash(etx, hash)
		}
	}

	receipt, err := t.keeper.GetReceipt(t.ctxProvider(LatestCtxHeight), hash)
	if err != nil {
		if errors.Is(err, receiptpkg.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if receipt == nil {
		return nil, fmt.Errorf("receipt lookup for %s returned nil", hash.Hex())
	}
	if scope, ok := DomainScopeFrom(ctx); ok {
		if !domainHashSet(t.domainIndex, t.ctxProvider(LatestCtxHeight), scope, receipt.BlockNumber)[hash] {
			return nil, nil
		}
	}
	if receipt.BlockNumber > math.MaxInt64 {
		return nil, fmt.Errorf("receipt block number %d exceeds int64", receipt.BlockNumber)
	}
	blockNumber := int64(receipt.BlockNumber)
	block, err := blockByNumberOrNullForJSONRPC(ctx, t.tmClient, t.watermarks, &blockNumber, 1)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	filteredMsgs := t.getFilteredMsgs(block)
	_, found, etx, _ := GetEvmTxIndex(t.ctxProvider(LatestCtxHeight), block, filteredMsgs, receipt.TransactionIndex, t.keeper, t.cacheCreationMutex, t.globalBlockCache)
	if !found {
		return nil, nil
	}
	if etx == nil {
		return nil, errors.New("transaction is not an EVM transaction and thus cannot be represented in _getTransaction* endpoints")
	}
	return rawEnvelopeForHash(etx, hash)
}

// rawEnvelopeForHash refuses a corrupt pending, mempool or receipt index rather
// than returning another transaction's otherwise valid binary envelope.
func rawEnvelopeForHash(tx *ethtypes.Transaction, hash common.Hash) (hexutil.Bytes, error) {
	if tx.Hash() != hash {
		return nil, fmt.Errorf("raw transaction hash mismatch: requested %s, found %s", hash.Hex(), tx.Hash().Hex())
	}
	return tx.MarshalBinary()
}
