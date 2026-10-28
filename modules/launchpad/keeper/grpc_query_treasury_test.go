package keeper_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryTreasuryServes(t *testing.T) {
	e := newEnv(t)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	response, err := client.Treasury(context.Background(), &types.QueryTreasuryRequest{Token: e.usdl.Hex()})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, sdk.ZeroInt().String(), response.Balance)
	require.Equal(t, sdk.ZeroInt().String(), response.Held)

	depositor := e.account(900)
	ctx, received, err := e.k.PullQuote(e.ctx, e.k.ModuleCaller(), e.evmAddress(depositor), keeper.TreasuryHeldLedger(), sdk.NewInt(900))
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(900), received)
	e.ctx = ctx
	require.NoError(t, e.k.TreasuryCreditFee(e.ctx, sdk.NewInt(600)))
	router.Ctx = e.ctx

	balance, err := e.k.TreasuryGetBalance(e.ctx, e.usdl)
	require.NoError(t, err)
	held, err := e.k.TreasuryHeld(e.ctx, e.usdl)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(600), balance)
	require.Equal(t, sdk.NewInt(900), held)
	for _, token := range []string{e.usdl.Hex(), strings.ToLower(e.usdl.Hex())} {
		response, err = client.Treasury(context.Background(), &types.QueryTreasuryRequest{Token: token})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, balance.String(), response.Balance)
		require.Equal(t, held.String(), response.Held)
	}

	otherToken := address("unfunded treasury token")
	response, err = client.Treasury(context.Background(), &types.QueryTreasuryRequest{Token: otherToken.Hex()})
	require.NoError(t, err)
	require.NotNil(t, response)
	otherBalance, err := e.k.TreasuryGetBalance(e.ctx, otherToken)
	require.NoError(t, err)
	otherHeld, err := e.k.TreasuryHeld(e.ctx, otherToken)
	require.NoError(t, err)
	require.Equal(t, sdk.ZeroInt(), otherBalance)
	require.Equal(t, sdk.ZeroInt(), otherHeld)
	require.Equal(t, otherBalance.String(), response.Balance)
	require.Equal(t, otherHeld.String(), response.Held)

	creationFee := e.k.GetParams(e.ctx).CreationFee
	e.create(e.account(200_000_000), types.FeeStrategyClaim)
	router.Ctx = e.ctx
	response, err = client.Treasury(context.Background(), &types.QueryTreasuryRequest{Token: e.usdl.Hex()})
	require.NoError(t, err)
	require.NotNil(t, response)
	createdBalance, err := e.k.TreasuryGetBalance(e.ctx, e.usdl)
	require.NoError(t, err)
	require.Equal(t, balance.Add(creationFee), createdBalance)
	require.Equal(t, held.Add(creationFee), e.treasury())
	require.Equal(t, createdBalance.String(), response.Balance)
	require.Equal(t, e.treasury().String(), response.Held)
}

func TestQueryTreasuryRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	for _, goCtx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
		response, err := querier.Treasury(goCtx, nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}

	cases := []struct {
		name  string
		token string
		want  *sdkerrors.Error
	}{
		{name: "empty", token: "", want: sdkerrors.ErrInvalidAddress},
		{name: "short", token: "0x1234", want: sdkerrors.ErrInvalidAddress},
		{name: "missing prefix", token: strings.TrimPrefix(e.usdl.Hex(), "0x"), want: sdkerrors.ErrInvalidAddress},
		{name: "non hex", token: "0xgg00000000000000000000000000000000000000", want: sdkerrors.ErrInvalidAddress},
		{name: "zero", token: (common.Address{}).Hex(), want: types.ErrZeroAddress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.Treasury(context.Background(), &types.QueryTreasuryRequest{Token: tc.token})
			require.Nil(t, response)
			require.ErrorIs(t, err, tc.want)
			require.Same(t, tc.want, errors.Unwrap(err))
			var registeredErr *sdkerrors.Error
			require.ErrorAs(t, err, &registeredErr)
			codespace, code, _ := sdkerrors.ABCIInfo(registeredErr, false)
			require.Equal(t, tc.want.Codespace(), codespace)
			require.Equal(t, tc.want.ABCICode(), code)
		})
	}

	for _, tc := range []struct {
		name string
		key  []byte
		read func(sdk.Context, common.Address) (sdk.Int, error)
	}{
		{name: "balance error", key: types.TreasuryLedgerKey(e.usdl), read: e.k.TreasuryGetBalance},
		{name: "custody error", key: types.TreasuryHeldKey(e.usdl), read: e.k.TreasuryHeld},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := e.ctx.CacheContext()
			invalid, err := sdk.NewInt(-1).Marshal()
			require.NoError(t, err)
			ctx.KVStore(e.key).Set(tc.key, invalid)
			_, keeperErr := tc.read(ctx, e.usdl)
			require.ErrorIs(t, keeperErr, types.ErrOverflow)
			router.Ctx = ctx
			response, err := client.Treasury(context.Background(), &types.QueryTreasuryRequest{Token: e.usdl.Hex()})
			require.Nil(t, response)
			require.ErrorIs(t, err, types.ErrOverflow)
			require.EqualError(t, err, keeperErr.Error())
			require.Equal(t, keeperErr, err)
			require.Same(t, types.ErrOverflow, errors.Unwrap(err))
			var registeredErr *sdkerrors.Error
			require.ErrorAs(t, err, &registeredErr)
			codespace, code, _ := sdkerrors.ABCIInfo(registeredErr, false)
			require.Equal(t, types.ErrOverflow.Codespace(), codespace)
			require.Equal(t, types.ErrOverflow.ABCICode(), code)
		})
	}
}
