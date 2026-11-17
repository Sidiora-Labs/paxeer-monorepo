package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	xevmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const gigaPerpsGas uint64 = 3_000_000

var gigaPerpsAddress = common.HexToAddress(perps.PerpsAddress)

type gigaPerpsOrder struct {
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

type gigaPerpsCall struct {
	kind, method string
	signer       int
	to           common.Address
	input        []byte
	value        *big.Int
	rejected     bool
	errorName    string
}

type gigaPerpsBlock struct {
	results  []*abci.ExecTxResult
	receipts []*evmtypes.Receipt
	messages []evmtypes.MsgEVMTransactionResponse
	hash     []byte
}

type gigaPerpsSequence struct {
	t              *testing.T
	ordinary, fast *app.TestWrapper
	abi, tokenABI  abi.ABI
	signers        []utils.TestAcct
	nonces         []uint64
	proposer       []byte
	timestamp      time.Time
	height         int64
	kinds          map[string]bool
	transactions   int
}

func TestGigaPerpsParity(t *testing.T) {
	previousOCC := app.EnableOCC
	previousSkipResultsHash := tmtypes.SkipLastResultsHashValidation.Load()
	app.EnableOCC = true
	t.Cleanup(func() {
		app.EnableOCC = previousOCC
		tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	})
	for _, absent := range []bool{false, true} {
		name := "stored_parameters"
		if absent {
			name = "default_parameters"
		}
		t.Run(name, func(t *testing.T) {
			s := newGigaPerpsSequence(t, absent)
			s.run()
			require.Len(t, s.kinds, 40)
			require.Equal(t, 94, s.transactions)
			t.Logf("%d kinds, %d transactions applied on each execution path", len(s.kinds), s.transactions)
		})
	}
}

func gigaPerpsUSD(whole int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(whole), big.NewInt(1_000_000_000_000_000_000))
}

func gigaPerpsWord(value *big.Int) uint256.Int {
	return *uint256.MustFromBig(value)
}

func gigaPerpsTokenSlot(holder common.Address, slot int64) common.Hash {
	return crypto.Keccak256Hash(common.LeftPadBytes(holder.Bytes(), 32), common.LeftPadBytes(big.NewInt(slot).Bytes(), 32))
}

func newGigaPerpsSequence(t *testing.T, absent bool) *gigaPerpsSequence {
	t.Helper()
	parsed, err := perpsabi.ABI()
	require.NoError(t, err)
	tokenABI, err := abi.JSON(strings.NewReader(`[
		{"type":"function","name":"approve","inputs":[{"type":"address"},{"type":"uint256"}],"outputs":[{"type":"bool"}]},
		{"type":"function","name":"balanceOf","inputs":[{"type":"address"}],"outputs":[{"type":"uint256"}]},
		{"type":"function","name":"allowance","inputs":[{"type":"address"},{"type":"address"}],"outputs":[{"type":"uint256"}]}
	]`))
	require.NoError(t, err)
	s := &gigaPerpsSequence{
		t: t, abi: parsed, tokenABI: tokenABI, timestamp: time.Unix(1_800_000_000, 0).UTC(),
		signers: utils.NewTestAccounts(4), nonces: make([]uint64, 4), kinds: make(map[string]bool),
		proposer: utils.NewTestAccounts(1)[0].PublicKey.Address(),
	}
	genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)
	raw, err := os.ReadFile("../precompiles/perps/testdata/usdl_runtime.hex")
	require.NoError(t, err)
	code := common.FromHex(strings.TrimSpace(string(raw)))
	require.NotEmpty(t, code)
	fixture, err := os.ReadFile("../modules/launchpad/keeper/testdata/v6120/usdl.json")
	require.NoError(t, err)
	var token struct{ Address, CodeHash string }
	require.NoError(t, json.Unmarshal(fixture, &token))
	usdl := common.HexToAddress(perpstypes.DefaultCollateralToken)
	require.Equal(t, usdl, common.HexToAddress(token.Address))
	require.Equal(t, common.HexToHash(token.CodeHash), crypto.Keccak256Hash(code))
	newApplication := func(giga bool) *app.TestWrapper {
		asyncWriteBuffer := 0
		a := app.SetupWithSc(t, true, true, app.TestAppOpts{
			UseSc: true, EnableGiga: true, EnableGigaOCC: true, ReceiptAsyncWriteBuffer: &asyncWriteBuffer,
		}, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(true)
			ba.SetConcurrencyWorkers(1)
		})
		t.Cleanup(func() { require.NoError(t, a.Close()) })
		_, err := a.InitChain(context.Background(), &abci.RequestInitChain{
			ChainId: chainsBlockChainID, Time: s.timestamp, InitialHeight: 1,
			ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
		})
		require.NoError(t, err)
		a.GigaExecutorEnabled = giga
		ctx := a.NewContext(false, tmproto.Header{Height: 1, ChainID: chainsBlockChainID,
			Time: s.timestamp, ProposerAddress: s.proposer})
		ctx = ctx.WithConsensusParams(a.GetConsensusParams(ctx))
		a.DistrKeeper.SetPreviousProposerConsAddr(ctx, sdk.ConsAddress(s.proposer))
		a.RefreshPrecompiles(ctx)
		require.Equal(t, app.LatestUpgrade, a.EvmKeeper.GetCustomPrecompilesVersions(ctx)[gigaPerpsAddress])
		require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), gigaPerpsAddress)
		a.EvmKeeper.SetCode(ctx, usdl, code)
		funding := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		for _, signer := range s.signers {
			a.EvmKeeper.SetAddressMapping(ctx, signer.AccountAddress, signer.EvmAddress)
			require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
			require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", signer.AccountAddress, funding))
			a.EvmKeeper.SetState(ctx, usdl, gigaPerpsTokenSlot(signer.EvmAddress, 0), common.BigToHash(big.NewInt(1_000_000_000_000)))
			require.Zero(t, a.EvmKeeper.GetNonce(ctx, signer.EvmAddress))
		}
		a.EvmKeeper.SetState(ctx, usdl, common.BigToHash(big.NewInt(2)), common.BigToHash(big.NewInt(4_000_000_000_000)))
		allowanceSlot := crypto.Keccak256Hash(common.LeftPadBytes(gigaPerpsAddress.Bytes(), 32),
			gigaPerpsTokenSlot(s.signers[1].EvmAddress, 1).Bytes())
		a.EvmKeeper.SetState(ctx, usdl, allowanceSlot, common.BigToHash(big.NewInt(5_000_000_000)))
		k := a.PerpsKeeper
		require.NoError(t, k.UpdateParams(ctx, k.Authority(), perpstypes.DefaultParams()))
		require.NoError(t, k.SetMarket(ctx, k.Authority(), perpstypes.Market{
			ID: 0, Symbol: "BTC", Name: "Bitcoin", OracleSymbol: "BTC", SzDecimals: 5, PxDecimals: 1,
			MaxLeverage: 1000, TierClass: perpstypes.TierClassA, ImpactNotionalUsd: sdk.NewIntWithDecimal(20_000, 18),
			OiCapUsd: sdk.NewIntWithDecimal(500_000_000, 18), FundingCapPpmPerHour: 40_000, Enabled: true,
		}))
		aggregate, err := json.Marshal(pricefeedtypes.Aggregate{
			IndexPrice: sdk.NewIntWithDecimal(60_000, 18), MarkPrice: sdk.NewIntWithDecimal(60_000, 18),
			Basis: sdk.ZeroInt(), TimestampMs: uint64(s.timestamp.UnixMilli()), Height: 1, Sources: 1,
		})
		require.NoError(t, err)
		ctx.KVStore(a.GetKey(pricefeedtypes.StoreKey)).Set(pricefeedtypes.AggregateKey(0), aggregate)
		_, err = k.UpdatePrices(ctx)
		require.NoError(t, err)
		for i := 2; i < 4; i++ {
			maker := s.signers[i].EvmAddress
			_, err := k.Deposit(ctx, maker, *uint256.NewInt(100_000_000_000))
			require.NoError(t, err)
			a.EvmKeeper.SetState(ctx, usdl, gigaPerpsTokenSlot(maker, 0), common.BigToHash(big.NewInt(900_000_000_000)))
			price := int64(60_100)
			if i == 3 {
				price = 59_900
			}
			_, err = k.PlaceOrder(ctx, maker, perpskeeper.OrderParams{
				MarketID: 0, IsLong: i == 3, OrderType: uint8(perpstypes.OrderTypeLimit), TimeInForce: uint8(perpstypes.TimeInForceGTC),
				SizeUsd: gigaPerpsWord(gigaPerpsUSD(200_000)), Leverage: gigaPerpsWord(gigaPerpsUSD(20)), LimitPrice: gigaPerpsWord(gigaPerpsUSD(price)),
			})
			require.NoError(t, err)
		}
		a.EvmKeeper.SetState(ctx, usdl, gigaPerpsTokenSlot(k.ModuleEVMAddress(), 0), common.BigToHash(big.NewInt(200_000_000_000)))
		a.EvmKeeper.SetParams(ctx, evmtypes.DefaultParams())
		params := xevmtypes.DefaultParams()
		for _, pair := range params.ParamSetPairs() {
			if absent {
				key := append([]byte(a.EvmKeeper.Paramstore.Name()+"/"), pair.Key...)
				ctx.KVStore(a.GetKey(paramtypes.StoreKey)).Delete(key)
			}
			require.Equal(t, !absent, a.EvmKeeper.Paramstore.Has(ctx, pair.Key), string(pair.Key))
			require.Equal(t, !absent, a.GigaEvmKeeper.Paramstore.Has(ctx, pair.Key), string(pair.Key))
		}
		if absent {
			ctx.KVStore(a.GetKey(perpstypes.StoreKey)).Delete(perpstypes.ParamsKey)
		}
		require.Equal(t, !absent, ctx.KVStore(a.GetKey(perpstypes.StoreKey)).Has(perpstypes.ParamsKey))
		paramsRead, err := k.GetParams(ctx)
		require.NoError(t, err)
		require.Equal(t, perpstypes.DefaultParams(), paramsRead)
		message, broken := perpskeeper.SolvencyInvariant(k)(ctx)
		require.False(t, broken, message)
		return &app.TestWrapper{App: a, Ctx: ctx}
	}
	s.ordinary, s.fast = newApplication(false), newApplication(true)
	require.NotNil(t, s.fast.App.GigaEvmKeeper.EvmoneVM)
	require.True(t, s.fast.App.GigaOCCEnabled)
	require.Equal(t, gigaStorageGasState(t, s.ordinary), gigaStorageGasState(t, s.fast), "initial state")
	return s
}

func (s *gigaPerpsSequence) call(kind, method string, args ...interface{}) gigaPerpsCall {
	s.t.Helper()
	resolved := ""
	for name, candidate := range s.abi.Methods {
		if candidate.RawName == method && len(candidate.Inputs) == len(args) {
			require.Empty(s.t, resolved, "%s overload is unique", method)
			resolved = name
		}
	}
	require.NotEmpty(s.t, resolved, "%s ABI method", method)
	input, err := s.abi.Pack(resolved, args...)
	require.NoError(s.t, err, kind)
	return gigaPerpsCall{kind: kind, method: resolved, to: gigaPerpsAddress, input: input, value: new(big.Int)}
}

func (s *gigaPerpsSequence) reject(call gigaPerpsCall, errorName string) gigaPerpsCall {
	call.rejected, call.errorName = true, errorName
	return call
}

func (s *gigaPerpsSequence) tokenCall(kind, method string, args ...interface{}) gigaPerpsCall {
	s.t.Helper()
	input, err := s.tokenABI.Pack(method, args...)
	require.NoError(s.t, err, kind)
	return gigaPerpsCall{kind: kind, method: method, to: common.HexToAddress(perpstypes.DefaultCollateralToken), input: input, value: new(big.Int)}
}

func (s *gigaPerpsSequence) approve(label string, amount int64) {
	good := s.tokenCall("approve", "approve", gigaPerpsAddress, big.NewInt(amount))
	bad := s.reject(good, "")
	bad.value = big.NewInt(1)
	s.block("approve_"+label, good, bad)
}

func (s *gigaPerpsSequence) order(kind perpstypes.OrderType, size, limit, trigger int64, cloid byte) gigaPerpsOrder {
	return gigaPerpsOrder{
		MarketId: big.NewInt(0), IsLong: true, OrderType: uint8(kind), TimeInForce: uint8(perpstypes.TimeInForceGTC),
		SizeUsd: gigaPerpsUSD(size), Leverage: gigaPerpsUSD(10), LimitPrice: gigaPerpsUSD(limit), TriggerPrice: gigaPerpsUSD(trigger),
		AcceptablePrice: new(big.Int), CollateralAmount: big.NewInt(100_000_000), TakeProfitPrice: new(big.Int), StopLossPrice: new(big.Int), Cloid: [16]byte{cloid},
	}
}

func (s *gigaPerpsSequence) run() {
	t := s.t
	usdl := common.HexToAddress(perpstypes.DefaultCollateralToken)
	zero, market := new(big.Int), big.NewInt(0)
	unknown := big.NewInt(999_999)
	deadline := uint64(s.timestamp.Unix() + 60)
	s.approve("deposit", 5_000_000_000)
	first := s.call("deposit_first_transaction", "deposit", usdl, big.NewInt(5_000_000_000))
	first.signer = 1
	require.Zero(t, s.nonces[1])
	for _, w := range []*app.TestWrapper{s.ordinary, s.fast} {
		require.Zero(t, w.App.EvmKeeper.GetNonce(w.App.GetCheckCtx(), s.signers[1].EvmAddress))
	}
	badFirst := s.reject(s.call("deposit_first_transaction", "deposit", usdl, zero), "PerpsZeroAmount")
	badFirst.signer = 1
	s.block("deposit", s.call("deposit", "deposit", usdl, big.NewInt(5_000_000_000)),
		s.reject(s.call("deposit", "deposit", usdl, zero), "PerpsZeroAmount"), first,
		badFirst)
	for _, w := range []*app.TestWrapper{s.ordinary, s.fast} {
		account, err := w.App.PerpsKeeper.GetAccount(w.App.GetCheckCtx(), s.signers[1].EvmAddress)
		require.NoError(t, err)
		require.True(t, account.Created)
		require.Equal(t, uint256.NewInt(5_000_000_000), &account.Collateral)
		require.Equal(t, uint64(2), w.App.EvmKeeper.GetNonce(w.App.GetCheckCtx(), s.signers[1].EvmAddress))
	}
	s.approve("open", 100_000_000)
	wallet := s.wallet()
	opened := s.block("openPosition", s.call("openPosition", "openPosition", market, usdl, big.NewInt(100_000_000), gigaPerpsUSD(10), true),
		s.reject(s.call("openPosition", "openPosition", market, usdl, zero, gigaPerpsUSD(10), true), "PerpsZeroAmount"))
	positionID := s.outputID("openPosition", opened.messages[0])
	require.Equal(t, new(big.Int).Sub(wallet, big.NewInt(100_000_000)), s.wallet())
	position := s.position(positionID)
	require.True(t, position.IsLong)
	require.False(t, position.SizeBase.IsZero())

	marketOrder := s.order(perpstypes.OrderTypeMarket, 600, 0, 0, 0)
	marketOrder.TimeInForce, marketOrder.Deadline = uint8(perpstypes.TimeInForceIOC), deadline
	marketOrder.AcceptablePrice = gigaPerpsUSD(60_300)
	aboveMargin := marketOrder
	aboveMargin.SizeUsd = gigaPerpsUSD(1_000_000)
	wallet = s.wallet()
	s.block("market_order", s.call("market_order", "placeOrder", marketOrder),
		s.reject(s.call("market_order", "placeOrder", aboveMargin), "PerpsInsufficientMargin"))
	require.Equal(t, wallet, s.wallet(), "market_order uses deposited collateral")
	larger := s.position(positionID)
	require.True(t, larger.SizeBase.Gt(&position.SizeBase))
	addSize := marketOrder
	addSize.SizeUsd, addSize.CollateralAmount = gigaPerpsUSD(300), big.NewInt(30_000_000)
	s.block("add_size_order", s.call("add_size_order", "placeOrder", addSize),
		s.reject(s.call("add_size_order", "placeOrder", aboveMargin), "PerpsInsufficientMargin"))
	position = s.position(positionID)
	require.True(t, position.SizeBase.Gt(&larger.SizeBase))
	require.Equal(t, wallet, s.wallet(), "add_size_order uses deposited collateral")

	s.approve("resting_orders", 200_000_000)
	limit := s.order(perpstypes.OrderTypeLimit, 500, 58_000, 0, 1)
	badLimit := limit
	badLimit.LimitPrice = zero
	placed := s.block("limit_order", s.call("limit_order", "placeOrder", limit),
		s.reject(s.call("limit_order", "placeOrder", badLimit), "PerpsZeroAmount"))
	limitID := s.outputID("placeOrder", placed.messages[0])
	stop := s.order(perpstypes.OrderTypeStopLimit, 500, 63_000, 62_000, 2)
	badStop := stop
	badStop.TriggerPrice = zero
	placed = s.block("stop_limit_order", s.call("stop_limit_order", "placeOrder", stop),
		s.reject(s.call("stop_limit_order", "placeOrder", badStop), "PerpsInvalidTriggerPrice"))
	stopID := s.outputID("placeOrder", placed.messages[0])
	require.Equal(t, wallet, s.wallet(), "resting placeOrder calls use deposited collateral")
	for _, w := range []*app.TestWrapper{s.ordinary, s.fast} {
		order, err := w.App.PerpsKeeper.GetOrder(w.App.GetCheckCtx(), gigaPerpsWord(limitID))
		require.NoError(t, err)
		require.Equal(t, perpstypes.OrderStatusResting, order.Status)
		order, err = w.App.PerpsKeeper.GetOrder(w.App.GetCheckCtx(), gigaPerpsWord(stopID))
		require.NoError(t, err)
		require.Equal(t, perpstypes.OrderStatusUntriggered, order.Status)
	}
	s.reads(positionID, limitID)
	s.block("cancelOrder", s.call("cancelOrder", "cancelOrder", limitID),
		s.reject(s.call("cancelOrder", "cancelOrder", unknown), "PerpsUnknownOrder"))
	s.block("setPositionTpSl", s.call("setPositionTpSl", "setPositionTpSl", positionID, gigaPerpsUSD(65_000), gigaPerpsUSD(55_000)),
		s.reject(s.call("setPositionTpSl", "setPositionTpSl", positionID, gigaPerpsUSD(55_000), gigaPerpsUSD(65_000)), "PerpsInvalidTriggerPrice"))
	s.approve("add_collateral", 10_000_000)
	wallet = s.wallet()
	s.block("addCollateral", s.call("addCollateral", "addCollateral", positionID, big.NewInt(10_000_000)),
		s.reject(s.call("addCollateral", "addCollateral", positionID, zero), "PerpsZeroAmount"))
	require.Equal(t, new(big.Int).Sub(wallet, big.NewInt(10_000_000)), s.wallet())
	wallet = s.wallet()
	s.block("removeCollateral", s.call("removeCollateral", "removeCollateral", positionID, big.NewInt(5_000_000)),
		s.reject(s.call("removeCollateral", "removeCollateral", positionID, big.NewInt(1_000_000_000_000)), "PerpsInsufficientMargin"))
	require.Equal(t, new(big.Int).Add(wallet, big.NewInt(5_000_000)), s.wallet())
	s.block("cancelByCloid", s.call("cancelByCloid", "cancelByCloid", market, stop.Cloid),
		s.reject(s.call("cancelByCloid", "cancelByCloid", market, [16]byte{99}), "PerpsUnknownOrder"))
	s.approve("top_up", 10_000_000)
	wallet = s.wallet()
	s.block("topUpPosition", s.call("topUpPosition", "topUpPosition", positionID, big.NewInt(10_000_000)),
		s.reject(s.call("topUpPosition", "topUpPosition", unknown, big.NewInt(10_000_000)), "PerpsUnknownPosition"))
	require.Equal(t, new(big.Int).Sub(wallet, big.NewInt(10_000_000)), s.wallet())

	s.approve("compatibility_limit", 100_000_000)
	wallet = s.wallet()
	placed = s.block("placeLimitOrder", s.call("placeLimitOrder", "placeLimitOrder", market, true, gigaPerpsUSD(58_000), gigaPerpsUSD(500), gigaPerpsUSD(10), usdl, big.NewInt(100_000_000)),
		s.reject(s.call("placeLimitOrder", "placeLimitOrder", market, true, gigaPerpsUSD(58_000), gigaPerpsUSD(500), gigaPerpsUSD(10), usdl, zero), "PerpsZeroAmount"))
	compatLimitID := s.outputID("placeLimitOrder", placed.messages[0])
	require.Equal(t, new(big.Int).Sub(wallet, big.NewInt(100_000_000)), s.wallet())
	s.approve("compatibility_stop_limit", 100_000_000)
	wallet = s.wallet()
	placed = s.block("placeStopLimitOrder", s.call("placeStopLimitOrder", "placeStopLimitOrder", market, true, gigaPerpsUSD(62_000), gigaPerpsUSD(63_000), gigaPerpsUSD(500), gigaPerpsUSD(10), usdl, big.NewInt(100_000_000)),
		s.reject(s.call("placeStopLimitOrder", "placeStopLimitOrder", market, true, gigaPerpsUSD(62_000), gigaPerpsUSD(63_000), gigaPerpsUSD(500), gigaPerpsUSD(10), usdl, zero), "PerpsZeroAmount"))
	compatStopID := s.outputID("placeStopLimitOrder", placed.messages[0])
	require.Equal(t, new(big.Int).Sub(wallet, big.NewInt(100_000_000)), s.wallet())
	s.block("cancel_compatibility_orders", s.call("cancelOrder", "cancelOrder", compatLimitID), s.call("cancelOrder", "cancelOrder", compatStopID))

	position = s.position(positionID)
	s.block("partial_close", s.call("partial_close", "closePositionWithParams", positionID, gigaPerpsUSD(100), gigaPerpsUSD(59_700), deadline),
		s.reject(s.call("partial_close", "closePositionWithParams", positionID, gigaPerpsUSD(1_000_000), gigaPerpsUSD(59_700), deadline), "PerpsCloseSizeExceedsPosition"))
	smaller := s.position(positionID)
	require.True(t, smaller.SizeBase.Lt(&position.SizeBase))
	require.False(t, smaller.SizeBase.IsZero())
	s.block("full_close", s.call("full_close", "closePositionWithParams", positionID, zero, gigaPerpsUSD(59_700), deadline),
		s.reject(s.call("full_close", "closePositionWithParams", unknown, zero, gigaPerpsUSD(59_700), deadline), "PerpsUnknownPosition"))
	for _, w := range []*app.TestWrapper{s.ordinary, s.fast} {
		positions, err := w.App.PerpsKeeper.AccountPositions(w.App.GetCheckCtx(), s.signers[0].EvmAddress)
		require.NoError(t, err)
		require.Empty(t, positions)
	}
	wallet = s.wallet()
	s.block("withdraw", s.call("withdraw", "withdraw", usdl, big.NewInt(1_000_000_000)),
		s.reject(s.call("withdraw", "withdraw", usdl, big.NewInt(1_000_000_000_000)), "PerpsInsufficientMargin"))
	require.Equal(t, new(big.Int).Add(wallet, big.NewInt(1_000_000_000)), s.wallet())
}

func (s *gigaPerpsSequence) reads(positionID, orderID *big.Int) {
	usdl := common.HexToAddress(perpstypes.DefaultCollateralToken)
	account := s.signers[0].EvmAddress
	calls := []gigaPerpsCall{
		s.call("getMarketConfig", "getMarketConfig", big.NewInt(0)),
		s.call("getMarginTiers", "getMarginTiers", big.NewInt(0)),
		s.call("getMarkPriceDetails", "getMarkPriceDetails", big.NewInt(0)),
		s.call("getFeeTier", "getFeeTier", account),
		s.call("getAccount", "getAccount", account),
		s.call("getAccountSummary", "getAccountSummary", account, usdl),
		s.call("getBalance", "getBalance", account, usdl),
		s.call("getLockedBalance", "getLockedBalance", account, usdl),
		s.call("getPositionMargin", "getPositionMargin", positionID),
		s.call("getPosition", "getPosition", positionID),
		s.call("getPositionDetails", "getPositionDetails", positionID),
		s.call("getUserPositionIds", "getUserPositionIds", account),
		s.call("getOrder", "getOrder", orderID),
		s.call("getUserOrderIds", "getUserOrderIds", account),
		s.call("getOpenInterest", "getOpenInterest", big.NewInt(0)),
		s.call("quoteMarket", "quoteMarket", big.NewInt(0)),
		s.call("quoteOpenPosition", "quoteOpenPosition", big.NewInt(0), usdl, big.NewInt(100_000_000), gigaPerpsUSD(10), true),
		s.call("quoteClosePosition", "quoteClosePosition", positionID),
		s.call("quotePartialClose", "quotePartialClose", positionID, gigaPerpsUSD(100)),
		s.tokenCall("balanceOf", "balanceOf", account),
		s.tokenCall("allowance", "allowance", account, gigaPerpsAddress),
	}
	for _, call := range calls {
		bad := s.reject(call, "")
		bad.value = big.NewInt(1)
		s.block(call.kind, call, bad)
	}
}

func (s *gigaPerpsSequence) wallet() *big.Int {
	s.t.Helper()
	var balance *big.Int
	for _, w := range []*app.TestWrapper{s.ordinary, s.fast} {
		value := w.App.EvmKeeper.GetState(w.App.GetCheckCtx(), common.HexToAddress(perpstypes.DefaultCollateralToken), gigaPerpsTokenSlot(s.signers[0].EvmAddress, 0)).Big()
		if balance != nil {
			require.Equal(s.t, balance, value, "wallet balance")
		}
		balance = value
	}
	return balance
}

func (s *gigaPerpsSequence) position(id *big.Int) perpstypes.Position {
	s.t.Helper()
	var position perpstypes.Position
	for i, w := range []*app.TestWrapper{s.ordinary, s.fast} {
		got, err := w.App.PerpsKeeper.GetPosition(w.App.GetCheckCtx(), gigaPerpsWord(id))
		require.NoError(s.t, err)
		if i > 0 {
			require.Equal(s.t, position, got, "position state")
		}
		position = got
	}
	return position
}

func (s *gigaPerpsSequence) outputID(method string, response evmtypes.MsgEVMTransactionResponse) *big.Int {
	s.t.Helper()
	out, err := s.abi.Unpack(method, response.ReturnData)
	require.NoError(s.t, err)
	require.Len(s.t, out, 1)
	id, ok := out[0].(*big.Int)
	require.True(s.t, ok)
	require.Positive(s.t, id.Sign())
	return id
}

func (s *gigaPerpsSequence) block(name string, calls ...gigaPerpsCall) gigaPerpsBlock {
	s.t.Helper()
	var want gigaPerpsBlock
	ok := s.t.Run(name, func(t *testing.T) {
		s.height++
		txs := make([][]byte, len(calls))
		signed := make([]*ethtypes.Transaction, len(calls))
		chainID := s.ordinary.App.EvmKeeper.ChainID(s.ordinary.Ctx)
		txConfig := app.MakeEncodingConfig().TxConfig
		for i, call := range calls {
			s.kinds[call.kind] = true
			tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
				Nonce: s.nonces[call.signer], GasPrice: big.NewInt(500_000_000_000), Gas: gigaPerpsGas,
				To: &call.to, Value: call.value, Data: call.input,
			}), ethtypes.LatestSignerForChainID(chainID), s.signers[call.signer].EvmPrivateKey)
			require.NoError(t, err)
			s.nonces[call.signer]++
			signed[i] = tx
			data, err := ethtx.NewTxDataFromTx(tx)
			require.NoError(t, err)
			msg, err := evmtypes.NewMsgEVMTransaction(data)
			require.NoError(t, err)
			builder := txConfig.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			builder.SetGasLimit(gigaPerpsGas)
			txs[i], err = txConfig.TxEncoder()(builder.GetTx())
			require.NoError(t, err)
		}
		before := mergeFallbackCounter(t)
		want = s.apply(t, s.ordinary, txs, signed)
		require.Equal(t, before, mergeFallbackCounter(t), "%s ordinary execution", name)
		got := s.apply(t, s.fast, txs, signed)
		if calls[0].to == gigaPerpsAddress {
			require.Equal(t, before+1, mergeFallbackCounter(t), "%s fast execution reaches the custom precompile", name)
		}
		for i, call := range calls {
			label := fmt.Sprintf("%s transaction %d rejected=%t", call.kind, i, call.rejected)
			a, b := want.results[i], got.results[i]
			if call.kind == "getMarginTiers" {
				for path, result := range []*abci.ExecTxResult{a, b} {
					t.Logf("%s path %d: code=%d log=%q gas limit=%d wanted=%d used=%d", label, path,
						result.Code, result.Log, gigaPerpsGas, result.GasWanted, result.GasUsed)
				}
			}
			require.Equal(t, a.Code, b.Code, "%s field status", label)
			require.Equal(t, a.Codespace, b.Codespace, "%s field codespace", label)
			require.Equal(t, a.GasWanted, b.GasWanted, "%s field gas wanted", label)
			require.Equal(t, a.GasUsed, b.GasUsed, "%s field gas used", label)
			require.Equal(t, a.Log, b.Log, "%s field result log", label)
			require.Equal(t, a.Info, b.Info, "%s field result info", label)
			require.Equal(t, a.Data, b.Data, "%s field returned data", label)
			require.Equal(t, a.Events, b.Events, "%s field events", label)
			require.Equal(t, a.EvmTxInfo, b.EvmTxInfo, "%s field transaction info", label)
			require.Equal(t, want.messages[i], got.messages[i], "%s field EVM response", label)
			require.Equal(t, want.receipts[i].Status, got.receipts[i].Status, "%s field receipt status", label)
			require.Equal(t, want.receipts[i].GasUsed, got.receipts[i].GasUsed, "%s field receipt gas used", label)
			require.Equal(t, want.receipts[i].Logs, got.receipts[i].Logs, "%s field logs", label)
			require.Equal(t, want.receipts[i], got.receipts[i], "%s field receipt", label)
			for path, out := range []gigaPerpsBlock{want, got} {
				result, receipt, message := out.results[i], out.receipts[i], out.messages[i]
				require.Positive(t, result.GasUsed, "%s path %d", label, path)
				require.Positive(t, receipt.GasUsed, "%s path %d", label, path)
				if call.rejected {
					require.Equal(t, sdkerrors.ErrEVMVMError.ABCICode(), result.Code, "%s path %d: %s", label, path, result.Log)
					require.Zero(t, receipt.Status, "%s path %d", label, path)
					require.NotEmpty(t, message.VmError, "%s path %d", label, path)
					require.Empty(t, receipt.Logs, "%s path %d reverted logs", label, path)
					if call.errorName != "" {
						require.Equal(t, crypto.Keccak256([]byte(call.errorName + "()"))[:4], message.ReturnData, "%s path %d refusal", label, path)
					}
				} else {
					require.Zero(t, result.Code, "%s path %d: %s", label, path, result.Log)
					require.Equal(t, uint32(1), receipt.Status, "%s path %d: %s", label, path, receipt.VmError)
					require.Empty(t, message.VmError, "%s path %d", label, path)
					parsed := s.abi
					if call.to != gigaPerpsAddress {
						parsed = s.tokenABI
					}
					_, err := parsed.Unpack(call.method, message.ReturnData)
					require.NoError(t, err, "%s path %d output", label, path)
					if call.to == gigaPerpsAddress && perps.GasTable[call.method].Write || call.method == "approve" {
						require.NotEmpty(t, receipt.Logs, "%s path %d emitted logs", label, path)
					}
				}
			}
		}
		require.Equal(t, want.hash, got.hash, "%s field state hash", name)
		s.transactions += len(calls)
	})
	if !ok {
		s.t.FailNow()
	}
	return want
}

func (s *gigaPerpsSequence) apply(t *testing.T, w *app.TestWrapper, txs [][]byte, signed []*ethtypes.Transaction) gigaPerpsBlock {
	t.Helper()
	response, err := w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{Height: s.height, ChainID: chainsBlockChainID,
			Time: s.timestamp.Add(time.Duration(s.height-1) * time.Millisecond), ProposerAddress: s.proposer},
		Txs: txs,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.TxResults, len(txs))
	require.NotEmpty(t, response.AppHash)
	_, err = w.App.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, response.AppHash, w.App.LastCommitID().Hash)
	ctx := w.App.GetCheckCtx()
	out := gigaPerpsBlock{results: response.TxResults, hash: append([]byte(nil), response.AppHash...)}
	for i, tx := range signed {
		result := response.TxResults[i]
		receipt, err := w.App.EvmKeeper.GetReceiptWithRetry(ctx, tx.Hash(), 5)
		require.NoError(t, err, "receipt %d: fast=%t code=%d codespace=%s log=%q gas limit=%d wanted=%d used=%d EVM info=%+v data=%x",
			i, w.App.GigaExecutorEnabled, result.Code, result.Codespace, result.Log, tx.Gas(), result.GasWanted, result.GasUsed, result.EvmTxInfo, result.Data)
		out.receipts = append(out.receipts, receipt)
		var data sdk.TxMsgData
		require.NoError(t, data.Unmarshal(response.TxResults[i].Data))
		require.Len(t, data.Data, 1, "transaction %d: %s", i, response.TxResults[i].Log)
		var message evmtypes.MsgEVMTransactionResponse
		require.NoError(t, message.Unmarshal(data.Data[0].Data))
		require.Equal(t, tx.Hash().Hex(), message.Hash)
		out.messages = append(out.messages, message)
	}
	message, broken := perpskeeper.SolvencyInvariant(w.App.PerpsKeeper)(ctx)
	require.False(t, broken, message)
	for i, signer := range s.signers {
		require.Equal(t, s.nonces[i], w.App.EvmKeeper.GetNonce(ctx, signer.EvmAddress), "signer %d nonce", i)
	}
	return out
}
