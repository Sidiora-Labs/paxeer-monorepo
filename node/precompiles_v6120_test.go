package app

import (
	"testing"
	"time"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	launchpadv610keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/legacy/v610/keeper"
	launchpadtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	layerxexchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	spottypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	tokenfactorykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	launchpadprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	launchpadv610 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad/legacy/v610"
	layerxexchangeprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxexchange"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

const (
	v6120TestActivationHeight int64 = 10
	v6120TestDoneHeight       int64 = 1000
)

var (
	v6120TestExchange  = common.HexToAddress(layerxexchangetypes.ExchangeAddress)
	v6120TestLaunchpad = common.HexToAddress(launchpadtypes.LaunchpadAddress)
	v6120TestGated     = []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000001020"),
		common.HexToAddress("0x0000000000000000000000000000000000001021"),
		common.HexToAddress("0x0000000000000000000000000000000000001022"),
		common.HexToAddress("0x0000000000000000000000000000000000001023"),
		common.HexToAddress("0x0000000000000000000000000000000000001024"),
		common.HexToAddress("0x0000000000000000000000000000000000001025"),
	}
	v6120TestCodeless = []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000001026"),
		common.HexToAddress("0x0000000000000000000000000000000000001027"),
	}
)

// v6120TestContracts holds the contracts one test serves: the v6.9 exchange as
// the legacy 0x1015, the v6.10 launchpad as the legacy 0x1017, and stand-ins
// built from the same constructors for every v6.12.0 contract.
type v6120TestContracts struct {
	perps, legacyExchange, g2, g1 vm.PrecompiledContract
	gated                         map[common.Address]vm.PrecompiledContract
	next                          map[common.Address][2]vm.PrecompiledContract
}

func newV6120TestContracts(t *testing.T, a *App) v6120TestContracts {
	t.Helper()
	keepers := a.GetPrecompileKeepers()
	exchange := func() vm.PrecompiledContract {
		contract, err := layerxexchangeprecompile.NewPrecompile(keepers)
		require.NoError(t, err)
		return contract
	}
	g2, err := launchpadprecompile.NewPrecompile(keepers)
	require.NoError(t, err)
	// The application keepers carry no v6.10 launchpad keeper, so the legacy
	// contract is built over the v6.10 keeper on the launchpad store.
	legacyLaunchpad := launchpadv610keeper.NewKeeper(a.GetKey(launchpadtypes.StoreKey), a.BankKeeper,
		tokenfactorykeeper.NewMsgServerImpl(a.TokenFactoryKeeper), &a.EvmKeeper)
	g1, err := launchpadv610.NewPrecompileWithKeeper(legacyLaunchpad, keepers.EVMK())
	require.NoError(t, err)
	c := v6120TestContracts{
		perps:          exchange(),
		legacyExchange: exchange(),
		g2:             g2,
		g1:             g1,
		gated:          make(map[common.Address]vm.PrecompiledContract, len(v6120TestGated)),
	}
	require.NotSame(t, c.perps, c.legacyExchange)
	require.NotSame(t, c.g2, c.g1)
	c.next = map[common.Address][2]vm.PrecompiledContract{
		v6120TestExchange:  {c.perps, c.legacyExchange},
		v6120TestLaunchpad: {c.g2, c.g1},
	}
	for _, addr := range v6120TestGated {
		c.gated[addr] = exchange()
		c.next[addr] = [2]vm.PrecompiledContract{c.gated[addr]}
	}
	return c
}

// recordV6120TestHeights records the activation plan as done at height 10 and
// the v6.12.0 plan at height 1000.
func recordV6120TestHeights(a *App, ctx sdk.Context) {
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(v6120TestActivationHeight), ActivationUpgrade)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(v6120TestDoneHeight), V6120Upgrade)
}

func TestFullGenesisSelectsTheV6120Set(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.Equal(t, a.mm.GetVersionMap(), versions)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, V6120Upgrade))
	require.True(t, a.v6120LiveInState(ctx))
	require.True(t, a.v6120Live)
	for _, addr := range v6120TestGated {
		require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), addr)
	}
	require.Empty(t, a.EvmKeeper.ActivationAddressesWithoutPrecompile())

	a.setV6120Live(false)
	a.RefreshPrecompiles(ctx)
	require.True(t, a.v6120Live)
	for _, tracing := range []bool{false, true} {
		served := a.EvmKeeper.CustomPrecompiles(ctx.WithIsTracing(tracing))
		for _, addr := range v6120TestGated {
			require.Contains(t, served, addr)
			contract, ok := served[addr].(precompiles.IPrecompile)
			require.True(t, ok)
			require.Equal(t, addr, contract.Address())
		}
	}
	require.Empty(t, a.EvmKeeper.ActivationAddressesWithoutPrecompile())
	plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)}
	t.Run("wrong execution height", func(t *testing.T) {
		cached, _ := ctx.CacheContext()
		_, err := a.mergeUpgradePlan(cached.WithBlockHeight(mergeHeight-1), plan, versions)
		require.ErrorContains(t, err, "is not running at height")
		_, found := a.UpgradeKeeper.GetUpgradePlan(cached)
		require.False(t, found)
	})
	t.Run("conflicting scheduled plan", func(t *testing.T) {
		cached, _ := ctx.CacheContext()
		other := plan
		other.Info = "different genesis"
		require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(cached, other))
		_, err := a.mergeUpgradePlan(cached, plan, versions)
		require.ErrorContains(t, err, "executing plan differs from scheduled plan")
		scheduled, found := a.UpgradeKeeper.GetUpgradePlan(cached)
		require.True(t, found)
		require.Equal(t, other, scheduled)
	})
	_, err := a.mergeUpgradePlan(ctx, plan, versions)
	require.NoError(t, err)
	require.True(t, a.Sequenced(ctx, mergeHeight+1))
	_, found := a.UpgradeKeeper.GetUpgradePlan(ctx)
	require.False(t, found)

	for _, name := range []string{pricefeedtypes.ModuleName, spottypes.ModuleName} {
		t.Run("missing "+name, func(t *testing.T) {
			before, _ := ctx.CacheContext()
			before.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
			require.False(t, a.v6120LiveInState(before))
			a.UpgradeKeeper.SetDone(before.WithBlockHeight(v6120TestDoneHeight), V6120Upgrade)
			require.True(t, a.v6120LiveInState(before))
			below := a.EvmKeeper.CustomPrecompiles(before.WithBlockHeight(v6120TestDoneHeight - 1))
			at := a.EvmKeeper.CustomPrecompiles(before.WithBlockHeight(v6120TestDoneHeight))
			for _, addr := range v6120TestGated {
				require.NotContains(t, below, addr)
				require.Contains(t, at, addr)
			}
		})
	}
}

func TestV6120PrecompileGate(t *testing.T) {
	gate := v6120PrecompileGate()
	require.Equal(t, V6120Upgrade, gate.Upgrade)
	require.Equal(t, map[common.Address][]string{
		v6120TestGated[0]: {pricefeedtypes.ModuleName},
		v6120TestGated[1]: {spottypes.ModuleName},
		v6120TestGated[2]: {spottypes.ModuleName},
		v6120TestGated[3]: {spottypes.ModuleName},
		v6120TestGated[4]: {spottypes.ModuleName},
		v6120TestGated[5]: {spottypes.ModuleName},
	}, gate.Modules)
	require.Equal(t, "pricefeed", pricefeedtypes.ModuleName)
	require.Equal(t, "spot", spottypes.ModuleName)
	for _, addr := range append([]common.Address{v6120TestExchange, v6120TestLaunchpad}, v6120TestCodeless...) {
		require.NotContains(t, gate.Modules, addr)
	}
	require.Contains(t, activationPrecompileGate().Modules, v6120TestExchange)
	require.Contains(t, activationPrecompileGate().Modules, v6120TestLaunchpad)
	for _, addr := range v6120TestCodeless {
		require.NotContains(t, activationPrecompileGate().Modules, addr)
	}

	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := testWrapper.App, testWrapper.Ctx

	// A v6.12.0 entry that also names 0x1015 is refused beside the activation entry.
	conflicting := v6120PrecompileGate()
	conflicting.Modules[v6120TestExchange] = []string{layerxexchangetypes.ModuleName}
	require.PanicsWithValue(t,
		"custom precompile activations "+ActivationUpgrade+" and "+V6120Upgrade+" both name "+v6120TestExchange.Hex(),
		func() {
			a.EvmKeeper.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{activationPrecompileGate(), conflicting})
		})
	require.NotPanics(t, func() {
		a.EvmKeeper.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{activationPrecompileGate(), v6120PrecompileGate()})
	})

	contracts := newV6120TestContracts(t, a)
	all := precompiles.GetCustomPrecompiles(LatestUpgrade, a.GetPrecompileKeepers())
	a.EvmKeeper.SetCustomPrecompiles(v6120PrecompileSet(all, contracts.next, true, true), LatestUpgrade)
	recordV6120TestHeights(a, ctx)

	below := a.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(v6120TestDoneHeight - 1))
	for _, addr := range v6120TestGated {
		require.NotContains(t, below, addr)
	}
	require.Contains(t, below, v6120TestExchange)
	require.Contains(t, below, v6120TestLaunchpad)
	at := a.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(v6120TestDoneHeight))
	for _, addr := range v6120TestGated {
		require.Contains(t, at, addr)
		require.Same(t, contracts.gated[addr], at[addr])
	}
	for _, addr := range v6120TestCodeless {
		require.NotContains(t, below, addr)
		require.NotContains(t, at, addr)
	}

	// Below the done height 0x1021 behaves as an account with no code.
	caller := a.AccountKeeper.GetModuleAddress(evmtypes.ModuleName)
	pecor := common.HexToAddress(spottypes.PECORAddress)
	output, err := a.EvmKeeper.StaticCallEVM(ctx.WithBlockHeight(v6120TestDoneHeight-1), caller, &pecor, []byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
	require.Empty(t, output)
}

func TestV6120ExchangeVersionSwitch(t *testing.T) {
	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := testWrapper.App, testWrapper.Ctx
	require.Equal(t, "v6.9", ActivationUpgrade)
	require.Equal(t, "v6.12.0", V6120Upgrade)
	require.NotEqual(t, ActivationUpgrade, LatestUpgrade)
	require.NotEqual(t, V6120Upgrade, LatestUpgrade)

	contracts := newV6120TestContracts(t, a)
	switches := []struct {
		name      string
		versioned func(live bool) putils.VersionedPrecompiles
		next      vm.PrecompiledContract
		legacy    vm.PrecompiledContract
	}{
		{
			name: "exchange",
			versioned: func(live bool) putils.VersionedPrecompiles {
				return exchangeVersionSwitch(contracts.perps, contracts.legacyExchange, live)
			},
			next:   contracts.perps,
			legacy: contracts.legacyExchange,
		},
		{
			name: "launchpad",
			versioned: func(live bool) putils.VersionedPrecompiles {
				return launchpadVersionSwitch(contracts.g2, contracts.g1, live)
			},
			next:   contracts.g2,
			legacy: contracts.g1,
		},
	}
	for _, sw := range switches {
		off := sw.versioned(false)
		require.Len(t, off, 2, sw.name)
		require.Same(t, sw.legacy, off[LatestUpgrade], sw.name)
		require.Same(t, sw.legacy, off[ActivationUpgrade], sw.name)
		require.NotContains(t, off, V6120Upgrade, sw.name)

		on := sw.versioned(true)
		require.Len(t, on, 3, sw.name)
		require.Same(t, sw.next, on[LatestUpgrade], sw.name)
		require.Same(t, sw.legacy, on[ActivationUpgrade], sw.name)
		require.Same(t, sw.next, on[V6120Upgrade], sw.name)
	}

	// Before the switch the set keeps the legacy contracts and adds nothing.
	all := precompiles.GetCustomPrecompiles(LatestUpgrade, a.GetPrecompileKeepers())
	xweb := common.HexToAddress(xwebtypes.PrecompileAddress)
	require.Contains(t, all, xweb)
	dormant := v6120PrecompileSet(all, contracts.next, false, false)
	require.Len(t, dormant, len(all)-1)
	require.NotContains(t, dormant, xweb)
	require.Same(t, contracts.legacyExchange, dormant[v6120TestExchange][LatestUpgrade])
	require.Same(t, contracts.g1, dormant[v6120TestLaunchpad][LatestUpgrade])
	for _, addr := range append(v6120TestGated, v6120TestCodeless...) {
		require.NotContains(t, dormant, addr)
	}
	live := v6120PrecompileSet(all, contracts.next, true, true)
	require.Len(t, live, len(all)+len(v6120TestGated))
	require.Contains(t, live, xweb)
	for _, addr := range v6120TestGated {
		require.Len(t, live[addr], 2)
		require.Same(t, contracts.gated[addr], live[addr][LatestUpgrade])
		require.Same(t, contracts.gated[addr], live[addr][V6120Upgrade])
	}
	for _, addr := range v6120TestCodeless {
		require.NotContains(t, live, addr)
	}

	// The handler's switch serves the new contracts under the latest upgrade.
	a.EvmKeeper.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{activationPrecompileGate(), v6120PrecompileGate()})
	a.customPrecompiles = all
	a.v6120Next = contracts.next
	ctx, _ = ctx.CacheContext()
	for _, name := range []string{pricefeedtypes.ModuleName, spottypes.ModuleName} {
		ctx.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
	}
	require.False(t, a.v6120LiveInState(ctx))
	a.setV6120Live(true)
	require.True(t, a.v6120Live)
	recordV6120TestHeights(a, ctx)
	require.True(t, a.v6120LiveInState(ctx))

	belowCtx := ctx.WithBlockHeight(v6120TestDoneHeight - 1)
	atCtx := ctx.WithBlockHeight(v6120TestDoneHeight)
	require.Same(t, contracts.perps, a.EvmKeeper.CustomPrecompiles(atCtx)[v6120TestExchange])
	require.Same(t, contracts.g2, a.EvmKeeper.CustomPrecompiles(atCtx)[v6120TestLaunchpad])

	// Tracing picks the version each address carried at the height.
	below := a.EvmKeeper.GetCustomPrecompilesVersions(belowCtx)
	require.Equal(t, ActivationUpgrade, below[v6120TestExchange])
	require.Equal(t, ActivationUpgrade, below[v6120TestLaunchpad])
	at := a.EvmKeeper.GetCustomPrecompilesVersions(atCtx)
	require.Equal(t, V6120Upgrade, at[v6120TestExchange])
	require.Equal(t, V6120Upgrade, at[v6120TestLaunchpad])
	tracedBelow := a.EvmKeeper.CustomPrecompiles(belowCtx.WithIsTracing(true))
	require.Same(t, contracts.legacyExchange, tracedBelow[v6120TestExchange])
	require.Same(t, contracts.g1, tracedBelow[v6120TestLaunchpad])
	tracedAt := a.EvmKeeper.CustomPrecompiles(atCtx.WithIsTracing(true))
	require.Same(t, contracts.perps, tracedAt[v6120TestExchange])
	require.Same(t, contracts.g2, tracedAt[v6120TestLaunchpad])

	// Switching the set off serves the legacy contracts again.
	a.setV6120Live(false)
	require.False(t, a.v6120Live)
	require.Same(t, contracts.legacyExchange, a.EvmKeeper.CustomPrecompiles(atCtx)[v6120TestExchange])
	require.Same(t, contracts.g1, a.EvmKeeper.CustomPrecompiles(atCtx)[v6120TestLaunchpad])
}
