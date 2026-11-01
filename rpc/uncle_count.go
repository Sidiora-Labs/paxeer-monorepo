package evmrpc

import (
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

func (a *UncleAPI) GetUncleCountByBlockNumber(ctx context.Context, blockNr rpc.BlockNumber) (result *hexutil.Uint, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getUncleCountByBlockNumber", a.connectionType, startTime, returnErr, recover())
	}()

	height, err := getBlockNumber(ctx, a.tmClient, a.watermarks, blockNr)
	if err != nil {
		return nil, err
	}
	block, err := blockByNumberOrNullForJSONRPC(ctx, a.tmClient, a.watermarks, height, 1)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	count := hexutil.Uint(0)
	return &count, nil
}

func (a *UncleAPI) GetUncleCountByBlockHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_getUncleCountByBlockHash", a.connectionType, startTime, returnErr, recover())
	}()

	block, err := blockByHashRespectingWatermarks(ctx, a.tmClient, a.watermarks, blockHash[:], 1)
	if errors.Is(err, ErrBlockNotFoundByHash) || errors.Is(err, ErrBlockHeightNotYetAvailable) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	count := hexutil.Uint(0)
	return &count, nil
}
