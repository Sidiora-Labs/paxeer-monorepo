package spot_test

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	spotkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	spottypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	spotprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func listingDisk(t *testing.T, source sdk.Context) (sdk.Context, func(sdk.Context) sdk.Context) {
	t.Helper()
	directory := t.TempDir()
	keys := source.MultiStore().StoreKeys()
	var root *rootmulti.Store
	open := func() {
		db, err := dbm.NewDB("listing", dbm.GoLevelDBBackend, directory)
		require.NoError(t, err)
		root = rootmulti.NewStore(db)
		for _, key := range keys {
			kind := storetypes.StoreTypeIAVL
			switch key.(type) {
			case *storetypes.TransientStoreKey:
				kind = storetypes.StoreTypeTransient
			case *storetypes.MemoryStoreKey:
				kind = storetypes.StoreTypeMemory
			}
			root.MountStoreWithDB(key, kind, nil)
		}
		require.NoError(t, root.LoadLatestVersion())
	}
	open()
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	for _, key := range keys {
		it := source.MultiStore().GetKVStore(key).Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			root.GetKVStore(key).Set(it.Key(), it.Value())
		}
		require.NoError(t, it.Close())
	}
	reopen := func(ctx sdk.Context) sdk.Context {
		t.Helper()
		root.Commit(true)
		require.NoError(t, root.Close())
		root = nil
		open()
		return ctx.WithMultiStore(root).WithEventManager(sdk.NewEventManager()).
			WithEvmEventManager(sdk.NewEVMEventManager()).
			WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	}
	return source.WithMultiStore(root), reopen
}

func TestGovernanceListedTokenTradesThroughPECOR(t *testing.T) {
	f := newProductionSpotRails(t)
	a := f.app
	ctx := f.ctx.WithBlockTime(time.Unix(1_800_000_000, 0)).WithEvmEventManager(sdk.NewEVMEventManager())
	a.EvmKeeper.BeginBlock(ctx)
	ctx, reopen := listingDisk(t, ctx)
	shareABI := *artifacts.GetParsedABI("native")
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
	for _, pool := range []common.Address{usdl, token} {
		_, _, exists := a.EvmKeeper.GetERC20NativePointer(ctx, a.SpotKeeper.ShareDenom(pool))
		require.False(t, exists, "shares must be created by governance")
	}
	beforeListing := len(ctx.EVMEventManager().Events())
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

	shares := make(map[common.Address]common.Address)
	for _, pool := range []common.Address{usdl, token} {
		share, _, exists := a.EvmKeeper.GetERC20NativePointer(ctx, a.SpotKeeper.ShareDenom(pool))
		require.True(t, exists)
		require.NotEqual(t, common.Address{}, share)
		require.Equal(t, share, read(vault, vaultABI, "shareTokens", pool)[0])
		require.NotEmpty(t, a.EvmKeeper.GetCode(ctx, share))
		shares[pool] = share
		created := 0
		for _, log := range ctx.EVMEventManager().Events()[beforeListing:] {
			if len(log.Topics) > 1 && log.Topics[0] == vaultABI.Events["ShareTokenCreated"].ID &&
				log.Topics[1] == common.BytesToHash(pool.Bytes()) {
				require.Equal(t, vault, log.Address)
				require.Equal(t, []common.Hash{vaultABI.Events["ShareTokenCreated"].ID,
					common.BytesToHash(pool.Bytes()), common.BytesToHash(share.Bytes())}, log.Topics)
				require.Empty(t, log.Data)
				created++
			}
		}
		require.Equal(t, 1, created)
		require.Zero(t, read(share, shareABI, "totalSupply")[0].(*big.Int).Sign())
	}
	assertShares := func(pool common.Address, balance, supply *big.Int) {
		t.Helper()
		denom := a.SpotKeeper.ShareDenom(pool)
		share := shares[pool]
		pointer, _, exists := a.EvmKeeper.GetERC20NativePointer(ctx, denom)
		require.True(t, exists)
		require.Equal(t, share, pointer)
		require.Equal(t, share, read(vault, vaultABI, "shareTokens", pool)[0])
		require.NotEmpty(t, a.EvmKeeper.GetCode(ctx, share))
		require.Zero(t, balance.Cmp(read(share, shareABI, "balanceOf", lp)[0].(*big.Int)))
		require.Zero(t, balance.Cmp(a.BankKeeper.GetBalance(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, lp), denom).Amount.BigInt()))
		require.Zero(t, supply.Cmp(read(share, shareABI, "totalSupply")[0].(*big.Int)))
		require.Zero(t, supply.Cmp(a.BankKeeper.GetSupply(ctx, denom).Amount.BigInt()))
		require.Zero(t, big.NewInt(spotkeeper.MinimumShares).Cmp(read(share, shareABI,
			"balanceOf", a.SpotKeeper.CustodyEVMAddress(ctx))[0].(*big.Int)))
	}
	// Reopen the stores and reconstruct the keeper and served rails, so no
	// registration, bank or EVM state can survive only in the producer's cache.
	restart := func() {
		t.Helper()
		want, err := json.Marshal(a.SpotKeeper.ExportGenesis(ctx))
		require.NoError(t, err)
		a.SpotKeeper = nil
		ctx = reopen(ctx)
		a.SpotKeeper = spotkeeper.NewKeeper(a.GetKey(spottypes.StoreKey), a.BankKeeper, &a.EvmKeeper, a.PricefeedKeeper)
		versioned := make(map[common.Address]putils.VersionedPrecompiles, len(f.contracts))
		for address, contract := range f.contracts {
			versioned[address] = putils.VersionedPrecompiles{app.LatestUpgrade: contract}
		}
		spot, err := spotprecompile.GetVersioned(app.LatestUpgrade, app.NewPrecompileKeepers(a))
		require.NoError(t, err)
		for address, contracts := range spot {
			versioned[address] = contracts
		}
		a.EvmKeeper.SetCustomPrecompiles(versioned, app.LatestUpgrade)
		f.ctx, f.contracts = ctx, a.EvmKeeper.CustomPrecompiles(ctx)
		got, err := json.Marshal(a.SpotKeeper.ExportGenesis(ctx))
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(got))
	}

	liquidity := sdk.NewInt(100_000_00000000)
	coins := sdk.NewCoins(sdk.NewCoin(denom, liquidity))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName,
		a.EvmKeeper.GetPaxAddressOrDefault(ctx, lp), coins))
	require.Equal(t, common.LeftPadBytes([]byte{1}, 32),
		call(lp, usdl, packed(t, erc20, "approve", vault, big.NewInt(500_000e6))))
	stableSupply := big.NewInt(500_000e6)
	require.Empty(t, call(lp, vault, packed(t, vaultABI, "deposit", usdl, stableSupply)))
	require.Empty(t, call(lp, vault, packed(t, vaultABI, "deposit", token, liquidity.BigInt())))
	assertShares(usdl, new(big.Int).Sub(stableSupply, big.NewInt(spotkeeper.MinimumShares)), stableSupply)
	assertShares(token, liquidity.SubRaw(spotkeeper.MinimumShares).BigInt(), liquidity.BigInt())
	restart()
	assertShares(usdl, new(big.Int).Sub(stableSupply, big.NewInt(spotkeeper.MinimumShares)), stableSupply)
	assertShares(token, liquidity.SubRaw(spotkeeper.MinimumShares).BigInt(), liquidity.BigInt())
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
	burn := liquidity.QuoRaw(10)
	nav, err := a.SpotKeeper.NAV(ctx, token)
	require.NoError(t, err)
	expectedPayout := burn.Mul(nav).Quo(liquidity)
	lpBefore := a.BankKeeper.GetBalance(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, lp), denom).Amount
	withdrawn, err := vaultABI.Unpack("withdraw", call(lp, vault, packed(t, vaultABI, "withdraw", token, burn.BigInt())))
	require.NoError(t, err)
	require.Zero(t, expectedPayout.BigInt().Cmp(withdrawn[0].(*big.Int)))
	require.Equal(t, lpBefore.Add(expectedPayout), a.BankKeeper.GetBalance(ctx,
		a.EvmKeeper.GetPaxAddressOrDefault(ctx, lp), denom).Amount)
	remainingSupply := liquidity.Sub(burn)
	assertShares(token, remainingSupply.SubRaw(spotkeeper.MinimumShares).BigInt(), remainingSupply.BigInt())
	withdrawBook, err := a.SpotKeeper.GetBook(ctx, token)
	require.NoError(t, err)
	require.Equal(t, after.Reserves.Sub(expectedPayout), withdrawBook.Reserves)
	require.Equal(t, liquidity, withdrawBook.TotalDeposited)
	require.Equal(t, expectedPayout, withdrawBook.TotalWithdrawn)
	restart()
	assertShares(token, remainingSupply.SubRaw(spotkeeper.MinimumShares).BigInt(), remainingSupply.BigInt())
	assertShares(usdl, new(big.Int).Sub(stableSupply, big.NewInt(spotkeeper.MinimumShares)), stableSupply)
	recoveredBook, err := a.SpotKeeper.GetBook(ctx, token)
	require.NoError(t, err)
	require.Equal(t, withdrawBook, recoveredBook)
	// A further redemption proves the recovered public consumer is usable.
	lpBefore = a.BankKeeper.GetBalance(ctx, a.EvmKeeper.GetPaxAddressOrDefault(ctx, lp), denom).Amount
	nav, err = a.SpotKeeper.NAV(ctx, token)
	require.NoError(t, err)
	expectedPayout = burn.Mul(nav).Quo(remainingSupply)
	withdrawn, err = vaultABI.Unpack("withdraw", call(lp, vault, packed(t, vaultABI, "withdraw", token, burn.BigInt())))
	require.NoError(t, err)
	require.Zero(t, expectedPayout.BigInt().Cmp(withdrawn[0].(*big.Int)))
	require.Equal(t, lpBefore.Add(expectedPayout), a.BankKeeper.GetBalance(ctx,
		a.EvmKeeper.GetPaxAddressOrDefault(ctx, lp), denom).Amount)
	remainingSupply = remainingSupply.Sub(burn)
	assertShares(token, remainingSupply.SubRaw(spotkeeper.MinimumShares).BigInt(), remainingSupply.BigInt())
}
