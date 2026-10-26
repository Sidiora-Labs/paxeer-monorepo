package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestBroadcastEvidenceUnderAutobahnRefused(t *testing.T) {
	const chainID = "autobahn-evidence-test"
	kv := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, kv.Close()) })
	app := proxy.New(kv, proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	router, _ := newAutobahnTestRouter(t, utils.TestRng(), chainID, app, stateStore, 1, 1_000)
	env := &Environment{Router: router}
	require.True(t, env.gigaRouter().IsPresent())
	require.False(t, env.EvidencePool.IsPresent())

	key := ed25519.TestSecretKey([]byte(chainID))
	validator := types.NewValidator(key.Public(), 10)
	validators := types.NewValidatorSet([]*types.Validator{validator})
	timestamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	voteA := &types.Vote{
		Type:             tmproto.PrevoteType,
		Height:           1,
		Timestamp:        timestamp,
		ValidatorAddress: validator.Address,
		ValidatorIndex:   0,
		BlockID: types.BlockID{
			Hash: crypto.Checksum([]byte("block-a")).Bytes(),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  crypto.Checksum([]byte("parts")).Bytes(),
			},
		},
	}
	voteB := voteA.Copy()
	voteB.BlockID.Hash = crypto.Checksum([]byte("block-b")).Bytes()
	for _, vote := range []*types.Vote{voteA, voteB} {
		vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(chainID, vote.ToProto())))
		require.NoError(t, vote.Verify(chainID, key.Public()))
	}
	evidence, err := types.NewDuplicateVoteEvidence(voteA, voteB, timestamp, validators)
	require.NoError(t, err)
	require.NoError(t, evidence.ValidateBasic())

	for _, tc := range []struct {
		name     string
		evidence types.Evidence
	}{
		{name: "valid", evidence: evidence},
		{name: "missing"},
		{name: "malformed", evidence: &types.DuplicateVoteEvidence{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := env.BroadcastEvidence(t.Context(), &coretypes.RequestBroadcastEvidence{Evidence: tc.evidence})
			require.ErrorIs(t, err, coretypes.ErrEvidenceNotAccepted)
			require.Nil(t, result)
		})
	}
}
