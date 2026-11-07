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

func TestRepair_170_RetryReopen(t *testing.T) {
	for _, kind := range []string{"prevote", "precommit", "proposal"} {
		for _, failure := range []string{"create", "rename"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				pv, keyPath, statePath := repair170Validator(t)
				client, stop := repair170Client(t, pv)
				previous := repair170Message(kind, 8, false)
				require.NoError(t, previous.sign(t, client))
				oldState, err := os.ReadFile(statePath)
				require.NoError(t, err)
				restore := repair170BlockWrites(t, statePath, failure)

				first := repair170Message(kind, 10, false)
				repair170Refused(t, first, client)
				repair170Refused(t, repair170Message(kind, 10, false), client)
				timestampRetry := repair170Message(kind, 10, false)
				if timestampRetry.vote != nil {
					timestampRetry.vote.Timestamp = timestampRetry.vote.Timestamp.Add(time.Second)
				}
				repair170Refused(t, timestampRetry, client)
				repair170Refused(t, repair170Message(kind, 10, true), client)
				otherKind := "proposal"
				if kind == "proposal" {
					otherKind = "prevote"
				}
				repair170Refused(t, repair170Message(otherKind, 11, false), client)
				require.EqualValues(t, 8, pv.LastSignState.Height)
				require.Equal(t, previous.signature(), pv.LastSignState.Signature)

				restore()
				diskState, err := os.ReadFile(statePath)
				require.NoError(t, err)
				require.Equal(t, oldState, diskState)
				require.NoError(t, timestampRetry.sign(t, client))
				require.Equal(t, first.signBytes(), timestampRetry.signBytes())
				signature := bytes.Clone(timestampRetry.signature())
				repair170Verify(t, pv, timestampRetry)
				stop()

				pv, err = LoadFilePV(keyPath, statePath)
				require.NoError(t, err)
				require.EqualValues(t, 10, pv.LastSignState.Height)
				require.EqualValues(t, 2, pv.LastSignState.Round)
				require.Equal(t, signature, pv.LastSignState.Signature)
				require.Equal(t, first.signBytes(), []byte(pv.LastSignState.SignBytes))
				client, _ = repair170Client(t, pv)
				repair170Refused(t, repair170Message(kind, 10, true), client)
				repair170Refused(t, repair170Message(kind, 8, false), client)
				retry := repair170Message(kind, 10, false)
				require.NoError(t, retry.sign(t, client))
				require.Equal(t, signature, retry.signature())
				repair170Verify(t, pv, retry)
			})
		}
	}
}

func TestRepair_170_PendingConflictReopen(t *testing.T) {
	for _, kind := range []string{"prevote", "proposal"} {
		t.Run(kind, func(t *testing.T) {
			pv, keyPath, statePath := repair170Validator(t)
			client, stop := repair170Client(t, pv)
			restore := repair170BlockWrites(t, statePath, "rename")
			repair170Refused(t, repair170Message(kind, 10, false), client)
			restore()
			repair170Refused(t, repair170Message(kind, 10, true), client)
			stop()

			reopened, err := LoadFilePV(keyPath, statePath)
			require.NoError(t, err)
			require.EqualValues(t, 10, reopened.LastSignState.Height)
			client, _ = repair170Client(t, reopened)
			repair170Refused(t, repair170Message(kind, 10, true), client)
			original := repair170Message(kind, 10, false)
			require.NoError(t, original.sign(t, client))
			require.Equal(t, reopened.LastSignState.Signature, original.signature())
			repair170Verify(t, reopened, original)
		})
	}
}

func TestRepair_170_SavePendingReopen(t *testing.T) {
	pv, keyPath, statePath := repair170Validator(t)
	restore := repair170BlockWrites(t, statePath, "create")
	original := repair170Message("prevote", 10, false)
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
	conflict := repair170Message("prevote", 10, true)
	require.Error(t, conflict.sign(t, reopened))
	require.Empty(t, conflict.signature())
	require.NoError(t, original.sign(t, reopened))
	repair170Verify(t, reopened, original)
}

func TestRepair_170_RestartBeforeRetry(t *testing.T) {
	pv, keyPath, statePath := repair170Validator(t)
	client, stop := repair170Client(t, pv)
	restore := repair170BlockWrites(t, statePath, "rename")
	repair170Refused(t, repair170Message("prevote", 10, false), client)
	repair170Refused(t, repair170Message("prevote", 10, false), client)
	stop()
	pv = nil
	restore()

	reopened, err := LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	require.Zero(t, reopened.LastSignState.Height)
	require.Empty(t, reopened.LastSignState.Signature)
	client, stop = repair170Client(t, reopened)
	vote := repair170Message("prevote", 10, true)
	require.NoError(t, vote.sign(t, client))
	repair170Verify(t, reopened, vote)
	stop()

	reopened, err = LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	original := repair170Message("prevote", 10, false)
	require.Error(t, original.sign(t, reopened))
	require.Empty(t, original.signature())
}

const repair170ChainID = "repair-170"

type repair170Signable struct {
	vote     *tmproto.Vote
	proposal *tmproto.Proposal
}

func repair170Message(kind string, height int64, conflict bool) repair170Signable {
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
		return repair170Signable{proposal: unsigned}
	}
	voteType := tmproto.PrevoteType
	if kind == "precommit" {
		voteType = tmproto.PrecommitType
	}
	vote := types.Vote{Type: voteType, Height: height, Round: 2, BlockID: blockID,
		Timestamp: timestamp, ValidatorAddress: bytes.Repeat([]byte{3}, crypto.AddressSize)}
	return repair170Signable{vote: vote.ToProto()}
}

func (message repair170Signable) sign(t *testing.T, signer types.PrivValidator) error {
	t.Helper()
	if message.vote != nil {
		return signer.SignVote(t.Context(), repair170ChainID, message.vote)
	}
	return signer.SignProposal(t.Context(), repair170ChainID, message.proposal)
}

func (message repair170Signable) signature() []byte {
	if message.vote != nil {
		return message.vote.Signature
	}
	return message.proposal.Signature
}

func (message repair170Signable) signBytes() []byte {
	if message.vote != nil {
		return types.VoteSignBytes(repair170ChainID, message.vote)
	}
	return types.ProposalSignBytes(repair170ChainID, message.proposal)
}

func repair170Refused(t *testing.T, message repair170Signable, client *SignerClient) {
	t.Helper()
	var remoteError *RemoteSignerError
	require.ErrorAs(t, message.sign(t, client), &remoteError)
	require.Empty(t, message.signature())
}

func repair170Verify(t *testing.T, pv *FilePV, message repair170Signable) {
	t.Helper()
	signature, err := crypto.SigFromBytes(message.signature())
	require.NoError(t, err)
	require.NoError(t, pv.Key.PubKey.Verify(message.signBytes(), signature))
}

func repair170Validator(t *testing.T) (*FilePV, string, string) {
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

func repair170BlockWrites(t *testing.T, statePath, failure string) func() {
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

func repair170Client(t *testing.T, pv *FilePV) (*SignerClient, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "pv170-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	socketPath := filepath.Join(dir, "signer.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	endpoint := NewSignerListenerEndpoint(listener)
	client, err := NewSignerClient(ctx, endpoint, repair170ChainID)
	require.NoError(t, err)
	dialer := NewSignerDialerEndpoint(DialUnixFn(socketPath))
	server := NewSignerServer(dialer, repair170ChainID, pv)
	require.NoError(t, server.Start(ctx))
	stop := sync.OnceFunc(func() {
		cancel()
		server.Stop()
		endpoint.Stop()
	})
	t.Cleanup(stop)
	return client, stop
}
