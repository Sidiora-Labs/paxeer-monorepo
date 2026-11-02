package spot_test

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	spottypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestGovernanceListedTokenTradesThroughPECOR(t *testing.T) {
	f := newProductionSpotRails(t)
	a := f.app
	ctx := f.ctx.WithBlockTime(time.Unix(1_800_000_000, 0)).WithEvmEventManager(sdk.NewEVMEventManager())
	a.EvmKeeper.BeginBlock(ctx)
	pecor := common.HexToAddress("0x1021")
	vault := common.HexToAddress("0x1025")
	pecorABI, vaultABI := f.contract(pecor).GetABI(), f.contract(vault).GetABI()
	usdl := common.HexToAddress(spottypes.USDLAddress)
	lp := hyperpaxAddress("governance listing liquidity provider")
	user := hyperpaxAddress("governance listing trader")
	deployer := hyperpaxAddress("governance listing USDL deployer")
	raw, err := os.ReadFile("../launchpad/abi/SidioraERC20.json")
	require.NoError(t, err)
	erc20, err := abi.JSON(bytes.NewReader(raw))
	require.NoError(t, err)
	call := func(from, to common.Address, input []byte) []byte {
		t.Helper()
		ret, err := a.EvmKeeper.CallEVM(ctx, from, &to, nil, input)
		require.NoError(t, err, "production EVM call to %s: %x", to.Hex(), input[:4])
		return ret
	}
	read := func(to common.Address, contract abi.ABI, method string, args ...interface{}) []interface{} {
		t.Helper()
		ret, err := a.EvmKeeper.StaticCallEVM(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, user), &to,
			packed(t, contract, method, args...))
		require.NoError(t, err, method)
		values, err := contract.Unpack(method, ret)
		require.NoError(t, err, method)
		return values
	}
	constructor, err := erc20.Constructor.Inputs.Pack("USD Ledger", "USDL", big.NewInt(1_000_000e6), lp)
	require.NoError(t, err)
	nonce := a.EvmKeeper.GetNonce(ctx, deployer)
	_, err = a.EvmKeeper.CallEVM(ctx, deployer, nil, nil, append(common.FromHex(ordersTokenCreation), constructor...))
	require.NoError(t, err)
	deployed := crypto.CreateAddress(deployer, nonce)
	code := a.EvmKeeper.GetCode(ctx, deployed)
	require.NotEmpty(t, code)
	var slots [][2]common.Hash
	a.EvmKeeper.IterateState(ctx, func(address common.Address, key, value common.Hash) bool {
		if address == deployed {
			slots = append(slots, [2]common.Hash{key, value})
		}
		return false
	})
	require.NotEmpty(t, slots)
	a.EvmKeeper.SetCode(ctx, usdl, code)
	for _, slot := range slots {
		a.EvmKeeper.SetState(ctx, usdl, slot[0], slot[1])
	}

	const denom, symbol, decimals = "ugovlisted", "GOVLIST", uint8(8)
	a.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{Base: denom, Display: symbol, Name: symbol, Symbol: symbol,
		DenomUnits: []*banktypes.DenomUnit{{Denom: denom}, {Denom: symbol, Exponent: uint32(decimals)}}})
	require.NoError(t, a.EvmKeeper.RunWithOneOffEVMInstance(ctx, func(evm *vm.EVM) error {
		_, err := a.EvmKeeper.UpsertERCNativePointer(ctx, evm, denom,
			utils.ERCMetadata{Name: symbol, Symbol: symbol, Decimals: decimals})
		return err
	}, func(string, string) {}))
	token, _, exists := a.EvmKeeper.GetERC20NativePointer(ctx, denom)
	require.True(t, exists)
	require.NotEmpty(t, a.EvmKeeper.GetCode(ctx, token))
	require.Equal(t, false, read(vault, vaultABI, "getTokenInfo", token)[0])

	feedParams := a.PricefeedKeeper.GetParams(ctx)
	marketID := uint64(len(feedParams.Markets))
	feedParams.Markets = append(feedParams.Markets, pricefeedtypes.Market{ID: marketID, Symbol: symbol,
		ChainflowSymbol: symbol, MinPrice: sdk.OneInt(), MaxPrice: sdk.NewIntWithDecimal(1, 30), Enabled: true})
	require.NoError(t, a.PricefeedKeeper.SetParams(ctx, feedParams))
	price := sdk.NewIntWithDecimal(2, 18)
	aggregate, err := json.Marshal(pricefeedtypes.Aggregate{IndexPrice: price, MarkPrice: price, Basis: sdk.ZeroInt(),
		TimestampMs: uint64(ctx.BlockTime().UnixMilli()), Height: ctx.BlockHeight(), Sources: 1})
	require.NoError(t, err)
	ctx.KVStore(a.GetKey(pricefeedtypes.StoreKey)).Set(pricefeedtypes.AggregateKey(marketID), aggregate)
	feedPrice, err := a.PricefeedKeeper.GetPrice(ctx, marketID)
	require.NoError(t, err)
	require.True(t, feedPrice.Fresh)
	require.Equal(t, price, feedPrice.Index)

	params, err := a.SpotKeeper.GetParams(ctx)
	require.NoError(t, err)
	params.Tokens = append(params.Tokens,
		spottypes.TokenParams{Address: usdl, Kind: spottypes.TokenKindERC20, IsStablecoin: true,
			Decimals: spottypes.OpeningTokenDecimals, MaxUtilizationBps: 9_000},
		spottypes.TokenParams{Address: token, Kind: spottypes.TokenKindNative, MarketID: marketID, MaxUtilizationBps: 9_000})
	proposal, err := layerxgovtypes.NewLayerXProposal("List GOVLIST on spot", "Register the priced native token for PECOR trading.",
		&spottypes.MsgUpdateParams{Authority: layerxgovtypes.GovernanceAuthority(), Params: params})
	require.NoError(t, err)
	require.NoError(t, layerxgov.NewProposalHandler(a.MsgServiceRouter())(ctx, proposal))
	registered, err := a.SpotKeeper.GetParams(ctx)
	require.NoError(t, err)
	listed := params.Tokens[len(params.Tokens)-1]
	require.Zero(t, listed.Decimals)
	listed.Decimals = uint32(decimals)
	require.Contains(t, registered.Tokens, listed)
	book, err := a.SpotKeeper.GetBook(ctx, token)
	require.NoError(t, err)
	require.Equal(t, decimals, book.Decimals)
	require.Equal(t, a.SpotKeeper.ShareDenom(token), book.ShareDenom)
	require.True(t, book.Reserves.IsZero())
	info := read(vault, vaultABI, "getTokenInfo", token)
	require.Len(t, info, 6)
	require.Equal(t, []interface{}{true, false, decimals}, info[:3])
	for _, value := range info[3:] {
		require.Zero(t, value.(*big.Int).Sign())
	}

	for _, pool := range []struct {
		token    common.Address
		symbol   string
		decimals uint8
	}{{usdl, "USDL", spottypes.OpeningTokenDecimals}, {token, symbol, decimals}} {
		require.NoError(t, a.EvmKeeper.RunWithOneOffEVMInstance(ctx, func(evm *vm.EVM) error {
			_, err := a.SpotKeeper.CreateShareToken(ctx, evm, pool.token, pool.symbol, pool.decimals)
			return err
		}, func(string, string) {}))
		share, _, exists := a.EvmKeeper.GetERC20NativePointer(ctx, a.SpotKeeper.ShareDenom(pool.token))
		require.True(t, exists)
		require.NotEmpty(t, a.EvmKeeper.GetCode(ctx, share))
	}

	liquidity := sdk.NewInt(100_000_00000000)
	coins := sdk.NewCoins(sdk.NewCoin(denom, liquidity))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName,
		a.EvmKeeper.GetPaxAddressOrDefault(ctx, lp), coins))
	require.Equal(t, common.LeftPadBytes([]byte{1}, 32),
		call(lp, usdl, packed(t, erc20, "approve", vault, big.NewInt(500_000e6))))
	call(lp, vault, packed(t, vaultABI, "deposit", usdl, big.NewInt(500_000e6)))
	call(lp, vault, packed(t, vaultABI, "deposit", token, liquidity.BigInt()))
	before, err := a.SpotKeeper.GetBook(ctx, token)
	require.NoError(t, err)
	require.Equal(t, liquidity, before.Reserves)
	require.Equal(t, liquidity, before.TotalDeposited)
	require.Equal(t, before.Reserves.BigInt(), read(vault, vaultABI, "getReserves", token)[0])
	amountIn := big.NewInt(100e6)
	require.Equal(t, common.LeftPadBytes([]byte{1}, 32), call(lp, usdl, packed(t, erc20, "transfer", user, amountIn)))
	require.Equal(t, common.LeftPadBytes([]byte{1}, 32), call(user, usdl, packed(t, erc20, "approve", pecor, amountIn)))
	userBefore := a.BankKeeper.GetBalance(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, user), denom).Amount
	input := packed(t, pecorABI, "swapExactIn", usdl, token, amountIn, big.NewInt(1), big.NewInt(ctx.BlockTime().Unix()+60))
	require.Equal(t, uint64(60_000)+16*uint64(len(input)-4), f.contract(pecor).RequiredGas(input))
	result, err := pecorABI.Unpack("swapExactIn", call(user, pecor, input))
	require.NoError(t, err)
	require.Len(t, result, 1)
	output := sdk.NewIntFromBigInt(result[0].(*big.Int))
	require.True(t, output.IsPositive())
	after, err := a.SpotKeeper.GetBook(ctx, token)
	require.NoError(t, err)
	require.Equal(t, before.Reserves.Sub(output), after.Reserves)
	require.Equal(t, before.TotalDeposited, after.TotalDeposited)
	require.Equal(t, userBefore.Add(output), a.BankKeeper.GetBalance(ctx,
		a.EvmKeeper.GetPaxAddressOrDefault(ctx, user), denom).Amount)
	require.Zero(t, read(usdl, erc20, "balanceOf", user)[0].(*big.Int).Sign())
	info = read(vault, vaultABI, "getTokenInfo", token)
	require.Equal(t, true, info[0])
	require.Equal(t, decimals, info[2])
	require.Equal(t, after.Reserves.BigInt(), info[3])
}
