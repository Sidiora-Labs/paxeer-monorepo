package keeper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// BindTransferMetricsForTest replaces package instruments for a serial test.
// Call the returned closure before shutting down the supplied provider.
func BindTransferMetricsForTest(provider metric.MeterProvider) func() {
	savedMeter, savedMetrics := meter, ibcTransferMetrics
	boundMeter := provider.Meter("ibc_transfer_keeper")
	boundMetrics := newTransferMetrics(boundMeter)
	meter, ibcTransferMetrics = boundMeter, boundMetrics
	return func() {
		meter, ibcTransferMetrics = savedMeter, savedMetrics
	}
}

func TestTransferMetricsProviderRestoration(t *testing.T) {
	savedMeter, savedMetrics := meter, ibcTransferMetrics
	globalProvider := otel.GetMeterProvider()
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewManualReader()))
			t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
			restore := BindTransferMetricsForTest(provider)
			t.Cleanup(restore)

			boundMeter := provider.Meter("ibc_transfer_keeper")
			expected := newTransferMetrics(boundMeter)
			require.True(t, meter == boundMeter, "keeper meter must use the supplied provider")
			require.True(t, ibcTransferMetrics.txMsgIbcTransfer == expected.txMsgIbcTransfer)
			require.True(t, ibcTransferMetrics.ibcTransferPacketReceive == expected.ibcTransferPacketReceive)
			require.True(t, ibcTransferMetrics.ibcTransferSend == expected.ibcTransferSend)
			require.True(t, ibcTransferMetrics.ibcTransferReceive == expected.ibcTransferReceive)
			require.True(t, meter != savedMeter)
			require.True(t, ibcTransferMetrics.txMsgIbcTransfer != savedMetrics.txMsgIbcTransfer)
			require.True(t, ibcTransferMetrics.ibcTransferPacketReceive != savedMetrics.ibcTransferPacketReceive)
			require.True(t, ibcTransferMetrics.ibcTransferSend != savedMetrics.ibcTransferSend)
			require.True(t, ibcTransferMetrics.ibcTransferReceive != savedMetrics.ibcTransferReceive)
			require.True(t, otel.GetMeterProvider() == globalProvider)

			restore()
			require.True(t, meter == savedMeter, "keeper meter must be restored")
			require.True(t, ibcTransferMetrics.txMsgIbcTransfer == savedMetrics.txMsgIbcTransfer)
			require.True(t, ibcTransferMetrics.ibcTransferPacketReceive == savedMetrics.ibcTransferPacketReceive)
			require.True(t, ibcTransferMetrics.ibcTransferSend == savedMetrics.ibcTransferSend)
			require.True(t, ibcTransferMetrics.ibcTransferReceive == savedMetrics.ibcTransferReceive)
			require.True(t, otel.GetMeterProvider() == globalProvider)
		})
	}
}
