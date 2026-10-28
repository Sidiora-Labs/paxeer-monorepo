package keeper_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryAirdropClaimedServes(t *testing.T) {
	h := newHistoryEnv(t)
	record := h.nextEpoch()
	require.Equal(t, uint64(1), record.Epoch)
	pool, holder := h.pool(h.denom), h.evmAddress(h.trader)
	router := baseapp.NewQueryServerTestHelper(h.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(h.k))
	client := types.NewQueryClient(router)
	goCtx := context.Background()
	req := &types.QueryAirdropClaimedRequest{Pool: pool.Hex(), Holder: holder.Hex(), Epoch: record.Epoch}

	response, err := client.AirdropClaimed(goCtx, req)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.False(t, response.Claimed)
	require.Equal(t, h.k.HasClaimedPoolAirdrop(h.ctx, pool, holder, record.Epoch), response.Claimed)

	amount, err := h.claim(holder, record.Epoch)
	require.NoError(t, err)
	require.True(t, amount.IsPositive())
	router.Ctx = h.ctx
	response, err = client.AirdropClaimed(goCtx, req)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.True(t, response.Claimed)
	require.Equal(t, h.k.HasClaimedPoolAirdrop(h.ctx, pool, holder, record.Epoch), response.Claimed)

	response, err = client.AirdropClaimed(goCtx, &types.QueryAirdropClaimedRequest{
		Pool: strings.ToLower(pool.Hex()), Holder: strings.ToLower(holder.Hex()), Epoch: record.Epoch,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.True(t, response.Claimed)

	for _, tc := range []struct {
		name string
		req  types.QueryAirdropClaimedRequest
	}{
		{"other pool", types.QueryAirdropClaimedRequest{Pool: address("other query pool").Hex(), Holder: holder.Hex(), Epoch: record.Epoch}},
		{"other holder", types.QueryAirdropClaimedRequest{Pool: pool.Hex(), Holder: address("other query holder").Hex(), Epoch: record.Epoch}},
		{"other epoch", types.QueryAirdropClaimedRequest{Pool: pool.Hex(), Holder: holder.Hex(), Epoch: record.Epoch + 1}},
		{"zero epoch", types.QueryAirdropClaimedRequest{Pool: pool.Hex(), Holder: holder.Hex(), Epoch: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.AirdropClaimed(goCtx, &tc.req)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.False(t, response.Claimed)
		})
	}
}

func TestQueryAirdropClaimedRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)
	goCtx := context.Background()

	response, err := querier.AirdropClaimed(goCtx, nil)
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	pool, holder := address("query pool").Hex(), address("query holder").Hex()
	for _, tc := range []struct {
		name   string
		pool   string
		holder string
		want   error
	}{
		{"malformed pool", "0x1234", holder, sdkerrors.ErrInvalidAddress},
		{"malformed holder", pool, "0x1234", sdkerrors.ErrInvalidAddress},
		{"empty pool", "", holder, sdkerrors.ErrInvalidAddress},
		{"empty holder", pool, "", sdkerrors.ErrInvalidAddress},
		{"pool without prefix", strings.TrimPrefix(pool, "0x"), holder, sdkerrors.ErrInvalidAddress},
		{"holder without prefix", pool, strings.TrimPrefix(holder, "0x"), sdkerrors.ErrInvalidAddress},
		{"zero pool", (common.Address{}).Hex(), holder, types.ErrZeroAddress},
		{"zero holder", pool, (common.Address{}).Hex(), types.ErrZeroAddress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.AirdropClaimed(goCtx, &types.QueryAirdropClaimedRequest{
				Pool: tc.pool, Holder: tc.holder, Epoch: 1,
			})
			require.Nil(t, response)
			require.ErrorIs(t, err, tc.want)
			require.Same(t, tc.want, err)
			wantSpace, wantCode, _ := sdkerrors.ABCIInfo(tc.want, false)
			gotSpace, gotCode, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, wantSpace, gotSpace)
			require.Equal(t, wantCode, gotCode)
		})
	}
}
