// Proves: perpskeeper.SeedV6120/markets.Refresh/Run -> evmrpc.SimulationAPI, source.RPC.Transport, PostgreSQL -> markets.Check, httpx.RegisterHealth, telemetry.Metrics.ServeHTTP.
//
// Repairs markets.go Check: MAX(refreshed_block) alone declared partially
// refreshed market sets healthy; readiness now requires one complete snapshot.
// The precompile has no result cache: two changes to the real keeper must be
// observed through fresh RPC calls and reopened database consumers.
// Outside scope, modules/perps/types/market.go Market.Validate and
// modules/perps/keeper/keeper.go SetMarket currently admit NUL in a market name.
// This proof preserves that behavior and requires markets.caller.market to
// reject the producer's actual ABI result before any database write.
package markets

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	nodeapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdkclient "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/httpx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/source"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/telemetry"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestProofMarkets(t *testing.T) {
	ctx, stop := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stop()

	// Real consensus supplies committed headers and block results, while the
	// application harness supplies the seeded EVM and perpetuals state, as in
	// rpc/simulate_basefee_test.go. No RPC service or transport is substituted.
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	ledger := kvstore.NewApplication()
	node, closeNode, err := rpctest.StartTendermint(ctx, conf, ledger, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closeNode(context.Background()))
		require.NoError(t, ledger.Close())
	})
	nodeService, ok := node.(rpclocal.NodeService)
	require.True(t, ok)
	consensus, err := rpclocal.New(nodeService)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		status, err := consensus.Status(ctx)
		return err == nil && status.SyncInfo.LatestBlockHeight >= 2
	}, 30*time.Second, 20*time.Millisecond, "consensus never committed its first two blocks")

	blockHeight := int64(1)
	block, err := consensus.Block(ctx, &blockHeight)
	require.NoError(t, err)
	application := nodeapp.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	base := application.GetContextForDeliverTx(nil).WithBlockHeight(1).
		WithBlockTime(block.Block.Time).WithChainID("paxeer-1")
	application.UpgradeKeeper.SetDone(base, nodeapp.V6120Upgrade)
	application.RefreshPrecompiles(base)
	require.NoError(t, perpskeeper.SeedV6120(base, *application.PerpsKeeper))
	require.NoError(t, application.SetSealedStateToCommit(base, base.MultiStore().(sdk.CacheMultiStore)))
	_, err = application.Commit(ctx)
	require.NoError(t, err)
	base = base.WithMultiStore(application.CommitMultiStore().CacheMultiStore())
	// Keep the RPC watermark at the one committed application snapshot. A
	// keeper update below changes the producer at that same height, exposing
	// any shortcut which reuses a previous eth_call result.
	provider := func(h int64) sdk.Context {
		if h == evmrpc.LatestCtxHeight {
			return base
		}
		return base.WithBlockHeight(h)
	}
	encoding := nodeapp.MakeEncodingConfig()
	txProvider := func(int64) sdkclient.TxConfig { return encoding.TxConfig }
	watermarks := evmrpc.NewWatermarkManager(consensus, provider, nil, nil)
	cache := evmrpc.NewBlockCache(64)
	cacheMutex := &sync.Mutex{}
	simulation := evmrpc.NewSimulationAPI(provider, &application.EvmKeeper, application.BeginBlockKeepers,
		txProvider, consensus, &evmrpc.SimulateConfig{GasCap: 10_000_000, EVMTimeout: time.Second},
		application.BaseApp, application.TracerAnteHandler, evmrpc.ConnectionTypeHTTP, cache, cacheMutex, watermarks)
	info := evmrpc.NewInfoAPI(consensus, &application.EvmKeeper, provider, txProvider, t.TempDir(), 64,
		evmrpc.ConnectionTypeHTTP, encoding.TxConfig.TxDecoder(), watermarks)
	server := gethrpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", simulation))
	require.NoError(t, server.RegisterName("eth", info))
	// Serialize application state access with producer mutations. The handler
	// remains the production JSON-RPC server, including all ABI/EVM execution.
	var chainMu sync.Mutex
	rpcHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chainMu.Lock()
		defer chainMu.Unlock()
		server.ServeHTTP(w, r)
	}))
	t.Cleanup(rpcHTTP.Close)
	pool, err := source.NewRPC(ctx, source.RPCOptions{URLs: []string{rpcHTTP.URL}})
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	rpcClient, err := gethrpc.DialOptions(ctx, rpcHTTP.URL,
		gethrpc.WithHTTPClient(&http.Client{Transport: pool.Transport()}))
	require.NoError(t, err)
	client := ethclient.NewClient(rpcClient)
	t.Cleanup(client.Close)

	db := testDB(t)
	reseed(t, db)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	readiness := func(want int) {
		t.Helper()
		mux := http.NewServeMux()
		httpx.RegisterHealth(mux, httpx.HealthDeps{Postgres: db.PingContext,
			Custom: map[string]httpx.Check{"markets": Check(db, time.Minute)}})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/ready", nil))
		require.Equal(t, want, rec.Code, rec.Body.String())
		var body struct {
			Checks map[string]string `json:"checks"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		expected := "ok"
		if want != http.StatusOK {
			expected = "failed"
		}
		require.Equal(t, expected, body.Checks["markets"])
	}
	consume := func(name string) {
		t.Helper()
		consumer, err := rest.NewMarkets(db, nil)
		require.NoError(t, err)
		mux := http.NewServeMux()
		consumer.Register(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/markets/BTC", nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Market rest.MarketRecord `json:"market"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "0", body.Market.MarketID)
		require.Equal(t, "BTC", body.Market.Symbol)
		require.Equal(t, name, body.Market.SymbolFull)
	}
	updateName := func(name string) perpstypes.Market {
		t.Helper()
		chainMu.Lock()
		defer chainMu.Unlock()
		market, err := application.PerpsKeeper.GetMarket(base, 0)
		require.NoError(t, err)
		market.Name = name
		require.NoError(t, application.PerpsKeeper.SetMarket(base, application.PerpsKeeper.Authority(), market))
		return market
	}
	// Seed listing times and index the header from the same real block that
	// the production simulation API uses for historical precompile calls.
	_, err = db.ExecContext(ctx, `INSERT INTO perps.blocks (number, hash, parent_hash, time, proposer, tx_count)
VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (number) DO UPDATE SET time = EXCLUDED.time`,
		blockHeight, []byte(block.BlockID.Hash), common.BytesToHash(block.Block.LastBlockID.Hash).Bytes(), block.Block.Time,
		fmt.Sprintf("%X", block.Block.ProposerAddress), len(block.Block.Txs))
	require.NoError(t, err)
	readiness(http.StatusServiceUnavailable)

	for pass := range 2 {
		name := fmt.Sprintf("Bitcoin proof %d", pass)
		market := updateName(name)
		require.NoError(t, Refresh(ctx, db, client))
		// Close and reopen the SQL consumer after the producer commits.
		require.NoError(t, db.Close())
		db, err = store.Open(os.Getenv(testDatabaseEnv))
		require.NoError(t, err)
		rows := readRows(t, db)
		require.Len(t, rows, 18)
		for _, row := range rows {
			require.Equal(t, "chain", row.source)
			require.Equal(t, sql.NullInt64{Int64: 1, Valid: true}, row.refreshedBlock)
		}
		tiers, err := perpstypes.TiersFor(market)
		require.NoError(t, err)
		require.Equal(t, name, rows[0].name)
		require.Equal(t, "1000", rows[0].maxLeverage)
		require.Equal(t, market.OiCapUsd.String(), rows[0].maxOpenInterest)
		require.Equal(t, fmt.Sprint(tiers[0].MaintenanceBps), rows[0].maintenanceMarginBps)
		tierRows, err := db.QueryContext(ctx, `SELECT lower_bound_usd::text, max_leverage::text, maintenance_bps::text, deduction_usd::text, refreshed_block FROM perps.market_tiers WHERE market_id = 0 ORDER BY tier`)
		require.NoError(t, err)
		n := 0
		for tierRows.Next() {
			require.Less(t, n, len(tiers))
			var lower, leverage, maintenance, deduction string
			var height int64
			require.NoError(t, tierRows.Scan(&lower, &leverage, &maintenance, &deduction, &height))
			require.Equal(t, tiers[n].LowerBoundUsd.String(), lower)
			require.Equal(t, fmt.Sprint(tiers[n].MaxLeverage), leverage)
			require.Equal(t, fmt.Sprint(tiers[n].MaintenanceBps), maintenance)
			require.Equal(t, tiers[n].DeductionUsd.String(), deduction)
			require.Equal(t, int64(1), height)
			n++
		}
		require.NoError(t, tierRows.Err())
		require.NoError(t, tierRows.Close())
		require.Equal(t, len(tiers), n)
		var taker, maker, liquidation, insurance, feeHeight int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT taker_fee_ppm, maker_fee_ppm, liquidation_fee_ppm, insurance_share_ppm, refreshed_block FROM perps.fee_config WHERE id = 1`).Scan(&taker, &maker, &liquidation, &insurance, &feeHeight))
		require.Equal(t, []int64{450, 150, 0, 0, 1}, []int64{taker, maker, liquidation, insurance, feeHeight})
		consume(name)
		readiness(http.StatusOK)
		// A recent row cannot conceal either a seed row or a different height.
		_, err = db.ExecContext(ctx, `UPDATE perps.markets SET source = 'seed' WHERE market_id = 17`)
		require.NoError(t, err)
		readiness(http.StatusServiceUnavailable)
		_, err = db.ExecContext(ctx, `UPDATE perps.markets SET source = 'chain', refreshed_block = 0 WHERE market_id = 17`)
		require.NoError(t, err)
		readiness(http.StatusServiceUnavailable)
		_, err = db.ExecContext(ctx, `UPDATE perps.markets SET refreshed_block = 1 WHERE market_id = 17`)
		require.NoError(t, err)
		readiness(http.StatusOK)
	}

	metrics, err := telemetry.New(config.Indexer)
	require.NoError(t, err)
	metricsHTTP := httptest.NewServer(metrics)
	t.Cleanup(metricsHTTP.Close)
	scrape := func() map[string]*dto.MetricFamily {
		t.Helper()
		response, err := http.Get(metricsHTTP.URL)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		parser := expfmt.NewTextParser(model.LegacyValidation)
		families, err := parser.TextToMetricFamilies(response.Body)
		require.NoError(t, err)
		return families
	}
	value := func(families map[string]*dto.MetricFamily, name string, kind dto.MetricType) float64 {
		t.Helper()
		family := families[name]
		require.NotNil(t, family, "missing metric %s", name)
		require.Equal(t, kind, family.GetType())
		require.Len(t, family.Metric, 1)
		if kind == dto.MetricType_COUNTER {
			return family.Metric[0].GetCounter().GetValue()
		}
		return family.Metric[0].GetGauge().GetValue()
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- Run(runCtx, db, client, 30*time.Millisecond, metrics) }()
	t.Cleanup(func() {
		cancelRun()
		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			t.Error("Run did not stop on cancellation")
		}
	})
	for pass := range 2 {
		name := fmt.Sprintf("Run Bitcoin %d", pass)
		updateName(name)
		require.Eventually(t, func() bool {
			var got string
			err := db.QueryRowContext(ctx, `SELECT name FROM perps.markets WHERE market_id = 0`).Scan(&got)
			return err == nil && got == name
		}, 10*time.Second, 10*time.Millisecond, "Run did not consume the new producer state")
		require.Eventually(t, func() bool {
			return value(scrape(), perps_markets_refresh_height, dto.MetricType_GAUGE) == 1
		}, time.Second, 10*time.Millisecond)
		families := scrape()
		require.Equal(t, float64(0), value(families, perps_markets_refresh_failures_total, dto.MetricType_COUNTER))
		consume(name)
		readiness(http.StatusOK)
	}
	snapshot := func() string {
		t.Helper()
		var state string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT jsonb_build_object(
'markets', (SELECT jsonb_agg(to_jsonb(m) ORDER BY market_id) FROM perps.markets m),
'fees', (SELECT jsonb_agg(to_jsonb(f) ORDER BY id) FROM perps.fee_config f),
 'tiers', (SELECT jsonb_agg(to_jsonb(t) ORDER BY market_id, tier) FROM perps.market_tiers t))::text`).Scan(&state))
		return state
	}
	before := snapshot()
	updateName("Bitcoin\x00malformed")
	err = Refresh(ctx, db, client)
	require.True(t, errors.Is(err, ErrChainValue), "Refresh must refuse decoded producer text: %v", err)
	require.Contains(t, err.Error(), "getMarket(0)")
	require.Contains(t, err.Error(), "not storable text")
	require.Eventually(t, func() bool {
		return value(scrape(), perps_markets_refresh_failures_total, dto.MetricType_COUNTER) == 1
	}, 10*time.Second, 10*time.Millisecond, "Run did not expose the refused refresh")
	require.Equal(t, float64(1), value(scrape(), perps_markets_refresh_height, dto.MetricType_GAUGE))
	require.Equal(t, before, snapshot(), "a refused producer changed committed markets, fees or tiers")
	consume("Run Bitcoin 1")
	readiness(http.StatusOK)
}
