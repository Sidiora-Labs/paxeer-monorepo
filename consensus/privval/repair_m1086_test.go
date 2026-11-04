package privval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/libs/protoio"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	privvalproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

const repairM1086Chain = "repair-m1086"

func TestRepairM_1086_RetrySigningCancellation(t *testing.T) {
	for _, method := range []string{"vote", "proposal", "pubkey"} {
		for _, unavailable := range []string{"connection", "retry-delay", "already-canceled"} {
			t.Run(method+"/"+unavailable, func(t *testing.T) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				lifetime, stop := context.WithCancel(t.Context())
				endpoint := NewSignerListenerEndpoint(ln)
				client, err := NewSignerClient(lifetime, endpoint, repairM1086Chain)
				require.NoError(t, err)
				t.Cleanup(func() { stop(); require.NoError(t, client.Close()) })
				if unavailable != "connection" {
					// A closed real loopback connection fails immediately, before
					// the request deadline, and enters the retry-delay branch.
					other, err := net.Listen("tcp", "127.0.0.1:0")
					require.NoError(t, err)
					defer other.Close()
					conn, err := net.Dial("tcp", other.Addr().String())
					require.NoError(t, err)
					require.NoError(t, conn.Close())
					endpoint.SetConnection(conn)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
				defer cancel()
				if unavailable == "already-canceled" {
					cancel()
				}
				clientWithRetries := NewRetrySignerClient(client, 0, 2*time.Second)
				started := time.Now()
				switch method {
				case "vote":
					err = clientWithRetries.SignVote(ctx, repairM1086Chain, repairM1086Vote())
				case "proposal":
					err = clientWithRetries.SignProposal(ctx, repairM1086Chain, repairM1086Proposal())
				case "pubkey":
					_, err = clientWithRetries.GetPubKey(ctx)
				}
				require.ErrorIs(t, err, ctx.Err())
				require.Less(t, time.Since(started), 500*time.Millisecond)
				if unavailable == "already-canceled" {
					// The closed connection remains installed when no attempt
					// was made; a write attempt would otherwise drop it.
					require.True(t, endpoint.IsConnected())
				}
			})
		}
	}
}

func TestRepairM_1086_PingTransportFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	lifetime, cancel := context.WithCancel(t.Context())
	client, err := NewSignerClient(lifetime, NewSignerListenerEndpoint(ln), repairM1086Chain)
	require.NoError(t, err)
	defer func() { cancel(); require.NoError(t, client.Close()) }()
	ctx, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	require.ErrorIs(t, client.Ping(ctx), context.DeadlineExceeded)
}

func TestRepairM_1086_PingResponseVariant(t *testing.T) {
	pv, _, _ := repairM1086Validator(t)
	nodeKey, signerKey := ed25519.GenerateSecretKey(), ed25519.GenerateSecretKey()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	endpoint := NewSignerListenerEndpoint(NewTCPListener(ln, nodeKey, signerKey.Public()))
	client, server, stop := repairM1086Transport(t, pv, endpoint,
		DialTCPFn(ln.Addr().String(), time.Second, signerKey, nodeKey.Public()))
	defer stop()
	// A loopback peer fixture deliberately returns a valid, differently typed
	// protobuf message. Both directions use the production encrypted framing.
	server.SetRequestHandler(func(ctx context.Context, pv types.PrivValidator, req privvalproto.Message, chain string) (privvalproto.Message, error) {
		if req.GetPingRequest() != nil {
			key, err := pv.GetPubKey(ctx)
			if err != nil {
				return privvalproto.Message{}, err
			}
			return mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: crypto.PubKeyToProto(key)}), nil
		}
		return DefaultValidationRequestHandler(ctx, pv, req, chain)
	})
	require.ErrorIs(t, client.Ping(t.Context()), ErrUnexpectedResponse)
	server.SetRequestHandler(DefaultValidationRequestHandler)
	require.NoError(t, client.Ping(t.Context()))
}

func TestRepairM_1086_TimeoutClassification(t *testing.T) {
	deadline := stalledHandshakeError(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	conn, refusal := DialTCPFn(addr, time.Second, testSocketDialerKey, testSocketListenerKey.Public())()
	require.Nil(t, conn)
	require.Error(t, refusal)
	var netErr net.Error
	require.ErrorAs(t, refusal, &netErr)
	require.False(t, netErr.Timeout())
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"refusal", refusal, false},
		{"handshake-deadline", deadline, true},
		{"endpoint-deadline", ErrConnectionTimeout, true},
		{"ordinary-error", errors.New("ordinary failure"), false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, IsConnTimeout(tc.err))
			if tc.err != nil {
				wrapped := fmt.Errorf("transport: %w", fmt.Errorf("signer: %w", fmt.Errorf("operation: %w", tc.err)))
				require.Equal(t, tc.want, IsConnTimeout(wrapped))
			}
		})
	}
}

func TestRepairM_1086_FilePVTransportReopen(t *testing.T) {
	pv, keyPath, statePath := repairM1086Validator(t)
	dir := t.TempDir()
	nodePath, signerPath := filepath.Join(dir, "node.key"), filepath.Join(dir, "signer.key")
	nodeKey, err := LoadOrGenSocketIdentity(nodePath)
	require.NoError(t, err)
	signerKey, err := LoadOrGenSocketIdentity(signerPath)
	require.NoError(t, err)
	start := func(pv *FilePV) (*SignerClient, func()) {
		endpoint, err := NewSignerListener("tcp://127.0.0.1:0", SocketIdentity{KeyFile: nodePath, PeerKey: signerKey.Public()})
		require.NoError(t, err)
		client, _, stop := repairM1086Transport(t, pv, endpoint,
			DialTCPFn("tcp://"+endpoint.listener.Addr().String(), time.Second, signerKey, nodeKey.Public()))
		return client, stop
	}
	client, stop := start(pv)
	pubKey, err := client.GetPubKey(t.Context())
	require.NoError(t, err)
	require.Equal(t, pv.Key.PubKey, pubKey)
	proposal := repairM1086Proposal()
	proposalBytes := types.ProposalSignBytes(repairM1086Chain, proposal)
	require.Empty(t, proposal.Signature)
	require.NoError(t, client.SignProposal(t.Context(), repairM1086Chain, proposal))
	repairM1086Verify(t, pubKey, proposalBytes, proposal.Signature)
	proposalSignature := bytes.Clone(proposal.Signature)
	stop()

	// Reopen both identity files, the listener, and FilePV's persisted safety state.
	reopenedNode, err := LoadOrGenSocketIdentity(nodePath)
	require.NoError(t, err)
	require.Equal(t, nodeKey.Public(), reopenedNode.Public())
	reopenedSigner, err := LoadOrGenSocketIdentity(signerPath)
	require.NoError(t, err)
	require.Equal(t, signerKey.Public(), reopenedSigner.Public())
	signerKey = reopenedSigner
	pv, err = LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	client, stop = start(pv)
	proposal = repairM1086Proposal()
	require.NoError(t, client.SignProposal(t.Context(), repairM1086Chain, proposal))
	require.Equal(t, proposalSignature, proposal.Signature)
	repairM1086Verify(t, pubKey, proposalBytes, proposal.Signature)
	conflict := repairM1086Proposal()
	conflict.BlockID.Hash = bytes.Repeat([]byte{9}, crypto.HashSize)
	var remoteErr *RemoteSignerError
	require.ErrorAs(t, client.SignProposal(t.Context(), repairM1086Chain, conflict), &remoteErr)
	require.Empty(t, conflict.Signature)

	vote := repairM1086Vote()
	voteBytes := types.VoteSignBytes(repairM1086Chain, vote)
	require.Empty(t, vote.Signature)
	require.NoError(t, client.SignVote(t.Context(), repairM1086Chain, vote))
	repairM1086Verify(t, pubKey, voteBytes, vote.Signature)
	voteSignature := bytes.Clone(vote.Signature)
	stop()
	pv, err = LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	client, stop = start(pv)
	defer stop()
	vote = repairM1086Vote()
	require.NoError(t, client.SignVote(t.Context(), repairM1086Chain, vote))
	require.Equal(t, voteSignature, vote.Signature)
	repairM1086Verify(t, pubKey, voteBytes, vote.Signature)
	vote = repairM1086Vote()
	vote.BlockID.Hash = bytes.Repeat([]byte{9}, crypto.HashSize)
	require.ErrorAs(t, client.SignVote(t.Context(), repairM1086Chain, vote), &remoteErr)
	require.Empty(t, vote.Signature)
}

func TestRepairM_1086_SocketPeerIdentity(t *testing.T) {
	for _, rejectedSide := range []string{"listener", "dialer", "missing-listener-pin", "missing-dialer-pin"} {
		t.Run(rejectedSide, func(t *testing.T) {
			nodeKey, signerKey, intruder := ed25519.GenerateSecretKey(), ed25519.GenerateSecretKey(), ed25519.GenerateSecretKey()
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer ln.Close()
			listener := NewTCPListener(ln, nodeKey, signerKey.Public())
			dialer := DialTCPFn(ln.Addr().String(), time.Second, signerKey, nodeKey.Public())
			switch rejectedSide {
			case "listener":
				dialer = DialTCPFn(ln.Addr().String(), time.Second, intruder, nodeKey.Public())
			case "dialer":
				dialer = DialTCPFn(ln.Addr().String(), time.Second, signerKey, intruder.Public())
			case "missing-listener-pin":
				listener = NewTCPListener(ln, nodeKey)
				conn, err := listener.Accept()
				require.Nil(t, conn)
				require.ErrorIs(t, err, ErrPeerNotAuthorized)
				return
			case "missing-dialer-pin":
				conn, err := DialTCPFn(ln.Addr().String(), time.Second, signerKey)()
				require.Nil(t, conn)
				require.ErrorIs(t, err, ErrPeerNotAuthorized)
				return
			}
			var acceptErr, dialErr, routedErr error
			var accepted, dialed net.Conn
			err = scope.Run(t.Context(), func(ctx context.Context, s scope.Scope) error {
				s.Spawn(func() error {
					accepted, acceptErr = listener.Accept()
					if accepted != nil {
						defer accepted.Close()
						// Rejection by the dialer must close the connection before
						// any framed request can reach the dispatch boundary.
						var req privvalproto.Message
						_, routedErr = protoio.NewDelimitedReader(accepted, 10240).ReadMsg(&req)
					}
					return nil
				})
				dialed, dialErr = dialer()
				if dialed != nil {
					defer dialed.Close()
					// Attempt a real signing request. A listener rejecting this
					// peer must never return its connection to a handler.
					req := mustWrapMsg(&privvalproto.SignVoteRequest{ChainId: repairM1086Chain, Vote: repairM1086Vote()})
					_, _ = protoio.NewDelimitedWriter(dialed).WriteMsg(&req)
				}
				return nil
			})
			require.NoError(t, err)
			if rejectedSide == "listener" {
				require.ErrorIs(t, acceptErr, ErrPeerNotAuthorized)
				require.Nil(t, accepted)
			} else {
				require.ErrorIs(t, dialErr, ErrPeerNotAuthorized)
				require.Nil(t, dialed)
				require.Error(t, routedErr)
			}
		})
	}
}

func TestRepairM_1086_IdentityStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.key")
	key, err := LoadOrGenSocketIdentity(path)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	reopened, err := LoadOrGenSocketIdentity(path)
	require.NoError(t, err)
	require.Equal(t, key.Public(), reopened.Public())
	require.NoError(t, os.WriteFile(path, []byte("corrupt"), 0600))
	_, err = LoadOrGenSocketIdentity(path)
	require.Error(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte("corrupt"), data)
	_, err = LoadOrGenSocketIdentity("")
	require.Error(t, err)
	_, err = NewSignerListener("tcp://127.0.0.1:0")
	require.ErrorIs(t, err, ErrPeerNotAuthorized)
	_, err = NewSignerListener("tcp://127.0.0.1:0", SocketIdentity{PeerKey: key.Public()})
	require.Error(t, err)
	unsafePath := filepath.Join(t.TempDir(), "public.key")
	require.NoError(t, os.WriteFile(unsafePath, []byte("invalid public identity"), 0644))
	_, err = LoadOrGenSocketIdentity(unsafePath)
	require.Error(t, err)
}

func repairM1086Validator(t *testing.T) (*FilePV, string, string) {
	t.Helper()
	dir := t.TempDir()
	keyPath, statePath := filepath.Join(dir, "validator-key.json"), filepath.Join(dir, "validator-state.json")
	pv, err := GenFilePV(keyPath, statePath, "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	return pv, keyPath, statePath
}

func repairM1086Transport(t *testing.T, pv *FilePV, endpoint *SignerListenerEndpoint, dialer SocketDialer) (*SignerClient, *SignerServer, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	client, err := NewSignerClient(ctx, endpoint, repairM1086Chain)
	require.NoError(t, err)
	dialerEndpoint := NewSignerDialerEndpoint(dialer)
	SignerDialerEndpointTimeoutReadWrite(time.Second)(dialerEndpoint)
	server := NewSignerServer(dialerEndpoint, repairM1086Chain, pv)
	stop := sync.OnceFunc(func() {
		cancel()
		server.Stop()
		endpoint.Stop()
	})
	t.Cleanup(stop)
	require.NoError(t, server.Start(ctx))
	wait, stopWait := context.WithTimeout(t.Context(), 2*time.Second)
	defer stopWait()
	require.NoError(t, client.WaitForConnection(wait, 2*time.Second))
	return client, server, stop
}

func repairM1086Block() types.BlockID {
	hash := bytes.Repeat([]byte{7}, crypto.HashSize)
	return types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Total: 1, Hash: bytes.Clone(hash)}}
}

func repairM1086Vote() *tmproto.Vote {
	vote := types.Vote{Type: tmproto.PrevoteType, Height: 10, Round: 2,
		BlockID: repairM1086Block(), Timestamp: time.Unix(1700000000, 0).UTC(),
		ValidatorAddress: bytes.Repeat([]byte{3}, crypto.AddressSize)}
	return vote.ToProto()
}

func repairM1086Proposal() *tmproto.Proposal {
	proposal := types.Proposal{Type: tmproto.ProposalType, Height: 10, Round: 2, POLRound: -1,
		BlockID: repairM1086Block(), Timestamp: time.Unix(1700000000, 0).UTC()}
	pb := proposal.ToProto()
	pb.Signature = nil
	return pb
}

func repairM1086Verify(t *testing.T, pubKey crypto.PubKey, signBytes, raw []byte) {
	t.Helper()
	require.NotEmpty(t, raw)
	signature, err := crypto.SigFromBytes(raw)
	require.NoError(t, err)
	require.NoError(t, pubKey.Verify(signBytes, signature))
	require.Error(t, pubKey.Verify(append(bytes.Clone(signBytes), 0), signature))
}
