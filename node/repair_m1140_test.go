package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/commitment"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_1140_EmptyBlock(t *testing.T)                 { TestEmptyBlockIdempotency(t) }
func TestRepairM_1140_UpgradePanicBoundary(t *testing.T)       { TestProcessBlockUpgradePanicLogic(t) }
func TestRepairM_1140_BenchmarkOutputAndShutdown(t *testing.T) { app.RepairM1140Benchmark(t) }
func TestRepairM_1140_GigaActivationSets(t *testing.T)         { app.RepairM1140GigaSets(t) }
func TestRepairM_1140_GigaOrdinaryParity(t *testing.T) {
	TestGigaMergeAddressFallsBackToOrdinaryPath(t)
}
func TestRepairM_1140_TenantProposalRestart(t *testing.T) {
	TestOneBlockCarriesRootAndChainTransactions(t)
	TestChainStateSurvivesRestart(t)
}
func TestRepairM_1140_LeaderCommittedBatches(t *testing.T) {
	TestAppLeaderScheduleReadsKeeperList(t)
	TestAppLeaderScheduleEmptyBeforeActivation(t)
}

// In addition to handler-originated panic fixtures, drive the SDK's own three
// supported upgrade halts through the production ProcessBlock boundary.
func TestRepairM_1140_SDKUpgradeHalts(t *testing.T) {
	for _, which := range []string{"needed", "wrong-version", "early-binary"} {
		t.Run(which, func(t *testing.T) {
			w := app.NewTestWrapperWithSc(t, repair102Time, secp256k1.GenPrivKey().PubKey(), false)
			t.Cleanup(func() { require.NoError(t, w.App.Close()) })
			params := w.App.GetConsensusParams(w.Ctx)
			params.Version = &tmproto.VersionParams{AppVersion: 5}
			w.Ctx = w.Ctx.WithConsensusParams(params)
			var expected string
			switch which {
			case "needed":
				require.NoError(t, w.App.UpgradeKeeper.ScheduleUpgrade(w.Ctx, upgradetypes.Plan{Name: "repair-unknown", Height: 1, Info: "halt"}))
				expected = `UPGRADE "repair-unknown" NEEDED at height: 1: halt`
			case "wrong-version":
				w.App.UpgradeKeeper.SetDone(w.Ctx, "repair-missing-handler")
				expected = fmt.Sprintf("Wrong app version %d, upgrade handler is missing for repair-missing-handler upgrade plan", w.Ctx.ConsensusParams().Version.AppVersion)
			case "early-binary":
				require.NoError(t, w.App.UpgradeKeeper.ScheduleUpgrade(w.Ctx, upgradetypes.Plan{Name: app.MergeUpgrade, Height: 2}))
				expected = `BINARY UPDATED BEFORE TRIGGER! UPGRADE "v7.0" - in binary but not executed on chain`
			}
			require.PanicsWithValue(t, expected, func() {
				_, _, _, _ = w.App.ProcessBlock(w.Ctx, nil, &app.BlockProcessRequest{Height: 1, Time: repair102Time}, abci.CommitInfo{}, false, nil)
			})
		})
	}
}

func TestRepairM_1140_InvarianceCorruption(t *testing.T) {
	address := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	balanceKey := append(banktypes.CreateAccountBalancesPrefix(address), []byte("uhpx")...)
	supplyKey := append(append([]byte{}, banktypes.SupplyKey...), []byte("uhpx")...)
	weiKey := append(append([]byte{}, banktypes.WeiBalancesPrefix...), address...)
	coin := sdk.NewInt64Coin("uhpx", 0)
	zeroCoin, err := coin.Marshal()
	require.NoError(t, err)
	zeroInt, err := sdk.ZeroInt().Marshal()
	require.NoError(t, err)
	for _, tc := range []struct {
		name       string
		key, valid []byte
		committed  bool
		message    string
	}{
		{"pre-supply", supplyKey, zeroInt, true, "failed to unmarshal pre total supply"},
		{"post-supply", supplyKey, zeroInt, false, "failed to unmarshal total supply"},
		{"pre-balance", balanceKey, zeroCoin, true, "failed to unmarshal preblock balance"},
		{"post-balance", balanceKey, zeroCoin, false, "failed to unmarshal balance"},
		{"pre-wei", weiKey, zeroInt, true, "failed to unmarshal preblock wei balance"},
		{"post-wei", weiKey, zeroInt, false, "failed to unmarshal wei balance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := app.NewTestWrapperWithSc(t, repair102Time, secp256k1.GenPrivKey().PubKey(), false)
			a := w.App
			t.Cleanup(func() { require.NoError(t, a.Close()) })
			a.SetDeliverStateToCommit()
			cms := a.WriteState()
			a.GetWorkingHash()
			require.NoError(t, a.LightInvarianceChecks(t.Context(), cms, app.LightInvarianceConfig{SupplyEnabled: true}))
			store, ok := cms.GetStore(a.BankKeeper.GetStoreKey()).(*commitment.Store)
			require.True(t, ok)
			corrupt := []byte{0xff, 0xff, 0xff}
			store.Set(tc.key, corrupt)
			if tc.committed {
				// Commit deliberately corrupted fixture bytes through the actual
				// commitment store; the next operation must refuse them.
				cms.Commit(true)
				store = cms.GetStore(a.BankKeeper.GetStoreKey()).(*commitment.Store)
				require.Equal(t, corrupt, store.Get(tc.key))
				if tc.name != "pre-supply" {
					store.Set(tc.key, tc.valid)
				}
			}
			before := a.LastCommitID()
			err := a.LightInvarianceChecks(t.Context(), cms, app.LightInvarianceConfig{SupplyEnabled: true})
			require.ErrorContains(t, err, "light invariance: "+tc.message)
			_, commitErr := a.Commit(t.Context())
			require.ErrorContains(t, commitErr, "commit refused after failed light invariance")
			require.ErrorIs(t, commitErr, err)
			require.Equal(t, before, a.LastCommitID())
		})
	}
}

func TestRepairM_1140_InvarianceUnsupportedStore(t *testing.T) {
	w := app.NewTestWrapper(t, repair102Time, secp256k1.GenPrivKey().PubKey(), false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	database := dbm.NewMemDB()
	defer database.Close()
	unsupported := rootmulti.NewStore(database)
	unsupported.MountStoreWithDB(w.App.BankKeeper.GetStoreKey(), storetypes.StoreTypeDB, nil)
	require.NoError(t, unsupported.LoadLatestVersion())
	err := w.App.LightInvarianceChecks(t.Context(), unsupported, app.LightInvarianceConfig{SupplyEnabled: true})
	require.ErrorContains(t, err, "unsupported bank store")
	_, commitErr := w.App.Commit(t.Context())
	require.ErrorIs(t, commitErr, err)
	require.Zero(t, w.App.LastBlockHeight())
}

func repairM1140ReplayChain(t *testing.T) *repair102Chain {
	t.Helper()
	h := repair102NewChain(t, repair102SigningKeys(t), 0, false)
	h.a.EvmKeeper.SetReplayedHeight(h.initialContext())
	h.a.EvmKeeper.SetReplayInitialHeight(h.initialContext(), 100)
	h.commitInitial(t, false)
	require.EqualValues(t, 1, h.a.EvmKeeper.GetReplayedHeight(h.a.GetCheckCtx()))
	return h
}

func repairM1140ReplayOptions() app.ReplayOptions {
	return app.ReplayOptions{RequestTimeout: 50 * time.Millisecond, RetryDelay: time.Millisecond, PollInterval: time.Minute, MaxRetries: 1}
}

func TestRepairM_1140_ReplayRequestDeadlineAndRestart(t *testing.T) {
	for _, delayedMethod := range []string{"eth_blockNumber", "eth_getBlockByNumber"} {
		t.Run(delayedMethod, func(t *testing.T) {
			h := repairM1140ReplayChain(t)
			var delayed atomic.Int32
			targets := make(chan string, 8)
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				if request.Method == "eth_getBlockByNumber" {
					targets <- string(request.Params[0])
				}
				if request.Method == delayedMethod {
					delayed.Add(1)
					select {
					case <-r.Context().Done():
					case <-time.After(time.Second):
					}
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x100"})
			}))
			defer peer.Close()
			for range 2 {
				h.a.EvmKeeper.EthReplayConfig.EthRPC = peer.URL
				before := h.a.LastCommitID()
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				started := time.Now()
				err := app.ReplayContext(ctx, h.a, repairM1140ReplayOptions())
				cancel()
				require.Error(t, err)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.Less(t, time.Since(started), time.Second)
				require.Equal(t, before, h.a.LastCommitID())
				require.EqualValues(t, 1, h.a.EvmKeeper.GetReplayedHeight(h.a.GetCheckCtx()))
				h.close()
				h.open(t)
				require.Equal(t, before, h.a.LastCommitID())
				require.EqualValues(t, 1, h.a.EvmKeeper.GetReplayedHeight(h.a.GetCheckCtx()))
			}
			require.EqualValues(t, 4, delayed.Load(), "each operation must exhaust exactly one retry")
			if delayedMethod == "eth_getBlockByNumber" {
				require.Len(t, targets, 4)
				for range 4 {
					require.Equal(t, `"0x66"`, <-targets, "restart must request the same uncommitted target")
				}
			}
		})
	}
}

func TestRepairM_1140_ReplayCancellation(t *testing.T) {
	for _, blockRequest := range []bool{false, true} {
		t.Run(fmt.Sprint(blockRequest), func(t *testing.T) {
			h := repairM1140ReplayChain(t)
			entered := make(chan struct{}, 1)
			release := make(chan struct{})
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage `json:"id"`
					Method string          `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				if blockRequest && request.Method == "eth_blockNumber" {
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x100"})
					return
				}
				entered <- struct{}{}
				select {
				case <-release:
				case <-r.Context().Done():
				}
			}))
			defer peer.Close()
			defer close(release)
			h.a.EvmKeeper.EthReplayConfig.EthRPC = peer.URL
			before := h.a.LastCommitID()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			opts := repairM1140ReplayOptions()
			opts.RequestTimeout = time.Second
			go func() { result <- app.ReplayContext(ctx, h.a, opts) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("replay never contacted HTTP peer")
			}
			cancel()
			select {
			case err := <-result:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("cancelled replay stuck on HTTP peer")
			}
			require.Equal(t, before, h.a.LastCommitID())
			require.EqualValues(t, 1, h.a.EvmKeeper.GetReplayedHeight(h.a.GetCheckCtx()))
		})
	}
}

func TestRepairM_1140_ReplayRetryClassification(t *testing.T) {
	for _, code := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusBadRequest} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			h := repairM1140ReplayChain(t)
			var calls atomic.Int32
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				http.Error(w, "recorded RPC refusal", code)
			}))
			defer peer.Close()
			h.a.EvmKeeper.EthReplayConfig.EthRPC = peer.URL
			err := app.ReplayContext(t.Context(), h.a, repairM1140ReplayOptions())
			require.Error(t, err)
			expected := int32(2)
			if code == http.StatusBadRequest {
				expected = 1
			}
			require.Equal(t, expected, calls.Load())
			require.EqualValues(t, 1, h.a.EvmKeeper.GetReplayedHeight(h.a.GetCheckCtx()))
		})
	}
}

func TestRepairM_1140_ReplayHeadPollingCancellation(t *testing.T) {
	h := repairM1140ReplayChain(t)
	polled := make(chan struct{}, 1)
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x1"})
		polled <- struct{}{}
	}))
	defer peer.Close()
	h.a.EvmKeeper.EthReplayConfig.EthRPC = peer.URL
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- app.ReplayContext(ctx, h.a, repairM1140ReplayOptions()) }()
	select {
	case <-polled:
	case <-time.After(time.Second):
		t.Fatal("replay did not poll")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("replay polling ignored cancellation")
	}
	require.EqualValues(t, 1, h.a.EvmKeeper.GetReplayedHeight(h.a.GetCheckCtx()))
	require.EqualValues(t, 1, h.a.LastBlockHeight())
}

func TestRepairM_1140_ReplayRPCRecovery(t *testing.T) {
	for _, code := range []int{-32005, -32002, -32602} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			h := repairM1140ReplayChain(t)
			var calls atomic.Int32
			recovered := make(chan struct{}, 1)
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID json.RawMessage `json:"id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				if calls.Add(1) == 1 {
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": code, "message": "recorded RPC error"}})
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x1"})
					recovered <- struct{}{}
				}
			}))
			defer peer.Close()
			h.a.EvmKeeper.EthReplayConfig.EthRPC = peer.URL
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- app.ReplayContext(ctx, h.a, repairM1140ReplayOptions()) }()
			if code == -32602 {
				select {
				case err := <-result:
					require.ErrorContains(t, err, "recorded RPC error")
				case <-time.After(time.Second):
					t.Fatal("permanent RPC error was not returned")
				}
				require.EqualValues(t, 1, calls.Load())
			} else {
				select {
				case <-recovered:
				case <-time.After(time.Second):
					t.Fatal("transient RPC error did not recover")
				}
				cancel()
				select {
				case err := <-result:
					require.ErrorIs(t, err, context.Canceled)
				case <-time.After(time.Second):
					t.Fatal("recovered replay did not stop")
				}
				require.EqualValues(t, 2, calls.Load())
			}
			require.EqualValues(t, 1, h.a.EvmKeeper.GetReplayedHeight(h.a.GetCheckCtx()))
		})
	}
}
