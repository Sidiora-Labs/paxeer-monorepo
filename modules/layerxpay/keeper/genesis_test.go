package keeper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/stretchr/testify/require"
)

func TestPayGenesisKeepsAgentStreamMetadata(t *testing.T) {
	for _, kind := range []string{"native", "pointer"} {
		t.Run(kind, func(t *testing.T) {
			h, _, token := newAgentStreamHarness(t, kind)
			payer, payee := h.actors[0].address, h.actors[1].address
			start := uint64(payHarnessTimeMs / 1000)
			id, err := h.keeper.OpenAgentStream(h.ctx, payer, payee, token, big.NewInt(2), start, start+100, big.NewInt(50))
			require.NoError(t, err)
			before, found, err := h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			exported := h.keeper.ExportGenesis(h.ctx)
			require.NoError(t, exported.Validate())
			var streamID [32]byte
			id.FillBytes(streamID[:])
			metadataKey := agentStreamMetadataKey(streamID)
			metadataValue := append(append(append([]byte{}, payer.Bytes()...), payee.Bytes()...), token.Bytes()...)
			require.Equal(t, []kv.Pair{{Key: metadataKey, Value: metadataValue}}, exported.AgentStreamMetadata)

			cloned := h.keeper.ExportGenesis(h.ctx)
			cloned.AgentStreamMetadata[0].Key[0] ^= 0xff
			cloned.AgentStreamMetadata[0].Value[0] ^= 0xff
			require.Equal(t, metadataValue, h.ctx.KVStore(h.storeKey).Get(metadataKey))
			require.Equal(t, exported, h.keeper.ExportGenesis(h.ctx))

			h.ctx.KVStore(h.storeKey).Delete(metadataKey)
			_, _, err = h.keeper.AgentStreamView(h.ctx, id)
			require.Error(t, err)
			removed := 0
			for _, record := range exported.KernelRecords {
				if bytes.HasPrefix(record.Key, types.StreamKVPrefix) {
					h.ctx.KVStore(h.hostKey).Delete(record.Key)
					removed++
				}
			}
			require.Positive(t, removed)
			_, found, err = h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.False(t, found)
			require.NoError(t, h.keeper.InitGenesis(h.ctx, *exported))
			require.Equal(t, exported, h.keeper.ExportGenesis(h.ctx))
			require.Equal(t, metadataValue, h.ctx.KVStore(h.storeKey).Get(metadataKey))
			after, found, err := h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, before, after)
			require.Equal(t, payer, after.Payer)
			require.Equal(t, payee, after.Payee)
			require.Equal(t, token, after.Token)

			h.ctx = h.ctx.WithBlockHeight(2).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 5000))
			paid, err := h.keeper.SettleAgentStream(h.ctx, payee, id)
			require.NoError(t, err)
			require.Equal(t, big.NewInt(10), paid)
			after, found, err = h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, paid, after.Settled)
		})
	}
}

func TestPayGenesisRefusesMalformedAgentStreamMetadata(t *testing.T) {
	key := append([]byte(types.AgentStreamMetadataPrefix), make([]byte, 32)...)
	key[len(key)-1] = 1
	value := bytes.Repeat([]byte{1}, 60)
	valid := kv.Pair{Key: key, Value: value}
	gs := types.DefaultGenesis()
	require.NoError(t, gs.Validate())
	gs.AgentStreamMetadata = []kv.Pair{valid}
	require.NoError(t, gs.Validate())
	wrongPrefix := bytes.Clone(key)
	wrongPrefix[0] ^= 0xff
	h := newPayHarness(t)
	for _, tc := range []struct {
		name    string
		records []kv.Pair
	}{
		{"short_value", []kv.Pair{{Key: key, Value: value[:59]}}},
		{"long_value", []kv.Pair{{Key: key, Value: append(bytes.Clone(value), 1)}}},
		{"short_key", []kv.Pair{{Key: key[:len(key)-1], Value: value}}},
		{"long_key", []kv.Pair{{Key: append(bytes.Clone(key), 1), Value: value}}},
		{"wrong_prefix", []kv.Pair{{Key: wrongPrefix, Value: value}}},
		{"zero_id", []kv.Pair{{Key: append([]byte(types.AgentStreamMetadataPrefix), make([]byte, 32)...), Value: value}}},
		{"duplicate_key", []kv.Pair{valid, {Key: bytes.Clone(key), Value: make([]byte, 60)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := *types.DefaultGenesis()
			invalid.AgentStreamMetadata = tc.records
			require.ErrorIs(t, invalid.Validate(), types.ErrInvalidGenesis)
			moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
			hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
			require.ErrorIs(t, h.keeper.InitGenesis(h.ctx, invalid), types.ErrInvalidGenesis)
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		})
	}
	t.Run("record_limit", func(t *testing.T) {
		bounded := *types.DefaultGenesis()
		for i := 1; i <= types.MaxKernelRecords+1; i++ {
			nextKey := bytes.Clone(key)
			binary.BigEndian.PutUint64(nextKey[len(nextKey)-8:], uint64(i))
			bounded.AgentStreamMetadata = append(bounded.AgentStreamMetadata, kv.Pair{Key: nextKey, Value: bytes.Clone(value)})
		}
		require.ErrorIs(t, bounded.Validate(), types.ErrInvalidGenesis)
		bounded.AgentStreamMetadata = bounded.AgentStreamMetadata[:types.MaxKernelRecords]
		require.NoError(t, bounded.Validate())
	})
}

func TestPayGenesisRoundTrip(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		h := newPayHarness(t)
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		recordsBefore := h.keeper.ExportGenesis(h.ctx).KernelRecords
		require.NoError(t, h.keeper.InitGenesis(h.ctx, *types.DefaultGenesis()))
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		exported := h.keeper.ExportGenesis(h.ctx)
		require.Equal(t, types.DefaultParams(), exported.Params)
		require.Equal(t, recordsBefore, exported.KernelRecords)
		require.NoError(t, exported.Validate())
		require.Len(t, payStoreSnapshot(h.ctx, h.storeKey), 2)
		require.Equal(t, []byte{1}, h.ctx.KVStore(h.storeKey).Get([]byte(serviceIndexVersionKey)))
	})

	t.Run("records", func(t *testing.T) {
		h := newPayHarness(t)
		gs := *h.keeper.ExportGenesis(h.ctx)
		gs.Params = types.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}
		for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
			key, err := types.KernelModuleEnableKey(moduleID)
			require.NoError(t, err)
			gs.KernelRecords = append(gs.KernelRecords, kv.Pair{Key: key, Value: payGateEnabledRecord()})
		}
		for moduleID := uint16(1); moduleID <= 11; moduleID++ {
			for _, key := range [][]byte{{0xff, 0x00}, {0x00, 0xff}} {
				if moduleID == types.ServiceModuleID {
					// Service bytes are now owned and must pass their real record decoder.
					fixture := serviceViewFixtures(t)[0]
					id := fixture.id
					id[31] = key[0]
					wire := bytes.Clone(fixture.wire)
					copy(wire[1:33], id[:])
					gs.KernelRecords = append(gs.KernelRecords, kv.Pair{Key: serviceKey(fixture.prefix, id), Value: wire})
					continue
				}
				record := kv.Pair{Key: types.KernelKVKey(moduleID, key), Value: []byte{byte(moduleID), key[0], key[1]}}
				require.Equal(t, append([]byte{0x01, 0x00, byte(moduleID)}, key...), record.Key)
				gs.KernelRecords = append(gs.KernelRecords, record)
			}
		}
		gs.KernelRecords = append(gs.KernelRecords,
			kv.Pair{Key: types.KernelKVKey(types.StreamModuleID, bytes.Repeat([]byte{0xa5}, types.MaxKernelKeyBytes)), Value: bytes.Repeat([]byte{0x5a}, types.MaxKernelValueBytes)},
			kv.Pair{Key: types.KernelKVKey(types.BudgetModuleID, []byte("empty")), Value: []byte{}},
			kv.Pair{Key: types.KernelKVKey(types.BudgetModuleID, []byte("nil"))},
		)
		sort.Slice(gs.KernelRecords, func(i, j int) bool { return bytes.Compare(gs.KernelRecords[i].Key, gs.KernelRecords[j].Key) > 0 })
		hostWant := payStoreSnapshot(h.ctx, h.hostKey)
		var recordsWant []kv.Pair
		for _, record := range gs.KernelRecords {
			value := append([]byte{}, record.Value...)
			hostWant[string(record.Key)] = value
			switch binary.BigEndian.Uint16(record.Key[1:3]) {
			case 2, 3, 4, 5, 7:
				recordsWant = append(recordsWant, kv.Pair{Key: bytes.Clone(record.Key), Value: value})
			}
		}
		sort.Slice(recordsWant, func(i, j int) bool { return bytes.Compare(recordsWant[i].Key, recordsWant[j].Key) < 0 })
		require.NoError(t, h.keeper.InitGenesis(h.ctx, gs))
		require.Equal(t, gs.Params, h.keeper.GetParams(h.ctx))
		require.Equal(t, hostWant, payStoreSnapshot(h.ctx, h.hostKey))
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		require.Len(t, moduleBefore, 2)
		require.Equal(t, []byte{1}, moduleBefore[serviceIndexVersionKey])
		require.Contains(t, moduleBefore, string(types.ParamsKey))
		for _, method := range []string{"escrowOpen", "budgetCreate", "streamOpen", "serviceOfferPublish"} {
			require.Contains(t, types.MethodActivity, method)
			require.NoError(t, h.keeper.CheckWrite(h.ctx, method))
		}
		exported := h.keeper.ExportGenesis(h.ctx)
		require.Equal(t, &types.GenesisState{Params: gs.Params, KernelRecords: recordsWant}, exported)
		require.NoError(t, exported.Validate())
		require.Equal(t, hostWant, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		for i := 1; i < len(exported.KernelRecords); i++ {
			require.Negative(t, bytes.Compare(exported.KernelRecords[i-1].Key, exported.KernelRecords[i].Key))
		}

		restored := newPayHarness(t)
		require.NoError(t, restored.keeper.InitGenesis(restored.ctx, *exported))
		require.Equal(t, exported, restored.keeper.ExportGenesis(restored.ctx))
		restored.registerActor(t, 3)
		for _, record := range recordsWant {
			require.Equal(t, record.Value, restored.ctx.KVStore(restored.hostKey).Get(record.Key))
		}
		exported.KernelRecords[0].Key[0] ^= 0xff
		exported.KernelRecords[0].Value[0] ^= 0xff
		require.Equal(t, recordsWant, h.keeper.ExportGenesis(h.ctx).KernelRecords)
	})
}

func TestPayGenesisRefusesGateDrift(t *testing.T) {
	h := newPayHarness(t)
	h.keeper.SetParams(h.ctx, types.Params{Paused: true, EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true})
	for _, family := range []struct {
		name   string
		id     uint16
		params types.Params
	}{
		{"escrow", types.EscrowModuleID, types.Params{EscrowEnabled: true}},
		{"budget", types.BudgetModuleID, types.Params{BudgetEnabled: true}},
		{"stream", types.StreamModuleID, types.Params{StreamEnabled: true}},
		{"service", types.ServiceModuleID, types.Params{ServiceEnabled: true}},
	} {
		key, err := types.KernelModuleEnableKey(family.id)
		require.NoError(t, err)
		h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
		for _, gate := range []struct {
			name  string
			value []byte
		}{
			{"absent", nil}, {"disabled", make([]byte, 32)}, {"malformed", []byte{1}},
		} {
			t.Run(family.name+"/"+gate.name, func(t *testing.T) {
				gs := types.GenesisState{Params: family.params, KernelRecords: []kv.Pair{
					{Key: types.KernelKVKey(types.EscrowModuleID, []byte("must-not-write")), Value: []byte{1}},
				}}
				if gate.value != nil {
					gs.KernelRecords = append(gs.KernelRecords, kv.Pair{Key: key, Value: gate.value})
				}
				moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
				hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
				require.ErrorIs(t, h.keeper.InitGenesis(h.ctx, gs), types.ErrInvalidGenesis)
				require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
				require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
			})
		}
	}

	t.Run("per_record_bounds", func(t *testing.T) {
		gs := types.GenesisState{Params: types.Params{}, KernelRecords: make([]kv.Pair, types.MaxKernelRecords+1)}
		for i := range gs.KernelRecords {
			gs.KernelRecords[i] = kv.Pair{Key: types.KernelKVKey(types.EscrowModuleID, []byte(fmt.Sprintf("record-%03d", i))), Value: []byte{1}}
		}
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		gs.KernelRecords[len(gs.KernelRecords)-1].Value = make([]byte, types.MaxKernelValueBytes+1)
		err := h.keeper.InitGenesis(h.ctx, gs)
		require.ErrorIs(t, err, types.ErrInvalidGenesis)
		require.Contains(t, err.Error(), "kernel record 512 value exceeds 1024 bytes")
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		gs.KernelRecords[len(gs.KernelRecords)-1].Value = []byte{1}
		require.NoError(t, h.keeper.InitGenesis(h.ctx, gs))
		require.Equal(t, gs.Params, h.keeper.GetParams(h.ctx))
		for _, record := range gs.KernelRecords {
			require.Equal(t, record.Value, h.ctx.KVStore(h.hostKey).Get(record.Key))
			require.Nil(t, h.ctx.KVStore(h.storeKey).Get(record.Key))
		}
	})
}
