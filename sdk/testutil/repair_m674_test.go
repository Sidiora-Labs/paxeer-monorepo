package testutil

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRepairM_674_ResponseAndHeaders(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			requests := make(chan string, 1)
			payload := []byte("response\x00with bytes")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Method + " " + r.URL.RequestURI() + " " + r.Header.Get("X-Repair")
				w.WriteHeader(status)
				_, _ = w.Write(payload)
			}))
			defer server.Close()

			body, err := GetRequestWithHeaders(server.URL+"/query?key=value", map[string]string{"X-Repair": "header-value"})
			if err != nil || !bytes.Equal(body, payload) {
				t.Fatalf("body = %q, err = %v; want %q", body, err, payload)
			}
			if got := <-requests; got != "GET /query?key=value header-value" {
				t.Fatalf("request = %q", got)
			}
		})
	}
}

func TestRepairM_674_ResponseSizeBound(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		for _, size := range []int{maxGetResponseBytes - 1, maxGetResponseBytes, maxGetResponseBytes + 1} {
			t.Run(strconv.FormatBool(streamed)+"/"+strconv.Itoa(size), func(t *testing.T) {
				payload := bytes.Repeat([]byte{'x'}, size)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if streamed {
						w.(http.Flusher).Flush()
					} else {
						w.Header().Set("Content-Length", strconv.Itoa(size))
					}
					_, _ = w.Write(payload)
				}))
				defer server.Close()

				body, err := GetRequestWithHeaders(server.URL, nil)
				if size > maxGetResponseBytes {
					if err == nil || !strings.Contains(err.Error(), "exceeds "+strconv.Itoa(maxGetResponseBytes)+" bytes") || body != nil {
						t.Fatalf("oversized response: len(body) = %d, err = %v", len(body), err)
					}
				} else if err != nil || !bytes.Equal(body, payload) {
					t.Fatalf("allowed response: len(body) = %d, err = %v; want %d bytes", len(body), err, size)
				}
			})
		}
	}
}

func TestRepairM_674_OversizeClosesStreamingBody(t *testing.T) {
	disconnected := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = io.CopyN(w, bytes.NewReader(bytes.Repeat([]byte{'x'}, maxGetResponseBytes+1)), maxGetResponseBytes+1)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(disconnected)
	}))
	defer server.Close()
	defer server.CloseClientConnections()

	body, err := GetRequestWithHeaders(server.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "exceeds") || body != nil {
		t.Fatalf("oversized stream: len(body) = %d, err = %v", len(body), err)
	}
	select {
	case <-disconnected:
	case <-time.After(2 * time.Second):
		t.Fatal("oversized response body was not closed promptly")
	}
}

func TestRepairM_674_RequestDeadline(t *testing.T) {
	for _, sendHeaders := range []bool{false, true} {
		t.Run(strconv.FormatBool(sendHeaders), func(t *testing.T) {
			disconnected := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if sendHeaders {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("partial"))
					w.(http.Flusher).Flush()
				}
				select {
				case <-r.Context().Done():
					close(disconnected)
				case <-time.After(3 * getRequestTimeout):
					w.WriteHeader(http.StatusGatewayTimeout)
				}
			}))
			defer server.Close()
			defer server.CloseClientConnections()

			start := time.Now()
			body, err := GetRequestWithHeaders(server.URL, nil)
			elapsed := time.Since(start)
			var timeout net.Error
			if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &timeout) || !timeout.Timeout() || body != nil {
				t.Fatalf("stalled response: body = %q, err = %v; want deadline error", body, err)
			}
			if elapsed > getRequestTimeout+2*time.Second {
				t.Fatalf("request took %s; timeout = %s", elapsed, getRequestTimeout)
			}
			select {
			case <-disconnected:
			case <-time.After(2 * time.Second):
				t.Fatal("timed-out request left its server connection open")
			}
		})
	}
}

func TestRepairM_674_PreservesReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("truncated"))
	}))
	defer server.Close()

	body, err := GetRequestWithHeaders(server.URL, nil)
	if !errors.Is(err, io.ErrUnexpectedEOF) || body != nil {
		t.Fatalf("truncated response: body = %q, err = %v; want unexpected EOF", body, err)
	}
}

func TestRepairM_674_PreservesTransportError(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("untrusted TLS request reached the handler")
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()

	body, err := GetRequestWithHeaders(server.URL, nil)
	var requestErr *url.Error
	var certificateErr x509.UnknownAuthorityError
	if body != nil || !errors.As(err, &requestErr) || !errors.As(err, &certificateErr) {
		t.Fatalf("TLS failure: body = %q, err = %v; want wrapped certificate error", body, err)
	}
	if requestErr.Op != "Get" || requestErr.URL != server.URL {
		t.Fatalf("transport error lost request details: %+v", requestErr)
	}
}
