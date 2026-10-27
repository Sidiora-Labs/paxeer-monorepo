package keeper_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAgentQueryJobAndPending(t *testing.T) {
	h := newHarness(t)
	ctx := sdk.WrapSDKContext(h.ctx)
	query := keeper.Querier{Keeper: h.keeper}
	creator := common.HexToAddress("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	job := keeper.Job{
		ID: 1, Creator: creator,
		Target:   common.HexToAddress("0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"),
		CallData: []byte{0, 0xde, 0xad, 0xff}, ExecuteAtBlock: dueAt,
		GasLimit: gasLimit, Deposit: sdk.NewIntFromBigInt(deposit), Active: true,
	}
	h.keeper.SetJob(h.ctx, job)
	response, err := query.Job(ctx, &types.QueryJobRequest{Id: 1})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, keeper.JobToProto(job), response.Job)
	response.Job.CallData[0] = 1
	stored, found := h.keeper.GetJob(h.ctx, job.ID)
	require.True(t, found)
	require.Equal(t, job, stored)

	response, err = query.Job(ctx, &types.QueryJobRequest{Id: 2})
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, response)
	response, err = query.Job(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, response)

	for _, address := range []string{creator.Hex(), strings.ToLower(creator.Hex())} {
		pending, err := query.Pending(ctx, &types.QueryPendingRequest{Creator: address})
		require.NoError(t, err)
		require.NotNil(t, pending)
		require.Equal(t, []uint64{1}, pending.JobIds)
	}

	other := job
	other.ID = 2
	other.Creator = job.Target
	h.keeper.SetJob(h.ctx, other)
	for _, id := range []uint64{9, 3} {
		next := job
		next.ID = id
		h.keeper.SetJob(h.ctx, next)
	}
	pending, err := query.Pending(ctx, &types.QueryPendingRequest{Creator: creator.Hex()})
	require.NoError(t, err)
	require.NotNil(t, pending)
	require.Equal(t, []uint64{1, 3, 9}, pending.JobIds)
	require.Equal(t, h.keeper.Pending(h.ctx, creator), pending.JobIds)
	h.keeper.DeleteJob(h.ctx, job)
	pending, err = query.Pending(ctx, &types.QueryPendingRequest{Creator: creator.Hex()})
	require.NoError(t, err)
	require.Equal(t, []uint64{3, 9}, pending.JobIds)
	pending, err = query.Pending(ctx, &types.QueryPendingRequest{Creator: common.Address{}.Hex()})
	require.NoError(t, err)
	require.NotNil(t, pending)
	require.Empty(t, pending.JobIds)

	for _, address := range []string{
		"", "0x12", creator.Hex()[2:], "0X" + creator.Hex()[2:],
		"0x" + strings.Repeat("z", 40), creator.Hex() + "00", " " + creator.Hex(),
	} {
		pending, err := query.Pending(context.Background(), &types.QueryPendingRequest{Creator: address})
		require.Equal(t, codes.InvalidArgument, status.Code(err), address)
		require.Nil(t, pending)
	}
	pending, err = query.Pending(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, pending)
}

func TestAgentQueryParams(t *testing.T) {
	response, err := (keeper.Querier{}).Params(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, response)

	h := newHarness(t)
	ctx := sdk.WrapSDKContext(h.ctx)
	query := keeper.Querier{Keeper: h.keeper}
	response, err = query.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, types.DefaultParams(), response.Params)

	params := types.DefaultParams()
	params.MaxJobsPerCreator = 7
	params.MaxScheduleHorizon = 123
	params.MinDepositFactor = 3
	params.MaxJobsPerBlock = 5
	params.MaxJobGasLimit = 200_000
	params.MaxReasonBytes = 17
	params.MaxCallDataBytes = 32
	params.MaxBlockGas = 400_000
	params.Paused = true
	require.NoError(t, h.keeper.SetParams(h.ctx, params))
	response, err = query.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, params, response.Params)
	require.Equal(t, h.keeper.GetParams(h.ctx), response.Params)
	response.Params.Paused = false
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
}
