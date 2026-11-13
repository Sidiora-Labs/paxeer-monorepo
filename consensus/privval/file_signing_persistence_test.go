package privval

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestFilePVSigningPersistence_RetryReopen(t *testing.T) {
	for _, kind := range []string{"prevote", "precommit", "proposal"} {
		for _, failure := range []string{"create", "rename"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				pv, keyPath, statePath := newPersistenceValidator(t)
				client, stop := newPersistenceSignerClient(t, pv)
				previous := persistenceMessage(kind, 8, false)
				require.NoError(t, previous.sign(t, client))
				oldState, err := os.ReadFile(statePath)
				require.NoError(t, err)
				restore := blockSigningStateWrites(t, statePath, failure)

				first := persistenceMessage(kind, 10, false)
				requirePersistenceSigningRefused(t, first, client)
				requirePersistenceSigningRefused(t, persistenceMessage(kind, 10, false), client)
				timestampRetry := persistenceMessage(kind, 10, false)
				if timestampRetry.vote != nil {
					timestampRetry.vote.Timestamp = timestampRetry.vote.Timestamp.Add(time.Second)
				}
				requirePersistenceSigningRefused(t, timestampRetry, client)
				requirePersistenceSigningRefused(t, persistenceMessage(kind, 10, true), client)
				otherKind := "proposal"
				if kind == "proposal" {
					otherKind = "prevote"
				}
				requirePersistenceSigningRefused(t, persistenceMessage(otherKind, 11, false), client)
				require.EqualValues(t, 8, pv.LastSignState.Height)
				require.Equal(t, previous.signature(), pv.LastSignState.Signature)

				restore()
				diskState, err := os.ReadFile(statePath)
				require.NoError(t, err)
				require.Equal(t, oldState, diskState)
				require.NoError(t, timestampRetry.sign(t, client))
				require.Equal(t, first.signBytes(), timestampRetry.signBytes())
				signature := bytes.Clone(timestampRetry.signature())
				verifyPersistenceSignature(t, pv, timestampRetry)
				stop()

				pv, err = LoadFilePV(keyPath, statePath)
				require.NoError(t, err)
				require.EqualValues(t, 10, pv.LastSignState.Height)
				require.EqualValues(t, 2, pv.LastSignState.Round)
				require.Equal(t, signature, pv.LastSignState.Signature)
				require.Equal(t, first.signBytes(), []byte(pv.LastSignState.SignBytes))
				client, _ = newPersistenceSignerClient(t, pv)
				requirePersistenceSigningRefused(t, persistenceMessage(kind, 10, true), client)
				requirePersistenceSigningRefused(t, persistenceMessage(kind, 8, false), client)
				retry := persistenceMessage(kind, 10, false)
				require.NoError(t, retry.sign(t, client))
				require.Equal(t, signature, retry.signature())
				verifyPersistenceSignature(t, pv, retry)
			})
		}
	}
}

func TestFilePVSigningPersistence_PendingConflictReopen(t *testing.T) {
	for _, kind := range []string{"prevote", "proposal"} {
		t.Run(kind, func(t *testing.T) {
			pv, keyPath, statePath := newPersistenceValidator(t)
			client, stop := newPersistenceSignerClient(t, pv)
			restore := blockSigningStateWrites(t, statePath, "rename")
			requirePersistenceSigningRefused(t, persistenceMessage(kind, 10, false), client)
			restore()
			requirePersistenceSigningRefused(t, persistenceMessage(kind, 10, true), client)
			stop()

			reopened, err := LoadFilePV(keyPath, statePath)
			require.NoError(t, err)
			require.EqualValues(t, 10, reopened.LastSignState.Height)
			client, _ = newPersistenceSignerClient(t, reopened)
			requirePersistenceSigningRefused(t, persistenceMessage(kind, 10, true), client)
			original := persistenceMessage(kind, 10, false)
			require.NoError(t, original.sign(t, client))
			require.Equal(t, reopened.LastSignState.Signature, original.signature())
			verifyPersistenceSignature(t, reopened, original)
		})
	}
}

func TestFilePVSigningPersistence_SavePendingReopen(t *testing.T) {
	pv, keyPath, statePath := newPersistenceValidator(t)
	restore := blockSigningStateWrites(t, statePath, "create")
	original := persistenceMessage("prevote", 10, false)
	err := original.sign(t, pv)
	var pathError *os.PathError
	require.ErrorAs(t, err, &pathError)
	require.Empty(t, original.signature())
	require.Error(t, pv.Save())
	restore()
	require.NoError(t, pv.Save())
	pv = nil

	reopened, err := LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	require.EqualValues(t, 10, reopened.LastSignState.Height)
	conflict := persistenceMessage("prevote", 10, true)
	require.Error(t, conflict.sign(t, reopened))
	require.Empty(t, conflict.signature())
	require.NoError(t, original.sign(t, reopened))
	verifyPersistenceSignature(t, reopened, original)
}

func TestFilePVSigningPersistence_RestartBeforeRetry(t *testing.T) {
	pv, keyPath, statePath := newPersistenceValidator(t)
	client, stop := newPersistenceSignerClient(t, pv)
	restore := blockSigningStateWrites(t, statePath, "rename")
	requirePersistenceSigningRefused(t, persistenceMessage("prevote", 10, false), client)
	requirePersistenceSigningRefused(t, persistenceMessage("prevote", 10, false), client)
	stop()
	pv = nil
	restore()

	reopened, err := LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	require.Zero(t, reopened.LastSignState.Height)
	require.Empty(t, reopened.LastSignState.Signature)
	client, stop = newPersistenceSignerClient(t, reopened)
	vote := persistenceMessage("prevote", 10, true)
	require.NoError(t, vote.sign(t, client))
	verifyPersistenceSignature(t, reopened, vote)
	stop()

	reopened, err = LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	original := persistenceMessage("prevote", 10, false)
	require.Error(t, original.sign(t, reopened))
	require.Empty(t, original.signature())
}

const signingPersistenceChainID = "repair-170"

type persistenceSignable struct {
	vote     *tmproto.Vote
	proposal *tmproto.Proposal
}

func persistenceMessage(kind string, height int64, conflict bool) persistenceSignable {
	blockByte := byte(1)
	if conflict {
		blockByte = 2
	}
	hash := bytes.Repeat([]byte{blockByte}, crypto.HashSize)
	blockID := types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Total: 1, Hash: hash}}
	timestamp := time.Unix(1700000000, 0).UTC()
	if kind == "proposal" {
		proposal := types.Proposal{Type: tmproto.ProposalType, Height: height, Round: 2,
			POLRound: -1, BlockID: blockID, Timestamp: timestamp}
		unsigned := proposal.ToProto()
		unsigned.Signature = nil
		return persistenceSignable{proposal: unsigned}
	}
	voteType := tmproto.PrevoteType
	if kind == "precommit" {
		voteType = tmproto.PrecommitType
	}
	vote := types.Vote{Type: voteType, Height: height, Round: 2, BlockID: blockID,
		Timestamp: timestamp, ValidatorAddress: bytes.Repeat([]byte{3}, crypto.AddressSize)}
	return persistenceSignable{vote: vote.ToProto()}
}

func (message persistenceSignable) sign(t *testing.T, signer types.PrivValidator) error {
	t.Helper()
	if message.vote != nil {
		return signer.SignVote(t.Context(), signingPersistenceChainID, message.vote)
	}
	return signer.SignProposal(t.Context(), signingPersistenceChainID, message.proposal)
}

func (message persistenceSignable) signature() []byte {
	if message.vote != nil {
		return message.vote.Signature
	}
	return message.proposal.Signature
}

func (message persistenceSignable) signBytes() []byte {
	if message.vote != nil {
		return types.VoteSignBytes(signingPersistenceChainID, message.vote)
	}
	return types.ProposalSignBytes(signingPersistenceChainID, message.proposal)
}

func requirePersistenceSigningRefused(t *testing.T, message persistenceSignable, client *SignerClient) {
	t.Helper()
	var remoteError *RemoteSignerError
	require.ErrorAs(t, message.sign(t, client), &remoteError)
	require.Empty(t, message.signature())
}

func verifyPersistenceSignature(t *testing.T, pv *FilePV, message persistenceSignable) {
	t.Helper()
	signature, err := crypto.SigFromBytes(message.signature())
	require.NoError(t, err)
	require.NoError(t, pv.Key.PubKey.Verify(message.signBytes(), signature))
}

func newPersistenceValidator(t *testing.T) (*FilePV, string, string) {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.json")
	stateDir := filepath.Join(dir, "state")
	require.NoError(t, os.Mkdir(stateDir, 0700))
	statePath := filepath.Join(stateDir, "state.json")
	pv, err := GenFilePV(keyPath, statePath, "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	return pv, keyPath, statePath
}

func blockSigningStateWrites(t *testing.T, statePath, failure string) func() {
	t.Helper()
	blockedPath := statePath
	if failure == "create" {
		blockedPath = filepath.Dir(statePath)
	}
	backupPath := blockedPath + ".saved"
	require.NoError(t, os.Rename(blockedPath, backupPath))
	if failure == "create" {
		require.NoError(t, os.WriteFile(blockedPath, []byte("block state directory"), 0600))
	} else {
		require.NoError(t, os.Mkdir(blockedPath, 0700))
	}
	restore := sync.OnceFunc(func() {
		require.NoError(t, os.Remove(blockedPath))
		require.NoError(t, os.Rename(backupPath, blockedPath))
	})
	t.Cleanup(restore)
	return restore
}

func newPersistenceSignerClient(t *testing.T, pv *FilePV) (*SignerClient, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "pv170-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	socketPath := filepath.Join(dir, "signer.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	endpoint := NewSignerListenerEndpoint(listener)
	client, err := NewSignerClient(ctx, endpoint, signingPersistenceChainID)
	require.NoError(t, err)
	dialer := NewSignerDialerEndpoint(DialUnixFn(socketPath))
	server := NewSignerServer(dialer, signingPersistenceChainID, pv)
	require.NoError(t, server.Start(ctx))
	stop := sync.OnceFunc(func() {
		cancel()
		server.Stop()
		endpoint.Stop()
	})
	t.Cleanup(stop)
	return client, stop
}
