package quoter

import (
	"context"
	"fmt"
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
	"golang.org/x/sync/errgroup"
)

func TestLocalChainOpenReadsAskAgain(t *testing.T) {
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
	ownerFile := filepath.Join(t.TempDir(), "owner.hex")
	require.NoError(t, os.WriteFile(ownerFile, []byte(ownerKey), 0600))
	q := config.DefaultQuoterConfig()
	q.KeyFile, q.Mode, q.MaxOpenReads = ownerFile, "live", 8
	cfg := &config.Config{Service: config.Quoter, ChainID: 125, RPCURLs: []string{endpoint}, Port: 8081, Quoting: &q}
	owner, err := NewRPC(ctx, cfg)
	require.NoError(t, err)
	defer owner.Close()
	tokenABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"approve","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]},{"type":"function","name":"transfer","inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]}]`))
	require.NoError(t, err)
	clients := make([]*RPC, 3)
	for i := range clients {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		keyFile := filepath.Join(t.TempDir(), "quoter.hex")
		require.NoError(t, os.WriteFile(keyFile, []byte(common.Bytes2Hex(crypto.FromECDSA(key))), 0600))
		settings := q
		settings.KeyFile = keyFile
		clientCfg := *cfg
		clientCfg.Quoting = &settings
		clients[i], err = NewRPC(ctx, &clientCfg)
		require.NoError(t, err)
		t.Cleanup(clients[i].Close)
		nonce, err := owner.client.NonceAt(ctx, owner.account, nil)
		require.NoError(t, err)
		gasPrice, err := owner.client.SuggestGasPrice(ctx)
		require.NoError(t, err)
		fundLocalAccount(t, ctx, owner, clients[i].account, nonce, gasPrice)
		transfer, err := tokenABI.Pack("transfer", clients[i].account, big.NewInt(1_000_000_000))
		require.NoError(t, err)
		require.NoError(t, owner.send(ctx, collateralAddress, transfer, fmt.Sprintf("owner:fund-reader:%d", i)))
		approval, err := tokenABI.Pack("approve", venueAddress, big.NewInt(1_000_000_000))
		require.NoError(t, err)
		require.NoError(t, clients[i].send(ctx, collateralAddress, approval, "reader:approve"))
		deposit, err := venueABI.Pack("deposit", collateralAddress, big.NewInt(1_000_000_000))
		require.NoError(t, err)
		require.NoError(t, clients[i].send(ctx, venueAddress, deposit, "reader:deposit"))
	}
	markets, err := owner.ActiveMarkets(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, markets)
	ownerNonce, err := owner.client.NonceAt(ctx, owner.account, nil)
	require.NoError(t, err)
	before := make([]uint64, len(clients))
	busyBefore := make([]uint64, len(clients))
	for i, client := range clients {
		before[i], err = client.client.NonceAt(ctx, client.account, nil)
		require.NoError(t, err)
		busyBefore[i] = client.BusyResponses()
	}
	start := make(chan struct{})
	group, reads := errgroup.WithContext(ctx)
	for _, client := range clients {
		group.Go(func() error {
			<-start
			for _, market := range markets {
				snapshot, err := client.Snapshot(reads, market)
				if err != nil {
					return fmt.Errorf("snapshot market %d: %w", market, err)
				}
				if err := validateSnapshot(snapshot); err != nil {
					return err
				}
				if snapshot.Market.ID != market || !snapshot.Market.Enabled || snapshot.FreeUSD.Sign() <= 0 || snapshot.PositionBase.Sign() != 0 || len(snapshot.Orders) != 0 {
					return fmt.Errorf("funded reader snapshot for market %d has unexpected account or market state", market)
				}
			}
			orders, err := client.Orders(reads)
			if err != nil {
				return err
			}
			if len(orders) != 0 {
				return fmt.Errorf("reader has %d unexpected orders", len(orders))
			}
			return nil
		})
	}
	close(start)
	require.NoError(t, group.Wait())
	var busy uint64
	for i, client := range clients {
		count := client.BusyResponses() - busyBefore[i]
		busy += count
		t.Logf("reader=%d max_open_reads=%d markets=%d busy=%d", i, client.openReadLimit(), len(markets), count)
		after, err := client.client.NonceAt(ctx, client.account, nil)
		require.NoError(t, err)
		require.Equal(t, before[i], after, "reads send no transactions")
	}
	afterOwnerNonce, err := owner.client.NonceAt(ctx, owner.account, nil)
	require.NoError(t, err)
	require.Equal(t, ownerNonce, afterOwnerNonce)
	require.Positive(t, busy, "concurrent readers must exercise busy responses")
	t.Logf("readers=%d snapshots=%d busy_total=%d", len(clients), len(clients)*len(markets), busy)
}
