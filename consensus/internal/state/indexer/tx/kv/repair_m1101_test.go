package kv

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query/syntax"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepairM_1101_HashConjunctions(t *testing.T) {
	store := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	txi := NewTxIndex(store)
	first := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{
			{Key: []byte("owner"), Value: []byte("Ivan"), Index: true},
			{Key: []byte("number"), Value: []byte("1"), Index: true},
			{Key: []byte("private"), Value: []byte("hidden"), Index: false},
		}},
	})
	first.Height = 7
	second := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{
			{Key: []byte("owner"), Value: []byte("Vlad"), Index: true},
			{Key: []byte("number"), Value: []byte("2"), Index: true},
		}},
	})
	second.Tx = types.Tx("second indexed transaction")
	second.Height = 9
	second.Index = 1
	require.NoError(t, txi.Index([]*abci.TxResultV2{first, second}))
	hash := types.Tx(first.Tx).Hash()
	otherHash := types.Tx(second.Tx).Hash()
	missingHash := types.Tx("not indexed").Hash()
	equal := fmt.Sprintf("tx.hash = '%X'", hash)

	tests := []struct {
		name string
		q    string
		want []*abci.TxResultV2
	}{
		{"hash only", equal, []*abci.TxResultV2{first}},
		{"matching height", equal + " AND tx.height = 7", []*abci.TxResultV2{first}},
		{"wrong height", equal + " AND tx.height = 9", nil},
		{"height before hash", "tx.height = 9 AND " + equal, nil},
		{"matching range", equal + " AND tx.height >= 7 AND tx.height < 9", []*abci.TxResultV2{first}},
		{"wrong lower bound", equal + " AND tx.height > 7", nil},
		{"wrong upper bound", "tx.height < 7 AND " + equal, nil},
		{"matching event", equal + " AND account.owner = 'Ivan'", []*abci.TxResultV2{first}},
		{"event from another transaction", equal + " AND account.owner = 'Vlad'", nil},
		{"event before hash", "account.owner = 'Vlad' AND " + equal, nil},
		{"unindexed event", equal + " AND account.private = 'hidden'", nil},
		{"multiple filters", equal + " AND account.number >= 1 AND account.number <= 2 AND account.owner = 'Ivan' AND tx.height = 7", []*abci.TxResultV2{first}},
		{"failed range with matching event", equal + " AND account.number > 1 AND account.owner = 'Ivan'", nil},
		{"event contains", equal + " AND account.owner CONTAINS 'van'", []*abci.TxResultV2{first}},
		{"failed event contains", equal + " AND account.owner CONTAINS 'lad'", nil},
		{"event matches", equal + " AND account.owner MATCHES '^Ivan$'", []*abci.TxResultV2{first}},
		{"failed event matches", equal + " AND account.owner MATCHES '^Vlad$'", nil},
		{"event exists", equal + " AND account.owner EXISTS", []*abci.TxResultV2{first}},
		{"event absent", equal + " AND account.missing EXISTS", nil},
		{"repeated equality", equal + " AND " + equal, []*abci.TxResultV2{first}},
		{"hash case", equal + fmt.Sprintf(" AND tx.hash = '%s'", strings.ToLower(fmt.Sprintf("%X", hash))), []*abci.TxResultV2{first}},
		{"contradictory hashes", equal + fmt.Sprintf(" AND tx.hash = '%X'", otherHash), nil},
		{"missing hash", fmt.Sprintf("tx.hash = '%X'", missingHash), nil},
		{"missing hash with matching event", fmt.Sprintf("tx.hash = '%X' AND account.owner = 'Ivan'", missingHash), nil},
		{"predicate after empty intersection", equal + fmt.Sprintf(" AND tx.hash = '%X' AND account.owner = 'Ivan'", otherHash), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := txi.Search(t.Context(), query.MustCompile(tc.q))
			require.NoError(t, err)
			require.Len(t, got, len(tc.want))
			require.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestRepairM_1101_UnsupportedHashOperators(t *testing.T) {
	store := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	txi := NewTxIndex(store)
	tx := txResultWithEvents(nil)
	require.NoError(t, txi.Index([]*abci.TxResultV2{tx}))
	hash := types.Tx(tx.Tx).Hash()
	missing := types.Tx("missing").Hash()
	for _, expression := range []string{
		fmt.Sprintf("tx.hash CONTAINS '%X'", hash),
		fmt.Sprintf("tx.hash MATCHES '%X'", hash),
		"tx.hash EXISTS",
		"tx.hash > 1",
		"tx.hash >= 1",
		"tx.hash < 1",
		"tx.hash <= 1",
	} {
		for _, prefix := range []string{
			"",
			fmt.Sprintf("tx.hash = '%X' AND ", hash),
			fmt.Sprintf("tx.hash = '%X' AND ", missing),
			"account.missing = 'none' AND ",
		} {
			t.Run(prefix+expression, func(t *testing.T) {
				q := query.MustCompile(prefix + expression)
				got, err := txi.Search(t.Context(), q)
				var refusal *UnsupportedHashOperatorError
				require.ErrorAs(t, err, &refusal)
				conditions := q.Syntax()
				require.Equal(t, conditions[len(conditions)-1].Op, refusal.Operator)
				require.NotEqual(t, syntax.Token(syntax.TEq), refusal.Operator)
				require.Empty(t, got)
			})
		}
	}
}

func TestRepairM_1101_InvalidHashes(t *testing.T) {
	store := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	txi := NewTxIndex(store)
	tx := txResultWithEvents(nil)
	require.NoError(t, txi.Index([]*abci.TxResultV2{tx}))
	hash := types.Tx(tx.Tx).Hash()
	for _, prefix := range []string{"", fmt.Sprintf("tx.hash = '%X' AND ", hash), "account.missing EXISTS AND "} {
		t.Run(prefix+"invalid hex", func(t *testing.T) {
			got, err := txi.Search(t.Context(), query.MustCompile(prefix+"tx.hash = 'GG'"))
			var invalid hex.InvalidByteError
			require.ErrorAs(t, err, &invalid)
			require.Empty(t, got)
		})
		t.Run(prefix+"empty hash", func(t *testing.T) {
			got, err := txi.Search(t.Context(), query.MustCompile(prefix+"tx.hash = ''"))
			require.ErrorIs(t, err, indexer.ErrorEmptyHash)
			require.Empty(t, got)
		})
	}
}

func TestRepairM_1101_LegacyIndexRecovery(t *testing.T) {
	testTxSearchDeprecatedIndexing(t)
}
