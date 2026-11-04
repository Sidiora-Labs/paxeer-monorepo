package keeper_test

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	spotprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
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

type repair17Env struct {
	t                    *testing.T
	a                    *app.App
	ctx                  sdk.Context
	stable, native, user common.Address
	vault                abi.ABI
	reopen               func(sdk.Context) sdk.Context
}

// Persist all application stores, including the bank balances, metadata,
// accounts, code and pointer registries consumed by the production EVM.
func repair17Disk(t *testing.T, source sdk.Context) (sdk.Context, func(sdk.Context) sdk.Context) {
	t.Helper()
	directory := t.TempDir()
	keys := source.MultiStore().StoreKeys()
	var root *rootmulti.Store
	open := func() {
		db, err := dbm.NewDB("repair17", dbm.GoLevelDBBackend, directory)
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

func newRepair17Env(t *testing.T) *repair17Env {
	t.Helper()
	a := app.Setup(t, false, true, false)
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	a.UpgradeKeeper.SetDone(ctx, app.V6120Upgrade)
	a.RefreshPrecompiles(ctx)
	ctx = ctx.WithBlockHeight(10).WithBlockTime(time.Unix(1000, 0).UTC())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	a.EvmKeeper.BeginBlock(ctx)
	a.SpotKeeper.InitGenesis(ctx, *types.DefaultGenesis())
	require.NoError(t, a.PricefeedKeeper.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	ctx, reopen := repair17Disk(t, ctx)
	vault, err := spotabi.Vault()
	require.NoError(t, err)
	e := &repair17Env{t: t, a: a, ctx: ctx, user: shareAddress("repair17 user"), vault: vault, reopen: reopen}
	e.stable = e.deploy("USDL")
	a.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{Base: "uhpx", Display: "PAX", Name: "PAX", Symbol: "PAX",
		DenomUnits: []*banktypes.DenomUnit{{Denom: "uhpx"}, {Denom: "PAX", Exponent: 6}}})
	require.NoError(t, a.EvmKeeper.RunWithOneOffEVMInstance(ctx, func(evm *vm.EVM) error {
		var err error
		e.native, err = a.EvmKeeper.UpsertERCNativePointer(ctx, evm, "uhpx",
			utils.ERCMetadata{Name: "PAX", Symbol: "PAX", Decimals: 6})
		return err
	}, func(string, string) {}))
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm",
		a.EvmKeeper.GetPaxAddressOrDefault(ctx, e.user), coins))
	e.price()
	return e
}

func (e *repair17Env) deploy(symbol string) common.Address {
	e.t.Helper()
	stringType, err := abi.NewType("string", "", nil)
	require.NoError(e.t, err)
	uintType, err := abi.NewType("uint256", "", nil)
	require.NoError(e.t, err)
	addressType, err := abi.NewType("address", "", nil)
	require.NoError(e.t, err)
	args, err := (abi.Arguments{{Type: stringType}, {Type: stringType}, {Type: uintType}, {Type: addressType}}).
		Pack("USD Ledger", symbol, big.NewInt(1_000_000_000_000), e.user)
	require.NoError(e.t, err)
	deployer := shareAddress("repair17 deployer")
	nonce := e.a.EvmKeeper.GetNonce(e.ctx, deployer)
	_, err = e.a.EvmKeeper.CallEVM(e.ctx, deployer, nil, nil, append(common.FromHex(limitTokenCreation), args...))
	require.NoError(e.t, err)
	address := crypto.CreateAddress(deployer, nonce)
	require.NotEmpty(e.t, e.a.EvmKeeper.GetCode(e.ctx, address))
	return address
}

func (e *repair17Env) price() {
	e.t.Helper()
	raw, err := json.Marshal(pricefeedtypes.Aggregate{IndexPrice: limitPrice(500), MarkPrice: limitPrice(500),
		Basis: sdk.ZeroInt(), TimestampMs: uint64(e.ctx.BlockTime().UnixMilli()), Height: e.ctx.BlockHeight(), Sources: 1})
	require.NoError(e.t, err)
	e.ctx.KVStore(e.a.GetKey(pricefeedtypes.StoreKey)).Set(pricefeedtypes.AggregateKey(types.PAXMarketID), raw)
}

func (e *repair17Env) govern(params types.Params) error {
	e.t.Helper()
	proposal, err := layerxgovtypes.NewLayerXProposal("Update spot pools", "Update the spot pool registry.",
		&types.MsgUpdateParams{Authority: e.a.SpotKeeper.Authority(), Params: params})
	require.NoError(e.t, err)
	require.NoError(e.t, proposal.ValidateBasic())
	return layerxgov.NewProposalHandler(e.a.MsgServiceRouter())(e.ctx, proposal)
}

func (e *repair17Env) listing() types.Params {
	e.t.Helper()
	params, err := e.a.SpotKeeper.GetParams(e.ctx)
	require.NoError(e.t, err)
	params.PriceImpactEnabled, params.OrderPriceImpactEnabled = false, false
	params.Tokens = []types.TokenParams{
		{Address: e.stable, Kind: types.TokenKindERC20, IsStablecoin: true, Decimals: 6, MaxUtilizationBps: 9000},
		{Address: e.native, Kind: types.TokenKindNative, Decimals: 6, MarketID: types.PAXMarketID, MaxUtilizationBps: 9000},
	}
	return params
}

func (e *repair17Env) call(from, to common.Address, contract abi.ABI, method string, args ...interface{}) []interface{} {
	e.t.Helper()
	input, err := contract.Pack(method, args...)
	require.NoError(e.t, err)
	ret, err := e.a.EvmKeeper.CallEVM(e.ctx, from, &to, nil, input)
	require.NoError(e.t, err, method)
	out, err := contract.Unpack(method, ret)
	require.NoError(e.t, err, method)
	return out
}

func (e *repair17Env) view(to common.Address, contract abi.ABI, method string, args ...interface{}) []interface{} {
	e.t.Helper()
	input, err := contract.Pack(method, args...)
	require.NoError(e.t, err)
	ret, err := e.a.EvmKeeper.StaticCallEVM(e.ctx, keeper.CustodyAddress(), &to, input)
	require.NoError(e.t, err, method)
	out, err := contract.Unpack(method, ret)
	require.NoError(e.t, err, method)
	return out
}

func (e *repair17Env) approveAndDeposit() {
	e.t.Helper()
	native := *artifacts.GetParsedABI("native")
	for _, rail := range []string{types.PECORVaultAddress, types.PECORAddress, types.PECOROrdersAddress} {
		e.call(e.user, e.stable, native, "approve", common.HexToAddress(rail), big.NewInt(500_000_000_000))
	}
	for _, token := range []common.Address{e.stable, e.native} {
		e.call(e.user, common.HexToAddress(types.PECORVaultAddress), e.vault, "deposit", token, big.NewInt(100_000_000_000))
	}
}

func (e *repair17Env) restart() {
	e.t.Helper()
	contracts := e.a.EvmKeeper.CustomPrecompiles(e.ctx)
	e.a.SpotKeeper = nil
	e.ctx = e.reopen(e.ctx)
	e.a.SpotKeeper = keeper.NewKeeper(e.a.GetKey(types.StoreKey), e.a.BankKeeper, &e.a.EvmKeeper, e.a.PricefeedKeeper)
	// Rebuild the production rails so their consumers hold the reopened keeper.
	versioned := make(map[common.Address]putils.VersionedPrecompiles, len(contracts))
	for address, contract := range contracts {
		versioned[address] = putils.VersionedPrecompiles{app.LatestUpgrade: contract}
	}
	spot, err := spotprecompile.GetVersioned(app.LatestUpgrade, app.NewPrecompileKeepers(e.a))
	require.NoError(e.t, err)
	for address, contract := range spot {
		versioned[address] = contract
	}
	e.a.EvmKeeper.SetCustomPrecompiles(versioned, app.LatestUpgrade)
}

func repair17RequireGenesisEqual(t *testing.T, expected, actual *types.GenesisState) {
	t.Helper()
	want, err := json.Marshal(expected)
	require.NoError(t, err)
	got, err := json.Marshal(actual)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
}

func TestRepair_17_listingSharePointer(t *testing.T) {
	e := newRepair17Env(t)
	before := len(e.ctx.EVMEventManager().Events())
	require.NoError(t, e.govern(e.listing()))
	vault := common.HexToAddress(types.PECORVaultAddress)
	require.Equal(t, []common.Address{e.stable, e.native}, e.view(vault, e.vault, "getRegisteredTokens")[0])
	shareABI := *artifacts.GetParsedABI("native")
	for _, token := range []common.Address{e.stable, e.native} {
		denom := e.a.SpotKeeper.ShareDenom(token)
		pointer, _, ok := e.a.EvmKeeper.GetERC20NativePointer(e.ctx, denom)
		require.True(t, ok)
		require.NotEqual(t, common.Address{}, pointer)
		require.Equal(t, pointer, e.view(vault, e.vault, "shareTokens", token)[0])
		metadata, ok := e.a.BankKeeper.GetDenomMetaData(e.ctx, denom)
		require.True(t, ok)
		require.NoError(t, metadata.Validate())
		require.Equal(t, uint32(6), metadata.DenomUnits[1].Exponent)
		require.Equal(t, uint8(6), e.view(pointer, shareABI, "decimals")[0])
		var created int
		for _, log := range logsSince(e.ctx, before) {
			if log.Topics[0] == e.vault.Events["ShareTokenCreated"].ID && log.Topics[1] == common.BytesToHash(token.Bytes()) {
				requireLog(t, log, vault, []common.Hash{e.vault.Events["ShareTokenCreated"].ID,
					common.BytesToHash(token.Bytes()), common.BytesToHash(pointer.Bytes())})
				created++
			}
		}
		require.Equal(t, 1, created)
	}
	e.approveAndDeposit()
	want := e.a.SpotKeeper.ExportGenesis(e.ctx)
	e.restart()
	repair17RequireGenesisEqual(t, want, e.a.SpotKeeper.ExportGenesis(e.ctx))
	for _, token := range []common.Address{e.stable, e.native} {
		denom := e.a.SpotKeeper.ShareDenom(token)
		pointer := e.view(vault, e.vault, "shareTokens", token)[0].(common.Address)
		require.Zero(t, big.NewInt(99_999_999_000).Cmp(e.view(pointer, shareABI, "balanceOf", e.user)[0].(*big.Int)))
		require.Zero(t, big.NewInt(100_000_000_000).Cmp(e.view(pointer, shareABI, "totalSupply")[0].(*big.Int)))
		require.Zero(t, e.a.BankKeeper.GetSupply(e.ctx, denom).Amount.BigInt().Cmp(e.view(pointer, shareABI, "totalSupply")[0].(*big.Int)))
		paid := e.call(e.user, vault, e.vault, "withdraw", token, big.NewInt(10_000_000_000))
		require.Zero(t, big.NewInt(10_000_000_000).Cmp(paid[0].(*big.Int)))
		require.Zero(t, big.NewInt(89_999_999_000).Cmp(e.view(pointer, shareABI, "balanceOf", e.user)[0].(*big.Int)))
		require.Zero(t, big.NewInt(90_000_000_000).Cmp(e.view(pointer, shareABI, "totalSupply")[0].(*big.Int)))
	}
	e.restart()
	for _, token := range []common.Address{e.stable, e.native} {
		pointer := e.view(vault, e.vault, "shareTokens", token)[0].(common.Address)
		require.Zero(t, big.NewInt(90_000_000_000).Cmp(e.view(pointer, shareABI, "totalSupply")[0].(*big.Int)))
		book, err := e.a.SpotKeeper.GetBook(e.ctx, token)
		require.NoError(t, err)
		require.Equal(t, "90000000000", book.Reserves.String())
	}
}

func TestRepair_17_listingRollback(t *testing.T) {
	e := newRepair17Env(t)
	bad := e.deploy(" ")
	params := e.listing()
	params.Tokens[0].Address = bad
	params.Tokens[0], params.Tokens[1] = params.Tokens[1], params.Tokens[0]
	before := e.a.SpotKeeper.ExportGenesis(e.ctx)
	logs := len(e.ctx.EVMEventManager().Events())
	events := len(e.ctx.EventManager().Events())
	require.Error(t, e.govern(params))
	repair17RequireGenesisEqual(t, before, e.a.SpotKeeper.ExportGenesis(e.ctx))
	require.Empty(t, logsSince(e.ctx, logs))
	require.Len(t, e.ctx.EventManager().Events(), events)
	for _, token := range params.Tokens {
		denom := e.a.SpotKeeper.ShareDenom(token.Address)
		_, _, exists := e.a.EvmKeeper.GetERC20NativePointer(e.ctx, denom)
		require.False(t, exists)
		_, exists = e.a.BankKeeper.GetDenomMetaData(e.ctx, denom)
		require.False(t, exists)
	}
}

func TestRepair_17_retainedTokenDecimals(t *testing.T) {
	e := newRepair17Env(t)
	require.NoError(t, e.govern(e.listing()))
	e.approveAndDeposit()
	params, err := e.a.SpotKeeper.GetParams(e.ctx)
	require.NoError(t, err)
	want := e.a.SpotKeeper.ExportGenesis(e.ctx)
	quote := e.a.SpotKeeper.Quote(e.ctx, e.stable, e.native, sdk.NewInt(1_000_000), true)
	require.True(t, quote.SufficientLiquidity)
	require.Zero(t, big.NewInt(2_000_000).Cmp(quote.GrossAmountOut))
	for _, tc := range []struct {
		name     string
		index    int
		decimals uint32
		kind     types.TokenKind
	}{
		{"funded ERC20 decimals", 0, 18, types.TokenKindERC20},
		{"zero is not rediscovery", 0, 0, types.TokenKindERC20},
		{"funded native decimals", 1, 18, types.TokenKindNative},
		{"ERC20 custody kind", 0, 6, types.TokenKindNative},
		{"native custody kind", 1, 6, types.TokenKindERC20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := params
			changed.Tokens = append([]types.TokenParams(nil), params.Tokens...)
			changed.Tokens[tc.index].Decimals, changed.Tokens[tc.index].Kind = tc.decimals, tc.kind
			logs := len(e.ctx.EVMEventManager().Events())
			require.Error(t, e.govern(changed))
			require.Empty(t, logsSince(e.ctx, logs))
			repair17RequireGenesisEqual(t, want, e.a.SpotKeeper.ExportGenesis(e.ctx))
			require.Equal(t, quote, e.a.SpotKeeper.Quote(e.ctx, e.stable, e.native, sdk.NewInt(1_000_000), true))
		})
	}
	// Re-read physical native metadata even when governance leaves its units unchanged.
	metadata, ok := e.a.BankKeeper.GetDenomMetaData(e.ctx, "uhpx")
	require.True(t, ok)
	drifted := metadata
	drifted.DenomUnits = []*banktypes.DenomUnit{{Denom: "uhpx"}, {Denom: "PAX", Exponent: 18}}
	e.a.BankKeeper.SetDenomMetaData(e.ctx, drifted)
	require.ErrorIs(t, e.govern(params), types.ErrUnsupportedToken)
	repair17RequireGenesisEqual(t, want, e.a.SpotKeeper.ExportGenesis(e.ctx))
	e.a.BankKeeper.SetDenomMetaData(e.ctx, metadata)
	book, err := e.a.SpotKeeper.GetBook(e.ctx, e.stable)
	require.NoError(t, err)
	wrongBook := book
	wrongBook.Decimals = 18
	require.NoError(t, e.a.SpotKeeper.SetBook(e.ctx, wrongBook))
	require.ErrorIs(t, e.govern(params), types.ErrUnsupportedToken)
	require.NoError(t, e.a.SpotKeeper.SetBook(e.ctx, book))
	params.Tokens[0].MaxUtilizationBps = 8500
	require.NoError(t, e.govern(params))
	for _, token := range []common.Address{e.stable, e.native} {
		info := e.view(common.HexToAddress(types.PECORVaultAddress), e.vault, "getTokenInfo", token)
		require.Equal(t, uint8(6), info[2])
		pointer := e.view(common.HexToAddress(types.PECORVaultAddress), e.vault, "shareTokens", token)[0].(common.Address)
		require.Equal(t, uint8(6), e.view(pointer, *artifacts.GetParsedABI("native"), "decimals")[0])
	}
	before := e.a.BankKeeper.GetBalance(e.ctx, e.a.EvmKeeper.GetPaxAddressOrDefault(e.ctx, e.user), "uhpx").Amount
	result, err := e.a.SpotKeeper.SwapExactIn(e.ctx, e.user, e.stable, e.native, big.NewInt(1_000_000), quote.AmountOut, new(big.Int))
	require.NoError(t, err)
	require.Zero(t, quote.AmountOut.Cmp(result.AmountOut))
	after := e.a.BankKeeper.GetBalance(e.ctx, e.a.EvmKeeper.GetPaxAddressOrDefault(e.ctx, e.user), "uhpx").Amount
	require.Zero(t, result.AmountOut.Cmp(after.Sub(before).BigInt()))
}

func (e *repair17Env) importSpot() {
	e.t.Helper()
	gs := e.a.SpotKeeper.ExportGenesis(e.ctx)
	raw, err := json.Marshal(gs)
	require.NoError(e.t, err)
	var decoded types.GenesisState
	require.NoError(e.t, json.Unmarshal(raw, &decoded))
	require.NoError(e.t, decoded.Validate())
	store := e.ctx.KVStore(e.a.GetKey(types.StoreKey))
	keys := proofSpotSnapshot(e.t, store)
	for key := range keys {
		store.Delete([]byte(key))
	}
	e.a.SpotKeeper.InitGenesis(e.ctx, decoded)
	repair17RequireGenesisEqual(e.t, gs, e.a.SpotKeeper.ExportGenesis(e.ctx))
}

func TestRepair_17_expiryHeadStarvation(t *testing.T) {
	e := newRepair17Env(t)
	params := e.listing()
	params.MaxExpiriesPerBlock = 3
	require.NoError(t, e.govern(params))
	e.approveAndDeposit()
	var failing, healthy []uint64
	for i := 0; i < 7; i++ {
		kind := types.OrderTypeLimitBuy
		if i >= 3 {
			kind = types.OrderTypeLimitSell
		}
		id, _, err := e.a.SpotKeeper.PlaceLimit(e.ctx, e.user, e.stable, e.native, sdk.NewInt(100_000_000),
			limitPrice(600), 1001, kind)
		require.NoError(t, err)
		if i < 3 {
			failing = append(failing, id)
		} else {
			healthy = append(healthy, id)
		}
	}
	future, _, err := e.a.SpotKeeper.PlaceLimit(e.ctx, e.user, e.stable, e.native, sdk.NewInt(100_000_000),
		limitPrice(600), 2000, types.OrderTypeLimitSell)
	require.NoError(t, err)
	params, err = e.a.SpotKeeper.GetParams(e.ctx)
	require.NoError(t, err)
	params.OrdersPaused = true
	require.NoError(t, e.govern(params))
	// A real ERC20 transfer removes this token's physical custody; its
	// refunds now revert for insufficient balance while native refunds work.
	custody := e.a.SpotKeeper.CustodyEVMAddress(e.ctx)
	erc20 := *artifacts.GetParsedABI("native")
	balance := e.view(e.stable, erc20, "balanceOf", custody)[0].(*big.Int)
	e.call(custody, e.stable, erc20, "transfer", shareAddress("repair17 loss recipient"), balance)
	require.Zero(t, e.view(e.stable, erc20, "balanceOf", custody)[0].(*big.Int).Sign())
	before := e.a.BankKeeper.GetBalance(e.ctx, e.a.EvmKeeper.GetPaxAddressOrDefault(e.ctx, e.user), "uhpx").Amount
	for block, expired := range []uint64{0, 3, 1, 0} {
		e.ctx = e.ctx.WithBlockHeight(11 + int64(block)).WithBlockTime(time.Unix(1001+int64(block), 0).UTC())
		e.price()
		work, err := e.a.SpotKeeper.ExecuteResting(e.ctx)
		require.NoError(t, err)
		require.LessOrEqual(t, work.ExpiriesExamined, uint64(3))
		require.Equal(t, expired, work.Expired)
		require.Zero(t, work.FillAttempts)
		for _, id := range failing {
			order, err := e.a.SpotKeeper.GetLimitOrder(e.ctx, id)
			require.NoError(t, err)
			require.Equal(t, types.OrderStatusPending, order.Status)
			backoff, err := e.a.SpotKeeper.FillBackoff(e.ctx, id)
			require.NoError(t, err)
			require.Positive(t, backoff.Failures)
		}
		if block == 0 {
			require.Equal(t, uint64(3), work.ExpiriesExamined)
			exported := e.a.SpotKeeper.ExportGenesis(e.ctx)
			require.Equal(t, healthy[0], exported.Expiries[0].OrderID)
			cursor := append(limitID(1001), limitID(failing[2])...)
			require.Equal(t, cursor, e.ctx.KVStore(e.a.GetKey(types.StoreKey)).Get([]byte{0x12, 0x02}))
			e.restart()
			require.Equal(t, cursor, e.ctx.KVStore(e.a.GetKey(types.StoreKey)).Get([]byte{0x12, 0x02}))
			repair17RequireGenesisEqual(t, exported, e.a.SpotKeeper.ExportGenesis(e.ctx))
		}
		// Import before every following block: the full leading failure
		// window must not regain its original priority at each import.
		e.importSpot()
	}
	for _, id := range healthy {
		order, err := e.a.SpotKeeper.GetLimitOrder(e.ctx, id)
		require.NoError(t, err)
		require.Equal(t, types.OrderStatusExpired, order.Status)
	}
	order, err := e.a.SpotKeeper.GetLimitOrder(e.ctx, future)
	require.NoError(t, err)
	require.Equal(t, types.OrderStatusPending, order.Status)
	after := e.a.BankKeeper.GetBalance(e.ctx, e.a.EvmKeeper.GetPaxAddressOrDefault(e.ctx, e.user), "uhpx").Amount
	require.Equal(t, "400000000", after.Sub(before).String())
	escrow, err := e.a.SpotKeeper.GetEscrow(e.ctx, e.stable)
	require.NoError(t, err)
	require.Equal(t, "300000000", escrow.String())
	escrow, err = e.a.SpotKeeper.GetEscrow(e.ctx, e.native)
	require.NoError(t, err)
	require.Equal(t, "100000000", escrow.String())
}
