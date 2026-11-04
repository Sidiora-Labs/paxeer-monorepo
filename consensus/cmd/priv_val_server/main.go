package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/paxeer-network/paxlog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	tmnet "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/net"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	grpcprivval "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval/grpc"
	privvalproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

var logger = paxlog.NewLogger("tendermint", "cmd", "priv-val-server")

func main() {
	var (
		addr             = flag.String("addr", "127.0.0.1:26659", "Address to listen on (host:port)")
		chainID          = flag.String("chain-id", "mychain", "chain id")
		privValKeyPath   = flag.String("priv-key", "", "priv val key file path")
		privValStatePath = flag.String("priv-state", "", "priv val state file path")
		insecure         = flag.Bool("insecure", false, "allow server to run insecurely (no TLS)")
		certFile         = flag.String("certfile", "", "absolute path to server certificate")
		keyFile          = flag.String("keyfile", "", "absolute path to server key")
		rootCA           = flag.String("rootcafile", "", "absolute path to root CA")
		prometheusAddr   = flag.String("prometheus-addr", "", "address for prometheus endpoint (host:port)")
	)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info(
		"Starting private validator",
		"addr", *addr,
		"chainID", *chainID,
		"privKeyPath", *privValKeyPath,
		"privStatePath", *privValStatePath,
		"insecure", *insecure,
		"certFile", *certFile,
		"keyFile", *keyFile,
		"rootCA", *rootCA,
	)

	pv, err := privval.LoadFilePV(*privValKeyPath, *privValStatePath)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	opts := []grpc.ServerOption{}
	if !*insecure {
		certificate, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load X509 key pair: %v", err)
			os.Exit(1)
		}

		certPool := x509.NewCertPool()
		bs, err := os.ReadFile(*rootCA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read client ca cert: %s", err)
			os.Exit(1)
		}

		if ok := certPool.AppendCertsFromPEM(bs); !ok {
			fmt.Fprintf(os.Stderr, "failed to append client certs")
			os.Exit(1)
		}

		tlsConfig := &tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
			MinVersion:   tls.VersionTLS13,
		}

		creds := grpc.Creds(credentials.NewTLS(tlsConfig))
		opts = append(opts, creds)
		logger.Info("SignerServer: Creating security credentials")
	} else {
		logger.Info("SignerServer: You are using an insecure gRPC connection!")
	}

	protocol, address := tmnet.ProtocolAndAddress(*addr)

	lis, err := net.Listen(protocol, address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SignerServer: Failed to listen %v", err)
		os.Exit(1)
	}

	s, reg := newSignerServer(*chainID, pv, opts...)

	metrics := utils.None[prometheusServer]()
	if *prometheusAddr != "" {
		httpSrv, err := registerPrometheus(*prometheusAddr, reg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to start a http server: %v", err)
			os.Exit(1)
		}
		metrics = utils.Some(httpSrv)
	}

	if err := serveSigner(ctx, s, lis, metrics); err != nil {
		fmt.Fprintf(os.Stderr, "SignerServer: %v", err)
		os.Exit(1)
	}
}

func newSignerServer(chainID string, pv types.PrivValidator, opts ...grpc.ServerOption) (*grpc.Server, *prometheus.Registry) {
	// Create a metrics registry and standard server metrics for this signer.
	reg := prometheus.NewRegistry()
	grpcMetrics := grpc_prometheus.NewServerMetrics()
	reg.MustRegister(grpcMetrics)

	// add prometheus metrics for unary RPC calls
	opts = append(opts, grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor()))
	s := grpc.NewServer(opts...)
	privvalproto.RegisterPrivValidatorAPIServer(s, grpcprivval.NewSignerServer(chainID, pv))
	grpcMetrics.InitializeMetrics(s)
	return s, reg
}

type prometheusServer struct {
	server   *http.Server
	listener net.Listener
}

func registerPrometheus(addr string, reg *prometheus.Registry) (prometheusServer, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return prometheusServer{}, err
	}
	// create http server to serve prometheus
	return prometheusServer{
		server: &http.Server{
			Handler:           promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
			Addr:              addr,
			ReadHeaderTimeout: 5 * time.Second,
		},
		listener: lis,
	}, nil
}

func serveSigner(ctx context.Context, s *grpc.Server, lis net.Listener, metrics utils.Option[prometheusServer]) error {
	return scope.Run(ctx, func(ctx context.Context, tasks scope.Scope) error {
		defer tasks.Cancel(nil)
		// Install shutdown supervision before either server begins accepting work.
		tasks.SpawnBg(func() error {
			<-ctx.Done()
			return stopSigner(context.WithoutCancel(ctx), s, metrics)
		})
		if httpSrv, ok := metrics.Get(); ok {
			tasks.SpawnBg(func() error {
				defer tasks.Cancel(nil)
				err := httpSrv.server.Serve(httpSrv.listener)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return fmt.Errorf("unable to serve metrics: %w", err)
				}
				return nil
			})
		}
		logger.Info("SignerServer: Starting grpc server")
		if err := s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("unable to serve signer: %w", err)
		}
		return nil
	})
}

func stopSigner(ctx context.Context, s *grpc.Server, metrics utils.Option[prometheusServer]) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return scope.Run(ctx, func(ctx context.Context, tasks scope.Scope) error {
		drained := make(chan struct{})
		tasks.Spawn(func() error {
			defer close(drained)
			s.GracefulStop()
			return nil
		})
		tasks.SpawnBg(func() error {
			select {
			case <-drained:
			case <-ctx.Done():
				s.Stop()
			}
			return nil
		})
		if httpSrv, ok := metrics.Get(); ok {
			tasks.Spawn(func() error {
				if err := httpSrv.server.Shutdown(ctx); err != nil {
					return errors.Join(fmt.Errorf("unable to stop http server: %w", err), httpSrv.server.Close())
				}
				return nil
			})
		}
		return nil
	})
}
