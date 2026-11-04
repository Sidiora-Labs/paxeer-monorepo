package types_test

import (
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

type repairM606Store struct {
	store  storetypes.CommitMultiStore
	ctx    sdk.Context
	cdc    *codec.ProtoCodec
	keeper *keeper.Keeper
	scopes map[string]keeper.ScopedKeeper
	closed bool
}

func repairM606Open(t *testing.T, dir string) *repairM606Store {
	t.Helper()
	db, err := dbm.NewGoLevelDB("capability", dir)
	require.NoError(t, err)
	ms := store.NewCommitMultiStore(db)
	h := &repairM606Store{store: ms}
	t.Cleanup(func() { h.close(t) })
	key := storetypes.NewKVStoreKey(types.StoreKey)
	memKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	ms.MountStoreWithDB(key, storetypes.StoreTypeDB, nil)
	ms.MountStoreWithDB(memKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, ms.LoadLatestVersion())
	h.ctx = sdk.NewContext(ms, tmproto.Header{Height: ms.LastCommitID().Version + 1}, false)
	h.cdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	h.keeper = keeper.NewKeeper(h.cdc, key, memKey)
	h.scopes = make(map[string]keeper.ScopedKeeper)
	for _, module := range []string{"bank", "ibc", "transfer"} {
		h.scopes[module] = h.keeper.ScopeToModule(module)
	}
	h.keeper.Seal()
	return h
}

func (h *repairM606Store) close(t *testing.T) {
	t.Helper()
	if !h.closed {
		require.NoError(t, h.store.Close())
		h.closed = true
	}
}

func repairM606Export(t *testing.T) (*types.GenesisState, *codec.ProtoCodec) {
	t.Helper()
	h := repairM606Open(t, t.TempDir())
	capability.InitGenesis(h.ctx, *h.keeper, *types.DefaultGenesis())
	bank, ibc, transfer := h.scopes["bank"], h.scopes["ibc"], h.scopes["transfer"]
	first, err := bank.NewCapability(h.ctx, "ports/transfer")
	require.NoError(t, err)
	// Claim in reverse order; the production owner set must export in key order.
	require.NoError(t, transfer.ClaimCapability(h.ctx, first, "channels/0"))
	require.NoError(t, ibc.ClaimCapability(h.ctx, first, "ports/transfer"))
	second, err := bank.NewCapability(h.ctx, "ports/second")
	require.NoError(t, err)
	require.NoError(t, ibc.ClaimCapability(h.ctx, second, "ports/second"))
	genesis := capability.ExportGenesis(h.ctx, *h.keeper)
	require.NoError(t, genesis.Validate())
	require.Equal(t, uint64(3), genesis.Index)
	require.Len(t, genesis.Owners, 2)
	h.close(t)
	return genesis, h.cdc
}

func TestRepairM_606_RejectConflictingGenesis(t *testing.T) {
	genesis, cdc := repairM606Export(t)
	validJSON := cdc.MustMarshalJSON(genesis)
	cases := []struct {
		name    string
		change  func(*types.GenesisState)
		message string
	}{
		{"duplicate index", func(gs *types.GenesisState) {
			gs.Owners = append(gs.Owners, gs.Owners[0])
		}, "duplicate capability index"},
		{"conflicting index", func(gs *types.GenesisState) {
			gs.Owners[1].Index = gs.Owners[0].Index
		}, "duplicate capability index"},
		{"duplicate binding within index", func(gs *types.GenesisState) {
			owners := gs.Owners[0].IndexOwners.Owners
			gs.Owners[0].IndexOwners.Owners = append(owners[:1], append([]types.Owner{owners[0]}, owners[1:]...)...)
		}, "duplicate module/name binding"},
		{"duplicate binding across indices", func(gs *types.GenesisState) {
			gs.Owners[1].IndexOwners.Owners[0] = gs.Owners[0].IndexOwners.Owners[0]
		}, "duplicate module/name binding"},
		{"unsorted owners", func(gs *types.GenesisState) {
			owners := gs.Owners[0].IndexOwners.Owners
			owners[0], owners[2] = owners[2], owners[0]
		}, "must be strictly ordered by owner key"},
		{"zero index", func(gs *types.GenesisState) { gs.Index = 0 }, "index must be non-zero"},
		{"owner index at next index", func(gs *types.GenesisState) {
			gs.Owners[0].Index = gs.Index
		}, "outside of valid range"},
		{"empty owners", func(gs *types.GenesisState) {
			gs.Owners[0].IndexOwners.Owners = nil
		}, "empty owners"},
		{"blank module", func(gs *types.GenesisState) {
			gs.Owners[0].IndexOwners.Owners[0].Module = " \t"
		}, "module cannot be blank"},
		{"blank name", func(gs *types.GenesisState) {
			gs.Owners[0].IndexOwners.Owners[0].Name = " \t"
		}, "name cannot be blank"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var invalid types.GenesisState
			cdc.MustUnmarshalJSON(validJSON, &invalid)
			tc.change(&invalid)
			raw := cdc.MustMarshalJSON(&invalid)
			require.ErrorContains(t, invalid.Validate(), tc.message)
			basic := capability.NewAppModuleBasic(cdc)
			require.ErrorContains(t, basic.ValidateGenesis(cdc, nil, raw), tc.message)
			require.Equal(t, raw, cdc.MustMarshalJSON(&invalid), "validation must not mutate genesis")
		})
	}
}

func TestRepairM_606_ImportRestartAuthenticateRelease(t *testing.T) {
	genesis, cdc := repairM606Export(t)
	raw := cdc.MustMarshalJSON(genesis)
	// This is the production JSON validation and import path, using fresh storage.
	basic := capability.NewAppModuleBasic(cdc)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, raw))
	dir := t.TempDir()
	h := repairM606Open(t, dir)
	module := capability.NewAppModule(h.cdc, *h.keeper)
	module.InitGenesis(h.ctx, h.cdc, raw)
	require.True(t, h.keeper.IsInitialized(h.ctx))
	require.Equal(t, genesis, capability.ExportGenesis(h.ctx, *h.keeper))
	bank := h.scopes["bank"]
	old, ok := bank.GetCapability(h.ctx, "ports/transfer")
	require.True(t, ok)
	require.True(t, bank.AuthenticateCapability(h.ctx, old, "ports/transfer"))
	require.Equal(t, int64(1), h.store.Commit(true).Version)
	h.close(t)

	h = repairM606Open(t, dir)
	require.Equal(t, int64(1), h.store.LastCommitID().Version)
	require.False(t, h.keeper.IsInitialized(h.ctx))
	bank, ibc, transfer := h.scopes["bank"], h.scopes["ibc"], h.scopes["transfer"]
	missing, found := bank.GetCapability(h.ctx, "ports/transfer")
	require.False(t, found, "memory mappings must be gone after reopening")
	require.Nil(t, missing)
	require.Equal(t, genesis, capability.ExportGenesis(h.ctx, *h.keeper))
	h.keeper.InitMemStore(h.ctx)
	require.True(t, h.keeper.IsInitialized(h.ctx))
	recovered, ok := bank.GetCapability(h.ctx, "ports/transfer")
	require.True(t, ok)
	require.Equal(t, genesis.Owners[0].Index, recovered.Index)
	require.NotSame(t, old, recovered)
	require.False(t, bank.AuthenticateCapability(h.ctx, old, "ports/transfer"))
	require.False(t, bank.AuthenticateCapability(h.ctx, types.NewCapability(recovered.Index), "ports/transfer"))
	for _, entry := range genesis.Owners {
		for _, owner := range entry.IndexOwners.Owners {
			scope := h.scopes[owner.Module]
			cap, exists := scope.GetCapability(h.ctx, owner.Name)
			require.True(t, exists)
			require.Equal(t, entry.Index, cap.Index)
			require.True(t, scope.AuthenticateCapability(h.ctx, cap, owner.Name))
			require.False(t, scope.AuthenticateCapability(h.ctx, cap, "wrong-name"))
		}
	}
	shared, ok := ibc.GetCapability(h.ctx, "ports/transfer")
	require.True(t, ok)
	require.Same(t, recovered, shared)
	h.keeper.InitMemStore(h.ctx)
	still, ok := bank.GetCapability(h.ctx, "ports/transfer")
	require.True(t, ok)
	require.Same(t, recovered, still, "repeated initialization preserves identity")

	// Remove the middle owner first: this exercises binary-search removal.
	require.NoError(t, ibc.ReleaseCapability(h.ctx, recovered))
	require.False(t, ibc.AuthenticateCapability(h.ctx, recovered, "ports/transfer"))
	_, found = ibc.GetCapability(h.ctx, "ports/transfer")
	require.False(t, found)
	owners, ok := h.keeper.GetOwners(h.ctx, recovered.Index)
	require.True(t, ok)
	require.Equal(t, []types.Owner{genesis.Owners[0].IndexOwners.Owners[0], genesis.Owners[0].IndexOwners.Owners[2]}, owners.Owners)
	require.True(t, bank.AuthenticateCapability(h.ctx, recovered, "ports/transfer"))
	require.True(t, transfer.AuthenticateCapability(h.ctx, recovered, "channels/0"))
	require.NoError(t, bank.ReleaseCapability(h.ctx, recovered))
	require.NoError(t, transfer.ReleaseCapability(h.ctx, recovered))
	_, found = h.keeper.GetOwners(h.ctx, recovered.Index)
	require.False(t, found, "final release must delete the persistent record")
	for _, owner := range genesis.Owners[0].IndexOwners.Owners {
		scope := h.scopes[owner.Module]
		require.False(t, scope.AuthenticateCapability(h.ctx, recovered, owner.Name))
		_, found = scope.GetCapability(h.ctx, owner.Name)
		require.False(t, found)
	}
	require.Equal(t, int64(2), h.store.Commit(true).Version)
	h.close(t)

	// A second restart must not resurrect released bindings or damage other indices.
	h = repairM606Open(t, dir)
	require.False(t, h.keeper.IsInitialized(h.ctx))
	h.keeper.InitMemStore(h.ctx)
	for _, owner := range genesis.Owners[0].IndexOwners.Owners {
		_, found := h.scopes[owner.Module].GetCapability(h.ctx, owner.Name)
		require.False(t, found)
	}
	for _, owner := range genesis.Owners[1].IndexOwners.Owners {
		scope := h.scopes[owner.Module]
		cap, found := scope.GetCapability(h.ctx, owner.Name)
		require.True(t, found)
		require.Equal(t, genesis.Owners[1].Index, cap.Index)
		require.True(t, scope.AuthenticateCapability(h.ctx, cap, owner.Name))
	}
	require.Equal(t, genesis.Index, h.keeper.GetLatestIndex(h.ctx))
	bank = h.scopes["bank"]
	next, err := bank.NewCapability(h.ctx, "ports/new")
	require.NoError(t, err)
	require.Equal(t, genesis.Index, next.Index)
	require.NoError(t, capability.ExportGenesis(h.ctx, *h.keeper).Validate())
}

func TestRepairM_606_OwnerKeyOrdering(t *testing.T) {
	owners := types.NewCapabilityOwners()
	for _, owner := range []types.Owner{{Module: "a", Name: "port"}, {Module: "a-", Name: "port"}} {
		require.NoError(t, owners.Set(owner))
	}
	genesis := types.GenesisState{Index: 2, Owners: []types.GenesisOwners{{Index: 1, IndexOwners: *owners}}}
	require.Equal(t, "a-", owners.Owners[0].Module)
	require.NoError(t, genesis.Validate(), "ordering must use the same composite key as CapabilityOwners.Get")
	owners.Owners[0], owners.Owners[1] = owners.Owners[1], owners.Owners[0]
	require.ErrorContains(t, genesis.Validate(), "must be strictly ordered by owner key")
}
