package kv_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	blockidxkv "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/block/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1100Indexer(t *testing.T) *blockidxkv.BlockerIndexer {
	t.Helper()
	store := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	idx := blockidxkv.New(dbm.NewPrefixDB(store, []byte("block_events")))
	for _, block := range []struct {
		height   int64
		proposer string
		score    string
	}{
		{7, "alice", "70"},
		{8, "bob", "80"},
	} {
		require.NoError(t, idx.Index(types.EventDataNewBlockHeader{
			Header: types.Header{Height: block.height},
			ResultFinalizeBlock: abci.ResponseFinalizeBlock{
				Events: []abci.Event{{
					Type: "finalize",
					Attributes: []abci.EventAttribute{
						{Key: []byte("proposer"), Value: []byte(block.proposer), Index: true},
						{Key: []byte("score"), Value: []byte(block.score), Index: true},
						{Key: []byte("private"), Value: []byte("hidden"), Index: false},
					},
				}},
			},
		}))
	}
	for _, height := range []int64{7, 8} {
		indexed, err := idx.Has(height)
		require.NoError(t, err)
		require.True(t, indexed)
	}
	return idx
}

func TestRepairM_1100_HeightAndEvents(t *testing.T) {
	idx := repairM1100Indexer(t)
	for _, tc := range []struct {
		name  string
		query string
		want  []int64
	}{
		{"height_only", "block.height = 7", []int64{7}},
		{"missing_height", "block.height = 9 AND finalize.proposer = 'alice'", []int64{}},
		{"matching_event", "block.height = 7 AND finalize.proposer = 'alice'", []int64{7}},
		{"matching_event_first", "finalize.proposer = 'alice' AND block.height = 7", []int64{7}},
		{"absent_event", "block.height = 7 AND missing.key = 'value'", []int64{}},
		{"absent_event_first", "missing.key = 'value' AND block.height = 7", []int64{}},
		{"other_block_event", "block.height = 7 AND finalize.proposer = 'bob'", []int64{}},
		{"unindexed_event", "block.height = 7 AND finalize.private = 'hidden'", []int64{}},
		{"absent_exists", "block.height = 7 AND missing.key EXISTS", []int64{}},
		{"matching_exists", "block.height = 7 AND finalize.proposer EXISTS", []int64{7}},
		{"absent_contains", "block.height = 7 AND finalize.proposer CONTAINS 'bob'", []int64{}},
		{"matching_contains", "block.height = 7 AND finalize.proposer CONTAINS 'ali'", []int64{7}},
		{"absent_matches", "block.height = 7 AND finalize.proposer MATCHES '^bob$'", []int64{}},
		{"matching_matches", "block.height = 7 AND finalize.proposer MATCHES '^alice$'", []int64{7}},
		{"event_range_excludes_height", "block.height = 7 AND finalize.score >= 80", []int64{}},
		{"event_range_includes_height", "block.height = 7 AND finalize.score >= 70 AND finalize.score < 80", []int64{7}},
		{"multiple_events", "block.height = 7 AND finalize.proposer = 'alice' AND finalize.score = 70", []int64{7}},
		{"later_event_excludes_height", "block.height = 7 AND finalize.proposer = 'alice' AND finalize.score = 80", []int64{}},
		{"without_height", "finalize.score >= 70", []int64{7, 8}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := idx.Search(t.Context(), query.MustCompile(tc.query))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRepairM_1100_ContradictoryHeights(t *testing.T) {
	idx := repairM1100Indexer(t)
	for _, tc := range []struct {
		name  string
		query string
		want  []int64
	}{
		{"different_equalities", "block.height = 7 AND block.height = 8", []int64{}},
		{"different_equalities_reversed", "block.height = 8 AND block.height = 7", []int64{}},
		{"missing_equality", "block.height = 7 AND block.height = 9", []int64{}},
		{"equalities_with_event", "block.height = 7 AND finalize.proposer = 'alice' AND block.height = 8", []int64{}},
		{"exclusive_lower_bound", "block.height = 7 AND block.height > 7", []int64{}},
		{"exclusive_upper_bound", "block.height = 7 AND block.height < 7", []int64{}},
		{"inclusive_lower_bound", "block.height = 7 AND block.height >= 8", []int64{}},
		{"inclusive_upper_bound", "block.height = 8 AND block.height <= 7", []int64{}},
		{"range_first", "block.height > 7 AND block.height = 7", []int64{}},
		{"duplicate_equality", "block.height = 7 AND block.height = 7", []int64{7}},
		{"matching_range", "block.height = 7 AND block.height >= 7 AND block.height < 8", []int64{7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := idx.Search(t.Context(), query.MustCompile(tc.query))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
