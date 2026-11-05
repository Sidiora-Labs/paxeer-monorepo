//go:build cgo

package app_test

import (
	"encoding/json"
	"reflect"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/stretchr/testify/require"
)

func TestAppOrdersStoreGenesisBeforeHostWriters(t *testing.T) {
	w := mergedAppWrapper(t)
	names := []string{keys.LayerXHostStoreKey, ledgertypes.ModuleName, authtypes.ModuleName, paytypes.ModuleName, agenttypes.ModuleName}
	order := mergedModuleOrder(t, w.App, names...)
	hostAt := -1
	for i, name := range order {
		if name == keys.LayerXHostStoreKey {
			hostAt = i
		}
	}
	require.GreaterOrEqual(t, hostAt, 0)
	require.GreaterOrEqual(t, len(order), hostAt+len(names))
	require.Equal(t, names, order[hostAt:hostAt+len(names)])
	manager := reflect.ValueOf(w.App).Elem().FieldByName("mm").Elem()
	exportOrder := manager.FieldByName("OrderExportGenesis")
	hostExportAt, ledgerExportAt := -1, -1
	for i := 0; i < exportOrder.Len(); i++ {
		switch exportOrder.Index(i).String() {
		case keys.LayerXHostStoreKey:
			hostExportAt = i
		case ledgertypes.ModuleName:
			ledgerExportAt = i
		}
	}
	require.GreaterOrEqual(t, hostExportAt, 0)
	require.Equal(t, hostExportAt+1, ledgerExportAt)

	imported := lxphost.StoreGenesis{LastSequence: 9, Records: []kv.Pair{{Key: []byte{1, 0, 1, 'a'}, Value: []byte{1, 2}}}}
	pay := paytypes.DefaultGenesis()
	pay.Params.Paused = false
	pay.KernelRecords = []kv.Pair{{Key: paytypes.KernelKVKey(paytypes.BudgetModuleID, []byte("genesis-probe")), Value: []byte{4, 5}}}
	hostJSON, err := json.Marshal(imported)
	require.NoError(t, err)
	payJSON, err := json.Marshal(pay)
	require.NoError(t, err)
	state, err := json.Marshal(map[string]json.RawMessage{keys.LayerXHostStoreKey: hostJSON, paytypes.ModuleName: payJSON})
	require.NoError(t, err)
	w.App.InitChainer(w.Ctx, abci.RequestInitChain{AppStateBytes: state})
	key := w.App.GetKey(keys.LayerXHostStoreKey)
	next, err := lxphost.NextSequence(w.Ctx, key)
	require.NoError(t, err)
	require.Equal(t, uint64(10), next)
	exported, err := lxphost.ExportStoreGenesis(w.Ctx, key)
	require.NoError(t, err)
	require.Equal(t, &imported, exported)
	require.Equal(t, pay, w.App.LayerXPayKeeper.ExportGenesis(w.Ctx))
	require.Equal(t, imported.Records[0].Value, w.Ctx.KVStore(key).Get(imported.Records[0].Key))
	require.Equal(t, pay.KernelRecords[0].Value, w.Ctx.KVStore(key).Get(pay.KernelRecords[0].Key))
}
