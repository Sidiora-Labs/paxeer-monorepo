package types_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	amino "github.com/tendermint/go-amino"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/gaskv"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func repairM646Store(t *testing.T, tracer *sdk.StoreTracer, module string) (dbadapter.Store, *gaskv.Store) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	parent := dbadapter.Store{DB: db}
	return parent, gaskv.NewStore(parent, storetypes.NewInfiniteMultiplierGasMeter(1, 1), storetypes.KVGasConfig(), module, tracer)
}

func repairM646Bounds(t *testing.T, tracer *sdk.StoreTracer) sdk.StoreTraceDump {
	t.Helper()
	accesses, retained := 0, 0
	require.LessOrEqual(t, len(tracer.Modules), sdk.RepairM646MaxModules)
	for name, module := range tracer.Modules {
		require.LessOrEqual(t, len(name), sdk.RepairM646MaxModuleBytes)
		retained += len(name)
		accesses += len(module.Accesses)
		for _, access := range module.Accesses {
			retained += len(access.Key) + len(access.Value)
		}
		require.LessOrEqual(t, len(module.Iterators), sdk.RepairM646MaxIterators)
		require.Equal(t, len(module.Iterators), sdk.RepairM646TraceState(tracer).IteratorIndexCounts[name])
		for _, it := range module.Iterators {
			retained += len(it.Start) + len(it.End)
			require.LessOrEqual(t, len(it.Keys), sdk.RepairM646MaxIteratorKeys)
			for _, key := range it.Keys {
				retained += len(key)
			}
		}
	}
	require.Equal(t, accesses, sdk.RepairM646TraceState(tracer).Accesses)
	require.Equal(t, retained, sdk.RepairM646TraceState(tracer).Bytes)
	require.LessOrEqual(t, accesses, sdk.RepairM646MaxAccesses)
	require.LessOrEqual(t, retained, sdk.RepairM646MaxBytes)
	dump := tracer.Dump()
	entries, wireBytes := 0, 0
	for _, module := range dump.Modules {
		entries += len(module.Reads) + len(module.Has)
		for k, v := range module.Reads {
			wireBytes += len(k) + len(v)
		}
		for _, k := range module.Has {
			wireBytes += len(k)
		}
		for _, it := range module.Iterators {
			entries += 1 + len(it.Keys)
			wireBytes += len(it.Start) + len(it.End)
			for _, k := range it.Keys {
				wireBytes += len(k)
			}
		}
	}
	require.LessOrEqual(t, entries, sdk.RepairM646MaxWireEntries)
	require.LessOrEqual(t, wireBytes, sdk.RepairM646MaxWireBytes)
	wire := tracer.DerivePrestateToJson()
	require.LessOrEqual(t, len(wire), sdk.RepairM646MaxWireBytes+(1<<20), "payload and JSON framing must remain bounded")
	var consumer sdk.StoreTraceDump
	require.NoError(t, json.Unmarshal(wire, &consumer))
	profileWire, err := json.Marshal(dump)
	require.NoError(t, err)
	require.JSONEq(t, string(profileWire), string(wire), "both debug trace transports must expose the same sample, counters and truncation")
	consumerWire, err := json.Marshal(consumer)
	require.NoError(t, err)
	require.JSONEq(t, string(wire), string(consumerWire), "the consumer must preserve every wire field")
	return consumer
}

func TestRepairM_646_TraceReplay(t *testing.T) {
	tracer := sdk.NewStoreTracer()
	parent, store := repairM646Store(t, tracer, "bank")
	const rows = 1500
	for i := 0; i < rows; i++ {
		parent.Set([]byte(fmt.Sprintf("k/%05d", i)), []byte(fmt.Sprintf("value/%05d", i)))
	}
	large := bytes.Repeat([]byte{0xab}, sdk.RepairM646MaxBytes+1)
	parent.Set([]byte("large"), large)
	for _, reverse := range []bool{false, true} {
		var it storetypes.Iterator
		if reverse {
			it = store.ReverseIterator([]byte("k/"), []byte("l/"))
		} else {
			it = store.Iterator([]byte("k/"), []byte("l/"))
		}
		seen := 0
		for ; it.Valid(); it.Next() {
			require.True(t, bytes.HasPrefix(it.Key(), []byte("k/")))
			require.True(t, bytes.HasPrefix(it.Value(), []byte("value/")))
			seen++
		}
		require.Equal(t, rows, seen)
		require.NoError(t, it.Close())
	}
	for i := 0; i < 6000; i++ {
		require.Equal(t, []byte("value/00000"), store.Get([]byte("k/00000")))
	}
	for i := 0; i < rows; i++ {
		require.True(t, store.Has([]byte(fmt.Sprintf("k/%05d", i))))
	}
	for i := 0; i < 4; i++ {
		require.Equal(t, large, store.Get([]byte("large")))
	}
	for i := 0; i < sdk.RepairM646MaxIterators+3; i++ {
		it := store.Iterator([]byte("x/"), []byte("y/"))
		require.False(t, it.Valid())
		require.NoError(t, it.Close())
	}
	store.Set([]byte("post"), []byte("new"))
	store.Delete([]byte("post"))
	dump := repairM646Bounds(t, tracer)
	require.True(t, dump.Truncated)
	require.True(t, dump.Modules["bank"].Truncated)
	want := map[string]int{"get": 6004, "has": rows, "iterator": 2 + sdk.RepairM646MaxIterators + 3, "iteratorNext": rows * 2, "iteratorValue": rows * 2, "set": 1, "delete": 1}
	for op, count := range want {
		require.Equal(t, count, dump.Stats[op].Count, op)
		require.Equal(t, count, dump.Modules["bank"].Stats[op].Count, op)
	}
	for _, it := range tracer.Modules["bank"].Iterators[:2] {
		require.Equal(t, rows, it.NextCount)
		require.Len(t, it.Keys, sdk.RepairM646MaxIteratorKeys)
		require.True(t, it.Truncated)
	}
	require.Len(t, tracer.Modules["bank"].Iterators, sdk.RepairM646MaxIterators)
	require.Empty(t, parent.Get([]byte("post")))
}

func TestRepairM_646_TraceByteRetention(t *testing.T) {
	t.Run("large_values_rejected_before_clone", func(t *testing.T) {
		tracer := sdk.NewStoreTracer()
		parent, store := repairM646Store(t, tracer, "bank")
		large := bytes.Repeat([]byte{0xee}, sdk.RepairM646MaxBytes+1)
		parent.Set([]byte("large"), large)
		require.Equal(t, large, store.Get([]byte("large")))
		require.Empty(t, tracer.Modules["bank"].Accesses)
		// The production tracer must not allocate payload copies after a drop.
		largeKey := []byte("large")
		allocs := testing.AllocsPerRun(20, func() { tracer.Get(largeKey, large, "bank", time.Nanosecond) })
		require.Zero(t, allocs)
		store.Set([]byte("after"), []byte("changed"))
		require.Equal(t, []byte("changed"), store.Get([]byte("after")))
		dump := repairM646Bounds(t, tracer)
		require.True(t, dump.Truncated)
		require.Empty(t, dump.Modules["bank"].Reads, "an omitted write must never produce a later prestate read")
	})
	t.Run("aggregate_bytes", func(t *testing.T) {
		tracer := sdk.NewStoreTracer()
		parent, store := repairM646Store(t, tracer, "bank")
		value := bytes.Repeat([]byte{0xba}, 64<<10)
		parent.Set([]byte("key"), value)
		for i := 0; i < 100; i++ {
			require.Equal(t, value, store.Get([]byte("key")))
		}
		dump := repairM646Bounds(t, tracer)
		require.True(t, dump.Truncated)
		require.Equal(t, 100, dump.Stats["get"].Count)
		require.Less(t, sdk.RepairM646TraceState(tracer).Accesses, sdk.RepairM646MaxAccesses)
		require.NotEmpty(t, dump.Modules["bank"].Reads)
	})
	t.Run("large_iterator_bounds_and_keys", func(t *testing.T) {
		tracer := sdk.NewStoreTracer()
		parent, store := repairM646Store(t, tracer, "bank")
		key := bytes.Repeat([]byte{'x'}, sdk.RepairM646MaxBytes+1)
		parent.Set(key, []byte("v"))
		it := store.Iterator(key, nil)
		require.True(t, it.Valid())
		require.Equal(t, []byte("v"), it.Value())
		it.Next()
		require.NoError(t, it.Close())
		it = store.Iterator(nil, nil)
		require.True(t, it.Valid())
		require.Equal(t, []byte("v"), it.Value())
		it.Next()
		require.NoError(t, it.Close())
		dump := repairM646Bounds(t, tracer)
		require.True(t, dump.Truncated)
		require.Equal(t, 2, dump.Stats["iterator"].Count)
		require.Equal(t, 2, dump.Stats["iteratorValue"].Count)
		require.Len(t, tracer.Modules["bank"].Iterators, 1)
		require.Empty(t, tracer.Modules["bank"].Iterators[0].Keys)
		require.True(t, tracer.Modules["bank"].Iterators[0].Truncated)
	})
}

func TestRepairM_646_TraceWireAndLifecycle(t *testing.T) {
	tracer := sdk.NewStoreTracer()
	parent, store := repairM646Store(t, tracer, "bank")
	for i := 0; i < sdk.RepairM646MaxWireEntries+200; i++ {
		key := []byte(fmt.Sprintf("k/%05d", i))
		parent.Set(key, []byte("v"))
		if i%2 == 0 {
			require.Equal(t, []byte("v"), store.Get(key))
		} else {
			require.True(t, store.Has(key))
		}
	}
	dump := repairM646Bounds(t, tracer)
	require.False(t, sdk.RepairM646TraceState(tracer).Truncated, "wire truncation is independent of memory truncation")
	require.True(t, dump.Truncated)
	require.True(t, dump.Modules["bank"].Truncated)
	require.Equal(t, sdk.RepairM646MaxWireEntries, len(dump.Modules["bank"].Reads)+len(dump.Modules["bank"].Has))
	// Returned counters must not alias the live tracer.
	dump.Stats["get"] = sdk.OperationSummary{}
	dump.Modules["bank"].Stats["get"] = sdk.OperationSummary{}
	require.Equal(t, (sdk.RepairM646MaxWireEntries+200)/2, tracer.Dump().Stats["get"].Count)
	tracer.Clear()
	empty := repairM646Bounds(t, tracer)
	require.Empty(t, empty.Modules)
	require.Empty(t, empty.Stats)
	require.False(t, empty.Truncated)
	key, value := []byte("a"), []byte("old")
	tracer.Get(key, value, "bank", 3*time.Nanosecond)
	key[0], value[0] = 'z', 'X'
	store.Set([]byte("a"), []byte("new"))
	require.Equal(t, []byte("new"), store.Get([]byte("a")))
	store.Set([]byte("b"), []byte("new"))
	require.Equal(t, []byte("new"), store.Get([]byte("b")))
	store.Delete([]byte("c"))
	require.False(t, store.Has([]byte("c")))
	dump = repairM646Bounds(t, tracer)
	require.False(t, dump.Truncated)
	require.Equal(t, map[string]string{hex.EncodeToString([]byte("a")): hex.EncodeToString([]byte("old"))}, dump.Modules["bank"].Reads)
	require.Empty(t, dump.Modules["bank"].Has)
	require.Equal(t, 3, dump.Stats["get"].Count)
	require.GreaterOrEqual(t, dump.Stats["get"].TotalNanos, int64(3))
}

func TestRepairM_646_TraceModuleBounds(t *testing.T) {
	tracer := sdk.NewStoreTracer()
	parent, _ := repairM646Store(t, tracer, "unused")
	parent.Set([]byte("key"), []byte("value"))
	for i := 0; i < sdk.RepairM646MaxModules+20; i++ {
		store := gaskv.NewStore(parent, storetypes.NewInfiniteMultiplierGasMeter(1, 1), storetypes.KVGasConfig(), fmt.Sprintf("module/%d", i), tracer)
		require.Equal(t, []byte("value"), store.Get([]byte("key")))
	}
	store := gaskv.NewStore(parent, storetypes.NewInfiniteMultiplierGasMeter(1, 1), storetypes.KVGasConfig(), strings.Repeat("x", sdk.RepairM646MaxModuleBytes+1), tracer)
	require.True(t, store.Has([]byte("key")))
	dump := repairM646Bounds(t, tracer)
	require.Len(t, dump.Modules, sdk.RepairM646MaxModules)
	require.True(t, dump.Truncated)
	require.Equal(t, sdk.RepairM646MaxModules+20, dump.Stats["get"].Count)
	require.Equal(t, 1, dump.Stats["has"].Count)
}

func TestRepairM_646_UintWireRoundTrips(t *testing.T) {
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	cdc := amino.NewCodec()
	for _, input := range []*big.Int{big.NewInt(0), big.NewInt(1), big.NewInt(123456789), maximum} {
		t.Run(input.String(), func(t *testing.T) {
			producer := sdk.NewUintFromBigInt(input)
			var consumer sdk.Uint
			wire, err := json.Marshal(producer)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(wire, &consumer))
			require.True(t, producer.Equal(consumer))
			var custom sdk.CustomProtobufType = &producer
			wire = make([]byte, custom.Size())
			n, err := custom.MarshalTo(wire)
			require.NoError(t, err)
			require.Len(t, wire, n)
			require.NoError(t, consumer.Unmarshal(wire))
			require.True(t, producer.Equal(consumer))
			wire, err = producer.MarshalAmino()
			require.NoError(t, err)
			require.NoError(t, consumer.UnmarshalAmino(wire))
			require.True(t, producer.Equal(consumer))
			wire, err = cdc.MarshalBinaryBare(producer)
			require.NoError(t, err)
			require.NoError(t, cdc.UnmarshalBinaryBare(wire, &consumer))
			require.True(t, producer.Equal(consumer))
			wire, err = cdc.MarshalJSON(producer)
			require.NoError(t, err)
			require.NoError(t, cdc.UnmarshalJSON(wire, &consumer))
			require.True(t, producer.Equal(consumer))
			require.NoError(t, sdk.UintOverflow(consumer.BigInt()))
		})
	}
}

func TestRepairM_646_UintRejectsInvalidWireAtomically(t *testing.T) {
	over := new(big.Int).Lsh(big.NewInt(1), 256)
	cdc := amino.NewCodec()
	cases := []string{"-1", "-0x1", over.String(), "0x" + over.Text(16), "not-an-integer", "12no", strings.Repeat("9", 101)}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			wireJSON, err := json.Marshal(text)
			require.NoError(t, err)
			wireAmino, err := cdc.MarshalBinaryBare([]byte(text))
			require.NoError(t, err)
			decoders := map[string]func(*sdk.Uint) error{
				"json":         func(u *sdk.Uint) error { return json.Unmarshal(wireJSON, u) },
				"protobuf":     func(u *sdk.Uint) error { return u.Unmarshal([]byte(text)) },
				"amino_custom": func(u *sdk.Uint) error { return u.UnmarshalAmino([]byte(text)) },
				"amino_binary": func(u *sdk.Uint) error { return cdc.UnmarshalBinaryBare(wireAmino, u) },
				"amino_json":   func(u *sdk.Uint) error { return cdc.UnmarshalJSON(wireJSON, u) },
			}
			for name, decode := range decoders {
				t.Run(name, func(t *testing.T) {
					u := sdk.NewUint(42)
					alias, original := u, sdk.RepairM646UintPointer(u)
					require.Error(t, decode(&u))
					require.Same(t, original, sdk.RepairM646UintPointer(u), "failure must preserve the receiver identity")
					require.Equal(t, "42", u.String())
					require.Equal(t, "42", alias.String(), "failure must preserve shared copies")
					var zero sdk.Uint
					require.Error(t, decode(&zero))
					require.Nil(t, sdk.RepairM646UintPointer(zero), "failure must preserve an uninitialized receiver")
				})
			}
		})
	}
	for _, wire := range [][]byte{nil, []byte{}, []byte(`""`), []byte(`null`), []byte(`123`), []byte(`"12`)} {
		u := sdk.NewUint(42)
		require.Error(t, json.Unmarshal(wire, &u))
		require.Equal(t, "42", u.String())
	}
}

func TestRepairM_646_UintEmptyAndSuccessfulAssignment(t *testing.T) {
	for name, decode := range map[string]func(*sdk.Uint, []byte) error{
		"protobuf": (*sdk.Uint).Unmarshal,
		"amino":    (*sdk.Uint).UnmarshalAmino,
	} {
		t.Run(name, func(t *testing.T) {
			for _, empty := range [][]byte{nil, {}} {
				u := sdk.NewUint(42)
				alias := u
				require.NoError(t, decode(&u, empty))
				require.Equal(t, "0", u.String())
				require.Zero(t, u.BigInt().Sign())
				require.Equal(t, "42", alias.String())
				var zero sdk.Uint
				require.NoError(t, decode(&zero, empty))
				require.Equal(t, "0", zero.String())
			}
		})
	}
	for _, decode := range []func(*sdk.Uint) error{
		func(u *sdk.Uint) error { return json.Unmarshal([]byte(`"7"`), u) },
		func(u *sdk.Uint) error { return u.Unmarshal([]byte("7")) },
		func(u *sdk.Uint) error { return u.UnmarshalAmino([]byte("7")) },
	} {
		u := sdk.NewUint(42)
		alias := u
		require.NoError(t, decode(&u))
		require.Equal(t, "7", u.String())
		require.Equal(t, "42", alias.String(), "successful decode must not mutate copies of the previous sdk.Uint")
	}
}
