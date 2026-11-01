package types_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestRegisteredErrorsCoverCodes2To45(t *testing.T) {
	want := []struct {
		err    *sdkerrors.Error
		reason string
	}{
		{types.ErrInvalidParams, "LXP_PERPS_INVALID_PARAMS"},
		{types.ErrUnknownMarket, "LXP_PERPS_UNKNOWN_MARKET"},
		{types.ErrMarketDisabled, "LXP_PERPS_MARKET_DISABLED"},
		{types.ErrPriceStale, "LXP_PERPS_PRICE_STALE"},
		{types.ErrZeroAmount, "LXP_PERPS_ZERO_AMOUNT"},
		{types.ErrUnsupportedCollateral, "LXP_PERPS_UNSUPPORTED_COLLATERAL"},
		{types.ErrInsufficientCollateral, "LXP_PERPS_INSUFFICIENT_COLLATERAL"},
		{types.ErrInsufficientMargin, "LXP_PERPS_INSUFFICIENT_MARGIN"},
		{types.ErrLeverageTooHigh, "LXP_PERPS_LEVERAGE_TOO_HIGH"},
		{types.ErrTickSize, "LXP_PERPS_TICK_SIZE"},
		{types.ErrLotSize, "LXP_PERPS_LOT_SIZE"},
		{types.ErrMinOrderValue, "LXP_PERPS_MIN_ORDER_VALUE"},
		{types.ErrMaxOrderValue, "LXP_PERPS_MAX_ORDER_VALUE"},
		{types.ErrOpenInterestCap, "LXP_PERPS_OPEN_INTEREST_CAP"},
		{types.ErrInvalidTimeInForce, "LXP_PERPS_INVALID_TIME_IN_FORCE"},
		{types.ErrInvalidOrderType, "LXP_PERPS_INVALID_ORDER_TYPE"},
		{types.ErrPostOnlyWouldCross, "LXP_PERPS_POST_ONLY_WOULD_CROSS"},
		{types.ErrFokNotFilled, "LXP_PERPS_FOK_NOT_FILLED"},
		{types.ErrNoLiquidity, "LXP_PERPS_NO_LIQUIDITY"},
		{types.ErrSlippageExceeded, "LXP_PERPS_SLIPPAGE_EXCEEDED"},
		{types.ErrDeadlineExpired, "LXP_PERPS_DEADLINE_EXPIRED"},
		{types.ErrReduceOnlyViolation, "LXP_PERPS_REDUCE_ONLY_VIOLATION"},
		{types.ErrDuplicateCloid, "LXP_PERPS_DUPLICATE_CLOID"},
		{types.ErrUnknownOrder, "LXP_PERPS_UNKNOWN_ORDER"},
		{types.ErrNotOrderHolder, "LXP_PERPS_NOT_ORDER_HOLDER"},
		{types.ErrUnknownPosition, "LXP_PERPS_UNKNOWN_POSITION"},
		{types.ErrNotPositionHolder, "LXP_PERPS_NOT_POSITION_HOLDER"},
		{types.ErrInvalidTriggerPrice, "LXP_PERPS_INVALID_TRIGGER_PRICE"},
		{types.ErrTriggerNotReached, "LXP_PERPS_TRIGGER_NOT_REACHED"},
		{types.ErrTooManyOpenOrders, "LXP_PERPS_TOO_MANY_OPEN_ORDERS"},
		{types.ErrCrossMarginOnly, "LXP_PERPS_CROSS_MARGIN_ONLY"},
		{types.ErrNotLiquidatable, "LXP_PERPS_NOT_LIQUIDATABLE"},
		{types.ErrLiquidationCooldown, "LXP_PERPS_LIQUIDATION_COOLDOWN"},
		{types.ErrUnauthorized, "LXP_PERPS_UNAUTHORIZED"},
		{types.ErrStaticCall, "LXP_PERPS_STATIC_CALL"},
		{types.ErrDelegatecall, "LXP_PERPS_DELEGATECALL"},
		{types.ErrOverflow, "LXP_PERPS_OVERFLOW"},
		{types.ErrInvalidGenesis, "LXP_PERPS_INVALID_GENESIS"},
		{types.ErrCloseSizeExceedsPosition, "LXP_PERPS_CLOSE_SIZE_EXCEEDS_POSITION"},
		{types.ErrSelfTransfer, "LXP_PERPS_SELF_TRANSFER"},
		{types.ErrZeroAddress, "LXP_PERPS_ZERO_ADDRESS"},
		{types.ErrNoAccount, "LXP_PERPS_NO_ACCOUNT"},
		{types.ErrTokenTransferFailed, "LXP_PERPS_TOKEN_TRANSFER_FAILED"},
		{types.ErrPaused, "LXP_PERPS_PAUSED"},
	}

	registered := types.RegisteredErrors()
	require.Len(t, registered, 44)
	require.Len(t, want, 44)
	for i, expected := range want {
		err := registered[i]
		require.NotNil(t, err, "entry %d", i)
		require.Same(t, expected.err, err, "entry %d", i)
		require.Equal(t, uint32(i+2), err.ABCICode(), "entry %d", i)
		require.Equal(t, "perps", err.Codespace(), "entry %d", i)
		require.Equal(t, expected.reason, err.Error(), "entry %d", i)
	}
}
