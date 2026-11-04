package occ

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/mem"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/messages"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM790StoreContext(t *testing.T, values map[string]string) sdk.Context {
	t.Helper()
	stores := make(map[sdk.StoreKey]sdk.CacheWrapper, len(values))
	keys := make(map[string]sdk.StoreKey, len(values))
	for name, value := range values {
		key := sdk.NewKVStoreKey(name)
		db := dbm.NewMemDB()
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		kv := mem.NewStoreWithDB(db)
		kv.Set([]byte("balance"), []byte(value))
		stores[key] = kv
		keys[name] = key
	}
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := cachemulti.NewStore(db, stores, keys, nil, nil, nil, 0)
	return sdk.NewContext(ms, tmproto.Header{}, false)
}

func TestRepairM_790_StoreNames(t *testing.T) {
	expected := repairM790StoreContext(t, map[string]string{"bank": "100", "auth": "1"})
	actual := repairM790StoreContext(t, map[string]string{"auth": "1", "bank": "100"})
	assertEqualState(t, expected, actual, t.Name())

	replaced := repairM790StoreContext(t, map[string]string{"auth": "1", "replacement": "100"})
	for _, pair := range [][2]sdk.Context{{expected, replaced}, {replaced, expected}} {
		err := compareStoreKeyNames(pair[0].MultiStore().StoreKeys(), pair[1].MultiStore().StoreKeys())
		require.ErrorContains(t, err, "store names do not match")
	}
	missing := repairM790StoreContext(t, map[string]string{"auth": "1"})
	require.ErrorContains(t, compareStoreKeyNames(expected.MultiStore().StoreKeys(), missing.MultiStore().StoreKeys()), "number of stores")

	// Ignored stores still belong to the named set; only their contents may differ.
	ignoredExpected := repairM790StoreContext(t, map[string]string{
		"bank": "100", "mem_capability": "before", "epoch": "before", "deferredcache": "before",
	})
	ignoredActual := repairM790StoreContext(t, map[string]string{
		"bank": "100", "mem_capability": "after", "epoch": "after", "deferredcache": "after",
	})
	assertEqualState(t, ignoredExpected, ignoredActual, t.Name())
	ignoredReplaced := repairM790StoreContext(t, map[string]string{
		"bank": "100", "mem_capability": "after", "epoch": "after", "replacement": "after",
	})
	require.Error(t, compareStoreKeyNames(ignoredExpected.MultiStore().StoreKeys(), ignoredReplaced.MultiStore().StoreKeys()))
}

func TestRepairM_790_EventAttributes(t *testing.T) {
	em := sdk.NewEventManager()
	em.EmitEvent(sdk.NewEvent("transfer", sdk.NewAttribute("amount", "1"), sdk.NewAttribute("amount", "2")))
	encoded, err := em.ABCIEvents()[0].Marshal()
	require.NoError(t, err)
	var decoded types.Event
	require.NoError(t, decoded.Unmarshal(encoded))
	require.Len(t, decoded.Attributes, 2)
	a, b := decoded.Attributes[0], decoded.Attributes[1]
	expected := []types.EventAttribute{a, a, b}
	unequal := []types.EventAttribute{a, b, b}
	require.Error(t, compareEventAttributes(expected, unequal))
	require.Error(t, compareEventAttributes(unequal, expected))
	assertEqualEventAttributes(t, t.Name(), expected, []types.EventAttribute{b, a, a})
	require.Error(t, compareEventAttributes(expected, []types.EventAttribute{a, b}))

	indexed := a
	indexed.Index = !a.Index
	require.Error(t, compareEventAttributes([]types.EventAttribute{a}, []types.EventAttribute{indexed}))

	// Key/value boundaries must remain distinct even when the bytes contain '='.
	em.EmitEvent(sdk.NewEvent("boundaries", sdk.NewAttribute("a=b", "c"), sdk.NewAttribute("a", "b=c")))
	boundaries := em.ABCIEvents()[1].Attributes
	require.Error(t, compareEventAttributes(boundaries[:1], boundaries[1:]))
	assertEqualEventAttributes(t, t.Name(), nil, nil)
}

func TestRepairM_790_ExecTxResults(t *testing.T) {
	accts := utils.NewTestAccounts(2)
	blockTime := time.Unix(1700000000, 0)
	sequential := utils.NewTestContext(t, accts, blockTime, 1, false)
	txs := messages.BankTransfer(sequential, 2)
	sEvents, sResults, _, err := utils.RunWithoutOCC(sequential, txs)
	require.NoError(t, err)
	require.Len(t, sResults, len(txs))

	parallel := utils.NewTestContext(t, accts, blockTime, 2, true)
	pEvents, pResults, _, err := utils.RunWithOCC(parallel, txs)
	require.NoError(t, err)
	require.Len(t, pResults, len(txs))
	assertExecTxResultCode(t, sResults, pResults, 0, t.Name())
	assertEqualEvents(t, sEvents, pEvents, t.Name())
	assertEqualExecTxResults(t, sResults, pResults, t.Name())
	assertEqualState(t, sequential.Ctx, parallel.Ctx, t.Name())

	// Round-trip results produced by the real bank transaction execution.
	decoded := make([]*types.ExecTxResult, len(sResults))
	for i, result := range sResults {
		encoded, err := result.Marshal()
		require.NoError(t, err)
		decoded[i] = new(types.ExecTxResult)
		require.NoError(t, decoded[i].Unmarshal(encoded))
	}
	a, b := decoded[0], decoded[1]
	require.NotEqual(t, a, b)
	expected := []*types.ExecTxResult{a, a, b}
	unequal := []*types.ExecTxResult{a, b, b}
	require.Error(t, compareExecTxResults(expected, unequal))
	require.Error(t, compareExecTxResults(unequal, expected))
	assertEqualExecTxResults(t, expected, []*types.ExecTxResult{b, a, a}, t.Name())
	require.Error(t, compareExecTxResults(expected, []*types.ExecTxResult{a, b}))
	assertEqualExecTxResults(t, nil, nil, t.Name())
	require.Equal(t, []*types.ExecTxResult{a, a, b}, expected)
	require.Equal(t, []*types.ExecTxResult{a, b, b}, unequal)
}
