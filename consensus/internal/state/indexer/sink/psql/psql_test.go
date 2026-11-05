package psql

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"testing"
	"time"

	"github.com/adlio/schema"
	"github.com/gogo/protobuf/proto"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"

	// Register the Postgres database driver.
	_ "github.com/lib/pq"
)

// Verify that the type satisfies the EventSink interface.
var _ indexer.EventSink = (*EventSink)(nil)

var (
	doPauseAtExit = flag.Bool("pause-at-exit", false,
		"If true, pause the test until interrupted at shutdown, to allow debugging")

	// A hook that test cases can call to obtain the shared database instance
	// used for testing the sink. This is initialized in TestMain (see below).
	testDB   func() *sql.DB
	testConn string
)

const (
	user     = "postgres"
	password = "secret"
	port     = "5432"
	dsn      = "postgres://%s:%s@localhost:%s/%s?sslmode=disable"
	dbName   = "postgres"
	chainID  = "test-chainID"

	viewTxEvents = "tx_events"
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Service-free repair tests must reach m.Run even without Docker. Initialize
	// the existing PostgreSQL suite only when a selected test requests its DB.
	var once sync.Once
	var db *sql.DB
	var cleanup func()
	testDB = func() *sql.DB {
		once.Do(func() { db, cleanup = startTestPostgres() })
		return db
	}
	code := m.Run()
	if cleanup != nil {
		cleanup()
	}
	os.Exit(code)
}

func startTestPostgres() (*sql.DB, func()) {
	// Set up docker.
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	if err != nil {
		log.Fatalf("Creating docker pool: %v", err)
	}

	// Selected PostgreSQL integration tests require an available Docker engine.
	if _, err := pool.Client.Info(); err != nil {
		log.Fatalf("Docker is not available: %v", err)
	}

	// Start a container running PostgreSQL.
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "13",
		Env: []string{
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + dbName,
			"listen_addresses = '*'",
		},
		ExposedPorts: []string{fmt.Sprintf("%s/tcp", port)},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("Starting docker pool: %v", err)
	}

	if *doPauseAtExit {
		log.Print("Pause at exit is enabled, containers will not expire")
	} else {
		const expireSeconds = 60
		_ = resource.Expire(expireSeconds)
		log.Printf("Container expiration set to %d seconds", expireSeconds)
	}

	// Connect to the database, clear any leftover data, and install the
	// indexing schema.
	conn := fmt.Sprintf(dsn, user, password, resource.GetPort(port+"/tcp"), dbName)
	testConn = conn
	var db *sql.DB

	if err := pool.Retry(func() error {
		sink, err := NewEventSink(conn, chainID)
		if err != nil {
			return err
		}
		db = sink.DB() // set global for test use
		return db.Ping()
	}); err != nil {
		log.Fatalf("Connecting to database: %v", err)
	}

	if err := resetDatabase(db); err != nil {
		log.Fatalf("Flushing database: %v", err)
	}

	sm, err := readSchema()
	if err != nil {
		log.Fatalf("Reading schema: %v", err)
	}
	migrator := schema.NewMigrator()
	if err := migrator.Apply(db, sm); err != nil {
		log.Fatalf("Applying schema: %v", err)
	}

	return db, func() {
		if *doPauseAtExit {
			log.Print("Testing complete, pausing for inspection. Send SIGINT to resume teardown")
			waitForInterrupt()
			log.Print("(resuming)")
		}
		log.Print("Shutting down database")
		if err := pool.Purge(resource); err != nil {
			log.Printf("WARNING: Purging pool failed: %v", err)
		}
		if err := db.Close(); err != nil {
			log.Printf("WARNING: Closing database failed: %v", err)
		}
	}
}

func TestType(t *testing.T) {
	psqlSink := &EventSink{store: testDB(), chainID: chainID}
	assert.Equal(t, indexer.PSQL, psqlSink.Type())
}

func TestIndexing(t *testing.T) {
	ctx := t.Context()

	t.Run("IndexBlockEvents", func(t *testing.T) {
		indexer := &EventSink{store: testDB(), chainID: chainID}
		for _, height := range []int64{1, 2} {
			block := newTestBlockHeader()
			block.Header.Height = height
			started := time.Now().UTC()
			require.NoError(t, indexer.IndexBlockEvents(block))
			finished := time.Now().UTC()
			require.NoError(t, verifyBlock(indexer.DB(), block.Header.Height, chainID))
			require.NoError(t, verifyTimeStamp(indexer.DB(), tableBlocks, block.Header.Height, chainID, started, finished))

			// Attempting to reindex the same events should gracefully succeed.
			require.NoError(t, indexer.IndexBlockEvents(block))
		}

		verifyNotImplemented(t, "hasBlock", func() (bool, error) { return indexer.HasBlock(1) })
		verifyNotImplemented(t, "hasBlock", func() (bool, error) { return indexer.HasBlock(2) })

		verifyNotImplemented(t, "block search", func() (bool, error) {
			v, err := indexer.SearchBlockEvents(ctx, nil)
			return v != nil, err
		})
	})

	t.Run("IndexTxEvents", func(t *testing.T) {
		indexer := &EventSink{store: testDB(), chainID: chainID}

		txResult := txResultWithEvents([]abci.Event{
			makeIndexedEvent("account.number", "1"),
			makeIndexedEvent("account.owner", "Ivan"),
			makeIndexedEvent("account.owner", "Yulieta"),

			{Type: "", Attributes: []abci.EventAttribute{{Key: []byte("not_allowed"), Value: []byte("Vlad"), Index: true}}},
		})
		started := time.Now().UTC()
		require.NoError(t, indexer.IndexTxEvents([]*abci.TxResultV2{txResult}))
		finished := time.Now().UTC()

		txr, err := loadTxResult(types.Tx(txResult.Tx).Hash().Bytes())
		require.NoError(t, err)
		assert.Equal(t, txResult, txr)

		require.NoError(t, verifyTimeStamp(indexer.DB(), tableTxResults, txResult.Height, chainID, started, finished))
		require.NoError(t, verifyTimeStamp(indexer.DB(), viewTxEvents, txResult.Height, chainID, started, finished))

		verifyNotImplemented(t, "getTxByHash", func() (bool, error) {
			txr, err := indexer.GetTxByHash(types.Tx(txResult.Tx).Hash().Bytes())
			return txr != nil, err
		})
		verifyNotImplemented(t, "tx search", func() (bool, error) {
			txr, err := indexer.SearchTxEvents(ctx, nil)
			return txr != nil, err
		})

		// try to insert the duplicate tx events.
		err = indexer.IndexTxEvents([]*abci.TxResultV2{txResult})
		require.NoError(t, err)
	})

	// The same repair assertions run against PostgreSQL in the integration
	// suite; the focused repair gate uses the embedded SQL engine instead.
	t.Run("Repair82Postgres", func(t *testing.T) {
		db := testDB()
		// Recreate the old attribute identity and upgrade existing rows with
		// the operator migration before exercising the new writer.
		var before, after int64
		require.NoError(t, db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM attributes").Scan(&before))
		_, err := db.ExecContext(t.Context(), `
ALTER TABLE attributes DROP CONSTRAINT attributes_pkey;
ALTER TABLE attributes DROP COLUMN ordinal;
ALTER TABLE attributes ADD UNIQUE (event_id, key);`)
		require.NoError(t, err)
		migration, err := os.ReadFile("migrate_attributes_ordinal.sql")
		require.NoError(t, err)
		_, err = db.ExecContext(t.Context(), string(migration))
		require.NoError(t, err)
		require.NoError(t, db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM attributes WHERE ordinal >= 0").Scan(&after))
		require.Equal(t, before, after)
		for _, table := range []string{tableBlocks, tableTxResults, tableEvents} {
			_, err := db.ExecContext(t.Context(), "SELECT setval(pg_get_serial_sequence($1, 'rowid'), $2)", table, int64(1<<32-1))
			require.NoError(t, err)
		}
		repair82RoundTrip(t, func() *EventSink {
			sink, err := NewEventSink(testConn, t.Name())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sink.Stop()) })
			return sink
		}, 1<<32)
	})
}

func TestStop(t *testing.T) {
	indexer := &EventSink{store: testDB()}
	require.NoError(t, indexer.Stop())
}

// newTestBlockHeader constructs a fresh copy of a block header containing
// known test values to exercise the indexer.
func newTestBlockHeader() types.EventDataNewBlockHeader {
	return types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		ResultFinalizeBlock: abci.ResponseFinalizeBlock{
			Events: []abci.Event{
				makeIndexedEvent("finalize_event.proposer", "FCAA001"),
				makeIndexedEvent("thingy.whatzit", "O.O"),
				makeIndexedEvent("my_event.foo", "100"),
				makeIndexedEvent("thingy.whatzit", "-.O"),
			},
		},
	}
}

// readSchema loads the indexing database schema file
func readSchema() ([]*schema.Migration, error) {
	const filename = "schema.sql"
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read sql file from '%s': %w", filename, err)
	}

	return []*schema.Migration{{
		ID:     time.Now().Local().String() + " db schema",
		Script: string(contents),
	}}, nil
}

// resetDB drops all the data from the test database.
func resetDatabase(db *sql.DB) error {
	_, err := db.Exec(`DROP TABLE IF EXISTS blocks,tx_results,events,attributes CASCADE;`)
	if err != nil {
		return fmt.Errorf("dropping tables: %w", err)
	}
	_, err = db.Exec(`DROP VIEW IF EXISTS event_attributes,block_events,tx_events CASCADE;`)
	if err != nil {
		return fmt.Errorf("dropping views: %w", err)
	}
	return nil
}

// txResultWithEvents constructs a fresh transaction result with fixed values
// for testing, that includes the specified events.
func txResultWithEvents(events []abci.Event) *abci.TxResultV2 {
	return &abci.TxResultV2{
		Height: 1,
		Index:  0,
		Tx:     types.Tx("HELLO WORLD"),
		Result: abci.ExecTxResult{
			Data:   []byte{0},
			Code:   abci.CodeTypeOK,
			Log:    "",
			Events: events,
		},
	}
}

func loadTxResult(hash []byte) (*abci.TxResultV2, error) {
	hashString := fmt.Sprintf("%X", hash)
	var resultData []byte
	if err := testDB().QueryRow(`
SELECT tx_result FROM `+tableTxResults+` WHERE tx_hash = $1;
`, hashString).Scan(&resultData); err != nil {
		return nil, fmt.Errorf("lookup transaction for hash %q failed: %v", hashString, err)
	}

	txr := new(abci.TxResult)
	if err := proto.Unmarshal(resultData, txr); err != nil {
		return nil, fmt.Errorf("unmarshaling txr: %w", err)
	}

	return &abci.TxResultV2{Height: txr.Height, Index: txr.Index, Tx: txr.Tx, Result: txr.Result}, nil
}

func verifyTimeStamp(db *sql.DB, tableName string, height int64, chain string, started, finished time.Time) error {
	var query string
	switch tableName {
	case tableBlocks, viewTxEvents:
		query = "SELECT DISTINCT created_at FROM " + tableName + " WHERE height = $1 AND chain_id = $2"
	case tableTxResults:
		query = `SELECT DISTINCT tx_results.created_at FROM tx_results
JOIN blocks ON blocks.rowid = tx_results.block_id WHERE blocks.height = $1 AND blocks.chain_id = $2`
	default:
		return fmt.Errorf("unsupported timestamp table %q", tableName)
	}
	// database/sql converts both PostgreSQL time.Time and the embedded fixture's
	// RFC3339 text to a string, so both suites scan and check the same value.
	var encoded string
	if err := db.QueryRow(query, height, chain).Scan(&encoded); err != nil {
		return fmt.Errorf("reading %s timestamp at height %d: %w", tableName, height, err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, encoded)
	if err != nil {
		return fmt.Errorf("decoding %s timestamp: %w", tableName, err)
	}
	if createdAt.Before(started) || createdAt.After(finished) {
		return fmt.Errorf("%s timestamp %s outside indexing interval [%s, %s]", tableName, createdAt, started, finished)
	}
	return nil
}

func verifyBlock(db *sql.DB, height int64, chain string) error {
	// Check that the blocks table contains an entry for this height and chain.
	var got int64
	if err := db.QueryRow(`
SELECT height FROM `+tableBlocks+` WHERE height = $1 AND chain_id = $2;
`, height, chain).Scan(&got); err != nil {
		return fmt.Errorf("reading block at height %d: %w", height, err)
	}
	if got != height {
		return fmt.Errorf("block height: got %d, want %d", got, height)
	}
	return nil
}

// verifyNotImplemented calls f and verifies that it returns both a
// false-valued flag and a non-nil error whose string matching the expected
// "not supported" message with label prefixed.
func verifyNotImplemented(t *testing.T, label string, f func() (bool, error)) {
	t.Helper()
	t.Logf("Verifying that %q reports it is not implemented", label)

	want := label + " is not supported via the postgres event sink"
	ok, err := f()
	assert.False(t, ok)
	require.Error(t, err)
	assert.Equal(t, want, err.Error())
}

// waitForInterrupt blocks until a SIGINT is received by the process.
func waitForInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
}
