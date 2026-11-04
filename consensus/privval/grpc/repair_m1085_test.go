package grpc_test

import (
	"context"
	stded25519 "crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmgrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval/grpc"
	privvalproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1085Server(t *testing.T, pv *privval.FilePV, opts ...grpc.ServerOption) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(opts...)
	privvalproto.RegisterPrivValidatorAPIServer(server, tmgrpc.NewSignerServer(chainID, pv))
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			server.Stop()
			err := <-served
			if !errors.Is(err, grpc.ErrServerStopped) {
				require.NoError(t, err)
			}
		})
	}
	t.Cleanup(stop)
	return listener.Addr().String(), stop
}

func repairM1085Connection(t *testing.T, address string) *grpc.ClientConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() {
		if conn.GetState() != connectivity.Shutdown {
			require.NoError(t, conn.Close())
		}
	})
	return conn
}

func TestRepairM_1085_CloseConnection(t *testing.T) {
	pv, _ := repair171FilePV(t)
	address, _ := repairM1085Server(t, pv)
	for range 3 {
		conn := repairM1085Connection(t, address)
		client, err := tmgrpc.NewSignerClient(conn, chainID)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		pk, err := client.GetPubKey(ctx)
		require.NoError(t, err)
		require.Equal(t, pv.Key.PubKey, pk)
		require.Equal(t, connectivity.Ready, conn.GetState())
		require.NoError(t, client.Close())
		require.Equal(t, connectivity.Shutdown, conn.GetState())
		require.NoError(t, client.Close())
		_, err = privvalproto.NewPrivValidatorAPIClient(conn).GetPubKey(ctx, &privvalproto.PubKeyRequest{ChainId: chainID})
		require.Error(t, err)
		cancel()
	}
}

func TestRepairM_1085_NilPayload(t *testing.T) {
	pv, files := repair171FilePV(t)
	address, _ := repairM1085Server(t, pv)
	conn := repairM1085Connection(t, address)
	wire := privvalproto.NewPrivValidatorAPIClient(conn)
	direct := tmgrpc.NewSignerServer(chainID, pv)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	before, err := os.ReadFile(files.state)
	require.NoError(t, err)
	pub, err := direct.GetPubKey(ctx, nil)
	require.Nil(t, pub)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	vote, err := direct.SignVote(ctx, nil)
	require.Nil(t, vote)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	proposal, err := direct.SignProposal(ctx, nil)
	require.Nil(t, proposal)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	// The generated client and codec send empty protobuf messages over real HTTP/2.
	for _, chain := range []string{"", chainID} {
		vote, err = wire.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: chain})
		require.Nil(t, vote)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		proposal, err = wire.SignProposal(ctx, &privvalproto.SignProposalRequest{ChainId: chain})
		require.Nil(t, proposal)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	pub, err = wire.GetPubKey(ctx, &privvalproto.PubKeyRequest{})
	require.Nil(t, pub)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	after, err := os.ReadFile(files.state)
	require.NoError(t, err)
	require.Equal(t, before, after)
	client, err := tmgrpc.NewSignerClient(conn, chainID)
	require.NoError(t, err)
	p := repair171Proposal(1, 1)
	require.NoError(t, client.SignProposal(ctx, chainID, p))
	repair171Verify(t, pv.Key.PubKey, types.ProposalSignBytes(chainID, p), p.Signature)
	v := repair171Vote(pv, 1, 1)
	require.NoError(t, client.SignVote(ctx, chainID, v))
	repair171Verify(t, pv.Key.PubKey, types.VoteSignBytes(chainID, v), v.Signature)
}

func TestRepairM_1085_SafetyState(t *testing.T) {
	testSignerServerSafetyState(t)
}

type repairM1085TLSFiles struct {
	cert, key, ca string
	server        *tls.Config
}

func repairM1085TLS(t *testing.T) repairM1085TLSFiles {
	t.Helper()
	pub, key, err := stded25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "signer test CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, pub, key)
	require.NoError(t, err)
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "loopback signer"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, caTemplate, pub, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	dir := t.TempDir()
	files := repairM1085TLSFiles{cert: filepath.Join(dir, "client.pem"), key: filepath.Join(dir, "client.key"), ca: filepath.Join(dir, "ca.pem")}
	require.NoError(t, os.WriteFile(files.cert, certPEM, 0600))
	require.NoError(t, os.WriteFile(files.key, keyPEM, 0600))
	require.NoError(t, os.WriteFile(files.ca, caPEM, 0600))
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(caPEM))
	files.server = &tls.Config{Certificates: []tls.Certificate{certificate}, ClientCAs: pool,
		ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS13}
	return files
}

func TestRepairM_1085_TLSLoaderErrors(t *testing.T) {
	files := repairM1085TLS(t)
	invalid := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(invalid, []byte("invalid PEM"), 0600))
	missing := filepath.Join(t.TempDir(), "missing.pem")
	for _, tc := range []struct {
		name, cert, key, ca string
		missing             bool
	}{
		{"missing-certificate", missing, files.key, files.ca, true},
		{"missing-key", files.cert, missing, files.ca, true},
		{"missing-ca", files.cert, files.key, missing, true},
		{"invalid-certificate", invalid, files.key, files.ca, false},
		{"invalid-key", files.cert, invalid, files.ca, false},
		{"invalid-ca", files.cert, files.key, invalid, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			option, err := tmgrpc.GenerateTLS(tc.cert, tc.key, tc.ca)
			require.Error(t, err)
			require.Nil(t, option)
			if tc.missing {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
			cfg := config.DefaultPrivValidatorConfig()
			cfg.ListenAddr = "grpc://127.0.0.1:1"
			cfg.ClientCertificate, cfg.ClientKey, cfg.RootCA = tc.cert, tc.key, tc.ca
			client, err := tmgrpc.DialRemoteSigner(t.Context(), cfg, chainID, false)
			require.Error(t, err)
			require.Nil(t, client)
			if tc.missing {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
	// A subsequent real authenticated signing call proves errors did not kill the host.
	pv, _ := repair171FilePV(t)
	address, _ := repairM1085Server(t, pv, grpc.Creds(credentials.NewTLS(files.server)))
	cfg := config.DefaultPrivValidatorConfig()
	cfg.ListenAddr = "grpc://" + address
	cfg.ClientCertificate, cfg.ClientKey, cfg.RootCA = files.cert, files.key, files.ca
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, err := tmgrpc.DialRemoteSigner(ctx, cfg, chainID, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	v := repair171Vote(pv, 1, 1)
	require.NoError(t, client.SignVote(ctx, chainID, v))
	repair171Verify(t, pv.Key.PubKey, types.VoteSignBytes(chainID, v), v.Signature)
}

func TestRepairM_1085_TLSSelection(t *testing.T) {
	files := repairM1085TLS(t)
	pv, state := repair171FilePV(t)
	address, _ := repairM1085Server(t, pv)
	before, err := os.ReadFile(state.state)
	require.NoError(t, err)
	for mask := 0; mask < 8; mask++ {
		for _, allowInsecure := range []bool{false, true} {
			t.Run(fmt.Sprintf("tuple-%03b/insecure-%t", mask, allowInsecure), func(t *testing.T) {
				cfg := config.DefaultPrivValidatorConfig()
				cfg.ListenAddr = "grpc://" + address
				cfg.Insecure = allowInsecure
				if mask&1 != 0 {
					cfg.ClientCertificate = files.cert
				}
				if mask&2 != 0 {
					cfg.ClientKey = files.key
				}
				if mask&4 != 0 {
					cfg.RootCA = files.ca
				}
				full := config.DefaultConfig()
				full.PrivValidator = cfg
				if mask == 7 && !allowInsecure {
					require.NoError(t, cfg.ValidateBasic())
					require.NoError(t, full.ValidateBasic())
					require.True(t, cfg.AreSecurityOptionsPresent())
					return // The complete TLS tuple is exercised against mTLS in TLSLoaderErrors.
				}
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				client, err := tmgrpc.DialRemoteSigner(ctx, cfg, chainID, false)
				if mask == 0 && allowInsecure {
					require.NoError(t, cfg.ValidateBasic())
					require.NoError(t, full.ValidateBasic())
					require.NoError(t, err)
					require.NotNil(t, client)
					t.Cleanup(func() { require.NoError(t, client.Close()) })
					pk, err := client.GetPubKey(ctx)
					require.NoError(t, err)
					require.Equal(t, pv.Key.PubKey, pk)
				} else {
					require.Error(t, cfg.ValidateBasic())
					require.Error(t, full.ValidateBasic())
					require.Error(t, err)
					require.Nil(t, client)
				}
			})
		}
	}
	after, err := os.ReadFile(state.state)
	require.NoError(t, err)
	require.Equal(t, before, after)
	cfg := config.DefaultPrivValidatorConfig()
	cfg.ListenAddr = address // Direct callers also require explicit insecure selection.
	client, err := tmgrpc.DialRemoteSigner(t.Context(), cfg, chainID, false)
	require.Error(t, err)
	require.Nil(t, client)
}

func TestRepairM_1085_DialError(t *testing.T) {
	client, err := tmgrpc.NewSignerClient(nil, chainID)
	require.Error(t, err)
	require.Nil(t, client)
	client, err = tmgrpc.DialRemoteSigner(t.Context(), nil, chainID, false)
	require.Error(t, err)
	require.Nil(t, client)
	cfg := config.DefaultPrivValidatorConfig()
	cfg.Insecure = true
	cfg.ListenAddr = "grpc://dns:///" // The real DNS resolver rejects a missing endpoint.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, err = tmgrpc.DialRemoteSigner(ctx, cfg, chainID, false)
	require.Error(t, err)
	require.Nil(t, client)
}
