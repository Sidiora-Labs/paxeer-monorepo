package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestArchive96HourReconstructionPostgres(t *testing.T) {
	for _, name := range []string{"L10_ARCHIVE_CHART_SCHEMA", "L10_ARCHIVE_MANIFESTS", "L10_ARCHIVE_POOL", "L10_ARCHIVE_FROM", "L10_ARCHIVE_TO", "L10_ARCHIVE_EXPECTED_DIGEST", "L10_TEST_RPC_URL", "L10_TEST_S3_BUCKET"} {
		if os.Getenv(name) == "" {
			t.Skipf("protected owner fixture unavailable: %s is not set", name)
		}
	}

	required := []string{"DELIVERY_TEST_DSN", "L10_ARCHIVE_CHART_SCHEMA", "L10_ARCHIVE_MANIFESTS", "L10_ARCHIVE_POOL", "L10_ARCHIVE_FROM", "L10_ARCHIVE_TO", "L10_ARCHIVE_EXPECTED_DIGEST", "L10_TEST_RPC_URL", "L10_TEST_S3_ENDPOINT", "L10_TEST_S3_BUCKET", "L10_TEST_S3_ACCESS_KEY_ID", "L10_TEST_S3_SECRET_ACCESS_KEY"}
	for _, name := range required {
		if os.Getenv(name) == "" {
			t.Fatalf("actual approved 96-hour fixture required: %s", name)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	config, err := pgxpool.ParseConfig(os.Getenv("DELIVERY_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.Host != "127.0.0.1" || config.ConnConfig.Port != 25432 || config.ConnConfig.Database != "kindle_fixture" {
		t.Fatal("refusing non-fixture PostgreSQL")
	}
	schema := os.Getenv("L10_ARCHIVE_CHART_SCHEMA")
	if !strings.HasPrefix(schema, "kd_") {
		t.Fatal("private chart schema required")
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	exact, err := store.NewExact(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := url.Parse(os.Getenv("L10_TEST_S3_ENDPOINT"))
	if err != nil || endpoint.Hostname() != "127.0.0.1" || !strings.HasPrefix(os.Getenv("L10_TEST_S3_BUCKET"), "kd-") {
		t.Fatal("private loopback object fixture required")
	}
	objects, err := storage.New(ctx, storage.Config{Endpoint: endpoint.String(), Region: "us-east-1", AccessKeyID: os.Getenv("L10_TEST_S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("L10_TEST_S3_SECRET_ACCESS_KEY"), Bucket: os.Getenv("L10_TEST_S3_BUCKET")})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(os.Getenv("L10_ARCHIVE_MANIFESTS"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(file, 16<<20+1))
	file.Close()
	if err != nil || len(raw) > 16<<20 {
		t.Fatal("manifest file bound", err)
	}
	var manifests []store.ArchiveManifest
	if err = json.Unmarshal(raw, &manifests); err != nil || len(manifests) == 0 {
		t.Fatal(err)
	}
	from, err := strconv.ParseInt(os.Getenv("L10_ARCHIVE_FROM"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	to, err := strconv.ParseInt(os.Getenv("L10_ARCHIVE_TO"), 10, 64)
	if err != nil || to-from != 96*3600 {
		t.Fatal("exact96-hour source interval required", err)
	}
	rpc, err := ethclient.DialContext(ctx, os.Getenv("L10_TEST_RPC_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer rpc.Close()
	chain, err := rpc.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(big.NewInt(125)) != 0 {
		t.Fatal("real chain125 required", err)
	}
	reader := store.ArchiveReader{Objects: objects, Bucket: objects.Bucket(), Exact: exact, BlockTime: archiveBlockTime(chainread.Reader{RPC: rpc})}
	firstBlock := manifests[0].Blocks[0]
	if _, err = reader.BlockTime(ctx, firstBlock.Height, firstBlock.Hash); err != nil {
		t.Fatal("committed node hash refused", err)
	}
	foreignHash := "0x" + strings.Repeat("f", 64)
	if foreignHash == firstBlock.Hash {
		foreignHash = "0x" + strings.Repeat("e", 64)
	}
	if _, err = reader.BlockTime(ctx, firstBlock.Height, foreignHash); err == nil {
		t.Fatal("foreign block identity accepted")
	}
	report, err := ReconstructArchive(ctx, exact, reader, "qualification_"+fmt.Sprint(time.Now().UnixNano()), os.Getenv("L10_ARCHIVE_POOL"), from, to, manifests)
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceFills == 0 || report.PromotionEpoch == 0 || report.ExpectedDigest != report.ActualDigest || report.ActualDigest != os.Getenv("L10_ARCHIVE_EXPECTED_DIGEST") {
		t.Fatal(report)
	}
	first := manifests[0]
	object, err := objects.Get(ctx, first.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(io.LimitReader(object.Body, first.ByteLength+1))
	object.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	payload[len(payload)/2] ^= 1
	if fills, err := store.ValidateArchive(payload, first); err == nil || fills != nil {
		t.Fatal("corrupt genuine archive accepted", err)
	}
	if _, err = reader.Read(ctx, store.ArchiveManifest{}); err == nil {
		t.Fatal("missing segment accepted")
	}
}

func archiveBlockTime(reader chainread.Reader) func(context.Context, int64, string) (int64, error) {
	return func(ctx context.Context, height int64, expected string) (int64, error) {
		block, err := reader.Block(ctx, height)
		if err != nil {
			return 0, err
		}
		if block.Hash != strings.ToLower(expected) {
			return 0, store.ErrIntegrity
		}
		return block.Time, nil
	}
}
