package types

import (
	"sync"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	"github.com/stretchr/testify/require"
)

const storeTypesMeter = "paxcosmos_store_types"

func gasExceeded(reader *oteltest.Reader) int64 {
	return reader.CounterSum(storeTypesMeter, "gas_exceeded")
}

func boundedCache(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(storeTypesMeter, "last_evicted_keys_bounded_cache")
}

func TestGasMeterOutOfGasRecordsGasExceeded(t *testing.T) {
	reader := oteltest.Install()
	before := gasExceeded(reader)
	gasMeter := &basicGasMeter{limit: 10, lock: &sync.Mutex{}}
	const descriptor = "otel_metrics_out_of_gas"

	require.PanicsWithValue(t, ErrorOutOfGas{Descriptor: descriptor}, func() {
		gasMeter.ConsumeGas(11, descriptor)
	})
	require.Equal(t, Gas(11), gasMeter.GasConsumed())
	require.Equal(t, Gas(10), gasMeter.Limit())
	require.True(t, gasMeter.IsPastLimit())
	after := gasExceeded(reader)
	require.Equal(t, int64(1), after-before)
}

func TestBoundedCacheEvictionRecordsEvictedKeys(t *testing.T) {
	reader := oteltest.Install()
	cache := NewBoundedCache(mapCacheBackend{make(map[string]*CValue)}, 2)

	cache.Set("dirty-first", NewCValue([]byte("first"), true))
	cache.Set("dirty-second", NewCValue([]byte("second"), true))
	cache.Set("dirty-third", NewCValue([]byte("third"), true))
	before, found := boundedCache(reader)
	require.True(t, found)
	require.Zero(t, before)
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
	require.Equal(t, int64(1), after)
	require.Equal(t, int64(1), after-before)
}
