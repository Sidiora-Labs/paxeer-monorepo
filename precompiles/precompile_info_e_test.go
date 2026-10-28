package precompiles

import (
	"maps"
	"slices"
	"testing"

	ibctransferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	layerxbridgekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	ecommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestGetPrecompileInfoEUnknownName(t *testing.T) {
	initialized, infoBefore := Initialized, PrecompileNamesToInfo
	mergeConstructorsMtx.Lock()
	constructorsBefore := mergeConstructors
	mergeConstructors = map[ecommon.Address]MergeConstructor{}
	mergeConstructorsMtx.Unlock()
	Initialized = false
	PrecompileNamesToInfo = map[string]PrecompileInfo{}
	t.Cleanup(func() {
		Initialized, PrecompileNamesToInfo = initialized, infoBefore
		mergeConstructorsMtx.Lock()
		defer mergeConstructorsMtx.Unlock()
		mergeConstructors = constructorsBefore
	})

	const name = "unknown_precompile"
	const message = name + " doesn't exist as a precompile"
	info, err := GetPrecompileInfoE(name)
	require.EqualError(t, err, message)
	require.Equal(t, PrecompileInfo{}, info)
	require.False(t, Initialized)
	info, err = GetPrecompileInfoE("bank")
	require.NoError(t, err)
	require.Equal(t, ecommon.HexToAddress(bank.BankAddress), info.Address)
	require.NotEmpty(t, info.ABI.Methods)

	Initialized = true
	info, err = GetPrecompileInfoE(name)
	require.EqualError(t, err, message)
	require.Equal(t, PrecompileInfo{}, info)
	func() {
		defer func() {
			panicErr, ok := recover().(error)
			require.True(t, ok, "GetPrecompileInfo must panic with an error")
			require.EqualError(t, panicErr, message)
		}()
		GetPrecompileInfo(name)
	}()

	address := ecommon.HexToAddress("0x0908")
	constructor := constructorsBefore[address]
	require.NotNil(t, constructor, "the real EIP-712 constructor must be registered")
	mergeConstructorsMtx.Lock()
	mergeConstructors[address] = constructor
	mergeConstructorsMtx.Unlock()
	Initialized = false
	initializationErr := InitializePrecompiles(true, &utils.EmptyKeepers{})
	require.ErrorContains(t, initializationErr, "evm keeper is unavailable")
	info, err = GetPrecompileInfoE(name)
	require.EqualError(t, err, initializationErr.Error())
	require.Equal(t, PrecompileInfo{}, info)
	func() {
		defer func() {
			panicErr, ok := recover().(error)
			require.True(t, ok, "GetPrecompileInfo must propagate initialization errors")
			require.EqualError(t, panicErr, initializationErr.Error())
		}()
		GetPrecompileInfo(name)
	}()
}

type initializationKeepers struct {
	utils.EmptyKeepers
	evm       *evmkeeper.Keeper
	bridge    *layerxbridgekeeper.Keeper
	launchpad *launchpadkeeper.Keeper
}

func (k *initializationKeepers) EVMK() utils.EVMKeeper                     { return k.evm }
func (k *initializationKeepers) LayerXBridgeK() *layerxbridgekeeper.Keeper { return k.bridge }
func (k *initializationKeepers) LaunchpadK() *launchpadkeeper.Keeper       { return k.launchpad }

func TestInitializePrecompilesKeepsMergeOutOfVM(t *testing.T) {
	initialized, infoBefore := Initialized, PrecompileNamesToInfo
	Initialized = false
	PrecompileNamesToInfo = map[string]PrecompileInfo{}
	t.Cleanup(func() {
		Initialized, PrecompileNamesToInfo = initialized, infoBefore
	})
	mergeConstructorsMtx.Lock()
	constructorsBefore := mergeConstructors
	address := ecommon.HexToAddress("0x0908")
	constructor := constructorsBefore[address]
	mergeConstructors = map[ecommon.Address]MergeConstructor{address: constructor}
	mergeConstructorsMtx.Unlock()
	t.Cleanup(func() {
		mergeConstructorsMtx.Lock()
		defer mergeConstructorsMtx.Unlock()
		mergeConstructors = constructorsBefore
	})
	require.NotNil(t, constructor, "the real EIP-712 constructor must be registered")

	for _, contracts := range []*vm.PrecompiledContracts{
		&vm.PrecompiledContractsHomestead, &vm.PrecompiledContractsByzantium,
		&vm.PrecompiledContractsIstanbul, &vm.PrecompiledContractsBerlin,
		&vm.PrecompiledContractsCancun, &vm.PrecompiledContractsBLS,
	} {
		before := *contracts
		*contracts = maps.Clone(before)
		t.Cleanup(func() { *contracts = before })
		for _, entry := range MergeAddressTable {
			require.NotContains(t, *contracts, entry.Address, entry.Name)
		}
	}
	for _, addresses := range []*[]ecommon.Address{
		&vm.PrecompiledAddressesHomestead, &vm.PrecompiledAddressesByzantium,
		&vm.PrecompiledAddressesIstanbul, &vm.PrecompiledAddressesBerlin,
		&vm.PrecompiledAddressesCancun,
	} {
		before := *addresses
		*addresses = slices.Clone(before)
		t.Cleanup(func() { *addresses = before })
	}

	subspace := paramtypes.NewSubspace(nil, nil, nil, nil, "evm")
	evm := evmkeeper.NewKeeper(nil, nil, subspace, nil,
		nil, nil, nil, ibctransferkeeper.Keeper{}, nil, nil, &upgradekeeper.Keeper{})
	keepers := &initializationKeepers{
		evm:       evm,
		bridge:    &layerxbridgekeeper.Keeper{},
		launchpad: launchpadkeeper.NewKeeper(nil, nil, nil, evm),
	}
	merge, err := newMergePrecompiles(keepers)
	require.NoError(t, err)
	require.Len(t, merge, 1)
	require.Equal(t, address, merge[0].Address())
	require.Equal(t, "eip712", merge[0].GetName())
	require.NoError(t, InitializePrecompiles(false, keepers))
	require.True(t, Initialized)
	require.Equal(t, address, PrecompileNamesToInfo["eip712"].Address)
	require.NotEmpty(t, PrecompileNamesToInfo["eip712"].ABI.Methods)
	require.Contains(t, vm.PrecompiledContractsCancun, ecommon.HexToAddress(bank.BankAddress))
	for _, entry := range MergeAddressTable {
		for _, contracts := range []vm.PrecompiledContracts{
			vm.PrecompiledContractsHomestead, vm.PrecompiledContractsByzantium,
			vm.PrecompiledContractsIstanbul, vm.PrecompiledContractsBerlin,
			vm.PrecompiledContractsCancun, vm.PrecompiledContractsBLS,
		} {
			require.NotContains(t, contracts, entry.Address, entry.Name)
		}
		for _, addresses := range [][]ecommon.Address{
			vm.PrecompiledAddressesHomestead, vm.PrecompiledAddressesByzantium,
			vm.PrecompiledAddressesIstanbul, vm.PrecompiledAddressesBerlin,
			vm.PrecompiledAddressesCancun,
		} {
			require.NotContains(t, addresses, entry.Address, entry.Name)
		}
	}
}
