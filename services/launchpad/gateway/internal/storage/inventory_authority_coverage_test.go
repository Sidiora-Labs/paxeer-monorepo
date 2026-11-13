package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	sharedstorage "github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
)

func TestConnectedInventoryAndSignedDownloadUseOwnedObject(t *testing.T) {
	a, ctx := connectedAuthority(t)
	_, key := seedConnectedAsset(t, a, ctx)
	if _, err := a.Pool.Exec(ctx, a.InventoryReportSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	report := InventoryReport{Checked: 1, GeneratedAt: time.Now().UTC()}
	if err := a.PersistInventory(ctx, "", approvedPrefix, report); !errors.Is(err, ErrConflict) {
		t.Fatalf("inventory without bucket: %v", err)
	}
	bucket := fmt.Sprintf("l41-authorized-%d", time.Now().UnixNano())
	if err := a.PersistInventory(ctx, bucket, approvedPrefix, report); err != nil {
		t.Fatalf("inventory persistence: %v", err)
	}
	var checked int
	if err := a.Pool.QueryRow(ctx, "SELECT checked FROM "+a.table("inventory_reports")+" WHERE bucket=$1", bucket).Scan(&checked); err != nil || checked != 1 {
		t.Fatalf("stored inventory: checked=%d err=%v", checked, err)
	}
	connectedProductionView(t, a, "390001.sql", "CREATE TABLE \"metadata\".\"asset_details_v2\"", "\"metadata\"", pgx.Identifier{a.MetadataSchema}.Sanitize())
	purgeAge, quarantineAge, err := a.MediaBacklogAges(ctx)
	if err != nil || purgeAge != 0 || quarantineAge != 0 {
		t.Fatalf("empty backlog: purge=%v quarantine=%v err=%v", purgeAge, quarantineAge, err)
	}
	backend := internaltest.NewMinIO(t, bucket)
	store := backend.Client(t, bucket)
	body := []byte("authorized immutable bytes")
	if err := store.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "image/png"); err != nil {
		t.Fatal(err)
	}
	upstream, err := url.Parse(backend.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(upstream))
	defer proxy.Close()
	client, raw, err := NewScopedObjectClient(ctx, sharedstorage.Config{Endpoint: proxy.URL, Region: "us-east-1", Bucket: bucket, AccessKeyID: backend.AccessKeyID, SecretAccessKey: backend.SecretAccessKey})
	if err != nil || client == nil || raw == nil {
		t.Fatalf("scoped object client: client=%v raw=%v err=%v", client, raw, err)
	}
	signed, err := a.PresignDownload(ctx, s3.NewPresignClient(raw), bucket, key, time.Minute)
	if err != nil || !strings.HasPrefix(signed, proxy.URL+"/") {
		t.Fatalf("signed approved download: url=%q err=%v", signed, err)
	}
	response, err := proxy.Client().Get(signed)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(got, body) {
		t.Fatalf("signed object response: status=%d body=%q err=%v", response.StatusCode, got, err)
	}
	if _, err := a.PresignDownload(context.Background(), nil, bucket, key, time.Minute); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing signing client: %v", err)
	}
}
