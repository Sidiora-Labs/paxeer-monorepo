package keeper_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryParamsAnswersTheStoredParams(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	k, ctx := xwebtestutil.NewKeeper(testApp, testApp.GetContextForDeliverTx([]byte{}))
	genesis := types.DefaultGenesis()
	want := types.Params{
		Authority:        types.DefaultAuthority(),
		Fee:              sdk.NewInt(75),
		MaxPayloadBytes:  256,
		MaxCallbackGas:   300_000,
		TimeoutBlocks:    100,
		KernelNetworkId:  7,
		ProgramFeeDenoms: []string{types.ProgramFeeDenomUSDC, types.ProgramFeeDenomPAX},
	}
	genesis.Params = want
	k.InitGenesis(ctx, *genesis)
	server := keeper.NewQueryServer(k)
	goCtx := sdk.WrapSDKContext(ctx)

	answer, err := server.Params(goCtx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Equal(t, want, answer.Params)

	require.NoError(t, k.Unpause(ctx, types.MsgUnpause{Authority: want.Authority}))
	answer, err = server.Params(goCtx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Equal(t, want, answer.Params)

	want.KernelNetworkId = 19
	want.ProgramFeeDenoms = []string{types.ProgramFeeDenomSID}
	require.NoError(t, k.SetParams(ctx, want))
	require.NoError(t, k.Pause(ctx, types.MsgPause{Authority: want.Authority}))
	answer, err = server.Params(goCtx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Equal(t, want, answer.Params)

	answer, err = server.Params(context.Background(), nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, answer)
}

func TestQueryAttestorsAnswersTheSetAndThreshold(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	k, ctx := xwebtestutil.NewKeeper(testApp, testApp.GetContextForDeliverTx([]byte{}))
	k.InitGenesis(ctx, *types.DefaultGenesis())
	server := keeper.NewQueryServer(k)
	goCtx := sdk.WrapSDKContext(ctx)

	answer, err := server.Attestors(goCtx, &types.QueryAttestorsRequest{})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Empty(t, answer.Attestors)
	require.Zero(t, answer.Threshold)

	attestors := xwebtestutil.Attestors(3)
	for i := len(attestors) - 1; i >= 0; i-- {
		require.NoError(t, k.RegisterAttestor(ctx, types.MsgRegisterAttestor{
			Authority: types.DefaultAuthority(), Attestor: attestors[i].Registration(),
		}))
	}
	want := xwebtestutil.Set(attestors, 2)
	answer, err = server.Attestors(goCtx, &types.QueryAttestorsRequest{})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Equal(t, want.Attestors, answer.Attestors)
	require.Equal(t, want.Threshold, answer.Threshold)
	require.Equal(t, k.GetAttestorSet(ctx).Attestors, answer.Attestors)
	require.Equal(t, k.Threshold(ctx), answer.Threshold)

	require.NoError(t, k.SetThreshold(ctx, types.MsgSetThreshold{
		Authority: types.DefaultAuthority(), Threshold: 3,
	}))
	answer, err = server.Attestors(goCtx, &types.QueryAttestorsRequest{})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Equal(t, want.Attestors, answer.Attestors)
	require.Equal(t, uint32(3), answer.Threshold)

	answer, err = server.Attestors(context.Background(), nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, answer)
}
