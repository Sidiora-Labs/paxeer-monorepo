package sequencer

import (
	"context"
	"crypto/ed25519"
	"math"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestReceiptLatencyP99UnderBound(t *testing.T) {
	const transfers = 500
	const bound = 50 * time.Millisecond
	const metricName = "sequencer_receipt_latency_seconds"
	ctx := context.Background()
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	params := app.DefaultConsensusParams
	configured := *params
	blockParams := *params.Block
	blockParams.MaxGas = (transfers+1)*21_000 + 1_000_000
	blockParams.MaxGasWanted = blockParams.MaxGas
	configured.Block = &blockParams
	app.DefaultConsensusParams = &configured
	t.Cleanup(func() { app.DefaultConsensusParams = params })

	sender := utils.NewSigner()
	primary := newFailoverRehearsalNode(t, sender)
	secondary := newFailoverRehearsalNode(t, sender)
	primaryPub, err := primary.consensus.GetPubKey(ctx)
	require.NoError(t, err)
	standbyReceipter, err := NewReceipter(secondary.source.exec, secondary.source.app,
		&secondary.source.app.EvmKeeper, primary.source.key, 12)
	require.NoError(t, err)
	public := primary.source.key.PublicKey()
	standby, err := NewStandby(ctx, StandbyConfig{
		WAL: secondary.log, Executor: NewAppReplayer(secondary.source.exec, standbyReceipter),
		ConsensusKey: secondary.consensus, ReceiptKey: secondary.source.key.private,
		PrimaryConsensusKey: primaryPub, PrimaryReceiptKey: ed25519.PublicKey(public[:]),
		CommittedHeight: 1, LastSeq: secondary.opts.LastSeq,
	})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- standby.Serve(listener) }()
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
		select {
		case err := <-served:
			require.ErrorIs(t, err, net.ErrClosed)
		case <-time.After(2 * time.Second):
			t.Error("loopback standby did not stop")
		}
	})

	boundaries := make([]float64, 0, 56)
	for ms := 1; ms < 50; ms++ {
		boundaries = append(boundaries, (time.Duration(ms) * time.Millisecond).Seconds())
	}
	boundaries = append(boundaries, math.Nextafter(bound.Seconds(), 0), bound.Seconds(), 0.1, 0.2, 0.5, 1, 2)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(sdkmetric.NewView(
		sdkmetric.Instrument{Name: metricName, Kind: sdkmetric.InstrumentKindHistogram},
		sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{Boundaries: boundaries}},
	), sdkmetric.NewView(
		sdkmetric.Instrument{Name: "sequencer_wal_sync_seconds", Kind: sdkmetric.InstrumentKindHistogram},
		sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{Boundaries: boundaries}},
	), sdkmetric.NewView(
		sdkmetric.Instrument{Name: "sequencer_standby_ack_seconds", Kind: sdkmetric.InstrumentKindHistogram},
		sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{Boundaries: boundaries}},
	)))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(ctx)) })
	metrics, err := NewMetrics(provider.Meter("receipt-latency"))
	require.NoError(t, err)
	replicator, err := NewReplicator(ReplicatorConfig{
		WAL: primary.log, Dial: DialStandby(listener.Addr().String(), 2*time.Second),
		ReplicationTimeout: 2 * time.Second, OnWALSync: metrics.ObserveWALSync,
	})
	require.NoError(t, err)
	t.Cleanup(replicator.Close)
	openMs := time.Now().Add(time.Minute).UnixMilli()
	require.NoError(t, primary.source.exec.Open(2, openMs))
	ack, err := replicator.Release(ctx,
		&wal.Lease{Epoch: 1, ExpiryMs: uint64(openMs) + 30_000},
		&wal.Open{Height: 2, OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)},
	)
	require.NoError(t, err)
	require.Equal(t, AckOK, ack.Status)
	receipter, err := NewReceipter(primary.source.exec, primary.source.app,
		&primary.source.app.EvmKeeper, primary.source.key, 12)
	require.NoError(t, err)
	source := NewReceiptSource(transfers)
	accepter := NewAccepter(receipter, replicator, source, metrics)
	txs := make([][]byte, transfers)
	for i := range txs {
		txs[i] = consensusFenceTransfer(t, primary.source.app, sender, uint64(i))
	}
	receipts := make([][]byte, transfers)
	standbySeqs := make([]uint64, transfers)
	runtime.GC()
	for i, tx := range txs {
		receipt, err := accepter.Accept(tx)
		require.NoError(t, err, "transfer %d", i)
		receipts[i] = receipt
		standbySeqs[i] = standby.LastSeq()
	}
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))
	for i, receipt := range receipts {
		verified, err := verify.ReceiptSignature(receipt, public)
		require.NoError(t, err, "transfer %d", i)
		require.Zero(t, verified.Receipt.ResultCode, "transfer %d", i)
		seq := primary.opts.LastSeq + uint64(i) + 1
		require.Equal(t, seq, verified.Receipt.GlobalSequence)
		require.Equal(t, seq, standbySeqs[i])
		require.NoError(t, standby.Err())
		send, err := source.SequencerSend(common.Hash(verified.Receipt.ActivityID))
		require.NoError(t, err)
		require.NotNil(t, send)
		require.Equal(t, receipt, send.Receipt)
	}

	var histogram metricdata.Histogram[float64]
	found := false
	for _, scope := range data.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == metricName {
				require.False(t, found, "duplicate receipt latency instrument")
				require.Equal(t, "s", metric.Unit)
				var ok bool
				histogram, ok = metric.Data.(metricdata.Histogram[float64])
				require.True(t, ok, "receipt latency must be a Float64Histogram")
				found = true
			}
		}
	}
	require.True(t, found, "Accepter must record receipt latency")
	require.Equal(t, metricdata.CumulativeTemporality, histogram.Temporality)
	require.Len(t, histogram.DataPoints, 1)
	point := histogram.DataPoints[0]
	require.Equal(t, uint64(transfers), point.Count)
	require.Positive(t, point.Sum)
	require.Equal(t, boundaries, point.Bounds)
	require.Len(t, point.BucketCounts, len(point.Bounds)+1)
	var total uint64
	for _, count := range point.BucketCounts {
		total += count
	}
	require.Equal(t, point.Count, total)
	rank := (99*point.Count + 99) / 100
	var cumulative uint64
	p99Upper := math.Inf(1)
	for i, upper := range point.Bounds {
		cumulative += point.BucketCounts[i]
		if cumulative >= rank {
			p99Upper = upper
			break
		}
	}
	t.Logf("%s: count=%d p99 upper bound=%.9f ms", metricName, point.Count, p99Upper*1000)
	if maximum, ok := point.Max.Value(); ok {
		t.Logf("maximum receipt latency=%.9f ms", maximum*1000)
	}
	t.Logf("latency sum=%.6f seconds bounds=%v bucket counts=%v", point.Sum, point.Bounds, point.BucketCounts)
	for _, diagnostic := range []struct {
		name  string
		count uint64
	}{
		{"sequencer_wal_sync_seconds", transfers + 1},
		{"sequencer_standby_ack_seconds", transfers},
	} {
		found := false
		for _, scope := range data.ScopeMetrics {
			for _, metric := range scope.Metrics {
				if metric.Name != diagnostic.name {
					continue
				}
				require.False(t, found, "duplicate %s instrument", diagnostic.name)
				found = true
				require.Equal(t, "s", metric.Unit, diagnostic.name)
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				require.True(t, ok, "%s must be a Float64Histogram", diagnostic.name)
				require.Equal(t, metricdata.CumulativeTemporality, histogram.Temporality)
				require.Len(t, histogram.DataPoints, 1, diagnostic.name)
				point := histogram.DataPoints[0]
				require.Equal(t, diagnostic.count, point.Count, diagnostic.name)
				require.Equal(t, boundaries, point.Bounds, diagnostic.name)
				require.Len(t, point.BucketCounts, len(point.Bounds)+1, diagnostic.name)
				rank := (99*point.Count + 99) / 100
				var cumulative uint64
				p99Upper := math.Inf(1)
				for i, upper := range point.Bounds {
					cumulative += point.BucketCounts[i]
					if cumulative >= rank {
						p99Upper = upper
						break
					}
				}
				t.Logf("%s: count=%d p99 upper bound=%.9f ms sum=%.6f seconds bounds=%v bucket counts=%v",
					diagnostic.name, point.Count, p99Upper*1000, point.Sum, point.Bounds, point.BucketCounts)
				if maximum, ok := point.Max.Value(); ok {
					t.Logf("%s: maximum=%.9f ms", diagnostic.name, maximum*1000)
				}
			}
		}
		require.True(t, found, "%s must be recorded", diagnostic.name)
	}
	require.Less(t, p99Upper, bound.Seconds(), "receipt latency p99 must be strictly below 50 ms")
}
