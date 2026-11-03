package tasks

import (
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestTasksEmitsNoLegacyTelemetry(t *testing.T) {
	sink := oteltest.InstallLegacySink(t)
	reader := oteltest.Install()
	beforeRetries := retries(reader)
	beforeIncarnations := incarnations(reader)

	ctx := initTestCtx(true)
	ctx.MultiStore().GetKVStore(testStoreKey).Set(itemKey, []byte("initial"))
	secondRead := make(chan struct{})
	var attempts [2]atomic.Int32
	deliver := func(ctx sdk.Context, req types.RequestDeliverTxV2, tx sdk.Tx, checksum [32]byte) (res types.ResponseDeliverTx) {
		defer abortRecoveryFunc(&res)
		attempt := attempts[ctx.TxIndex()].Add(1)
		kv := ctx.MultiStore().GetKVStore(testStoreKey)
		previous := kv.Get(itemKey)
		if ctx.TxIndex() == 0 {
			<-secondRead
		} else if attempt == 1 {
			close(secondRead)
		}
		kv.Set(itemKey, req.Tx)
		return types.ResponseDeliverTx{Info: string(previous)}
	}
	tracer := trace.NewNoopTracerProvider().Tracer("scheduler-legacy-metrics-test")
	s := NewScheduler(2, tracing.NewTracingInfo(tracer, true), deliver)

	responses, err := s.ProcessAll(ctx, requestList(2))
	require.NoError(t, err)
	require.Len(t, responses, 2)
	require.Equal(t, "initial", responses[0].Info)
	require.Equal(t, "0", responses[1].Info)
	require.Equal(t, []byte("1"), ctx.MultiStore().GetKVStore(testStoreKey).Get(itemKey))
	require.Equal(t, int32(1), attempts[0].Load())
	require.Equal(t, int32(2), attempts[1].Load())
	require.Equal(t, int64(1), retries(reader)-beforeRetries)
	require.Equal(t, int64(1), incarnations(reader)-beforeIncarnations)
	keys := sink.Keys()
	require.NotContains(t, keys, "oteltest.scheduler.retries")
	require.NotContains(t, keys, "oteltest.scheduler.incarnations")
}
