package metadata_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
)

func TestDevelopmentLegacyImageUploadAndReadback(t *testing.T) {
	ctx := context.Background()
	pool, dsn := internaltest.NewPostgresURL(t)
	redisURL := internaltest.NewRedisURL(t)
	applyIndexerPools(t, pool)
	objects := internaltest.NewMinIO(t, "metadata-legacy-image")
	scannerHost, scannerPort := internaltest.NewClamd(t)
	if scannerHost == "" || scannerPort == 0 {
		t.Fatal("scanner required for image publication")
	}
	gateway := internaltest.NewGateway(t, dsn, redisURL, objects, devOrigin)
	wallet, bearer := gateway.SignIn(t)
	_, otherBearer := gateway.SignIn(t)
	chain := newDevChain(t)
	supply, _ := new(big.Int).SetString("1000000000000000", 10)
	receipt := chain.emitTokenDeployed(t, devToken, devPool, wallet, supply)
	seedDevPool(t, pool, common.HexToAddress(devPool).Hex(), common.HexToAddress(devToken).Hex(), common.HexToAddress(wallet).Hex(), receipt.TxHash.Hex(), receipt.BlockNumber.Int64())
	cfg := config.Config{
		NodeEnv: "development", DatabaseURL: dsn, RedisURL: redisURL, RPCURL: chain.url,
		S3Endpoint: objects.Endpoint, S3Bucket: objects.Bucket, S3AccessKeyID: objects.AccessKeyID, S3SecretAccessKey: objects.SecretAccessKey, S3Region: "us-east-1",
		LogLevel: "info", Port: 3000, MaxLogoSizeBytes: 2 << 20, MaxBannerSizeBytes: 5 << 20,
		CORSAllowedOrigins: devOrigin, ClamAVHost: scannerHost, ClamAVPort: scannerPort,
		IdentityServiceURL: gateway.URL, IdentityServiceToken: gateway.ServiceToken,
	}
	instance, err := app.New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(instance.Close)
	server := httptest.NewServer(instance.Router)
	t.Cleanup(server.Close)
	logo := rasterBytes(t, "png", 18, 12)
	post := func(token string, image []byte) (*http.Response, []byte) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("metadata", `{"name":"Legacy Falcon","symbol":"LFC","description":"Creator image"}`); err != nil {
			t.Fatal(err)
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="logo"; filename="logo.png"`)
		header.Set("Content-Type", "image/png")
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(image); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequest(http.MethodPost, server.URL+"/metadata/"+devToken, &body)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Origin", devOrigin)
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response, raw
	}
	if response, raw := post(otherBearer, logo); response.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign creator upload: %d %s", response.StatusCode, raw)
	}
	if response, raw := post(bearer, logo); response.StatusCode != http.StatusOK || !strings.Contains(string(raw), `"metadata_updated":true`) {
		t.Fatalf("creator upload: %d %s", response.StatusCode, raw)
	}
	row, err := sqlcdb.New(pool).GetTokenMetadata(ctx, devToken)
	if err != nil || row.Name.String != "Legacy Falcon" || row.CreatedBy != wallet {
		t.Fatalf("persisted metadata: %+v %v", row, err)
	}
	response, err := server.Client().Get(fmt.Sprintf("%s/logo/%s.png", server.URL, devToken))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	stored, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(stored, logo) {
		t.Fatalf("published image: status %d bytes %d, %v", response.StatusCode, len(stored), err)
	}
}
