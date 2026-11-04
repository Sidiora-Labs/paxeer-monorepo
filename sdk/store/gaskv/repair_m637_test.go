package gaskv_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/gaskv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair637Parent(t *testing.T) dbadapter.Store {
	t.Helper()
	parent := dbadapter.Store{DB: dbm.NewMemDB()}
	t.Cleanup(func() { require.NoError(t, parent.Close()) })
	return parent
}

func repair637Config() types.GasConfig {
	config := types.KVGasConfig()
	config.IterNextCostFlat = 7
	config.ReadCostPerByte = 3
	config.DeleteCost = 11
	return config
}

func repair637ScanGas(t *testing.T, parent types.KVStore, config types.GasConfig, start, end []byte) types.Gas {
	t.Helper()
	meter := types.NewInfiniteMultiplierGasMeter(1, 1)
	store := gaskv.NewStore(parent, meter, config, "reference", nil)
	iter := store.Iterator(start, end)
	for ; iter.Valid(); iter.Next() {
		iter.Key()
		iter.Value()
	}
	require.NoError(t, iter.Close())
	return meter.GasConsumed()
}

func repair637Trace(t *testing.T, tracer gaskv.IStoreTracer) sdk.StoreTraceDump {
	t.Helper()
	var dump sdk.StoreTraceDump
	require.NoError(t, json.Unmarshal(tracer.DerivePrestateToJson(), &dump))
	return dump
}

func TestRepairM_637_BulkRanges(t *testing.T) {
	for _, config := range []struct {
		name string
		gas  types.GasConfig
	}{
		{"default", types.KVGasConfig()},
		{"custom", repair637Config()},
		{"transient", types.TransientGasConfig()},
	} {
		t.Run(config.name, func(t *testing.T) {
			for _, bounds := range []struct {
				name       string
				start, end []byte
			}{
				{"partial", []byte("b"), []byte("d")},
				{"unbounded_start", nil, []byte("b")},
				{"unbounded_end", []byte("c"), nil},
				{"all", nil, nil},
				{"empty", []byte("b"), []byte("b")},
				{"absent", []byte("e"), []byte("f")},
				{"binary", []byte{0x80}, []byte{0xff}},
			} {
				t.Run(bounds.name, func(t *testing.T) {
					parent := repair637Parent(t)
					all := []string{"\x00", "a", "b", "c", "d", "\x80\x00", "\xff"}
					var want, encoded []string
					for _, key := range all {
						parent.Set([]byte(key), []byte("value:"+key))
						if (bounds.start == nil || bytes.Compare([]byte(key), bounds.start) >= 0) &&
							(bounds.end == nil || bytes.Compare([]byte(key), bounds.end) < 0) {
							want = append(want, key)
							encoded = append(encoded, hex.EncodeToString([]byte(key)))
						}
					}
					readGas := repair637ScanGas(t, parent, config.gas, bounds.start, bounds.end)
					if len(want) == 0 {
						require.Equal(t, config.gas.IterNextCostFlat, readGas)
					}
					meter := types.NewMultiplierGasMeter(readGas, 1, 1)
					tracer := sdk.NewStoreTracer()
					store := gaskv.NewStore(parent, meter, config.gas, "bulk", tracer)
					require.Equal(t, want, store.GetAllKeyStrsInRange(bounds.start, bounds.end))
					require.Equal(t, readGas, meter.GasConsumed())
					for _, key := range all {
						require.Equal(t, []byte("value:"+key), parent.Get([]byte(key)))
					}
					dump := repair637Trace(t, tracer)
					require.Equal(t, 1, dump.Stats["iterator"].Count)
					require.Equal(t, len(want), dump.Stats["iteratorNext"].Count)
					require.Equal(t, len(want), dump.Stats["iteratorValue"].Count)
					require.Len(t, dump.Modules["bulk"].Iterators, 1)
					require.Equal(t, encoded, dump.Modules["bulk"].Iterators[0].Keys)
					for _, key := range want {
						require.Equal(t, hex.EncodeToString([]byte("value:"+key)), dump.Modules["bulk"].Reads[hex.EncodeToString([]byte(key))])
					}

					deleteGas := readGas + types.Gas(len(want))*config.gas.DeleteCost
					meter = types.NewMultiplierGasMeter(deleteGas, 1, 1)
					tracer = sdk.NewStoreTracer()
					store = gaskv.NewStore(parent, meter, config.gas, "bulk", tracer)
					require.NoError(t, store.DeleteAll(bounds.start, bounds.end))
					require.Equal(t, deleteGas, meter.GasConsumed())
					dump = repair637Trace(t, tracer)
					require.Equal(t, len(want), dump.Stats["delete"].Count)
					require.Equal(t, len(want), dump.Stats["iteratorValue"].Count)
					require.Equal(t, encoded, dump.Modules["bulk"].Iterators[0].Keys)
					for _, key := range all {
						deleted := false
						for _, target := range want {
							deleted = deleted || target == key
						}
						if deleted {
							require.Nil(t, parent.Get([]byte(key)))
						} else {
							require.Equal(t, []byte("value:"+key), parent.Get([]byte(key)))
						}
					}
				})
			}
		})
	}
}

func TestRepairM_637_ExactGasAndMutationSafety(t *testing.T) {
	parent := repair637Parent(t)
	parent.Set([]byte("aa"), []byte("one"))
	parent.Set([]byte("bb"), []byte("four"))
	config := repair637Config()
	meter := types.NewMultiplierGasMeter(69, 1, 1)
	store := gaskv.NewStore(parent, meter, config, "bulk", nil)
	keys := store.GetAllKeyStrsInRange(nil, nil)
	require.Equal(t, []string{"aa", "bb"}, keys)
	// The existing iterator charges three flat seeks and byte costs for
	// the first entry on both initial seek and Next, then the second on Next.
	require.Equal(t, types.Gas(3*7+3*(2*5+6)), meter.GasConsumed())
	meter = types.NewMultiplierGasMeter(91, 1, 1)
	store = gaskv.NewStore(parent, meter, config, "bulk", nil)
	require.NoError(t, store.DeleteAll(nil, nil))
	require.Equal(t, types.Gas(91), meter.GasConsumed())
	require.False(t, parent.Has([]byte("aa")))
	require.False(t, parent.Has([]byte("bb")))
	parent.Set([]byte("aa"), []byte("replacement"))
	require.Equal(t, []string{"aa", "bb"}, keys, "returned keys must survive iterator closure and parent mutation")
}

func TestRepairM_637_OutOfGas(t *testing.T) {
	for _, operation := range []string{"read", "delete"} {
		t.Run(operation, func(t *testing.T) {
			for _, limit := range []types.Gas{0, 22, 68, 69, 80} {
				if operation == "read" && limit >= 69 {
					continue
				}
				t.Run(types.NewMultiplierGasMeter(limit, 1, 1).String(), func(t *testing.T) {
					parent := repair637Parent(t)
					parent.Set([]byte("aa"), []byte("one"))
					parent.Set([]byte("bb"), []byte("four"))
					meter := types.NewMultiplierGasMeter(limit, 1, 1)
					tracer := sdk.NewStoreTracer()
					store := gaskv.NewStore(parent, meter, repair637Config(), "bulk", tracer)
					func() {
						defer func() { require.IsType(t, types.ErrorOutOfGas{}, recover()) }()
						if operation == "read" {
							store.GetAllKeyStrsInRange(nil, nil)
						} else {
							require.NoError(t, store.DeleteAll(nil, nil))
						}
						t.Error("bulk operation exceeded its gas budget without panicking")
					}()
					require.Greater(t, meter.GasConsumed(), limit)
					deleted := operation == "delete" && limit == 80
					require.Equal(t, !deleted, parent.Has([]byte("aa")))
					require.Equal(t, []byte("four"), parent.Get([]byte("bb")))
					wantDeletes := 0
					if deleted {
						wantDeletes = 1
					}
					require.Equal(t, wantDeletes, repair637Trace(t, tracer).Stats["delete"].Count)
					// MemDB writes require the iterator's read lock to be released.
					parent.Set([]byte("zz"), []byte("still usable"))
					require.NoError(t, gaskv.NewStore(parent, types.NewInfiniteMultiplierGasMeter(1, 1), repair637Config(), "", nil).DeleteAll(nil, nil))
				})
			}
		})
	}
}

func TestRepairM_637_ContextPrefixAccounting(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "root"
		if cached {
			name = "nested_cache"
		}
		t.Run(name, func(t *testing.T) {
			db := dbm.NewMemDB()
			root := rootmulti.NewStore(db)
			t.Cleanup(func() { require.NoError(t, root.Close()) })
			key := types.NewKVStoreKey("evm")
			root.MountStoreWithDB(key, types.StoreTypeDB, nil)
			require.NoError(t, root.LoadLatestVersion())
			ctx := sdk.NewContext(root, tmproto.Header{}, false)
			if cached {
				ctx = ctx.WithMultiStore(root.CacheMultiStore().CacheMultiStore())
			}
			parent := ctx.MultiStore().GetKVStore(key)
			parent.Set([]byte("state/a"), []byte("one"))
			parent.Set([]byte("state/b"), []byte("four"))
			parent.Set([]byte("state0/a"), []byte("outside"))
			gas := repair637ScanGas(t, parent, types.KVGasConfig(), []byte("state/"), []byte("state0")) + 2*types.KVGasConfig().DeleteCost
			ctx = ctx.WithGasMeter(types.NewMultiplierGasMeter(gas, 1, 1)).WithIsTracing(true)
			// PurgePrefix's production Context and prefix-store path, without
			// importing the keeper package's native Wasm runtime dependencies.
			store := prefix.NewStore(ctx.KVStore(key), []byte("state/"))
			require.NoError(t, store.DeleteAll(nil, nil))
			require.Equal(t, gas, ctx.GasMeter().GasConsumed())
			require.Nil(t, parent.Get([]byte("state/a")))
			require.Nil(t, parent.Get([]byte("state/b")))
			require.Equal(t, []byte("outside"), parent.Get([]byte("state0/a")))
			dump := repair637Trace(t, ctx.StoreTracer())
			require.Equal(t, 2, dump.Modules["evm"].Stats["delete"].Count)
			require.Equal(t, 2, dump.Modules["evm"].Stats["iteratorValue"].Count)
			require.Equal(t, []string{hex.EncodeToString([]byte("state/a")), hex.EncodeToString([]byte("state/b"))}, dump.Modules["evm"].Iterators[0].Keys)
			require.Equal(t, hex.EncodeToString([]byte("state/")), dump.Modules["evm"].Iterators[0].Start)
			require.Equal(t, hex.EncodeToString([]byte("state0")), dump.Modules["evm"].Iterators[0].End)

			parent.Set([]byte("state/a"), []byte("one"))
			parent.Set([]byte("state/b"), []byte("four"))
			scanGas := gas - 2*types.KVGasConfig().DeleteCost
			ctx = ctx.WithGasMeter(types.NewMultiplierGasMeter(scanGas, 1, 1)).WithIsTracing(true)
			store = prefix.NewStore(ctx.KVStore(key), []byte("state/"))
			require.PanicsWithValue(t, types.ErrorOutOfGas{Descriptor: types.GasDeleteDesc}, func() {
				_ = store.DeleteAll(nil, nil)
			})
			require.Equal(t, []byte("one"), parent.Get([]byte("state/a")))
			require.Equal(t, []byte("four"), parent.Get([]byte("state/b")))
			require.Equal(t, []byte("outside"), parent.Get([]byte("state0/a")))
			dump = repair637Trace(t, ctx.StoreTracer())
			require.Equal(t, 0, dump.Modules["evm"].Stats["delete"].Count)
			require.Equal(t, 2, dump.Modules["evm"].Stats["iteratorValue"].Count)
		})
	}
}

func TestRepairM_637_NoLegacyTiming(t *testing.T) {
	sink := oteltest.InstallLegacySink(t)
	telemetry.MeasureSince(time.Now(), "repair_m637", "control")
	control := sink.Keys()
	require.Equal(t, []string{"oteltest.repair_m637.control"}, control)
	parent := repair637Parent(t)
	parent.Set([]byte("aa"), []byte("one"))
	store := gaskv.NewStore(parent, types.NewMultiplierGasMeter(10000, 1, 1), types.KVGasConfig(), "bulk", nil)
	require.True(t, store.Has([]byte("aa")))
	require.Equal(t, []string{"aa"}, store.GetAllKeyStrsInRange(nil, nil))
	require.NoError(t, store.DeleteAll(nil, nil))
	store.Delete([]byte("missing"))
	require.Equal(t, control, sink.Keys())
}
