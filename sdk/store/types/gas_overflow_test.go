package types

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfiniteGasMeterOverflowSaturatesConsumed(t *testing.T) {
	t.Parallel()
	meter := NewInfiniteMultiplierGasMeter(1, 1)
	meter.ConsumeGas(math.MaxUint64-1, "near maximum")
	require.Equal(t, uint64(math.MaxUint64-1), meter.GasConsumed())

	require.PanicsWithValue(t, ErrorGasOverflow{Descriptor: "overflow"}, func() {
		meter.ConsumeGas(2, "overflow")
	})
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumedToLimit())

	meter.ConsumeGas(0, "zero after overflow")
	for _, amount := range []Gas{1, 2, math.MaxUint64} {
		require.PanicsWithValue(t, ErrorGasOverflow{Descriptor: "after overflow"}, func() {
			meter.ConsumeGas(amount, "after overflow")
		})
		require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
		require.Equal(t, uint64(math.MaxUint64), meter.GasConsumedToLimit())
	}
}

func TestInfiniteGasMeterRefundAfterOverflowPanics(t *testing.T) {
	t.Parallel()
	meter := NewInfiniteMultiplierGasMeter(1, 1)
	meter.ConsumeGas(math.MaxUint64, "maximum")
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
	require.PanicsWithValue(t, ErrorGasOverflow{Descriptor: "overflow"}, func() {
		meter.ConsumeGas(1, "overflow")
	})

	meter.RefundGas(1, "refund after overflow")
	require.Equal(t, uint64(math.MaxUint64-1), meter.GasConsumed())
	require.Equal(t, uint64(math.MaxUint64-1), meter.GasConsumedToLimit())

	meter.ConsumeGas(1, "restore maximum")
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
	require.PanicsWithValue(t, ErrorGasOverflow{Descriptor: "overflow after refund"}, func() {
		meter.ConsumeGas(1, "overflow after refund")
	})
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumedToLimit())
}
