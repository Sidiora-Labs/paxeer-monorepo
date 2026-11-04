package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	noderuntime "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/node/runtime"
	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
)

func TestRepairM_1190_NodeDelayConfig(t *testing.T) {
	// The runner is a command, so compile its unchanged setup.go as a test-only
	// source fixture. Check its identity before exercising the real producer;
	// no copied serialization logic or subprocess toolchain is used.
	producer, err := os.ReadFile(filepath.Join("..", "runner", "setup.go"))
	require.NoError(t, err)
	compiledProducer, err := os.ReadFile("repair_m1190_runner_test.go")
	require.NoError(t, err)
	require.Equal(t, string(producer), string(compiledProducer), "runner source fixture must match the production producer exactly")
	network, err := e2e.LoadTestnet(filepath.Join("testdata", "repair_m1190_manifest.toml"))
	require.NoError(t, err)
	generated, err := MakeAppConfig(network.Nodes[0])
	require.NoError(t, err)
	file := filepath.Join(t.TempDir(), "app.toml")
	require.NoError(t, os.WriteFile(file, generated, 0600))
	cfg, err := LoadConfig(file)
	require.NoError(t, err)
	require.Equal(t, network.Name, cfg.ChainID)
	require.Equal(t, string(network.ABCIProtocol), cfg.Protocol)
	applicationConfig := cfg.App()

	for _, tc := range []struct {
		name        string
		want        int
		node        uint64
		application uint64
	}{
		{"prepare_proposal", network.PrepareProposalDelayMS, cfg.PrepareProposalDelayMS, applicationConfig.PrepareProposalDelayMS},
		{"process_proposal", network.ProcessProposalDelayMS, cfg.ProcessProposalDelayMS, applicationConfig.ProcessProposalDelayMS},
		{"check_tx", network.CheckTxDelayMS, cfg.CheckTxDelayMS, applicationConfig.CheckTxDelayMS},
		{"finalize_block", network.FinalizeBlockDelayMS, cfg.FinalizeBlockDelayMS, applicationConfig.FinalizeBlockDelayMS},
	} {
		t.Run(tc.name+"_forwarded", func(t *testing.T) {
			require.Positive(t, tc.want)
			require.Equal(t, uint64(tc.want), tc.node)
			require.Equal(t, uint64(tc.want), tc.application)
		})
	}

	applicationConfig.Dir = t.TempDir()
	application, err := app.NewApplication(applicationConfig)
	require.NoError(t, err)
	_, err = application.InitChain(t.Context(), &abci.RequestInitChain{InitialHeight: 1})
	require.NoError(t, err)
	tx := []byte("delay=observed")

	// PrepareProposal is absent from the current ABCI Application interface;
	// its retained configuration field is checked above (see the wiring record).
	t.Run("process_proposal_elapsed", func(t *testing.T) {
		repairM1190MeasureDelay(t, time.Duration(network.ProcessProposalDelayMS)*time.Millisecond, func() error {
			response, err := application.ProcessProposal(t.Context(), &abci.RequestProcessProposal{Txs: [][]byte{tx}})
			if err != nil {
				return err
			}
			if response == nil || response.Status != abci.ResponseProcessProposal_ACCEPT {
				return fmt.Errorf("valid proposal was not accepted: %v", response)
			}
			return nil
		})
	})
	t.Run("check_tx_elapsed", func(t *testing.T) {
		repairM1190MeasureDelay(t, time.Duration(network.CheckTxDelayMS)*time.Millisecond, func() error {
			response := application.CheckTx(t.Context(), &abci.RequestCheckTxV2{Tx: tx})
			if response == nil || response.ResponseCheckTx == nil || response.Code != abci.CodeTypeOK || response.GasWanted != 1 {
				return fmt.Errorf("valid transaction was not accepted: %v", response)
			}
			return nil
		})
	})
	t.Run("finalize_block_elapsed", func(t *testing.T) {
		repairM1190MeasureDelay(t, time.Duration(network.FinalizeBlockDelayMS)*time.Millisecond, func() error {
			response, err := application.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
				Header: &tmproto.Header{Height: 1}, Txs: [][]byte{tx},
			})
			if err != nil {
				return err
			}
			if response == nil || len(response.TxResults) != 1 || response.TxResults[0].Code != abci.CodeTypeOK || len(response.AppHash) == 0 {
				return fmt.Errorf("block did not execute the transaction: %v", response)
			}
			return nil
		})
	})
	_, err = application.Commit(t.Context())
	require.NoError(t, err)
	response, err := application.Query(t.Context(), &abci.RequestQuery{Data: []byte("delay")})
	require.NoError(t, err)
	require.Equal(t, []byte("observed"), response.Value)
	require.Equal(t, int64(1), response.Height)
}

func repairM1190MeasureDelay(t *testing.T, delay time.Duration, call func() error) {
	t.Helper()
	type result struct {
		elapsed time.Duration
		err     error
	}
	done := make(chan result, 1)
	go func() {
		start := time.Now()
		err := call()
		done <- result{time.Since(start), err}
	}()
	const bound = 3 * time.Second
	timer := time.NewTimer(bound)
	defer timer.Stop()
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.GreaterOrEqual(t, result.elapsed, delay, "configured application delay was lost")
		require.Less(t, result.elapsed, bound, "application exceeded the bounded callback time")
	case <-timer.C:
		t.Fatalf("application callback did not finish within %s", bound)
	}
}

func TestRepairM_1190_SocketExample(t *testing.T) {
	examples, err := filepath.Glob("*.toml")
	require.NoError(t, err)
	require.Contains(t, examples, "socket.toml")
	require.Contains(t, examples, "built-in.toml")
	for _, file := range examples {
		t.Run(file, func(t *testing.T) {
			cfg, err := LoadConfig(file)
			require.NoError(t, err)
			require.Equal(t, "builtin", cfg.Protocol)
			require.NoError(t, cfg.Validate())
			applicationConfig := cfg.App()
			require.Zero(t, applicationConfig.PrepareProposalDelayMS)
			require.Zero(t, applicationConfig.ProcessProposalDelayMS)
			require.Zero(t, applicationConfig.CheckTxDelayMS)
			require.Zero(t, applicationConfig.FinalizeBlockDelayMS)
			applicationConfig.Dir = t.TempDir()
			application, err := app.NewApplication(applicationConfig)
			require.NoError(t, err)
			_, err = application.InitChain(t.Context(), &abci.RequestInitChain{InitialHeight: 1})
			require.NoError(t, err)
			response := application.CheckTx(t.Context(), &abci.RequestCheckTxV2{Tx: []byte("example=builtin")})
			require.Equal(t, abci.CodeTypeOK, response.Code)
		})
	}
	for _, protocol := range []string{"socket", "tcp", "unix", "grpc", "file", "unknown", ""} {
		t.Run("refuse_"+protocol, func(t *testing.T) {
			cfg, err := LoadConfig("socket.toml")
			require.NoError(t, err)
			cfg.Protocol = protocol
			require.Error(t, cfg.Validate())
			var encoded bytes.Buffer
			require.NoError(t, toml.NewEncoder(&encoded).Encode(cfg))
			file := filepath.Join(t.TempDir(), "unsupported.toml")
			require.NoError(t, os.WriteFile(file, encoded.Bytes(), 0600))
			loaded, err := LoadConfig(file)
			require.Error(t, err)
			require.NotNil(t, loaded)
			require.Equal(t, protocol, loaded.Protocol)
			require.Error(t, noderuntime.Run(t.Context(), file, t.TempDir()))
		})
	}
}
