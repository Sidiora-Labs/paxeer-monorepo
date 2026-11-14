package tasks

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	gigastore "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/stretchr/testify/require"
)

func iterationDeliver(key sdk.StoreKey, reverse bool) func(sdk.Context, abci.RequestDeliverTxV2, sdk.Tx, [32]byte) abci.ResponseDeliverTx {
	return func(ctx sdk.Context, req abci.RequestDeliverTxV2, _ sdk.Tx, _ [32]byte) abci.ResponseDeliverTx {
		store := ctx.KVStore(key)
		var iter storetypes.Iterator
		if reverse {
			iter = store.ReverseIterator(nil, nil)
		} else {
			iter = store.Iterator(nil, nil)
		}
		var data strings.Builder
		for ; iter.Valid(); iter.Next() {
			fmt.Fprintf(&data, "%s=%s;", iter.Key(), iter.Value())
		}
		if err := iter.Error(); err != nil {
			panic(err)
		}
		if err := iter.Close(); err != nil {
			panic(err)
		}
		store.Set(req.Tx, []byte(data.String()))
		return abci.ResponseDeliverTx{Data: []byte(data.String())}
	}
}

func TestDeliverSchedulerIteration(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			ctx, key := newSchedulerCancellationContext(t, context.Background(), true)
			parent := ctx.MultiStore().GetKVStore(key)
			parent.Set([]byte("a"), []byte("parent"))
			parent.Set([]byte("d"), []byte("deleted"))
			ctx.GigaMultiStore().SetGigaKVStores(func(sk storetypes.StoreKey, _ storetypes.KVStore) storetypes.KVStore {
				return gigastore.NewStore(ctx.MultiStore().GetKVStore(sk), sk, 0)
			})
			shared := ctx.GigaKVStore(key)
			shared.Set([]byte("a"), []byte("unwritten"))
			shared.Set([]byte("b"), []byte("added"))
			shared.Delete([]byte("d"))
			sequential, sequentialKey := newSchedulerCancellationContext(t, context.Background(), false)
			sequential.KVStore(sequentialKey).Set([]byte("a"), []byte("unwritten"))
			sequential.KVStore(sequentialKey).Set([]byte("b"), []byte("added"))
			reqs := schedulerCancellationRequests(2)
			var want []abci.ResponseDeliverTx
			for _, req := range reqs {
				want = append(want, iterationDeliver(sequentialKey, reverse)(sequential, req.Request, nil, [32]byte{}))
			}
			require.Contains(t, string(want[1].Data), string(reqs[0].Request.Tx)+"=")
			readerDone := make(chan struct{})
			var once sync.Once
			deliver := iterationDeliver(key, reverse)
			s := NewDeliverScheduler(2, tracing.NewTracingInfo(nil, false), func(ctx sdk.Context, req abci.RequestDeliverTxV2, tx sdk.Tx, sum [32]byte) abci.ResponseDeliverTx {
				if ctx.TxIndex() == 0 {
					<-readerDone
				}
				response := deliver(ctx, req, tx, sum)
				if ctx.TxIndex() == 1 {
					once.Do(func() { close(readerDone) })
				}
				return response
			})
			got, err := s.ProcessAll(ctx, reqs)
			require.NoError(t, err)
			require.Equal(t, want, got)
			require.Positive(t, s.(*scheduler).metrics.retries)
			for _, keyBytes := range [][]byte{[]byte("a"), []byte("b"), []byte("d"), reqs[0].Request.Tx, reqs[1].Request.Tx} {
				require.Equal(t, sequential.KVStore(sequentialKey).Get(keyBytes), shared.Get(keyBytes))
			}
			require.Equal(t, []byte("parent"), parent.Get([]byte("a")))
			require.PanicsWithValue(t, gigastore.ErrIteratorUnsupported, func() { shared.Iterator(nil, nil) })
		})
	}
}

func TestFastSchedulerIterationAbort(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			ctx, key := newSchedulerCancellationContext(t, context.Background(), true)
			s := NewScheduler(1, tracing.NewTracingInfo(nil, false), iterationDeliver(key, reverse)).(*scheduler)
			s.synchronous = true
			require.PanicsWithValue(t, gigastore.ErrIteratorUnsupported, func() {
				_, _ = s.ProcessAll(ctx, schedulerCancellationRequests(1))
			})
		})
	}
}
