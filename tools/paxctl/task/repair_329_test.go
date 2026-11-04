package task

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	sidecar "github.com/sei-protocol/seictl/sidecar/client"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/server"
	"github.com/sei-protocol/seictl/sidecar/tasks"
	"github.com/urfave/cli/v3"
	"k8s.io/client-go/rest"
)

const repair329Host = "validator-0.validator.chain.svc.cluster.local"

type repair329PKI struct {
	ca   *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
	file string
}

func repair329CA(t *testing.T) *repair329PKI {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sidecar-proxy-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	bundle := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	file := filepath.Join(t.TempDir(), "proxy-ca.pem")
	if err := os.WriteFile(file, bundle, 0600); err != nil {
		t.Fatal(err)
	}
	return &repair329PKI{ca: cert, key: key, pem: bundle, file: file}
}

func (p *repair329PKI) certificate(t *testing.T, host string, client bool) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if client {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, p.ca, &key.PublicKey, p.key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert, certPEM, keyPEM
}

func repair329TLS(t *testing.T, pki *repair329PKI, host string, handler http.Handler, clientAuth bool) *httptest.Server {
	t.Helper()
	cert, _, _ := pki.certificate(t, host, false)
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	if clientAuth {
		pool := x509.NewCertPool()
		pool.AddCert(pki.ca)
		srv.TLS.ClientCAs = pool
		srv.TLS.ClientAuth = tls.RequireAndVerifyClientCert
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

// Only the TCP destination changes for local qualification. The production URL,
// SNI, hostname verification, TLS handshake, auth wrappers and HTTP codec run.
func repair329Config(t *testing.T, srv *httptest.Server, expectedPort int32) *rest.Config {
	t.Helper()
	return &rest.Config{
		Host: "https://api-server.invalid", BearerToken: "repair-329-credential",
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			want := net.JoinHostPort(repair329Host, strconv.Itoa(int(expectedPort)))
			if address != want {
				return nil, fmt.Errorf("dial address = %s, want %s", address, want)
			}
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		},
	}
}

func repair329Backend(t *testing.T) (*engine.Engine, string) {
	t.Helper()
	store, err := engine.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	eng := engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{
		engine.TaskMarkNotReady: tasks.NewMarkNotReadier(store).Handler(),
	}, store)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	srv := server.NewServer(addr, eng, t.TempDir(), server.AuthnModeTrustedHeader)
	stopped := make(chan error, 1)
	go func() { stopped <- srv.ListenAndServe(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-stopped:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("sidecar did not stop")
		}
	})
	// Wait on the real listener's liveness handler before constructing the proxy.
	hc := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := hc.Get("http://" + addr + "/v0/livez")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return eng, "http://" + addr
			}
		}
		select {
		case err := <-stopped:
			t.Fatalf("sidecar stopped during startup: %v", err)
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("sidecar did not start")
	return nil, ""
}

func TestRepair_329_TLSSubmission(t *testing.T) {
	eng, backend := repair329Backend(t)
	pki := repair329CA(t)
	target, err := url.Parse(backend)
	if err != nil {
		t.Fatal(err)
	}
	var authenticated atomic.Int32
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		if pr.In.TLS == nil || len(pr.In.TLS.VerifiedChains) == 0 {
			t.Error("proxy received no verified TLS caller identity")
			return
		}
		if pr.In.TLS.ServerName != repair329Host {
			t.Errorf("SNI = %q", pr.In.TLS.ServerName)
		}
		if pr.In.Header.Get("Authorization") != "Bearer repair-329-credential" {
			t.Error("kubeconfig bearer credential did not reach TLS proxy")
		}
		authenticated.Add(1)
		pr.SetURL(target)
		pr.Out.Header.Del("Authorization")
		pr.Out.Header.Set("X-Remote-User", pr.In.TLS.VerifiedChains[0][0].Subject.CommonName)
	}}
	srv := repair329TLS(t, pki, repair329Host, proxy, true)
	cfg := repair329Config(t, srv, sidecarProxyPort)
	_, certPEM, keyPEM := pki.certificate(t, "task-operator", true)
	// API-server policy must neither disable verification nor choose the proxy CA/name.
	cfg.TLSClientConfig = rest.TLSClientConfig{
		Insecure: true, ServerName: "api-server.invalid", CAData: []byte("API CA is not proxy trust"),
		CertData: certPEM, KeyData: keyPEM,
	}
	sc, err := newSidecarClient(cfg, "chain", "validator", 0, pki.file)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	got, err := sc.SubmitTask(context.Background(), sidecar.TaskRequest{Id: &id, Type: string(engine.TaskMarkNotReady)})
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("submitted ID = %s, want %s", got, id)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := sc.GetTask(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == sidecar.Completed {
			stored := eng.GetResult(id.String())
			if stored == nil || stored.Status != engine.TaskStatusCompleted || stored.Type != string(engine.TaskMarkNotReady) {
				t.Fatalf("real engine did not persist submitted hold: %+v", stored)
			}
			if authenticated.Load() < 2 {
				t.Fatal("submission and result did not both traverse TLS proxy")
			}
			if !cfg.Insecure || cfg.ServerName != "api-server.invalid" || string(cfg.CAData) != "API CA is not proxy trust" {
				t.Fatal("proxy client mutated API-server configuration")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("real hold handler did not complete")
}

func TestRepair_329_RejectUntrustedProxy(t *testing.T) {
	for _, kind := range []string{"wrong-ca", "wrong-identity"} {
		t.Run(kind, func(t *testing.T) {
			ca := repair329CA(t)
			signingCA := ca
			host := repair329Host
			if kind == "wrong-ca" {
				signingCA = repair329CA(t)
			} else {
				host = "other-0.other.chain.svc.cluster.local"
			}
			var requests atomic.Int32
			srv := repair329TLS(t, signingCA, host, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				t.Error("HTTP request reached an untrusted proxy")
			}), false)
			cfg := repair329Config(t, srv, 9443)
			// Even an insecure API config trusting the wrong peer must not weaken proxy trust.
			cfg.TLSClientConfig = rest.TLSClientConfig{Insecure: true, ServerName: host, CAData: signingCA.pem}
			sc, err := newSidecarClient(cfg, "chain", "validator", 9443, ca.file)
			if err != nil {
				t.Fatal(err)
			}
			_, err = sc.SubmitTask(context.Background(), sidecar.TaskRequest{Type: string(engine.TaskMarkNotReady)})
			if err == nil {
				t.Fatal("untrusted proxy accepted")
			}
			if !strings.Contains(err.Error(), "certificate") {
				t.Fatalf("expected certificate verification failure: %v", err)
			}
			if requests.Load() != 0 {
				t.Fatal("credentials crossed an unverified TLS connection")
			}
		})
	}
}

func TestRepair_329_RejectRedirect(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			pki := repair329CA(t)
			var redirected atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
			t.Cleanup(destination.Close)
			location := strings.Replace(destination.URL, "http://", scheme+"://", 1) + "/v0/tasks"
			var requests atomic.Int32
			srv := repair329TLS(t, pki, repair329Host, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Authorization") != "Bearer repair-329-credential" {
					t.Error("missing authenticated request")
				}
				http.Redirect(w, r, location, http.StatusTemporaryRedirect)
			}), false)
			cfg := repair329Config(t, srv, 8443)
			// A followed redirect must reach the actual destination even with the local DNS routing.
			originalDial := cfg.Dial
			cfg.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				if strings.HasPrefix(address, repair329Host+":") {
					return originalDial(ctx, network, address)
				}
				redirected.Add(1)
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}
			sc, err := newSidecarClient(cfg, "chain", "validator", 8443, pki.file)
			if err != nil {
				t.Fatal(err)
			}
			_, err = sc.SubmitTask(context.Background(), sidecar.TaskRequest{Type: string(engine.TaskMarkNotReady)})
			if err == nil || !strings.Contains(err.Error(), "307") {
				t.Fatalf("expected redirect rejection, got %v", err)
			}
			if requests.Load() != 1 || redirected.Load() != 0 {
				t.Fatalf("requests=%d redirected=%d", requests.Load(), redirected.Load())
			}
		})
	}
}

func TestRepair_329_RequireExplicitTrust(t *testing.T) {
	pki := repair329CA(t)
	badCA := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(badCA, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, node, ns, ca string
		port               int32
		custom             bool
	}{
		{name: "missing-ca", node: "validator", ns: "chain", port: 8443},
		{name: "unreadable-ca", node: "validator", ns: "chain", ca: filepath.Join(t.TempDir(), "absent.pem"), port: 8443},
		{name: "invalid-ca", node: "validator", ns: "chain", ca: badCA, port: 8443},
		{name: "invalid-node", node: "validator/other", ns: "chain", ca: pki.file, port: 8443},
		{name: "invalid-namespace", node: "validator", ns: "chain@other", ca: pki.file, port: 8443},
		{name: "negative-port", node: "validator", ns: "chain", ca: pki.file, port: -1},
		{name: "overflow-port", node: "validator", ns: "chain", ca: pki.file, port: 65536},
		{name: "custom-transport", node: "validator", ns: "chain", ca: pki.file, port: 8443, custom: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &rest.Config{TLSClientConfig: rest.TLSClientConfig{CAData: pki.pem}}
			if tc.custom {
				cfg.Transport = http.DefaultTransport
			}
			if _, err := newSidecarClient(cfg, tc.ns, tc.node, tc.port, tc.ca); err == nil {
				t.Fatal("unsafe proxy configuration accepted")
			}
		})
	}
}

func TestRepair_329_AllVerbsExposeProxyTrust(t *testing.T) {
	for _, cmd := range Cmd.Commands {
		t.Run(cmd.Name, func(t *testing.T) {
			var proxyCA *cli.StringFlag
			for _, flag := range cmd.Flags {
				if f, ok := flag.(*cli.StringFlag); ok && f.Name == "proxy-ca-file" {
					proxyCA = f
				}
			}
			if proxyCA == nil {
				t.Fatal("task verb lacks explicit proxy CA flag")
			}
		})
	}
}
