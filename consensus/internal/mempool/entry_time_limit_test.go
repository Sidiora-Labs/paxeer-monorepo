package mempool

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
)

type entryTimeLimitApplication struct {
	*application
	rejectRechecks bool
}

func (app *entryTimeLimitApplication) CheckTx(ctx context.Context, req *abci.RequestCheckTxV2) *abci.ResponseCheckTxV2 {
	res := app.application.CheckTx(ctx, req)
	if app.rejectRechecks && req.Type == abci.CheckTxTypeV2Recheck {
		res.Code = 1
	}
	return res
}

func TestEntryTimeLimitInvalidEntryLeavesWithoutLimit(t *testing.T) {
	app := &entryTimeLimitApplication{application: &application{Application: kvstore.NewApplication()}, rejectRechecks: true}
	cfg := TestConfig()
	cfg.TTLDuration = utils.None[time.Duration]()
	cfg.TTLNumBlocks = utils.None[int64]()
	txmp := setup(cfg, proxy.New(app, proxy.NopMetrics()), NopTxConstraintsFetcher)
	tx := []byte("sender=key=1")

	_, err := txmp.CheckTx(t.Context(), tx)
	require.NoError(t, err)
	txmp.Lock()
	require.NoError(t, txmp.Update(t.Context(), 1, nil, nil, NopTxConstraints(), false))
	txmp.Unlock()

	require.Equal(t, 1, txmp.Size())
}

func TestEntryTimeLimitInvalidEntryLeavesAfterLimit(t *testing.T) {
	app := &entryTimeLimitApplication{application: &application{Application: kvstore.NewApplication()}, rejectRechecks: true}
	cfg := TestConfig()
	cfg.TTLDuration = utils.Some(time.Nanosecond)
	cfg.TTLNumBlocks = utils.None[int64]()
	cfg.RemoveExpiredTxsFromQueue = true
	txmp := setup(cfg, proxy.New(app, proxy.NopMetrics()), NopTxConstraintsFetcher)
	tx := []byte("sender=key=1")

	_, err := txmp.CheckTx(t.Context(), tx)
	require.NoError(t, err)
	time.Sleep(time.Millisecond)
	txmp.Lock()
	require.NoError(t, txmp.Update(t.Context(), 1, nil, nil, NopTxConstraints(), false))
	txmp.Unlock()

	require.Equal(t, 0, txmp.Size())
}

func TestEntryTimeLimitWarningLoggedOnce(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := logger
	logger = slog.New(slog.NewTextHandler(&logs, nil))
	defer func() { logger = previousLogger }()

	noLimit := TestConfig()
	noLimit.TTLDuration = utils.None[time.Duration]()
	noLimit.TTLNumBlocks = utils.None[int64]()
	withLimit := TestConfig()
	withLimit.TTLDuration = utils.Some(time.Second)

	NewTxMempool(noLimit, proxy.New(&application{Application: kvstore.NewApplication()}, proxy.NopMetrics()), NopMetrics(), NopTxConstraintsFetcher)
	NewTxMempool(withLimit, proxy.New(&application{Application: kvstore.NewApplication()}, proxy.NopMetrics()), NopMetrics(), NopTxConstraintsFetcher)

	require.Equal(t, 1, strings.Count(logs.String(), "mempool has no entry time limit"))
	require.Contains(t, logs.String(), "ttl-duration")
	require.Contains(t, logs.String(), "ttl-num-blocks")
}
