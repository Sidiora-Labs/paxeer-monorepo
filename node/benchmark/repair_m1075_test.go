package benchmark

import (
	"context"
	"math/big"
	"runtime"
	"strings"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	evmruntime "github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/stretchr/testify/require"
)

func repairM1075TxConfig() client.TxConfig {
	registry := codectypes.NewInterfaceRegistry()
	evmtypes.RegisterInterfaces(registry)
	return authtx.NewTxConfig(codec.NewProtoCodec(registry), authtx.DefaultSignModes)
}

func repairM1075Manager(t *testing.T, scenarioConfigs ...config.Scenario) (*Manager, context.CancelFunc) {
	t.Helper()
	cfg := &config.LoadConfig{
		ChainID: 98765, PaxChainID: "repair-m1075", MockDeploy: true,
		Scenarios: scenarioConfigs,
	}
	gen, err := NewGenerator(cfg, repairM1075TxConfig())
	require.NoError(t, err)
	gen.txsPerBatch = 2
	ctx, cancel := context.WithCancel(context.Background())
	m := startManager(ctx, gen)
	t.Cleanup(func() {
		cancel()
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		for {
			select {
			case _, ok := <-m.ProposalChannel():
				if !ok {
					return
				}
			case <-timer.C:
				t.Fatal("proposal worker did not stop after cancellation")
			}
		}
	})
	return m, cancel
}

func repairM1075Scenario(name string, count int) config.Scenario {
	return config.Scenario{Name: name, Weight: 1, Accounts: &config.AccountConfig{Accounts: count}}
}

func repairM1075Batch(t *testing.T, m *Manager) []*ethtypes.Transaction {
	t.Helper()
	select {
	case batch, ok := <-m.ProposalChannel():
		require.True(t, ok, "proposal channel closed before producing transactions")
		require.NotEmpty(t, batch)
		txs := make([]*ethtypes.Transaction, 0, len(batch))
		for _, wire := range batch {
			decoded, err := m.Generator.txConfig.TxDecoder()(wire)
			require.NoError(t, err)
			require.Len(t, decoded.GetMsgs(), 1)
			msg, ok := decoded.GetMsgs()[0].(*evmtypes.MsgEVMTransaction)
			require.True(t, ok)
			require.NoError(t, msg.ValidateBasic())
			tx, _ := msg.AsTransaction()
			require.NotNil(t, tx)
			require.Equal(t, m.Generator.chainID, tx.ChainId())
			_, err = ethtypes.Sender(ethtypes.NewCancunSigner(tx.ChainId()), tx)
			require.NoError(t, err, "generated transaction signature must survive the codec")
			txs = append(txs, tx)
		}
		return txs
	case <-time.After(time.Second):
		t.Fatal("manager did not produce a proposal after chain progress")
		return nil
	}
}

func repairM1075Idle(t *testing.T, m *Manager) {
	t.Helper()
	// Inspect the running production worker, rather than infer bounded work from
	// elapsed time alone: an empty result must park in a select, not retry Generate.
	require.Eventually(t, func() bool {
		stacks := make([]byte, 1<<20)
		n := runtime.Stack(stacks, true)
		for _, stack := range strings.Split(string(stacks[:n]), "\n\n") {
			if strings.Contains(stack, "(*Generator).StartProposalChannel.func1()") &&
				strings.Contains(strings.SplitN(stack, "\n", 2)[0], "[select]") {
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond, "empty proposal worker must yield awaiting state or cancellation")
	select {
	case batch, ok := <-m.ProposalChannel():
		t.Fatalf("unexpected proposal while waiting: open=%v transactions=%d", ok, len(batch))
	case <-time.After(20 * time.Millisecond):
	}
}

func repairM1075Commit(m *Manager) {
	// This is the same producer called by App.RecordBenchmarkCommitTime after
	// Commit, transported through the manager's real logger to the generator.
	m.Logger.RecordCommitTime(time.Millisecond)
}

func TestRepairM_1075_ScenarioOnlyAccounts(t *testing.T) {
	m, _ := repairM1075Manager(t,
		repairM1075Scenario(scenarios.EVMTransfer, 2),
		repairM1075Scenario(scenarios.EVMTransferNoop, 3))
	g := m.Generator
	require.Nil(t, g.cfg.Accounts)
	require.Nil(t, g.sharedAccounts)
	require.Len(t, g.accountPools, 2)
	addresses := make(map[common.Address]bool)
	for i, count := range []int{2, 3} {
		require.Same(t, g.accountPools[i], g.scenarios[i].accounts)
		for j := 0; j < count; j++ {
			account := g.accountPools[i].NextAccount()
			require.False(t, addresses[account.Address], "scenario pools must contain their own accounts")
			addresses[account.Address] = true
		}
	}
	require.Len(t, addresses, 5)
	for i := 0; i < WarmupBlocks; i++ {
		repairM1075Commit(m)
	}
	for _, tx := range repairM1075Batch(t, m) {
		sender, err := ethtypes.Sender(ethtypes.NewCancunSigner(g.chainID), tx)
		require.NoError(t, err)
		require.True(t, addresses[sender], "load must use the constructed scenario pools")
		require.NotNil(t, tx.To())
	}
	require.Equal(t, PhaseLoad, g.Phase())
}

func TestRepairM_1075_CommittedBlockWarmup(t *testing.T) {
	m, _ := repairM1075Manager(t, repairM1075Scenario(scenarios.EVMTransfer, 2))
	g := m.Generator
	for commits := 0; commits < WarmupBlocks; commits++ {
		for attempt := 0; attempt < 100; attempt++ {
			require.Empty(t, g.Generate(), "requests cannot consume block warmup")
		}
		repairM1075Idle(t, m)
		require.Equal(t, PhaseWarmup, g.Phase())
		g.mu.RLock()
		counter := g.warmupCounter
		g.mu.RUnlock()
		require.Equal(t, commits, counter)
		// Receipts, even repeated empty block results, do not count as commits.
		m.ProcessReceipts(nil)
		require.Equal(t, PhaseWarmup, g.Phase())
		repairM1075Commit(m)
	}
	require.Len(t, repairM1075Batch(t, m), 2)
	require.Equal(t, PhaseLoad, g.Phase())
	g.mu.RLock()
	counter := g.warmupCounter
	g.mu.RUnlock()
	require.Equal(t, WarmupBlocks, counter)
}

func TestRepairM_1075_DeploymentWaitAndResume(t *testing.T) {
	m, _ := repairM1075Manager(t, repairM1075Scenario(scenarios.ERC20, 2))
	for i := 0; i < WarmupBlocks; i++ {
		repairM1075Commit(m)
	}
	deployments := repairM1075Batch(t, m)
	require.Len(t, deployments, 1)
	deployment := deployments[0]
	require.Nil(t, deployment.To())
	require.Equal(t, []common.Hash{deployment.Hash()}, m.GetPendingDeployHashes())
	require.True(t, m.IsSetupPhase())
	repairM1075Idle(t, m)
	for i := 0; i < 10; i++ {
		m.ProcessReceipts(nil)
		repairM1075Commit(m)
	}
	repairM1075Idle(t, m)
	require.Equal(t, []common.Hash{deployment.Hash()}, m.GetPendingDeployHashes())

	// Execute the actual deployment emitted through the proposal codec against
	// the real EVM and its in-memory store; receipt fields come from that result.
	sender, err := ethtypes.Sender(ethtypes.NewCancunSigner(m.Generator.chainID), deployment)
	require.NoError(t, err)
	evmCfg := &evmruntime.Config{Origin: sender, GasLimit: deployment.Gas(), Value: deployment.Value()}
	code, address, gasLeft, err := evmruntime.Create(deployment.Data(), evmCfg)
	require.NoError(t, err)
	require.NotEmpty(t, code)
	require.Equal(t, crypto.CreateAddress(sender, deployment.Nonce()), address)
	require.Equal(t, code, evmCfg.State.GetCode(address))
	receipt := &evmtypes.Receipt{
		TxHashHex: deployment.Hash().Hex(), ContractAddress: address.Hex(),
		Status: uint32(ethtypes.ReceiptStatusSuccessful), GasUsed: deployment.Gas() - gasLeft,
	}
	wire, err := receipt.Marshal()
	require.NoError(t, err)
	var decodedReceipt evmtypes.Receipt
	require.NoError(t, decodedReceipt.Unmarshal(wire))
	m.ProcessReceipts(map[common.Hash]*evmtypes.Receipt{deployment.Hash(): &decodedReceipt})
	require.Empty(t, m.GetPendingDeployHashes())
	require.False(t, m.IsSetupPhase())
	require.Equal(t, PhaseLoad, m.Generator.Phase())
	for _, tx := range repairM1075Batch(t, m) {
		require.NotNil(t, tx.To())
		require.Equal(t, address, *tx.To(), "receipt must attach the real deployment address")
		require.GreaterOrEqual(t, len(tx.Data()), 4)
		require.Equal(t, crypto.Keccak256([]byte("transfer(address,uint256)"))[:4], tx.Data()[:4])
	}
}

func TestRepairM_1075_CancellationWhileWaiting(t *testing.T) {
	for _, phase := range []Phase{PhaseWarmup, PhaseSetup} {
		t.Run([]string{"warmup", "pending-deployment"}[int(phase)], func(t *testing.T) {
			m, cancel := repairM1075Manager(t, repairM1075Scenario(scenarios.ERC20, 2))
			if phase == PhaseSetup {
				for i := 0; i < WarmupBlocks; i++ {
					repairM1075Commit(m)
				}
				require.Len(t, repairM1075Batch(t, m), 1)
			}
			repairM1075Idle(t, m)
			require.Equal(t, phase, m.Generator.Phase())
			cancel()
			select {
			case batch, ok := <-m.ProposalChannel():
				require.False(t, ok, "cancellation must close the waiting worker")
				require.Empty(t, batch)
			case <-time.After(time.Second):
				t.Fatal("waiting worker ignored cancellation")
			}
		})
	}
}

func TestRepairM_1075_SharedAndScenarioPools(t *testing.T) {
	cfg := &config.LoadConfig{
		ChainID: 98765, MockDeploy: true,
		Accounts: &config.AccountConfig{Accounts: 2},
		Scenarios: []config.Scenario{
			{Name: scenarios.EVMTransfer, Weight: 1},
			{Name: scenarios.EVMTransferNoop, Weight: 1},
			repairM1075Scenario(scenarios.EVMTransfer, 3),
		},
	}
	g, err := NewGenerator(cfg, repairM1075TxConfig())
	require.NoError(t, err)
	require.Len(t, g.accountPools, 2, "shared pool must be constructed and counted once")
	require.Same(t, g.sharedAccounts, g.scenarios[0].accounts)
	require.Same(t, g.sharedAccounts, g.scenarios[1].accounts)
	require.NotSame(t, g.sharedAccounts, g.scenarios[2].accounts)
	require.Equal(t, big.NewInt(cfg.ChainID), g.chainID)
}
