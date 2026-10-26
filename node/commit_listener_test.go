package app

import (
	"context"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestCommitCallsListenerWithHeightAndAppHash(t *testing.T) {
	app := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false).App
	ctx := context.Background()
	calls := 0
	var gotHeight, committedHeight int64
	var gotHash []byte
	app.SetCommitListener(func(height int64, appHash []byte) {
		calls++
		gotHeight = height
		committedHeight = app.LastBlockHeight()
		gotHash = appHash
	})

	for height := int64(1); height <= 2; height++ {
		finalized, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: "pax-test", Height: height},
		})
		require.NoError(t, err)
		require.NotEmpty(t, finalized.AppHash)
		require.Equal(t, int(height-1), calls, "listener must wait for Commit")

		res, err := app.Commit(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, int(height), calls, "listener must run once per Commit")
		require.Equal(t, height, gotHeight)
		require.Equal(t, height, committedHeight, "listener must see committed state")
		require.Equal(t, finalized.AppHash, gotHash)
		require.Equal(t, finalized.AppHash, app.LastCommitID().Hash)
	}
}

func TestCommitSurvivesPanickingListener(t *testing.T) {
	app := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false).App
	ctx := context.Background()
	calls := 0
	var gotHeight int64
	var gotHash []byte
	app.SetCommitListener(func(height int64, appHash []byte) {
		calls++
		gotHeight, gotHash = height, appHash
		panic("commit listener failed")
	})

	for height := int64(1); height <= 2; height++ {
		finalized, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: "pax-test", Height: height},
		})
		require.NoError(t, err)
		require.NotEmpty(t, finalized.AppHash)

		var res *abci.ResponseCommit
		require.NotPanics(t, func() {
			res, err = app.Commit(ctx)
		})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, int(height), calls)
		require.Equal(t, height, gotHeight)
		require.Equal(t, finalized.AppHash, gotHash)
		require.Equal(t, height, app.LastBlockHeight())
		require.Equal(t, finalized.AppHash, app.LastCommitID().Hash)
	}
}
