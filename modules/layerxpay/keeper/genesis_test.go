package keeper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/stretchr/testify/require"
)

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
		require.Len(t, payStoreSnapshot(h.ctx, h.storeKey), 1)
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
				record := kv.Pair{Key: types.KernelKVKey(moduleID, key), Value: []byte{byte(moduleID), key[0], key[1]}}
				require.Equal(t, append([]byte{0x01, 0x00, byte(moduleID)}, key...), record.Key)
				gs.KernelRecords = append(gs.KernelRecords, record)
			}
		}
		gs.KernelRecords = append(gs.KernelRecords,
			kv.Pair{Key: types.KernelKVKey(types.StreamModuleID, bytes.Repeat([]byte{0xa5}, types.MaxKernelKeyBytes)), Value: bytes.Repeat([]byte{0x5a}, types.MaxKernelValueBytes)},
			kv.Pair{Key: types.KernelKVKey(types.BudgetModuleID, []byte("empty")), Value: []byte{}},
			kv.Pair{Key: types.KernelKVKey(types.ServiceModuleID, []byte("nil"))},
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
		require.Len(t, moduleBefore, 1)
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

	t.Run("record_limit", func(t *testing.T) {
		gs := types.GenesisState{Params: types.Params{}, KernelRecords: make([]kv.Pair, types.MaxKernelRecords+1)}
		for i := range gs.KernelRecords {
			gs.KernelRecords[i] = kv.Pair{Key: types.KernelKVKey(types.EscrowModuleID, []byte(fmt.Sprintf("record-%03d", i))), Value: []byte{1}}
		}
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		err := h.keeper.InitGenesis(h.ctx, gs)
		require.ErrorIs(t, err, types.ErrInvalidGenesis)
		require.Contains(t, err.Error(), "kernel record count 513 exceeds 512")
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		gs.KernelRecords = gs.KernelRecords[:types.MaxKernelRecords]
		require.NoError(t, h.keeper.InitGenesis(h.ctx, gs))
		require.Equal(t, gs.Params, h.keeper.GetParams(h.ctx))
		for _, record := range gs.KernelRecords {
			require.Equal(t, record.Value, h.ctx.KVStore(h.hostKey).Get(record.Key))
			require.Nil(t, h.ctx.KVStore(h.storeKey).Get(record.Key))
		}
	})
}
