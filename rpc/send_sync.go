package evmrpc

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func (s *SendAPI) SendRawTransactionSync(ctx context.Context, input hexutil.Bytes) (map[string]interface{}, error) {
	hash, err := s.SendRawTransaction(ctx, input)
	if err != nil {
		return nil, err
	}

	if s.sendConfig.syncTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.sendConfig.syncTimeout)
		defer cancel()
	}
	if timeout, ok := s.methodTimeout.Get(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
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
		receipt, err := receipts.GetTransactionReceipt(ctx, hash)
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			return receipt, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
