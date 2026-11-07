package psql

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/psql/repairfixture"
)

func repairM1106Sink(t *testing.T) *EventSink {
	t.Helper()
	db, err := repairfixture.Open(":memory:")
	require.NoError(t, err)
	sink := NewEventSinkWithDB(db, t.Name())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	schema, err := os.ReadFile("schema.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), string(schema))
	require.NoError(t, err)
	return sink
}

func repairM1106Produce(t *testing.T, sink *EventSink, height int64) (time.Time, time.Time) {
	t.Helper()
	block := newTestBlockHeader()
	block.Header.Height = height
	// created_at records the sink's write time, rather than the header's time.
	block.Header.Time = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	txr := txResultWithEvents([]abci.Event{makeIndexedEvent("account.owner", "repair-1106")})
	txr.Height = block.Header.Height
	started := time.Now().UTC()
	require.NoError(t, sink.IndexBlockEvents(block))
	require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{txr}))
	return started, time.Now().UTC()
}

func TestRepairM_1106_PersistedHeightAndTimestamp(t *testing.T) {
	sink := repairM1106Sink(t)
	for _, height := range []int64{1, 2} {
		started, finished := repairM1106Produce(t, sink, height)
		require.NoError(t, verifyBlock(sink.DB(), height, sink.chainID))
		for _, table := range []string{tableBlocks, tableTxResults, viewTxEvents} {
			require.NoError(t, verifyTimeStamp(sink.DB(), table, height, sink.chainID, started, finished))
		}
	}
}

func TestRepairM_1106_UnproducedRowsFailQualification(t *testing.T) {
	sink := repairM1106Sink(t)
	started, finished := repairM1106Produce(t, sink, 1)
	for _, tc := range []struct {
		height int64
		chain  string
	}{
		{2, sink.chainID},
		{1, sink.chainID + "-absent"},
	} {
		require.ErrorIs(t, verifyBlock(sink.DB(), tc.height, tc.chain), sql.ErrNoRows)
		for _, table := range []string{tableBlocks, tableTxResults, viewTxEvents} {
			require.ErrorIs(t, verifyTimeStamp(sink.DB(), table, tc.height, tc.chain, started, finished), sql.ErrNoRows)
		}
	}
}

func TestRepairM_1106_RemovedRowsFailQualification(t *testing.T) {
	for _, table := range []string{tableBlocks, tableTxResults, viewTxEvents} {
		t.Run(table, func(t *testing.T) {
			sink := repairM1106Sink(t)
			started, finished := repairM1106Produce(t, sink, 1)
			// A different persisted height must not hide the expected row's loss.
			repairM1106Produce(t, sink, 2)
			require.NoError(t, verifyBlock(sink.DB(), 1, sink.chainID))
			require.NoError(t, verifyTimeStamp(sink.DB(), table, 1, sink.chainID, started, finished))

			predicate := "block_id IN (SELECT rowid FROM blocks WHERE height = 1)"
			if table != tableBlocks {
				predicate += " AND tx_id IS NOT NULL"
			}
			_, err := sink.DB().ExecContext(t.Context(), "DELETE FROM attributes WHERE event_id IN (SELECT rowid FROM events WHERE "+predicate+")")
			require.NoError(t, err)
			deleted, err := sink.DB().ExecContext(t.Context(), "DELETE FROM events WHERE "+predicate)
			require.NoError(t, err)
			n, err := deleted.RowsAffected()
			require.NoError(t, err)
			require.Positive(t, n)
			if table != viewTxEvents {
				deleted, err = sink.DB().ExecContext(t.Context(), "DELETE FROM tx_results WHERE block_id IN (SELECT rowid FROM blocks WHERE height = 1)")
				require.NoError(t, err)
				n, err = deleted.RowsAffected()
				require.NoError(t, err)
				require.EqualValues(t, 1, n)
			}
			if table == tableBlocks {
				deleted, err = sink.DB().ExecContext(t.Context(), "DELETE FROM blocks WHERE height = 1")
				require.NoError(t, err)
				n, err = deleted.RowsAffected()
				require.NoError(t, err)
				require.EqualValues(t, 1, n)
				require.ErrorIs(t, verifyBlock(sink.DB(), 1, sink.chainID), sql.ErrNoRows)
			} else {
				require.NoError(t, verifyBlock(sink.DB(), 1, sink.chainID))
			}
			require.ErrorIs(t, verifyTimeStamp(sink.DB(), table, 1, sink.chainID, started, finished), sql.ErrNoRows)
			require.NoError(t, verifyBlock(sink.DB(), 2, sink.chainID))
		})
	}
}

func TestRepairM_1106_OutOfIntervalTimestampsFailQualification(t *testing.T) {
	for _, table := range []string{tableBlocks, tableTxResults, viewTxEvents} {
		t.Run(table, func(t *testing.T) {
			sink := repairM1106Sink(t)
			started, finished := repairM1106Produce(t, sink, 1)
			require.NoError(t, verifyTimeStamp(sink.DB(), table, 1, sink.chainID, started, finished))
			source := table
			if table == viewTxEvents {
				source = tableTxResults
			}
			for _, timestamp := range []time.Time{started.Add(-time.Second), finished.Add(time.Second)} {
				updated, err := sink.DB().ExecContext(t.Context(), "UPDATE "+source+" SET created_at = $1", timestamp)
				require.NoError(t, err)
				n, err := updated.RowsAffected()
				require.NoError(t, err)
				require.EqualValues(t, 1, n)
				err = verifyTimeStamp(sink.DB(), table, 1, sink.chainID, started, finished)
				require.Error(t, err)
				require.NotErrorIs(t, err, sql.ErrNoRows)
			}
		})
	}
}
