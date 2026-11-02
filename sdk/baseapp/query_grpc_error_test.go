package baseapp

import (
	"context"
	"errors"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

var errQueryUnknownMarket = sdkerrors.Register("baseapptest", 12, "unknown market")

func TestHandleQueryGRPCKeepsModuleCodespace(t *testing.T) {
	app := NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{})

	app.setDeliverState(tmproto.Header{ChainID: app.ChainID, Height: 1})
	app.SetDeliverStateToCommit()
	_, err := app.Commit(context.Background())
	require.NoError(t, err)
	ctx, err := app.CreateQueryContext(1, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), ctx.BlockHeight())

	testCases := []struct {
		name      string
		err       error
		codespace string
		code      uint32
	}{
		{
			name:      "registered module error",
			err:       errQueryUnknownMarket,
			codespace: "baseapptest",
			code:      12,
		},
		{
			name:      "wrapped module error",
			err:       sdkerrors.Wrap(errQueryUnknownMarket, "query market"),
			codespace: "baseapptest",
			code:      12,
		},
		{
			name:      "plain error",
			err:       errors.New("plain"),
			codespace: sdkerrors.ErrInvalidRequest.Codespace(),
			code:      sdkerrors.ErrInvalidRequest.ABCICode(),
		},
		{
			name:      "grpc not found",
			err:       grpcstatus.Error(codes.NotFound, "missing"),
			codespace: sdkerrors.ErrKeyNotFound.Codespace(),
			code:      sdkerrors.ErrKeyNotFound.ABCICode(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var handler GRPCQueryHandler = func(_ sdk.Context, _ abci.RequestQuery) (abci.ResponseQuery, error) {
				return abci.ResponseQuery{}, tc.err
			}
			res := app.handleQueryGRPC(handler, abci.RequestQuery{Height: 1})
			require.Equal(t, tc.codespace, res.Codespace)
			require.Equal(t, tc.code, res.Code)
		})
	}
}
