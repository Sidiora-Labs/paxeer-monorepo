package main

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func measurementBroker(t *testing.T, ctx context.Context) jetstream.Consumer {
	consumer, _ := measurementBrokerWithURL(t, ctx)
	return consumer
}

func measurementBrokerWithURL(t *testing.T, ctx context.Context) (jetstream.Consumer, string) {
	t.Helper()
	binary, err := exec.LookPath("nats-server")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	log, err := os.OpenFile(filepath.Join(directory, "broker.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-js", "-sd", filepath.Join(directory, "data"), "-a", "127.0.0.1", "-p", strconv.Itoa(port))
	command.Stdout, command.Stderr = log, log
	if err = command.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	_ = log.Close()
	t.Cleanup(func() {
		if err := command.Process.Signal(syscall.SIGINT); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error(err)
		}
		if err := command.Wait(); err != nil {
			t.Error(err)
		}
	})
	url := "nats://127.0.0.1:" + strconv.Itoa(port)
	var connection *nats.Conn
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		connection, err = nats.Connect(url, nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Close)
	js, err := jetstream.New(connection)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "CHART_MEASURE", Subjects: []string{"chart.measure"}, Storage: jetstream.FileStorage})
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{Durable: "observer", AckPolicy: jetstream.AckExplicitPolicy})
	if err != nil {
		t.Fatal(err)
	}
	return consumer, url
}

func TestCaptureReadsCanonicalSourceAndSeriesState(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA indexer;
CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id bigint,stream_id text,raw_complete_height bigint,typed_complete_height bigint);
CREATE TABLE indexer.indexer_ingest_leases_v1(chain_id bigint,stream_id text,expires_at timestamptz);
INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,'canonical',100,100);
INSERT INTO indexer.indexer_ingest_leases_v1 VALUES(125,'canonical',clock_timestamp()+interval '1 hour')`); err != nil {
		t.Fatal(err)
	}
	poolA, poolB := "0x"+strings.Repeat("a", 40), "0x"+strings.Repeat("b", 40)
	for _, address := range []string{poolA, poolB} {
		if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,100,1704067200,false)`, address); err != nil {
			t.Fatal(err)
		}
	}
	broker, brokerURL := measurementBrokerWithURL(t, ctx)
	directory := t.TempDir()
	open := func(name string) boundedArtifact {
		file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		return boundedArtifact{file: file, budget: &rawBudget{maximum: 1 << 20}}
	}
	output := measurements{Samples: open("samples.ndjson"), Latencies: open("latencies.ndjson"), Locks: open("locks.ndjson")}
	cfg := config{Schema: "candles", SourceStream: "canonical", Applications: []string{"charts"}, Pools: []string{poolA, poolB}, HotPool: poolA, BurstOffsetSeconds: 60}
	observed, err := capture(ctx, pool, cfg, broker, 2*time.Second, output)
	if err != nil {
		t.Fatal(err)
	}
	if observed.SourceHeight != 100 || observed.ChartWatermark != 100 || observed.GapSeries != 0 || observed.Stage != "steady" || observed.BrokerPending != 0 || observed.QueryNanos <= 0 {
		t.Fatalf("source and series observation: %+v", observed)
	}
	raw, err := os.ReadFile(filepath.Join(directory, "samples.ndjson"))
	if err != nil || !strings.Contains(string(raw), `"sourceHeight":100`) {
		t.Fatalf("captured sample: %s, %v", raw, err)
	}
	for _, relation := range []string{
		"indexer.indexer_ingest_watermarks_v1", "candles.chart_series_clock", "candles.chart_inbox",
		"candles.chart_outbox", "candles.chart_rollup_pending", "candles.chart_pool_fences",
		"candles.chart_delivery_attempts",
	} {
		t.Run(relation, func(t *testing.T) {
			parts := strings.Split(relation, ".")
			missing := parts[1] + "_unavailable"
			if _, err := pool.Exec(ctx, `ALTER TABLE `+relation+` RENAME TO `+missing); err != nil {
				t.Fatal(err)
			}
			_, observedErr := capture(ctx, pool, cfg, broker, 2*time.Second, output)
			if _, err := pool.Exec(ctx, `ALTER TABLE `+parts[0]+`.`+missing+` RENAME TO `+parts[1]); err != nil {
				t.Fatal(err)
			}
			if observedErr == nil || !strings.Contains(observedErr.Error(), parts[1]) {
				t.Fatalf("missing observation source %s: %v", relation, observedErr)
			}
		})
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks_v1 SET raw_complete_height=99`); err != nil {
		t.Fatal(err)
	}
	if observed, err := capture(ctx, pool, cfg, broker, 2*time.Second, output); err == nil || observed.SourceHeight != 100 || !strings.Contains(err.Error(), "complete canonical source") {
		t.Fatalf("source completeness fence: %+v, %v", observed, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks_v1 SET raw_complete_height=100`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM candles.chart_series_clock WHERE pool=$1`, poolB); err != nil {
		t.Fatal(err)
	}
	if observed, err := capture(ctx, pool, cfg, broker, 2*time.Second, output); err == nil || !strings.Contains(err.Error(), "all observed series clocks") {
		t.Fatalf("missing series clock: %+v, %v", observed, err)
	}
	if _, err = pool.Exec(ctx, `CREATE TABLE indexer.indexer_accounting_fills(chain_id bigint,transaction_hash text,log_index bigint,pool_address text,payload jsonb,source_fact_digest text);
CREATE TABLE indexer.indexer_raw_logs_v1(chain_id bigint,transaction_hash text,log_index bigint,height bigint,block_hash text,transaction_index bigint);
CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint,stream_id text,height bigint,block_hash text,fetch_complete boolean,required_decode_complete boolean)`); err != nil {
		t.Fatal(err)
	}
	cfg.From, cfg.To, cfg.MaximumBars = 1704067200, 1704672000, 100
	if result, err := reconcile(ctx, pool, cfg, output, 100); err == nil || result != nil || !strings.Contains(err.Error(), "actual canonical fills required") {
		t.Fatalf("empty canonical fill range: %+v, %v", result, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,100,1704067200,false)`, poolB); err != nil {
		t.Fatal(err)
	}
	connection, err := nats.Connect(brokerURL, nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Close)
	js, err := jetstream.New(connection)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.Stream(ctx, "CHART_MEASURE")
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.DeleteConsumer(ctx, "observer"); err != nil {
		t.Fatal(err)
	}
	if sample, err := capture(ctx, pool, cfg, broker, 2*time.Second, output); err == nil || sample.SourceHeight != 100 {
		t.Fatalf("missing broker observation: %+v, %v", sample, err)
	}
	if err := output.Samples.file.Close(); err != nil {
		t.Fatal(err)
	}
	broker, _ = measurementBrokerWithURL(t, ctx)
	if sample, err := capture(ctx, pool, cfg, broker, 2*time.Second, output); err == nil || sample.SourceHeight != 100 || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("private sample write: %+v, %v", sample, err)
	}
	pool.Close()
	if sample, err := capture(ctx, pool, cfg, broker, 2*time.Second, output); err == nil || !strings.Contains(err.Error(), "closed pool") || sample.SourceHeight != 0 {
		t.Fatalf("closed observation database: %+v, %v", sample, err)
	}
	if result, err := reconcile(ctx, pool, cfg, output, 100); result != nil || err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("closed reconciliation database: %+v, %v", result, err)
	}
}
