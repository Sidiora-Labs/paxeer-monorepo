package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// These fixtures retain the real SDK, HTTP transport and storage client. Only
// the S3 peer is a loopback server backed by objects uploaded through Client.Put.
type bufferLimitObject struct {
	data        []byte
	contentType string
}

type bufferLimitBody struct {
	io.ReadCloser
	read   atomic.Int64
	closed atomic.Int64
}

func (b *bufferLimitBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.read.Add(int64(n))
	return n, err
}

func (b *bufferLimitBody) Close() error {
	b.closed.Add(1)
	return b.ReadCloser.Close()
}

type bufferLimitTransport struct {
	base     http.RoundTripper
	requests atomic.Int64
	body     *bufferLimitBody
}

func (tr *bufferLimitTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	tr.requests.Add(1)
	response, err := tr.base.RoundTrip(r)
	if err == nil && r.Method == http.MethodGet && response.StatusCode == http.StatusOK {
		tr.body = &bufferLimitBody{ReadCloser: response.Body}
		response.Body = tr.body
	}
	return response, err
}

func bufferLimitClient(t *testing.T, mode string) (*Client, *bufferLimitTransport) {
	t.Helper()
	var mu sync.Mutex
	objects := make(map[string]bufferLimitObject)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repair-m981/") {
			http.Error(w, "unexpected bucket", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			objects[r.URL.Path] = bufferLimitObject{data: data, contentType: r.Header.Get("Content-Type")}
			mu.Unlock()
			w.Header().Set("ETag", `"uploaded"`)
		case http.MethodGet:
			mu.Lock()
			object, exists := objects[r.URL.Path]
			mu.Unlock()
			if !exists {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, "<Error><Code>NoSuchKey</Code><Message>absent</Message></Error>")
				return
			}
			w.Header().Set("Content-Type", object.contentType)
			w.Header().Set("ETag", `"uploaded"`)
			switch mode {
			case "known":
				w.Header().Set("Content-Length", strconv.Itoa(len(object.data)))
			case "truncated":
				w.Header().Set("Content-Length", strconv.Itoa(len(object.data)+1))
			case "chunked":
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
			default:
				http.Error(w, "unknown fixture mode", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(object.data)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	transport := &bufferLimitTransport{base: server.Client().Transport}
	raw := s3.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("fixture-key", "fixture-secret", ""),
		HTTPClient:  &http.Client{Transport: transport},
		Retryer:     func() aws.Retryer { return aws.NopRetryer{} },
	}, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(server.URL)
		options.UsePathStyle = true
	})
	return NewFromS3(raw, "repair-m981"), transport
}

func bufferLimitPut(t *testing.T, client *Client, data []byte) {
	t.Helper()
	if err := client.Put(context.Background(), "objects/asset", bytes.NewReader(data), int64(len(data)), "image/png"); err != nil {
		t.Fatalf("upload fixture through production Put: %v", err)
	}
}

func bufferLimitClosed(t *testing.T, transport *bufferLimitTransport, wantRead int64) {
	t.Helper()
	if transport.body == nil {
		t.Fatal("no successful HTTP GET response body observed")
	}
	if got := transport.body.read.Load(); got != wantRead {
		t.Errorf("body bytes consumed = %d, want %d", got, wantRead)
	}
	if got := transport.body.closed.Load(); got != 1 {
		t.Errorf("body closed %d times, want exactly once", got)
	}
}

func TestBufferLimitBufferBoundaries(t *testing.T) {
	for _, mode := range []string{"known", "chunked"} {
		for _, size := range []int{0, 31, 32, 33} {
			t.Run(mode+"/"+strconv.Itoa(size), func(t *testing.T) {
				client, transport := bufferLimitClient(t, mode)
				want := bytes.Repeat([]byte{0x89}, size)
				bufferLimitPut(t, client, want)
				var reader Reader = client
				got, contentType, err := reader.GetBytes(context.Background(), "objects/asset", 32)
				if size > 32 {
					if !errors.Is(err, ErrObjectTooLarge) || got != nil || contentType != "" {
						t.Fatalf("oversize result = (%v, %q, %v), want no content and ErrObjectTooLarge", got, contentType, err)
					}
					consumed := int64(33)
					if mode == "known" {
						consumed = 0
					}
					bufferLimitClosed(t, transport, consumed)
					return
				}
				if err != nil || !bytes.Equal(got, want) || contentType != "image/png" {
					t.Fatalf("buffered round trip = (%x, %q, %v), want (%x, image/png, nil)", got, contentType, err, want)
				}
				bufferLimitClosed(t, transport, int64(size))
			})
		}
	}
}

func TestBufferLimitMaximumPlusOne(t *testing.T) {
	for _, mode := range []string{"known", "chunked"} {
		t.Run(mode, func(t *testing.T) {
			client, transport := bufferLimitClient(t, mode)
			bufferLimitPut(t, client, bytes.Repeat([]byte("x"), 1<<20))
			got, contentType, err := client.GetBytes(context.Background(), "objects/asset", 16)
			if !errors.Is(err, ErrObjectTooLarge) || got != nil || contentType != "" {
				t.Fatalf("large result = (%v, %q, %v), want ErrObjectTooLarge without partial content", got, contentType, err)
			}
			consumed := int64(17)
			if mode == "known" {
				consumed = 0
			}
			bufferLimitClosed(t, transport, consumed)
		})
	}
}

func TestBufferLimitIncorrectSize(t *testing.T) {
	client, transport := bufferLimitClient(t, "chunked")
	bufferLimitPut(t, client, bytes.Repeat([]byte("x"), 1024))
	obj, err := client.Get(context.Background(), "objects/asset")
	if err != nil {
		t.Fatal(err)
	}
	if obj.Size != -1 {
		t.Fatalf("unknown Content-Length size = %d, want -1", obj.Size)
	}
	// Fault-inject understated metadata after a real producer/HTTP/SDK read;
	// the production buffered consumer must still enforce its own byte budget.
	obj.Size = 1
	got, contentType, err := readBufferedObject("objects/asset", obj, 16)
	if !errors.Is(err, ErrObjectTooLarge) || got != nil || contentType != "" {
		t.Fatalf("understated size result = (%v, %q, %v), want ErrObjectTooLarge", got, contentType, err)
	}
	bufferLimitClosed(t, transport, 17)
}

func TestBufferLimitBudgetValidation(t *testing.T) {
	client, transport := bufferLimitClient(t, "known")
	for _, limit := range []int64{-1, math.MinInt64, math.MaxInt64} {
		got, contentType, err := client.GetBytes(context.Background(), "objects/asset", limit)
		if !errors.Is(err, ErrInvalidBufferLimit) || got != nil || contentType != "" {
			t.Fatalf("limit %d = (%v, %q, %v), want ErrInvalidBufferLimit", limit, got, contentType, err)
		}
	}
	if got := transport.requests.Load(); got != 0 {
		t.Fatalf("invalid budgets issued %d HTTP requests", got)
	}
	bufferLimitPut(t, client, nil)
	got, contentType, err := client.GetBytes(context.Background(), "objects/asset", 0)
	if err != nil || len(got) != 0 || contentType != "image/png" {
		t.Fatalf("zero budget empty object = (%v, %q, %v)", got, contentType, err)
	}
	bufferLimitClosed(t, transport, 0)
	bufferLimitPut(t, client, []byte("x"))
	got, contentType, err = client.GetBytes(context.Background(), "objects/asset", 0)
	if !errors.Is(err, ErrObjectTooLarge) || got != nil || contentType != "" {
		t.Fatalf("zero budget nonempty object = (%v, %q, %v)", got, contentType, err)
	}
	bufferLimitClosed(t, transport, 0)
	got, contentType, err = client.GetBytes(context.Background(), "objects/asset", math.MaxInt64-1)
	if err != nil || string(got) != "x" || contentType != "image/png" {
		t.Fatalf("largest valid budget = (%v, %q, %v)", got, contentType, err)
	}
	bufferLimitClosed(t, transport, 1)
}

func TestBufferLimitReadFailures(t *testing.T) {
	client, transport := bufferLimitClient(t, "truncated")
	bufferLimitPut(t, client, []byte("short"))
	got, contentType, err := client.GetBytes(context.Background(), "objects/asset", 32)
	if !errors.Is(err, io.ErrUnexpectedEOF) || got != nil || contentType != "" {
		t.Fatalf("truncated body = (%v, %q, %v), want no partial result and unexpected EOF", got, contentType, err)
	}
	bufferLimitClosed(t, transport, 5)
	got, contentType, err = client.GetBytes(context.Background(), "missing", 32)
	if !errors.Is(err, ErrNotFound) || got != nil || contentType != "" {
		t.Fatalf("missing result = (%v, %q, %v)", got, contentType, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, contentType, err = client.GetBytes(ctx, "objects/asset", 32)
	if !errors.Is(err, context.Canceled) || got != nil || contentType != "" {
		t.Fatalf("canceled result = (%v, %q, %v)", got, contentType, err)
	}
}

func TestBufferLimitStreamingLargerObject(t *testing.T) {
	client, transport := bufferLimitClient(t, "known")
	want := bytes.Repeat([]byte("stream"), 1024)
	bufferLimitPut(t, client, want)
	if _, _, err := client.GetBytes(context.Background(), "objects/asset", 16); !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("buffered read = %v, want ErrObjectTooLarge", err)
	}
	bufferLimitClosed(t, transport, 0)
	obj, err := client.Get(context.Background(), "objects/asset")
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	_, readErr := io.Copy(&got, obj.Body)
	closeErr := obj.Body.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(got.Bytes(), want) || obj.Size != int64(len(want)) || obj.ContentType != "image/png" {
		t.Fatalf("streamed round trip: bytes=%d size=%d type=%q read=%v close=%v", got.Len(), obj.Size, obj.ContentType, readErr, closeErr)
	}
	bufferLimitClosed(t, transport, int64(len(want)))
}
