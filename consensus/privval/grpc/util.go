package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/paxeer-network/paxlog"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"google.golang.org/grpc/keepalive"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnet "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/net"
)

var logger = paxlog.NewLogger("tendermint", "privval", "grpc")

// DefaultDialOptions constructs a list of grpc dial options
func DefaultDialOptions(
	extraOpts ...grpc.DialOption,
) []grpc.DialOption {
	const (
		retries            = 50 // 50 * 100ms = 5s total
		timeout            = 1 * time.Second
		maxCallRecvMsgSize = 1 << 20 // Default 5Mb
	)

	var kacp = keepalive.ClientParameters{
		Time:    10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout: 2 * time.Second,  // wait 2 seconds for ping ack before considering the connection dead
	}

	opts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(timeout)),
	}

	dialOpts := make([]grpc.DialOption, 0, 3+len(extraOpts))
	dialOpts = append(dialOpts,
		grpc.WithKeepaliveParams(kacp),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxCallRecvMsgSize),
			grpc_retry.WithMax(retries),
		),
		grpc.WithUnaryInterceptor(
			grpc_retry.UnaryClientInterceptor(opts...),
		),
	)

	dialOpts = append(dialOpts, extraOpts...)

	return dialOpts
}

func GenerateTLS(certPath, keyPath, ca string) (grpc.DialOption, error) {
	certificate, err := tls.LoadX509KeyPair(
		certPath,
		keyPath,
	)
	if err != nil {
		return nil, fmt.Errorf("load signer TLS certificate and key: %w", err)
	}

	certPool := x509.NewCertPool()
	bs, err := os.ReadFile(filepath.Clean(ca))
	if err != nil {
		return nil, fmt.Errorf("read signer TLS root CA: %w", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, fmt.Errorf("invalid signer TLS root CA PEM in %q", ca)
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS13,
	})

	return grpc.WithTransportCredentials(transportCreds), nil
}

// DialRemoteSigner is  a generalized function to dial the gRPC server.
func DialRemoteSigner(
	ctx context.Context,
	cfg *config.PrivValidatorConfig,
	chainID string,
	usePrometheus bool,
) (*SignerClient, error) {
	if err := cfg.ValidateBasic(); err != nil {
		return nil, err
	}
	var transportSecurity grpc.DialOption
	if cfg.AreSecurityOptionsPresent() {
		var err error
		transportSecurity, err = GenerateTLS(cfg.ClientCertificateFile(),
			cfg.ClientKeyFile(), cfg.RootCAFile())
		if err != nil {
			return nil, err
		}
	} else {
		if !cfg.Insecure {
			return nil, fmt.Errorf("signer gRPC requires TLS or explicit insecure transport")
		}
		transportSecurity = grpc.WithTransportCredentials(insecure.NewCredentials())
		logger.Info("Using an insecure gRPC connection!")
	}

	dialOptions := DefaultDialOptions()
	if usePrometheus {
		grpcMetrics := grpc_prometheus.DefaultClientMetrics
		dialOptions = append(dialOptions, grpc.WithUnaryInterceptor(grpcMetrics.UnaryClientInterceptor()))
	}

	dialOptions = append(dialOptions, transportSecurity)

	_, address := tmnet.ProtocolAndAddress(cfg.ListenAddr)
	conn, err := grpc.DialContext(ctx, address, dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("dial signer %q: %w", address, err)
	}

	return NewSignerClient(conn, chainID)
}
