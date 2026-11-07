package ante

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
)

var _ sdk.AnteDecorator = RevocationDecorator{}

type RevocationDecorator struct {
	keeper *keeper.Keeper
	evm    types.EVMKeeper
}

func NewRevocationDecorator(k *keeper.Keeper, evm types.EVMKeeper) RevocationDecorator {
	return RevocationDecorator{keeper: k, evm: evm}
}

func (d RevocationDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	ctx, err := d.walk(ctx, tx.GetMsgs())
	if err != nil {
		return ctx, err
	}
	return next(ctx, tx, simulate)
}

func (d RevocationDecorator) walk(ctx sdk.Context, msgs []sdk.Msg) (sdk.Context, error) {
	for _, msg := range msgs {
		exec, ok := msg.(*authz.MsgExec)
		if !ok {
			continue
		}
		inner, err := exec.GetMessages()
		if err != nil {
			return ctx, err
		}
		for _, authorized := range inner {
			signers := authorized.GetSigners()
			if len(signers) != 1 {
				return ctx, sdkerrors.ErrInvalidRequest.Wrap("authorization can be given to msg with only one signer")
			}
			ctx, err = d.WithGrantorRevocation(ctx, signers[0])
			if err != nil {
				return ctx, err
			}
		}
		ctx, err = d.walk(ctx, inner)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func (d RevocationDecorator) WithGrantorRevocation(ctx sdk.Context, grantor sdk.AccAddress) (sdk.Context, error) {
	evmAddress, associated := d.evm.GetEVMAddress(ctx, grantor)
	if !associated {
		return ctx, nil
	}
	did, bound := d.evm.GetLayerXDid(ctx, evmAddress)
	if !bound {
		return ctx, nil
	}
	sequence, err := d.keeper.RevocationSequence(ctx, did)
	if err != nil {
		return ctx, err
	}
	return types.WithGrantorRevocation(ctx, grantor, sequence), nil
}
