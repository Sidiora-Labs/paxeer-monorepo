package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
)

func TestAggregateIngressReadsAgainstSharedByteBudget(t *testing.T) {
	db, _ := internaltest.NewRedis(t)
	reader := &aggregateIngressReader{ReadCloser: io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), 40000))), ctx: context.Background(), db: db, key: "gateway-upload-budget", limit: 1 << 20}
	buffer := make([]byte, 40000)
	if n, err := reader.Read(buffer[:0]); n != 0 || err != nil {
		t.Fatalf("empty ingress read: n=%d err=%v", n, err)
	}
	n, err := reader.Read(buffer)
	if err != nil || n != 32768 || !bytes.Equal(buffer[:n], bytes.Repeat([]byte("x"), n)) {
		t.Fatalf("bounded ingress read: n=%d err=%v", n, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	reader = &aggregateIngressReader{ReadCloser: io.NopCloser(bytes.NewReader([]byte("pending"))), ctx: ctx, db: db, key: "gateway-upload-busy", limit: 1}
	if n, err := reader.Read(make([]byte, 8)); n != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("byte budget cancellation: n=%d err=%v", n, err)
	}
}

func TestIngressReaderEnforcesElapsedThroughput(t *testing.T) {
	w := httptest.NewRecorder()
	controller := http.NewResponseController(w)
	reader := &ingressReader{ReadCloser: io.NopCloser(bytes.NewReader([]byte("a"))), controller: controller, window: time.Now().Add(-6 * time.Second), deadline: time.Now().Add(time.Second)}
	buffer := make([]byte, 8)
	if n, err := reader.Read(buffer); n != 1 || !errors.Is(err, errSlowIngress) {
		t.Fatalf("slow ingress: n=%d err=%v", n, err)
	}
	reader = &ingressReader{ReadCloser: io.NopCloser(bytes.NewReader([]byte("accepted"))), controller: controller, window: time.Now(), deadline: time.Now().Add(time.Second)}
	if n, err := reader.Read(buffer); n != len("accepted") || err != nil || string(buffer[:n]) != "accepted" {
		t.Fatalf("admitted ingress: n=%d err=%v body=%q", n, err, buffer[:n])
	}
	reader.deadline = time.Now().Add(-time.Millisecond)
	if n, err := reader.Read(buffer); n != 0 || !errors.Is(err, errSlowIngress) {
		t.Fatalf("expired ingress: n=%d err=%v", n, err)
	}
}
