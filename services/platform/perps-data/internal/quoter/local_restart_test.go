package quoter

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func waitLocalRestartRound(t *testing.T, p *localQuoterProcess, transactions int) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, line := range strings.Split(p.logs(), "\n") {
			var entry struct {
				Message      string `json:"msg"`
				Transactions int    `json:"transactions"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Message == "quoter round" {
				require.Equal(t, transactions, entry.Transactions, "quoter: %s", p.logs())
				return true
			}
		}
		return false
	}, 30*time.Second, 50*time.Millisecond, "quoter: %s", p.logs())
}

func TestLocalChainRestartKeepsTheQuotes(t *testing.T) {
	endpoint, ownerKey := os.Getenv("PERPS_DATA_TEST_RPC"), os.Getenv("LOCAL_NODE_KEY")
	if endpoint == "" || ownerKey == "" {
		t.Skip("PERPS_DATA_TEST_RPC and LOCAL_NODE_KEY select a funded local chain")
	}
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "::1", "localhost"}, u.Hostname())
	require.Equal(t, "http", u.Scheme)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	keyFile := filepath.Join(t.TempDir(), "quoter.hex")
	require.NoError(t, os.WriteFile(keyFile, []byte(common.Bytes2Hex(crypto.FromECDSA(key))), 0600))
	q := config.DefaultQuoterConfig()
	q.KeyFile, q.Mode, q.Markets = keyFile, "live", []uint64{0}
	q.Handover = true
	cfg := &config.Config{Service: config.Quoter, ChainID: 125, RPCURLs: []string{endpoint}, Port: 8081, Quoting: &q}
	maker, err := NewRPC(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(maker.Close)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 2*time.Minute)
		defer done()
		engine, err := New(maker, q, slog.Default())
		require.NoError(t, err)
		result, err := engine.Stop(cleanup)
		require.NoError(t, err)
		require.Zero(t, result.OrdersLeft)
	})
	ownerFile := filepath.Join(t.TempDir(), "owner.hex")
	require.NoError(t, os.WriteFile(ownerFile, []byte(ownerKey), 0600))
	ownerQ := q
	ownerQ.KeyFile = ownerFile
	ownerCfg := *cfg
	ownerCfg.Quoting = &ownerQ
	owner, err := NewRPC(ctx, &ownerCfg)
	require.NoError(t, err)
	defer owner.Close()
	nonce, err := owner.client.NonceAt(ctx, owner.account, nil)
	require.NoError(t, err)
	gasPrice, err := owner.client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	fundLocalAccount(t, ctx, owner, maker.account, nonce, gasPrice)
	tokenABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"approve","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]},{"type":"function","name":"transfer","inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]}]`))
	require.NoError(t, err)
	transfer, err := tokenABI.Pack("transfer", maker.account, big.NewInt(1_000_000_000))
	require.NoError(t, err)
	require.NoError(t, owner.send(ctx, collateralAddress, transfer, "owner:fund-maker"))
	approval, err := tokenABI.Pack("approve", venueAddress, big.NewInt(1_000_000_000))
	require.NoError(t, err)
	require.NoError(t, maker.send(ctx, collateralAddress, approval, "maker:approve"))
	deposit, err := venueABI.Pack("deposit", collateralAddress, big.NewInt(1_000_000_000))
	require.NoError(t, err)
	require.NoError(t, maker.send(ctx, venueAddress, deposit, "maker:deposit"))
	settings := []string{"PERPS_QUOTER_HANDOVER=true", "PERPS_QUOTER_MAX_QUOTE_AGE_SECONDS=600", "PERPS_QUOTER_REFRESH_BPS=9999"}
	first := startLocalQuoter(t, ctx, cfg, settings...)
	first.ordersLeftOnStop = 4
	before := first.waitOrders(t, ctx, maker, 0)
	waitLocalRestartRound(t, first, 4)
	first.stop(t)
	require.Equal(t, before, orders(t, maker))
	require.Contains(t, first.logs(), `"handover":true`)
	startNonce, err := maker.client.NonceAt(ctx, maker.account, nil)
	require.NoError(t, err)
	second := startLocalQuoter(t, ctx, cfg, settings...)
	second.ordersLeftOnStop = 4
	after := second.waitOrders(t, ctx, maker, 0)
	waitLocalRestartRound(t, second, 0)
	require.Equal(t, before, after)
	require.Contains(t, second.logs(), `"msg":"quoter keep"`)
	require.NotContains(t, second.logs(), `"msg":"quoter cancel"`)
	require.NotContains(t, second.logs(), `"msg":"quoter place"`)
	second.stop(t)
	endNonce, err := maker.client.NonceAt(ctx, maker.account, nil)
	require.NoError(t, err)
	require.Equal(t, startNonce, endNonce)
	require.Equal(t, before, orders(t, maker))
	last := startLocalQuoter(t, ctx, cfg, append(settings, "PERPS_QUOTER_STOP=true")...)
	select {
	case err := <-last.done:
		last.stopped = true
		require.NoError(t, err, "quoter: %s", last.logs())
	case <-ctx.Done():
		t.Fatal("stop invocation did not finish")
	}
	require.Contains(t, last.logs(), `"msg":"perps-quoter shutdown","mode":"live","ordersLeft":0`)
	require.NotContains(t, last.logs(), `"msg":"quoter place"`)
	require.Empty(t, orders(t, maker))
}
