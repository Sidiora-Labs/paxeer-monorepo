package keeper_test

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"encoding/binary"
	"encoding/json"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachekv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"math/big"
)

// repair14NowMs is the block time of every repair14 test; the pricefeed
// aggregate is one second older, so the price is fresh.
const repair14NowMs int64 = 1_800_000_000_000

var (
	repair14TraderA = common.HexToAddress("0x00000000000000000000000000000000000A11CE")
	repair14MakerB  = common.HexToAddress("0x00000000000000000000000000000000000000B2")
	repair14BidderC = common.HexToAddress("0x00000000000000000000000000000000000000C3")
	repair14KeeperK = common.HexToAddress("0x00000000000000000000000000000000000000D4")
)

// repair14Keepers are the application's precompile keepers plus the perps
// perpskeeper.
type repair14Keepers struct {
	utils.Keepers
	perps *perpskeeper.Keeper
}

func (k repair14Keepers) PerpsK() *perpskeeper.Keeper { return k.perps }

// repair14Env is the perps precompile over the real perps keeper, the real
// pricefeed keeper and a real EVM, with BTC (market 0, tick 0.1, lot 0.00001)
// listed, a fresh price and mark of 60,000 USD and the USDL ERC20 runtime at
// Params.CollateralToken.
type repair14Env struct {
	t           *testing.T
	k           *perpskeeper.Keeper
	perpsKey    sdk.StoreKey
	stateDB     *state.DBImpl
	baseStore   storetypes.MultiStore
	receiptLogs []*ethtypes.Log
	evm         *vm.EVM
	precompile  *pcommon.Precompile
	perpsABI    abi.ABI
	erc20ABI    abi.ABI
	usdl        common.Address
	askID       uint256.Int
}

func repair14E18(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(1_000_000_000_000_000_000))
}

func repair14Word(t *testing.T, decimal string) uint256.Int {
	t.Helper()
	value, err := uint256.FromDecimal(decimal)
	require.NoError(t, err)
	return *value
}

func repair14Market() perpstypes.Market {
	return perpstypes.Market{
		ID: 0, Symbol: "BTC", Name: "BTC", OracleSymbol: "BTC", SzDecimals: 5, PxDecimals: 1, MaxLeverage: 1000,
		TierClass: perpstypes.TierClassA, ImpactNotionalUsd: sdk.NewIntWithDecimal(20_000, 18),
		OiCapUsd: sdk.NewIntWithDecimal(500_000_000, 18), FundingCapPpmPerHour: 40_000, Enabled: true,
	}
}

// newRepair14Env seeds walletA USDL units and the same allowance to 0x1015
// for trader A, books 10,000 USDL for maker B, bidder C and keeper K in the
// store with the matching USDL held by the perps module, and rests B's ask of
// 1 BTC at 60,000 USD.
func newRepair14Env(t *testing.T, walletA int64) *repair14Env {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx([]byte{}).WithBlockHeight(12).WithBlockTime(time.UnixMilli(repair14NowMs).UTC())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	pfKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	pricefeed := pricefeedkeeper.NewKeeper(pfKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, pfKey)
	perpsKey := sdk.NewKVStoreKey(perpstypes.StoreKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, perpsKey)
	k := perpskeeper.NewKeeper(perpsKey, &app.EvmKeeper, pricefeed)
	require.NoError(t, k.SetMarket(ctx, k.Authority(), repair14Market()))
	aggregate, err := json.Marshal(pricefeedtypes.Aggregate{
		IndexPrice: sdk.NewIntWithDecimal(60_000, 18), MarkPrice: sdk.NewIntWithDecimal(60_000, 18),
		Basis: sdk.ZeroInt(), TimestampMs: uint64(repair14NowMs - 1_000), Height: ctx.BlockHeight(), Sources: 1,
	})
	require.NoError(t, err)
	ctx.KVStore(pfKey).Set(pricefeedtypes.AggregateKey(0), aggregate)

	erc20ABI := repair14ReadABI(t, "../../../tests/fixtures/example/contracts/erc20/ERC20.abi")
	perpsABI, err := perpsabi.ABI()
	require.NoError(t, err)
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	e := &repair14Env{t: t, k: k, perpsKey: perpsKey, perpsABI: perpsABI, erc20ABI: erc20ABI,
		usdl: params.CollateralToken}

	creation, err := os.ReadFile("../../../tests/fixtures/example/contracts/erc20/ERC20.bin")
	require.NoError(t, err)
	deployer := common.HexToAddress("0x00000000000000000000000000000000000DE910")
	nonce := app.EvmKeeper.GetNonce(ctx, deployer)
	_, err = app.EvmKeeper.CallEVM(ctx, deployer, nil, nil, common.FromHex(strings.TrimSpace(string(creation))))
	require.NoError(t, err)
	runtime := app.EvmKeeper.GetCode(ctx, crypto.CreateAddress(deployer, nonce))
	require.NotEmpty(t, runtime)
	app.EvmKeeper.SetCode(ctx, e.usdl, runtime)

	perpsAddress := common.HexToAddress(perps.PerpsAddress)
	e.seedBalance(ctx, repair14TraderA, walletA)
	e.seedAllowance(ctx, repair14TraderA, perpsAddress, walletA)
	const booked int64 = 10_000_000_000
	e.seedBalance(ctx, k.ModuleEVMAddress(), 3*booked)
	for _, account := range []common.Address{repair14MakerB, repair14BidderC, repair14KeeperK} {
		_, err := k.Deposit(ctx, account, *uint256.NewInt(uint64(booked)))
		require.NoError(t, err)
	}
	marketState, err := k.GetMarketState(ctx, 0)
	require.NoError(t, err)
	marketState.MarkPrice = *uint256.MustFromBig(repair14E18(60_000))
	k.SetMarketState(ctx, marketState)
	e.askID = e.rest(ctx, repair14MakerB, false, 60_000)

	precompile, err := perps.NewPrecompile(repair14Keepers{Keepers: app.GetPrecompileKeepers(), perps: k})
	require.NoError(t, err)
	e.precompile = precompile
	e.stateDB = state.NewDBImpl(ctx, &app.EvmKeeper, false)
	e.baseStore = ctx.MultiStore()
	blockCtx, err := app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	cfg := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(ctx))
	e.evm = vm.NewEVM(*blockCtx, e.stateDB, cfg, vm.Config{}, app.EvmKeeper.CustomPrecompiles(ctx))
	return e
}

func repair14ReadABI(t *testing.T, path string) abi.ABI {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	parsed, err := abi.JSON(file)
	require.NoError(t, err)
	return parsed
}

// seedBalance writes balanceOf[holder] (slot 3 of the ERC20) of the USDL
// runtime.
func (e *repair14Env) seedBalance(ctx sdk.Context, holder common.Address, amount int64) {
	slot := crypto.Keccak256Hash(common.LeftPadBytes(holder.Bytes(), 32), common.LeftPadBytes([]byte{3}, 32))
	testkeeper.EVMTestApp.EvmKeeper.SetState(ctx, e.usdl, slot, common.BigToHash(big.NewInt(amount)))
}

// seedAllowance writes allowance[owner][spender] (slot 4 of the ERC20) of the
// USDL runtime.
func (e *repair14Env) seedAllowance(ctx sdk.Context, owner, spender common.Address, amount int64) {
	inner := crypto.Keccak256(common.LeftPadBytes(owner.Bytes(), 32), common.LeftPadBytes([]byte{4}, 32))
	slot := crypto.Keccak256Hash(common.LeftPadBytes(spender.Bytes(), 32), inner)
	testkeeper.EVMTestApp.EvmKeeper.SetState(ctx, e.usdl, slot, common.BigToHash(big.NewInt(amount)))
}

func (e *repair14Env) ctx() sdk.Context { return e.stateDB.Ctx() }

// rest places a GTC limit order of 1 BTC at price whole USD for account.
func (e *repair14Env) rest(ctx sdk.Context, account common.Address, isLong bool, price int64) uint256.Int {
	e.t.Helper()
	result, err := e.k.PlaceOrder(ctx, account, perpskeeper.OrderParams{
		MarketID: 0, IsLong: isLong, OrderType: uint8(perpstypes.OrderTypeLimit),
		TimeInForce: uint8(perpstypes.TimeInForceGTC), SizeUsd: *uint256.MustFromBig(repair14E18(price)),
		Leverage: *uint256.MustFromBig(repair14E18(20)), LimitPrice: *uint256.MustFromBig(repair14E18(price)),
	})
	require.NoError(e.t, err)
	return result.OrderID
}

// setMark sets the BTC mark the keeper settles and sizes at.
func (e *repair14Env) setMark(price *big.Int) {
	e.t.Helper()
	marketState, err := e.k.GetMarketState(e.ctx(), 0)
	require.NoError(e.t, err)
	marketState.MarkPrice = *uint256.MustFromBig(price)
	e.k.SetMarketState(e.ctx(), marketState)
}

func (e *repair14Env) setPaused(paused bool) {
	e.t.Helper()
	params, err := e.k.GetParams(e.ctx())
	require.NoError(e.t, err)
	params.Paused = paused
	require.NoError(e.t, e.k.UpdateParams(e.ctx(), e.k.Authority(), params))
}

// cancelAsk cancels what remains of maker B's ask.
func (e *repair14Env) cancelAsk() {
	e.t.Helper()
	_, err := e.k.CancelOrder(e.ctx(), repair14MakerB, e.askID)
	require.NoError(e.t, err)
}

func (e *repair14Env) call(caller common.Address, method string, args ...interface{}) ([]byte, error) {
	e.t.Helper()
	input, err := e.perpsABI.Pack(method, args...)
	require.NoError(e.t, err)
	var (
		ret     []byte
		callErr error
	)
	snapshot := e.stateDB.Snapshot()
	require.NotPanics(e.t, func() {
		ret, callErr = e.precompile.Run(e.evm, caller, caller, input, big.NewInt(0), false, false, nil)
	})
	if callErr != nil {
		e.stateDB.RevertToSnapshot(snapshot)
	}
	// Each public call is a transaction. Flush its successful writes before
	// creating the next StateDB instead of accumulating snapshot cache layers.
	_, err = e.stateDB.Finalize()
	require.NoError(e.t, err)
	e.receiptLogs = append(e.receiptLogs, e.stateDB.GetAllLogs()...)
	ctx := e.ctx().WithMultiStore(e.baseStore)
	e.stateDB = state.NewDBImpl(ctx, &testkeeper.EVMTestApp.EvmKeeper, false)
	e.evm.StateDB = e.stateDB
	return ret, callErr
}

func (e *repair14Env) logs() []*ethtypes.Log {
	return append(append([]*ethtypes.Log(nil), e.receiptLogs...), e.stateDB.GetAllLogs()...)
}

func (e *repair14Env) mustCall(caller common.Address, method string, args ...interface{}) []byte {
	e.t.Helper()
	ret, err := e.call(caller, method, args...)
	require.NoError(e.t, err, "%s reverted with %x", method, ret)
	return ret
}

// requireRevert asserts the call reverted with exactly the selector of the
// parameterless custom error name.
func (e *repair14Env) requireRevert(name string, caller common.Address, method string, args ...interface{}) {
	e.t.Helper()
	ret, err := e.call(caller, method, args...)
	require.ErrorIs(e.t, err, vm.ErrExecutionReverted, method)
	require.Equal(e.t, crypto.Keccak256([]byte(name + "()"))[:4], ret, "%s revert data", method)
}

// open runs openPosition(0, usdl, 100 USDL, 10x, long) for A and returns the
// position id.
func (e *repair14Env) open() *big.Int {
	e.t.Helper()
	ret := e.mustCall(repair14TraderA, "openPosition", big.NewInt(0), e.usdl, big.NewInt(100_000_000),
		repair14E18(10), true)
	out, err := e.perpsABI.Methods["openPosition"].Outputs.Unpack(ret)
	require.NoError(e.t, err)
	require.Len(e.t, out, 1)
	return out[0].(*big.Int)
}

func (e *repair14Env) usdlBalance(holder common.Address) *big.Int {
	e.t.Helper()
	input, err := e.erc20ABI.Pack("balanceOf", holder)
	require.NoError(e.t, err)
	ret, _, err := e.evm.StaticCall(holder, e.usdl, input, 100_000)
	require.NoError(e.t, err)
	out, err := e.erc20ABI.Methods["balanceOf"].Outputs.Unpack(ret)
	require.NoError(e.t, err)
	return out[0].(*big.Int)
}

func (e *repair14Env) collateral(account common.Address) uint256.Int {
	e.t.Helper()
	held, err := e.k.GetAccount(e.ctx(), account)
	require.NoError(e.t, err)
	return held.Collateral
}

// position reads account's BTC position through the AccountMarketPosition
// index; ok is false when the account holds none.
func (e *repair14Env) position(account common.Address) (perpstypes.Position, bool) {
	e.t.Helper()
	store := e.ctx().KVStore(e.perpsKey)
	idBytes := store.Get(perpstypes.AccountMarketPositionKey(account, 0))
	if idBytes == nil {
		return perpstypes.Position{}, false
	}
	var id uint256.Int
	id.SetBytes(idBytes)
	bz := store.Get(perpstypes.PositionKey(id))
	require.NotNil(e.t, bz)
	var position perpstypes.Position
	require.NoError(e.t, position.Unmarshal(bz))
	return position, true
}

// perpsLogs returns the logs of event name emitted by 0x1015, each with its
// non-indexed fields by name.
func (e *repair14Env) perpsLogs(name string) ([]*ethtypes.Log, []map[string]interface{}) {
	e.t.Helper()
	event, ok := e.perpsABI.Events[name]
	require.True(e.t, ok, name)
	var (
		logs   []*ethtypes.Log
		fields []map[string]interface{}
	)
	for _, log := range e.logs() {
		if log.Address != common.HexToAddress(perps.PerpsAddress) || len(log.Topics) == 0 || log.Topics[0] != event.ID {
			continue
		}
		values := map[string]interface{}{}
		require.NoError(e.t, event.Inputs.NonIndexed().UnpackIntoMap(values, log.Data))
		logs = append(logs, log)
		fields = append(fields, values)
	}
	return logs, fields
}

func (e *repair14Env) quoteOpen(isLong bool) perpskeeper.OpenPositionQuote {
	e.t.Helper()
	beforeLogs := len(e.logs())
	ret := e.mustCall(repair14TraderA, "quoteOpenPosition", big.NewInt(0), e.usdl, big.NewInt(100_000_000), repair14E18(10), isLong)
	values, err := e.perpsABI.Methods["quoteOpenPosition"].Outputs.Unpack(ret)
	require.NoError(e.t, err)
	require.Len(e.t, values, 1)
	require.Len(e.t, e.logs(), beforeLogs)
	return *abi.ConvertType(values[0], new(perpskeeper.OpenPositionQuote)).(*perpskeeper.OpenPositionQuote)
}

func (e *repair14Env) assertQuotedOpen(quote perpskeeper.OpenPositionQuote, isLong bool, expectedBase string) {
	e.t.Helper()
	before, err := e.k.GetAccount(e.ctx(), repair14TraderA)
	if err != nil {
		require.ErrorIs(e.t, err, perpstypes.ErrNoAccount)
	}
	ret := e.mustCall(repair14TraderA, "openPosition", big.NewInt(0), e.usdl, big.NewInt(100_000_000), repair14E18(10), isLong)
	ids, err := e.perpsABI.Methods["openPosition"].Outputs.Unpack(ret)
	require.NoError(e.t, err)
	position, ok := e.position(repair14TraderA)
	require.True(e.t, ok)
	require.Equal(e.t, ids[0], position.ID.ToBig())
	require.Equal(e.t, repair14Word(e.t, expectedBase), position.SizeBase)
	require.Equal(e.t, isLong, position.IsLong)
	require.Equal(e.t, quote.EntryPrice, position.EntryPrice.ToBig())
	require.Equal(e.t, quote.Leverage, position.Leverage.ToBig())
	_, fills := e.perpsLogs("Fill")
	notional, fee := new(big.Int), new(big.Int)
	for _, fill := range fills {
		if fill["taker"] == repair14TraderA {
			notional.Add(notional, fill["sizeUsd"].(*big.Int))
			fee.Add(fee, fill["takerFeeUsd"].(*big.Int))
		}
	}
	require.Positive(e.t, notional.Sign())
	require.Equal(e.t, quote.SizeUsd, notional)
	require.Equal(e.t, quote.TradingFeeUsd, fee)
	after, err := e.k.GetAccount(e.ctx(), repair14TraderA)
	require.NoError(e.t, err)
	expectedCollateral := new(big.Int).Add(before.Collateral.ToBig(), big.NewInt(100_000_000))
	expectedCollateral.Sub(expectedCollateral, quote.TradingFee)
	require.Equal(e.t, expectedCollateral, after.Collateral.ToBig())
	// The persisted ADL score uses the final equity, including this fill's fee.
	equity, err := e.k.Equity(e.ctx(), repair14TraderA)
	require.NoError(e.t, err)
	rankNumerator := new(big.Int).Quo(new(big.Int).Mul(position.SizeBase.ToBig(), position.ReferencePrice.ToBig()), repair14E18(1))
	rankNumerator.Mul(rankNumerator, repair14E18(1))
	rankDenominator := equity.BigInt()
	if position.IsLong {
		rankNumerator.Mul(rankNumerator, position.ReferencePrice.ToBig())
		rankDenominator.Mul(rankDenominator, position.EntryPrice.ToBig())
	} else {
		rankNumerator.Mul(rankNumerator, position.EntryPrice.ToBig())
		rankDenominator.Mul(rankDenominator, position.ReferencePrice.ToBig())
	}
	require.Equal(e.t, rankNumerator.Quo(rankNumerator, rankDenominator), position.RankScore.ToBig())
	liquidation, err := e.k.LiquidationPrice(e.ctx(), position.ID)
	require.NoError(e.t, err)
	require.Equal(e.t, quote.LiquidationPrice, liquidation.ToBig())
	marketState, err := e.k.GetMarketState(e.ctx(), 0)
	require.NoError(e.t, err)
	funding := new(big.Int).Mul(position.SizeBase.ToBig(), marketState.IndexPrice.ToBig())
	funding.Mul(funding, marketState.HourlyRate.ToBig()).Mul(funding, big.NewInt(24))
	funding.Quo(funding, new(big.Int).Mul(repair14E18(1), repair14E18(1)))
	if isLong {
		funding.Neg(funding)
	}
	require.Equal(e.t, quote.EstimatedFunding24h.String(), funding.String())
}

func TestRepair_14_quote_open_base_rounding(t *testing.T) {
	e := newRepair14Env(t, 100_000_000)
	e.setMark(new(big.Int).Add(repair14E18(60_000), big.NewInt(1)))
	quote := e.quoteOpen(true)
	// An extra wei in the mark must preserve the exact 0.01650 BTC lot.
	e.assertQuotedOpen(quote, true, "16500000000000000")
}

func TestRepair_14_quote_short_self_bid(t *testing.T) {
	e := newRepair14Env(t, 5_000_000_000)
	e.cancelAsk()
	e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(4_000_000_000))
	e.rest(e.ctx(), repair14TraderA, true, 60_100)
	e.rest(e.ctx(), repair14BidderC, true, 60_000)
	quote := e.quoteOpen(false)
	// Both paths size at the visible 60,100 own bid, then cancel it through
	// self-trade prevention and execute against external liquidity at 60,000.
	e.assertQuotedOpen(quote, false, "16560000000000000")
	ownBid, err := e.k.GetOrder(e.ctx(), *uint256.NewInt(2))
	require.NoError(t, err)
	require.Equal(t, perpstypes.OrderStatusCancelled, ownBid.Status)
	_, fills := e.perpsLogs("Fill")
	require.Len(t, fills, 1)
	require.Equal(t, repair14BidderC, fills[0]["maker"])
}

func repair14ABCITransport(t *testing.T, produced error, root *sdkerrors.Error, detail string) {
	t.Helper()
	require.ErrorIs(t, produced, root)
	require.Contains(t, produced.Error(), detail)
	response := sdkerrors.ResponseDeliverTx(produced, 70, 50, false)
	require.Equal(t, root.Codespace(), response.Codespace)
	require.Equal(t, root.ABCICode(), response.Code)
	require.Contains(t, response.Log, root.Error())
	require.Contains(t, response.Log, detail)
	wire, err := response.Marshal()
	require.NoError(t, err)
	var received abci.ResponseDeliverTx
	require.NoError(t, received.Unmarshal(wire))
	require.Equal(t, response, received)
	require.ErrorIs(t, sdkerrors.ABCIError(received.Codespace, received.Code, received.Log), root)
	require.NotErrorIs(t, sdkerrors.ABCIError("other", received.Code, received.Log), root)
	require.NotErrorIs(t, sdkerrors.ABCIError(received.Codespace, 999999, received.Log), root)
	var malformed abci.ResponseDeliverTx
	require.Error(t, malformed.Unmarshal(wire[:len(wire)-1]))
	reason, err := abi.UnpackRevert(pcommon.LXPReason(produced))
	require.NoError(t, err)
	require.Equal(t, root.Error(), strings.SplitN(reason, ":", 2)[0])
	require.Contains(t, reason, produced.Error())
}

func TestRepair_14_abci_keeper_core(t *testing.T) {
	e := newRepair14Env(t, 0)
	_, err := e.k.GetMarket(e.ctx(), 987)
	repair14ABCITransport(t, err, perpstypes.ErrUnknownMarket, "987")
	e.requireRevert("PerpsUnknownMarket", repair14TraderA, "getMarket", big.NewInt(987))
	repair14ABCITransport(t, e.k.UpdateParams(e.ctx(), "wrong", perpstypes.DefaultParams()), perpstypes.ErrUnauthorized, "wrong")
	market := repair14Market()
	repair14ABCITransport(t, e.k.SetMarket(e.ctx(), "wrong", market), perpstypes.ErrUnauthorized, "wrong")
	market.ImpactNotionalUsd = sdk.ZeroInt()
	repair14ABCITransport(t, e.k.SetMarket(e.ctx(), e.k.Authority(), market), perpstypes.ErrInvalidParams, "impact")
	invalid := perpstypes.DefaultParams()
	invalid.CollateralToken = common.Address{}
	repair14ABCITransport(t, e.k.UpdateParams(e.ctx(), e.k.Authority(), invalid), perpstypes.ErrInvalidParams, "collateral")
}

func TestRepair_14_abci_governance_params(t *testing.T) {
	k, ctx, _ := newKeeper(t)
	server := perpskeeper.NewMsgServerImpl(k)
	_, err := server.UpdateParams(sdk.WrapSDKContext(ctx), &perpstypes.MsgUpdateParams{Authority: "wrong", Params: perpstypes.DefaultParams()})
	repair14ABCITransport(t, err, perpstypes.ErrUnauthorized, "wrong")
	changed := perpstypes.DefaultParams()
	changed.CollateralToken = common.HexToAddress("0x9911")
	_, err = server.UpdateParams(sdk.WrapSDKContext(ctx), &perpstypes.MsgUpdateParams{Authority: k.Authority(), Params: changed})
	repair14ABCITransport(t, err, perpstypes.ErrInvalidParams, "collateral token cannot change")
	require.Empty(t, ctx.EventManager().Events())
}

func TestRepair_14_abci_governance_market(t *testing.T) {
	k, ctx, _, _ := newMsgMarketKeeper(t)
	server := perpskeeper.NewMsgServerImpl(k)
	market := classEMarket(30, "BTC")
	_, err := server.SetMarket(sdk.WrapSDKContext(ctx), &perpstypes.MsgSetMarket{Authority: "wrong", Market: market})
	repair14ABCITransport(t, err, perpstypes.ErrUnauthorized, "wrong")
	_, err = server.SetMarket(sdk.WrapSDKContext(ctx), &perpstypes.MsgSetMarket{Authority: k.Authority(), Market: market})
	repair14ABCITransport(t, err, perpstypes.ErrUnknownMarket, "pricefeed market 30")
	require.NoError(t, k.SetMarket(ctx, k.Authority(), classEMarket(0, "BTC")))
	market = classEMarket(0, "BTC")
	market.Symbol = "CHANGED"
	_, err = server.SetMarket(sdk.WrapSDKContext(ctx), &perpstypes.MsgSetMarket{Authority: k.Authority(), Market: market})
	repair14ABCITransport(t, err, perpstypes.ErrInvalidParams, "identity cannot change")
	require.Empty(t, ctx.EventManager().Events())
}

func TestRepair_14_abci_margin(t *testing.T) {
	e := newRepair14Env(t, 0)
	req := perpskeeper.OpenRequest{Account: repair14MakerB, MarketID: 0, IsLong: true,
		NotionalUsd: sdk.NewIntWithDecimal(100_000_000, 18), Leverage: *uint256.MustFromBig(repair14E18(10)), TakerFeePpm: 450}
	repair14ABCITransport(t, e.k.CheckOpen(e.ctx(), req), perpstypes.ErrInsufficientMargin, "equity")
	req.NotionalUsd = sdk.NewIntWithDecimal(6000, 18)
	req.Leverage = *uint256.MustFromBig(repair14E18(1001))
	repair14ABCITransport(t, e.k.CheckOpen(e.ctx(), req), perpstypes.ErrLeverageTooHigh, "above")
	repair14ABCITransport(t, e.k.SetMarginMode(e.ctx(), repair14MakerB, 1), perpstypes.ErrCrossMarginOnly, "margin mode 1")
	e.requireRevert("PerpsCrossMarginOnly", repair14MakerB, "setMarginMode", uint8(1))
}

func TestRepair_14_abci_freshness(t *testing.T) {
	e := newRepair14Env(t, 0)
	stale := e.ctx().WithBlockTime(time.UnixMilli(repair14NowMs + 120_000))
	repair14ABCITransport(t, e.k.RequireFresh(stale, 0), perpstypes.ErrPriceStale, "market 0")
	raw := e.ctx().KVStore(e.perpsKey).Get(perpstypes.MarketStateKey(0))
	e.ctx().KVStore(e.perpsKey).Set(perpstypes.MarketStateKey(0), raw[:len(raw)-1])
	_, err := e.k.GetMarketState(e.ctx(), 0)
	repair14ABCITransport(t, err, perpstypes.ErrInvalidParams, "market state 0")
	e.requireRevert("PerpsInvalidParams", repair14TraderA, "getMarkPriceDetails", big.NewInt(0))
	e.ctx().KVStore(e.perpsKey).Set(perpstypes.MarketStateKey(0), raw)
}

func TestRepair_14_abci_positions(t *testing.T) {
	e := newRepair14Env(t, 200_000_000)
	_, err := e.k.ClosePosition(e.ctx(), repair14TraderA, *uint256.NewInt(987))
	repair14ABCITransport(t, err, perpstypes.ErrUnknownPosition, "987")
	e.requireRevert("PerpsUnknownPosition", repair14TraderA, "closePosition", big.NewInt(987))
	id := e.open()
	_, err = e.k.ClosePosition(e.ctx(), repair14MakerB, *uint256.MustFromBig(id))
	repair14ABCITransport(t, err, perpstypes.ErrNotPositionHolder, "position")
	e.requireRevert("PerpsNotPositionHolder", repair14MakerB, "closePosition", id)
	_, err = e.k.PartialClose(e.ctx(), repair14TraderA, *uint256.MustFromBig(id), *uint256.MustFromBig(repair14E18(100_000)))
	repair14ABCITransport(t, err, perpstypes.ErrCloseSizeExceedsPosition, "worth")
}

func TestRepair_14_abci_oi(t *testing.T) {
	e := newRepair14Env(t, 0)
	_, err := e.k.CheckOpenInterest(e.ctx(), perpskeeper.OpenInterestFill{MarketID: 0})
	repair14ABCITransport(t, err, perpstypes.ErrZeroAmount, "needs a size and a price")
	max := *new(uint256.Int).SetAllOne()
	_, err = e.k.CheckOpenInterest(e.ctx(), perpskeeper.OpenInterestFill{MarketID: 0, SizeBase: max, PriceUsd: *uint256.NewInt(1), BuyerOpensBase: max, SellerOpensBase: max})
	repair14ABCITransport(t, err, perpstypes.ErrOverflow, "opening size")
	size := *uint256.MustFromBig(repair14E18(10_000))
	_, err = e.k.CheckOpenInterest(e.ctx(), perpskeeper.OpenInterestFill{MarketID: 0, SizeBase: size,
		PriceUsd: *uint256.MustFromBig(repair14E18(60_000)), BuyerOpensBase: size, SellerOpensBase: size})
	repair14ABCITransport(t, err, perpstypes.ErrOpenInterestCap, "above cap")
}

func TestRepair_14_abci_liquidation(t *testing.T) {
	e := newRepair14Env(t, 200_000_000)
	_, err := e.k.Liquidate(e.ctx(), repair14KeeperK, *uint256.NewInt(987))
	repair14ABCITransport(t, err, perpstypes.ErrUnknownPosition, "987")
	id := e.open()
	_, err = e.k.Liquidate(e.ctx(), repair14KeeperK, *uint256.MustFromBig(id))
	repair14ABCITransport(t, err, perpstypes.ErrNotLiquidatable, "account")
	e.requireRevert("PerpsNotLiquidatable", repair14KeeperK, "liquidate", id)
}

func TestRepair_14_abci_fees(t *testing.T) {
	k, ctx, _ := newKeeper(t)
	_, err := k.TakerFee(ctx, uint256.Int{}, repair14TraderA, 0, *uint256.MustFromBig(repair14E18(6000)))
	repair14ABCITransport(t, err, perpstypes.ErrNoAccount, repair14TraderA.Hex())
	_, _, err = k.EnsureAccount(ctx, repair14TraderA)
	require.NoError(t, err)
	_, err = k.TakerFee(ctx, uint256.Int{}, repair14TraderA, 0, *uint256.MustFromBig(repair14E18(6000)))
	repair14ABCITransport(t, err, perpstypes.ErrInsufficientCollateral, "due")
	max := *new(uint256.Int).SetAllOne()
	require.NoError(t, k.RecordVolume(ctx, repair14TraderA, repair14MakerB, max))
	repair14ABCITransport(t, k.RecordVolume(ctx, repair14TraderA, repair14MakerB, *uint256.NewInt(1)), perpstypes.ErrOverflow, "USD sum")
}

func TestRepair_14_abci_insurance(t *testing.T) {
	e := newRepair14Env(t, 0)
	_, err := e.k.Backstop(e.ctx(), e.k.InsuranceAccountAddress())
	repair14ABCITransport(t, err, perpstypes.ErrNotLiquidatable, "never backstopped")
	_, err = e.k.Backstop(e.ctx(), repair14TraderA)
	repair14ABCITransport(t, err, perpstypes.ErrNotLiquidatable, "holds no position")
}

func TestRepair_14_abci_funding(t *testing.T) {
	e := newRepair14Env(t, 200_000_000)
	id := e.open()
	var position perpstypes.Position
	require.NoError(t, position.Unmarshal(e.ctx().KVStore(e.perpsKey).Get(perpstypes.PositionKey(*uint256.MustFromBig(id)))))
	other, err := e.k.GetAccount(e.ctx(), repair14MakerB)
	require.NoError(t, err)
	_, err = e.k.SettlePositionFunding(e.ctx(), &other, &position, uint256.Int{})
	repair14ABCITransport(t, err, perpstypes.ErrNotPositionHolder, "position")
	market, err := e.k.GetMarket(e.ctx(), 0)
	require.NoError(t, err)
	market.ImpactNotionalUsd = sdk.ZeroInt()
	state, err := e.k.GetMarketState(e.ctx(), 0)
	require.NoError(t, err)
	state.IndexPrice = state.MarkPrice
	e.k.SetMarketState(e.ctx(), state)
	_, err = e.k.SamplePremium(e.ctx(), market)
	repair14ABCITransport(t, err, perpstypes.ErrInvalidParams, "impact notional")
}

func TestRepair_14_abci_keeper_genesis(t *testing.T) {
	e := newRepair14Env(t, 0)
	exported := e.k.ExportGenesis(e.ctx())
	require.NoError(t, perpskeeper.ValidateGenesis(*exported))
	ask, err := e.k.GetOrder(e.ctx(), e.askID)
	require.NoError(t, err)
	exported.BookEntries = []perpstypes.BookEntry{{
		MarketID: ask.MarketID, Side: perpstypes.SideOf(ask.IsLong), Price: ask.LimitPrice,
		Sequence: ask.Sequence, OrderID: ask.ID,
	}}
	require.NoError(t, perpskeeper.ValidateGenesis(*exported))
	exported.BookEntries = append(exported.BookEntries, exported.BookEntries[0])
	repair14ABCITransport(t, perpskeeper.ValidateGenesis(*exported), perpstypes.ErrInvalidGenesis, "book entries")
	k, ctx, key := newKeeper(t)
	k.InitGenesis(ctx, *perpstypes.DefaultGenesis())
	raw := ctx.KVStore(key).Get(perpstypes.SequenceKey(perpstypes.SequenceOrderID))
	ctx.KVStore(key).Set(perpstypes.SequenceKey(perpstypes.SequenceOrderID), raw[:len(raw)-1])
	var produced error
	func() {
		defer func() {
			if r := recover(); r != nil {
				produced = r.(error)
			}
		}()
		k.ExportGenesis(ctx)
	}()
	repair14ABCITransport(t, produced, perpstypes.ErrInvalidGenesis, "counter")
}

type repair14TriggerOrder struct {
	MarketId         *big.Int
	IsLong           bool
	OrderType        uint8
	TimeInForce      uint8
	SizeUsd          *big.Int
	Leverage         *big.Int
	LimitPrice       *big.Int
	TriggerPrice     *big.Int
	AcceptablePrice  *big.Int
	CollateralAmount *big.Int
	ReduceOnly       bool
	TakeProfitPrice  *big.Int
	StopLossPrice    *big.Int
	Deadline         uint64
	Cloid            [16]byte
}

func repair14OpeningStop(kind perpstypes.OrderType, tif perpstypes.TimeInForce) repair14TriggerOrder {
	p := repair14TriggerOrder{MarketId: big.NewInt(0), IsLong: true, OrderType: uint8(kind), TimeInForce: uint8(tif),
		SizeUsd: repair14E18(6_000), Leverage: repair14E18(10), LimitPrice: new(big.Int), TriggerPrice: repair14E18(61_000),
		AcceptablePrice: new(big.Int), CollateralAmount: new(big.Int), TakeProfitPrice: new(big.Int), StopLossPrice: new(big.Int)}
	if kind == perpstypes.OrderTypeStopLimit {
		p.LimitPrice = repair14E18(61_100)
	}
	return p
}

func repair14PlacePublic(e *repair14Env, account common.Address, p repair14TriggerOrder) uint256.Int {
	e.t.Helper()
	ret := e.mustCall(account, "placeOrder", p)
	values, err := e.perpsABI.Methods["placeOrder"].Outputs.Unpack(ret)
	require.NoError(e.t, err)
	return *uint256.MustFromBig(values[0].(*big.Int))
}

func repair14Fresh(e *repair14Env) {
	e.t.Helper()
	_, err := e.k.UpdatePrices(e.ctx())
	require.NoError(e.t, err)
	current, err := e.k.GetMarketState(e.ctx(), 0)
	require.NoError(e.t, err)
	require.True(e.t, current.Fresh)
}

func repair14Evaluate(e *repair14Env, mark int64) []perpstypes.EventRecord {
	e.t.Helper()
	e.setMark(repair14E18(mark))
	market, err := e.k.GetMarket(e.ctx(), 0)
	require.NoError(e.t, err)
	visited, events, err := e.k.EvaluateTriggers(e.ctx(), market, *uint256.MustFromBig(repair14E18(mark)), 256)
	require.NoError(e.t, err)
	require.Positive(e.t, visited)
	return events
}

func repair14Reopen(e *repair14Env) {
	e.t.Helper()
	gs := e.k.ExportGenesis(e.ctx())
	require.NoError(e.t, perpstypes.ValidateGenesis(*gs))
	require.NoError(e.t, perpskeeper.ValidateGenesis(*gs))
	app := testkeeper.EVMTestApp
	ctx := e.ctx()
	pfKey := sdk.NewKVStoreKey("repair14_reopened_pricefeed")
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, pfKey)
	feed := pricefeedkeeper.NewKeeper(pfKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	for _, market := range gs.Markets {
		aggregate, err := json.Marshal(pricefeedtypes.Aggregate{IndexPrice: sdk.NewIntWithDecimal(60_000, 18), MarkPrice: sdk.NewIntWithDecimal(60_000, 18), Basis: sdk.ZeroInt(), TimestampMs: uint64(ctx.BlockTime().UnixMilli() - 1_000), Height: ctx.BlockHeight(), Sources: 1})
		require.NoError(e.t, err)
		ctx.KVStore(pfKey).Set(pricefeedtypes.AggregateKey(market.ID), aggregate)
	}
	key := sdk.NewKVStoreKey("repair14_reopened_perps")
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, key)
	reopened := perpskeeper.NewKeeper(key, &app.EvmKeeper, feed)
	require.NotPanics(e.t, func() { reopened.InitGenesis(ctx, *gs) })
	require.Equal(e.t, gs, reopened.ExportGenesis(ctx))
	pc, err := perps.NewPrecompile(repair14Keepers{Keepers: app.GetPrecompileKeepers(), perps: reopened})
	require.NoError(e.t, err)
	db := state.NewDBImpl(ctx, &app.EvmKeeper, false)
	block, err := app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(e.t, err)
	cfg := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(ctx))
	e.k, e.perpsKey, e.precompile, e.stateDB = reopened, key, pc, db
	e.baseStore = ctx.MultiStore()
	e.evm = vm.NewEVM(*block, db, cfg, vm.Config{}, app.EvmKeeper.CustomPrecompiles(ctx))
}

func TestRepair_14_trigger_live_margin(t *testing.T) {
	for _, kind := range []perpstypes.OrderType{perpstypes.OrderTypeStopMarket, perpstypes.OrderTypeStopLimit} {
		name := "stop_market"
		tif := perpstypes.TimeInForceIOC
		if kind == perpstypes.OrderTypeStopLimit {
			name = "stop_limit"
			tif = perpstypes.TimeInForceGTC
		}
		t.Run(name, func(t *testing.T) {
			e := newRepair14Env(t, 1_000_000_000)
			repair14Fresh(e)
			e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000_000))
			id := repair14PlacePublic(e, repair14TraderA, repair14OpeningStop(kind, tif))
			e.mustCall(repair14TraderA, "withdraw", e.usdl, big.NewInt(990_000_000))
			account, err := e.k.GetAccount(e.ctx(), repair14TraderA)
			require.NoError(t, err)
			require.Equal(t, uint64(10_000_000), account.Collateral.Uint64())
			events := repair14Evaluate(e, 61_000)
			order, err := e.k.GetOrder(e.ctx(), id)
			require.NoError(t, err)
			require.Equal(t, perpstypes.OrderStatusCancelled, order.Status)
			require.True(t, order.FilledBase.IsZero())
			_, exists := e.position(repair14TraderA)
			require.False(t, exists)
			after, err := e.k.GetAccount(e.ctx(), repair14TraderA)
			require.NoError(t, err)
			require.Equal(t, account.Collateral, after.Collateral)
			for _, event := range events {
				require.NotEqual(t, perpstypes.EventFill, event.Name)
				require.NotEqual(t, perpstypes.EventFeeCollected, event.Name)
			}
			repair14Reopen(e)
		})
	}
	for _, governed := range []string{"minimum_value", "leverage_tier"} {
		t.Run(governed, func(t *testing.T) {
			e := newRepair14Env(t, 1_000_000_000)
			repair14Fresh(e)
			e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000_000))
			p := repair14OpeningStop(perpstypes.OrderTypeStopMarket, perpstypes.TimeInForceIOC)
			if governed == "leverage_tier" {
				p.SizeUsd = repair14E18(300_000)
				p.Leverage = repair14E18(1_000)
			}
			id := repair14PlacePublic(e, repair14TraderA, p)
			if governed == "minimum_value" {
				params, err := e.k.GetParams(e.ctx())
				require.NoError(t, err)
				params.MinOrderValueUsd = sdk.NewIntWithDecimal(7_000, 18)
				require.NoError(t, e.k.UpdateParams(e.ctx(), e.k.Authority(), params))
			} else {
				market, err := e.k.GetMarket(e.ctx(), 0)
				require.NoError(t, err)
				market.TierClass = perpstypes.TierClassF
				require.NoError(t, e.k.SetMarket(e.ctx(), e.k.Authority(), market))
			}
			repair14Evaluate(e, 61_000)
			order, err := e.k.GetOrder(e.ctx(), id)
			require.NoError(t, err)
			require.Equal(t, perpstypes.OrderStatusCancelled, order.Status)
			require.True(t, order.FilledBase.IsZero())
		})
	}
}

func TestRepair_14_pending_reduce_only_flip(t *testing.T) {
	for _, takeProfit := range []bool{true, false} {
		name := "stop_loss"
		if takeProfit {
			name = "take_profit"
		}
		t.Run(name, func(t *testing.T) {
			e := newRepair14Env(t, 100_000_000)
			repair14Fresh(e)
			positionID := e.open()
			e.cancelAsk()
			bidID := e.rest(e.ctx(), repair14BidderC, true, 60_000)
			e.mustCall(repair14TraderA, "setPositionTpSl", positionID, repair14E18(61_000), repair14E18(59_000))
			positionWord := *uint256.MustFromBig(positionID)
			pair, err := e.k.GetAttachedTpSl(e.ctx(), positionWord)
			require.NoError(t, err)
			e.mustCall(repair14TraderA, "partialClose", positionID, repair14E18(300))
			remaining, ok := e.position(repair14TraderA)
			require.True(t, ok)
			require.Equal(t, repair14Word(t, "11500000000000000"), remaining.SizeBase)
			for _, id := range []uint256.Int{pair.TakeProfitOrderID, pair.StopLossOrderID} {
				order, err := e.k.GetOrder(e.ctx(), id)
				require.NoError(t, err)
				require.Equal(t, perpstypes.OrderStatusUntriggered, order.Status)
				require.True(t, order.ReduceOnly)
				require.False(t, order.IsLong)
				require.Equal(t, remaining.SizeBase, order.SizeBase)
			}
			repair14Reopen(e)
			recovered, ok := e.position(repair14TraderA)
			require.True(t, ok)
			require.Equal(t, remaining, recovered)
			e.mustCall(repair14BidderC, "cancelOrder", bidID.ToBig())
			price := int64(59_000)
			selected, other := pair.StopLossOrderID, pair.TakeProfitOrderID
			if takeProfit {
				price = 61_000
				selected, other = pair.TakeProfitOrderID, pair.StopLossOrderID
			}
			e.rest(e.ctx(), repair14BidderC, true, price)
			e.setMark(repair14E18(price))
			e.mustCall(repair14KeeperK, "executeOrder", selected.ToBig())
			_, ok = e.position(repair14TraderA)
			require.False(t, ok, "a reduce-only leg must never flip the position")
			filled, err := e.k.GetOrder(e.ctx(), selected)
			require.NoError(t, err)
			require.Equal(t, perpstypes.OrderStatusFilled, filled.Status)
			require.Equal(t, remaining.SizeBase, filled.FilledBase)
			cancelled, err := e.k.GetOrder(e.ctx(), other)
			require.NoError(t, err)
			require.Equal(t, perpstypes.OrderStatusCancelled, cancelled.Status)
			require.Nil(t, e.ctx().KVStore(e.perpsKey).Get(perpstypes.AttachedTpSlKey(positionWord)))
			repair14Reopen(e)
		})
	}
}

func TestRepair_14_cancelled_attachment_export(t *testing.T) {
	t.Run("public_cancellation", func(t *testing.T) {
		e := newRepair14Env(t, 100_000_000)
		repair14Fresh(e)
		positionID := e.open()
		e.mustCall(repair14TraderA, "setPositionTpSl", positionID, repair14E18(61_000), repair14E18(59_000))
		word := *uint256.MustFromBig(positionID)
		pair, err := e.k.GetAttachedTpSl(e.ctx(), word)
		require.NoError(t, err)
		e.mustCall(repair14TraderA, "cancelOrder", pair.TakeProfitOrderID.ToBig())
		after, err := e.k.GetAttachedTpSl(e.ctx(), word)
		require.NoError(t, err)
		require.True(t, after.TakeProfitOrderID.IsZero())
		require.Equal(t, pair.StopLossOrderID, after.StopLossOrderID)
		repair14Reopen(e)
		e.mustCall(repair14TraderA, "cancelOrder", pair.StopLossOrderID.ToBig())
		require.Nil(t, e.ctx().KVStore(e.perpsKey).Get(perpstypes.AttachedTpSlKey(word)))
		repair14Reopen(e)
	})
	t.Run("refused_activation", func(t *testing.T) {
		e := newRepair14Env(t, 100_000_000)
		repair14Fresh(e)
		positionID := e.open()
		e.cancelAsk()
		e.mustCall(repair14TraderA, "setPositionTpSl", positionID, repair14E18(61_000), new(big.Int))
		word := *uint256.MustFromBig(positionID)
		pair, err := e.k.GetAttachedTpSl(e.ctx(), word)
		require.NoError(t, err)
		repair14Evaluate(e, 61_000)
		order, err := e.k.GetOrder(e.ctx(), pair.TakeProfitOrderID)
		require.NoError(t, err)
		require.Equal(t, perpstypes.OrderStatusCancelled, order.Status)
		require.True(t, order.FilledBase.IsZero())
		require.Nil(t, e.ctx().KVStore(e.perpsKey).Get(perpstypes.AttachedTpSlKey(word)))
		repair14Reopen(e)
	})
}

func TestRepair_14_trigger_fok_becomes_ioc(t *testing.T) {
	e := newRepair14Env(t, 1_000_000_000)
	repair14Fresh(e)
	e.cancelAsk()
	e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000_000))
	ask := repair14OpeningStop(perpstypes.OrderTypeStopMarket, perpstypes.TimeInForceGTC)
	ask.IsLong = false
	ask.OrderType = uint8(perpstypes.OrderTypeLimit)
	ask.SizeUsd = repair14E18(610)
	ask.TriggerPrice = new(big.Int)
	ask.LimitPrice = repair14E18(61_000)
	makerID := repair14PlacePublic(e, repair14MakerB, ask)
	stopID := repair14PlacePublic(e, repair14TraderA, repair14OpeningStop(perpstypes.OrderTypeStopMarket, perpstypes.TimeInForceFOK))
	e.setMark(repair14E18(61_000))
	before := storeContents(t, e.ctx().KVStore(e.perpsKey))
	logsBefore := len(e.logs())
	walletBefore := e.usdlBalance(repair14TraderA)
	e.requireRevert("PerpsFokNotFilled", repair14KeeperK, "executeOrder", stopID.ToBig())
	require.Equal(t, before, storeContents(t, e.ctx().KVStore(e.perpsKey)))
	require.Len(t, e.logs(), logsBefore)
	require.Equal(t, walletBefore, e.usdlBalance(repair14TraderA))
	_, exists := e.position(repair14TraderA)
	require.False(t, exists)
	_, exists = e.position(repair14MakerB)
	require.False(t, exists)
	pending, err := e.k.GetOrder(e.ctx(), stopID)
	require.NoError(t, err)
	require.Equal(t, perpstypes.TimeInForceFOK, pending.TimeInForce)
	require.Equal(t, perpstypes.OrderStatusUntriggered, pending.Status)
	events := repair14Evaluate(e, 61_000)
	for _, event := range events {
		require.NotEqual(t, perpstypes.EventFill, event.Name)
		require.NotEqual(t, perpstypes.EventFeeCollected, event.Name)
	}
	cancelled, err := e.k.GetOrder(e.ctx(), stopID)
	require.NoError(t, err)
	require.Equal(t, perpstypes.OrderStatusCancelled, cancelled.Status)
	maker, err := e.k.GetOrder(e.ctx(), makerID)
	require.NoError(t, err)
	require.True(t, maker.FilledBase.IsZero())
	require.Equal(t, perpstypes.OrderStatusResting, maker.Status)
	repair14Reopen(e)
}

func TestRepair_14_open_fill_margin(t *testing.T) {
	for _, collateral := range []int64{602_700_000, 607_000_000} {
		t.Run(big.NewInt(collateral).String(), func(t *testing.T) {
			e := newRepair14Env(t, collateral)
			repair14Fresh(e)
			e.cancelAsk()
			e.rest(e.ctx(), repair14MakerB, false, 60_300)
			e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(collateral))
			p := repair14OpeningStop(perpstypes.OrderTypeMarket, perpstypes.TimeInForceIOC)
			p.TriggerPrice = new(big.Int)
			before := storeContents(t, e.ctx().KVStore(e.perpsKey))
			logs := len(e.logs())
			wallet := e.usdlBalance(repair14TraderA)
			e.requireRevert("PerpsInsufficientMargin", repair14TraderA, "placeOrder", p)
			require.Equal(t, before, storeContents(t, e.ctx().KVStore(e.perpsKey)))
			require.Len(t, e.logs(), logs)
			require.Equal(t, wallet, e.usdlBalance(repair14TraderA))
			_, ok := e.position(repair14TraderA)
			require.False(t, ok)
			_, ok = e.position(repair14MakerB)
			require.False(t, ok)
			// The larger balance passes executable-notional admission, but the
			// adverse fill still leaves equity below margin at mark after its fee.
			if collateral == 607_000_000 {
				_, err := e.k.PlaceOrder(e.ctx(), repair14TraderA, perpskeeper.OrderParams{MarketID: 0, IsLong: true,
					OrderType: uint8(perpstypes.OrderTypeMarket), TimeInForce: uint8(perpstypes.TimeInForceIOC),
					SizeUsd: *uint256.MustFromBig(repair14E18(6_000)), Leverage: *uint256.MustFromBig(repair14E18(10))})
				require.ErrorIs(t, err, perpstypes.ErrInsufficientMargin)
				require.Contains(t, err.Error(), "post-fee equity")
				require.Equal(t, before, storeContents(t, e.ctx().KVStore(e.perpsKey)))
			}
		})
	}
	t.Run("stale_maker", func(t *testing.T) {
		e := newRepair14Env(t, 1_000_000_000)
		repair14Fresh(e)
		e.cancelAsk()
		e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000_000))
		e.rest(e.ctx(), repair14BidderC, true, 60_000)
		e.mustCall(repair14BidderC, "withdraw", e.usdl, big.NewInt(7_000_000_000))
		e.setMark(repair14E18(59_900))
		p := repair14OpeningStop(perpstypes.OrderTypeMarket, perpstypes.TimeInForceIOC)
		p.IsLong = false
		p.SizeUsd = repair14E18(5_990)
		p.TriggerPrice = new(big.Int)
		before := storeContents(t, e.ctx().KVStore(e.perpsKey))
		logs := len(e.logs())
		e.requireRevert("PerpsInsufficientMargin", repair14TraderA, "placeOrder", p)
		require.Equal(t, before, storeContents(t, e.ctx().KVStore(e.perpsKey)))
		require.Len(t, e.logs(), logs)
		_, ok := e.position(repair14BidderC)
		require.False(t, ok)
	})
}

func TestRepair_14_expired_orders_still_fill(t *testing.T) {
	t.Run("public_makers_before_cleanup", func(t *testing.T) {
		e := newRepair14Env(t, 1_000_000_000)
		repair14Fresh(e)
		e.cancelAsk()
		e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000_000))
		params, err := e.k.GetParams(e.ctx())
		require.NoError(t, err)
		params.ExpiriesPerBlock = 1
		require.NoError(t, e.k.UpdateParams(e.ctx(), e.k.Authority(), params))
		var expired []uint256.Int
		for i := 0; i < 16; i++ {
			p := repair14OpeningStop(perpstypes.OrderTypeLimit, perpstypes.TimeInForceGTC)
			p.IsLong = false
			p.SizeUsd = repair14E18(600)
			p.LimitPrice = repair14E18(60_000)
			p.TriggerPrice = new(big.Int)
			p.Deadline = uint64(e.ctx().BlockTime().Unix())
			expired = append(expired, repair14PlacePublic(e, repair14MakerB, p))
		}
		live := e.rest(e.ctx(), repair14BidderC, false, 60_100)
		e.stateDB.WithCtx(e.ctx().WithBlockTime(e.ctx().BlockTime().Add(time.Second)))
		p := repair14OpeningStop(perpstypes.OrderTypeMarket, perpstypes.TimeInForceIOC)
		p.TriggerPrice = new(big.Int)
		repair14PlacePublic(e, repair14TraderA, p)
		for _, id := range expired {
			order, err := e.k.GetOrder(e.ctx(), id)
			require.NoError(t, err)
			require.Equal(t, perpstypes.OrderStatusExpired, order.Status)
			require.True(t, order.FilledBase.IsZero())
		}
		order, err := e.k.GetOrder(e.ctx(), live)
		require.NoError(t, err)
		require.Equal(t, repair14Word(t, "100000000000000000"), order.FilledBase)
		_, fills := e.perpsLogs("Fill")
		require.Len(t, fills, 1)
		require.Equal(t, repair14BidderC, fills[0]["maker"])
		_, ok := e.position(repair14MakerB)
		require.False(t, ok)
		repair14Reopen(e)
	})
	t.Run("expired_trigger_with_backlog", func(t *testing.T) {
		e := newRepair14Env(t, 1_000_000_000)
		repair14Fresh(e)
		e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000_000))
		for i := 0; i < 8; i++ {
			p := repair14OpeningStop(perpstypes.OrderTypeLimit, perpstypes.TimeInForceGTC)
			p.IsLong = true
			p.SizeUsd = repair14E18(580)
			p.LimitPrice = repair14E18(58_000)
			p.TriggerPrice = new(big.Int)
			p.Deadline = uint64(e.ctx().BlockTime().Unix())
			repair14PlacePublic(e, repair14BidderC, p)
		}
		p := repair14OpeningStop(perpstypes.OrderTypeStopMarket, perpstypes.TimeInForceIOC)
		p.Deadline = uint64(e.ctx().BlockTime().Unix())
		id := repair14PlacePublic(e, repair14TraderA, p)
		e.stateDB.WithCtx(e.ctx().WithBlockTime(e.ctx().BlockTime().Add(time.Second)))
		e.setMark(repair14E18(61_000))
		before := storeContents(t, e.ctx().KVStore(e.perpsKey))
		logs := len(e.logs())
		e.requireRevert("PerpsDeadlineExpired", repair14KeeperK, "executeOrder", id.ToBig())
		require.Equal(t, before, storeContents(t, e.ctx().KVStore(e.perpsKey)))
		require.Len(t, e.logs(), logs)
		events := repair14Evaluate(e, 61_000)
		for _, event := range events {
			require.NotEqual(t, perpstypes.EventFill, event.Name)
		}
		order, err := e.k.GetOrder(e.ctx(), id)
		require.NoError(t, err)
		require.Equal(t, perpstypes.OrderStatusCancelled, order.Status)
		require.True(t, order.FilledBase.IsZero())
		iterator := e.ctx().KVStore(e.perpsKey).Iterator(perpstypes.ExpiryPrefix, perpstypes.ExpiryKey(uint64(e.ctx().BlockTime().Unix()), uint256.Int{}))
		due := 0
		for ; iterator.Valid(); iterator.Next() {
			due++
		}
		iterator.Close()
		require.Equal(t, 8, due, "bounded expiry cleanup has not run")
		repair14Reopen(e)
	})
	t.Run("expired_admitted_taker", func(t *testing.T) {
		e := newRepair14Env(t, 1_000_000_000)
		repair14Fresh(e)
		e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000_000))
		p := repair14OpeningStop(perpstypes.OrderTypeLimit, perpstypes.TimeInForceGTC)
		p.LimitPrice = repair14E18(58_000)
		p.TriggerPrice = new(big.Int)
		p.Deadline = uint64(e.ctx().BlockTime().Unix())
		id := repair14PlacePublic(e, repair14TraderA, p)
		admitted, err := e.k.GetOrder(e.ctx(), id)
		require.NoError(t, err)
		market, err := e.k.GetMarket(e.ctx(), 0)
		require.NoError(t, err)
		before := storeContents(t, e.ctx().KVStore(e.perpsKey))
		result, err := e.k.Match(e.ctx().WithBlockTime(e.ctx().BlockTime().Add(time.Second)), market, admitted)
		require.ErrorIs(t, err, perpstypes.ErrDeadlineExpired)
		require.Empty(t, result.Fills)
		require.Empty(t, result.Events)
		require.Equal(t, before, storeContents(t, e.ctx().KVStore(e.perpsKey)))
	})
}

func repair14PausedRun(t *testing.T, openingCount int) uint64 {
	t.Helper()
	e := newEndblockEnv(t, 0)
	e.setMark(usdAmount(60_000))
	holder, seller, bidder, entrant := e.address(101), e.address(102), e.address(103), e.address(104)
	for _, address := range []common.Address{holder, seller, bidder, entrant} {
		_, err := e.k.Deposit(e.ctx, address, *uint256.NewInt(10_000_000_000))
		require.NoError(t, err)
	}
	place := func(address common.Address, params perpskeeper.OrderParams) perpskeeper.PlaceResult {
		result, err := e.k.PlaceOrder(e.ctx, address, params)
		require.NoError(t, err)
		return result
	}
	place(seller, perpskeeper.OrderParams{MarketID: 0, IsLong: false,
		OrderType: uint8(perpstypes.OrderTypeLimit), TimeInForce: uint8(perpstypes.TimeInForceGTC),
		SizeUsd: usdAmount(6000), Leverage: usdAmount(10), LimitPrice: usdAmount(60_000)})
	place(holder, perpskeeper.OrderParams{MarketID: 0, IsLong: true,
		OrderType: uint8(perpstypes.OrderTypeMarket), TimeInForce: uint8(perpstypes.TimeInForceIOC),
		SizeUsd: usdAmount(6000), Leverage: usdAmount(10), AcceptablePrice: usdAmount(60_000)})
	var positionID uint256.Int
	positionID.SetBytes32(e.ctx.KVStore(e.key).Get(perpstypes.AccountMarketPositionKey(holder, 0)))
	require.False(t, positionID.IsZero())
	openings := make([]uint256.Int, 0, openingCount)
	before := make(map[uint256.Int][]byte)
	for i := 0; i < openingCount; i++ {
		admitted := place(entrant, perpskeeper.OrderParams{MarketID: 0, IsLong: false,
			OrderType: uint8(perpstypes.OrderTypeStopMarket), TimeInForce: uint8(perpstypes.TimeInForceIOC),
			SizeUsd: usdAmount(600), Leverage: usdAmount(10), TriggerPrice: usdAmount(59_500), AcceptablePrice: usdAmount(59_000)})
		openings = append(openings, admitted.OrderID)
		before[admitted.OrderID] = append([]byte(nil), e.ctx.KVStore(e.key).Get(perpstypes.OrderKey(admitted.OrderID))...)
	}
	_, err := e.k.SetPositionTpSl(e.ctx, holder, positionID, uint256.Int{}, usdAmount(59_500))
	require.NoError(t, err)
	attached, err := e.k.GetAttachedTpSl(e.ctx, positionID)
	require.NoError(t, err)
	require.False(t, attached.StopLossOrderID.IsZero())
	place(bidder, perpskeeper.OrderParams{MarketID: 0, IsLong: true,
		OrderType: uint8(perpstypes.OrderTypeLimit), TimeInForce: uint8(perpstypes.TimeInForceGTC),
		SizeUsd: usdAmount(5900), Leverage: usdAmount(10), LimitPrice: usdAmount(59_000)})
	params, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	params.TriggersPerBlock = 1
	require.NoError(t, e.k.UpdateParams(e.ctx, e.k.Authority(), params))
	_, err = perpskeeper.NewMsgServerImpl(e.k).SetPaused(sdk.WrapSDKContext(e.ctx), &perpstypes.MsgSetPaused{Authority: e.k.Authority(), Paused: true})
	require.NoError(t, err)
	state, err := e.k.GetMarketState(e.ctx, 0)
	require.NoError(t, err)
	state.LastIndexHour = uint64(e.ctx.BlockTime().Unix()) / perpskeeper.SecondsPerHour
	state.LastSampleTime = uint64(e.ctx.BlockTime().Unix())
	e.k.SetMarketState(e.ctx, state)
	exported := e.k.ExportGenesis(e.ctx)
	require.NoError(t, perpstypes.ValidateGenesis(*exported))
	require.NoError(t, perpskeeper.ValidateGenesis(*exported))

	// Reopen the actual exported venue in a fresh store. InitGenesis must
	// reconstruct the protective index from the canonical pending orders.
	reopened := newEndblockEnv(t, 0)
	reopened.k.InitGenesis(reopened.ctx, *exported)
	e = nil
	protective := sdk.KVStorePrefixIterator(reopened.ctx.KVStore(reopened.key), []byte{0x19})
	require.True(t, protective.Valid())
	idBytes := attached.StopLossOrderID.Bytes32()
	require.Equal(t, idBytes[:], protective.Value())
	protective.Next()
	require.False(t, protective.Valid())
	protective.Close()
	reopened.setAggregate(0, 59_000, reopened.ctx.BlockTime().UnixMilli())
	reopened.ctx = reopened.ctx.WithEventManager(sdk.NewEventManager())
	meter := sdk.NewInfiniteGasMeterWithMultiplier(reopened.ctx)
	reopened.ctx = reopened.ctx.WithGasMeter(meter)
	start := meter.GasConsumed()
	records := reopened.k.EndBlock(reopened.ctx)
	consumed := meter.GasConsumed() - start
	require.Empty(t, reopened.errorEvents())
	triggered := eventsNamed(records, perpstypes.EventOrderTriggered)
	require.Len(t, triggered, 1)
	require.Equal(t, attached.StopLossOrderID.ToBig(), triggered[0].Args[0])
	filled, err := reopened.k.GetOrder(reopened.ctx, attached.StopLossOrderID)
	require.NoError(t, err)
	require.Equal(t, perpstypes.OrderStatusFilled, filled.Status)
	require.False(t, reopened.ctx.KVStore(reopened.key).Has(perpstypes.PositionKey(positionID)))
	for _, id := range openings {
		require.Equal(t, before[id], reopened.ctx.KVStore(reopened.key).Get(perpstypes.OrderKey(id)))
	}
	require.NoError(t, perpstypes.ValidateGenesis(*reopened.k.ExportGenesis(reopened.ctx)))
	return consumed
}

func TestRepair_14_paused_trigger_visit_bound(t *testing.T) {
	small := repair14PausedRun(t, 8)
	large := repair14PausedRun(t, 128)
	require.Equal(t, small, large, "paused EndBlock work must not grow with the crossed opening-trigger prefix")
}

type repair14Parent = storetypes.CacheMultiStore

// The overlay keeps the application's real EVM receipt store while moving the
// perps store onto a real reopenable database. Every cache branch writes both
// stores through the same SDK CacheMultiStore interface used by EndBlock.
type repair14PersistentOverlay struct {
	repair14Parent
	key sdk.StoreKey
	kv  storetypes.CacheKVStore
}

func (s *repair14PersistentOverlay) GetKVStore(key sdk.StoreKey) sdk.KVStore {
	if key.Name() == s.key.Name() {
		return s.kv
	}
	return s.repair14Parent.GetKVStore(key)
}
func (s *repair14PersistentOverlay) GetStore(key sdk.StoreKey) storetypes.Store {
	if key.Name() == s.key.Name() {
		return s.kv
	}
	return s.repair14Parent.GetStore(key)
}
func (s *repair14PersistentOverlay) CacheMultiStore() storetypes.CacheMultiStore {
	return &repair14PersistentOverlay{repair14Parent: s.repair14Parent.CacheMultiStore(), key: s.key, kv: cachekv.NewStore(s.kv, s.key, storetypes.DefaultCacheSizeLimit)}
}
func (s *repair14PersistentOverlay) Write() { s.kv.Write(); s.repair14Parent.Write() }

func repair14DurableEndblock(t *testing.T, e *endblockEnv) func() {
	t.Helper()
	path := t.TempDir()
	db, err := dbm.NewDB("perps", dbm.GoLevelDBBackend, path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	parent := e.ctx.MultiStore().CacheMultiStore()
	it := e.ctx.KVStore(e.key).Iterator(nil, nil)
	for ; it.Valid(); it.Next() {
		require.NoError(t, db.Set(it.Key(), it.Value()))
	}
	require.NoError(t, it.Close())
	var overlay *repair14PersistentOverlay
	install := func() {
		overlay = &repair14PersistentOverlay{repair14Parent: parent, key: e.key, kv: cachekv.NewStore(dbadapter.Store{DB: db}, e.key, storetypes.DefaultCacheSizeLimit)}
		e.ctx = e.ctx.WithMultiStore(overlay)
		app := testkeeper.EVMTestApp
		pf := pricefeedkeeper.NewKeeper(e.pfKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
		e.k = perpskeeper.NewKeeper(e.key, &app.EvmKeeper, pf)
	}
	install()
	return func() {
		overlay.kv.Write()
		e.k = nil
		require.NoError(t, db.Close())
		db, err = dbm.NewDB("perps", dbm.GoLevelDBBackend, path)
		require.NoError(t, err)
		install()
	}
}

func TestRepair_14_adl_previous_mark_endblock_recovery(t *testing.T) {
	e := newEndblockEnv(t, 0)
	restart := repair14DurableEndblock(t, e)
	e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
	require.NotEmpty(t, e.k.EndBlock(e.ctx))
	require.Empty(t, e.errorEvents())
	// A previously backstopped short is settled at 65,000; an already booked
	// uncovered deficit forces ADL after this block updates the live mark.
	fund := e.k.InsuranceAccountAddress()
	cp := e.address(3)
	e.account(fund, 0)
	e.account(cp, 12_000_000_000)
	e.addOpenInterest(usdAmount(1))
	e.position(5, fund, false, usdAmount(1), usdAmount(65_000))
	cpID := e.position(9, cp, true, usdAmount(1), usdAmount(54_000))
	// Fixture positions carry the actual rank producer before the restart.
	_, err := e.k.Settle(e.ctx, cp)
	require.NoError(t, err)
	fundAccount := e.readAccount(fund)
	fundAccount.DeficitUsd = usdAmount(2_000)
	e.ctx.KVStore(e.key).Set(perpstypes.AccountKey(fund), fundAccount.Marshal())
	e.ctx.KVStore(e.key).Set(perpstypes.VenueBooksKey, (perpstypes.VenueBooks{DeficitSumUsd: usdAmount(2_000)}).Marshal())
	restart()
	e.ctx = e.ctx.WithBlockHeight(e.ctx.BlockHeight() + 1).WithBlockTime(e.ctx.BlockTime().Add(time.Second)).WithEventManager(sdk.NewEventManager())
	e.setAggregate(0, 65_000, e.ctx.BlockTime().UnixMilli())
	records := e.k.EndBlock(e.ctx)
	repair14InsuranceScanRefusal(t, e)
	require.Equal(t, usdAmount(65_000), e.marketState(0).MarkPrice)
	var matches []perpstypes.EventRecord
	for _, record := range records {
		if record.Name == perpstypes.EventADLMatched {
			matches = append(matches, record)
		}
	}
	require.Len(t, matches, 1)
	require.Equal(t, cpID.ToBig(), matches[0].Args[1])
	require.Equal(t, insuranceUsdBig(60_000), matches[0].Args[4])
	require.Equal(t, *uint256.NewInt(16_000_000_000), e.readAccount(cp).Collateral)
	require.Equal(t, *uint256.NewInt(5_000_000_000), e.readAccount(fund).Collateral)
	remainingFund := e.readAccount(fund)
	require.True(t, remainingFund.DeficitUsd.IsZero())
}

func TestRepair_14_adl_visit_bound_endblock_recovery(t *testing.T) {
	e := newEndblockEnv(t, 0)
	restart := repair14DurableEndblock(t, e)
	e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
	require.NotEmpty(t, e.k.EndBlock(e.ctx))
	params, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	params.AdlPositionsPerBlock = 2
	require.NoError(t, e.k.UpdateParams(e.ctx, e.k.Authority(), params))
	fund := e.k.InsuranceAccountAddress()
	e.account(fund, 0)
	e.position(99, fund, true, usdAmount(7), usdAmount(60_000))
	e.addOpenInterest(usdAmount(7))
	for id := uint64(1); id <= 7; id++ {
		account := e.address(id)
		e.account(account, 20_000_000_000)
		entry := usdAmount(59_000)
		if id == 7 {
			entry = usdAmount(66_000)
		}
		e.position(id, account, false, usdAmount(1), entry)
		_, err = e.k.Settle(e.ctx, account)
		require.NoError(t, err)
	}
	fundAccount := e.readAccount(fund)
	fundAccount.DeficitUsd = usdAmount(2_000)
	e.ctx.KVStore(e.key).Set(perpstypes.AccountKey(fund), fundAccount.Marshal())
	e.ctx.KVStore(e.key).Set(perpstypes.VenueBooksKey, (perpstypes.VenueBooks{DeficitSumUsd: usdAmount(2_000)}).Marshal())
	for pass := 0; pass < 4; pass++ {
		restart()
		e.ctx = e.ctx.WithBlockHeight(e.ctx.BlockHeight() + 1).WithBlockTime(e.ctx.BlockTime().Add(time.Second)).WithEventManager(sdk.NewEventManager())
		e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
		records := e.k.EndBlock(e.ctx)
		repair14InsuranceScanRefusal(t, e)
		raw := e.ctx.KVStore(e.key).Get([]byte{0x18})
		require.Len(t, raw, 24)
		visits := binary.BigEndian.Uint64(raw[8:16])
		require.LessOrEqual(t, visits, uint64(2))
		matches := 0
		for _, record := range records {
			if record.Name == perpstypes.EventADLMatched {
				matches++
				require.Equal(t, uint256.NewInt(7).ToBig(), record.Args[1])
			}
		}
		if pass < 3 {
			require.Zero(t, matches)
			require.Equal(t, uint64(2), visits)
		} else {
			require.Equal(t, 1, matches)
		}
	}
}

func repair14InsuranceScanRefusal(t *testing.T, e *endblockEnv) {
	t.Helper()
	failures := e.errorEvents()
	require.Len(t, failures, 1)
	require.Equal(t, perpskeeper.StepLiquidation, failures[0][perpskeeper.AttributeStep])
	require.Contains(t, failures[0][perpskeeper.AttributeError], "insurance fund is never backstopped")
}

func TestRepair_14_settlement_rank_endblock_recovery(t *testing.T) {
	e := newEndblockEnv(t, 0)
	restart := repair14DurableEndblock(t, e)
	e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
	require.NotEmpty(t, e.k.EndBlock(e.ctx))
	require.Empty(t, e.errorEvents())
	first, second := e.address(1), e.address(2)
	fund := e.k.InsuranceAccountAddress()
	e.account(first, 10_000_000_000)
	e.account(second, 20_000_000_000)
	e.account(fund, 0)
	firstID := e.position(1, first, false, usdAmount(1), usdAmount(66_000))
	secondID := e.position(2, second, false, usdAmount(1), usdAmount(66_000))
	deficitID := e.position(99, fund, true, usdAmount(2), usdAmount(60_000))
	e.addOpenInterest(usdAmount(2))
	for _, account := range []perpstypes.Account{e.readAccount(first), e.readAccount(second)} {
		_, err := e.k.Settle(e.ctx, account.Address)
		require.NoError(t, err)
	}
	firstPosition, err := e.k.GetPosition(e.ctx, firstID)
	require.NoError(t, err)
	// The settled short's score is (66,000 / 60,000) * 60,000 / 16,000,
	// with the profit already included in the final 16,000 USD account value.
	require.Equal(t, *uint256.NewInt(4_125_000_000_000_000_000), firstPosition.RankScore)
	assertLeader := func(id uint256.Int) {
		t.Helper()
		ranked, rankErr := e.k.RankADL(e.ctx, deficitID, usdAmount(60_000), 2)
		require.NoError(t, rankErr)
		require.Len(t, ranked, 2)
		require.Equal(t, id, ranked[0].ID)
	}
	assertLeader(firstID)
	_, err = e.k.Deposit(e.ctx, first, *uint256.NewInt(20_000_000_000))
	require.NoError(t, err)
	assertLeader(secondID)
	_, err = e.k.Withdraw(e.ctx, first, *uint256.NewInt(20_000_000_000))
	require.NoError(t, err)
	assertLeader(firstID)
	fundAccount := e.readAccount(fund)
	fundAccount.DeficitUsd = usdAmount(2_000)
	e.ctx.KVStore(e.key).Set(perpstypes.AccountKey(fund), fundAccount.Marshal())
	e.ctx.KVStore(e.key).Set(perpstypes.VenueBooksKey, (perpstypes.VenueBooks{DeficitSumUsd: usdAmount(2_000)}).Marshal())
	restart()
	assertLeader(firstID)
	recovered, err := e.k.GetPosition(e.ctx, firstID)
	require.NoError(t, err)
	require.Equal(t, firstPosition.RankScore, recovered.RankScore)
	e.ctx = e.ctx.WithBlockHeight(e.ctx.BlockHeight() + 1).WithBlockTime(e.ctx.BlockTime().Add(time.Second)).WithEventManager(sdk.NewEventManager())
	e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
	records := e.k.EndBlock(e.ctx)
	repair14InsuranceScanRefusal(t, e)
	var matches []perpstypes.EventRecord
	for _, record := range records {
		if record.Name == perpstypes.EventADLMatched {
			matches = append(matches, record)
		}
	}
	require.NotEmpty(t, matches)
	require.Equal(t, firstID.ToBig(), matches[0].Args[1])
	require.Equal(t, insuranceUsdBig(60_000), matches[0].Args[4])
}
