package keeper_test

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryFeeAccountingServes(t *testing.T) {
	e := newEnv(t)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)
	check := func(want sdk.Int) {
		t.Helper()
		liabilities, err := e.k.TotalLiabilities(e.ctx)
		require.NoError(t, err)
		require.Equal(t, want, liabilities)
		router.Ctx = e.ctx
		response, err := client.FeeAccounting(context.Background(), &types.QueryFeeAccountingRequest{})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, liabilities, response.Accounting.TotalLiabilities)
		require.Equal(t, sdk.ZeroInt(), response.Accounting.PendingProtocolFees)
	}
	check(sdk.ZeroInt())

	creator := e.account(100_000_000)
	trader := e.account(1_000_000_000)
	denom := e.create(creator, types.FeeStrategyClaim).Denom
	first := e.swap(trader, denom, true, 333_333_334)
	require.Positive(t, first.ProtocolCut.Sign())
	require.Positive(t, first.PoolCut.Sign())
	check(sdk.NewIntFromBigInt(first.PoolCut))

	second := e.swap(trader, denom, true, 3334)
	require.Positive(t, second.ProtocolCut.Sign())
	require.Positive(t, second.PoolCut.Sign())
	check(sdk.NewIntFromBigInt(new(big.Int).Add(first.PoolCut, second.PoolCut)))
}

func TestQueryFeeAccountingRefusals(t *testing.T) {
	e := newEnv(t)
	server := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)

	t.Run("nil request", func(t *testing.T) {
		response, err := server.FeeAccounting(context.Background(), nil)
		require.Nil(t, response)
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Equal(t, "empty request", status.Convert(err).Message())
	})

	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{name: "malformed liabilities", raw: []byte("invalid")},
		{name: "negative liabilities", raw: []byte("-1")},
		{name: "overflowing liabilities", raw: []byte(new(big.Int).Lsh(big.NewInt(1), 256).String())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := e.ctx.KVStore(e.key)
			key := []byte{0x49}
			before := store.Get(key)
			t.Cleanup(func() { store.Set(key, before) })
			store.Set(key, tc.raw)

			_, keeperErr := e.k.TotalLiabilities(e.ctx)
			require.ErrorIs(t, keeperErr, types.ErrOverflow)
			response, err := client.FeeAccounting(context.Background(), &types.QueryFeeAccountingRequest{})
			require.Nil(t, response)
			require.ErrorIs(t, err, types.ErrOverflow)
			require.EqualError(t, err, keeperErr.Error())
			require.Equal(t, keeperErr, err)
			require.Same(t, types.ErrOverflow, errors.Unwrap(err))
			var registeredErr *sdkerrors.Error
			require.ErrorAs(t, err, &registeredErr)
			codespace, code, _ := sdkerrors.ABCIInfo(registeredErr, false)
			require.Equal(t, types.ModuleName, codespace)
			require.Equal(t, types.ErrOverflow.ABCICode(), code)
			require.Equal(t, tc.raw, store.Get(key))
		})
	}
}
