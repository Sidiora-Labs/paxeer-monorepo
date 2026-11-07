package grpc

import (
	context "context"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	privvalproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// SignerServer implements PrivValidatorAPIServer 9generated via protobuf services)
// Handles remote validator connections that provide signing services
type SignerServer struct {
	chainID string
	privVal types.PrivValidator
	// signingMu covers the safety-state check, signing and persistence for both RPCs.
	signingMu sync.Mutex
}

func NewSignerServer(chainID string, privVal types.PrivValidator) *SignerServer {
	return &SignerServer{
		chainID: chainID,
		privVal: privVal,
	}
}

var _ privvalproto.PrivValidatorAPIServer = (*SignerServer)(nil)

// PubKey receives a request for the pubkey
// returns the pubkey on success and error on failure
func (ss *SignerServer) GetPubKey(ctx context.Context, req *privvalproto.PubKeyRequest) (
	*privvalproto.PubKeyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing public key request")
	}
	if req.ChainId != ss.chainID {
		return nil, status.Errorf(codes.InvalidArgument, "chain ID mismatch: got %q, want %q", req.ChainId, ss.chainID)
	}
	var pubKey crypto.PubKey

	pubKey, err := ss.privVal.GetPubKey(ctx)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "error getting pubkey: %v", err)
	}
	logger.Info("SignerServer: GetPubKey Success")
	return &privvalproto.PubKeyResponse{PubKey: crypto.PubKeyToProto(pubKey)}, nil
}

// SignVote receives a vote sign requests, attempts to sign it
// returns SignedVoteResponse on success and error on failure
func (ss *SignerServer) SignVote(ctx context.Context, req *privvalproto.SignVoteRequest) (*privvalproto.SignedVoteResponse, error) {
	if req == nil || req.Vote == nil {
		return nil, status.Error(codes.InvalidArgument, "missing vote payload")
	}
	if req.ChainId != ss.chainID {
		return nil, status.Errorf(codes.InvalidArgument, "chain ID mismatch: got %q, want %q", req.ChainId, ss.chainID)
	}
	ss.signingMu.Lock()
	defer ss.signingMu.Unlock()

	vote := req.Vote

	err := ss.privVal.SignVote(ctx, req.ChainId, vote)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error signing vote: %v", err)
	}

	logger.Info("SignerServer: SignVote Success", "height", req.Vote.Height)

	return &privvalproto.SignedVoteResponse{Vote: *vote}, nil
}

// SignProposal receives a proposal sign requests, attempts to sign it
// returns SignedProposalResponse on success and error on failure
func (ss *SignerServer) SignProposal(ctx context.Context, req *privvalproto.SignProposalRequest) (*privvalproto.SignedProposalResponse, error) {
	if req == nil || req.Proposal == nil {
		return nil, status.Error(codes.InvalidArgument, "missing proposal payload")
	}
	if req.ChainId != ss.chainID {
		return nil, status.Errorf(codes.InvalidArgument, "chain ID mismatch: got %q, want %q", req.ChainId, ss.chainID)
	}
	ss.signingMu.Lock()
	defer ss.signingMu.Unlock()

	proposal := req.Proposal

	err := ss.privVal.SignProposal(ctx, req.ChainId, proposal)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error signing proposal: %v", err)
	}

	logger.Info("SignerServer: SignProposal Success", "height", req.Proposal.Height)

	return &privvalproto.SignedProposalResponse{Proposal: *proposal}, nil
}
