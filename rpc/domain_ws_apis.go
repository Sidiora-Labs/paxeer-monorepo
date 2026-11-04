package evmrpc

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/export"
	"github.com/ethereum/go-ethereum/rpc"
)

type domainBlockAPI struct {
	*BlockAPI
	scope DomainScope
}

func (a *domainBlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error) {
	return a.BlockAPI.GetBlockTransactionCountByNumber(WithDomainScope(ctx, a.scope), number)
}

func (a *domainBlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error) {
	return a.BlockAPI.GetBlockTransactionCountByHash(WithDomainScope(ctx, a.scope), blockHash)
}

func (a *domainBlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error) {
	return a.BlockAPI.GetBlockByHash(WithDomainScope(ctx, a.scope), blockHash, fullTx)
}

func (a *domainBlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error) {
	return a.BlockAPI.GetBlockByNumber(WithDomainScope(ctx, a.scope), number, fullTx)
}

func (a *domainBlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error) {
	return a.BlockAPI.GetBlockReceipts(WithDomainScope(ctx, a.scope), blockNrOrHash)
}

type domainTransactionAPI struct {
	*TransactionAPI
	scope DomainScope
}

func (a *domainTransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error) {
	return a.TransactionAPI.GetTransactionReceipt(WithDomainScope(ctx, a.scope), hash)
}

func (a *domainTransactionAPI) GetVMError(ctx context.Context, hash common.Hash) (result string, returnErr error) {
	return a.TransactionAPI.GetVMError(WithDomainScope(ctx, a.scope), hash)
}

func (a *domainTransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error) {
	return a.TransactionAPI.GetTransactionByBlockNumberAndIndex(WithDomainScope(ctx, a.scope), blockNr, txIndex)
}

func (a *domainTransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error) {
	return a.TransactionAPI.GetTransactionByBlockHashAndIndex(WithDomainScope(ctx, a.scope), blockHash, txIndex)
}

func (a *domainTransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (result *export.RPCTransaction, returnErr error) {
	return a.TransactionAPI.GetTransactionByHash(WithDomainScope(ctx, a.scope), hash)
}

func (a *domainTransactionAPI) GetTransactionErrorByHash(ctx context.Context, hash common.Hash) (result string, returnErr error) {
	return a.TransactionAPI.GetTransactionErrorByHash(WithDomainScope(ctx, a.scope), hash)
}

func (a *domainTransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Uint64, returnErr error) {
	return a.TransactionAPI.GetTransactionCount(WithDomainScope(ctx, a.scope), address, blockNrOrHash)
}

func (a *domainTransactionAPI) Sign(ctx context.Context, addr common.Address, data hexutil.Bytes) (result hexutil.Bytes, returnErr error) {
	return a.TransactionAPI.Sign(WithDomainScope(ctx, a.scope), addr, data)
}

type domainRawTransactionAPI struct {
	*RawTransactionAPI
	scope DomainScope
}

func (a *domainRawTransactionAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (result hexutil.Bytes, returnErr error) {
	return a.RawTransactionAPI.GetRawTransactionByHash(WithDomainScope(ctx, a.scope), hash)
}

func (a *domainRawTransactionAPI) GetRawTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (result hexutil.Bytes, returnErr error) {
	return a.RawTransactionAPI.GetRawTransactionByBlockNumberAndIndex(WithDomainScope(ctx, a.scope), blockNr, index)
}

func (a *domainRawTransactionAPI) GetRawTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (result hexutil.Bytes, returnErr error) {
	return a.RawTransactionAPI.GetRawTransactionByBlockHashAndIndex(WithDomainScope(ctx, a.scope), blockHash, index)
}

type domainFilterAPI struct {
	*FilterAPI
	scope DomainScope
}

func (a *domainFilterAPI) NewFilter(
	ctx context.Context,
	crit filters.FilterCriteria,
) (id rpc.ID, err error) {
	return a.FilterAPI.NewFilter(WithDomainScope(ctx, a.scope), crit)
}

func (a *domainFilterAPI) NewBlockFilter(
	ctx context.Context,
) (id rpc.ID, err error) {
	return a.FilterAPI.NewBlockFilter(WithDomainScope(ctx, a.scope))
}

func (a *domainFilterAPI) NewPendingTransactionFilter(
	ctx context.Context,
	fullTx *bool,
) (id rpc.ID, err error) {
	return a.FilterAPI.NewPendingTransactionFilter(WithDomainScope(ctx, a.scope), fullTx)
}

func (a *domainFilterAPI) GetFilterChanges(
	ctx context.Context,
	filterID rpc.ID,
) (res interface{}, err error) {
	return a.FilterAPI.GetFilterChanges(WithDomainScope(ctx, a.scope), filterID)
}

func (a *domainFilterAPI) GetFilterLogs(
	ctx context.Context,
	filterID rpc.ID,
) (res []*FilterLog, err error) {
	return a.FilterAPI.GetFilterLogs(WithDomainScope(ctx, a.scope), filterID)
}

func (a *domainFilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error) {
	return a.FilterAPI.GetLogs(WithDomainScope(ctx, a.scope), crit)
}

func (a *domainFilterAPI) UninstallFilter(
	ctx context.Context,
	filterID rpc.ID,
) (res bool) {
	return a.FilterAPI.UninstallFilter(WithDomainScope(ctx, a.scope), filterID)
}
