package internaltest

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestFreePortsReturnsDistinctLoopbackPorts(t *testing.T) {
	ports := FreePorts(t, 3)
	if len(ports) != 3 {
		t.Fatalf("ports = %v", ports)
	}
	seen := make(map[int]bool)
	for _, port := range ports {
		if port <= 0 || seen[port] {
			t.Fatalf("ports are not distinct: %v", ports)
		}
		seen[port] = true
	}
}

func TestWalletSignsPersonalMessage(t *testing.T) {
	wallet := NewWallet(t)
	message := "gateway identity fixture"
	signature, err := hex.DecodeString(strings.TrimPrefix(wallet.Sign(t, message), "0x"))
	if err != nil || len(signature) != 65 {
		t.Fatalf("signature length=%d err=%v", len(signature), err)
	}
	if signature[64] != 27 && signature[64] != 28 {
		t.Fatalf("recovery id = %d", signature[64])
	}
	signature[64] -= 27
	key, err := crypto.SigToPub(accounts.TextHash([]byte(message)), signature)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.ToLower(crypto.PubkeyToAddress(*key).Hex()); got != wallet.Addr {
		t.Fatalf("recovered wallet = %s, want %s", got, wallet.Addr)
	}
}

func TestInfrastructureFixturesPreserveStoredData(t *testing.T) {
	ctx := context.Background()
	rdb, uri := NewRedis(t)
	if !strings.HasPrefix(uri, "redis://") {
		t.Fatalf("fixture Redis URL has unexpected scheme: %q", uri)
	}
	if err := rdb.Set(ctx, "gateway-fixture", "ready", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if value, err := rdb.Get(ctx, "gateway-fixture").Result(); err != nil || value != "ready" {
		t.Fatalf("fixture Redis value=%q err=%v", value, err)
	}
	bucket := "gateway-fixture-" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	minio := NewMinIO(t, bucket)
	if minio.Endpoint == "" || minio.AccessKeyID == "" || minio.SecretAccessKey == "" {
		t.Fatal("object fixture has incomplete connection details")
	}
	client := minio.Client(t, bucket)
	body := []byte("gateway fixture object")
	if err := client.Put(ctx, "fixture.txt", bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Delete(context.Background(), "fixture.txt") })
	second := NewStore(t, bucket)
	object, err := second.Get(ctx, "fixture.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	stored, err := io.ReadAll(object.Body)
	if err != nil || !bytes.Equal(stored, body) {
		t.Fatalf("fixture object=%q err=%v", stored, err)
	}
}
