package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/gofrs/flock"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"
)

func chartReadJSON(t *testing.T, path string, out any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
}

func chartWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func chartReceiptFill(t *testing.T, chain internaltest.MarketChain) accounting.FillV1 {
	t.Helper()
	receipt := chain.Swaps[0]
	log := receipt.Logs[0]
	if len(log.Data) != 7*32 || len(log.Topics) != 3 {
		t.Fatal("complete Swap log required")
	}
	amount := func(index int) string { return new(big.Int).SetBytes(log.Data[index*32 : (index+1)*32]).String() }
	rawReceipt, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	header, err := chain.RPC.HeaderByNumber(context.Background(), receipt.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	rawHeader, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	receiptDigest, headerDigest := sha256.Sum256(rawReceipt), sha256.Sum256(rawHeader)
	block := chain.Headers[receipt.BlockNumber.Int64()]
	fill := accounting.FillV1{
		Version:       accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: receipt.BlockNumber.String(), TransactionIndex: strconv.FormatUint(uint64(receipt.TransactionIndex), 10), LogIndex: strconv.FormatUint(uint64(log.Index), 10), TransactionHash: strings.ToLower(receipt.TxHash.Hex()), BlockHash: block.Hash},
		ActualEmitter: strings.ToLower(log.Address.Hex()), PoolAddress: strings.ToLower(chain.Pool.Hex()),
		Token: accounting.Asset{Address: strings.ToLower(chain.Token.Hex()), Decimals: 6}, Settlement: accounting.Asset{Address: strings.ToLower(chain.Settlement.Hex()), Decimals: 6},
		Side: "buy", PaidInputRaw: amount(1), USDLAmountRaw: amount(1), TokenAmountRaw: amount(2), FeeRaw: amount(3), InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
		Attribution: accounting.Attribution{TransactionSender: strings.ToLower(chain.Sender.Hex()), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance: accounting.Provenance{DeploymentID: "chart-market-chain", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: hex.EncodeToString(receiptDigest[:]), HeaderSHA256: hex.EncodeToString(headerDigest[:])}, BlockTimestampSeconds: amount(5),
	}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	if err = accounting.ValidateFillV1(fill); err != nil {
		t.Fatal(err)
	}
	return fill
}

func assertComposedChartStartup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	lock := flock.New(filepath.Join(os.TempDir(), "charts-container-port.lock"))
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil || !locked {
		t.Fatalf("container listener lock: %v", err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			t.Error(err)
		}
	}()
	root := coordinatortest.RepositoryRoot(t)
	database := coordinatortest.Bootstrap(t, os.Getenv(internaltest.PostgresEnv), "candles_runtime")
	runtime := database.Runtimes["candles_runtime"]
	dsn := runtime.DSN
	redisURL := internaltest.NewRedisURL(t)
	chain := internaltest.NewMarketChain(t)
	credentials := productionChartCredentials(t)
	nc, bootstrap := internaltest.NewBroker(t, "CHART_BOOTSTRAP", []string{"chart.bootstrap"})
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err = js.DeleteStream(ctx, bootstrap.CachedInfo().Config.Name); err != nil {
		t.Fatal(err)
	}
	examples := filepath.Join(root, "deploy/launchpad/documents/values")
	var broker brokerprovision.Document
	chartReadJSON(t, filepath.Join(examples, "broker.example.json"), &broker)
	provision := brokerprovision.Document{Durables: broker.Durables[:0:0]}
	for _, stream := range broker.Streams {
		if stream.Name == "CHART_OUTBOX" || stream.Name == "KINDLE_INDEXER_V1" {
			provision.Streams = append(provision.Streams, stream)
		}
	}
	for _, durable := range broker.Durables {
		if durable.Durable == "charts_market" {
			provision.Durables = append(provision.Durables, durable)
		}
	}
	if len(provision.Streams) != 2 || len(provision.Durables) != 1 {
		t.Fatal("chart stream inventory required")
	}
	directory := t.TempDir()
	brokerPath, addressPath := filepath.Join(directory, "broker.json"), filepath.Join(directory, "broker-url.txt")
	chartWriteJSON(t, brokerPath, provision)
	if err = os.WriteFile(addressPath, []byte(nc.ConnectedUrl()), 0600); err != nil {
		t.Fatal(err)
	}
	provisionBroker := func(action string) {
		t.Helper()
		var output bytes.Buffer
		if status := brokerprovision.Run(ctx, []string{action, "--document", brokerPath, "--broker-file", addressPath, "--credentials-file", credentials}, &output, &output); status != 0 {
			t.Fatalf("broker %s: %d %s", action, status, output.String())
		}
	}
	provisionBroker("provision")
	provisionBroker("check")
	values := filepath.Join(directory, "values")
	if err = os.MkdirAll(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(values, "expected-schema/charts.json"), runtime.ExpectedSchema, 0600); err != nil {
		t.Fatal(err)
	}
	var budget production.ConnectionBudget
	chartReadJSON(t, filepath.Join(examples, "common.example.json"), &budget)
	upstream, err := url.Parse(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	upstreamPort, err := strconv.ParseUint(upstream.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	budget.Targets[0].Host, budget.Targets[0].Port = upstream.Hostname(), uint16(upstreamPort)
	admin, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	if err = admin.QueryRow(ctx, "SELECT setting::bigint FROM pg_settings WHERE name='max_connections'").Scan(&budget.Targets[0].MaxConnections); err != nil {
		t.Fatal(err)
	}
	if err = admin.Close(ctx); err != nil {
		t.Fatal(err)
	}
	for i := range budget.Clients {
		if budget.Clients[i].Role == "chartsd" {
			budget.Clients[i].Transport = runtimedb.TransportDirect
			budget.Clients[i].Pooler = ""
			budget.Clients[i].Alias = strings.TrimPrefix(upstream.Path, "/")
			budget.Clients[i].User = runtime.Role
		}
	}
	chartWriteJSON(t, filepath.Join(values, "common.json"), budget)
	var chartValues struct {
		Features config.ChartRuntimeFeatures `json:"features"`
		Delivery chartRuntimeDocument        `json:"delivery"`
	}
	chartReadJSON(t, filepath.Join(examples, "charts.example.json"), &chartValues)
	chartValues.Features.AnchorHeight, chartValues.Features.AnchorHash = 1, chain.Headers[1].Hash
	chartValues.Delivery.Epoch, chartValues.Delivery.Region = coordinatortest.Epoch, coordinatortest.Region
	chartWriteJSON(t, filepath.Join(values, "charts.json"), chartValues)
	compose := exec.CommandContext(ctx, "python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", filepath.Join(directory, "composed"), "--service", "charts")
	if output, err := compose.CombinedOutput(); err != nil {
		t.Fatalf("compose charts: %v %s", err, output)
	}
	environment := map[string]string{}
	chartReadJSON(t, filepath.Join(directory, "composed/charts.json"), &environment)
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	chartReadJSON(t, filepath.Join(root, "deploy/launchpad/railway/variables.json"), &contract)
	for key, value := range contract["charts"].Fixed {
		environment[key] = value
	}
	for key, value := range map[string]string{"DATABASE_URL": dsn, "REDIS_URL": redisURL, "REDIS_BULL_URL": redisURL, "RPC_URL": chain.RPCURL, "NATS_URL": nc.ConnectedUrl(), "NATS_CREDENTIALS": credentials, "WEBHOOK_HMAC_SECRET": strings.Repeat("s", 32), "DB_POOL_ID": "chartsd_pool", "INSTANCE_ID": "charts-container", "ZONE": "zone-a"} {
		environment[key] = value
	}
	for _, key := range contract["charts"].Required {
		if environment[key] == "" {
			t.Fatalf("required chart variable absent: %s", key)
		}
	}
	cfg := config.Config{RuntimeProfile: environment["RUNTIME_PROFILE"], BindAddress: environment["BIND_ADDRESS"], RuntimeBudgetDocument: environment["DB_BUDGET_DOCUMENT"], RuntimeBudgetSHA256: environment["DB_BUDGET_SHA256"], RuntimePoolID: environment["DB_POOL_ID"], RuntimeInstanceID: environment["INSTANCE_ID"], ExpectedSchemaDocument: environment["EXPECTED_SCHEMA_DOCUMENT"], ExpectedSchemaSHA256: environment["EXPECTED_SCHEMA_SHA256"]}
	cfg.NodeEnv, cfg.Port, cfg.DatabaseURL = "production", 8080, dsn
	pool, err := cfg.OpenRuntimePool(ctx, "chartsd", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if pool.Config().MaxConns != 1 || pool.Config().ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeCacheStatement || pool.Config().ConnConfig.StatementCacheCapacity != 512 {
		t.Fatal("direct PostgreSQL client bounds differ")
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	fill := chartReceiptFill(t, chain)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	height, _ := strconv.ParseUint(fill.Position.Height, 10, 64)
	timestamp, _ := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err = engine.ApplySeriesTx(ctx, tx, exact, fill, 1, engine.SeriesCheckpoint{Height: height, Timestamp: timestamp}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "chartsd")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/chartsd")
	build.Dir = filepath.Join(root, "services/launchpad/trading-charts")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("charts container entrypoint: %v %s", err, output)
	}
	childEnv := make([]string, 0, len(environment))
	for key, value := range environment {
		childEnv = append(childEnv, key+"="+value)
	}
	sort.Strings(childEnv)
	start := func() func() {
		t.Helper()
		output, err := os.CreateTemp(directory, "charts-output-*.txt")
		if err != nil {
			t.Fatal(err)
		}
		process := exec.Command(binary)
		process.Env, process.Stdout, process.Stderr = childEnv, output, output
		if err = process.Start(); err != nil {
			_ = output.Close()
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- process.Wait() }()
		stopped := false
		stop := func() {
			t.Helper()
			if stopped {
				return
			}
			stopped = true
			_ = process.Process.Signal(syscall.SIGTERM)
			select {
			case err := <-done:
				if err != nil {
					raw, _ := os.ReadFile(output.Name())
					t.Errorf("charts shutdown: %v %s", err, raw)
				}
			case <-time.After(15 * time.Second):
				_ = process.Process.Kill()
				<-done
				t.Error("charts shutdown timed out")
			}
			_ = output.Close()
		}
		t.Cleanup(stop)
		client := http.Client{Timeout: time.Second}
		ready := false
		var last []byte
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
			response, err := client.Get("http://127.0.0.1:8080/health/ready")
			if err == nil {
				last, _ = io.ReadAll(response.Body)
				_ = response.Body.Close()
				if response.StatusCode == http.StatusOK {
					ready = true
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !ready {
			raw, _ := os.ReadFile(output.Name())
			t.Fatalf("charts readiness: %s %s", last, raw)
		}
		return stop
	}
	from, err := engine.BucketStart(timestamp, "1m")
	if err != nil {
		t.Fatal(err)
	}
	read := func() []byte {
		t.Helper()
		client := http.Client{Timeout: 3 * time.Second}
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:8080/v2/series/%s/candles?interval=1m&from=%d&to=%d", fill.PoolAddress, from, from+60))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			SchemaVersion int                   `json:"schemaVersion"`
			Algorithm     string                `json:"algorithmVersion"`
			Items         []engine.SeriesCandle `json:"items"`
			NoData        bool                  `json:"noData"`
		}
		if err = json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 || body.SchemaVersion != 2 || body.Algorithm != engine.SeriesAlgorithm || body.NoData || len(body.Items) != 1 || body.Items[0].Trades != 1 || body.Items[0].CloseSource.SourceFillID != fill.SourceFillID || body.Items[0].VolumeUsdlRaw != fill.USDLAmountRaw {
			t.Fatalf("committed chart series: %d %s", response.StatusCode, raw)
		}
		return raw
	}
	stop := start()
	first := read()
	stream, err := js.Stream(ctx, "CHART_OUTBOX")
	if err != nil {
		t.Fatal(err)
	}
	published := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		info, err := stream.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if info.State.Msgs == uint64(len(engine.ExactIntervals)) {
			published = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !published {
		t.Fatal("committed chart publications not delivered")
	}
	stop()
	stop = start()
	if second := read(); !bytes.Equal(first, second) {
		t.Fatalf("chart series changed across restart: %s %s", first, second)
	}
	provisionBroker("check")
	info, err := stream.Info(ctx)
	if err != nil || info.State.Msgs != uint64(len(engine.ExactIntervals)) {
		t.Fatalf("restarted chart outbox: %+v %v", info, err)
	}
	stop()
}
