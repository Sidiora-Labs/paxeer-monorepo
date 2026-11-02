package keeper_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryRequestAnswersARecordedRequest(t *testing.T) {
	s := newSuite(t, true)
	id := s.request()
	server := keeper.NewQueryServer(s.k)
	goCtx := sdk.WrapSDKContext(s.ctx)

	answer, err := server.Request(goCtx, &types.QueryRequestRequest{RequestId: id})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Equal(t, types.Request{
		ID: id, Requester: types.Address20(requester), Kind: types.KindFetch,
		PayloadHash: types.Keccak(payload), CallbackGas: callbackGas, Fee: sdk.NewInt(fee),
		Height: startHeight, TimeoutHeight: startHeight + int64(timeout), Status: types.StatusPending,
	}, answer.Request)
	stored, found := s.k.GetRequest(s.ctx, id)
	require.True(t, found)
	require.Equal(t, stored, answer.Request)

	for _, missing := range []uint64{0, id + 1} {
		answer, err = server.Request(goCtx, &types.QueryRequestRequest{RequestId: missing})
		require.Nil(t, answer)
		require.Equal(t, codes.NotFound, status.Code(err))
		require.Equal(t, types.ErrUnknownRequest.Error(), status.Convert(err).Message())
	}
	answer, err = server.Request(context.Background(), nil)
	require.Nil(t, answer)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestQueryResultUnknownIdIsNotFound(t *testing.T) {
	s := newSuite(t, true)
	server := keeper.NewQueryServer(s.k)
	goCtx := sdk.WrapSDKContext(s.ctx)
	answer, err := server.Result(goCtx, &types.QueryResultRequest{RequestId: 1})
	require.Nil(t, answer)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, types.ErrUnknownRequest.Error(), status.Convert(err).Message())

	id := s.request()
	answer, err = server.Result(goCtx, &types.QueryResultRequest{RequestId: id})
	require.Nil(t, answer)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, types.ErrUnknownRequest.Error(), status.Convert(err).Message())

	_, want, err := s.k.Fulfil(s.ctx, id, response, digest, 9000,
		s.signed(id, response, digest, 9000, s.attestors[0], s.attestors[1]))
	require.NoError(t, err)
	answer, err = server.Result(goCtx, &types.QueryResultRequest{RequestId: id})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Equal(t, want, answer.Result)
	require.Equal(t, response, answer.Result.Response)
	require.Equal(t, digest, answer.Result.ContentDigest)
	require.Equal(t, uint32(9000), answer.Result.FullLength)

	require.NoError(t, s.k.RecordCallback(s.ctx, id, types.CallbackDelivered, 21_000))
	answer, err = server.Result(goCtx, &types.QueryResultRequest{RequestId: id})
	require.NoError(t, err)
	stored, found := s.k.GetResult(s.ctx, id)
	require.True(t, found)
	require.Equal(t, stored, answer.Result)
	require.Equal(t, types.CallbackDelivered, answer.Result.Callback)
	require.Equal(t, uint64(21_000), answer.Result.CallbackGasUsed)

	answer, err = server.Result(context.Background(), nil)
	require.Nil(t, answer)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
