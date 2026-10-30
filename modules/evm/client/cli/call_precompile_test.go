package cli_test

import (
	"io"
	"maps"
	"slices"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func initializeCallPrecompileRegistry(t *testing.T) {
	t.Helper()
	initialized := precompiles.Initialized
	infoBefore := precompiles.PrecompileNamesToInfo
	precompiles.Initialized = false
	precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
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
	require.NoError(t, precompiles.InitializePrecompiles(false, testkeeper.EVMTestApp.GetPrecompileKeepers()))
}

func TestCallPrecompileUnknownNameReturnsError(t *testing.T) {
	initializeCallPrecompileRegistry(t)
	cmd := cli.CmdCallPrecompile()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"unknown-precompile", "totalMarkets"})

	require.NotPanics(t, func() {
		require.EqualError(t, cmd.Execute(), "unknown-precompile doesn't exist as a precompile")
	})
}

func TestCallPrecompileResolvesPerps(t *testing.T) {
	initializeCallPrecompileRegistry(t)
	info, err := precompiles.GetPrecompileInfoE("perps")
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x1015"), info.Address)
	wantABI, err := perpsabi.ABI()
	require.NoError(t, err)
	require.Equal(t, wantABI, info.ABI)

	for _, entry := range []struct {
		name    string
		address string
		method  string
	}{
		{"perps", "0x1015", "quoteMarket"},
		{"pricefeed", "0x1020", "getPrice"},
		{"spot_pecor", "0x1021", "accumulatedFees"},
		{"spot_orders", "0x1022", "activeLimitOrderIds"},
		{"spot_stoporders", "0x1023", "activeStopLimitOrderIds"},
		{"spot_quoter", "0x1024", "getLiquidityInfo"},
		{"spot_vault", "0x1025", "authorizedOperators"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			info, err := precompiles.GetPrecompileInfoE(entry.name)
			require.NoError(t, err)
			require.Equal(t, common.HexToAddress(entry.address), info.Address)
			method, found := info.ABI.Methods[entry.method]
			require.True(t, found)
			require.NotEmpty(t, method.Inputs)

			cmd := cli.CmdCallPrecompile()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{entry.name, entry.method})
			require.NotPanics(t, func() {
				require.EqualError(t, cmd.Execute(), "not enough arguments")
			})
		})
	}
}
