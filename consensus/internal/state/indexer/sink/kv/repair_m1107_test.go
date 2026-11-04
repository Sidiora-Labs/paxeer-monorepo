package kv

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/google/orderedcode"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1107CompareResults(got, want []*abci.TxResultV2) error {
	if len(got) != len(want) {
		return fmt.Errorf("transaction result count: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] == nil || want[i] == nil {
			return fmt.Errorf("nil transaction result at index %d", i)
		}
	}
	got, want = slices.Clone(got), slices.Clone(want)
	byTx := func(a, b *abci.TxResultV2) int {
		return bytes.Compare(a.Tx, b.Tx)
	}
	slices.SortFunc(got, byTx)
	slices.SortFunc(want, byTx)
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			return fmt.Errorf("transaction result %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	return nil
}

func TestRepairM_1107_LegacySearchResults(t *testing.T) {
	TestTxSearchDeprecatedIndexing(t)
}

func TestRepairM_1107_RejectMissingMatches(t *testing.T) {
	store, sink, expected := deprecatedTxSearchFixture(t)
	q := query.MustCompile("sender = 'addr1'")
	results, err := sink.SearchTxEvents(t.Context(), q)
	require.NoError(t, err)
	require.NoError(t, repairM1107CompareResults(results, expected[1:]))

	key, err := orderedcode.Append(nil, types.TxHashKey, string(types.Tx(expected[1].Tx).Hash().Bytes()))
	require.NoError(t, err)
	require.NoError(t, store.Delete(key))
	results, err = sink.SearchTxEvents(t.Context(), q)
	require.NoError(t, err)
	require.Empty(t, results)
	require.Error(t, repairM1107CompareResults(results, expected[1:]))

	results, err = sink.SearchTxEvents(t.Context(), query.MustCompile("tx.height = 1"))
	require.NoError(t, err)
	require.NoError(t, repairM1107CompareResults(results, expected[:1]))
	require.Error(t, repairM1107CompareResults(results, expected))
}

func TestRepairM_1107_RejectUnexpectedMatches(t *testing.T) {
	_, sink, expected := deprecatedTxSearchFixture(t)
	q := query.MustCompile("tx.height = 1")
	results, err := sink.SearchTxEvents(t.Context(), q)
	require.NoError(t, err)
	require.NoError(t, repairM1107CompareResults(results, expected))

	extra := txResultWithEvents(nil)
	extra.Tx = types.Tx("UNEXPECTED TRANSACTION")
	extra.Index = 2
	require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{extra}))
	results, err = sink.SearchTxEvents(t.Context(), q)
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Error(t, repairM1107CompareResults(results, expected))

	results, err = sink.SearchTxEvents(t.Context(), query.MustCompile(fmt.Sprintf("tx.hash = '%X'", types.Tx(extra.Tx).Hash())))
	require.NoError(t, err)
	require.NoError(t, repairM1107CompareResults(results, []*abci.TxResultV2{extra}))
	require.Error(t, repairM1107CompareResults(results, expected[1:]))
	require.Error(t, repairM1107CompareResults(results, nil))
}

func TestRepairM_1107_LegacyProducerRecord(t *testing.T) {
	store, sink, expected := deprecatedTxSearchFixture(t)
	legacy := expected[1]
	hash := types.Tx(legacy.Tx).Hash().Bytes()
	key, err := orderedcode.Append(nil, types.TxHashKey, string(hash))
	require.NoError(t, err)
	legacyBytes, err := store.Get(key)
	require.NoError(t, err)
	require.NotEmpty(t, legacyBytes)

	producerStore, producerSink, _ := deprecatedTxSearchFixture(t)
	require.NoError(t, producerSink.IndexTxEvents([]*abci.TxResultV2{legacy}))
	producerBytes, err := producerStore.Get(key)
	require.NoError(t, err)
	require.Equal(t, producerBytes, legacyBytes)

	result, err := sink.GetTxByHash(hash)
	require.NoError(t, err)
	require.Equal(t, legacy, result)
	require.Equal(t, legacy.Tx, result.Tx)
}
