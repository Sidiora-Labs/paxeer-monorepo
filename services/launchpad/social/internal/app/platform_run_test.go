package app

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestPlatformRunServesHealthAndDrainsBothListeners(t *testing.T) {
	h := newPlatformHarness(t)
	cfg := platformConfig(h, h.db)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"NODE_ENV": "production", "RUNTIME_PROFILE": "platform", "ZONE": cfg.RuntimeZone,
		"METRICS_ENVIRONMENT": cfg.RuntimeMetricsEnvironment, "BIND_ADDRESS": "0.0.0.0",
		"PORT": strconv.Itoa(port), "EXPECTED_SCHEMA_FILE": cfg.ExpectedSchemaFile,
		"EXPECTED_SCHEMA_SHA256": cfg.ExpectedSchemaSHA256, "IDENTITY_SERVICE_URL": cfg.IdentityURL,
		"IDENTITY_MTLS_CA_FILE":          cfg.IdentityMTLSCAFile,
		"IDENTITY_MTLS_CLIENT_CERT_FILE": cfg.IdentityMTLSClientCertFile,
		"IDENTITY_MTLS_CLIENT_KEY_FILE":  cfg.IdentityMTLSClientKeyFile,
		"IDENTITY_MTLS_PEER_CERT_SHA256": cfg.IdentityMTLSPeerCertSHA256,
		"SOCIAL_PRIVATE_LISTEN_ADDRESS":  cfg.PrivateListenAddress,
		"SOCIAL_TLS_CERT_FILE":           cfg.PrivateTLSCertFile,
		"SOCIAL_TLS_KEY_FILE":            cfg.PrivateTLSKeyFile,
		"SOCIAL_TLS_CLIENT_CA_FILE":      cfg.PrivateTLSClientCAFile,
		"GATEWAY_PEER_CERT_SHA256":       cfg.GatewayPeerCertSHA256,
		"SOCIAL_AUTHORITY_PEERS":         cfg.AuthorityPeers,
		"CORS_ALLOWED_ORIGINS":           cfg.CORSAllowedOrigins,
	} {
		t.Setenv(key, value)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()
	client := &http.Client{Timeout: time.Second}
	address := "http://127.0.0.1:" + strconv.Itoa(port) + "/health/live"
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(address)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("platform shutdown: %v", err)
					}
				case <-time.After(15 * time.Second):
					t.Fatal("platform listeners did not drain")
				}
				return
			}
		}
		select {
		case err := <-done:
			t.Fatalf("platform listener stopped before health check: %v", err)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	t.Fatalf("platform health listener did not become ready: %v", <-done)
}
