package app

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestLastBlockIDIsTheCommittedBlock(t *testing.T) {
	tm := time.Now().UTC()
	valPub := secp256k1.GenPrivKey().PubKey()
	a := NewTestWrapper(t, tm, valPub, false).App
	validatorsHash := sha256.Sum256(valPub.Bytes())
	parent := tmtypes.BlockID{}
	require.Equal(t, parent.ToProto(), a.LastBlockID())

	for height := int64(1); height <= 2; height++ {
		previous := a.LastBlockID()
		block := tmtypes.MakeBlock(height, nil, &tmtypes.Commit{}, nil)
		block.ChainID = "pax-test"
		block.Time = tm.Add(time.Duration(height) * time.Second)
		block.ProposerAddress = valPub.Address().Bytes()
		block.ValidatorsHash = validatorsHash[:]
		block.NextValidatorsHash = validatorsHash[:]
		block.AppHash = a.LastCommitID().Hash
		block.LastBlockID = parent
		parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
		require.NoError(t, err)
		id := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
		want := id.ToProto()
		require.NotEmpty(t, want.Hash)
		require.Positive(t, want.PartSetHeader.Total)
		require.NotEmpty(t, want.PartSetHeader.Hash)
		require.NotEqual(t, parent.ToProto(), want)
		want.PartSetHeader = tmproto.PartSetHeader{}

		_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Hash: block.Hash(), Header: block.Header.ToProto(),
		})
		require.NoError(t, err)
		require.Equal(t, previous, a.LastBlockID(), "FinalizeBlock must not publish an uncommitted ID")
		a.SetDeliverStateToCommit()
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, height, a.LastBlockHeight())
		require.Equal(t, parent.ToProto(), a.GetCheckCtx().BlockHeader().LastBlockId)
		require.Equal(t, want, a.LastBlockID(), "height %d must identify the committed block", height)

		got := a.LastBlockID()
		got.Hash[0] ^= 0xff
		got.PartSetHeader.Total = 1
		got.PartSetHeader.Hash = []byte{1}
		require.Equal(t, want, a.LastBlockID(), "callers must not mutate the committed ID")
		parent = id
	}
}
