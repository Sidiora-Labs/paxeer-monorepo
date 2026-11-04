package serve

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// These tests exercise the actual gateway response producers directly. The
// PostgreSQL authority and Redis purge receipt contract belong to storage;
// neither is replaced by a test implementation of its interface here.
type repair50Asset struct {
	body []byte
	hash string
	key  string
	file string
}

func repair50NewAsset(t *testing.T) *repair50Asset {
	t.Helper()
	body := []byte("owned approved raster bytes for downstream cache qualification")
	digest := sha256.Sum256(body)
	hash := hex.EncodeToString(digest[:])
	file := filepath.Join(t.TempDir(), hash+".png")
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	return &repair50Asset{body: body, hash: hash, key: "approved/" + hash + "/256/" + hash + ".png", file: file}
}

// emit uses the same buffered and streamed response functions as Handler.serve,
// including its missing-object response, without inventing an S3 Reader.
func (a *repair50Asset) emit(t *testing.T, buffered, approved bool) http.HandlerFunc {
	t.Helper()
	h := New(Deps{})
	return func(w http.ResponseWriter, r *http.Request) {
		if approved {
			w = approvedResponseWriter{ResponseWriter: w}
		}
		body, err := os.Open(a.file)
		if os.IsNotExist(err) {
			h.missing(w, r, "token", a.key)
			return
		}
		if err != nil {
			t.Error(err)
			http.Error(w, "origin unavailable", 502)
			return
		}
		defer body.Close()
		if buffered {
			data, err := io.ReadAll(body)
			if err != nil {
				t.Error(err)
				http.Error(w, "origin unavailable", 502)
				return
			}
			h.write(w, r, "image/png", a.hash, bytes.NewReader(data), int64(len(data)))
		} else {
			h.stream(w, r, "image/png", a.hash, body, int64(len(a.body)))
		}
	}
}

func repair50Read(t *testing.T, client *http.Client, method, url string, headers http.Header, status int, body []byte, noStore bool) http.Header {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = headers.Clone()
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	got, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status {
		t.Fatalf("%s %s: status=%d want=%d body=%q", method, url, res.StatusCode, status, got)
	}
	if body != nil && !bytes.Equal(got, body) {
		t.Fatalf("%s %s: body=%q want=%q", method, url, got, body)
	}
	if noStore {
		for _, name := range []string{"Cache-Control", "CDN-Cache-Control"} {
			if values := res.Header.Values(name); len(values) != 1 || values[0] != "no-store" {
				t.Fatalf("%s %s: %s=%q", method, url, name, values)
			}
		}
	}
	return res.Header
}

func TestRepair_50_ResponsePaths(t *testing.T) {
	a := repair50NewAsset(t)
	client := &http.Client{Timeout: 5 * time.Second}
	for _, buffered := range []bool{false, true} {
		t.Run(fmt.Sprintf("buffered_%t", buffered), func(t *testing.T) {
			server := httptest.NewServer(a.emit(t, buffered, true))
			t.Cleanup(server.Close)
			url := server.URL + "/" + a.key
			head := repair50Read(t, client, "HEAD", url, nil, 200, []byte{}, true)
			etag := head.Get("ETag")
			if etag == "" {
				t.Fatal("HEAD omitted ETag")
			}
			conditional := http.Header{"If-None-Match": {etag}}
			repair50Read(t, client, "GET", url, conditional, 304, []byte{}, true)
			repair50Read(t, client, "GET", url, nil, 200, a.body, true)
			repair50Read(t, client, "HEAD", url, nil, 200, []byte{}, true)
			repair50Read(t, client, "GET", url, conditional, 304, []byte{}, true)
			repair50Read(t, client, "HEAD", url, conditional, 304, []byte{}, true)
			part := repair50Read(t, client, "GET", url, http.Header{"Range": {"bytes=2-6"}}, 206, a.body[2:7], true)
			if part.Get("Content-Range") != fmt.Sprintf("bytes 2-6/%d", len(a.body)) {
				t.Fatal(part)
			}
			for _, value := range []string{"bytes=99999999-", "bytes=0-1,3-4"} {
				repair50Read(t, client, "GET", url, http.Header{"Range": {value}}, 416, nil, true)
			}
			if buffered {
				// ServeContent removes cache headers on precondition failures.
				repair50Read(t, client, "GET", url, http.Header{"If-Match": {`"other"`}}, 412, nil, true)
			}
		})
	}
	if err := os.Remove(a.file); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(a.emit(t, true, true))
	t.Cleanup(server.Close)
	for i := 0; i < 2; i++ {
		repair50Read(t, client, "GET", server.URL+"/"+a.key, nil, 404, nil, true)
		repair50Read(t, client, "HEAD", server.URL+"/"+a.key, nil, 404, []byte{}, true)
	}
}

func TestRepair_50_ApprovedDenials(t *testing.T) {
	router := chi.NewRouter()
	NewApproved(Deps{}, nil).RegisterRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{"private/original.png", "approved/" + strings.Repeat("a", 64) + "/png256"} {
		for _, method := range []string{"GET", "HEAD"} {
			headers := repair50Read(t, client, method, server.URL+"/media/token/"+path, nil, 404, nil, true)
			if headers.Get("X-Content-Type-Options") != "nosniff" || headers.Get("Content-Security-Policy") != "default-src 'none'; sandbox" {
				t.Fatal("missing approved security boundary", headers)
			}
		}
	}
}

func TestRepair_50_HeaderCommitment(t *testing.T) {
	for _, status := range []int{200, 206, 304, 400, 404, 412, 416, 502, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writer := approvedResponseWriter{ResponseWriter: recorder}
			// Delegates may overwrite or remove either policy before committing.
			writer.Header().Set("Cache-Control", immutableCacheControl)
			writer.Header().Add("Cache-Control", "public, max-age=30")
			writer.Header().Del("CDN-Cache-Control")
			writer.WriteHeader(status)
			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != status {
				t.Fatal(response.StatusCode, status)
			}
			for _, name := range []string{"Cache-Control", "CDN-Cache-Control"} {
				if values := response.Header.Values(name); len(values) != 1 || values[0] != "no-store" {
					t.Fatal(name, values)
				}
			}
			if writer.Unwrap() != recorder {
				t.Fatal("response controller cannot reach underlying writer")
			}
		})
	}
	recorder := httptest.NewRecorder()
	writer := approvedResponseWriter{ResponseWriter: recorder}
	writer.Header().Set("Cache-Control", immutableCacheControl)
	writer.Header().Set("CDN-Cache-Control", "public, max-age=31536000")
	body := []byte("implicit status")
	if n, err := writer.Write(body); err != nil || n != len(body) {
		t.Fatal(n, err)
	}
	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != 200 || !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Fatal(response.StatusCode, recorder.Body)
	}
	for _, name := range []string{"Cache-Control", "CDN-Cache-Control"} {
		if values := response.Header.Values(name); len(values) != 1 || values[0] != "no-store" {
			t.Fatal(name, values)
		}
	}
}

// Nginx is an actual HTTP cache, with separate policies for browser-style
// Cache-Control and CDN-Cache-Control. A cacheable control must yield a HIT.
func repair50Downstream(t *testing.T, origin string, cdn bool) (*http.Client, string) {
	t.Helper()
	binary, err := exec.LookPath("nginx")
	if err != nil {
		t.Fatal("repair_50 requires the nginx HTTP cache executable: ", err)
	}
	dir, err := os.MkdirTemp("", "repair50-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "http.sock")
	policy := ""
	if cdn {
		policy = "proxy_ignore_headers Cache-Control; proxy_no_cache $cdn_no_store;"
	}
	conf := fmt.Sprintf(`daemon off;
master_process off;
pid %s/nginx.pid;
error_log %s/error.log;
events { worker_connections 128; }
http {
 access_log off;
 proxy_cache_path %s/cache keys_zone=downstream:1m;
 proxy_temp_path %s/temp;
 map $upstream_http_cdn_cache_control $cdn_no_store { default 0; ~*no-store 1; }
 server {
  listen unix:%s;
  location / {
   proxy_pass %s;
   proxy_cache downstream;
   proxy_cache_key "$request_method:$request_uri";
   proxy_cache_convert_head off;
   proxy_cache_valid 200 1h;
   proxy_set_header If-None-Match $http_if_none_match;
   proxy_set_header If-Match $http_if_match;
   proxy_force_ranges on;
   add_header X-Repair-Cache $upstream_cache_status always;
   %s
  }
 }
}
`, dir, dir, dir, dir, socket, origin, policy)
	config := filepath.Join(dir, "nginx.conf")
	if err = os.WriteFile(config, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(filepath.Join(dir, "process.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { output.Close() })
	cmd := exec.Command(binary, "-p", dir+"/", "-c", config)
	cmd.Stdout, cmd.Stderr = output, output
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			cmd.Process.Kill()
			<-done
			t.Error("nginx did not stop")
		}
	})
	ready := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		conn, err := net.DialTimeout("unix", socket, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		logs, _ := os.ReadFile(filepath.Join(dir, "error.log"))
		process, _ := os.ReadFile(filepath.Join(dir, "process.log"))
		t.Fatalf("nginx did not listen: %s %s", logs, process)
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}, "http://downstream"
}

func TestRepair_50_ApprovedCachePurge(t *testing.T) {
	for _, cdn := range []bool{false, true} {
		t.Run(fmt.Sprintf("cdn_%t", cdn), func(t *testing.T) {
			a := repair50NewAsset(t)
			// Switch to the real Approved denial path once delivery is revoked. This
			// tests the no-store delivery contract, not a replacement storage authority.
			var approved atomic.Pointer[http.HandlerFunc]
			delivery := a.emit(t, true, true)
			approved.Store(&delivery)
			public := a.emit(t, true, false)
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/media/") {
					public.ServeHTTP(w, r)
					return
				}
				approved.Load().ServeHTTP(w, r)
			}))
			t.Cleanup(origin.Close)
			client, downstream := repair50Downstream(t, origin.URL, cdn)
			control := downstream + "/media/token/" + a.key
			for i, want := range []string{"MISS", "HIT"} {
				headers := repair50Read(t, client, "GET", control, nil, 200, a.body, false)
				if headers.Get("X-Repair-Cache") != want {
					t.Fatalf("control request %d: cache=%q want=%s", i, headers.Get("X-Repair-Cache"), want)
				}
				if headers.Get("Cache-Control") != immutableCacheControl || headers.Get("CDN-Cache-Control") != "public, max-age=31536000" {
					t.Fatal("public response policy changed", headers)
				}
			}
			url := downstream + "/" + a.key
			var etag string
			for i := 0; i < 2; i++ {
				headers := repair50Read(t, client, "GET", url, nil, 200, a.body, true)
				if headers.Get("X-Repair-Cache") != "MISS" {
					t.Fatal("approved copy retained downstream: ", headers)
				}
				etag = headers.Get("ETag")
			}
			if etag == "" {
				t.Fatal("missing approved ETag")
			}
			for i := 0; i < 2; i++ {
				repair50Read(t, client, "HEAD", url, nil, 200, []byte{}, true)
				repair50Read(t, client, "GET", url, http.Header{"If-None-Match": {etag}}, 304, []byte{}, true)
			}
			denied := NewApproved(Deps{}, nil)
			denyRouter := chi.NewRouter()
			// Register the actual connected URL against the same Approved handler;
			// unavailable authority fails closed before it consults any object store.
			denyRouter.Handle("/approved/*", denied)
			deniedHandler := http.HandlerFunc(denyRouter.ServeHTTP)
			approved.Store(&deniedHandler)
			repair50Read(t, client, "GET", url, nil, 404, nil, true)
			// Physically purge the local source, retaining the public cached control.
			// OriginCachePurgeProvider's database receipt proof is an out-of-path
			// storage-domain wiring request, not a synthesized completion receipt.
			if err := os.Remove(a.file); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				repair50Read(t, client, "GET", url, nil, 404, nil, true)
				repair50Read(t, client, "HEAD", url, nil, 404, []byte{}, true)
				repair50Read(t, client, "GET", url, http.Header{"If-None-Match": {etag}}, 404, nil, true)
				repair50Read(t, client, "GET", url, http.Header{"Range": {"bytes=0-3"}}, 404, nil, true)
			}
			headers := repair50Read(t, client, "GET", control, nil, 200, a.body, false)
			if headers.Get("X-Repair-Cache") != "HIT" {
				t.Fatal("downstream cache stopped retaining the control", headers)
			}
		})
	}
}
