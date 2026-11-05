package vesting

import (
	"context"

	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

type msgServer struct {
	keeper.AccountKeeper
	types.BankKeeper
}

// NewMsgServerImpl returns an implementation of the vesting MsgServer interface,
// wrapping the corresponding AccountKeeper and BankKeeper.
func NewMsgServerImpl(k keeper.AccountKeeper, bk types.BankKeeper) types.MsgServer {
	return &msgServer{AccountKeeper: k, BankKeeper: bk}
}

var _ types.MsgServer = msgServer{}

func (s msgServer) CreateVestingAccount(goCtx context.Context, msg *types.MsgCreateVestingAccount) (*types.MsgCreateVestingAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	ak := s.AccountKeeper
	bk := s.BankKeeper

	if err := bk.IsSendEnabledCoins(ctx, msg.Amount...); err != nil {
		return nil, err
	}

	from, err := sdk.AccAddressFromBech32(msg.FromAddress)
	if err != nil {
		return nil, err
	}
	to, err := sdk.AccAddressFromBech32(msg.ToAddress)
	if err != nil {
		return nil, err
	}

	allowListCache := make(map[string]bankkeeper.AllowedAddresses)
	if !bk.IsInDenomAllowList(ctx, from, msg.Amount, allowListCache) {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to send funds", msg.FromAddress)
	}

	if bk.BlockedAddr(to) || !bk.IsInDenomAllowList(ctx, to, msg.Amount, allowListCache) {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to receive funds", msg.ToAddress)
	}

	if acc := ak.GetAccount(ctx, to); acc != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "account %s already exists", msg.ToAddress)
	}

	startTime := ctx.BlockTime().Unix()
	if !msg.Delayed && msg.EndTime <= startTime {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "continuous vesting end time must be after block time")
	}

	baseAccount := ak.NewAccountWithAddress(ctx, to)
	if _, ok := baseAccount.(*authtypes.BaseAccount); !ok {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid account type; expected: BaseAccount, got: %T", baseAccount)
	}

	var admin sdk.AccAddress
	if len(msg.Admin) > 0 {
		admin, err = sdk.AccAddressFromBech32(msg.Admin)
		if err != nil {
			return nil, err
		}
		if acc := ak.GetAccount(ctx, admin); acc == nil {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "admin account %s doesn't exist", msg.ToAddress)
		}
	}

	baseVestingAccount := types.NewBaseVestingAccount(baseAccount.(*authtypes.BaseAccount), msg.Amount.Sort(), msg.EndTime, admin)

	var acc authtypes.GenesisAccount

	if msg.Delayed {
		acc = types.NewDelayedVestingAccountRaw(baseVestingAccount)
	} else {
		acc = types.NewContinuousVestingAccountRaw(baseVestingAccount, startTime)
	}

	if err := acc.Validate(); err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}

	ak.SetAccount(ctx, acc)

	err = bk.SendCoins(ctx, from, to, msg.Amount)
	if err != nil {
		return nil, err
	}

	vestingMetrics.newAccount.Add(goCtx, 1)
	for _, a := range msg.Amount {
		if a.Amount.IsInt64() {
			vestingMetrics.accountAmount.Record(goCtx, a.Amount.Int64(), otelmetric.WithAttributes(attribute.String("denom_class", telemetry.DenomClass(a.Denom))))
		}
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	)

	return &types.MsgCreateVestingAccountResponse{}, nil
}
