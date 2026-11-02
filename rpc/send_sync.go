package evmrpc

import (
	"context"
	"math"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type transactionNotIncludedError struct {
	hash common.Hash
}

func (e *transactionNotIncludedError) Error() string          { return "transaction not yet included" }
func (e *transactionNotIncludedError) ErrorCode() int         { return 4 }
func (e *transactionNotIncludedError) ErrorData() interface{} { return e.hash.Hex() }

var (
	_ rpc.Error     = (*transactionNotIncludedError)(nil)
	_ rpc.DataError = (*transactionNotIncludedError)(nil)
)

func (s *SendAPI) SendRawTransactionSync(ctx context.Context, input hexutil.Bytes, timeoutMs *hexutil.Uint64) (result map[string]interface{}, returnErr error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "eth_sendRawTransactionSync", s.connectionType, startTime, returnErr, recover())
	}()

	hash, err := s.SendRawTransaction(ctx, input)
	if err != nil {
		return nil, err
	}

	if timeout, ok := s.methodTimeout.Get(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	timeout := s.sendConfig.syncTimeout
	if timeoutMs != nil {
		requested := time.Duration(math.MaxInt64)
		if uint64(*timeoutMs) <= uint64(math.MaxInt64/int64(time.Millisecond)) {
			requested = time.Duration(*timeoutMs) * time.Millisecond
		}
		if timeout <= 0 || requested < timeout {
			timeout = requested
		}
	}
	waitCtx := ctx
	if timeout > 0 || timeoutMs != nil {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeoutCause(ctx, timeout, &transactionNotIncludedError{hash: hash})
		defer cancel()
	}

	receipts := NewTransactionAPI(
		s.tmClient, s.keeper, s.ctxProvider, s.txConfigProvider, s.homeDir,
		s.connectionType, s.methodTimeout, s.backend.watermarks,
		s.backend.globalBlockCache, s.backend.cacheCreationMutex,
	)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if waitCtx.Err() != nil {
			return nil, context.Cause(waitCtx)
		}
		receipt, err := receipts.GetTransactionReceipt(waitCtx, hash)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if waitCtx.Err() != nil {
			return nil, context.Cause(waitCtx)
		}
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			return receipt, nil
		}
		select {
		case <-waitCtx.Done():
		case <-ticker.C:
		}
	}
}
