package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	upstream, directory := os.Args[1], os.Args[2]
	cert, err := os.ReadFile(filepath.Join(directory, "cert.pem"))
	must(err)
	pair, err := tls.LoadX509KeyPair(filepath.Join(directory, "cert.pem"), filepath.Join(directory, "key.pem"))
	must(err)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cert) {
		panic("certificate unavailable")
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	must(err)
	connected, err := proxy.NewConnected(proxy.ConnectedDeps{Name: "metadata", URL: upstream, Routes: proxy.MetadataConnectedRoutes(), SigningKey: key, Identity: auth.HTTPIdentityAuthority{Endpoint: upstream}, TLS: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{pair}}})
	must(err)
	observed, err := os.OpenFile(filepath.Join(directory, "requests.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	must(err)
	defer observed.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(observed, r.URL.RequestURI())
		connected.ServeHTTP(w, r)
	}))
	defer server.Close()
	fmt.Println(server.URL)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
