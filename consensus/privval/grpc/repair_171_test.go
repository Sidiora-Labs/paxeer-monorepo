package grpc_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmgrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval/grpc"
	privvalproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type repair171Files struct {
	key, state string
}

func repair171FilePV(t *testing.T) (*privval.FilePV, repair171Files) {
	t.Helper()
	dir := t.TempDir()
	files := repair171Files{filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json")}
	pv, err := privval.LoadOrGenFilePV(files.key, files.state)
	require.NoError(t, err)
	return pv, files
}

func repair171RPC(t *testing.T, pv *privval.FilePV, chain string, opts ...grpc.ServerOption) (*grpc.ClientConn, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(opts...)
	privvalproto.RegisterPrivValidatorAPIServer(server, tmgrpc.NewSignerServer(chain, pv))
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	var once sync.Once
	var conn *grpc.ClientConn
	stop := func() {
		once.Do(func() {
			if conn != nil {
				require.NoError(t, conn.Close())
			}
			server.Stop()
			err := <-served
			if !errors.Is(err, grpc.ErrServerStopped) {
				require.NoError(t, err)
			}
		})
	}
	t.Cleanup(stop)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	conn, err = grpc.DialContext(ctx, listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	return conn, stop
}

func repair171Vote(pv *privval.FilePV, height int64, block byte) *tmproto.Vote {
	hash := sha256.Sum256([]byte{block})
	return (&types.Vote{
		Type: tmproto.PrecommitType, Height: height, Round: 1,
		BlockID:   types.BlockID{Hash: hash[:], PartSetHeader: types.PartSetHeader{Total: 1, Hash: hash[:]}},
		Timestamp: time.Unix(100, 0).UTC(), ValidatorAddress: pv.GetAddress(), ValidatorIndex: 0,
	}).ToProto()
}

func repair171Proposal(height int64, block byte) *tmproto.Proposal {
	hash := sha256.Sum256([]byte{block})
	proposal := (&types.Proposal{
		Type: tmproto.ProposalType, Height: height, Round: 1, POLRound: -1,
		BlockID:   types.BlockID{Hash: hash[:], PartSetHeader: types.PartSetHeader{Total: 1, Hash: hash[:]}},
		Timestamp: time.Unix(100, 0).UTC(),
	}).ToProto()
	// ToProto serializes the zero-value fixed-size signature as 64 zero bytes.
	// Signing requests carry an unsigned proposal, with no signature attached.
	proposal.Signature = nil
	return proposal
}

func repair171Verify(t *testing.T, pk crypto.PubKey, signBytes, signature []byte) {
	t.Helper()
	sig, err := ed25519.SignatureFromBytes(signature)
	require.NoError(t, err)
	require.NoError(t, pk.Verify(signBytes, sig))
}

func TestRepair_171_ConcurrentFilePV(t *testing.T) {
	for _, kind := range []string{"vote", "proposal", "mixed"} {
		t.Run(kind, func(t *testing.T) {
			pv, files := repair171FilePV(t)
			const requests = 32
			var arrived atomic.Int32
			release := make(chan struct{})
			conn, stop := repair171RPC(t, pv, chainID, grpc.UnaryInterceptor(
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
			client, err := tmgrpc.NewSignerClient(conn, chainID)
			require.NoError(t, err)
			votes := make([]*tmproto.Vote, requests)
			proposals := make([]*tmproto.Proposal, requests)
			results := make([]error, requests)
			for i := range requests {
				if kind == "proposal" || (kind == "mixed" && i%2 == 0) {
					proposals[i] = repair171Proposal(10, byte(i))
				} else {
					votes[i] = repair171Vote(pv, 10, byte(i))
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
				for i := range requests {
					s.Spawn(func() error {
						if votes[i] != nil {
							results[i] = client.SignVote(ctx, chainID, votes[i])
						} else {
							results[i] = client.SignProposal(ctx, chainID, proposals[i])
						}
						return nil
					})
				}
				return nil
			}))
			voteWinner, proposalWinner := -1, -1
			for i, err := range results {
				if err != nil {
					require.Equal(t, codes.InvalidArgument, status.Code(err))
					if votes[i] != nil {
						require.Empty(t, votes[i].Signature)
					} else {
						require.Empty(t, proposals[i].Signature)
					}
					continue
				}
				if votes[i] != nil {
					require.Equal(t, -1, voteWinner, "two conflicting votes were accepted")
					voteWinner = i
					repair171Verify(t, pv.Key.PubKey, types.VoteSignBytes(chainID, votes[i]), votes[i].Signature)
				} else {
					require.Equal(t, -1, proposalWinner, "two conflicting proposals were accepted")
					proposalWinner = i
					repair171Verify(t, pv.Key.PubKey, types.ProposalSignBytes(chainID, proposals[i]), proposals[i].Signature)
				}
			}
			if kind == "proposal" {
				require.NotEqual(t, -1, proposalWinner)
			} else {
				require.NotEqual(t, -1, voteWinner)
			}
			stop()
			reopened, err := privval.LoadFilePV(files.key, files.state)
			require.NoError(t, err)
			require.Equal(t, int64(10), reopened.LastSignState.Height)
			require.Equal(t, int32(1), reopened.LastSignState.Round)
			var wantBytes, wantSig []byte
			if voteWinner >= 0 {
				require.Equal(t, int8(3), reopened.LastSignState.Step)
				wantBytes = types.VoteSignBytes(chainID, votes[voteWinner])
				wantSig = votes[voteWinner].Signature
			} else {
				require.Equal(t, int8(1), reopened.LastSignState.Step)
				wantBytes = types.ProposalSignBytes(chainID, proposals[proposalWinner])
				wantSig = proposals[proposalWinner].Signature
			}
			require.Equal(t, wantBytes, []byte(reopened.LastSignState.SignBytes))
			require.Equal(t, wantSig, reopened.LastSignState.Signature)
			conn, _ = repair171RPC(t, reopened, chainID)
			client, err = tmgrpc.NewSignerClient(conn, chainID)
			require.NoError(t, err)
			before, err := os.ReadFile(files.state)
			require.NoError(t, err)
			for i := range requests {
				if votes[i] != nil {
					vote := repair171Vote(reopened, 10, byte(i))
					err = client.SignVote(ctx, chainID, vote)
					if i == voteWinner {
						require.NoError(t, err)
						require.Equal(t, wantSig, vote.Signature)
					} else {
						require.Equal(t, codes.InvalidArgument, status.Code(err))
						require.Empty(t, vote.Signature)
					}
				} else {
					proposal := repair171Proposal(10, byte(i))
					err = client.SignProposal(ctx, chainID, proposal)
					if voteWinner < 0 && i == proposalWinner {
						require.NoError(t, err)
						require.Equal(t, wantSig, proposal.Signature)
					} else {
						require.Equal(t, codes.InvalidArgument, status.Code(err))
						require.Empty(t, proposal.Signature)
					}
				}
			}
			after, err := os.ReadFile(files.state)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestRepair_171_ServerChainBoundary(t *testing.T) {
	pv, files := repair171FilePV(t)
	conn, stop := repair171RPC(t, pv, chainID)
	client := privvalproto.NewPrivValidatorAPIClient(conn)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	seed, err := client.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: chainID, Vote: repair171Vote(pv, 10, 1)})
	require.NoError(t, err)
	before, err := os.ReadFile(files.state)
	require.NoError(t, err)
	for _, rejectedChain := range []string{"other-chain", ""} {
		pub, err := client.GetPubKey(ctx, &privvalproto.PubKeyRequest{ChainId: rejectedChain})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Nil(t, pub)
		vote, err := client.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: rejectedChain, Vote: repair171Vote(pv, 100, 2)})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Nil(t, vote)
		proposal, err := client.SignProposal(ctx, &privvalproto.SignProposalRequest{ChainId: rejectedChain, Proposal: repair171Proposal(101, 3)})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Nil(t, proposal)
		after, err := os.ReadFile(files.state)
		require.NoError(t, err)
		require.Equal(t, before, after)
	}
	stop()
	reopened, err := privval.LoadFilePV(files.key, files.state)
	require.NoError(t, err)
	require.Equal(t, int64(10), reopened.LastSignState.Height)
	require.Equal(t, int32(1), reopened.LastSignState.Round)
	require.Equal(t, int8(3), reopened.LastSignState.Step)
	require.Equal(t, types.VoteSignBytes(chainID, &seed.Vote), []byte(reopened.LastSignState.SignBytes))
	require.Equal(t, seed.Vote.Signature, reopened.LastSignState.Signature)
	conn, _ = repair171RPC(t, reopened, chainID)
	client = privvalproto.NewPrivValidatorAPIClient(conn)
	pub, err := client.GetPubKey(ctx, &privvalproto.PubKeyRequest{ChainId: chainID})
	require.NoError(t, err)
	pk, err := crypto.PubKeyFromProto(pub.PubKey)
	require.NoError(t, err)
	require.Equal(t, pv.Key.PubKey, pk)
	proposal, err := client.SignProposal(ctx, &privvalproto.SignProposalRequest{ChainId: chainID, Proposal: repair171Proposal(11, 4)})
	require.NoError(t, err)
	repair171Verify(t, pk, types.ProposalSignBytes(chainID, &proposal.Proposal), proposal.Proposal.Signature)
	vote, err := client.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: chainID, Vote: repair171Vote(reopened, 11, 4)})
	require.NoError(t, err)
	repair171Verify(t, pk, types.VoteSignBytes(chainID, &vote.Vote), vote.Vote.Signature)
}

func TestRepair_171_ClientChainArgument(t *testing.T) {
	pv, files := repair171FilePV(t)
	conn, _ := repair171RPC(t, pv, chainID)
	client, err := tmgrpc.NewSignerClient(conn, chainID)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	pk, err := client.GetPubKey(ctx)
	require.NoError(t, err)
	proposal := repair171Proposal(1, 1)
	require.NoError(t, client.SignProposal(ctx, chainID, proposal))
	repair171Verify(t, pk, types.ProposalSignBytes(chainID, proposal), proposal.Signature)
	proposalSig, err := ed25519.SignatureFromBytes(proposal.Signature)
	require.NoError(t, err)
	require.Error(t, pk.Verify(types.ProposalSignBytes("other-chain", proposal), proposalSig))
	vote := repair171Vote(pv, 1, 1)
	require.NoError(t, client.SignVote(ctx, chainID, vote))
	repair171Verify(t, pk, types.VoteSignBytes(chainID, vote), vote.Signature)
	voteSig, err := ed25519.SignatureFromBytes(vote.Signature)
	require.NoError(t, err)
	require.Error(t, pk.Verify(types.VoteSignBytes("other-chain", vote), voteSig))

	for _, rejectedChain := range []string{"other-chain", ""} {
		// Also use a server that accepts the requested chain, so server validation
		// cannot hide a missing client-side check on either signing method.
		for _, serverChain := range []string{chainID, rejectedChain} {
			otherPV, otherFiles := repair171FilePV(t)
			otherConn, stop := repair171RPC(t, otherPV, serverChain)
			otherClient, err := tmgrpc.NewSignerClient(otherConn, chainID)
			require.NoError(t, err)
			before, err := os.ReadFile(otherFiles.state)
			require.NoError(t, err)
			v := repair171Vote(otherPV, 100, 2)
			vBefore := proto.Clone(v)
			require.Equal(t, codes.InvalidArgument, status.Code(otherClient.SignVote(ctx, rejectedChain, v)))
			require.True(t, proto.Equal(vBefore, v))
			p := repair171Proposal(101, 3)
			pBefore := proto.Clone(p)
			require.Equal(t, codes.InvalidArgument, status.Code(otherClient.SignProposal(ctx, rejectedChain, p)))
			require.True(t, proto.Equal(pBefore, p))
			after, err := os.ReadFile(otherFiles.state)
			require.NoError(t, err)
			require.True(t, bytes.Equal(before, after))
			stop()
			reopened, err := privval.LoadFilePV(otherFiles.key, otherFiles.state)
			require.NoError(t, err)
			require.Zero(t, reopened.LastSignState.Height)
			require.Empty(t, reopened.LastSignState.Signature)
		}
	}
	state, err := os.ReadFile(files.state)
	require.NoError(t, err)
	require.NotEmpty(t, state)
}
