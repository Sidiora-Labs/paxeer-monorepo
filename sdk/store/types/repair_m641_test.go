package types_test

import (
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/gaskv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/mem"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	"github.com/stretchr/testify/require"
)

func repairM641AnteMeter(t *testing.T, numerator, denominator, limit uint64, mode string) storetypes.GasMeter {
	t.Helper()
	parent := sdk.NewInfiniteGasMeter(numerator, denominator)
	ctx := sdk.Context{}.WithBlockHeight(1).WithGasMeter(parent)
	if mode == "genesis" {
		ctx = ctx.WithBlockHeight(0)
	}
	ctx = ante.SetGasMeter(mode == "simulation", ctx, limit, nil)
	meter := ctx.GasMeter()
	require.NotSame(t, parent, meter)
	n, d := meter.Multiplier()
	require.Equal(t, numerator, n)
	require.Equal(t, denominator, d)
	if mode == "finite" {
		require.Equal(t, limit, meter.Limit())
	} else {
		require.Zero(t, meter.Limit())
	}
	require.Zero(t, meter.GasConsumed())
	return meter
}

func TestRepairM_641_FullWidthRatiosThroughAnte(t *testing.T) {
	cases := []struct {
		name                           string
		amount, numerator, denominator uint64
		want                           uint64
	}{
		{"overflow_before_division", 1 << 63, 2, 2, 1 << 63},
		{"full_width_product", math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64},
		{"large_fraction", math.MaxUint64, 2, 3, 12297829382473034410},
		{"fraction_rounds_down", 7, 3, 2, 10},
		{"fraction_below_one", 1, 1, 2, 0},
		{"large_denominator", math.MaxUint64, 1, math.MaxUint64, 1},
		{"zero_amount", 0, math.MaxUint64, 1, 0},
		{"zero_numerator", math.MaxUint64, 0, 1, 0},
	}
	for _, mode := range []string{"finite", "simulation", "genesis"} {
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				meter := repairM641AnteMeter(t, tc.numerator, tc.denominator, math.MaxUint64, mode)
				parent := mem.NewStore()
				t.Cleanup(func() { require.NoError(t, parent.DB.Close()) })
				key, value := []byte("ratio"), []byte("stored")
				parent.Set(key, value)
				store := gaskv.NewStore(parent, meter, storetypes.GasConfig{HasCost: tc.amount}, "repair_m641", nil)
				require.True(t, store.Has(key))
				require.Equal(t, tc.want, meter.GasConsumed())
				require.Equal(t, tc.want, meter.GasConsumedToLimit())
				require.Equal(t, value, parent.Get(key))
				meter.RefundGas(tc.amount, "refund scaled store charge")
				require.Zero(t, meter.GasConsumed())
				require.False(t, meter.IsPastLimit())
				require.False(t, meter.IsOutOfGas())
			})
		}
	}
}

func TestRepairM_641_RejectZeroDenominatorAtConstruction(t *testing.T) {
	constructors := []struct {
		name string
		make func(uint64, uint64) storetypes.GasMeter
	}{
		{"finite", func(n, d uint64) storetypes.GasMeter { return storetypes.NewMultiplierGasMeter(100, n, d) }},
		{"infinite", storetypes.NewInfiniteMultiplierGasMeter},
		{"sdk_finite", func(n, d uint64) storetypes.GasMeter { return sdk.NewGasMeter(100, n, d) }},
		{"sdk_infinite", sdk.NewInfiniteGasMeter},
	}
	for _, constructor := range constructors {
		t.Run(constructor.name, func(t *testing.T) {
			for _, numerator := range []uint64{0, 1, math.MaxUint64} {
				require.PanicsWithValue(t, "gas multiplier denominator must be positive", func() {
					constructor.make(numerator, 0)
				})
			}
		})
	}
}

func TestRepairM_641_QuotientOverflowSaturates(t *testing.T) {
	for _, mode := range []string{"finite", "simulation", "genesis"} {
		t.Run(mode, func(t *testing.T) {
			for _, ratio := range [][2]uint64{{2, 1}, {math.MaxUint64, math.MaxUint64 - 1}} {
				meter := repairM641AnteMeter(t, ratio[0], ratio[1], 100, mode)
				require.PanicsWithValue(t, storetypes.ErrorGasOverflow{Descriptor: "scaled overflow"}, func() {
					meter.ConsumeGas(math.MaxUint64, "scaled overflow")
				})
				require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
				if mode == "finite" {
					require.Equal(t, uint64(100), meter.GasConsumedToLimit())
					require.True(t, meter.IsPastLimit())
					require.True(t, meter.IsOutOfGas())
				} else {
					require.Equal(t, uint64(math.MaxUint64), meter.GasConsumedToLimit())
					require.False(t, meter.IsPastLimit())
					require.False(t, meter.IsOutOfGas())
				}
				require.PanicsWithValue(t, storetypes.ErrorGasOverflow{Descriptor: "still saturated"}, func() {
					meter.ConsumeGas(math.MaxUint64, "still saturated")
				})
				require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
				meter.RefundGas(1, "refund after saturation")
				require.Equal(t, uint64(math.MaxUint64-ratio[0]/ratio[1]), meter.GasConsumed())
			}
		})
	}
}

func TestRepairM_641_AccumulationOverflowSaturates(t *testing.T) {
	for _, mode := range []string{"finite", "simulation", "genesis"} {
		t.Run(mode, func(t *testing.T) {
			meter := repairM641AnteMeter(t, 2, 2, math.MaxUint64, mode)
			meter.ConsumeGas(1<<63, "first half")
			require.Equal(t, uint64(1<<63), meter.GasConsumed())
			require.PanicsWithValue(t, storetypes.ErrorGasOverflow{Descriptor: "total overflow"}, func() {
				meter.ConsumeGas(1<<63, "total overflow")
			})
			require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
			require.Equal(t, uint64(math.MaxUint64), meter.GasConsumedToLimit())
			meter.ConsumeGas(0, "zero at maximum")
			require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
			meter.RefundGas(1<<63, "full-width refund")
			require.Equal(t, uint64(math.MaxUint64-(1<<63)), meter.GasConsumed())
		})
	}
}

func TestRepairM_641_RefundOverflowPreservesConsumption(t *testing.T) {
	for _, mode := range []string{"finite", "simulation", "genesis"} {
		t.Run(mode, func(t *testing.T) {
			meter := repairM641AnteMeter(t, 2, 1, 100, mode)
			meter.ConsumeGas(5, "seed")
			require.Equal(t, uint64(10), meter.GasConsumed())
			require.PanicsWithValue(t, storetypes.ErrorGasOverflow{Descriptor: "refund overflow"}, func() {
				meter.RefundGas(1<<63, "refund overflow")
			})
			require.Equal(t, uint64(10), meter.GasConsumed())
			require.PanicsWithValue(t, storetypes.ErrorNegativeGasConsumed{Descriptor: "over refund"}, func() {
				meter.RefundGas(6, "over refund")
			})
			require.Equal(t, uint64(10), meter.GasConsumed())
			meter.RefundGas(5, "valid refund")
			require.Zero(t, meter.GasConsumed())
		})
	}
}

func TestRepairM_641_FractionalFiniteLimitThroughAnte(t *testing.T) {
	meter := repairM641AnteMeter(t, 3, 2, 10, "finite")
	meter.ConsumeGas(7, "at limit")
	require.Equal(t, uint64(10), meter.GasConsumed())
	require.False(t, meter.IsPastLimit())
	require.True(t, meter.IsOutOfGas())
	require.PanicsWithValue(t, storetypes.ErrorOutOfGas{Descriptor: "past limit"}, func() {
		meter.ConsumeGas(1, "past limit")
	})
	require.Equal(t, uint64(11), meter.GasConsumed())
	require.Equal(t, uint64(10), meter.GasConsumedToLimit())
	meter.RefundGas(1, "restore limit")
	require.Equal(t, uint64(10), meter.GasConsumed())
}

func TestRepairM_641_InfiniteRefundRegression(t *testing.T) {
	meter := storetypes.NewInfiniteMultiplierGasMeter(1, 1)
	meter.ConsumeGas(10, "initial charge")
	meter.RefundGas(1, "initial refund")
	require.Equal(t, uint64(9), meter.GasConsumed())
	meter.ConsumeGas(math.MaxUint64/2, "half maximum")
	require.PanicsWithValue(t, storetypes.ErrorGasOverflow{Descriptor: "overflow"}, func() {
		meter.ConsumeGas(math.MaxUint64/2+2, "overflow")
	})
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumedToLimit())
	meter.RefundGas(0, "zero refund at saturation")
	require.Equal(t, uint64(math.MaxUint64), meter.GasConsumed())

	refundMeter := storetypes.NewInfiniteMultiplierGasMeter(1, 1)
	refundMeter.ConsumeGas(9, "below maximum")
	require.PanicsWithValue(t, storetypes.ErrorNegativeGasConsumed{Descriptor: "over refund"}, func() {
		refundMeter.RefundGas(refundMeter.GasConsumed()+1, "over refund")
	})
	require.Equal(t, uint64(9), refundMeter.GasConsumed())
	refundMeter.RefundGas(9, "valid refund after rejection")
	require.Zero(t, refundMeter.GasConsumed())
}
