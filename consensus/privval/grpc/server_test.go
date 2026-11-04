package grpc_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testrequire "github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmrand "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/rand"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmgrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval/grpc"
	privvalproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

const ChainID = "123"

var testKey = ed25519.TestSecretKey([]byte("test"))

func TestGetPubKey(t *testing.T) {

	testCases := []struct {
		name string
		pv   types.PrivValidator
		err  bool
	}{
		{name: "valid", pv: types.NewMockPV(), err: false},
		{name: "error on pubkey", pv: types.NewErroringMockPV(), err: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			s := tmgrpc.NewSignerServer(ChainID, tc.pv)

			req := &privvalproto.PubKeyRequest{ChainId: ChainID}
			resp, err := s.GetPubKey(ctx, req)
			if tc.err {
				require.Error(t, err)
			} else {
				pk, err := tc.pv.GetPubKey(ctx)
				require.NoError(t, err)
				require.Equal(t, resp.PubKey, crypto.PubKeyToProto(pk))
			}
		})
	}

}

func TestSignVote(t *testing.T) {

	ts := time.Now()
	hash := tmrand.Bytes(crypto.HashSize)
	valAddr := tmrand.Bytes(crypto.AddressSize)

	testCases := []struct {
		name       string
		pv         types.PrivValidator
		have, want *types.Vote
		err        bool
	}{
		{name: "valid", pv: types.NewMockPV(), have: &types.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
		}, want: &types.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
		},
			err: false},
		{name: "invalid vote", pv: types.NewErroringMockPV(), have: &types.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
			Signature:        utils.Some(testKey.Sign([]byte("signed"))),
		}, want: &types.Vote{
			Type:             tmproto.PrecommitType,
			Height:           1,
			Round:            2,
			BlockID:          types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp:        ts,
			ValidatorAddress: valAddr,
			ValidatorIndex:   1,
			Signature:        utils.Some(testKey.Sign([]byte("signed"))),
		},
			err: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			s := tmgrpc.NewSignerServer(ChainID, tc.pv)

			req := &privvalproto.SignVoteRequest{ChainId: ChainID, Vote: tc.have.ToProto()}
			resp, err := s.SignVote(ctx, req)
			if tc.err {
				require.Error(t, err)
			} else {
				pbVote := tc.want.ToProto()

				require.NoError(t, tc.pv.SignVote(ctx, ChainID, pbVote))

				assert.Equal(t, pbVote.Signature, resp.Vote.Signature)
			}
		})
	}
}

func TestSignProposal(t *testing.T) {

	ts := time.Now()
	hash := tmrand.Bytes(crypto.HashSize)

	testCases := []struct {
		name       string
		pv         types.PrivValidator
		have, want *types.Proposal
		err        bool
	}{
		{name: "valid", pv: types.NewMockPV(), have: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
		}, want: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
		},
			err: false},
		{name: "invalid proposal", pv: types.NewErroringMockPV(), have: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
			Signature: testKey.Sign([]byte("signed")),
		}, want: &types.Proposal{
			Type:      tmproto.ProposalType,
			Height:    1,
			Round:     2,
			POLRound:  2,
			BlockID:   types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Hash: hash, Total: 2}},
			Timestamp: ts,
			Signature: testKey.Sign([]byte("signed")),
		},
			err: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			s := tmgrpc.NewSignerServer(ChainID, tc.pv)

			req := &privvalproto.SignProposalRequest{ChainId: ChainID, Proposal: tc.have.ToProto()}
			resp, err := s.SignProposal(ctx, req)
			if tc.err {
				require.Error(t, err)
			} else {
				pbProposal := tc.want.ToProto()
				require.NoError(t, tc.pv.SignProposal(ctx, ChainID, pbProposal))
				assert.Equal(t, pbProposal.Signature, resp.Proposal.Signature)
			}
		})
	}
}

func testSignerServerSafetyState(t *testing.T) {
	t.Helper()
	for _, kind := range []string{"vote", "proposal"} {
		t.Run(kind, func(t *testing.T) {
			pv, files := repair171FilePV(t)
			const requests = 16
			var arrived atomic.Int32
			release := make(chan struct{})
			address, stop := repairM1085Server(t, pv, grpc.UnaryInterceptor(
				func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
					if arrived.Add(1) == requests {
						close(release)
					}
					select {
					case <-release:
						return handler(ctx, req)
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}))
			conn := repairM1085Connection(t, address)
			client, err := tmgrpc.NewSignerClient(conn, chainID)
			testrequire.NoError(t, err)
			votes := make([]*tmproto.Vote, requests)
			proposals := make([]*tmproto.Proposal, requests)
			results := make([]error, requests)
			for i := range requests {
				votes[i] = repair171Vote(pv, 10, byte(i))
				proposals[i] = repair171Proposal(10, byte(i))
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			testrequire.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
				for i := range requests {
					s.Spawn(func() error {
						if kind == "vote" {
							results[i] = client.SignVote(ctx, chainID, votes[i])
						} else {
							results[i] = client.SignProposal(ctx, chainID, proposals[i])
						}
						return nil
					})
				}
				return nil
			}))
			winner := -1
			var wantSig, wantBytes []byte
			for i, err := range results {
				var sig, signBytes []byte
				if kind == "vote" {
					sig, signBytes = votes[i].Signature, types.VoteSignBytes(chainID, votes[i])
				} else {
					sig, signBytes = proposals[i].Signature, types.ProposalSignBytes(chainID, proposals[i])
				}
				if err != nil {
					testrequire.Equal(t, codes.InvalidArgument, status.Code(err))
					testrequire.Empty(t, sig)
					continue
				}
				testrequire.Equal(t, -1, winner, "conflicting messages must not both be signed")
				winner, wantSig, wantBytes = i, sig, signBytes
				repair171Verify(t, pv.Key.PubKey, signBytes, sig)
			}
			testrequire.NotEqual(t, -1, winner)
			before, err := os.ReadFile(files.state)
			testrequire.NoError(t, err)
			wire := privvalproto.NewPrivValidatorAPIClient(conn)
			for _, wrongChain := range []string{"", "other-chain"} {
				pub, err := wire.GetPubKey(ctx, &privvalproto.PubKeyRequest{ChainId: wrongChain})
				testrequire.Equal(t, codes.InvalidArgument, status.Code(err))
				testrequire.Nil(t, pub)
				vote, err := wire.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: wrongChain, Vote: repair171Vote(pv, 100, 1)})
				testrequire.Equal(t, codes.InvalidArgument, status.Code(err))
				testrequire.Nil(t, vote)
				proposal, err := wire.SignProposal(ctx, &privvalproto.SignProposalRequest{ChainId: wrongChain, Proposal: repair171Proposal(100, 1)})
				testrequire.Equal(t, codes.InvalidArgument, status.Code(err))
				testrequire.Nil(t, proposal)
			}
			after, err := os.ReadFile(files.state)
			testrequire.NoError(t, err)
			testrequire.Equal(t, before, after)
			testrequire.NoError(t, client.Close())
			stop()
			reopened, err := privval.LoadFilePV(files.key, files.state)
			testrequire.NoError(t, err)
			testrequire.Equal(t, int64(10), reopened.LastSignState.Height)
			testrequire.Equal(t, int32(1), reopened.LastSignState.Round)
			if kind == "vote" {
				testrequire.Equal(t, int8(3), reopened.LastSignState.Step)
			} else {
				testrequire.Equal(t, int8(1), reopened.LastSignState.Step)
			}
			testrequire.Equal(t, wantSig, reopened.LastSignState.Signature)
			testrequire.Equal(t, wantBytes, []byte(reopened.LastSignState.SignBytes))
			address, _ = repairM1085Server(t, reopened)
			conn = repairM1085Connection(t, address)
			client, err = tmgrpc.NewSignerClient(conn, chainID)
			testrequire.NoError(t, err)
			for i := range requests {
				var sig []byte
				if kind == "vote" {
					vote := repair171Vote(reopened, 10, byte(i))
					err = client.SignVote(ctx, chainID, vote)
					sig = vote.Signature
				} else {
					proposal := repair171Proposal(10, byte(i))
					err = client.SignProposal(ctx, chainID, proposal)
					sig = proposal.Signature
				}
				if i == winner {
					testrequire.NoError(t, err)
					testrequire.Equal(t, wantSig, sig)
				} else {
					testrequire.Equal(t, codes.InvalidArgument, status.Code(err))
					testrequire.Empty(t, sig)
				}
			}
			after, err = os.ReadFile(files.state)
			testrequire.NoError(t, err)
			testrequire.Equal(t, before, after)
			next := repair171Vote(reopened, 11, 1)
			testrequire.NoError(t, client.SignVote(ctx, chainID, next))
			repair171Verify(t, reopened.Key.PubKey, types.VoteSignBytes(chainID, next), next.Signature)
		})
	}
}
