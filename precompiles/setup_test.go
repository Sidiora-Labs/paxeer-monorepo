package precompiles_test

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/feetoken"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/clearing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/eip712"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/orob"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/pofq"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxchains"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	precompiletest "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func isExchangeHandoffAddress(address common.Address, name, previous string) bool {
	names := []string{"layerxExchange", "layerxexchange", "perps"}
	return address == common.HexToAddress("0x0000000000000000000000000000000000001015") &&
		slices.Contains(names, name) && slices.Contains(names, previous)
}

func TestFeeTokenRegistration(t *testing.T) {
	initialized := precompiles.Initialized
	infoBefore := precompiles.PrecompileNamesToInfo
	precompiles.Initialized = false
	precompiles.PrecompileNamesToInfo = maps.Clone(infoBefore)
	t.Cleanup(func() {
		precompiles.Initialized = initialized
		precompiles.PrecompileNamesToInfo = infoBefore
	})
	for _, contracts := range []*vm.PrecompiledContracts{
		&vm.PrecompiledContractsHomestead, &vm.PrecompiledContractsByzantium,
		&vm.PrecompiledContractsIstanbul, &vm.PrecompiledContractsBerlin,
		&vm.PrecompiledContractsCancun, &vm.PrecompiledContractsBLS,
	} {
		before := *contracts
		*contracts = maps.Clone(before)
		t.Cleanup(func() { *contracts = before })
	}
	for _, addresses := range []*[]common.Address{
		&vm.PrecompiledAddressesHomestead, &vm.PrecompiledAddressesByzantium,
		&vm.PrecompiledAddressesIstanbul, &vm.PrecompiledAddressesBerlin,
		&vm.PrecompiledAddressesCancun,
	} {
		before := *addresses
		*addresses = slices.Clone(before)
		t.Cleanup(func() { *addresses = before })
	}
	address := common.HexToAddress("0x0000000000000000000000000000000000001018")
	require.Equal(t, address, common.HexToAddress(feetoken.FeeTokenAddress))
	require.NoError(t, precompiles.InitializePrecompiles(true, precompiletest.Keepers(testkeeper.EVMTestApp)))
	require.False(t, precompiles.Initialized)
	info := precompiles.GetPrecompileInfo(feetoken.PrecompileName)
	require.False(t, precompiles.Initialized)
	require.Equal(t, address, info.Address)
	require.Len(t, info.ABI.Methods, 3)
	seen := make(map[common.Address]string)
	for name, entry := range precompiles.PrecompileNamesToInfo {
		previous, exists := seen[entry.Address]
		require.False(t, exists && !isExchangeHandoffAddress(entry.Address, name, previous), "%s collides with %s at %s", name, previous, entry.Address)
		seen[entry.Address] = name
	}
	require.Equal(t, "v6.7", precompiles.FeeTokenUpgrade)
	below := precompiles.GetCustomPrecompiles("v6.6", precompiletest.Keepers(testkeeper.EVMTestApp))
	_, belowFound := below[address]
	require.False(t, belowFound)
	versioned := precompiles.GetCustomPrecompiles(precompiles.FeeTokenUpgrade, precompiletest.Keepers(testkeeper.EVMTestApp))
	versions, found := versioned[address]
	require.True(t, found)
	require.Len(t, versions, 1)
	p, ok := versions[precompiles.FeeTokenUpgrade].(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, feetoken.PrecompileName, p.GetName())
	require.Equal(t, address, p.Address())
	for registered, entries := range versioned {
		for _, entry := range entries {
			named, ok := entry.(precompiles.IPrecompile)
			require.True(t, ok)
			require.Equal(t, registered, named.Address())
			if registered != address {
				require.NotEqual(t, feetoken.PrecompileName, named.GetName())
			}
		}
	}
	require.NoError(t, precompiles.InitializePrecompiles(false, precompiletest.Keepers(testkeeper.EVMTestApp)))
	for _, contracts := range []map[common.Address]vm.PrecompiledContract{
		vm.PrecompiledContractsHomestead, vm.PrecompiledContractsByzantium,
		vm.PrecompiledContractsIstanbul, vm.PrecompiledContractsBerlin,
		vm.PrecompiledContractsCancun, vm.PrecompiledContractsBLS,
	} {
		entry, exists := contracts[address]
		require.True(t, exists)
		named, ok := entry.(precompiles.IPrecompile)
		require.True(t, ok)
		require.Equal(t, feetoken.PrecompileName, named.GetName())
	}
}

func TestXWebRegistration(t *testing.T) {
	initialized := precompiles.Initialized
	infoBefore := precompiles.PrecompileNamesToInfo
	precompiles.Initialized = false
	precompiles.PrecompileNamesToInfo = maps.Clone(infoBefore)
	t.Cleanup(func() {
		precompiles.Initialized = initialized
		precompiles.PrecompileNamesToInfo = infoBefore
	})
	for _, contracts := range []*vm.PrecompiledContracts{
		&vm.PrecompiledContractsHomestead, &vm.PrecompiledContractsByzantium,
		&vm.PrecompiledContractsIstanbul, &vm.PrecompiledContractsBerlin,
		&vm.PrecompiledContractsCancun, &vm.PrecompiledContractsBLS,
	} {
		before := *contracts
		*contracts = maps.Clone(before)
		t.Cleanup(func() { *contracts = before })
	}
	for _, addresses := range []*[]common.Address{
		&vm.PrecompiledAddressesHomestead, &vm.PrecompiledAddressesByzantium,
		&vm.PrecompiledAddressesIstanbul, &vm.PrecompiledAddressesBerlin,
		&vm.PrecompiledAddressesCancun,
	} {
		before := *addresses
		*addresses = slices.Clone(before)
		t.Cleanup(func() { *addresses = before })
	}
	address := common.HexToAddress("0x0000000000000000000000000000000000001019")
	require.Equal(t, address, common.HexToAddress(xweb.XWebAddress))
	require.NoError(t, precompiles.InitializePrecompiles(true, precompiletest.Keepers(testkeeper.EVMTestApp)))
	require.False(t, precompiles.Initialized)
	info := precompiles.GetPrecompileInfo(xweb.PrecompileName)
	require.False(t, precompiles.Initialized)
	require.Equal(t, address, info.Address)
	require.Len(t, info.ABI.Methods, 12)
	require.Len(t, info.ABI.Events, 4)
	seen := make(map[common.Address]string)
	for name, entry := range precompiles.PrecompileNamesToInfo {
		previous, exists := seen[entry.Address]
		require.False(t, exists && !isExchangeHandoffAddress(entry.Address, name, previous), "%s collides with %s at %s", name, previous, entry.Address)
		seen[entry.Address] = name
	}
	versioned := precompiles.GetCustomPrecompiles("v6.8", precompiletest.Keepers(testkeeper.EVMTestApp))
	versions, found := versioned[address]
	require.True(t, found)
	require.Len(t, versions, 1)
	p, ok := versions["v6.8"].(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, xweb.PrecompileName, p.GetName())
	require.Equal(t, address, p.Address())
	_, metered := versions["v6.8"].(vm.DynamicGasPrecompiledContract)
	require.True(t, metered)
	for registered, entries := range versioned {
		for _, entry := range entries {
			named, ok := entry.(precompiles.IPrecompile)
			require.True(t, ok)
			require.Equal(t, registered, named.Address())
			if registered != address {
				require.NotEqual(t, xweb.PrecompileName, named.GetName())
			}
		}
	}
	require.NoError(t, precompiles.InitializePrecompiles(false, precompiletest.Keepers(testkeeper.EVMTestApp)))
	for _, contracts := range []map[common.Address]vm.PrecompiledContract{
		vm.PrecompiledContractsHomestead, vm.PrecompiledContractsByzantium,
		vm.PrecompiledContractsIstanbul, vm.PrecompiledContractsBerlin,
		vm.PrecompiledContractsCancun, vm.PrecompiledContractsBLS,
	} {
		entry, exists := contracts[address]
		require.True(t, exists)
		named, ok := entry.(precompiles.IPrecompile)
		require.True(t, ok)
		require.Equal(t, xweb.PrecompileName, named.GetName())
	}
	require.Contains(t, vm.PrecompiledAddressesCancun, address)
}

func MergeAddressTable() []precompiles.MergeAddress {
	// The address and name pairs of contracts/address-table.md section (a).
	return []precompiles.MergeAddress{
		{Address: common.HexToAddress("0x000000000000000000000000000000000000101A"), Name: "layerxledger"},
		{Address: common.HexToAddress("0x000000000000000000000000000000000000101B"), Name: "layerxpay"},
		{Address: common.HexToAddress("0x000000000000000000000000000000000000101C"), Name: "layerxauth"},
		{Address: common.HexToAddress("0x000000000000000000000000000000000000101D"), Name: "layerxprograms"},
		{Address: common.HexToAddress("0x000000000000000000000000000000000000101E"), Name: "layerxchains"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000901"), Name: "orob"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000902"), Name: "clearing"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000903"), Name: "oracle aggregator"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000904"), Name: "pofq"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000905"), Name: "scheduler"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000906"), Name: "streams"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000907"), Name: "tee attestor"},
		{Address: common.HexToAddress("0x0000000000000000000000000000000000000908"), Name: "eip712"},
	}
}

func TestMergeAddressTable(t *testing.T) {
	const latestUpgrade = "v7.1"
	expected := MergeAddressTable()
	require.Len(t, precompiles.MergeAddressTable, 13)
	require.Equal(t, expected, precompiles.MergeAddressTable)
	require.Equal(t, app.MergeUpgrade, precompiles.MergeUpgrade)

	// The wallet gateway routes reach the agent-function addresses.
	source, err := os.ReadFile("../services/wallet/gateway/src/precompiles.ts")
	require.NoError(t, err)
	tables := regexp.MustCompile(`(?s)export const PRECOMPILE_ADDRESSES\s*=\s*(?:Object\.freeze\(\s*)?\{(.*?)\}\s*as const`).FindAllStringSubmatch(string(source), -1)
	require.Len(t, tables, 1, "expected one gateway address table")
	routes := map[string]string{}
	for _, match := range regexp.MustCompile(`(?m)^\s+(\w+): '(0x[0-9a-fA-F]{40})',\s*$`).FindAllStringSubmatch(tables[0][1], -1) {
		routes[match[1]] = match[2]
	}
	for i, key := range []string{"orob", "clearing", "oracle", "pofq", "scheduler", "streams", "teeAttestor", "eip712"} {
		route, found := routes[key]
		require.True(t, found, key)
		require.Equal(t, expected[5+i].Address, common.HexToAddress(route), key)
	}

	// The registry refuses an address outside the table, a nil constructor and
	// a second constructor for one address.
	testApp := app.Setup(t, false, true, false)
	keepers := precompiletest.Keepers(testApp)
	upgradeKeeper := &testApp.UpgradeKeeper
	t.Run("merge is latest", func(t *testing.T) {
		set, err := precompiles.GetCustomPrecompilesE(precompiles.MergeUpgrade, keepers)
		require.NoError(t, err)
		for _, entry := range expected {
			versions, found := set[entry.Address]
			require.True(t, found, entry.Name)
			require.Len(t, versions, 1, entry.Name)
			served, ok := versions[precompiles.MergeUpgrade].(precompiles.IPrecompile)
			require.True(t, ok, entry.Name)
			require.Equal(t, entry.Address, served.Address())
			require.Equal(t, entry.Name, served.GetName())
		}
	})
	outside := common.HexToAddress(xweb.XWebAddress)
	require.EqualError(t, precompiles.RegisterMergeConstructor(outside, func(utils.Keepers, string) (precompiles.IPrecompile, error) {
		return orob.NewPrecompile(upgradeKeeper, precompiles.MergeUpgrade, 64)
	}), "merge constructor for "+outside.Hex()+" outside the merge address table")
	require.EqualError(t, precompiles.RegisterMergeConstructor(expected[5].Address, nil),
		"nil merge constructor for "+expected[5].Address.Hex())

	// Register the agent functions whose packages are in the tree, unless
	// their packages registered them already.
	constructors := map[common.Address]precompiles.MergeConstructor{
		expected[5].Address: func(_ utils.Keepers, plan string) (precompiles.IPrecompile, error) {
			return orob.NewPrecompile(upgradeKeeper, plan, 64)
		},
		expected[6].Address: func(_ utils.Keepers, plan string) (precompiles.IPrecompile, error) {
			return clearing.NewPrecompile(upgradeKeeper, plan, 64)
		},
		expected[8].Address: func(_ utils.Keepers, plan string) (precompiles.IPrecompile, error) {
			return pofq.NewPrecompile(upgradeKeeper, plan, 64)
		},
		expected[12].Address: func(_ utils.Keepers, plan string) (precompiles.IPrecompile, error) {
			return eip712.NewPrecompile(upgradeKeeper, plan, 64, 64)
		},
	}
	registeredBefore := precompiles.GetCustomPrecompiles(latestUpgrade, keepers)
	for addr, constructor := range constructors {
		if _, registered := registeredBefore[addr]; registered {
			continue
		}
		require.NoError(t, precompiles.RegisterMergeConstructor(addr, constructor))
	}
	require.EqualError(t, precompiles.RegisterMergeConstructor(expected[5].Address, constructors[expected[5].Address]),
		"merge constructor for "+expected[5].Address.Hex()+" already registered")

	// Every address with a registered constructor carries its precompile at
	// the latest upgrade and at the merge plan; no other merge address is in
	// the set.
	set := precompiles.GetCustomPrecompiles(latestUpgrade, keepers)
	served := map[common.Address]string{}
	for _, entry := range expected {
		versions, found := set[entry.Address]
		_, constructed := constructors[entry.Address]
		if !found {
			require.False(t, constructed, entry.Name)
			continue
		}
		require.Len(t, versions, 2, entry.Name)
		p, ok := versions[precompiles.MergeUpgrade].(precompiles.IPrecompile)
		require.True(t, ok, entry.Name)
		require.Equal(t, entry.Name, p.GetName())
		require.Equal(t, entry.Address, p.Address())
		require.Same(t, p, versions[latestUpgrade])
		served[entry.Address] = entry.Name
	}
	for addr := range constructors {
		require.Contains(t, served, addr)
	}

	// Served from the merge plan height through the keeper activation entry:
	// below it the set equals the set without the merge entries, at and above
	// it every registered address is served with its precompile.
	unmerged := maps.Clone(set)
	for _, entry := range expected {
		delete(unmerged, entry.Address)
	}
	modules := map[common.Address][]string{
		expected[0].Address: {"layerxledger"},
		expected[1].Address: {"layerxpay"},
		expected[2].Address: {"layerxauth"},
		expected[3].Address: {"layerxprograms"},
		expected[4].Address: {"layerxchains"},
	}
	for _, entry := range expected[5:] {
		modules[entry.Address] = []string{"layerxagent"}
	}
	k := &testApp.EvmKeeper
	k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{{Upgrade: precompiles.MergeUpgrade, Modules: modules}})
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(8).WithBlockTime(time.Now())
	const planHeight = int64(20)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)

	heights := []int64{1, planHeight - 1, planHeight, planHeight + 1}
	baselines := map[bool]map[int64]map[common.Address]vm.PrecompiledContract{}
	k.SetCustomPrecompiles(unmerged, latestUpgrade)
	for _, tracing := range []bool{false, true} {
		baselines[tracing] = map[int64]map[common.Address]vm.PrecompiledContract{}
		for _, height := range heights {
			baselines[tracing][height] = k.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(tracing))
		}
	}
	k.SetCustomPrecompiles(set, latestUpgrade)
	for _, tracing := range []bool{false, true} {
		baseline := baselines[tracing]
		for _, height := range []int64{1, planHeight - 1} {
			below := k.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(tracing))
			require.Len(t, below, len(baseline[height]), height)
			for addr, p := range baseline[height] {
				require.Same(t, p, below[addr], height)
			}
			for _, entry := range expected {
				require.NotContains(t, below, entry.Address, height)
			}
		}
		for _, height := range []int64{planHeight, planHeight + 1} {
			at := k.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(tracing))
			require.Len(t, at, len(baseline[height])+len(served), height)
			for addr, p := range baseline[height] {
				require.Same(t, p, at[addr], height)
			}
			for _, entry := range expected {
				name, registered := served[entry.Address]
				if !registered {
					require.NotContains(t, at, entry.Address, height)
					continue
				}
				named, ok := at[entry.Address].(precompiles.IPrecompile)
				require.True(t, ok, height)
				require.Equal(t, name, named.GetName(), height)
				require.Same(t, set[entry.Address][precompiles.MergeUpgrade], at[entry.Address], height)
			}
		}
	}
}

func TestPrecompileInfoResolvesWithoutMergeKeepers(t *testing.T) {
	initialized, before := precompiles.Initialized, precompiles.PrecompileNamesToInfo
	precompiles.Initialized = false
	precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
	t.Cleanup(func() {
		precompiles.Initialized, precompiles.PrecompileNamesToInfo = initialized, before
	})
	contractsBefore := make(map[*vm.PrecompiledContracts]vm.PrecompiledContracts)
	for _, contracts := range []*vm.PrecompiledContracts{
		&vm.PrecompiledContractsHomestead, &vm.PrecompiledContractsByzantium,
		&vm.PrecompiledContractsIstanbul, &vm.PrecompiledContractsBerlin,
		&vm.PrecompiledContractsCancun, &vm.PrecompiledContractsPrague,
		&vm.PrecompiledContractsBLS, &vm.PrecompiledContractsVerkle,
	} {
		contractsBefore[contracts] = maps.Clone(*contracts)
	}
	addressesBefore := make(map[*[]common.Address][]common.Address)
	for _, addresses := range []*[]common.Address{
		&vm.PrecompiledAddressesHomestead, &vm.PrecompiledAddressesByzantium,
		&vm.PrecompiledAddressesIstanbul, &vm.PrecompiledAddressesBerlin,
		&vm.PrecompiledAddressesCancun, &vm.PrecompiledAddressesPrague,
	} {
		addressesBefore[addresses] = slices.Clone(*addresses)
	}
	payABI, err := layerxpay.GetABI()
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		address string
		method  string
	}{
		{xweb.PrecompileName, xweb.XWebAddress, xweb.FeeMethod},
		{layerxpay.PrecompileName, layerxpay.PayAddress, "getStream"},
		{scheduler.PrecompileName, "0x0000000000000000000000000000000000000905", "schedule"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info, err := precompiles.GetPrecompileInfoE(tc.name)
			require.NoError(t, err)
			require.Equal(t, common.HexToAddress(tc.address), info.Address)
			require.Contains(t, info.ABI.Methods, tc.method)
			if tc.name == layerxpay.PrecompileName {
				require.Equal(t, payABI.Methods, info.ABI.Methods)
				require.Equal(t, payABI.Events, info.ABI.Events)
			}
			if tc.name == scheduler.PrecompileName {
				require.Len(t, info.ABI.Events, 5)
				for _, event := range []string{"JobScheduled", "JobCancelled", "JobRescheduled", "JobExecuted", "JobFailed"} {
					require.Contains(t, info.ABI.Events, event)
				}
			}
			require.False(t, precompiles.Initialized)
			require.Empty(t, precompiles.PrecompileNamesToInfo)
		})
	}
	cold := make(map[common.Address]precompiles.PrecompileInfo)
	for _, entry := range precompiles.MergeAddressTable {
		info, err := precompiles.GetPrecompileInfoE(entry.Name)
		require.NoError(t, err, entry.Name)
		require.Equal(t, entry.Address, info.Address, entry.Name)
		require.NotEmpty(t, info.ABI.Methods, entry.Name)
		cold[entry.Address] = info
	}
	require.False(t, precompiles.Initialized)
	require.Empty(t, precompiles.PrecompileNamesToInfo)
	for contracts, snapshot := range contractsBefore {
		require.Equal(t, snapshot, *contracts)
	}
	for addresses, snapshot := range addressesBefore {
		require.Equal(t, snapshot, *addresses)
	}
	a := app.Setup(t, false, true, false)
	served, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, precompiletest.Keepers(a))
	require.NoError(t, err)
	for _, entry := range precompiles.MergeAddressTable {
		contract, ok := served[entry.Address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
		require.True(t, ok, entry.Name)
		require.Equal(t, contract.GetABI(), cold[entry.Address].ABI, entry.Name)
	}
}

func TestMergeTableRequiresPayAndChainsKeepers(t *testing.T) {
	a := app.Setup(t, false, true, false)
	keepers := precompiletest.Keepers(a)
	var _ layerxpay.LayerXPayKeepers = keepers
	var _ layerxchains.LayerXChainsKeepers = keepers
	require.Same(t, a.LayerXPayKeeper, keepers.LayerXPayK())
	require.Same(t, a.LayerXChainsKeeper, keepers.LayerXChainsK())

	t.Run("missing pay capability", func(t *testing.T) {
		custom, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, struct{ utils.Keepers }{keepers})
		require.Nil(t, custom)
		require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT: LayerXPayKeepers is required")
	})
	t.Run("missing chains keeper", func(t *testing.T) {
		custom, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, app.NewPrecompileKeepers(a))
		require.Nil(t, custom)
		require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT: layerxchains keeper is unavailable")
	})
	t.Run("pay and chains keepers", func(t *testing.T) {
		custom, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, keepers)
		require.NoError(t, err)
		for _, entry := range precompiles.MergeAddressTable {
			require.Contains(t, custom, entry.Address)
			served, ok := custom[entry.Address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
			require.True(t, ok, entry.Name)
			require.Equal(t, entry.Address, served.Address())
			require.Equal(t, entry.Name, served.GetName())
		}
	})
}

func TestInitializePrecompilesUsesMergeActivationContext(t *testing.T) {
	initialized, before := precompiles.Initialized, precompiles.PrecompileNamesToInfo
	t.Cleanup(func() {
		precompiles.Initialized, precompiles.PrecompileNamesToInfo = initialized, before
	})
	for _, contracts := range []*vm.PrecompiledContracts{
		&vm.PrecompiledContractsHomestead, &vm.PrecompiledContractsByzantium,
		&vm.PrecompiledContractsIstanbul, &vm.PrecompiledContractsBerlin,
		&vm.PrecompiledContractsCancun, &vm.PrecompiledContractsBLS,
	} {
		original := *contracts
		*contracts = maps.Clone(original)
		t.Cleanup(func() { *contracts = original })
	}
	for _, addresses := range []*[]common.Address{
		&vm.PrecompiledAddressesHomestead, &vm.PrecompiledAddressesByzantium,
		&vm.PrecompiledAddressesIstanbul, &vm.PrecompiledAddressesBerlin,
		&vm.PrecompiledAddressesCancun,
	} {
		original := *addresses
		*addresses = slices.Clone(original)
		t.Cleanup(func() { *addresses = original })
	}
	a := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, true, nil, app.MakeEncodingConfig(),
		wasm.EnableAllProposals, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: time.Now(), ConsensusParams: app.DefaultConsensusParams, ChainId: "pax-test", AppStateBytes: genesis,
	})
	require.NoError(t, err)
	keepers := precompiletest.Keepers(a)
	require.Zero(t, a.LastBlockHeight())
	require.False(t, keepers.MergePlanActive())
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: time.Now()},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, a.LastBlockHeight())
	require.False(t, keepers.MergePlanActive())
	for _, active := range []bool{false, true} {
		if active {
			_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
				Header: &tmproto.Header{ChainID: "pax-test", Height: 2, Time: time.Now()},
			})
			require.NoError(t, err)
			ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(2).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
			a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
			require.False(t, keepers.MergePlanActive())
			a.SetDeliverStateToCommit()
			_, err = a.Commit(context.Background())
			require.NoError(t, err)
		}
		require.Equal(t, active, keepers.MergePlanActive())
		precompiles.Initialized = false
		precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
		require.NoError(t, precompiles.InitializePrecompiles(false, keepers))
		for _, entry := range precompiles.MergeAddressTable {
			for _, table := range []vm.PrecompiledContracts{
				vm.PrecompiledContractsHomestead, vm.PrecompiledContractsByzantium,
				vm.PrecompiledContractsIstanbul, vm.PrecompiledContractsBerlin,
				vm.PrecompiledContractsCancun, vm.PrecompiledContractsBLS,
			} {
				if active {
					served, ok := table[entry.Address].(precompiles.IPrecompile)
					require.True(t, ok, entry.Name)
					require.Equal(t, entry.Name, served.GetName())
				} else {
					require.NotContains(t, table, entry.Address, entry.Name)
				}
			}
			for _, addresses := range [][]common.Address{
				vm.PrecompiledAddressesHomestead, vm.PrecompiledAddressesByzantium,
				vm.PrecompiledAddressesIstanbul, vm.PrecompiledAddressesBerlin,
				vm.PrecompiledAddressesCancun,
			} {
				if active {
					require.Contains(t, addresses, entry.Address, entry.Name)
				} else {
					require.NotContains(t, addresses, entry.Address, entry.Name)
				}
			}
		}
	}
}
