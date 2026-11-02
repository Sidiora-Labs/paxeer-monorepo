package types

import (
	"math"
	"sync"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/stretchr/testify/require"
)

func TestStoretypesEmitsNoLegacyTelemetry(t *testing.T) {
	sink := oteltest.InstallLegacySink(t)
	telemetry.IncrCounter(1, "storetypes_test", "counter_control")
	telemetry.SetGauge(1, "storetypes_test", "gauge_control")
	controlKeys := sink.Keys()
	require.Equal(t, []string{
		"oteltest.storetypes_test.counter_control",
		"oteltest.storetypes_test.gauge_control",
	}, controlKeys)
	reader := oteltest.Install()

	t.Run("out_of_gas", func(t *testing.T) {
		before := gasExceeded(reader)
		gasMeter := &basicGasMeter{limit: 10, lock: &sync.Mutex{}}
		const descriptor = "legacy_metrics_out_of_gas"

		require.PanicsWithValue(t, ErrorOutOfGas{Descriptor: descriptor}, func() {
			gasMeter.ConsumeGas(11, descriptor)
		})
		require.Equal(t, Gas(11), gasMeter.GasConsumed())
		require.True(t, gasMeter.IsPastLimit())
		require.Equal(t, int64(1), gasExceeded(reader)-before)
		require.Equal(t, controlKeys, sink.Keys())
	})

	t.Run("gas_overflow", func(t *testing.T) {
		before := gasExceeded(reader)
		gasMeter := &basicGasMeter{limit: math.MaxUint64, consumed: math.MaxUint64, lock: &sync.Mutex{}}
		const descriptor = "legacy_metrics_overflow"

		require.PanicsWithValue(t, ErrorGasOverflow{Descriptor: descriptor}, func() {
			gasMeter.ConsumeGas(1, descriptor)
		})
		require.Equal(t, Gas(math.MaxUint64), gasMeter.GasConsumed())
		require.Equal(t, int64(1), gasExceeded(reader)-before)
		require.Equal(t, controlKeys, sink.Keys())
	})

	t.Run("cache_eviction", func(t *testing.T) {
		cache := NewBoundedCache(mapCacheBackend{make(map[string]*CValue)}, 2)
		cache.Set("dirty-first", NewCValue([]byte("first"), true))
		cache.Set("dirty-second", NewCValue([]byte("second"), true))
		cache.Set("dirty-third", NewCValue([]byte("third"), true))
		before, found := boundedCache(reader)
		require.True(t, found)
		require.Zero(t, before)
		require.Equal(t, 3, cache.Len())
		require.Equal(t, controlKeys, sink.Keys())
		cache.DeleteAll()

		cache.Set("first", NewCValue([]byte("first"), false))
		cache.Set("second", NewCValue([]byte("second"), false))
		require.Equal(t, 2, cache.Len())
		cache.Set("third", NewCValue([]byte("third"), false))

		require.Equal(t, 2, cache.Len())
		value, ok := cache.Get("third")
		require.True(t, ok)
		require.Equal(t, []byte("third"), value.Value())
		_, firstPresent := cache.Get("first")
		_, secondPresent := cache.Get("second")
		require.NotEqual(t, firstPresent, secondPresent)
		after, found := boundedCache(reader)
		require.True(t, found)
		require.Equal(t, int64(1), after-before)
		require.Equal(t, controlKeys, sink.Keys())
	})
}
