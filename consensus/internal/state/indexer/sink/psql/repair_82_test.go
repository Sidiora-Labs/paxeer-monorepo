package psql

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/psql/internal/repairdb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repair82Embedded(t *testing.T, sequence int64) func() *EventSink {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events.sqlite")
	open := func() *EventSink {
		db, err := sql.Open(repairdb.Name, path)
		require.NoError(t, err)
		db.SetMaxOpenConns(1)
		require.NoError(t, db.PingContext(t.Context()))
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return &EventSink{store: db, chainID: t.Name()}
	}
	sink := open()
	contents, err := os.ReadFile("schema.sql")
	require.NoError(t, err)
	_, err = sink.DB().ExecContext(t.Context(), string(contents))
	require.NoError(t, err)
	for _, table := range []string{tableBlocks, tableTxResults, tableEvents} {
		_, err := sink.DB().ExecContext(t.Context(), "INSERT INTO sqlite_sequence (name, seq) VALUES ($1, $2)", table, sequence)
		require.NoError(t, err)
	}
	require.NoError(t, sink.Stop())
	return open
}

// Finding 7: exercise all three generated IDs, SQL foreign keys, driver Scan,
// block lookup, transaction encoding, and the joined consumer views.
func TestRepair_82_BigintIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sequence int64
	}{
		{"uint32_boundary", 1<<32 - 1},
		{"signed_64_bit", math.MaxInt64 - 64},
	} {
		t.Run(tc.name, func(t *testing.T) { repair82RoundTrip(t, repair82Embedded(t, tc.sequence), tc.sequence+1) })
	}
}

// Finding 34: repeated keys (including identical key/value pairs) are preserved
// separately in block and transaction events, including after Stop and reopen.
func TestRepair_82_RepeatedKeysSurviveReopen(t *testing.T) {
	repair82RoundTrip(t, repair82Embedded(t, 0), 1)
}

func repair82RoundTrip(t *testing.T, open func() *EventSink, firstID int64) {
	t.Helper()
	event := abci.Event{Type: "transfer", Attributes: []abci.EventAttribute{
		{Key: []byte("account"), Value: []byte("sender"), Index: true},
		{Key: []byte("account"), Value: []byte("private"), Index: false},
		{Key: []byte("account"), Value: []byte("receiver"), Index: true},
		{Key: []byte("account"), Value: []byte("receiver"), Index: true},
		{Key: []byte(""), Value: []byte(""), Index: true},
	}}
	block := types.EventDataNewBlockHeader{
		Header: types.Header{Height: 82},
		ResultFinalizeBlock: abci.ResponseFinalizeBlock{Events: []abci.Event{
			event,
			{Type: "", Attributes: event.Attributes},
		}},
	}
	txr := &abci.TxResultV2{Height: 82, Index: 0, Tx: types.Tx("repair-82"), Result: abci.ExecTxResult{Events: []abci.Event{event}}}
	sink := open()
	require.NoError(t, sink.IndexBlockEvents(block))
	require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{txr}))
	repair82AssertStored(t, sink.DB(), sink.chainID, txr, firstID)

	// Identity still rejects conflicting positions and invalid ordinals.
	for _, ordinal := range []int{-1, 0} {
		_, err := sink.DB().ExecContext(t.Context(), `
INSERT INTO attributes (event_id, ordinal, key, composite_key, value)
SELECT e.rowid, $1, 'account', 'transfer.account', 'invalid'
FROM events e JOIN blocks b ON b.rowid = e.block_id
WHERE b.chain_id = $2 AND e.type = 'transfer' AND e.tx_id IS NULL`, ordinal, sink.chainID)
		require.Error(t, err)
	}
	require.NoError(t, sink.Stop())

	sink = open()
	repair82AssertStored(t, sink.DB(), sink.chainID, txr, firstID)
	// Replaying committed records after reopening must not duplicate attributes.
	require.NoError(t, sink.IndexBlockEvents(block))
	require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{txr}))
	repair82AssertStored(t, sink.DB(), sink.chainID, txr, firstID)
	require.NoError(t, sink.Stop())
}

func repair82AssertStored(t *testing.T, db *sql.DB, chain string, txr *abci.TxResultV2, firstID int64) {
	t.Helper()
	var blockID, txID int64
	var encoded []byte
	var hash string
	require.NoError(t, db.QueryRowContext(t.Context(), `
SELECT b.rowid, t.rowid, t.tx_result, t.tx_hash
FROM blocks b JOIN tx_results t ON t.block_id = b.rowid
WHERE b.chain_id = $1 AND b.height = $2 AND t.index = $3`, chain, txr.Height, txr.Index).Scan(&blockID, &txID, &encoded, &hash))
	require.Equal(t, firstID, blockID)
	require.Equal(t, firstID, txID)
	require.Equal(t, fmt.Sprintf("%X", types.Tx(txr.Tx).Hash()), hash)
	var decoded abci.TxResult
	require.NoError(t, proto.Unmarshal(encoded, &decoded))
	require.True(t, proto.Equal(&abci.TxResult{Height: txr.Height, Index: txr.Index, Tx: txr.Tx, Result: txr.Result}, &decoded))

	var minID, maxID, count int64
	require.NoError(t, db.QueryRowContext(t.Context(), `
SELECT MIN(rowid), MAX(rowid), COUNT(*) FROM events WHERE block_id = $1`, blockID).Scan(&minID, &maxID, &count))
	require.Equal(t, firstID, minID)
	require.Equal(t, firstID+4, maxID)
	require.EqualValues(t, 5, count)
	require.NoError(t, db.QueryRowContext(t.Context(), `
SELECT COUNT(*) FROM attributes a JOIN events e ON a.event_id = e.rowid WHERE e.block_id = $1`, blockID).Scan(&count))
	require.EqualValues(t, 11, count)

	for _, side := range []struct{ view, predicate string }{
		{"block_events", "e.tx_id IS NULL"},
		{"tx_events", "e.tx_id IS NOT NULL"},
	} {
		rows, err := db.QueryContext(t.Context(), `
SELECT a.ordinal, a.key, a.composite_key, a.value
FROM attributes a JOIN events e ON a.event_id = e.rowid
WHERE e.block_id = $1 AND e.type = 'transfer' AND `+side.predicate+` ORDER BY a.ordinal`, blockID)
		require.NoError(t, err)
		type attribute struct {
			ordinal               int
			key, composite, value string
		}
		var got []attribute
		for rows.Next() {
			var a attribute
			require.NoError(t, rows.Scan(&a.ordinal, &a.key, &a.composite, &a.value))
			got = append(got, a)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		require.Equal(t, []attribute{
			{0, "account", "transfer.account", "sender"},
			{2, "account", "transfer.account", "receiver"},
			{3, "account", "transfer.account", "receiver"},
			{4, "", "transfer.", ""},
		}, got, side.view)
		rows, err = db.QueryContext(t.Context(), `SELECT key, composite_key, value FROM `+side.view+` WHERE chain_id = $1 AND height = $2 AND type = 'transfer'`, chain, txr.Height)
		require.NoError(t, err)
		var consumer []string
		for rows.Next() {
			var key, composite, value string
			require.NoError(t, rows.Scan(&key, &composite, &value))
			consumer = append(consumer, key+"|"+composite+"|"+value)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		require.ElementsMatch(t, []string{"account|transfer.account|sender", "account|transfer.account|receiver", "account|transfer.account|receiver", "|transfer.|"}, consumer, side.view)
	}
}
