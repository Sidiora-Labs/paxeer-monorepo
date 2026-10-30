package app

import (
	"context"
	"errors"
	"fmt"
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
	app.SetCommitListener(func(height int64, appHash []byte) error {
		calls++
		gotHeight = height
		committedHeight = app.LastBlockHeight()
		gotHash = appHash
		return nil
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

func TestCommitReturnsListenerFailure(t *testing.T) {
	for _, panics := range []bool{false, true} {
		name := "returned error"
		if panics {
			name = "panic"
		}
		t.Run(name, func(t *testing.T) {
			app := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false).App
			ctx := context.Background()
			cause := errors.New("commit listener failed")
			calls := 0
			var gotHeight int64
			var gotHash []byte
			app.SetCommitListener(func(height int64, appHash []byte) error {
				calls++
				gotHeight, gotHash = height, appHash
				if panics {
					panic(cause.Error())
				}
				return cause
			})

			for height := int64(1); height <= 2; height++ {
				previousCommit := app.LastCommitID()
				finalized, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{
					Header: &tmproto.Header{ChainID: "pax-test", Height: height},
				})
				require.NoError(t, err)
				require.NotEmpty(t, finalized.AppHash)
				require.Equal(t, int(height-1), calls, "listener must wait for Commit")

				var res *abci.ResponseCommit
				require.NotPanics(t, func() {
					res, err = app.Commit(ctx)
				})
				require.Error(t, err)
				require.Contains(t, err.Error(), cause.Error())
				require.Contains(t, err.Error(), fmt.Sprintf("height %d", height))
				if !panics {
					require.ErrorIs(t, err, cause)
				}
				require.NotNil(t, res)
				require.Equal(t, int(height), calls)
				require.Equal(t, height, gotHeight)
				require.Equal(t, finalized.AppHash, gotHash)
				require.Equal(t, height, app.LastBlockHeight())
				require.Greater(t, app.LastCommitID().Version, previousCommit.Version)
				require.Equal(t, height, app.LastCommitID().Version)
				require.Equal(t, finalized.AppHash, app.LastCommitID().Hash)
			}
		})
	}
}
