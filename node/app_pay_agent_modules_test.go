package app_test

import (
	"encoding/json"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/stretchr/testify/require"
)

func TestAppBuildsPayKeeperOverSharedHost(t *testing.T) {
	w := mergedAppWrapper(t)
	require.NotNil(t, w.App.LayerXPayKeeper)
	require.NotNil(t, w.App.LayerXHost)
	require.Same(t, w.App.LayerXHost, w.App.LayerXPayKeeper.Host())
	require.Same(t, w.App.LayerXAgentKeeper.Host(), w.App.LayerXPayKeeper.Host())
	require.Equal(t, govtypes.GovernanceAuthority(), w.App.LayerXPayKeeper.Authority())
	params := paytypes.DefaultParams()
	params.Paused = false
	w.App.LayerXPayKeeper.SetParams(w.Ctx, params)
	require.Equal(t, params, w.App.LayerXPayKeeper.GetParams(w.Ctx))
	encoded := w.Ctx.KVStore(w.App.GetKey(keys.LayerXPayStoreKey)).Get(paytypes.ParamsKey)
	require.NotEmpty(t, encoded)
	var stored paytypes.Params
	require.NoError(t, json.Unmarshal(encoded, &stored))
	require.Equal(t, params, stored)
	record := kv.Pair{Key: paytypes.KernelKVKey(paytypes.EscrowModuleID, []byte("shared-host-probe")), Value: []byte{1, 2, 3}}
	require.NoError(t, w.App.LayerXPayKeeper.InitGenesis(w.Ctx, paytypes.GenesisState{Params: params, KernelRecords: []kv.Pair{record}}))
	require.Equal(t, record.Value, w.Ctx.KVStore(w.App.GetKey(app.LayerXHostStoreKey)).Get(record.Key))
	require.Contains(t, w.App.LayerXPayKeeper.ExportGenesis(w.Ctx).KernelRecords, record)
}

func TestAppRegistersPayAndAgentModulesInGenesisOrder(t *testing.T) {
	w := mergedAppWrapper(t)
	order := mergedModuleOrder(t, w.App, authtypes.ModuleName, paytypes.ModuleName, agenttypes.ModuleName)
	authAt := -1
	for i, name := range order {
		if name == authtypes.ModuleName {
			authAt = i
		}
	}
	require.GreaterOrEqual(t, authAt, 0)
	require.GreaterOrEqual(t, len(order), authAt+3)
	require.Equal(t, []string{paytypes.ModuleName, agenttypes.ModuleName}, order[authAt+1:authAt+3])
	pay := paytypes.DefaultGenesis()
	pay.Params.Paused = false
	pay.KernelRecords = []kv.Pair{{Key: paytypes.KernelKVKey(paytypes.BudgetModuleID, []byte("genesis-probe")), Value: []byte{4, 5}}}
	agent := agenttypes.DefaultGenesis()
	agent.Params.MaxJobsPerBlock = 13
	agent.NextJobId = 42
	payJSON, err := json.Marshal(pay)
	require.NoError(t, err)
	state, err := json.Marshal(map[string]json.RawMessage{
		paytypes.ModuleName:   payJSON,
		agenttypes.ModuleName: w.App.AppCodec().MustMarshalJSON(agent),
	})
	require.NoError(t, err)
	w.App.InitChainer(w.Ctx, abci.RequestInitChain{AppStateBytes: state})
	require.Equal(t, pay, w.App.LayerXPayKeeper.ExportGenesis(w.Ctx))
	require.Equal(t, agent.Params, w.App.LayerXAgentKeeper.GetParams(w.Ctx))
	require.Equal(t, agent.NextJobId, w.App.LayerXAgentKeeper.NextJobID(w.Ctx))
}
