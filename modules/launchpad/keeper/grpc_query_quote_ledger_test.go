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
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryQuoteLedgerServes(t *testing.T) {
	e := newEnv(t)
	owner, other := address("query ledger owner"), address("query ledger other")
	user := e.account(10_000)
	cases := []struct {
		name    string
		kind    string
		address string
		ledger  keeper.QuoteLedger
		amount  int64
	}{
		{"held", "held", owner.Hex(), keeper.HeldQuoteLedger(owner), 111},
		{"other held", "held", other.Hex(), keeper.HeldQuoteLedger(other), 222},
		{"fee-held", "fee-held", owner.Hex(), keeper.FeeHeldLedger(owner), 333},
		{"other fee-held", "fee-held", other.Hex(), keeper.FeeHeldLedger(other), 444},
		{"treasury-held", "treasury-held", "", keeper.TreasuryHeldLedger(), 555},
	}
	for _, tc := range cases {
		ctx, received, err := e.k.PullQuote(e.ctx, e.k.ModuleCaller(), e.evmAddress(user), tc.ledger, sdk.NewInt(tc.amount))
		require.NoError(t, err)
		e.ctx = ctx
		require.Equal(t, sdk.NewInt(tc.amount), received)
	}
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := e.k.GetQuoteLedger(e.ctx, tc.ledger)
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(tc.amount), want)
			response, err := client.QuoteLedger(context.Background(), &types.QueryQuoteLedgerRequest{Kind: tc.kind, Address: tc.address})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, want.String(), response.Amount)
		})
	}
	t.Run("treasury ignores address", func(t *testing.T) {
		response, err := client.QuoteLedger(context.Background(), &types.QueryQuoteLedgerRequest{Kind: "treasury-held", Address: "unused"})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, e.quoteLedger(keeper.TreasuryHeldLedger()).String(), response.Amount)
	})
	for _, kind := range []string{"held", "fee-held"} {
		t.Run("empty "+kind, func(t *testing.T) {
			response, err := client.QuoteLedger(context.Background(), &types.QueryQuoteLedgerRequest{Kind: kind, Address: address("query ledger absent").Hex()})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, sdk.ZeroInt().String(), response.Amount)
		})
	}
}

func TestQueryQuoteLedgerRefusals(t *testing.T) {
	e := newEnv(t)
	server := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)
	t.Run("nil request", func(t *testing.T) {
		response, err := server.QuoteLedger(context.Background(), nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	for _, kind := range []string{"", "unknown", "HELD", "fee_held"} {
		t.Run("unknown kind "+kind, func(t *testing.T) {
			response, err := client.QuoteLedger(context.Background(), &types.QueryQuoteLedgerRequest{Kind: kind, Address: address("query ledger owner").Hex()})
			require.Nil(t, response)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
		})
	}
	for _, kind := range []string{"held", "fee-held"} {
		for _, owner := range []string{"", "not-an-address", "0x1234", "0x" + strings.Repeat("g", 40), "0x" + strings.Repeat("1", 42)} {
			t.Run(kind+" invalid address "+owner, func(t *testing.T) {
				response, err := client.QuoteLedger(context.Background(), &types.QueryQuoteLedgerRequest{Kind: kind, Address: owner})
				require.Nil(t, response)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
			})
		}
	}
	t.Run("keeper error preserved", func(t *testing.T) {
		owner := address("query ledger corrupt")
		e.ctx.KVStore(e.key).Set(types.HeldQuoteKey(owner), []byte("invalid amount"))
		_, keeperErr := e.k.GetQuoteLedger(e.ctx, keeper.HeldQuoteLedger(owner))
		require.ErrorIs(t, keeperErr, types.ErrOverflow)
		response, err := client.QuoteLedger(context.Background(), &types.QueryQuoteLedgerRequest{Kind: "held", Address: owner.Hex()})
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
	})
}
