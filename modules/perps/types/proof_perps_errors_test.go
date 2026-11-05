// Proves: perps/types registered errors and keeper.Keeper -> sdkerrors.ResponseDeliverTx / ABCI protobuf and common.LXPReason -> sdkerrors.ABCIError / abi.UnpackRevert and keeper.BookInsert.
//
// Repairs account.go, adl.go, book.go, matching.go, orders.go and triggers.go:
// fmt.Errorf(%w) preserved errors.Is but lost registered codes in ABCIInfo,
// which follows Cause(). SDK Error.Wrapf preserves both consumer contracts.
// The error registry has no persistent state; every transport and consumer is
// invoked twice independently. Real account and book records are also read
// twice, then their actual encoded bytes are truncated at the decoding edge.
//
// ValidateGenesis and GetMarket preserve their registered ABCI identities.
package types_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// proofPerpsTransport uses the application's encoder and protobuf codec, then
// the SDK client's own decoder. Neither error metadata nor wire bytes are
// reconstructed by the test.
func proofPerpsTransport(t *testing.T, produced error, root *sdkerrors.Error) {
	t.Helper()
	require.ErrorIs(t, produced, root)
	for pass := 0; pass < 2; pass++ {
		response := sdkerrors.ResponseDeliverTx(produced, 7, 5, false)
		require.Equal(t, "perps", response.Codespace)
		require.Equal(t, root.ABCICode(), response.Code)
		require.Contains(t, response.Log, root.Error())
		wire, err := response.Marshal()
		require.NoError(t, err)
		var received abci.ResponseDeliverTx
		require.NoError(t, received.Unmarshal(wire))
		require.Equal(t, response, received)
		observed := sdkerrors.ABCIError(received.Codespace, received.Code, received.Log)
		require.ErrorIs(t, observed, root)
		// A code from a different namespace cannot impersonate a perps refusal,
		// even if its log is the real producer's description.
		require.NotErrorIs(t, sdkerrors.ABCIError("proof_wrong_namespace", received.Code, received.Log), root)
		require.NotErrorIs(t, sdkerrors.ABCIError(received.Codespace, 999999, received.Log), root)
		var malformed abci.ResponseDeliverTx
		require.Error(t, malformed.Unmarshal(wire[:len(wire)-1]))
	}
	proofPerpsLXP(t, produced, root)
}

func proofPerpsLXP(t *testing.T, produced error, root *sdkerrors.Error) {
	t.Helper()
	for pass := 0; pass < 2; pass++ {
		wire := pcommon.LXPReason(produced)
		reason, err := abi.UnpackRevert(wire)
		require.NoError(t, err)
		require.Equal(t, root.Error(), strings.SplitN(reason, ":", 2)[0])
		require.Contains(t, reason, produced.Error())
		// Truncate the producer's ABI result into its dynamic-string header.
		_, err = abi.UnpackRevert(wire[:35])
		require.Error(t, err)
	}
}

func TestProofPerpsErrorsRegistry(t *testing.T) {
	// Independent inventory of every registered error, including all eight
	// anchors. Check numbers and names before crossing either transport.
	names := []string{
		"INVALID_PARAMS", "UNKNOWN_MARKET", "MARKET_DISABLED", "PRICE_STALE", "ZERO_AMOUNT",
		"UNSUPPORTED_COLLATERAL", "INSUFFICIENT_COLLATERAL", "INSUFFICIENT_MARGIN", "LEVERAGE_TOO_HIGH",
		"TICK_SIZE", "LOT_SIZE", "MIN_ORDER_VALUE", "MAX_ORDER_VALUE", "OPEN_INTEREST_CAP",
		"INVALID_TIME_IN_FORCE", "INVALID_ORDER_TYPE", "POST_ONLY_WOULD_CROSS", "FOK_NOT_FILLED",
		"NO_LIQUIDITY", "SLIPPAGE_EXCEEDED", "DEADLINE_EXPIRED", "REDUCE_ONLY_VIOLATION", "DUPLICATE_CLOID",
		"UNKNOWN_ORDER", "NOT_ORDER_HOLDER", "UNKNOWN_POSITION", "NOT_POSITION_HOLDER", "INVALID_TRIGGER_PRICE",
		"TRIGGER_NOT_REACHED", "TOO_MANY_OPEN_ORDERS", "CROSS_MARGIN_ONLY", "NOT_LIQUIDATABLE",
		"LIQUIDATION_COOLDOWN", "UNAUTHORIZED", "STATIC_CALL", "DELEGATECALL", "OVERFLOW", "INVALID_GENESIS",
		"CLOSE_SIZE_EXCEEDS_POSITION", "SELF_TRANSFER", "ZERO_ADDRESS", "NO_ACCOUNT", "TOKEN_TRANSFER_FAILED", "PAUSED",
	}
	registered := types.RegisteredErrors()
	require.Len(t, registered, len(names))
	for i, root := range registered {
		t.Run(names[i], func(t *testing.T) {
			require.Equal(t, uint32(i+2), root.ABCICode())
			require.Equal(t, "perps", root.Codespace())
			require.Equal(t, "LXP_PERPS_"+names[i], root.Error())
			proofPerpsTransport(t, root, root)
			proofPerpsTransport(t, root.Wrapf("request %d", i), root)
			// Description alone is not a registered identity.
			require.False(t, errors.Is(errors.New(root.Error()), root))
		})
	}
	response := sdkerrors.ResponseDeliverTx(nil, 7, 5, false)
	require.Zero(t, response.Code)
	require.Empty(t, response.Codespace)
	require.Empty(t, response.Log)
}

func TestProofPerpsErrorsKeeper(t *testing.T) {
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("proof_perps_errors")).WithBlockTime(time.Unix(1800000000, 0))
	k := keeper.NewKeeper(key, nil, nil)
	owner := common.HexToAddress("0x7101")
	other := common.HexToAddress("0x7102")
	id := *uint256.NewInt(71)
	for pass := 0; pass < 2; pass++ {
		_, err := k.GetAccount(ctx, owner)
		proofPerpsTransport(t, err, types.ErrNoAccount)
		_, err = k.PlaceOrder(ctx, owner, keeper.OrderParams{})
		proofPerpsTransport(t, err, types.ErrNoAccount)
		_, err = k.Deposit(ctx, owner, uint256.Int{})
		proofPerpsTransport(t, err, types.ErrZeroAmount)
		_, err = k.GetOrder(ctx, id)
		proofPerpsTransport(t, err, types.ErrUnknownOrder)
		_, _, err = k.ExecuteOrder(ctx, id)
		proofPerpsTransport(t, err, types.ErrUnknownOrder)
		_, err = k.CancelByCloid(ctx, owner, 0, [16]byte{})
		proofPerpsTransport(t, err, types.ErrUnknownOrder)
		_, err = k.Match(ctx, types.Market{ID: 1}, types.Order{ID: id, MarketID: 2})
		proofPerpsTransport(t, err, types.ErrUnknownMarket)
		_, _, err = keeper.ADLRankScore(types.Position{}, uint256.Int{}, *uint256.NewInt(1))
		proofPerpsTransport(t, err, types.ErrPriceStale)
		max := new(uint256.Int).SetAllOne()
		_, profitable, err := keeper.ADLRankScore(types.Position{IsLong: true, EntryPrice: *uint256.NewInt(1), SizeBase: *uint256.NewInt(1)}, *max, *uint256.NewInt(1))
		require.True(t, profitable)
		proofPerpsTransport(t, err, types.ErrOverflow)
	}
	// A valid ADL producer computes a real score without an error transport.
	unit := *uint256.NewInt(1000000000000000000)
	mark := *uint256.NewInt(2000000000000000000)
	score, profitable, err := keeper.ADLRankScore(types.Position{IsLong: true, EntryPrice: unit, SizeBase: unit}, mark, unit)
	require.NoError(t, err)
	require.True(t, profitable)
	require.Equal(t, *uint256.NewInt(4000000000000000000), score)

	account, created, err := k.EnsureAccount(ctx, owner)
	require.NoError(t, err)
	require.True(t, created)
	for pass := 0; pass < 2; pass++ {
		observed, err := k.GetAccount(ctx, owner)
		require.NoError(t, err)
		require.Equal(t, account, observed)
	}
	accountWire := ctx.KVStore(key).Get(types.AccountKey(owner))
	ctx.KVStore(key).Set(types.AccountKey(owner), accountWire[:len(accountWire)-1])
	_, err = k.GetAccount(ctx, owner)
	proofPerpsTransport(t, err, types.ErrInvalidParams)
	ctx.KVStore(key).Set(types.AccountKey(owner), accountWire)

	// ExecuteADL consumes the real account encoding produced by EnsureAccount.
	insurance := k.InsuranceAccountAddress()
	_, created, err = k.EnsureAccount(ctx, insurance)
	require.NoError(t, err)
	require.True(t, created)
	for pass := 0; pass < 2; pass++ {
		events, err := k.ExecuteADL(ctx, nil, nil)
		require.NoError(t, err)
		require.Empty(t, events)
	}
	insuranceWire := ctx.KVStore(key).Get(types.AccountKey(insurance))
	ctx.KVStore(key).Set(types.AccountKey(insurance), insuranceWire[:len(insuranceWire)-1])
	_, err = k.ExecuteADL(ctx, nil, nil)
	proofPerpsTransport(t, err, types.ErrInvalidParams)
	ctx.KVStore(key).Set(types.AccountKey(insurance), insuranceWire)

	// BookInsert is itself a consumer of GetOrder's ErrUnknownOrder. Run that
	// branch twice with separate real records, then exercise its refusal branch.
	for pass := uint64(0); pass < 2; pass++ {
		order := types.Order{ID: *uint256.NewInt(71 + pass), Account: owner, MarketID: 1, IsLong: true,
			Status: types.OrderStatusResting, LimitPrice: unit, Sequence: pass + 1}
		require.NoError(t, k.BookInsert(ctx, order))
		for read := 0; read < 2; read++ {
			observed, err := k.GetOrder(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, order, observed)
		}
		proofPerpsTransport(t, k.BookInsert(ctx, order), types.ErrInvalidParams)
		_, err = k.CancelOrder(ctx, other, order.ID)
		proofPerpsTransport(t, err, types.ErrNotOrderHolder)
		wire := ctx.KVStore(key).Get(types.OrderKey(order.ID))
		ctx.KVStore(key).Set(types.OrderKey(order.ID), wire[:len(wire)-1])
		proofPerpsTransport(t, k.BookInsert(ctx, order), types.ErrInvalidParams)
		_, _, err = k.ExecuteOrder(ctx, order.ID)
		proofPerpsTransport(t, err, types.ErrInvalidParams)
		ctx.KVStore(key).Set(types.OrderKey(order.ID), wire)
		order.Status = types.OrderStatusCancelled
		require.NoError(t, k.BookRemove(ctx, order))
		_, _, err = k.ExecuteOrder(ctx, order.ID)
		proofPerpsTransport(t, err, types.ErrUnknownOrder)
	}
}

func TestProofPerpsErrorsOutsideScope(t *testing.T) {
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("proof_perps_outside"))
	k := keeper.NewKeeper(key, nil, nil)
	for pass := 0; pass < 2; pass++ {
		genesis := types.DefaultGenesis()
		require.NoError(t, types.ValidateGenesis(*genesis))
		genesis.Params.CollateralToken = common.Address{}
		invalidGenesis := types.ValidateGenesis(*genesis)
		proofPerpsTransport(t, invalidGenesis, types.ErrInvalidGenesis)
		_, unknownMarket := k.GetMarket(ctx, 987)
		proofPerpsTransport(t, unknownMarket, types.ErrUnknownMarket)
	}
}
