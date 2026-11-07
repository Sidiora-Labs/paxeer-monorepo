package producer

import (
	"context"
	"fmt"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/data"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
)

func repairM1081Execute(ctx context.Context, env *testEnv, block *types.GlobalBlock) error {
	h := block.Header.Hash()
	resp, err := env.state.cfg.App.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Txs: block.Payload.Txs(), Hash: h[:]})
	if err != nil {
		return err
	}
	return env.data.PushAppHash(ctx, block.GlobalNumber, resp.AppHash)
}

func TestRepairM_1081_EstimatedGasSealing(t *testing.T) {
	for _, tc := range []struct {
		name      string
		estimate  uint64
		effective uint64
		perBlock  int
	}{
		{"estimate", 42_000, 42_000, 3},
		{"below_intrinsic", 20_999, 63_000, 2},
		{"above_wanted", 63_001, 63_000, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rng := utils.TestRng()
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			app := newTestApp()
			cfg := app.Cfg()
			cfg.MaxTxsPerBlock = 20
			cfg.MaxGasWantedPerBlock = 1_000_000
			cfg.MaxGasEstimatedPerBlock = 126_000
			env := newTestEnv(rng, cfg)
			addr, nonce := app.NewAccount(rng)
			txs := make([]*txSpec, 2*tc.perBlock+1)
			for i := range txs {
				txs[i] = env.genTx(rng, addr, nonce+uint64(i))
				txs[i].GasWanted = 63_000
				txs[i].GasEstimated = tc.estimate
			}
			// The shared oracle must count normalized estimates, even when
			// count, bytes and wanted gas would permit another transaction.
			stats := blockStats{}
			for _, tx := range txs[:tc.perBlock] {
				require.True(t, stats.Push(tx, cfg))
			}
			require.Equal(t, uint64(tc.perBlock)*tc.effective, stats.gasEstimated)
			require.False(t, stats.Push(txs[tc.perBlock], cfg))
			require.Less(t, stats.count, cfg.MaxTxsPerBlock)
			require.Less(t, stats.sizeBytes, types.MaxTxsBytesPerBlock)
			require.Less(t, stats.gasWanted, cfg.MaxGasWantedPerBlock)

			require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
				s.SpawnBgNamed("data", func() error { return utils.IgnoreCancel(env.data.Run(ctx)) })
				s.SpawnBgNamed("consensus", func() error { return utils.IgnoreCancel(env.consensus.Run(ctx)) })
				s.SpawnBgNamed("network", func() error {
					return utils.IgnoreCancel(consensus.RunTestNetwork(ctx, []*consensus.State{env.consensus}))
				})
				s.SpawnBgNamed("producer", func() error { return utils.IgnoreCancel(env.state.Run(ctx)) })
				inserted := 0
				for blockIndex := range 2 {
					end := (blockIndex + 1) * tc.perBlock
					for inserted < end {
						resp, err := env.state.InsertTx(ctx, txs[inserted].encode())
						if err != nil {
							return err
						}
						require.True(t, resp.IsOK())
						inserted++
					}
					require.Len(t, env.state.UnconfirmedTxs(), tc.perBlock)
					require.Equal(t, types.BlockNumber(blockIndex), env.consensus.Avail().NextBlock(env.consensus.Avail().PublicKey()), "block must remain open until the next transaction")
					resp, err := env.state.InsertTx(ctx, txs[inserted].encode())
					if err != nil {
						return err
					}
					require.True(t, resp.IsOK())
					inserted++
					block, err := env.data.GlobalBlock(ctx, env.data.Committee().FirstBlock()+types.GlobalBlockNumber(blockIndex))
					if err != nil {
						return err
					}
					require.Len(t, block.Payload.Txs(), tc.perBlock)
					stats := blockStats{}
					for j, raw := range block.Payload.Txs() {
						tx := txs[blockIndex*tc.perBlock+j]
						require.Equal(t, tx.encode(), raw)
						require.True(t, stats.Push(tx, cfg), "every emitted transaction must fit")
					}
					require.Equal(t, uint64(tc.perBlock)*tc.effective, block.Payload.TotalGasEstimated())
					require.Equal(t, stats.gasEstimated, block.Payload.TotalGasEstimated())
					require.Equal(t, stats.gasWanted, block.Payload.TotalGasWanted())
					require.LessOrEqual(t, stats.gasEstimated, cfg.MaxGasEstimatedPerBlock)
					require.False(t, stats.Push(txs[end], cfg), "the following transaction must force sealing")
					if err := repairM1081Execute(ctx, env, block); err != nil {
						return err
					}
				}
				return nil
			}))
			require.Equal(t, nonce+uint64(2*tc.perBlock), app.EvmNonce(addr))
		})
	}
}

func TestRepairM_1081_PendingIndexRestart(t *testing.T) {
	rng := utils.TestRng()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	app := newTestApp()
	cfg := app.Cfg()
	cfg.MaxTxsPerBlock = 1
	cfg.BlockInterval = 0
	committee, keys := types.GenCommittee(rng, 1)
	dir := t.TempDir()
	open := func() *testEnv {
		wal, err := data.NewDataWAL(utils.None[string](), committee)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, wal.Close()) })
		ds, err := data.NewState(&data.Config{Committee: committee}, wal)
		require.NoError(t, err)
		cs, err := consensus.NewState(&consensus.Config{
			Key: keys[0], ViewTimeout: func(types.View) time.Duration { return time.Hour },
			PersistentStateDir: utils.Some(dir),
		}, ds)
		require.NoError(t, err)
		return &testEnv{state: NewState(cfg, cs), consensus: cs, data: ds}
	}
	env := open()
	addr, nonce := app.NewAccount(rng)
	txs := []*txSpec{env.genTx(rng, addr, nonce), env.genTx(rng, addr, nonce+1), env.genTx(rng, addr, nonce+2)}
	for _, tx := range txs[:2] {
		resp, err := env.state.InsertTx(ctx, tx.encode())
		require.NoError(t, err)
		require.True(t, resp.IsOK())
	}
	pendingBefore := env.state.EvmNextPendingNonce(addr)
	require.Equal(t, nonce+2, pendingBefore)
	var persisted []*types.Signed[*types.LaneProposal]
	// No consensus network or executor runs in this phase: the real producer
	// emits and persists two local blocks, which remain unexecuted at shutdown.
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("availability", func() error { return utils.IgnoreCancel(env.consensus.Avail().Run(ctx)) })
		s.SpawnBgNamed("producer", func() error { return utils.IgnoreCancel(env.state.Run(ctx)) })
		votes := env.consensus.Avail().SubscribeLaneVotes()
		for len(persisted) < 2 {
			batch, err := votes.RecvBatch(ctx)
			if err != nil {
				return err
			}
			for _, vote := range batch {
				h := vote.Msg().Header()
				p, err := env.consensus.Avail().Block(ctx, h.Lane(), h.BlockNumber())
				if err != nil {
					return err
				}
				persisted = append(persisted, p)
			}
		}
		return nil
	}))
	require.Equal(t, nonce, app.EvmNonce(addr))
	require.Len(t, persisted, 2)
	for i, p := range persisted {
		require.Equal(t, [][]byte{txs[i].encode()}, p.Msg().Block().Payload().Txs())
	}

	env = open()
	require.Equal(t, types.BlockNumber(2), env.consensus.Avail().NextBlock(keys[0].Public()))
	// Admission before replay is cancellable and cannot use the committed nonce.
	cancelled, stop := context.WithCancel(ctx)
	stop()
	_, err := env.state.TryInsertTx(cancelled, txs[0].encode())
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("data", func() error { return utils.IgnoreCancel(env.data.Run(ctx)) })
		s.SpawnBgNamed("consensus", func() error { return utils.IgnoreCancel(env.consensus.Run(ctx)) })
		s.SpawnBgNamed("producer", func() error { return utils.IgnoreCancel(env.state.Run(ctx)) })
		if err := env.state.waitForReplay(ctx); err != nil {
			return err
		}
		require.Equal(t, pendingBefore, env.state.EvmNextPendingNonce(addr))
		for i, tx := range txs[:2] {
			got, ok := env.state.EvmTxByHash(tx.EVMHash)
			require.True(t, ok)
			require.Equal(t, tx.encode(), []byte(got))
			p, err := env.consensus.Avail().Block(ctx, keys[0].Public(), types.BlockNumber(i))
			if err != nil {
				return err
			}
			require.Equal(t, persisted[i].Msg().Block().Header().Hash(), p.Msg().Block().Header().Hash())
		}
		_, err := env.state.TryInsertTx(ctx, txs[0].encode())
		require.ErrorIs(t, err, errBadNonce)
		resp, err := env.state.TryInsertTx(ctx, txs[2].encode())
		if err != nil {
			return fmt.Errorf("following nonce after restart: %w", err)
		}
		require.True(t, resp.IsOK())
		require.Equal(t, nonce+3, env.state.EvmNextPendingNonce(addr))
		s.SpawnBgNamed("network", func() error {
			return utils.IgnoreCancel(consensus.RunTestNetwork(ctx, []*consensus.State{env.consensus}))
		})
		for i, tx := range txs {
			block, err := env.data.GlobalBlock(ctx, committee.FirstBlock()+types.GlobalBlockNumber(i))
			if err != nil {
				return err
			}
			require.Equal(t, types.BlockNumber(i), block.Header.BlockNumber(), "recovered blocks must not be produced again")
			require.Equal(t, [][]byte{tx.encode()}, block.Payload.Txs())
			if err := repairM1081Execute(ctx, env, block); err != nil {
				return err
			}
		}
		// Pruning must also own the recovered blocks, so hashes disappear only
		// after execution and the application supplies the final nonce.
		for m, ctrl := range env.state.mempool.Lock() {
			if err := ctrl.WaitUntil(ctx, func() bool { return len(m.evmTxs) == 0 }); err != nil {
				return err
			}
		}
		for _, tx := range txs {
			_, ok := env.state.EvmTxByHash(tx.EVMHash)
			require.False(t, ok)
		}
		require.Equal(t, nonce+3, env.state.EvmNextPendingNonce(addr))
		return nil
	}))
	require.Equal(t, nonce+3, app.EvmNonce(addr))
}

func TestRepairM_1081_EstimatedGasOracle(t *testing.T) {
	cfg := newTestApp().Cfg()
	cfg.MaxGasEstimatedPerBlock = 63_000
	stats := blockStats{}
	for _, estimate := range []uint64{21_000, 0, 42_001} {
		tx := &txSpec{GasWanted: 42_000, GasEstimated: estimate}
		fits := stats.Push(tx, cfg)
		if stats.count == 1 {
			require.True(t, fits)
			require.Equal(t, uint64(21_000), stats.gasEstimated)
		} else if stats.count == 2 {
			require.True(t, fits)
			require.Equal(t, uint64(63_000), stats.gasEstimated)
		} else {
			require.False(t, fits)
			require.Equal(t, uint64(105_000), stats.gasEstimated)
		}
	}
}
