package sender

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/types"
	"github.com/stretchr/testify/require"
)

func TestTxsWriter_Flush(t *testing.T) {
	// two evm transfer txs
	writer := NewTxsWriter(42000, t.TempDir(), 1, 1)

	loadConfig := &config.LoadConfig{
		ChainID: 7777,
	}

	sharedAccounts, err := types.NewAccountPool(&types.AccountConfig{
		Accounts:       types.GenerateAccounts(10),
		NewAccountRate: 0.0,
	})

	require.NoError(t, err)
	evmScenario := scenarios.CreateScenario(config.Scenario{
		Name:   "EVMTransfer",
		Weight: 1,
	})
	evmScenario.Deploy(loadConfig, sharedAccounts.NextAccount())

	generator := generator.NewScenarioGenerator(sharedAccounts, evmScenario)

	txs := generator.GenerateN(3)

	err = writer.Send(context.Background(), txs[0])
	require.NoError(t, err)
	require.Equal(t, uint64(1), writer.nextHeight)
	require.Equal(t, uint64(21000), writer.bufferGas)
	require.Len(t, writer.txBuffer, 1)
	require.Equal(t, txs[0], writer.txBuffer[0])

	err = writer.Send(context.Background(), txs[1])
	require.NoError(t, err)
	require.Equal(t, uint64(1), writer.nextHeight)
	require.Equal(t, uint64(42000), writer.bufferGas)
	require.Len(t, writer.txBuffer, 2)
	require.Equal(t, txs[1], writer.txBuffer[1])

	err = writer.Send(context.Background(), txs[2])
	require.ErrorIs(t, err, ErrWriterComplete)
	require.Equal(t, uint64(2), writer.nextHeight)
	require.Zero(t, writer.bufferGas)
	require.Empty(t, writer.txBuffer)
	repair324ReadBatch(t, writer.batchPath(1), txs[0], txs[1])
	require.NoError(t, writer.Flush())
	files, err := filepath.Glob(filepath.Join(writer.txsDir, "*_txs.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Empty(t, writer.txBuffer)
}

func TestTxsWriter_DurablePublication(t *testing.T) {
	TestRepairM_821_WriterDurability(t)
}
