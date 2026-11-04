package indexer_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/psql"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/psql/repairfixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// The application emits app.key from the transaction, the sink persists the
// encoded result and indexes, and searches decode the stored result again.
func repairM1099Produce(t *testing.T, sink indexer.EventSink, height int64, txs ...string) []*abci.TxResultV2 {
	t.Helper()
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	raw := make([][]byte, len(txs))
	for i, tx := range txs {
		raw[i] = []byte(tx)
	}
	header := types.Header{Height: height}
	response, err := app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: header.ToProto(), Txs: raw,
	})
	require.NoError(t, err)
	_, err = app.Commit(t.Context())
	require.NoError(t, err)
	require.NoError(t, sink.IndexBlockEvents(types.EventDataNewBlockHeader{
		Header: types.Header{Height: height}, ResultFinalizeBlock: *response,
	}))
	results := make([]*abci.TxResultV2, len(raw))
	for i := range raw {
		require.Zero(t, response.TxResults[i].Code)
		results[i] = &abci.TxResultV2{Height: height, Index: uint32(i), Tx: raw[i], Result: *response.TxResults[i]}
	}
	require.NoError(t, sink.IndexTxEvents(results))
	return results
}

func repairM1099Permutations(parts []string, visit func([]string)) {
	var walk func(int)
	walk = func(i int) {
		if i == len(parts) {
			visit(parts)
			return
		}
		for j := i; j < len(parts); j++ {
			parts[i], parts[j] = parts[j], parts[i]
			walk(i + 1)
			parts[i], parts[j] = parts[j], parts[i]
		}
	}
	walk(0)
}

func repairM1099AssertSearch(t *testing.T, sink indexer.EventSink, expression string, want []int64, block bool) {
	t.Helper()
	q, err := query.New(expression)
	require.NoError(t, err)
	var got []int64
	if block {
		got, err = sink.SearchBlockEvents(t.Context(), q)
		require.NoError(t, err, expression)
	} else {
		results, err := sink.SearchTxEvents(t.Context(), q)
		require.NoError(t, err, expression)
		for _, result := range results {
			got = append(got, result.Height)
			stored, err := sink.GetTxByHash(types.Tx(result.Tx).Hash().Bytes())
			require.NoError(t, err)
			require.Equal(t, result, stored)
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	}
	// Check count as well as values to catch duplicates and extra endpoint rows.
	require.Equal(t, len(want), len(got), expression)
	for i := range want {
		require.Equal(t, want[i], got[i], expression)
	}
}

func TestRepairM_1099_RangeIntersection(t *testing.T) {
	sink := kv.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	for height := int64(1); height <= 15; height++ {
		repairM1099Produce(t, sink, height, fmt.Sprintf("%d=value", height))
	}
	cases := []struct {
		name       string
		conditions string
		want       []int64
	}{
		{"stronger_strict_lower", "x >= 5 AND x > 10", []int64{11, 12, 13, 14, 15}},
		{"stronger_inclusive_lower", "x > 5 AND x >= 10", []int64{10, 11, 12, 13, 14, 15}},
		{"lower_strict_tie", "x >= 10 AND x > 10", []int64{11, 12, 13, 14, 15}},
		{"lower_duplicates", "x >= 10 AND x >= 10 AND x >= 5", []int64{10, 11, 12, 13, 14, 15}},
		{"lower_strict_duplicates", "x > 10 AND x > 10 AND x >= 10", []int64{11, 12, 13, 14, 15}},
		{"stronger_strict_upper", "x <= 12 AND x < 5", []int64{1, 2, 3, 4}},
		{"stronger_inclusive_upper", "x < 12 AND x <= 5", []int64{1, 2, 3, 4, 5}},
		{"upper_strict_tie", "x <= 5 AND x < 5", []int64{1, 2, 3, 4}},
		{"upper_duplicates", "x <= 5 AND x <= 5 AND x <= 12", []int64{1, 2, 3, 4, 5}},
		{"upper_strict_duplicates", "x < 5 AND x < 5 AND x <= 5", []int64{1, 2, 3, 4}},
		{"mixed_inclusive", "x > 3 AND x >= 5 AND x < 12 AND x <= 10", []int64{5, 6, 7, 8, 9, 10}},
		{"mixed_strict", "x >= 3 AND x > 5 AND x <= 12 AND x < 10", []int64{6, 7, 8, 9}},
		{"inclusive_singleton", "x >= 5 AND x <= 5 AND x >= 3 AND x <= 12", []int64{5}},
		{"strict_lower_empty", "x > 5 AND x <= 5", nil},
		{"strict_upper_empty", "x >= 5 AND x < 5", nil},
		{"contradictory", "x >= 10 AND x <= 5 AND x >= 3 AND x <= 12", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repairM1099Permutations(strings.Split(tc.conditions, " AND "), func(parts []string) {
				for _, consumer := range []struct {
					key   string
					block bool
				}{
					{"block.height", true}, {"tx.height", false}, {"app.key", false},
				} {
					expression := strings.ReplaceAll(strings.Join(parts, " AND "), "x ", consumer.key+" ")
					repairM1099AssertSearch(t, sink, expression, tc.want, consumer.block)
				}
			})
		})
	}
}

func TestRepairM_1099_TemporalRangeIntersection(t *testing.T) {
	sink := kv.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for day := int64(0); day < 5; day++ {
		height := start.Add(time.Duration(day) * 24 * time.Hour).Unix()
		repairM1099Produce(t, sink, height, fmt.Sprintf("%d=value", height))
	}
	for _, tc := range []struct {
		expression string
		wantDays   []int
	}{
		{"x >= DATE 2020-01-01 AND x > TIME 2020-01-02T00:00:00Z AND x <= DATE 2020-01-04", []int{2, 3}},
		{"x >= DATE 2020-01-02 AND x > TIME 2020-01-02T00:00:00Z", []int{2, 3, 4}},
		{"x <= DATE 2020-01-04 AND x < TIME 2020-01-03T00:00:00Z", []int{0, 1}},
		{"x <= DATE 2020-01-03 AND x < TIME 2020-01-03T00:00:00Z", []int{0, 1}},
		{"x >= DATE 2020-01-04 AND x <= DATE 2020-01-02", nil},
	} {
		var want []int64
		for _, day := range tc.wantDays {
			want = append(want, start.Add(time.Duration(day)*24*time.Hour).Unix())
		}
		repairM1099Permutations(strings.Split(tc.expression, " AND "), func(parts []string) {
			for _, key := range []string{"block.height", "tx.height", "app.key"} {
				expression := strings.ReplaceAll(strings.Join(parts, " AND "), "x ", key+" ")
				repairM1099AssertSearch(t, sink, expression, want, key == "block.height")
			}
		})
	}
}

func TestRepairM_1099_DocumentedSQL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.sqlite")
	open := func(chain string) *psql.EventSink {
		db, err := repairfixture.Open(path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return psql.NewEventSinkWithDB(db, chain)
	}
	sink := open("repair-m1099")
	schema, err := os.ReadFile("sink/psql/schema.sql")
	require.NoError(t, err)
	_, err = sink.DB().ExecContext(t.Context(), string(schema))
	require.NoError(t, err)
	results := repairM1099Produce(t, sink, 25, "recipient=alice", "other=bob")
	repairM1099Produce(t, sink, 26, "third=carol")
	other := open("another-chain")
	repairM1099Produce(t, other, 25, "other-chain=dan")
	// A committed ABCI event fixture supplies the type.key used in the docs.
	// All view rows, composite keys, hashes and encoded results are sink output.
	transfer := &abci.TxResultV2{Height: 26, Index: 1, Tx: []byte("transfer-to-alice"), Result: abci.ExecTxResult{Events: []abci.Event{
		{Type: "transfer", Attributes: []abci.EventAttribute{
			{Key: []byte("recipient"), Value: []byte("alice"), Index: true},
			{Key: []byte("recipient"), Value: []byte("private"), Index: false},
		}},
		{Type: "other", Attributes: []abci.EventAttribute{
			{Key: []byte("recipient"), Value: []byte("wrong-type"), Index: true},
			{Key: []byte("transfer.recipient"), Value: []byte("wrong-composite"), Index: true},
		}},
	}}}
	require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{transfer}))
	require.NoError(t, other.Stop())
	require.NoError(t, sink.Stop())
	sink = open("repair-m1099") // Queries consume persisted producer rows after reopen.

	// Execute the actual documentation text, substituting only its sample hash.
	file, err := parser.ParseFile(token.NewFileSet(), "doc.go", nil, parser.ParseComments)
	require.NoError(t, err)
	var doc *ast.CommentGroup = file.Doc
	require.NotNil(t, doc)
	examples := regexp.MustCompile(`(?s)SELECT\b.*?;`).FindAllString(doc.Text(), -1)
	require.Len(t, examples, 3)
	hash := fmt.Sprintf("%X", types.Tx(results[0].Tx).Hash())
	examples[0] = strings.ReplaceAll(examples[0], "3E7D1F...", hash)
	type eventRow struct {
		height           int64
		index            int
		chain, eventType string
		key, composite   string
		value, createdAt string
	}
	run := func(statement string) []eventRow {
		rows, err := sink.DB().QueryContext(t.Context(), statement)
		require.NoError(t, err, statement)
		defer func() { require.NoError(t, rows.Close()) }()
		var got []eventRow
		for rows.Next() {
			var row eventRow
			require.NoError(t, rows.Scan(&row.height, &row.index, &row.chain, &row.eventType, &row.key, &row.composite, &row.value, &row.createdAt))
			got = append(got, row)
		}
		require.NoError(t, rows.Err())
		return got
	}
	byHash := run(examples[0])
	require.Len(t, byHash, 5) // tx.hash, tx.height and three indexed application attributes.
	var composites []string
	for _, row := range byHash {
		require.EqualValues(t, 25, row.height)
		require.Zero(t, row.index)
		require.Equal(t, "repair-m1099", row.chain)
		composites = append(composites, row.composite)
		if row.composite == "tx.hash" {
			require.Equal(t, hash, row.value)
		}
	}
	require.ElementsMatch(t, []string{"tx.hash", "tx.height", "app.creator", "app.key", "app.index_key"}, composites)
	byHeight := run(examples[1])
	require.Len(t, byHeight, 15) // Two transactions on one chain and one on the other.
	counts := make(map[string]int)
	for _, row := range byHeight {
		require.EqualValues(t, 25, row.height)
		counts[row.chain]++
	}
	require.Equal(t, map[string]int{"repair-m1099": 10, "another-chain": 5}, counts)
	byComposite := run(examples[2])
	require.Len(t, byComposite, 1)
	require.Equal(t, "transfer", byComposite[0].eventType)
	require.Equal(t, "recipient", byComposite[0].key)
	require.Equal(t, "transfer.recipient", byComposite[0].composite)
	require.Equal(t, "alice", byComposite[0].value)
	require.EqualValues(t, 26, byComposite[0].height)
	require.Equal(t, 1, byComposite[0].index)

	// The documented join also recovers the full, actual producer protobuf.
	rawQuery := strings.Replace(examples[0], "SELECT e.*", "SELECT DISTINCT r.tx_result", 1)
	var encoded []byte
	require.NoError(t, sink.DB().QueryRowContext(t.Context(), rawQuery).Scan(&encoded))
	var decoded abci.TxResult
	require.NoError(t, proto.Unmarshal(encoded, &decoded))
	require.True(t, proto.Equal(&abci.TxResult{Height: results[0].Height, Index: results[0].Index, Tx: results[0].Tx, Result: results[0].Result}, &decoded))
}
