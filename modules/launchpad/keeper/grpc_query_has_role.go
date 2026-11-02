package keeper

import (
	"context"
	"errors"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func (q Querier) HasRole(goCtx context.Context, req *types.QueryHasRoleRequest) (*types.QueryHasRoleResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	contract, err := types.ParseHexAddress("contract", req.Contract)
	if err != nil {
		return nil, errors.Unwrap(err)
	}
	account, err := types.ParseHexAddress("account", req.Account)
	if err != nil {
		return nil, errors.Unwrap(err)
	}
	decoded, err := hexutil.Decode(req.Role)
	if err != nil || !strings.HasPrefix(req.Role, "0x") || len(decoded) != 32 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "role must be 0x followed by 64 hex digits")
	}
	var role [32]byte
	copy(role[:], decoded)
	return &types.QueryHasRoleResponse{HasRole: q.Keeper.HasRole(ctx, contract, role, account)}, nil
}
