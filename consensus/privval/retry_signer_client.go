package privval

import (
	"context"
	"fmt"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// RetrySignerClient wraps SignerClient adding retry for each operation (except
// Ping) w/ a timeout.
type RetrySignerClient struct {
	next    *SignerClient
	retries int
	timeout time.Duration
}

// NewRetrySignerClient returns RetrySignerClient. If +retries+ is 0, the
// client will be retrying each operation indefinitely.
func NewRetrySignerClient(sc *SignerClient, retries int, timeout time.Duration) *RetrySignerClient {
	return &RetrySignerClient{sc, retries, timeout}
}

var _ types.PrivValidator = (*RetrySignerClient)(nil)

func (sc *RetrySignerClient) Close() error {
	return sc.next.Close()
}

func (sc *RetrySignerClient) IsConnected() bool {
	return sc.next.IsConnected()
}

func (sc *RetrySignerClient) WaitForConnection(ctx context.Context, maxWait time.Duration) error {
	return sc.next.WaitForConnection(ctx, maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

func (sc *RetrySignerClient) Ping(ctx context.Context) error {
	return sc.next.Ping(ctx)
}

func (sc *RetrySignerClient) GetPubKey(ctx context.Context) (crypto.PubKey, error) {
	var (
		pk  crypto.PubKey
		err error
	)

	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		if err := ctx.Err(); err != nil {
			return pk, err
		}
		pk, err = sc.next.GetPubKey(ctx)
		if err == nil {
			return pk, nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return pk, err
		}
		if sc.retries > 0 && i+1 == sc.retries {
			break
		}
		if err := sc.waitRetry(ctx); err != nil {
			return pk, err
		}
	}
	return pk, fmt.Errorf("exhausted all attempts to get pubkey: %w", err)
}

func (sc *RetrySignerClient) SignVote(ctx context.Context, chainID string, vote *tmproto.Vote) error {
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = sc.next.SignVote(ctx, chainID, vote)
		if err == nil {
			return nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return err
		}
		if sc.retries > 0 && i+1 == sc.retries {
			break
		}
		if err := sc.waitRetry(ctx); err != nil {
			return err
		}
	}
	return fmt.Errorf("exhausted all attempts to sign vote: %w", err)
}

func (sc *RetrySignerClient) SignProposal(ctx context.Context, chainID string, proposal *tmproto.Proposal) error {
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = sc.next.SignProposal(ctx, chainID, proposal)
		if err == nil {
			return nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return err
		}
		if sc.retries > 0 && i+1 == sc.retries {
			break
		}
		if err := sc.waitRetry(ctx); err != nil {
			return err
		}
	}
	return fmt.Errorf("exhausted all attempts to sign proposal: %w", err)
}

// waitRetry releases its timer on cancellation as well as on expiry.
func (sc *RetrySignerClient) waitRetry(ctx context.Context) error {
	timer := time.NewTimer(sc.timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}
