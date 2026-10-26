package keeper

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPayQueryParams(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		response, err := (Querier{}).Params(context.Background(), nil)
		require.Nil(t, response)
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Equal(t, "empty request", status.Convert(err).Message())
	})

	t.Run("stored params", func(t *testing.T) {
		h := newPayHarness(t)
		query := Querier{Keeper: h.keeper}
		ctx := sdk.WrapSDKContext(h.ctx)
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)

		before := payStoreSnapshot(h.ctx, h.storeKey)
		response, err := query.Params(ctx, &types.QueryParamsRequest{})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, types.DefaultParams(), response.Params)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.storeKey))

		for _, params := range []types.Params{
			{Paused: false, EscrowEnabled: true, BudgetEnabled: false, StreamEnabled: true, ServiceEnabled: false},
			{Paused: true, EscrowEnabled: false, BudgetEnabled: true, StreamEnabled: false, ServiceEnabled: true},
		} {
			h.keeper.SetParams(h.ctx, params)
			before = payStoreSnapshot(h.ctx, h.storeKey)
			response, err = query.Params(ctx, &types.QueryParamsRequest{})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, params, response.Params)
			require.Equal(t, before, payStoreSnapshot(h.ctx, h.storeKey))
		}
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	})
}
