package keeper_test

import (
	"encoding/json"
	"math"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	chains "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair196Node struct {
	ctx    sdk.Context
	cms    sdk.CommitMultiStore
	key    sdk.StoreKey
	k      *keeper.Keeper
	closed bool
}

func repair196Open(t *testing.T, dir string) *repair196Node {
	t.Helper()
	db, err := dbm.NewGoLevelDB("repair-196", dir)
	require.NoError(t, err)
	n := &repair196Node{cms: store.NewCommitMultiStore(db), key: sdk.NewKVStoreKey(types.StoreKey)}
	t.Cleanup(func() { n.close(t) })
	n.cms.MountStoreWithDB(n.key, sdk.StoreTypeIAVL, db)
	require.NoError(t, n.cms.LoadLatestVersion())
	n.ctx = sdk.NewContext(n.cms, tmproto.Header{ChainID: "hyperpax_125-1", Height: 12}, false)
	app := testkeeper.EVMTestApp
	n.k, err = keeper.NewKeeper(app.AppCodec(), n.key, app.BankKeeper, app.EvmKeeper.GetBaseDenom)
	require.NoError(t, err)
	return n
}

func (n *repair196Node) close(t *testing.T) {
	t.Helper()
	if !n.closed {
		require.NoError(t, n.cms.Close())
		n.closed = true
	}
}

func repair196Registry(t *testing.T) types.GenesisState {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(7)
	k := app.LayerXChainsKeeper
	require.Zero(t, k.DomainCount(ctx))
	p := k.GetParams(ctx)
	value := p.MinBond.Add(p.CreationFee)
	coins := sdk.NewCoins(sdk.NewCoin(app.EvmKeeper.GetBaseDenom(ctx), value.MulRaw(2)))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, coins))
	account := app.EvmKeeper.GetPaxAddressOrDefault(ctx, operator)
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, account, coins))
	for i, name := range []string{"repair-first", "repair-second"} {
		_, err := k.Create(ctx, operator, name, uint64(i+1)*500000, 65536, 100, value, sdk.NewDec(int64(i+1)*100))
		require.NoError(t, err)
	}
	return *k.ExportGenesis(ctx)
}

func repair196History(t *testing.T, n *repair196Node, registry types.GenesisState) []keeper.DomainHistory {
	t.Helper()
	rows := []keeper.DomainHistory{
		{DomainID: registry.Domains[0].Id, Height: 7, TxHashes: []common.Hash{common.HexToHash("0x010701"), common.HexToHash("0x010702")}, GasUsed: 500000},
		{DomainID: registry.Domains[0].Id, Height: 8, TxHashes: []common.Hash{common.HexToHash("0x010801"), common.HexToHash("0x010802")}, GasUsed: 250000},
		{DomainID: registry.Domains[0].Id, Height: 11, TxHashes: []common.Hash{common.HexToHash("0x010b01"), common.HexToHash("0x010b02")}, GasUsed: 0},
		{DomainID: registry.Domains[1].Id, Height: 7, TxHashes: []common.Hash{common.HexToHash("0x020701"), common.HexToHash("0x020702")}, GasUsed: 0},
		{DomainID: registry.Domains[1].Id, Height: 8, TxHashes: []common.Hash{common.HexToHash("0x020801"), common.HexToHash("0x020802")}, GasUsed: 500000},
		{DomainID: registry.Domains[1].Id, Height: 11, TxHashes: []common.Hash{common.HexToHash("0x020b01"), common.HexToHash("0x020b02")}, GasUsed: 1000000},
	}
	// Record in reverse key order; exports must still be canonical and preserve
	// the order of transactions within each domain's block.
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		n.k.RecordDomainTx(n.ctx, row.DomainID, row.Height, row.TxHashes[0], row.GasUsed/3)
		n.k.RecordDomainTx(n.ctx, row.DomainID, row.Height, row.TxHashes[1], row.GasUsed-row.GasUsed/3)
	}
	for _, row := range rows {
		n.k.AdjustDomainBaseFee(n.ctx, row.DomainID, n.k.DomainGasUsed(n.ctx, row.DomainID, row.Height), sdk.NewDec(10), sdk.NewDec(1000), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.2"))
	}
	require.True(t, sdk.NewDec(88).Equal(n.k.NextBaseFee(n.ctx, registry.Domains[0].Id)))
	require.True(t, sdk.NewDec(176).Equal(n.k.NextBaseFee(n.ctx, registry.Domains[1].Id)))
	return rows
}

func repair196AssertHistory(t *testing.T, n *repair196Node, rows []keeper.DomainHistory) {
	t.Helper()
	for _, row := range rows {
		require.Equal(t, row.TxHashes, n.k.DomainTxs(n.ctx, row.DomainID, row.Height))
		require.Equal(t, row.GasUsed, n.k.DomainGasUsed(n.ctx, row.DomainID, row.Height))
	}
	for _, id := range []uint32{rows[0].DomainID, rows[len(rows)-1].DomainID} {
		require.Empty(t, n.k.DomainTxs(n.ctx, id, 9))
		require.Zero(t, n.k.DomainGasUsed(n.ctx, id, 9))
	}
}

func TestRepair_196_ChainHistoryRestartRecovery(t *testing.T) {
	registry := repair196Registry(t)
	sourceDir, targetDir := t.TempDir(), t.TempDir()
	source := repair196Open(t, sourceDir)
	require.NoError(t, source.k.InitGenesis(source.ctx, registry))
	rows := repair196History(t, source, registry)
	// Paused domains retain committed history even though RecordDomainTx
	// refuses new transactions for them.
	require.NoError(t, source.k.Pause(source.ctx, operator, registry.Domains[1].Id))
	before := snapshot(source.ctx, source.key)
	raw, err := json.Marshal(source.k.ExportGenesisWithHistory(source.ctx))
	require.NoError(t, err)
	source.cms.Commit(true)
	source.close(t)
	source = repair196Open(t, sourceDir)
	repair196AssertHistory(t, source, rows)
	reopenedRaw, err := json.Marshal(source.k.ExportGenesisWithHistory(source.ctx))
	require.NoError(t, err)
	require.Equal(t, raw, reopenedRaw)
	require.Equal(t, before, snapshot(source.ctx, source.key))
	var decoded keeper.GenesisState
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NoError(t, decoded.Validate())
	require.Equal(t, rows, decoded.DomainHistory)
	target := repair196Open(t, targetDir)
	require.NoError(t, target.k.InitGenesisWithHistory(target.ctx, decoded))
	require.Equal(t, before, snapshot(target.ctx, target.key))
	target.cms.Commit(true)
	target.close(t)
	target = repair196Open(t, targetDir)
	repair196AssertHistory(t, target, rows)
	importedRaw, err := json.Marshal(target.k.ExportGenesisWithHistory(target.ctx))
	require.NoError(t, err)
	require.Equal(t, raw, importedRaw)
	for _, row := range rows {
		a := source.k.AdjustDomainBaseFee(source.ctx, row.DomainID, source.k.DomainGasUsed(source.ctx, row.DomainID, row.Height), sdk.NewDec(10), sdk.NewDec(1000), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.2"))
		b := target.k.AdjustDomainBaseFee(target.ctx, row.DomainID, target.k.DomainGasUsed(target.ctx, row.DomainID, row.Height), sdk.NewDec(10), sdk.NewDec(1000), sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.2"))
		require.True(t, a.Equal(b))
	}
	row := rows[0]
	nextHash := common.HexToHash("0x010703")
	target.k.RecordDomainTx(target.ctx, row.DomainID, row.Height, nextHash, 17)
	rows[0].TxHashes = append(rows[0].TxHashes, nextHash)
	rows[0].GasUsed += 17
	target.cms.Commit(true)
	target.close(t)
	target = repair196Open(t, targetDir)
	repair196AssertHistory(t, target, rows)
}

func TestRepair_196_ValidatesHistoryBeforeImport(t *testing.T) {
	registry := repair196Registry(t)
	source := repair196Open(t, t.TempDir())
	require.NoError(t, source.k.InitGenesis(source.ctx, registry))
	repair196History(t, source, registry)
	raw, err := json.Marshal(source.k.ExportGenesisWithHistory(source.ctx))
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		mutate func(*keeper.GenesisState)
	}{
		{"unknown-domain", func(gs *keeper.GenesisState) { gs.DomainHistory[0].DomainID = 999 }},
		{"root-domain", func(gs *keeper.GenesisState) { gs.DomainHistory[0].DomainID = 0 }},
		{"duplicate-height", func(gs *keeper.GenesisState) { gs.DomainHistory = append(gs.DomainHistory, gs.DomainHistory[0]) }},
		{"empty-membership", func(gs *keeper.GenesisState) { gs.DomainHistory[0].TxHashes = nil }},
		{"missing-fee", func(gs *keeper.GenesisState) { delete(gs.NextBaseFees, gs.Domains[0].Id) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gs keeper.GenesisState
			require.NoError(t, json.Unmarshal(raw, &gs))
			tc.mutate(&gs)
			target := repair196Open(t, t.TempDir())
			before := snapshot(target.ctx, target.key)
			require.ErrorIs(t, gs.Validate(), types.ErrInvalidGenesis)
			require.ErrorIs(t, target.k.InitGenesisWithHistory(target.ctx, gs), types.ErrInvalidGenesis)
			require.Equal(t, before, snapshot(target.ctx, target.key))
		})
	}
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	fields["domain_history"] = json.RawMessage(`[{"domain_id":1,"height":7,"tx_hashes":["0x01"],"gas_used":1}]`)
	malformed, err := json.Marshal(fields)
	require.NoError(t, err)
	var decoded keeper.GenesisState
	require.Error(t, json.Unmarshal(malformed, &decoded))
	// Both scalar extremes are accepted by the real producer and must survive
	// genesis; zero gas is also valid for indexed transactions.
	source.k.RecordDomainTx(source.ctx, registry.Domains[0].Id, 0, common.Hash{}, 0)
	source.k.RecordDomainTx(source.ctx, registry.Domains[0].Id, math.MaxUint64, common.HexToHash("0xffff"), math.MaxUint64)
	extremes := source.k.ExportGenesisWithHistory(source.ctx)
	target := repair196Open(t, t.TempDir())
	require.NoError(t, target.k.InitGenesisWithHistory(target.ctx, *extremes))
	require.Equal(t, snapshot(source.ctx, source.key), snapshot(target.ctx, target.key))
	before := snapshot(target.ctx, target.key)
	require.ErrorIs(t, target.k.InitGenesisWithHistory(target.ctx, *extremes), types.ErrInvalidGenesis)
	require.Equal(t, before, snapshot(target.ctx, target.key))
}

func TestRepair_196_RefusesLossyLegacyExport(t *testing.T) {
	registry := repair196Registry(t)
	n := repair196Open(t, t.TempDir())
	require.NoError(t, n.k.InitGenesis(n.ctx, registry))
	module := chains.NewAppModule(n.k)
	legacy := module.ExportGenesis(n.ctx, nil)
	var gs types.GenesisState
	require.NoError(t, json.Unmarshal(legacy, &gs))
	require.Equal(t, registry, gs)
	n.k.RecordDomainTx(n.ctx, registry.Domains[0].Id, 7, common.HexToHash("0x196"), 196)
	before := snapshot(n.ctx, n.key)
	require.PanicsWithValue(t, "layerxchains: domain history requires ExportGenesisWithHistory and InitGenesisWithHistory", func() {
		module.ExportGenesis(n.ctx, nil)
	})
	require.Equal(t, before, snapshot(n.ctx, n.key))
	require.Len(t, n.k.ExportGenesisWithHistory(n.ctx).DomainHistory, 1)
}

func TestRepair_196_RefusesCorruptStoredHistory(t *testing.T) {
	registry := repair196Registry(t)
	for _, name := range []string{"missing-gas", "orphan-gas", "short-gas", "short-hash", "short-tx-key", "short-gas-key", "unknown-domain", "preexisting-history"} {
		t.Run(name, func(t *testing.T) {
			n := repair196Open(t, t.TempDir())
			require.NoError(t, n.k.InitGenesis(n.ctx, registry))
			id := registry.Domains[0].Id
			n.k.RecordDomainTx(n.ctx, id, 7, common.HexToHash("0x196"), 196)
			store := n.ctx.KVStore(n.key)
			txKey := types.HeightKey(types.DomainTxPrefix, id, 7)
			gasKey := types.HeightKey(types.DomainGasUsedPrefix, id, 7)
			switch name {
			case "missing-gas":
				store.Delete(gasKey)
			case "orphan-gas":
				store.Delete(txKey)
			case "short-gas":
				store.Set(gasKey, []byte{1})
			case "short-hash":
				store.Set(txKey, []byte{1})
			case "short-tx-key":
				store.Set(append([]byte{}, types.DomainTxPrefix...), []byte{1})
			case "short-gas-key":
				store.Set(append([]byte{}, types.DomainGasUsedPrefix...), []byte{1})
			case "unknown-domain":
				store.Set(types.HeightKey(types.DomainTxPrefix, 999, 7), store.Get(txKey))
				store.Set(types.HeightKey(types.DomainGasUsedPrefix, 999, 7), store.Get(gasKey))
			case "preexisting-history":
				for _, d := range registry.Domains {
					store.Delete(types.DomainKey(d.Id))
				}
				before := snapshot(n.ctx, n.key)
				require.ErrorIs(t, n.k.InitGenesisWithHistory(n.ctx, keeper.GenesisState{GenesisState: registry}), types.ErrInvalidGenesis)
				require.Equal(t, before, snapshot(n.ctx, n.key))
				return
			}
			before := snapshot(n.ctx, n.key)
			require.Panics(t, func() { n.k.ExportGenesisWithHistory(n.ctx) })
			require.Equal(t, before, snapshot(n.ctx, n.key))
		})
	}
}
