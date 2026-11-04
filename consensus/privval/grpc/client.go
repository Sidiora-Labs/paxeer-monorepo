package grpc

import (
	"context"
	"errors"
	"sync"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	privvalproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// SignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type SignerClient struct {
	client    privvalproto.PrivValidatorAPIClient
	conn      *grpc.ClientConn
	chainID   string
	closeOnce sync.Once
	closeErr  error
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(conn *grpc.ClientConn,
	chainID string) (*SignerClient, error) {
	if conn == nil {
		return nil, errors.New("signer client requires a non-nil connection")
	}

	sc := &SignerClient{
		conn:    conn,
		chainID: chainID,
		client:  privvalproto.NewPrivValidatorAPIClient(conn), // Create the Private Validator Client
	}

	return sc, nil
}

// Close closes the underlying connection
func (sc *SignerClient) Close() error {
	logger.Info("Stopping service")
	sc.closeOnce.Do(func() {
		if sc.conn != nil {
			sc.closeErr = sc.conn.Close()
		}
	})
	return sc.closeErr
}

//--------------------------------------------------------
// Implement PrivValidator

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey(ctx context.Context) (crypto.PubKey, error) {
	resp, err := sc.client.GetPubKey(ctx, &privvalproto.PubKeyRequest{ChainId: sc.chainID})
	if err != nil {
		errStatus, _ := status.FromError(err)
		logger.Error("SignerClient::GetPubKey", "err", errStatus)
		return crypto.PubKey{}, errStatus.Err()
	}

	pk, err := crypto.PubKeyFromProto(resp.PubKey)
	if err != nil {
		return crypto.PubKey{}, err
	}

	return pk, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *SignerClient) SignVote(ctx context.Context, chainID string, vote *tmproto.Vote) error {
	if chainID != sc.chainID {
		return status.Errorf(codes.InvalidArgument, "chain ID mismatch: got %q, want %q", chainID, sc.chainID)
	}
	resp, err := sc.client.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: chainID, Vote: vote})
	if err != nil {
		errStatus, _ := status.FromError(err)
		logger.Error("Client SignVote", "err", errStatus)
		return errStatus.Err()
	}

	*vote = resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(ctx context.Context, chainID string, proposal *tmproto.Proposal) error {
	if chainID != sc.chainID {
		return status.Errorf(codes.InvalidArgument, "chain ID mismatch: got %q, want %q", chainID, sc.chainID)
	}
	resp, err := sc.client.SignProposal(
		ctx, &privvalproto.SignProposalRequest{ChainId: chainID, Proposal: proposal})

	if err != nil {
		errStatus, _ := status.FromError(err)
		logger.Error("SignerClient::SignProposal", "err", errStatus)
		return errStatus.Err()
	}

	*proposal = resp.Proposal

	return nil
}
