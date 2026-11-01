package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestPermissionlessCustodyMsgsAreRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnv(t, 0)
	server := keeper.NewMsgServerImpl(e.k)
	const planHeight int64 = 20
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(planHeight), types.MergePlanUpgrade)
	before := e.k.ExportGenesis(e.ctx)

	for _, msg := range []struct {
		name   string
		before error
		run    func(context.Context) (any, error)
	}{
		{"RequestWithdrawal", types.ErrInvalidProof, func(ctx context.Context) (any, error) {
			return server.RequestWithdrawal(ctx, &types.MsgRequestWithdrawal{
				Sender: e.payerAcc.String(), Receipt: []byte{0xff}, Proof: []byte{0xff},
				Header: []byte{0xff}, HeaderSignature: make([]byte, 64),
			})
		}},
		{"FinaliseWithdrawal", types.ErrInvalidProof, func(ctx context.Context) (any, error) {
			return server.FinaliseWithdrawal(ctx, &types.MsgFinaliseWithdrawal{
				Sender: e.payerAcc.String(), Receipt: []byte{0xff}, Proof: []byte{0xff},
				Header: []byte{0xff}, HeaderSignature: make([]byte, 64),
			})
		}},
		{"RequestForcedExit", types.ErrInvalidHex, func(ctx context.Context) (any, error) {
			return server.RequestForcedExit(ctx, &types.MsgRequestForcedExit{
				Sender: e.payerAcc.String(), Witness: []byte{0xff}, Account: "invalid",
				AssetId: "invalid", Recipient: "invalid", RecipientSignature: []byte{0xff},
			})
		}},
		{"ExecuteForcedExit", types.ErrInvalidHex, func(ctx context.Context) (any, error) {
			return server.ExecuteForcedExit(ctx, &types.MsgExecuteForcedExit{
				Sender: e.payerAcc.String(), Witness: []byte{0xff}, Account: "invalid",
				AssetId: "invalid", Recipient: "invalid", RecipientSignature: []byte{0xff},
			})
		}},
	} {
		for _, height := range []int64{planHeight - 1, planHeight, planHeight + 1} {
			t.Run(fmt.Sprintf("%s/height_%d", msg.name, height), func(t *testing.T) {
				ctx := e.ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
				response, err := msg.run(sdk.WrapSDKContext(ctx))
				if height < planHeight {
					require.ErrorIs(t, err, msg.before)
				} else {
					require.ErrorIs(t, err, types.ErrModuleRetired)
				}
				require.Nil(t, response)
				require.Equal(t, before, e.k.ExportGenesis(ctx))
				require.Empty(t, ctx.EventManager().Events())
			})
		}
	}
}

func TestGovernanceCustodyMsgsAreRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnv(t, 0)
	server := keeper.NewMsgServerImpl(e.k)
	signer := e.payerAcc.String()
	require.NotEqual(t, e.k.Authority(e.ctx), signer)
	const planHeight int64 = 20
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(planHeight), types.MergePlanUpgrade)
	before := e.k.ExportGenesis(e.ctx)

	for _, msg := range []struct {
		name string
		run  func(context.Context) (any, error)
	}{
		{"UpdateParams", func(ctx context.Context) (any, error) {
			return server.UpdateParams(ctx, &types.MsgUpdateParams{Authority: signer, Params: types.DefaultParams()})
		}},
		{"SetAsset", func(ctx context.Context) (any, error) {
			return server.SetAsset(ctx, &types.MsgSetAsset{Authority: signer})
		}},
		{"RegisterCheckpoint", func(ctx context.Context) (any, error) {
			return server.RegisterCheckpoint(ctx, &types.MsgRegisterCheckpoint{
				Authority: signer, StateRoot: "invalid", ReceiptRoot: "invalid",
			})
		}},
		{"SetEmergency", func(ctx context.Context) (any, error) {
			return server.SetEmergency(ctx, &types.MsgSetEmergency{Authority: signer, Enabled: true})
		}},
		{"CancelClaim", func(ctx context.Context) (any, error) {
			return server.CancelClaim(ctx, &types.MsgCancelClaim{Authority: signer, ClaimId: "invalid"})
		}},
	} {
		for _, height := range []int64{planHeight - 1, planHeight, planHeight + 1} {
			t.Run(fmt.Sprintf("%s/height_%d", msg.name, height), func(t *testing.T) {
				ctx := e.ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
				response, err := msg.run(sdk.WrapSDKContext(ctx))
				if height < planHeight {
					require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				} else {
					require.ErrorIs(t, err, types.ErrModuleRetired)
				}
				require.Nil(t, response)
				require.Equal(t, before, e.k.ExportGenesis(ctx))
				require.Empty(t, ctx.EventManager().Events())
			})
		}
	}
}
