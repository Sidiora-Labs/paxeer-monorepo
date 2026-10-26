package gaskv_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/gaskv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
)

func TestGasKVHasAndDeleteEmitNoLegacyTiming(t *testing.T) {
	sink := oteltest.InstallLegacySink(t)
	telemetry.MeasureSince(time.Now(), "gaskv_test", "control")
	controlKeys := sink.Keys()
	require.Equal(t, []string{"oteltest.gaskv_test.control"}, controlKeys)

	mem := dbadapter.Store{DB: dbm.NewMemDB()}
	t.Cleanup(func() { require.NoError(t, mem.Close()) })
	key := []byte("present")
	missing := []byte("missing")
	mem.Set(key, []byte("value"))
	st := gaskv.NewStore(mem, types.NewInfiniteMultiplierGasMeter(1, 1), types.KVGasConfig(), "", nil)

	require.True(t, st.Has(key))
	require.Equal(t, controlKeys, sink.Keys(), "Has must not emit legacy timing for a present key")
	require.False(t, st.Has(missing))
	require.Equal(t, controlKeys, sink.Keys(), "Has must not emit legacy timing for a missing key")

	st.Delete(key)
	require.False(t, mem.Has(key))
	require.Equal(t, controlKeys, sink.Keys(), "Delete must not emit legacy timing for a present key")
	st.Delete(missing)
	require.Equal(t, controlKeys, sink.Keys(), "Delete must not emit legacy timing for a missing key")
}

func TestGasKVHasAndDeleteChargeConfiguredGas(t *testing.T) {
	custom := types.KVGasConfig()
	custom.HasCost = 37
	custom.DeleteCost = 91

	for _, tc := range []struct {
		name   string
		config types.GasConfig
	}{
		{name: "default", config: types.KVGasConfig()},
		{name: "custom", config: custom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, present := range []bool{true, false} {
				name := "missing"
				if present {
					name = "present"
				}
				t.Run(name, func(t *testing.T) {
					mem := dbadapter.Store{DB: dbm.NewMemDB()}
					t.Cleanup(func() { require.NoError(t, mem.Close()) })
					key := []byte("key")
					if present {
						mem.Set(key, []byte("value"))
					}
					meter := types.NewMultiplierGasMeter(tc.config.HasCost+tc.config.DeleteCost, 1, 1)
					st := gaskv.NewStore(mem, meter, tc.config, "", nil)

					before := meter.GasConsumed()
					require.Equal(t, present, st.Has(key))
					require.Equal(t, tc.config.HasCost, meter.GasConsumed()-before)

					before = meter.GasConsumed()
					st.Delete(key)
					require.Equal(t, tc.config.DeleteCost, meter.GasConsumed()-before)
					require.False(t, mem.Has(key))
				})
			}
		})
	}
}
