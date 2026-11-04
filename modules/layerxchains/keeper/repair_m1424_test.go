package keeper_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1424_GenesisAccountingRestartRecovery(t *testing.T) {
	// These fixtures use the production bank, EVM address resolver, keeper,
	// codec and IAVL store; closing the node releases its on-disk LevelDB.
	registry := repair196Registry(t)
	sourceDir, targetDir := t.TempDir(), t.TempDir()
	source := repair196Open(t, sourceDir)
	require.NoError(t, source.k.InitGenesis(source.ctx, registry))
	first, second := registry.Domains[0].Id, registry.Domains[1].Id
	rows := []keeper.DomainHistory{
		{DomainID: first, Height: 7, TxHashes: []common.Hash{common.HexToHash("0x14240702"), common.HexToHash("0x14240701")}, GasUsed: 21017},
		{DomainID: first, Height: 11, TxHashes: []common.Hash{common.HexToHash("0x14241102"), common.HexToHash("0x14241101")}, GasUsed: 43029},
		{DomainID: first, Height: 20, TxHashes: []common.Hash{common.HexToHash("0x14242002"), common.HexToHash("0x14242001")}, GasUsed: 0},
		{DomainID: second, Height: 7, TxHashes: []common.Hash{common.HexToHash("0x14240702"), common.HexToHash("0x24240701")}, GasUsed: 65031},
		{DomainID: second, Height: 11, TxHashes: []common.Hash{common.HexToHash("0x24241102"), common.HexToHash("0x24241101")}, GasUsed: 87043},
		{DomainID: second, Height: 20, TxHashes: []common.Hash{common.HexToHash("0x24242002"), common.HexToHash("0x24242001")}, GasUsed: 109057},
	}
	// Interleave domains and heights, and deliberately avoid hash sort order.
	for _, i := range []int{5, 0, 4, 2, 3, 1} {
		row := rows[i]
		ctx := source.ctx.WithBlockHeight(int64(row.Height))
		source.k.RecordDomainTx(ctx, row.DomainID, row.Height, row.TxHashes[0], row.GasUsed/2)
		source.k.RecordDomainTx(ctx, row.DomainID, row.Height, row.TxHashes[1], row.GasUsed-row.GasUsed/2)
	}
	source.k.SetNextBaseFee(source.ctx, first, sdk.MustNewDecFromStr("123.456"))
	source.k.SetNextBaseFee(source.ctx, second, sdk.MustNewDecFromStr("987.654"))
	require.NoError(t, source.k.SetOperator(source.ctx, operator, first, other))
	require.NoError(t, source.k.Pause(source.ctx, operator, second))
	assertHistory := func(n *repair196Node) {
		t.Helper()
		for _, row := range rows {
			require.Equal(t, row.TxHashes, n.k.DomainTxs(n.ctx, row.DomainID, row.Height), "domain %d height %d membership", row.DomainID, row.Height)
			require.Equal(t, row.GasUsed, n.k.DomainGasUsed(n.ctx, row.DomainID, row.Height), "domain %d height %d gas", row.DomainID, row.Height)
		}
		for _, id := range []uint32{first, second} {
			for _, height := range []uint64{0, 6, 8, 12, 19, 21} {
				require.Empty(t, n.k.DomainTxs(n.ctx, id, height))
				require.Zero(t, n.k.DomainGasUsed(n.ctx, id, height))
			}
		}
	}
	assertHistory(source)
	source.cms.Commit(true)
	before := snapshot(source.ctx, source.key)
	gs := source.k.ExportGenesisWithHistory(source.ctx)
	require.NoError(t, gs.Validate())
	require.Equal(t, rows, gs.DomainHistory)
	raw, err := json.Marshal(gs)
	require.NoError(t, err)
	source.close(t)
	source = repair196Open(t, sourceDir)
	assertHistory(source)
	require.Equal(t, before, snapshot(source.ctx, source.key))
	require.Equal(t, gs, source.k.ExportGenesisWithHistory(source.ctx))

	var decoded keeper.GenesisState
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NoError(t, decoded.Validate())
	require.Equal(t, *gs, decoded)
	target := repair196Open(t, targetDir)
	require.Empty(t, snapshot(target.ctx, target.key))
	require.NoError(t, target.k.InitGenesisWithHistory(target.ctx, decoded))
	target.cms.Commit(true)
	target.close(t)
	target = repair196Open(t, targetDir)
	assertHistory(target)
	require.Equal(t, before, snapshot(target.ctx, target.key))
	require.Equal(t, gs.Params, target.k.GetParams(target.ctx))
	require.Equal(t, gs.NextID, target.k.NextID(target.ctx))
	require.Equal(t, uint32(len(gs.Domains)), target.k.DomainCount(target.ctx))
	for _, want := range gs.Domains {
		got, ok := target.k.GetDomain(target.ctx, want.Id)
		require.True(t, ok)
		require.Equal(t, want, got)
		byName, ok := target.k.GetDomainByName(target.ctx, want.Name)
		require.True(t, ok)
		require.Equal(t, want, byName)
		require.True(t, gs.NextBaseFees[want.Id].Equal(target.k.NextBaseFee(target.ctx, want.Id)))
	}
	imported, err := json.Marshal(target.k.ExportGenesisWithHistory(target.ctx))
	require.NoError(t, err)
	require.Equal(t, raw, imported)

	// Resume accounting on the recovered active domain, retaining earlier
	// membership and adding gas to the recovered total, then reopen again.
	nextHash := common.HexToHash("0x14240703")
	target.k.RecordDomainTx(target.ctx.WithBlockHeight(7), first, 7, nextHash, 101)
	rows[0].TxHashes = append(rows[0].TxHashes, nextHash)
	rows[0].GasUsed += 101
	target.cms.Commit(true)
	target.close(t)
	target = repair196Open(t, targetDir)
	assertHistory(target)
}
