//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/stretchr/testify/require"
)

func TestStoreGenesisRoundTrip(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	ctx := chain.context(1, blockTime, 43)
	empty, err := ExportStoreGenesis(ctx, chain.key)
	require.NoError(t, err)
	require.Equal(t, DefaultStoreGenesis(), empty)
	host := chain.host(t)
	issuer := newTestActor(t, 22)
	asset := newTestAsset(t, issuer, 0x44)
	for i, activity := range []Activity{
		issuer.registerAsset(t, host, 0, blockTime, asset, 0x44, 1000),
		issuer.openAssetAccount(t, host, 1, blockTime, asset),
		issuer.mint(t, host, 2, blockTime, asset, 250),
	} {
		receipt := execute(t, host, chain.context(1, blockTime, uint64(41+i)), activity, issuer.authority())
		require.Equal(t, Code(0), receipt.ResultCode)
	}
	chain.commit()
	programs := kv.Pair{Key: moduleKVKey(9, []byte("progfee/active/v1")), Value: []byte{1, 2, 3}}
	ctx.KVStore(chain.key).Set(programs.Key, programs.Value)
	for _, key := range [][]byte{
		{idempotencyPrefix, 1}, moduleKVKey(2, []byte("foreign")),
		moduleKVKey(10, []byte("foreign")), {5, 1},
	} {
		ctx.KVStore(chain.key).Set(key, []byte{1, 2, 3})
	}
	gs, err := ExportStoreGenesis(ctx, chain.key)
	require.NoError(t, err)
	require.Equal(t, uint64(43), gs.LastSequence)
	require.NoError(t, gs.Validate())
	expected := map[string][]byte{}
	familyCounts := map[byte]int{}
	moduleCounts := map[uint16]int{}
	for key, value := range chain.dump() {
		if bytes.HasPrefix([]byte(key), []byte{1, 0, 1}) || bytes.HasPrefix([]byte(key), []byte{1, 0, 9}) || key[0] == lastSequenceKey || key[0] == identitySequencePrefix || key[0] == accountPrefix {
			expected[key] = value
		}
	}
	exportedRows := map[string][]byte{string([]byte{lastSequenceKey}): binary.BigEndian.AppendUint64(nil, gs.LastSequence)}
	for i, record := range gs.Records {
		if i > 0 {
			require.Less(t, bytes.Compare(gs.Records[i-1].Key, record.Key), 0)
		}
		switch record.Key[0] {
		case moduleKVPrefix:
			module := binary.BigEndian.Uint16(record.Key[1:3])
			require.Contains(t, []uint16{1, 9}, module)
			moduleCounts[module]++
		case identitySequencePrefix, accountPrefix:
		default:
			t.Fatalf("foreign row %x", record.Key)
		}
		familyCounts[record.Key[0]]++
		exportedRows[string(record.Key)] = record.Value
	}
	require.Positive(t, familyCounts[moduleKVPrefix])
	require.Positive(t, moduleCounts[1])
	require.Equal(t, 1, moduleCounts[9])
	require.Equal(t, programs.Value, exportedRows[string(programs.Key)])
	require.Equal(t, 1, familyCounts[identitySequencePrefix])
	require.Equal(t, 2, familyCounts[accountPrefix])
	require.Equal(t, expected, exportedRows)
	document, err := json.Marshal(gs)
	require.NoError(t, err)
	var decoded StoreGenesis
	require.NoError(t, json.Unmarshal(document, &decoded))
	restored := newTestChain(t)
	restoredCtx := restored.context(2, blockTime+1000, 44)
	require.NoError(t, InitStoreGenesis(restoredCtx, restored.key, decoded))
	restored.commit()
	again, err := ExportStoreGenesis(restoredCtx, restored.key)
	require.NoError(t, err)
	againJSON, err := json.Marshal(again)
	require.NoError(t, err)
	require.Equal(t, document, againJSON)
	require.Equal(t, expected, restored.dump())
	require.Equal(t, programs.Value, restoredCtx.KVStore(restored.key).Get(programs.Key))
	second := restored.host(t)
	before := restored.dump()
	var firstAsset AssetRecord
	for i, view := range []struct {
		host *Host
		ctx  sdk.Context
		key  sdk.StoreKey
	}{
		{host, ctx, chain.key}, {second, restoredCtx, restored.key},
	} {
		record, err := view.host.ReadAsset(view.ctx, asset.id)
		require.NoError(t, err)
		if i == 0 {
			firstAsset = record
		} else {
			require.Equal(t, firstAsset, record)
		}
		require.Equal(t, asset.id, record.AssetID)
		for _, balance := range []struct {
			id     [32]byte
			amount int64
		}{{asset.issuance, 750}, {asset.account, 250}} {
			amount, err := view.host.ReadBalance(view.ctx, balance.id, asset.id)
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(balance.amount), amount)
		}
		last, err := view.host.ReadLastSequence(view.ctx, issuer.did)
		require.NoError(t, err)
		require.Equal(t, uint64(43), last)
		identity, err := view.host.ReadIdentitySequence(view.ctx, issuer.did)
		require.NoError(t, err)
		require.Equal(t, uint64(3), identity)
		next, err := NextSequence(view.ctx, view.key)
		require.NoError(t, err)
		require.Equal(t, uint64(44), next)
	}
	replay, err := second.Execute(WithSequence(restoredCtx, 43), issuer.mint(t, second, 3, blockTime+1000, asset, 1), issuer.authority())
	require.Equal(t, codeSequenceReused, err)
	require.Equal(t, Receipt{}, replay)
	require.Equal(t, before, restored.dump())
	minted := execute(t, second, restoredCtx, issuer.mint(t, second, 3, blockTime+1000, asset, 1), issuer.authority())
	require.Equal(t, Code(0), minted.ResultCode)
	require.Equal(t, uint64(44), minted.GlobalSequence)
	before = restored.dump()
	require.NoError(t, InitStoreGenesis(restoredCtx, restored.key, *DefaultStoreGenesis()))
	require.Equal(t, before, restored.dump())
	NewStoreAppModule(restored.key).InitGenesis(restoredCtx, nil, (StoreAppModuleBasic{}).DefaultGenesis(nil))
	require.Equal(t, before, restored.dump())

	t.Run("empty JSON value", func(t *testing.T) {
		c := newTestChain(t)
		context := c.context(1, blockTime, 1)
		encoded, err := json.Marshal(StoreGenesis{Records: []kv.Pair{{Key: moduleKVKey(1, []byte("empty"))}}})
		require.NoError(t, err)
		var state StoreGenesis
		require.NoError(t, json.Unmarshal(encoded, &state))
		require.Nil(t, state.Records[0].Value)
		require.NoError(t, InitStoreGenesis(context, c.key, state))
		require.True(t, context.KVStore(c.key).Has(state.Records[0].Key))
		require.Empty(t, context.KVStore(c.key).Get(state.Records[0].Key))
	})
	t.Run("corrupt last sequence", func(t *testing.T) {
		branch, _ := ctx.CacheContext()
		branch.KVStore(chain.key).Set([]byte{lastSequenceKey}, []byte{1})
		state, err := ExportStoreGenesis(branch, chain.key)
		require.Equal(t, codeFatalInvariant, err)
		require.Nil(t, state)
	})
}

func TestStoreGenesisRefusesForeignRows(t *testing.T) {
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 22)
	asset := newTestAsset(t, actor, 0x44)
	receipt := execute(t, host, chain.context(1, blockTime, 41), actor.registerAsset(t, host, 0, blockTime, asset, 0x44, 1000), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	chain.commit()
	accounts := rawAccountRows(chain)
	require.NotEmpty(t, accounts)
	account := kv.Pair{Key: accountKey(asset.issuance), Value: chain.get(accountKey(asset.issuance))}
	require.NoError(t, (StoreGenesis{Records: []kv.Pair{account}}).Validate())
	foreignID := bytes.Clone(account.Key)
	foreignID[32] ^= 1
	tooMany := make([]kv.Pair, 4097)
	for i := range tooMany {
		tooMany[i] = kv.Pair{Key: moduleKVKey(1, binary.BigEndian.AppendUint32(nil, uint32(i))), Value: []byte{1}}
	}
	a := kv.Pair{Key: moduleKVKey(1, []byte("a")), Value: []byte{1}}
	b := kv.Pair{Key: moduleKVKey(1, []byte("b")), Value: []byte{2}}
	programs := kv.Pair{Key: moduleKVKey(9, []byte("progfee/active/v1")), Value: []byte{1, 2, 3}}
	require.NoError(t, (StoreGenesis{Records: []kv.Pair{programs}}).Validate())
	require.ErrorContains(t, (StoreGenesis{Records: []kv.Pair{{Key: moduleKVKey(2, []byte("progfee/active/v1")), Value: programs.Value}}}).Validate(), "foreign module row")
	programsRows := make([]kv.Pair, len(tooMany))
	for i := range programsRows {
		programsRows[i] = kv.Pair{Key: moduleKVKey(9, binary.BigEndian.AppendUint32(nil, uint32(i))), Value: []byte{1}}
	}
	require.NoError(t, (StoreGenesis{Records: programsRows[:4096]}).Validate())
	mixedRows := append(append([]kv.Pair(nil), tooMany[:4096]...), programs)
	require.NoError(t, (StoreGenesis{Records: append(append([]kv.Pair(nil), tooMany[:4095]...), programs)}).Validate())
	require.NoError(t, (StoreGenesis{Records: []kv.Pair{{Key: moduleKVKey(9, make([]byte, 128)), Value: make([]byte, 1024)}}}).Validate())
	cases := []struct {
		name    string
		records []kv.Pair
		index   int
	}{
		{"unsorted", []kv.Pair{b, a}, 1},
		{"duplicate", []kv.Pair{a, a}, 1},
		{"last sequence record", []kv.Pair{{Key: []byte{2}, Value: make([]byte, 8)}}, 0},
		{"legacy idempotency", []kv.Pair{{Key: []byte{4, 1}}}, 0},
		{"program blob", []kv.Pair{{Key: []byte{5, 1}}}, 0},
		{"foreign module", []kv.Pair{{Key: moduleKVKey(2, []byte("a"))}}, 0},
		{"short ledger key", []kv.Pair{{Key: []byte{1, 0, 1}}}, 0},
		{"long ledger key", []kv.Pair{{Key: moduleKVKey(1, make([]byte, 129))}}, 0},
		{"long ledger value", []kv.Pair{{Key: a.Key, Value: make([]byte, 1025)}}, 0},
		{"too many ledger rows", tooMany, 4096},
		{"short programs key", []kv.Pair{{Key: []byte{moduleKVPrefix, 0, 9}}}, 0},
		{"long programs key", []kv.Pair{{Key: moduleKVKey(9, make([]byte, 129))}}, 0},
		{"long programs value", []kv.Pair{{Key: programs.Key, Value: make([]byte, 1025)}}, 0},
		{"too many programs rows", programsRows, 4096},
		{"too many mixed module rows", mixedRows, 4096},
		{"short identity key", []kv.Pair{{Key: append([]byte{3}, make([]byte, 31)...), Value: make([]byte, 8)}}, 0},
		{"short identity value", []kv.Pair{{Key: append([]byte{3}, make([]byte, 32)...), Value: make([]byte, 7)}}, 0},
		{"short account key", []kv.Pair{{Key: []byte{6, 1}, Value: account.Value}}, 0},
		{"invalid account encoding", []kv.Pair{{Key: account.Key, Value: []byte{1}}}, 0},
		{"account id mismatch", []kv.Pair{{Key: foreignID, Value: account.Value}}, 0},
		{"empty key", []kv.Pair{{}}, 0},
	}
	before := chain.dump()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := StoreGenesis{LastSequence: 999, Records: tc.records}
			err := gs.Validate()
			require.ErrorContains(t, err, fmt.Sprintf("record %d:", tc.index))
			err = InitStoreGenesis(chain.context(2, blockTime, 42), chain.key, gs)
			require.ErrorContains(t, err, fmt.Sprintf("record %d:", tc.index))
			require.Equal(t, before, chain.dump())
		})
	}
}
