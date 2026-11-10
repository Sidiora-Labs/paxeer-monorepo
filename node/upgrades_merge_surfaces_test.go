package app

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	launchpadtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

const mergeUnguardedSurfaceRuntime = "0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3"

var mergeSurfaceHashes = struct{ unguarded, guarded common.Hash }{
	common.HexToHash("0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f"),
	common.HexToHash("0xe67f4b121cfa00710e9a74cd027b8d56f7ca67d14ebe357a8c080f8c2e819993"),
}

func newMergeSurfacesChain(t *testing.T) (*App, sdk.Context) {
	t.Helper()
	w := NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	rewindBeforeMerge(t, a, ctx)
	unguarded := common.FromHex(mergeUnguardedSurfaceRuntime)
	require.Equal(t, mergeSurfaceHashes.unguarded, crypto.Keccak256Hash(unguarded))
	require.Equal(t, mergeSurfaceHashes.guarded, crypto.Keccak256Hash(launchpadkeeper.SurfaceRuntime))
	require.Len(t, launchpadkeeper.Generation2Surfaces, 9)
	for role, address := range launchpadkeeper.Generation2Surfaces {
		require.NoError(t, a.LaunchpadKeeper.SetSurface(ctx, address, launchpadkeeper.Surface{
			Role: role, Generation: launchpadtypes.Generation2,
		}))
		a.EvmKeeper.SetCode(ctx, address, unguarded)
		a.EvmKeeper.SetNonce(ctx, address, uint64(address[0])+1)
		for _, slot := range []common.Hash{{}, common.HexToHash("0x06"), crypto.Keccak256Hash([]byte(role))} {
			a.EvmKeeper.SetState(ctx, address, slot, crypto.Keccak256Hash(address.Bytes(), slot.Bytes()))
		}
		funding := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), int64(address[0])+1))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm",
			a.EvmKeeper.GetPaxAddressOrDefault(ctx, address), funding))
		require.Equal(t, mergeSurfaceHashes.unguarded, a.EvmKeeper.GetCodeHash(ctx, address), role)
	}
	return a, ctx
}

func TestMergeUpgradeInstallsGuardedRoleSurfaces(t *testing.T) {
	a, ctx := newMergeSurfacesChain(t)
	records := make(map[common.Address][]byte)
	balances := make(map[common.Address]*big.Int)
	storage := make(map[common.Address]map[common.Hash]common.Hash)
	for role, address := range launchpadkeeper.Generation2Surfaces {
		records[address] = bytes.Clone(ctx.KVStore(a.GetKey(launchpadtypes.StoreKey)).Get(launchpadtypes.SurfaceKey(address)))
		balances[address] = a.EvmKeeper.GetBalance(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, address))
		require.Positive(t, balances[address].Sign(), role)
		storage[address] = make(map[common.Hash]common.Hash)
		for _, slot := range []common.Hash{{}, common.HexToHash("0x06"), crypto.Keccak256Hash([]byte(role))} {
			storage[address][slot] = a.EvmKeeper.GetState(ctx, address, slot)
		}
	}
	assertSurfaces := func() {
		t.Helper()
		for role, address := range launchpadkeeper.Generation2Surfaces {
			require.Equal(t, launchpadkeeper.SurfaceRuntime, a.EvmKeeper.GetCode(ctx, address), role)
			require.Equal(t, mergeSurfaceHashes.guarded, a.EvmKeeper.GetCodeHash(ctx, address), role)
			require.Equal(t, len(launchpadkeeper.SurfaceRuntime), a.EvmKeeper.GetCodeSize(ctx, address), role)
			surface, found, err := a.LaunchpadKeeper.GetSurface(ctx, address)
			require.NoError(t, err, role)
			require.True(t, found, role)
			require.Equal(t, launchpadkeeper.Surface{Role: role, Generation: launchpadtypes.Generation2}, surface)
			require.Equal(t, records[address], ctx.KVStore(a.GetKey(launchpadtypes.StoreKey)).Get(launchpadtypes.SurfaceKey(address)), role)
			require.Equal(t, balances[address], a.EvmKeeper.GetBalance(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, address)), role)
			require.Equal(t, uint64(address[0])+1, a.EvmKeeper.GetNonce(ctx, address), role)
			for slot, value := range storage[address] {
				require.Equal(t, value, a.EvmKeeper.GetState(ctx, address, slot), "%s slot %s", role, slot)
			}
		}
	}
	plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)}
	versions, err := a.mergeUpgradePlan(ctx, plan, a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.NoError(t, err)
	require.Equal(t, a.mm.GetVersionMap(), versions)
	assertSurfaces()
	installed := snapshotMergeStores(t, a, ctx)
	replayed, err := a.mergeUpgradePlan(ctx, plan, versions)
	require.NoError(t, err)
	require.Equal(t, versions, replayed)
	require.Equal(t, installed, snapshotMergeStores(t, a, ctx))
	assertSurfaces()

	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	require.NotPanics(t, func() { a.UpgradeKeeper.ApplyUpgrade(ctx, plan) })
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	assertSurfaces()
	completed := snapshotMergeStores(t, a, ctx)
	_, err = a.mergeUpgradePlan(ctx, plan, a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.ErrorContains(t, err, "already been completed")
	require.Equal(t, completed, snapshotMergeStores(t, a, ctx))
}

func TestMergeUpgradeRollsBackFailedRoleSurfaces(t *testing.T) {
	a, ctx := newMergeSurfacesChain(t)
	address := launchpadkeeper.Generation2Surfaces[launchpadtypes.RoleTreasury]
	ctx.KVStore(a.GetKey(launchpadtypes.StoreKey)).Set(launchpadtypes.SurfaceKey(address), []byte("{"))
	before := snapshotMergeStores(t, a, ctx)
	plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)}
	_, err := a.mergeUpgradePlan(ctx, plan, a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.ErrorContains(t, err, "launchpad role surfaces")
	require.ErrorContains(t, err, "corrupt surface "+address.Hex())
	require.Equal(t, before, snapshotMergeStores(t, a, ctx))
	for role, address := range launchpadkeeper.Generation2Surfaces {
		require.Equal(t, common.FromHex(mergeUnguardedSurfaceRuntime), a.EvmKeeper.GetCode(ctx, address), role)
		require.Equal(t, mergeSurfaceHashes.unguarded, a.EvmKeeper.GetCodeHash(ctx, address), role)
	}
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
}
