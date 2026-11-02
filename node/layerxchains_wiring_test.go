package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	layerxchainsmodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains"
	layerxchainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	layerxpaytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	storekeys "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestChainRegistryModuleIsWired(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	name := layerxchainstypes.ModuleName
	require.NotNil(t, a.LayerXChainsKeeper)
	require.Equal(t, storekeys.LayerXChainsStoreKey, layerxchainstypes.StoreKey)
	require.Contains(t, ModuleBasics, name)
	require.Contains(t, kvStoreKeyNames, storekeys.LayerXChainsStoreKey)
	require.NotNil(t, a.GetKey(storekeys.LayerXChainsStoreKey))
	require.NotEmpty(t, w.Ctx.KVStore(a.GetKey(storekeys.LayerXChainsStoreKey)).Get(layerxchainstypes.ParamsKey))
	require.IsType(t, layerxchainsmodule.AppModule{}, a.mm.Modules[name])
	for _, order := range [][]string{a.mm.OrderInitGenesis, a.mm.OrderExportGenesis} {
		payAt, chainsAt := -1, -1
		for i, moduleName := range order {
			switch moduleName {
			case layerxpaytypes.ModuleName:
				payAt = i
			case name:
				chainsAt = i
			}
		}
		require.GreaterOrEqual(t, payAt, 0)
		require.Equal(t, payAt+1, chainsAt)
	}
	require.Equal(t, storekeys.MemIAVLStoreKeys, kvStoreKeyNames)
	require.Contains(t, storekeys.MemIAVLStoreKeys, storekeys.LayerXChainsStoreKey)
	require.True(t, storekeys.IsMemIAVLStoreKey(storekeys.LayerXChainsStoreKey))
	require.NotNil(t, a.AccountKeeper.GetModuleAccount(w.Ctx, name))
	upgrades, ok := layerxStoreUpgrades(MergeUpgrade)
	require.True(t, ok)
	require.Equal(t, mergeStoreUpgrades(), upgrades)
	require.Contains(t, upgrades.Added, storekeys.LayerXChainsStoreKey)

	versions := a.mm.GetVersionMap()
	delete(versions, name)
	migrated, err := a.mm.RunMigrations(w.Ctx, a.configurator, versions)
	require.NoError(t, err)
	require.Equal(t, a.mm.Modules[name].ConsensusVersion(), migrated[name])
	require.Equal(t, layerxchainstypes.DefaultGenesis(), a.LayerXChainsKeeper.ExportGenesis(w.Ctx))
}

func TestChainRegistryGenesisRoundTripsThroughTheApp(t *testing.T) {
	at := time.Unix(1_700_000_000, 0).UTC()
	const chainID = "pax-test"
	a := Setup(t, true, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	genesis, err := json.Marshal(NewDefaultGenesisState(a.AppCodec()))
	require.NoError(t, err)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: at, ChainId: chainID, InitialHeight: 1,
		ConsensusParams: DefaultConsensusParams, AppStateBytes: genesis,
	})
	require.NoError(t, err)
	ctx := a.NewContext(false, tmproto.Header{ChainID: chainID, Height: 1, Time: at})
	operator := common.HexToAddress("0x1234567890123456789012345678901234567890")
	params := a.LayerXChainsKeeper.GetParams(ctx)
	value := params.MinBond.Add(params.CreationFee)
	denom := a.EvmKeeper.GetBaseDenom(ctx)
	coins := sdk.NewCoins(sdk.NewCoin(denom, value))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, sdk.AccAddress(operator.Bytes()), coins))
	baseFee := a.EvmKeeper.GetMinimumFeePerGas(ctx)
	domain, err := a.LayerXChainsKeeper.Create(ctx, operator, "app-chain", 500_000, 65_536, 100, value, baseFee)
	require.NoError(t, err)
	require.Equal(t, params.MinBond, domain.Bond)
	require.Equal(t, params.MinBond, a.BankKeeper.GetBalance(ctx, authtypes.NewModuleAddress(layerxchainstypes.ModuleName), denom).Amount)
	require.Equal(t, params.CreationFee, a.BankKeeper.GetBalance(ctx, authtypes.NewModuleAddress(authtypes.FeeCollectorName), denom).Amount)
	require.NoError(t, a.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), a.LastBlockHeight())

	exported, err := a.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	var exportedGenesis GenesisState
	require.NoError(t, json.Unmarshal(exported.AppState, &exportedGenesis))
	require.Contains(t, exportedGenesis, layerxchainstypes.ModuleName)
	var registry layerxchainstypes.GenesisState
	require.NoError(t, json.Unmarshal(exportedGenesis[layerxchainstypes.ModuleName], &registry))
	require.NoError(t, registry.Validate())
	require.Equal(t, []layerxchainstypes.Domain{domain}, registry.Domains)
	require.Equal(t, params, registry.Params)
	require.Equal(t, domain.Id+1, registry.NextID)
	require.Equal(t, baseFee, registry.NextBaseFees[domain.Id])

	restored := Setup(t, true, true, false)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	_, err = restored.InitChain(context.Background(), &abci.RequestInitChain{
		Time: at, ChainId: chainID, InitialHeight: exported.Height,
		ConsensusParams: exported.ConsensusParams, AppStateBytes: exported.AppState,
	})
	require.NoError(t, err)
	importCtx := restored.NewContext(false, tmproto.Header{ChainID: chainID, Height: exported.Height, Time: at})
	got, found := restored.LayerXChainsKeeper.GetDomain(importCtx, domain.Id)
	require.True(t, found)
	require.Equal(t, domain, got)
	byName, found := restored.LayerXChainsKeeper.GetDomainByName(importCtx, domain.Name)
	require.True(t, found)
	require.Equal(t, domain, byName)
	require.Equal(t, uint32(1), restored.LayerXChainsKeeper.DomainCount(importCtx))
	require.Equal(t, registry.Params, restored.LayerXChainsKeeper.GetParams(importCtx))
	require.Equal(t, registry.NextID, restored.LayerXChainsKeeper.NextID(importCtx))
	require.Equal(t, baseFee, restored.LayerXChainsKeeper.NextBaseFee(importCtx, domain.Id))
	require.Equal(t, params.MinBond, restored.BankKeeper.GetBalance(importCtx, authtypes.NewModuleAddress(layerxchainstypes.ModuleName), denom).Amount)
	require.Equal(t, params.CreationFee, restored.BankKeeper.GetBalance(importCtx, authtypes.NewModuleAddress(authtypes.FeeCollectorName), denom).Amount)
}
