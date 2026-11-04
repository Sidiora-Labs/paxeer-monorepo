package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	grpcprivval "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval/grpc"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type repairM1150Signer struct {
	server     *grpc.Server
	listener   net.Listener
	metrics    utils.Option[prometheusServer]
	validator  *privval.FilePV
	chainID    string
	keyPath    string
	statePath  string
	terminated chan struct{}
	cancel     context.CancelFunc
	running    *scope.GlobalHandle[error]
}

func repairM1150NewSigner(t *testing.T, withMetrics bool, opts ...grpc.ServerOption) *repairM1150Signer {
	t.Helper()
	dir := t.TempDir()
	keyPath, statePath := filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json")
	pv := privval.NewFilePV(ed25519.TestSecretKey([]byte(t.Name())), keyPath, statePath)
	if err := pv.Save(); err != nil {
		t.Fatal(err)
	}
	pv, err := privval.LoadFilePV(keyPath, statePath)
	if err != nil {
		t.Fatal(err)
	}
	server, reg := newSignerServer(t.Name(), pv, opts...)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &repairM1150Signer{server: server, listener: listener, validator: pv, chainID: t.Name(), keyPath: keyPath, statePath: statePath, terminated: make(chan struct{})}
	t.Cleanup(func() {
		if f.cancel != nil {
			f.cancel()
		}
		f.server.Stop()
		_ = f.listener.Close()
		if metrics, ok := f.metrics.Get(); ok {
			_ = metrics.server.Close()
			_ = metrics.listener.Close()
		}
		if f.running != nil {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
			defer cancel()
			if _, err := f.running.Join(ctx); err != nil {
				t.Error(err)
			}
		}
	})
	if withMetrics {
		metrics, err := registerPrometheus("127.0.0.1:0", reg)
		if err != nil {
			t.Fatal(err)
		}
		f.metrics = utils.Some(metrics)
	}
	return f
}

func (f *repairM1150Signer) start(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	f.cancel = cancel
	f.running = scope.SpawnGlobal(func(context.Context) error {
		defer close(f.terminated)
		return serveSigner(ctx, f.server, f.listener, f.metrics)
	})
}

func (f *repairM1150Signer) join(t *testing.T) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err, joinErr := f.running.Join(ctx)
	if joinErr != nil {
		t.Fatalf("signer failed to join its serving and shutdown paths: %v", joinErr)
	}
	return err
}

func (f *repairM1150Signer) client(t *testing.T) *grpcprivval.SignerClient {
	t.Helper()
	conn, err := grpc.DialContext(t.Context(), f.listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client, err := grpcprivval.NewSignerClient(conn, f.chainID)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func (f *repairM1150Signer) publicKey(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	key, err := f.client(t).GetPubKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if key != f.validator.Key.PubKey {
		t.Fatal("RPC returned a different validator public key")
	}
}

func repairM1150AssertClosed(t *testing.T, lis net.Listener) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", lis.Addr().String(), time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("listener %s still accepts connections after shutdown joined", lis.Addr())
	}
}

func TestRepairM_1150_LiveServerTermination(t *testing.T) {
	f := repairM1150NewSigner(t, true)
	f.start(t)
	f.publicKey(t)
	metrics := f.metrics.OrPanic("metrics enabled")
	repairM1150Scrape(t, metrics)
	f.cancel()
	if err := f.join(t); err != nil {
		t.Fatalf("normal cancellation returned an error: %v", err)
	}
	repairM1150AssertClosed(t, f.listener)
	repairM1150AssertClosed(t, metrics.listener)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := f.client(t).GetPubKey(ctx); err == nil {
		t.Fatal("signer served a request after termination")
	}
}

func TestRepairM_1150_NormalServerStop(t *testing.T) {
	for _, withMetrics := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_metrics", true: "with_metrics"}[withMetrics], func(t *testing.T) {
			f := repairM1150NewSigner(t, withMetrics)
			f.start(t)
			f.publicKey(t)
			f.server.GracefulStop()
			if err := f.join(t); err != nil {
				t.Fatalf("normal gRPC stop returned an error: %v", err)
			}
			repairM1150AssertClosed(t, f.listener)
			if metrics, ok := f.metrics.Get(); ok {
				repairM1150AssertClosed(t, metrics.listener)
			}
		})
	}
}

func TestRepairM_1150_CancelBeforeServe(t *testing.T) {
	f := repairM1150NewSigner(t, true)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := serveSigner(ctx, f.server, f.listener, f.metrics); err != nil {
		t.Fatal(err)
	}
	repairM1150AssertClosed(t, f.listener)
	repairM1150AssertClosed(t, f.metrics.OrPanic("metrics enabled").listener)
}

func TestRepairM_1150_ServeFailureStopsPeer(t *testing.T) {
	for _, failedServer := range []string{"grpc", "metrics"} {
		t.Run(failedServer, func(t *testing.T) {
			f := repairM1150NewSigner(t, true)
			metrics := f.metrics.OrPanic("metrics enabled")
			failedListener := f.listener
			if failedServer == "metrics" {
				failedListener = metrics.listener
			}
			if err := failedListener.Close(); err != nil {
				t.Fatal(err)
			}
			f.start(t)
			if err := f.join(t); !errors.Is(err, net.ErrClosed) {
				t.Fatalf("expected listener failure to be propagated, got %v", err)
			}
			repairM1150AssertClosed(t, f.listener)
			repairM1150AssertClosed(t, metrics.listener)
		})
	}
}

func repairM1150Scrape(t *testing.T, metrics prometheusServer) map[string]float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+metrics.listener.Addr().String()+"/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics endpoint returned HTTP %d", resp.StatusCode)
	}
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]float64)
	for _, family := range []string{"grpc_server_started_total", "grpc_server_handled_total", "grpc_server_msg_received_total", "grpc_server_msg_sent_total"} {
		for _, metric := range families[family].GetMetric() {
			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["grpc_service"] != "tendermint.privval.PrivValidatorAPI" || labels["grpc_type"] != "unary" {
				continue
			}
			if family == "grpc_server_handled_total" && labels["grpc_code"] != "OK" {
				continue
			}
			values[family+"/"+labels["grpc_method"]] = metric.GetCounter().GetValue()
		}
	}
	return values
}

func TestRepairM_1150_SignerPrometheusMetrics(t *testing.T) {
	f := repairM1150NewSigner(t, true)
	f.start(t)
	metrics := f.metrics.OrPanic("metrics enabled")
	before := repairM1150Scrape(t, metrics)
	client := f.client(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	key, err := client.GetPubKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if key != f.validator.Key.PubKey {
		t.Fatal("RPC returned a different validator public key")
	}
	vote := (&types.Vote{
		Type:             tmproto.PrecommitType,
		Height:           1,
		Round:            0,
		Timestamp:        time.Unix(100, 0).UTC(),
		ValidatorAddress: key.Address(),
		ValidatorIndex:   0,
	}).ToProto()
	if err := client.SignVote(ctx, f.chainID, vote); err != nil {
		t.Fatal(err)
	}
	signature, err := crypto.SigFromBytes(vote.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if err := key.Verify(types.VoteSignBytes(f.chainID, vote), signature); err != nil {
		t.Fatalf("RPC signing result is invalid: %v", err)
	}
	after := repairM1150Scrape(t, metrics)
	for _, method := range []string{"GetPubKey", "SignVote"} {
		for _, family := range []string{"grpc_server_started_total", "grpc_server_handled_total", "grpc_server_msg_received_total", "grpc_server_msg_sent_total"} {
			name := family + "/" + method
			if got := after[name] - before[name]; got != 1 {
				t.Errorf("scraped %s increment = %v, want 1", name, got)
			}
		}
	}
	f.cancel()
	if err := f.join(t); err != nil {
		t.Fatal(err)
	}
	recovered, err := privval.LoadFilePV(f.keyPath, f.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.LastSignState.Height != vote.Height || string(recovered.LastSignState.Signature) != string(vote.Signature) {
		t.Fatal("real signing RPC did not persist its safety state")
	}
}

func TestRepairM_1150_DrainInFlightRPC(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	// This middleware only synchronizes the live RPC; the production signer
	// handler, file validator, gRPC transport and shutdown path all run normally.
	f := repairM1150NewSigner(t, true, grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		close(entered)
		select {
		case <-release:
			return handler(ctx, req)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))
	shutdownStarted := make(chan struct{})
	f.metrics.OrPanic("metrics enabled").server.RegisterOnShutdown(func() { close(shutdownStarted) })
	f.start(t)
	client := f.client(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	rpc := scope.SpawnGlobal(func(context.Context) error {
		key, err := client.GetPubKey(ctx)
		if err != nil {
			return err
		}
		if key != f.validator.Key.PubKey {
			return errors.New("RPC returned a different validator public key")
		}
		return nil
	})
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	f.cancel()
	select {
	case <-shutdownStarted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-f.terminated:
		t.Fatal("serving lifecycle returned before the in-flight RPC drained")
	default:
	}
	unblock()
	if err, joinErr := rpc.Join(ctx); joinErr != nil || err != nil {
		t.Fatalf("in-flight real public-key RPC did not complete: rpc=%v join=%v", err, joinErr)
	}
	if err := f.join(t); err != nil {
		t.Fatal(err)
	}
	repairM1150AssertClosed(t, f.listener)
	repairM1150AssertClosed(t, f.metrics.OrPanic("metrics enabled").listener)
}
