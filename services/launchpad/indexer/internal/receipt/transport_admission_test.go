package receipt

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBoundedTransportRejectsInterruptedAndStreamingResponses(t *testing.T) {
	for _, mode := range []string{"stream exceeds bound", "interrupted body", "connection unavailable"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "interrupted body" {
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, "{")
					return
				}
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				_, _ = io.WriteString(w, strings.Repeat("x", rpcScalarResponseBytes+1))
			}))
			defer server.Close()
			if mode == "connection unavailable" {
				server.Close()
			}
			client, closeClient, err := NewBoundedRPCClient(context.Background(), server.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			defer closeClient()
			if _, err = client.ChainID(context.Background()); err == nil {
				t.Fatal("incomplete or unbounded response admitted")
			}
		})
	}
}

func TestArchiveTransportPreservesCancellationAndRequestErrors(t *testing.T) {
	ctx := context.Background()
	upstream := http.DefaultTransport.(*http.Transport).Clone()
	defer upstream.CloseIdleConnections()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	archiveURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := &boundedRPCTransport{lifetime: ctx, calls: make(chan struct{}, 1), upstream: upstream, historical: archiveURL}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(`{"method":"eth_chainId","params":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err = transport.archiveExchange(canceled, request, 1024); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled archive: %v", err)
	}
	sentinel := errors.New("request body unavailable")
	broken := request.Clone(ctx)
	broken.GetBody = func() (io.ReadCloser, error) { return nil, sentinel }
	if _, _, err = transport.archiveExchange(ctx, broken, 1024); !errors.Is(err, sentinel) {
		t.Fatalf("archive body error: %v", err)
	}
	if _, err = transport.RoundTrip(broken); !errors.Is(err, sentinel) {
		t.Fatalf("primary body error: %v", err)
	}
	window, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	response, _, err := transport.archiveExchange(window, request, 1024)
	if err != nil || response == nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("bounded overload response %+v: %v", response, err)
	}
	if len(transport.calls) != 0 {
		t.Fatal("archive call permit retained")
	}
	transport.lifetime = canceled
	if _, err = transport.RoundTrip(request); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed lifetime: %v", err)
	}
	transport.lifetime = ctx
	transport.calls <- struct{}{}
	if _, err = transport.RoundTrip(request.Clone(canceled)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled primary: %v", err)
	}
	<-transport.calls
	plain, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = transport.exchangeVia(canceled, nil, plain, 1024); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled response observation: %v", err)
	}
}
