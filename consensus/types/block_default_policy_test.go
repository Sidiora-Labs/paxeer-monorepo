//go:build !mock_block_validation && !mock_chain_validation

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmrand "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/rand"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
)

func TestBlockValidateBasic(t *testing.T) {
	ctx := t.Context()

	require.Error(t, (*Block)(nil).ValidateBasic(DefaultConsensusPolicy()))

	txs := []Tx{Tx("foo"), Tx("bar")}
	lastID := makeBlockIDRandom()
	h := int64(3)

	voteSet, valSet, vals := randVoteSet(ctx, t, h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := MakeCommit(ctx, lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	ev, err := NewMockDuplicateVoteEvidenceWithValidator(ctx, h, time.Now(), vals[0], "block-test-chain")
	require.NoError(t, err)
	evList := []Evidence{ev}

	testCases := []struct {
		testName      string
		malleateBlock func(*Block)
		expErr        bool
	}{
		{"Make Block", func(blk *Block) {}, false},
		{"Make Block w/ proposer Addr", func(blk *Block) { blk.ProposerAddress = valSet.GetProposer().Address }, false},
		{"Negative Height", func(blk *Block) { blk.Height = -1 }, true},
		{"Remove 1/2 the commits", func(blk *Block) {
			blk.LastCommit.Signatures = commit.Signatures[:commit.Size()/2]
			blk.LastCommit.hash = nil // clear hash or change wont be noticed
		}, true},
		{"Remove LastCommitHash", func(blk *Block) { blk.LastCommitHash = []byte("something else") }, true},
		{"Tampered Data", func(blk *Block) {
			blk.Data.Txs[0] = Tx("something else")
			blk.Data.hash = nil // clear hash or change wont be noticed
		}, true},
		{"Tampered DataHash", func(blk *Block) {
			blk.DataHash = tmrand.Bytes(len(blk.DataHash))
		}, true},
		{"Tampered EvidenceHash", func(blk *Block) {
			blk.EvidenceHash = tmrand.Bytes(len(blk.EvidenceHash))
		}, true},
		{"Incorrect block protocol version", func(blk *Block) {
			blk.Version.Block = 1
		}, true},
		{"Missing LastCommit", func(blk *Block) {
			blk.LastCommit = nil
		}, true},
		{"Invalid LastCommit", func(blk *Block) {
			blk.LastCommit = &Commit{
				Height:  -1,
				BlockID: *voteSet.maj23,
			}
		}, true},
		{"Invalid Evidence", func(blk *Block) {
			emptyEv := &DuplicateVoteEvidence{}
			blk.Evidence = []Evidence{emptyEv}
		}, true},
	}
	for i, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			block := MakeBlock(h, txs, commit, evList)
			block.ProposerAddress = valSet.GetProposer().Address
			tc.malleateBlock(block)
			err = block.ValidateBasic(DefaultConsensusPolicy())
			t.Log(err)
			assert.Equal(t, tc.expErr, err != nil, "#%d: %v", i, err)
		})
	}
}
