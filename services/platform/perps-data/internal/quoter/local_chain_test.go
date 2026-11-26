package quoter

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
)

type dryQuoteEvidence struct {
	Market uint64 `json:"market"`
	Side   string `json:"side"`
	Price  string `json:"price"`
	Base   string `json:"sizeBase"`
	USD    string `json:"sizeUsd"`
}
type localQuoteEvidence struct {
	Dry               []dryQuoteEvidence `json:"dry"`
	FillPrice         string             `json:"fillPrice"`
	FillBase          string             `json:"fillSizeBase"`
	QuoterPosition    string             `json:"quoterPositionBase"`
	TakerPosition     string             `json:"takerPositionBase"`
	OrdersRested      int                `json:"ordersRested"`
	OrdersCancelled   int                `json:"ordersCancelled"`
	OrdersLeft        int                `json:"ordersLeft"`
	RoundTransactions []int              `json:"roundTransactions"`
	TotalTransactions uint64             `json:"totalTransactions"`
	TransactionBound  int                `json:"transactionBound"`
}

func TestLocalChainMarketOrderFillsAgainstTheQuotes(t *testing.T) {
	endpoint, key := os.Getenv("PERPS_DATA_TEST_RPC"), os.Getenv("LOCAL_NODE_KEY")
	if endpoint == "" || key == "" {
		t.Skip("PERPS_DATA_TEST_RPC and LOCAL_NODE_KEY select a funded local chain")
	}
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "::1", "localhost"}, u.Hostname(), "live quoting case requires a local node")
	require.Equal(t, "http", u.Scheme)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	keyFile := filepath.Join(t.TempDir(), "quoter.hex")
	require.NoError(t, os.WriteFile(keyFile, []byte(key), 0600))
	q := config.DefaultQuoterConfig()
	q.KeyFile = keyFile
	q.Markets = []uint64{0}
	cfg := &config.Config{Service: config.Quoter, ChainID: 125, RPCURLs: []string{endpoint}, Port: 8081, Quoting: &q}
	dryRPC, err := NewRPC(ctx, cfg)
	require.NoError(t, err)
	defer dryRPC.Close()
	var dryLogs bytes.Buffer
	dry, err := New(dryRPC, q, slog.New(slog.NewJSONHandler(&dryLogs, nil)))
	require.NoError(t, err)
	before, err := dryRPC.Orders(ctx)
	require.NoError(t, err)
	require.Empty(t, before)
	dryNonce, err := dryRPC.client.NonceAt(ctx, dryRPC.account, nil)
	require.NoError(t, err)
	plans, err := dry.Round(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, plans[0], 4)
	require.Equal(t, before, orders(t, dryRPC))
	afterDryNonce, err := dryRPC.client.NonceAt(ctx, dryRPC.account, nil)
	require.NoError(t, err)
	require.Equal(t, dryNonce, afterDryNonce)
	require.Zero(t, dry.Health().TransactionsLastRound)
	require.Contains(t, dryLogs.String(), "quoter place")
	evidence := localQuoteEvidence{}
	for _, o := range plans[0] {
		side := "sell"
		if o.Buy {
			side = "buy"
		}
		evidence.Dry = append(evidence.Dry, dryQuoteEvidence{Market: o.Market, Side: side, Price: o.Price.String(), Base: o.Base.String(), USD: o.SizeUSD.String()})
		t.Logf("dry quote %s", orderText(o))
	}
	dryResult, err := dry.Stop(ctx)
	require.NoError(t, err)
	require.Zero(t, dryResult.OrdersLeft)
	q.Mode = "live"
	maker, err := NewRPC(ctx, cfg)
	require.NoError(t, err)
	defer maker.Close()
	// The owner supplies collateral; these setup transfers belong to the case.
	deposit, err := venueABI.Pack("deposit", collateralAddress, big.NewInt(1_000_000_000))
	require.NoError(t, err)
	tokenABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"approve","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]},{"type":"function","name":"transfer","inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"type":"bool"}]}]`))
	require.NoError(t, err)
	approval, err := tokenABI.Pack("approve", venueAddress, big.NewInt(1_000_000_000))
	require.NoError(t, err)
	require.NoError(t, maker.send(ctx, collateralAddress, approval, "owner:approve"))
	require.NoError(t, maker.send(ctx, venueAddress, deposit, "owner:deposit"))
	takerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	takerFile := filepath.Join(t.TempDir(), "taker.hex")
	require.NoError(t, os.WriteFile(takerFile, []byte(common.Bytes2Hex(crypto.FromECDSA(takerKey))), 0600))
	takerQ := q
	takerQ.KeyFile = takerFile
	takerCfg := *cfg
	takerCfg.Quoting = &takerQ
	taker, err := NewRPC(ctx, &takerCfg)
	require.NoError(t, err)
	defer taker.Close()
	// Native gas funding uses an ordinary value transfer, signed by the owner.
	nonce, err := maker.client.PendingNonceAt(ctx, maker.account)
	require.NoError(t, err)
	gasPrice, err := maker.client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	fundLocalAccount(t, ctx, maker, taker.account, nonce, gasPrice)
	transfer, err := tokenABI.Pack("transfer", taker.account, big.NewInt(500_000_000))
	require.NoError(t, err)
	require.NoError(t, maker.send(ctx, collateralAddress, transfer, "owner:fund-taker"))
	approval, err = tokenABI.Pack("approve", venueAddress, big.NewInt(500_000_000))
	require.NoError(t, err)
	require.NoError(t, taker.send(ctx, collateralAddress, approval, "taker:approve"))
	deposit, err = venueABI.Pack("deposit", collateralAddress, big.NewInt(500_000_000))
	require.NoError(t, err)
	require.NoError(t, taker.send(ctx, venueAddress, deposit, "taker:deposit"))
	makerBefore, err := maker.Snapshot(ctx, 0)
	require.NoError(t, err)
	require.Zero(t, makerBefore.PositionBase.Sign())
	startNonce, err := maker.client.NonceAt(ctx, maker.account, nil)
	require.NoError(t, err)
	process := startLocalQuoter(t, ctx, cfg)
	open := process.waitOrders(t, ctx, maker, 0)
	require.Len(t, open, 4)
	evidence.OrdersRested = len(open)
	t.Logf("market=0 rested=%d", len(open))
	asks := []Order{}
	for _, o := range open {
		if !o.Buy {
			asks = append(asks, o)
		}
	}
	require.Len(t, asks, 2)
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price.Cmp(asks[j].Price) < 0 })
	ask := asks[0]
	snap, err := maker.Snapshot(ctx, 0)
	require.NoError(t, err)
	fillBase := new(big.Int).Quo(ask.Base, big.NewInt(2))
	fillBase.Mul(new(big.Int).Quo(fillBase, snap.Market.Lot), snap.Market.Lot)
	fillUSD := ceilDiv(new(big.Int).Mul(fillBase, snap.Price.Mark), unit)
	require.GreaterOrEqual(t, fillUSD.Cmp(snap.Market.MinOrderUSD), 0)
	p := params(Order{Market: 0, Buy: true, SizeUSD: fillUSD, Price: new(big.Int)})
	p.OrderType = 4
	p.TimeInForce = 2
	p.AcceptablePrice = clone(ask.Price)
	input, err := venueABI.Pack("placeOrder", p)
	require.NoError(t, err)
	header, err := maker.client.HeaderByNumber(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, taker.send(ctx, venueAddress, input, "taker:market-buy"))
	makerAfter, err := maker.Snapshot(ctx, 0)
	require.NoError(t, err)
	takerAfter, err := taker.Snapshot(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, new(big.Int).Neg(fillBase), makerAfter.PositionBase)
	require.Equal(t, fillBase, takerAfter.PositionBase)
	logs, err := maker.client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: header.Number, Addresses: []common.Address{venueAddress}, Topics: [][]common.Hash{{venueABI.Events["Fill"].ID}}})
	require.NoError(t, err)
	found := false
	for _, log := range logs {
		values, err := venueABI.Events["Fill"].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		if values[0].(common.Address) != maker.account || values[1].(common.Address) != taker.account {
			continue
		}
		require.False(t, found, "one resting order covers this fill")
		found = true
		price, size := values[3].(*big.Int), values[4].(*big.Int)
		require.Equal(t, ask.Price, price)
		require.Equal(t, new(big.Int).Quo(new(big.Int).Mul(fillBase, price), unit), size)
		evidence.FillPrice = price.String()
		evidence.FillBase = fillBase.String()
		evidence.QuoterPosition = makerAfter.PositionBase.String()
		evidence.TakerPosition = takerAfter.PositionBase.String()
		t.Logf("fill price=%s base=%s quoterPosition=%s takerPosition=%s", evidence.FillPrice, evidence.FillBase, evidence.QuoterPosition, evidence.TakerPosition)
	}
	require.True(t, found, "the venue must emit a fill against the quoter")
	renewed := process.waitOrders(t, ctx, maker, ask.Round)
	require.Len(t, renewed, 4)
	freshSnap, err := maker.Snapshot(ctx, 0)
	require.NoError(t, err)
	for _, o := range renewed {
		require.NotEqual(t, ask.Round, o.Round)
		if o.Buy {
			require.Negative(t, o.Price.Cmp(freshSnap.Price.Mark))
		} else {
			require.Positive(t, o.Price.Cmp(freshSnap.Price.Mark))
		}
	}
	process.stop(t)
	require.Empty(t, orders(t, maker))
	evidence.OrdersCancelled = len(renewed)
	evidence.OrdersLeft = 0
	evidence.TransactionBound = q.MaxTransactionsPerRound
	evidence.RoundTransactions, evidence.TotalTransactions = process.transactions(t, q.MaxTransactionsPerRound)
	endNonce, err := maker.client.NonceAt(ctx, maker.account, nil)
	require.NoError(t, err)
	require.Equal(t, endNonce-startNonce, evidence.TotalTransactions)
	t.Logf("transaction bound=%d roundTransactions=%v totalTransactions=%d", evidence.TransactionBound, evidence.RoundTransactions, evidence.TotalTransactions)
	t.Logf("market=0 stop cancelled=%d ordersLeft=%d", evidence.OrdersCancelled, evidence.OrdersLeft)
	raw, err := json.MarshalIndent(evidence, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "perps-quoter-local-proof.json"), raw, 0600))
}
