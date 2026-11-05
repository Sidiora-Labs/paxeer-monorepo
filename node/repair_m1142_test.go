package app

import (
	"fmt"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	oracleprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	anchorprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

// Audit 22.30.50 repair record (22.33.1142):
// activation-test-requires-retired-anchor-module: TestRepairM_1142_RetainedAnchorLifecycle
// activation-test-rejects-live-retirement-dependency: TestRepairM_1142_ActivationDependencies
// merge-gate-test-expects-missing-production-constructors: TestRepairM_1142_RegisteredMergeConstructors
// merge-name-test-assumes-last-plan: TestRepairM_1142_MergePlanOrder
// merge-stores-test-omits-layerxchains: TestRepairM_1142_MergeStoresIncludeChains
// sequencer-upgrade-test-obsolete-store-contract: TestRepairM_1142_CompleteSequencerUpgrade
// fee-upgrade-test-stale-tag-tail: TestRepairM_1142_HistoricalFeeTokenMigration
// v610-test-stale-current-upgrade-metadata: TestRepairM_1142_CurrentV610Metadata
// The prior attempt failed ActivationDependencies on contract-instance equality
// (gotcha 3009). Qualification revision and private evidence are in hf-done's ledger.
// An earlier verify failed because ordinary execution serves perps at
// 0x1015 while tracing serves the historical exchange after the activation plan.
// No out-of-path wiring changes are required by these eight repairs.

func repairM1142Addresses(contracts map[common.Address]vm.PrecompiledContract) []string {
	addresses := make([]string, 0, len(contracts))
	for address := range contracts {
		addresses = append(addresses, address.Hex())
	}
	sort.Strings(addresses)
	return addresses
}

func requireRepairM1142ABIValues(t *testing.T, want, got []interface{}) {
	t.Helper()
	require.Len(t, got, len(want))
	for i, expected := range want {
		if number, ok := expected.(*big.Int); ok {
			actual, ok := got[i].(*big.Int)
			require.True(t, ok, "ABI result %d must be *big.Int", i)
			require.NotNil(t, actual)
			require.Zero(t, number.Cmp(actual), "ABI result %d: want %s, got %s", i, number, actual)
		} else {
			require.Equal(t, expected, got[i], "ABI result %d", i)
		}
	}
}

func requireRepairM1142RestoredCalls(t *testing.T, a *App, ctx sdk.Context, below map[common.Address]vm.PrecompiledContract, oracleActive bool) {
	t.Helper()
	owner := common.Address{1}
	cases := map[common.Address]struct {
		method string
		args   []interface{}
		want   []interface{}
	}{
		common.HexToAddress("0x1013"): {"depositCount", nil, []interface{}{a.LayerXCustodyKeeper.GetDepositCount(ctx)}},
		common.HexToAddress("0x1015"): {"intentNonce", []interface{}{owner}, []interface{}{a.LayerXExchangeKeeper.GetOwnerNonce(ctx, owner)}},
		common.HexToAddress("0x1016"): {"isPaused", nil, []interface{}{a.LayerXBridgeKeeper.IsPaused(ctx)}},
		common.HexToAddress("0x1017"): {"getPoolCount", nil, []interface{}{big.NewInt(0)}},
		common.HexToAddress("0x1018"): {"getFeeDenom", []interface{}{owner}, []interface{}{a.EvmKeeper.GetAccountFeeDenom(ctx, owner)}},
		common.HexToAddress("0x1019"): {"threshold", nil, []interface{}{a.XWebKeeper.Threshold(ctx)}},
	}
	// Full genesis enables the v6.12.0 contracts for ordinary execution. With
	// only v6.9 recorded as done, tracing selects the historical versions at
	// the exchange and launchpad addresses. Exercise both selected ABIs.
	if ctx.IsTracing() {
		launchpad := cases[common.HexToAddress("0x1017")]
		launchpad.method = "getMarketCount"
		cases[common.HexToAddress("0x1017")] = launchpad
	} else {
		exchange := cases[common.HexToAddress("0x1015")]
		exchange.method = "totalMarkets"
		exchange.args = nil
		exchange.want = []interface{}{big.NewInt(0)}
		cases[common.HexToAddress("0x1015")] = exchange
	}
	served := a.EvmKeeper.CustomPrecompiles(ctx)
	for address, contract := range served {
		if _, preserved := below[address]; preserved {
			continue
		}
		require.NotNil(t, contract, address.Hex())
		typed, ok := contract.(interface {
			Address() common.Address
			GetABI() abi.ABI
		})
		require.True(t, ok, address.Hex())
		require.Equal(t, address, typed.Address())
		if address == common.HexToAddress(oracleprecompile.OracleAddress) {
			requireRepairM1142OracleCall(t, a, ctx, oracleActive)
			continue
		}
		tc, ok := cases[address]
		require.True(t, ok, "restored address needs a production call: %s", address.Hex())
		input, err := typed.GetABI().Pack(tc.method, tc.args...)
		require.NoError(t, err)
		evm := &vm.EVM{StateDB: state.NewDBImpl(ctx, &a.EvmKeeper, true)}
		output, err := contract.Run(evm, owner, owner, input, nil, true, false, nil)
		require.NoError(t, err, "%s %s", address.Hex(), tc.method)
		values, err := typed.GetABI().Unpack(tc.method, output)
		require.NoError(t, err)
		requireRepairM1142ABIValues(t, tc.want, values)
	}
}

func requireRepairM1142OracleCall(t *testing.T, a *App, ctx sdk.Context, active bool) {
	t.Helper()
	address := common.HexToAddress(oracleprecompile.OracleAddress)
	contract := a.EvmKeeper.CustomPrecompiles(ctx)[address]
	require.NotNil(t, contract)
	typed, ok := contract.(*oracleprecompile.Precompile)
	require.True(t, ok)
	require.Equal(t, address, typed.Address())
	feeds := []oracleprecompile.Feed{{Price: big.NewInt(17), Confidence: big.NewInt(3), Timestamp: big.NewInt(ctx.BlockTime().Unix())}}
	input, err := typed.GetABI().Pack(oracleprecompile.AggregateMethod, feeds)
	require.NoError(t, err)
	evm := &vm.EVM{StateDB: state.NewDBImpl(ctx, &a.EvmKeeper, true)}
	output, err := contract.Run(evm, common.Address{1}, common.Address{1}, input, nil, true, false, nil)
	if !active {
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, decodeErr := abi.UnpackRevert(output)
		require.NoError(t, decodeErr)
		require.Equal(t, fmt.Sprintf("LXP_AGENT_NOT_ACTIVE: %s not active at height %d", MergeUpgrade, ctx.BlockHeight()), reason)
		return
	}
	require.NoError(t, err)
	values, err := typed.GetABI().Unpack(oracleprecompile.AggregateMethod, output)
	require.NoError(t, err)
	requireRepairM1142ABIValues(t, []interface{}{big.NewInt(17), big.NewInt(3)}, values)
}

func TestRepairM_1142_RetainedAnchorLifecycle(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx.WithBlockHeight(mergeHeight-1)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.Equal(t, []string{"layerxcustody", "layerxanchor", "layerxexchange", "layerxbridge", "launchpad", "xweb"}, activationStoreUpgrades().Added)
	for _, name := range activationModules() {
		require.Contains(t, kvStoreKeyNames, name)
		require.Contains(t, keys.MemIAVLStoreKeys, name)
		require.NotNil(t, a.GetKey(name))
		require.NotNil(t, a.CommitMultiStore().GetCommitKVStore(a.GetKey(name)))
		requireActivationModuleLifecycle(t, a, name)
	}

	// Historical state is produced by the retained keeper and bank, independent
	// of the retired module lifecycle.
	a.LayerXAnchorKeeper.InitGenesis(ctx, *anchortypes.DefaultGenesis())
	operator := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	params := a.LayerXAnchorKeeper.GetParams(ctx)
	bond := params.MinBond.AddRaw(9)
	funding := sdk.NewCoins(sdk.NewCoin(params.BondDenom, bond.MulRaw(2)))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", funding))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", operator, funding))
	_, err := a.LayerXAnchorKeeper.RegisterGuarantor(ctx, operator, [32]byte{1}, [20]byte{1}, bond)
	require.NoError(t, err)
	_, err = a.LayerXAnchorKeeper.BeginUnbond(ctx, operator, [32]byte{1}, sdk.OneInt())
	require.NoError(t, err)
	historical := a.LayerXAnchorKeeper.ExportGenesis(ctx)
	rewindBeforeActivation(t, a, ctx)
	a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: ActivationUpgrade, Height: ctx.BlockHeight()})
	require.Equal(t, historical, a.LayerXAnchorKeeper.ExportGenesis(ctx))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), anchortypes.ModuleName)

	ctx = ctx.WithBlockHeight(mergeHeight).WithEventManager(sdk.NewEventManager())
	supply := a.BankKeeper.GetSupply(ctx, params.BondDenom)
	a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)})
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	require.Equal(t, funding[0], a.BankKeeper.GetBalance(ctx, operator, params.BondDenom))
	require.Equal(t, supply, a.BankKeeper.GetSupply(ctx, params.BondDenom))
	require.True(t, a.LayerXAnchorKeeper.ModuleBalance(ctx).IsZero())
	guarantors := a.LayerXAnchorKeeper.GetGuarantors(ctx)
	require.Len(t, guarantors, 1)
	require.True(t, guarantors[0].Bond.IsZero())
	require.Empty(t, a.LayerXAnchorKeeper.GetUnbondings(ctx))
	require.Contains(t, ctx.EventManager().Events(), sdk.NewEvent(anchortypes.EventAnchorRetired,
		sdk.NewAttribute(anchortypes.AttributeReturned, bond.String())))
	requireAnchorRetirementRevert(t, a, ctx, anchorprecompile.FinalizeMethod, uint64(1))
	require.NotNil(t, a.CommitMultiStore().GetCommitKVStore(a.GetKey(anchortypes.StoreKey)))
	require.False(t, a.upgradePlanPending(ctx, MergeUpgrade))
	require.True(t, a.upgradePlanPending(ctx, AnchorStoreDeleteUpgrade))

	// Commit the real migration, then load the deletion plan's store upgrade
	// through the same loader a node uses at the next startup height.
	cms := a.CommitMultiStore()
	require.NoError(t, cms.SetInitialVersion(mergeHeight))
	ctx.MultiStore().(sdk.CacheMultiStore).Write()
	commit := cms.Commit(true)
	require.Equal(t, mergeHeight, commit.Version)
	deletion, ok := layerxStoreUpgrades(AnchorStoreDeleteUpgrade)
	require.True(t, ok)
	require.Empty(t, deletion.Added)
	require.Empty(t, deletion.Renamed)
	require.Equal(t, []string{anchortypes.StoreKey}, deletion.Deleted)
	require.NoError(t, upgradetypes.UpgradeStoreLoader(mergeHeight+1, &deletion)(cms))
	require.Nil(t, cms.GetCommitKVStore(a.GetKey(anchortypes.StoreKey)))
	ctx = ctx.WithMultiStore(cms.CacheMultiStore()).WithBlockHeight(mergeHeight + 1)
	plan := upgradetypes.Plan{Name: AnchorStoreDeleteUpgrade, Height: ctx.BlockHeight()}
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	a.UpgradeKeeper.ApplyUpgrade(ctx, plan)
	require.Equal(t, mergeHeight+1, a.UpgradeKeeper.GetDoneHeight(ctx, AnchorStoreDeleteUpgrade))
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	require.Equal(t, funding[0], a.BankKeeper.GetBalance(ctx, operator, params.BondDenom))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), anchortypes.ModuleName)
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{mergeHeight + 1, mergeHeight + 2} {
			requireAnchorRetirementRevert(t, a, ctx.WithBlockHeight(height).WithIsTracing(tracing), anchorprecompile.LatestFinalizedMethod)
		}
	}
}

func TestRepairM_1142_ActivationDependencies(t *testing.T) {
	TestActivationServesTheForkPrecompilesOnlyFromItsHeight(t)
}

func TestRepairM_1142_RegisteredMergeConstructors(t *testing.T) {
	TestMergePlanRefusesMissingMergeConstructors(t)
}

func TestRepairM_1142_MergePlanOrder(t *testing.T) {
	TestMergeUpgradeNameAgreesAcrossPackages(t)
	TestAnchorStoreDeletedAtStep(t)
}

func TestRepairM_1142_MergeStoresIncludeChains(t *testing.T) {
	TestMergeStoreUpgradesAddsLayerXStores(t)
}

func TestRepairM_1142_CompleteSequencerUpgrade(t *testing.T) {
	TestUpgradesSequencerInitialisesAllMergeStores(t)
	TestUpgradesSequencerGenesisAtActivation(t)
	TestMergePlanRequiresKeyForFirstSequencedHeight(t)
	TestMergePlanRefusesKeyCoveringOnlyActivationHeight(t)
}

func TestRepairM_1142_HistoricalFeeTokenMigration(t *testing.T) {
	TestSidioraFeeTokenUpgradeIsRegisteredAndRunsTheFeeTokenMigration(t)
}

func TestRepairM_1142_CurrentV610Metadata(t *testing.T) {
	TestV610IsRegisteredBesideThePlansThatPrecedeIt(t)
	TestV610AttestorsAreTheSuppliedSetInAscendingOrder(t)
	TestV610WritesTheOperatingValuesInTheBlockTheUpgradeInfoNames(t)
}
