package launchpad_test

import (
	"math/big"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func chainHarness(t *testing.T) (*harness, *big.Int) {
	t.Helper()
	h := newHarness(t)
	h.keeper.SetChainRegistry(testkeeper.EVMTestApp.LayerXChainsKeeper)
	h.stateDB.WithCtx(h.ctx().WithBlockHeight(42))
	h.evm.Context.BlockNumber = big.NewInt(42)
	params := testkeeper.EVMTestApp.LayerXChainsKeeper.GetParams(h.ctx())
	value := params.MinBond.Add(params.CreationFee)
	funds := sdk.NewCoins(sdk.NewCoin(testkeeper.EVMTestApp.EvmKeeper.GetBaseDenom(h.ctx()), value.MulRaw(3)))
	require.NoError(t, testkeeper.EVMTestApp.BankKeeper.MintCoins(h.ctx(), evmtypes.ModuleName, funds))
	require.NoError(t, testkeeper.EVMTestApp.BankKeeper.SendCoinsFromModuleToAccount(h.ctx(), evmtypes.ModuleName,
		testkeeper.EVMTestApp.EvmKeeper.GetPaxAddressOrDefault(h.ctx(), h.caller), funds))
	h.approve(h.usdl, h.caller, h.creationFee())
	return h, new(big.Int).Mul(value.BigInt(), state.UhpxToSweiMultiplier)
}

func launchThroughABI(t *testing.T, h *harness, value *big.Int, name string) []interface{} {
	t.Helper()
	params := testkeeper.EVMTestApp.LayerXChainsKeeper.GetParams(h.ctx())
	input := pack(t, directRole, "launchChain", "Chain Token", "CHN", uint8(types.FeeStrategyClaim),
		common.Address{}, name, params.MinGasAllowance, params.MinByteAllowance, uint16(100))
	ret, _, err := h.evm.Call(h.caller, precompile, input, callGas, uint256.MustFromBig(value))
	require.NoError(t, err, "%x", ret)
	out, err := method(t, directRole, "launchChain").Outputs.Unpack(ret)
	require.NoError(t, err)
	return out
}

func TestChainLaunchReturnsTokenPoolRightChainAndChainId(t *testing.T) {
	h, value := chainHarness(t)
	out := launchThroughABI(t, h, value, "chain-token")
	token, pool, nftID := out[0].(common.Address), out[1].(common.Address), out[2].(*big.Int)
	chain, chainID := out[3].(uint32), out[4].(uint64)
	require.NotEqual(t, common.Address{}, token)
	require.NotEqual(t, common.Address{}, pool)
	require.True(t, nftID.Sign() > 0)
	require.Equal(t, chainstypes.ChainIDOf(chain).Uint64(), chainID)
	record, found, err := h.keeper.GetLaunchedChain(h.ctx(), uint64(chain))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, token.Hex(), record.Token)
	require.Equal(t, pool.Hex(), record.Pool)
	require.Equal(t, nftID.Uint64(), record.FeeRightID)
	domain, found := testkeeper.EVMTestApp.LayerXChainsKeeper.GetDomain(h.ctx(), chain)
	require.True(t, found)
	require.Equal(t, common.BytesToAddress(types.ChainRevenueAccount(uint64(chain))), domain.Operator)
}

func TestChainLaunchEmitsChainLaunchedAndTheMarketEvents(t *testing.T) {
	h, value := chainHarness(t)
	before := h.logCount()
	out := launchThroughABI(t, h, value, "chain-events")
	chain, pool, token := out[3].(uint32), out[1].(common.Address), out[0].(common.Address)
	signature := "ChainLaunched(uint32,address,address,uint64,uint256,address,uint256,uint256)"
	logs := h.logs(before, precompile, signature)
	require.Len(t, logs, 1)
	require.Equal(t, common.BigToHash(new(big.Int).SetUint64(uint64(chain))), logs[0].Topics[1])
	require.Equal(t, common.BytesToHash(pool.Bytes()), logs[0].Topics[2])
	require.Equal(t, common.BytesToHash(token.Bytes()), logs[0].Topics[3])
	require.Len(t, h.logs(before, precompile, "MarketCreated(bytes32,address,address,address,address,uint256,uint256)"), 1)
	require.Len(t, h.logs(before, factory, "MarketCreated(address,address,address,uint256,address)"), 1)
}

func TestChainLaunchRevertLeavesNoState(t *testing.T) {
	h, value := chainHarness(t)
	params := testkeeper.EVMTestApp.LayerXChainsKeeper.GetParams(h.ctx())
	next := testkeeper.EVMTestApp.LayerXChainsKeeper.NextID(h.ctx())
	count, logs := h.keeper.GetMarketCount(h.ctx()), h.logCount()
	input := pack(t, directRole, "launchChain", "Chain Token", "CHN", uint8(types.FeeStrategyClaim),
		common.Address{}, "short-bond", params.MinGasAllowance, params.MinByteAllowance, uint16(100))
	short := new(big.Int).Sub(value, state.UhpxToSweiMultiplier)
	_, _, err := h.evm.Call(h.caller, precompile, input, callGas, uint256.MustFromBig(short))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, count, h.keeper.GetMarketCount(h.ctx()))
	require.Equal(t, next, testkeeper.EVMTestApp.LayerXChainsKeeper.NextID(h.ctx()))
	require.Equal(t, logs, h.logCount())
	_, found, err := h.keeper.GetLaunchedChain(h.ctx(), uint64(next))
	require.NoError(t, err)
	require.False(t, found)
}

func TestChainLaunchDuplicateNameKeepsTheFirstMarket(t *testing.T) {
	h, value := chainHarness(t)
	first := launchThroughABI(t, h, value, "reserved-chain")
	count := h.keeper.GetMarketCount(h.ctx())
	next := testkeeper.EVMTestApp.LayerXChainsKeeper.NextID(h.ctx())
	params := testkeeper.EVMTestApp.LayerXChainsKeeper.GetParams(h.ctx())
	input := pack(t, directRole, "launchChain", "Another Token", "NEXT", uint8(types.FeeStrategyClaim),
		common.Address{}, "reserved-chain", params.MinGasAllowance, params.MinByteAllowance, uint16(100))
	_, _, err := h.evm.Call(h.caller, precompile, input, callGas, uint256.MustFromBig(value))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, count, h.keeper.GetMarketCount(h.ctx()))
	require.Equal(t, next, testkeeper.EVMTestApp.LayerXChainsKeeper.NextID(h.ctx()))
	require.Equal(t, first[0], h.view(precompile, directRole, "launchedChain", first[3].(uint32))[3])
}

func TestChainLaunchStaticCallIsRefused(t *testing.T) {
	h, value := chainHarness(t)
	params := testkeeper.EVMTestApp.LayerXChainsKeeper.GetParams(h.ctx())
	input := pack(t, directRole, "launchChain", "Chain Token", "CHN", uint8(types.FeeStrategyClaim),
		common.Address{}, "static-chain", params.MinGasAllowance, params.MinByteAllowance, uint16(100))
	_, err := h.direct(h.caller, input, value, true, false)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	_, err = h.direct(h.caller, input, value, false, true)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Zero(t, h.keeper.GetMarketCount(h.ctx()))
}

func TestChainLaunchReadsReturnTheRecord(t *testing.T) {
	h, value := chainHarness(t)
	out := launchThroughABI(t, h, value, "chain-reads")
	chain, pool := out[3].(uint32), out[1].(common.Address)
	record := h.view(precompile, directRole, "launchedChain", chain)
	require.Equal(t, chain, record[0])
	require.Equal(t, out[4], record[1])
	require.Equal(t, pool, record[2])
	require.Equal(t, out[0], record[3])
	require.Equal(t, out[2], record[4])
	require.Equal(t, h.caller, record[5])
	require.Equal(t, uint64(42), record[6])
	require.Equal(t, chain, h.view(precompile, directRole, "launchedChainOfPool", pool)[0])
	require.Equal(t, out[4], h.view(precompile, directRole, "homeChainOf", pool)[0])
	require.Equal(t, uint32(0), h.view(precompile, directRole, "launchedChainOfPool", h.caller)[0])
	require.Equal(t, uint64(0), h.view(precompile, directRole, "homeChainOf", h.caller)[0])
}

func TestChainRevenueOpenAndClaimThroughTheAbi(t *testing.T) {
	h, _ := chainHarness(t)
	token := h.deployERC20("Revenue Token", "REV", big.NewInt(100), h.caller)
	pool := common.HexToAddress("0x000000000000000000000000000000000000A321")
	chain := testkeeper.EVMTestApp.LayerXChainsKeeper.NextID(h.ctx())
	require.NoError(t, h.keeper.SetLaunchedChain(h.ctx(), types.LaunchedChain{
		RegistryNumber: uint64(chain), ChainID: chainstypes.ChainIDOf(chain).Uint64(),
		Pool: pool.Hex(), Token: token.Hex(), FeeRightID: 1, Creator: h.caller.Hex(), LaunchBlock: 42,
	}))
	h.stateDB.WithCtx(h.ctx().WithBlockHeight(43))
	h.evm.Context.BlockNumber = big.NewInt(43)
	denom := testkeeper.EVMTestApp.EvmKeeper.GetBaseDenom(h.ctx())
	revenue := sdk.NewCoins(sdk.NewInt64Coin(denom, 101))
	require.NoError(t, testkeeper.EVMTestApp.BankKeeper.MintCoins(h.ctx(), evmtypes.ModuleName, revenue))
	require.NoError(t, testkeeper.EVMTestApp.BankKeeper.SendCoinsFromModuleToAccount(h.ctx(), evmtypes.ModuleName,
		types.ChainRevenueAccount(uint64(chain)), revenue))
	opened := h.do(h.caller, precompile, directRole, "openChainRevenueEpoch", chain)
	require.Equal(t, uint64(1), opened[0])
	require.Equal(t, new(big.Int).Mul(big.NewInt(101), state.UhpxToSweiMultiplier), opened[1])
	epoch := h.view(precompile, directRole, "chainRevenueEpoch", chain, uint64(1))
	require.Equal(t, uint64(42), epoch[3])
	require.Equal(t, big.NewInt(100), epoch[4])
	claimable := h.view(precompile, directRole, "chainRevenueClaimable", chain, uint64(1), h.caller)[0].(*big.Int)
	require.Equal(t, opened[1], claimable)
	paid := h.do(h.caller, precompile, directRole, "claimChainRevenue", chain, uint64(1))[0]
	require.Equal(t, claimable, paid)
	require.Equal(t, crypto.Keccak256([]byte("AlreadyClaimed()"))[:4], h.fails(h.caller, precompile, directRole, "claimChainRevenue", chain, uint64(1))[:4])
}

func TestLaunchedChainControlThroughTheAbi(t *testing.T) {
	h, value := chainHarness(t)
	out := launchThroughABI(t, h, value, "chain-control")
	chain := out[3].(uint32)
	other := h.account()
	require.Equal(t, crypto.Keccak256([]byte("NotFeeRightHolder()"))[:4],
		h.fails(other, precompile, directRole, "pauseLaunchedChain", chain)[:4])
	h.do(h.caller, precompile, directRole, "pauseLaunchedChain", chain)
	domain, found := testkeeper.EVMTestApp.LayerXChainsKeeper.GetDomain(h.ctx(), chain)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusPaused, domain.Status)
	h.do(h.caller, precompile, directRole, "resumeLaunchedChain", chain)
	domain, found = testkeeper.EVMTestApp.LayerXChainsKeeper.GetDomain(h.ctx(), chain)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusActive, domain.Status)
	h.do(h.caller, precompile, directRole, "retireLaunchedChain", chain)
	domain, found = testkeeper.EVMTestApp.LayerXChainsKeeper.GetDomain(h.ctx(), chain)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusRetired, domain.Status)
	require.Len(t, h.logs(0, precompile, "LaunchedChainRetired(uint32,address,uint256,uint256,uint256)"), 1)
}

func TestHomeChainRefusalIsWrongChain(t *testing.T) {
	h, value := chainHarness(t)
	out := launchThroughABI(t, h, value, "chain-home")
	pool := out[1].(common.Address)
	h.approve(h.usdl, h.caller, big.NewInt(1_000_000))
	data := h.fails(h.caller, router, types.RoleRouter, "buy", pool, big.NewInt(1_000_000), big.NewInt(1), deadline)
	require.Equal(t, crypto.Keccak256([]byte("WrongChain()"))[:4], data[:4])
}

func TestChainLaunchAbiMatchesSolidityInterface(t *testing.T) {
	source, err := os.ReadFile("Launchpad.sol")
	require.NoError(t, err)
	direct := roleABI(t, directRole)
	functions := []string{"launchChain", "launchChainWithPermit", "openChainRevenueEpoch", "claimChainRevenue",
		"claimChainRevenueBatch", "pauseLaunchedChain", "resumeLaunchedChain", "retireLaunchedChain",
		"launchedChain", "launchedChainOfPool", "homeChainOf", "chainRevenueEpoch", "chainRevenueClaimable"}
	for _, name := range functions {
		declaration := regexp.MustCompile(`(?m)^    function ` + name + `\((.*?)\) external`).FindStringSubmatch(string(source))
		require.Len(t, declaration, 2, name)
		var typesIn []string
		for _, field := range strings.Split(declaration[1], ",") {
			if strings.TrimSpace(field) == "" {
				continue
			}
			typ := strings.Fields(field)[0]
			if typ == "PermitParams" {
				typ = "(uint256,uint256,uint8,bytes32,bytes32)"
			}
			typesIn = append(typesIn, typ)
		}
		signature := name + "(" + strings.Join(typesIn, ",") + ")"
		method := direct.Methods[name]
		require.Equal(t, signature, method.Sig)
		require.Equal(t, crypto.Keccak256([]byte(signature))[:4], method.ID)
	}
	for _, name := range []string{"ChainLaunched", "ChainRevenueEpochOpened", "ChainRevenueClaimed", "LaunchedChainRetired"} {
		declaration := regexp.MustCompile(`(?m)^    event ` + name + `\((.*?)\);`).FindStringSubmatch(string(source))
		require.Len(t, declaration, 2)
		var typesIn []string
		for _, field := range strings.Split(declaration[1], ",") {
			typesIn = append(typesIn, strings.Fields(field)[0])
		}
		require.Equal(t, crypto.Keccak256Hash([]byte(name+"("+strings.Join(typesIn, ",")+")")), direct.Events[name].ID)
	}
	require.GreaterOrEqual(t, launchpad.Gas(63, launchpad.Writes("launchChain")),
		launchpad.Gas(63, launchpad.Writes("createMarket"))+60_000)
}
