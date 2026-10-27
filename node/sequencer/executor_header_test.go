package sequencer_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestOpenHeaderCarriesCommittedAppHash(t *testing.T) {
	h := newHarness(t)
	committed := h.app.CommitMultiStore().LastCommitID()
	require.Equal(t, committedTip, committed.Version)
	require.NotEmpty(t, committed.Hash)
	want := append([]byte(nil), committed.Hash...)

	h.open()
	require.Equal(t, want, h.openCtx().BlockHeader().AppHash)

	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, want, batch.Ctx.BlockHeader().AppHash)
}

func TestOpenHeaderCarriesLastBlockID(t *testing.T) {
	tm := time.Now().UTC()
	valPub := secp256k1.GenPrivKey().PubKey()
	tw := app.NewTestWrapper(t, tm, valPub, false)
	a := tw.App
	validatorsHash := sha256.Sum256(valPub.Bytes())
	block := tmtypes.MakeBlock(committedTip, nil, &tmtypes.Commit{}, nil)
	block.ChainID = testChainID
	block.Time = tm
	block.ProposerAddress = valPub.Address().Bytes()
	block.ValidatorsHash = validatorsHash[:]
	block.NextValidatorsHash = validatorsHash[:]
	parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
	require.NoError(t, err)
	id := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	want := id.ToProto()
	require.NotEmpty(t, want.Hash)
	require.Positive(t, want.PartSetHeader.Total)
	require.NotEmpty(t, want.PartSetHeader.Hash)

	block.LastBlockID = id
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Hash: block.Hash(), Header: block.Header.ToProto(),
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, committedTip, a.LastBlockHeight())
	require.Equal(t, want, a.GetCheckCtx().BlockHeader().LastBlockId)
	require.Equal(t, want, a.LastBlockID())

	exec, err := sequencer.NewExecutor(a, sequencer.Options{
		ChainID: testChainID, ProposerAddress: valPub.Address().Bytes(),
	})
	require.NoError(t, err)
	require.NoError(t, exec.Open(openHeight, tm.Add(time.Second).UnixMilli()))
	ctx, err := exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, want, ctx.BlockHeader().LastBlockId)

	batch, err := exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, want, batch.Ctx.BlockHeader().LastBlockId)
}
