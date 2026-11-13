package streams

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestConnectedReadinessChecksProviderBudgetAndVault(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.BudgetSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	f.useConnectedRuntime()
	f.connectedConfig.VaultTransitURL = f.transit.Endpoint
	f.connectedConfig.VaultTransitToken = f.transit.Token
	f.connectedConfig.VaultTransitKey = f.transit.Key
	f.close()
	f.open()
	_, err := f.database.Exec(ctx, "INSERT INTO livestream.stream_vendor_usage(period_start,vendor_user_id,viewer_minutes,delivery_minutes,transcode_minutes,storage_minutes,cost_units,query_from,query_to,source_revision) VALUES(date_trunc('month',clock_timestamp() AT TIME ZONE 'UTC')::date,'repair_vendor',0,0,0,0,0,clock_timestamp()-interval '1 hour',clock_timestamp(),'fixture')")
	if err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	leaf := pki.Leaf("provider", "livepeer.studio")
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/stream":
			if r.URL.RawQuery != "streamsonly=true" {
				t.Errorf("provider readiness query = %s", r.URL)
			}
			_, _ = w.Write([]byte(`[]`))
		case "/api/webhook/playback_hook":
			_, _ = fmt.Fprintf(w, `{"id":"playback_hook","url":%q,"sharedSecret":%q,"events":["playback.accessControl"]}`, f.connectedConfig.PlaybackWebhookURL, f.connectedConfig.PlaybackWebhookSecret)
		default:
			t.Errorf("unexpected provider readiness path = %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{leaf.Pair}}
	server.StartTLS()
	defer server.Close()
	vaultTransport := f.transit.Client.Transport.(*http.Transport)
	roots := vaultTransport.TLSClientConfig.RootCAs.Clone()
	providerCA, err := os.ReadFile(pki.CAFile)
	if err != nil || !roots.AppendCertsFromPEM(providerCA) {
		t.Fatalf("provider CA = %v", err)
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if strings.HasPrefix(address, "livepeer.studio:") {
			address = server.Listener.Addr().String()
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = previous; transport.CloseIdleConnections() }()
	readyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := f.handlers.providerReady(readyCtx); err != nil {
		t.Fatalf("connected provider readiness = %v", err)
	}
	if _, err := f.database.Exec(ctx, "DELETE FROM livestream.stream_vendor_usage WHERE vendor_user_id='repair_vendor'"); err != nil {
		t.Fatal(err)
	}
	if err := f.handlers.providerReady(readyCtx); err == nil {
		t.Fatal("missing current vendor usage accepted")
	}
}
