package capability_test

import (
	"os"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM609Store struct {
	store         storetypes.CommitMultiStore
	ctx           sdk.Context
	cdc           *codec.ProtoCodec
	keeper        *keeper.Keeper
	ibc, transfer keeper.ScopedKeeper
	module        capability.AppModule
	closed        bool
}

func repairM609Open(t *testing.T, dir string) *repairM609Store {
	t.Helper()
	db, err := dbm.NewGoLevelDB("capability", dir)
	require.NoError(t, err)
	h := &repairM609Store{store: store.NewCommitMultiStore(db)}
	t.Cleanup(func() { h.close(t) })
	key := storetypes.NewKVStoreKey(types.StoreKey)
	memKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	h.store.MountStoreWithDB(key, storetypes.StoreTypeDB, nil)
	h.store.MountStoreWithDB(memKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, h.store.LoadLatestVersion())
	h.ctx = sdk.NewContext(h.store, tmproto.Header{Height: h.store.LastCommitID().Version + 1}, false)
	h.cdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	h.keeper = keeper.NewKeeper(h.cdc, key, memKey)
	h.ibc = h.keeper.ScopeToModule("ibc")
	h.transfer = h.keeper.ScopeToModule("transfer")
	h.keeper.Seal()
	h.module = capability.NewAppModule(h.cdc, *h.keeper)
	return h
}

func (h *repairM609Store) close(t *testing.T) {
	t.Helper()
	if !h.closed {
		require.NoError(t, h.store.Close())
		h.closed = true
	}
}

func TestRepairM_609_GenesisAndRestartLifecycle(t *testing.T) {
	source := repairM609Open(t, t.TempDir())
	require.False(t, source.keeper.IsInitialized(source.ctx), "construction has no context to initialize memory")
	defaults := source.module.DefaultGenesis(source.cdc)
	require.NoError(t, source.module.ValidateGenesis(source.cdc, nil, defaults))
	require.Empty(t, source.module.InitGenesis(source.ctx, source.cdc, defaults))
	require.True(t, source.keeper.IsInitialized(source.ctx))
	require.Equal(t, uint64(1), source.keeper.GetLatestIndex(source.ctx))
	require.Empty(t, capability.ExportGenesis(source.ctx, *source.keeper).Owners)

	shared, err := source.ibc.NewCapability(source.ctx, "ports/transfer")
	require.NoError(t, err)
	require.NoError(t, source.transfer.ClaimCapability(source.ctx, shared, "channels/0"))
	released, err := source.ibc.NewCapability(source.ctx, "ports/released")
	require.NoError(t, err)
	require.NoError(t, source.ibc.ReleaseCapability(source.ctx, released))

	exported := source.module.ExportGenesis(source.ctx, source.cdc)
	var genesis types.GenesisState
	require.NoError(t, source.cdc.UnmarshalAsJSON(exported, &genesis))
	require.NoError(t, genesis.Validate())
	require.Equal(t, uint64(3), genesis.Index)
	require.Equal(t, []types.GenesisOwners{{Index: shared.Index, IndexOwners: types.CapabilityOwners{Owners: []types.Owner{
		{Module: "ibc", Name: "ports/transfer"},
		{Module: "transfer", Name: "channels/0"},
	}}}}, genesis.Owners)

	dir := t.TempDir()
	imported := repairM609Open(t, dir)
	require.NoError(t, imported.module.ValidateGenesis(imported.cdc, nil, exported))
	require.Empty(t, imported.module.InitGenesis(imported.ctx, imported.cdc, exported))
	require.True(t, imported.keeper.IsInitialized(imported.ctx))
	canonical, found := imported.ibc.GetCapability(imported.ctx, "ports/transfer")
	require.True(t, found)
	require.NotSame(t, shared, canonical, "genesis transports identity, not a capability pointer")
	require.True(t, imported.ibc.AuthenticateCapability(imported.ctx, canonical, "ports/transfer"))
	require.False(t, imported.ibc.AuthenticateCapability(imported.ctx, shared, "ports/transfer"))
	claimed, found := imported.transfer.GetCapability(imported.ctx, "channels/0")
	require.True(t, found)
	require.Same(t, canonical, claimed, "owners share one canonical pointer per running keeper")
	require.True(t, imported.transfer.AuthenticateCapability(imported.ctx, claimed, "channels/0"))
	require.JSONEq(t, string(exported), string(imported.module.ExportGenesis(imported.ctx, imported.cdc)))
	commit := imported.store.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	imported.close(t)

	restarted := repairM609Open(t, dir)
	require.Equal(t, commit, restarted.store.LastCommitID())
	require.False(t, restarted.keeper.IsInitialized(restarted.ctx))
	require.Equal(t, genesis.Index, restarted.keeper.GetLatestIndex(restarted.ctx))
	owners, found := restarted.keeper.GetOwners(restarted.ctx, shared.Index)
	require.True(t, found)
	require.Equal(t, genesis.Owners[0].IndexOwners, owners)
	_, found = restarted.ibc.GetCapability(restarted.ctx, "ports/transfer")
	require.False(t, found, "persisted owners alone do not populate memory lookups")

	capability.BeginBlocker(restarted.ctx, *restarted.keeper)
	require.True(t, restarted.keeper.IsInitialized(restarted.ctx))
	recovered, found := restarted.ibc.GetCapability(restarted.ctx, "ports/transfer")
	require.True(t, found)
	require.Equal(t, shared.Index, recovered.Index)
	require.NotSame(t, canonical, recovered)
	require.True(t, restarted.ibc.AuthenticateCapability(restarted.ctx, recovered, "ports/transfer"))
	require.False(t, restarted.ibc.AuthenticateCapability(restarted.ctx, canonical, "ports/transfer"))
	require.False(t, restarted.ibc.AuthenticateCapability(restarted.ctx, types.NewCapability(recovered.Index), "ports/transfer"))
	claimed, found = restarted.transfer.GetCapability(restarted.ctx, "channels/0")
	require.True(t, found)
	require.Same(t, recovered, claimed)
	require.True(t, restarted.transfer.AuthenticateCapability(restarted.ctx, claimed, "channels/0"))
	_, found = restarted.ibc.GetCapability(restarted.ctx, "ports/released")
	require.False(t, found, "export/import and restart must preserve released-index gaps")
	require.JSONEq(t, string(exported), string(restarted.module.ExportGenesis(restarted.ctx, restarted.cdc)))

	capability.BeginBlocker(restarted.ctx, *restarted.keeper)
	again, found := restarted.ibc.GetCapability(restarted.ctx, "ports/transfer")
	require.True(t, found)
	require.Same(t, recovered, again, "repeated initialization must preserve live references")
	next, err := restarted.transfer.NewCapability(restarted.ctx, "channels/next")
	require.NoError(t, err)
	require.Equal(t, genesis.Index, next.Index, "recovery preserves the next allocation index")
}

func TestRepairM_609_OverviewLifecycleContract(t *testing.T) {
	text, err := os.ReadFile("spec/README.md")
	require.NoError(t, err)
	for _, contract := range []string{
		"capabilitykeeper.NewKeeper(",
		"sdk/x/capability/keeper",
		"`ScopeToModule`",
		"`InitGenesis`",
		"`InitMemStore`",
		"`BeginBlocker`",
		"`ExportGenesis`",
		"`GenesisState{Index, Owners}`",
		"`Owner{Module, Name}`",
		"process-local",
		"`ScopedKeeper.GetCapability`",
	} {
		require.Contains(t, string(text), contract)
	}
	require.NotContains(t, string(text), "capability.NewKeeper(")
	require.NotContains(t, string(text), "no genesis state")
	require.NotContains(t, string(text), "does not establish\nsorted, unique owner mappings")
}
