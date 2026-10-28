package keeper_test

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

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

func TestQueryQuoteServes(t *testing.T) {
	e := newEnv(t)
	creator := e.account(100_000_000)
	trader := e.account(200_000_000)
	denom := e.create(creator, types.FeeStrategyClaim).Denom
	e.swap(trader, denom, true, 100_000_000)
	e.at(time.Minute)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	for _, tc := range []struct {
		name   string
		isBuy  bool
		amount string
	}{
		{"buy", true, "2000000"},
		{"sell", false, "10000000000"},
		{"decimal leading zeros", true, "0000002000008"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := storeEntries(e.ctx, e.key)
			quoteBalance := e.usdlOf(trader)
			tokenBalance := e.balance(trader, denom)
			poolBalance := e.tokenBalance(e.token(denom), e.pool(denom))
			custodyBalance := e.custodyUsdl()
			eventCount := len(e.ctx.EventManager().Events())
			amount, ok := new(big.Int).SetString(tc.amount, 10)
			require.True(t, ok)
			var expected keeper.SwapResult
			var err error
			if tc.isBuy {
				expected, err = e.k.QuoteBuy(e.ctx, denom, amount)
			} else {
				expected, err = e.k.QuoteSell(e.ctx, denom, amount)
			}
			require.NoError(t, err)
			response, err := client.Quote(context.Background(), &types.QueryQuoteRequest{
				Denom: denom, IsBuy: tc.isBuy, AmountIn: tc.amount,
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, sdk.NewIntFromBigInt(expected.AmountIn), response.Quote.AmountIn)
			require.Equal(t, sdk.NewIntFromBigInt(expected.AmountOut), response.Quote.AmountOut)
			require.Equal(t, sdk.NewIntFromBigInt(expected.FeeBps), response.Quote.FeeBps)
			require.Equal(t, sdk.NewIntFromBigInt(expected.FeeAmount), response.Quote.FeeAmount)
			require.Equal(t, sdk.NewIntFromBigInt(expected.ProtocolCut), response.Quote.ProtocolCut)
			require.Equal(t, sdk.NewIntFromBigInt(expected.PoolCut), response.Quote.PoolCut)
			require.Equal(t, before, storeEntries(e.ctx, e.key))
			require.Equal(t, eventCount, len(e.ctx.EventManager().Events()))
			require.Equal(t, quoteBalance, e.usdlOf(trader))
			require.Equal(t, tokenBalance, e.balance(trader, denom))
			require.Equal(t, poolBalance, e.tokenBalance(e.token(denom), e.pool(denom)))
			require.Equal(t, custodyBalance, e.custodyUsdl())
		})
	}
}

func TestQueryQuoteRefusals(t *testing.T) {
	e := newEnv(t)
	creator := e.account(100_000_000)
	denom := e.create(creator, types.FeeStrategyClaim).Denom
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)
	before := storeEntries(e.ctx, e.key)

	for _, goCtx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
		response, err := querier.Quote(goCtx, nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	for _, isBuy := range []bool{true, false} {
		for _, amount := range []string{
			"", "0", "000", "-1", "+1", "1.5", "1e3", " 1", "1 ", "0x10", "0b10", "0o10", "1_000", "abc", "１２",
			new(big.Int).Add(types.MaxUint256(), big.NewInt(1)).String(), strings.Repeat("9", 301),
		} {
			response, err := client.Quote(context.Background(), &types.QueryQuoteRequest{
				Denom: denom, IsBuy: isBuy, AmountIn: amount,
			})
			require.Nil(t, response)
			require.Equal(t, codes.InvalidArgument, status.Code(err), "amount %q, buy %t", amount, isBuy)
		}
		response, err := client.Quote(context.Background(), &types.QueryQuoteRequest{
			Denom: "unknown-market", IsBuy: isBuy, AmountIn: "1000000",
		})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrUnknownMarket)
		require.Same(t, types.ErrUnknownMarket, errors.Unwrap(err))
		require.EqualError(t, err, types.ErrUnknownMarket.Error()+": unknown-market")
		var launchpadErr *sdkerrors.Error
		require.ErrorAs(t, err, &launchpadErr)
		codespace, code, _ := sdkerrors.ABCIInfo(launchpadErr, false)
		require.Equal(t, types.ErrUnknownMarket.Codespace(), codespace)
		require.Equal(t, types.ErrUnknownMarket.ABCICode(), code)
	}
	response, err := client.Quote(context.Background(), &types.QueryQuoteRequest{
		Denom: denom, IsBuy: false, AmountIn: "10000000000",
	})
	require.Nil(t, response)
	require.ErrorIs(t, err, types.ErrVirtualFloorBreached)
	require.Same(t, types.ErrVirtualFloorBreached, err)
	codespace, code, _ := sdkerrors.ABCIInfo(err, false)
	require.Equal(t, types.ErrVirtualFloorBreached.Codespace(), codespace)
	require.Equal(t, types.ErrVirtualFloorBreached.ABCICode(), code)
	require.Equal(t, before, storeEntries(e.ctx, e.key))
}
