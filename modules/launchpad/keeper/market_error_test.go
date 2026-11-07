package keeper_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestMustMarketKeepsUnknownMarketCode(t *testing.T) {
	e := newEnv(t)
	const denom = "unknown-market"
	querier := keeper.NewQuerier(e.k)
	for _, tc := range []struct {
		name  string
		isBuy bool
	}{
		{name: "buy", isBuy: true},
		{name: "sell", isBuy: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := querier.Quote(sdk.WrapSDKContext(e.ctx), &types.QueryQuoteRequest{
				Denom: denom, IsBuy: tc.isBuy, AmountIn: "1000000",
			})
			require.Nil(t, response)
			require.ErrorIs(t, err, types.ErrUnknownMarket)
			require.Same(t, types.ErrUnknownMarket, errors.Unwrap(err))
			require.EqualError(t, err, types.ErrUnknownMarket.Error()+": "+denom)
			cause, ok := err.(interface{ Cause() error })
			require.True(t, ok)
			require.Same(t, types.ErrUnknownMarket, cause.Cause())
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, "launchpad", codespace)
			require.Equal(t, uint32(12), code)
		})
	}

	_, err := e.k.QuoteBuy(e.ctx, denom, big.NewInt(1))
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	_, err = e.k.QuoteSell(e.ctx, denom, big.NewInt(1))
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	require.ErrorIs(t, e.k.TransferGuardian(e.ctx, nil, denom, nil), types.ErrUnknownMarket)
	require.ErrorIs(t, e.k.AcceptGuardian(e.ctx, nil, denom), types.ErrUnknownMarket)
}
