package littbuilder

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
)

func repair1167Config(t *testing.T, enabled bool) *litt.Config {
	t.Helper()
	config, err := litt.DefaultConfig(t.TempDir())
	require.NoError(t, err)
	config.MetricsEnabled = enabled
	config.MetricsPort = 0
	config.MetricsUpdateInterval = time.Hour
	return config
}

func repair1167Open(t *testing.T, config *litt.Config, rc ...*litt.RuntimeConfig) *db {
	t.Helper()
	database, err := NewDB(config, rc...)
	require.NoError(t, err)
	result := database.(*db)
	t.Cleanup(func() { require.NoError(t, result.Close()) })
	return result
}

func repair1167Table(t *testing.T, database *db, name string) litt.Table {
	t.Helper()
	config := litt.DefaultTableConfig(name)
	config.ShardingFactor = 1
	table, err := database.BuildTable(config)
	require.NoError(t, err)
	return table
}

func repair1167Write(t *testing.T, table litt.Table, key, value string) {
	t.Helper()
	require.NoError(t, table.Put([]byte(key), []byte(value)))
	require.NoError(t, table.Flush())
}

func repair1167Read(t *testing.T, table litt.Table, key, expected string) {
	t.Helper()
	value, found, err := table.Get([]byte(key))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, expected, string(value))
}

func repair1167Coordinators() int {
	stack := make([]byte, 4<<20)
	n := runtime.Stack(stack, true)
	return strings.Count(string(stack[:n]), "disktable.(*flushCoordinator).controlLoop(")
}

func TestRepairM_1167_TableConstructorUnwind(t *testing.T) {
	for _, stage := range []string{"segment_directories", "segment_gather"} {
		t.Run(stage, func(t *testing.T) {
			config := repair1167Config(t, false)
			original := repair1167Open(t, config)
			table := repair1167Table(t, original, "recovered")
			repair1167Write(t, table, "before", "durable before failure")
			require.NoError(t, original.Close())

			// Keep the initialized Pebble keymap intact so a retry cannot hide
			// a leaked lock by removing and recreating its directory.
			initialized := filepath.Join(config.Paths[0], "recovered", keymap.KeymapDirectoryName, keymap.KeymapInitializedFileName)
			_, err := os.Stat(initialized)
			require.NoError(t, err)
			config.MinimumFlushInterval = time.Millisecond
			database := repair1167Open(t, config)
			baseline := repair1167Coordinators()
			segments := filepath.Join(config.Paths[0], "recovered", segment.SegmentDirectory)
			var restore func()
			if stage == "segment_directories" {
				require.NoError(t, os.Rename(segments, segments+"-saved"))
				require.NoError(t, os.WriteFile(segments, []byte("blocks mkdir"), 0600))
				restore = func() {
					require.NoError(t, os.Remove(segments))
					require.NoError(t, os.Rename(segments+"-saved", segments))
				}
			} else {
				badFile := filepath.Join(segments, "invalid"+segment.KeyFileExtension)
				require.NoError(t, os.WriteFile(badFile, []byte("invalid segment name"), 0600))
				restore = func() { require.NoError(t, os.Remove(badFile)) }
			}
			for attempt := 0; attempt < 3; attempt++ {
				failed, err := database.BuildTable(litt.DefaultTableConfig("recovered"))
				require.Error(t, err)
				require.Nil(t, failed)
				if stage == "segment_gather" {
					require.ErrorContains(t, err, "failed to gather segment files")
				} else {
					require.ErrorContains(t, err, "failed to create segment directories")
				}
				keys, reload, err := keymap.NewPebbleDBKeymap(database.runtimeConfig.Logger,
					filepath.Join(config.Paths[0], "recovered", keymap.KeymapDirectoryName, keymap.KeymapDataDirectoryName), false)
				require.NoError(t, err, "failed construction must release the real Pebble lock")
				require.False(t, reload)
				_, found, err := keys.Get([]byte("before"))
				require.NoError(t, err)
				require.True(t, found)
				require.NoError(t, keys.Stop())
				require.Eventually(t, func() bool { return repair1167Coordinators() == baseline }, 3*time.Second, time.Millisecond,
					"failed construction must cancel its coordinator")
			}
			restore()
			retried := repair1167Table(t, database, "recovered")
			repair1167Read(t, retried, "before", "durable before failure")
			repair1167Write(t, retried, "after", "durable after retry")
			require.Eventually(t, func() bool { return repair1167Coordinators() == baseline+1 }, 3*time.Second, time.Millisecond,
				"the real successful table must start a coordinator")
			require.NoError(t, database.Close())
			reopened := repair1167Open(t, config)
			recovered := repair1167Table(t, reopened, "recovered")
			repair1167Read(t, recovered, "before", "durable before failure")
			repair1167Read(t, recovered, "after", "durable after retry")
			// Drop also releases the construction context after segment deletion.
			require.NoError(t, recovered.Drop())
		})
	}
}

func TestRepairM_1167_KeymapInitializedFailure(t *testing.T) {
	config := repair1167Config(t, false)
	original := repair1167Open(t, config)
	table := repair1167Table(t, original, "initialized")
	repair1167Write(t, table, "before", "preserved")
	require.NoError(t, original.Close())
	keymapDirectory := filepath.Join(config.Paths[0], "initialized", keymap.KeymapDirectoryName)
	marker := filepath.Join(keymapDirectory, keymap.KeymapInitializedFileName)
	require.NoError(t, os.Remove(marker))
	require.NoError(t, os.Mkdir(marker, 0750))
	database := repair1167Open(t, config)
	failed, err := database.BuildTable(litt.DefaultTableConfig("initialized"))
	require.ErrorContains(t, err, "failed to create keymap initialized file")
	require.Nil(t, failed)
	keys, reload, err := keymap.NewPebbleDBKeymap(database.runtimeConfig.Logger,
		filepath.Join(keymapDirectory, keymap.KeymapDataDirectoryName), false)
	require.NoError(t, err, "the keymap builder must release its acquired Pebble handle on marker failure")
	require.False(t, reload)
	_, found, err := keys.Get([]byte("before"))
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, keys.Stop())
	require.NoError(t, os.Remove(marker))
	require.NoError(t, os.WriteFile(marker, nil, 0600))
	repair1167Read(t, repair1167Table(t, database, "initialized"), "before", "preserved")
}

func repair1167Address(database *db) string {
	port := database.metricsServer.address.(*net.TCPAddr).Port
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func repair1167Scrape(t *testing.T, database *db) map[string]*dto.MetricFamily {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + repair1167Address(database) + "/metrics")
	require.NoError(t, err)
	defer response.Body.Close()
	defer client.CloseIdleConnections()
	require.Equal(t, http.StatusOK, response.StatusCode)
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(response.Body)
	require.NoError(t, err)
	return families
}

func repair1167Counter(t *testing.T, families map[string]*dto.MetricFamily, name, table string, expected float64) {
	t.Helper()
	family := families[name]
	require.NotNil(t, family, "missing metric %s", name)
	for _, sample := range family.Metric {
		matches := table == ""
		for _, label := range sample.Label {
			if label.GetName() == "table" && label.GetValue() == table {
				matches = true
			}
		}
		if matches {
			require.NotNil(t, sample.Counter)
			require.Equal(t, expected, sample.Counter.GetValue(), "%s table=%s", name, table)
			return
		}
	}
	t.Fatalf("missing sample %s table=%s", name, table)
}

func TestRepairM_1167_GlobalMetricsProvider(t *testing.T) {
	for _, order := range []string{"first", "second"} {
		t.Run("close_"+order, func(t *testing.T) {
			first := repair1167Open(t, repair1167Config(t, true))
			provider := otel.GetMeterProvider()
			nodeName := "repair_m1167_node_" + order
			nodeWrites, err := otel.Meter("repair-m1167-node").Int64Counter(nodeName)
			require.NoError(t, err)
			nodeWrites.Add(context.Background(), 1)
			firstName, secondName := "m1167_first_"+order, "m1167_second_"+order
			firstTable := repair1167Table(t, first, firstName)
			repair1167Write(t, firstTable, "before", "first")
			repair1167Counter(t, repair1167Scrape(t, first), "litt_keys_written_total", firstName, 1)

			second := repair1167Open(t, repair1167Config(t, true))
			require.Same(t, provider, otel.GetMeterProvider(), "opening a DB must preserve existing instruments")
			secondTable := repair1167Table(t, second, secondName)
			repair1167Write(t, secondTable, "before", "second")
			repair1167Write(t, firstTable, "during", "first after second opened")
			nodeWrites.Add(context.Background(), 1)
			for _, database := range []*db{first, second} {
				families := repair1167Scrape(t, database)
				repair1167Counter(t, families, "litt_keys_written_total", firstName, 2)
				repair1167Counter(t, families, "litt_keys_written_total", secondName, 1)
				repair1167Counter(t, families, nodeName+"_total", "", 2)
			}
			closed, survivor, survivorTable, survivorName, expected := first, second, secondTable, secondName, float64(2)
			if order == "second" {
				closed, survivor, survivorTable, survivorName, expected = second, first, firstTable, firstName, 3
			}
			require.NoError(t, closed.Close())
			require.Same(t, provider, otel.GetMeterProvider())
			repair1167Write(t, survivorTable, "after", "survives peer close")
			nodeWrites.Add(context.Background(), 1)
			families := repair1167Scrape(t, survivor)
			repair1167Counter(t, families, "litt_keys_written_total", survivorName, expected)
			repair1167Counter(t, families, nodeName+"_total", "", 3)
			require.NoError(t, survivor.Close())

			// The same process provider remains usable across a period with no open DBs.
			reopened := repair1167Open(t, repair1167Config(t, true))
			require.Same(t, provider, otel.GetMeterProvider())
			nodeWrites.Add(context.Background(), 1)
			repair1167Counter(t, repair1167Scrape(t, reopened), nodeName+"_total", "", 4)
		})
	}
}

func repair1167AssertClosed(t *testing.T, database *db) {
	t.Helper()
	select {
	case <-database.metricsServer.done:
	default:
		t.Fatal("Close must join metrics serving and listener shutdown")
	}
	select {
	case <-database.metricsDone:
	default:
		t.Fatal("Close must join periodic collection without waiting for its ticker")
	}
	connection, err := net.DialTimeout("tcp", repair1167Address(database), time.Second)
	if connection != nil {
		connection.Close()
	}
	require.Error(t, err, "closed DB must release its endpoint")
}

func TestRepairM_1167_MetricsServerClose(t *testing.T) {
	config := repair1167Config(t, true)
	rc := litt.DefaultRuntimeConfig()
	database := repair1167Open(t, config, rc)
	table := repair1167Table(t, database, "m1167_endpoint")
	repair1167Write(t, table, "before", "persist through endpoint restart")
	repair1167Scrape(t, database)
	closeDone := make(chan error, 1)
	go func() { closeDone <- database.Close() }()
	select {
	case err := <-closeDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close must not wait for the one-hour collection ticker")
	}
	repair1167AssertClosed(t, database)
	require.NoError(t, rc.CTX.Err(), "Close must not cancel the caller's context")
	config.MetricsPort = database.metricsServer.address.(*net.TCPAddr).Port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.MetricsPort))
	require.NoError(t, err, "Close must free the port before returning")
	require.NoError(t, listener.Close())
	require.NoError(t, database.Close())
	reopened := repair1167Open(t, config, rc)
	recovered := repair1167Table(t, reopened, "m1167_endpoint")
	repair1167Read(t, recovered, "before", "persist through endpoint restart")
	repair1167Write(t, recovered, "after", "new instance")
	repair1167Counter(t, repair1167Scrape(t, reopened), "litt_keys_written_total", "m1167_endpoint", 2)
	require.NoError(t, reopened.Destroy())
	repair1167AssertClosed(t, reopened)
	require.NoError(t, rc.CTX.Err())
}

func TestRepairM_1167_MetricsBindingFailure(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener.Close()
	config := repair1167Config(t, true)
	config.MetricsPort = listener.Addr().(*net.TCPAddr).Port
	database, err := NewDB(config)
	if database != nil {
		t.Cleanup(func() { database.Close() })
	}
	require.ErrorContains(t, err, "bind metrics listener")
	require.ErrorIs(t, err, syscall.EADDRINUSE)
	require.Nil(t, database, "NewDB cannot report success when its endpoint is unavailable")
	require.NoError(t, listener.Close())
	retried := repair1167Open(t, config)
	repair1167Write(t, repair1167Table(t, retried, "m1167_bind_retry"), "key", "retry succeeded")
	repair1167Counter(t, repair1167Scrape(t, retried), "litt_keys_written_total", "m1167_bind_retry", 1)
}

func TestRepairM_1167_MetricsParentCancellation(t *testing.T) {
	config := repair1167Config(t, true)
	rc := litt.DefaultRuntimeConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rc.CTX = ctx
	database := repair1167Open(t, config, rc)
	repair1167Scrape(t, database)
	cancel()
	select {
	case <-database.metricsServer.done:
	case <-time.After(5 * time.Second):
		t.Fatal("parent cancellation must release the listener")
	}
	require.NoError(t, database.Close())
	repair1167AssertClosed(t, database)
	failed, err := NewDB(config, rc)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, failed)
	// A failed startup also releases the storage locks on this path.
	retried := repair1167Open(t, config)
	repair1167Scrape(t, retried)
}

func TestRepairM_1167_MetricsInvalidInterval(t *testing.T) {
	config := repair1167Config(t, true)
	config.MetricsUpdateInterval = -time.Second
	database, err := NewDB(config)
	require.ErrorContains(t, err, "metrics update interval must be positive")
	require.Nil(t, database)
	config.MetricsUpdateInterval = time.Hour
	repair1167Scrape(t, repair1167Open(t, config))
}
