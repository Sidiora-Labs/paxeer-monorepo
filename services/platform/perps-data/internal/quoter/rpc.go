package quoter

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	priceabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
)

var venueAddress = common.HexToAddress("0x0000000000000000000000000000000000001015")
var feedAddress = common.HexToAddress("0x0000000000000000000000000000000000001020")
var collateralAddress = common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
var venueABI, venueABIError = perpsabi.ABI()
var feedABI, feedABIError = abi.JSON(strings.NewReader(priceabi.JSON))

const requestTimeout = 45 * time.Second
const pendingTagRetryInterval = time.Minute

// RPC owns one signing account and a serial transaction stream. Ambiguous
// submissions retain the signed transaction and nonce until a receipt exists.
type RPC struct {
	client            *ethclient.Client
	key               *ecdsa.PrivateKey
	account           common.Address
	chain             *big.Int
	mode              string
	mu                sync.Mutex
	pending           *pendingTx
	completed         map[string]bool
	completionOrder   []string
	now               func() time.Time
	pendingTagRetryAt time.Time
}
type pendingTx struct {
	tx   *types.Transaction
	to   common.Address
	data []byte
	key  string
}

func readKey(path string) (*ecdsa.PrivateKey, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("quoter key file cannot be opened")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || (info.Mode().Perm() != 0600 && info.Mode().Perm() != 0400) || info.Size() > 256 {
		return nil, errors.New("quoter key file must be a private regular file with mode 0600 or 0400")
	}
	raw, err := io.ReadAll(io.LimitReader(file, 257))
	if err != nil {
		return nil, errors.New("quoter key file cannot be read")
	}
	defer func() {
		for i := range raw {
			raw[i] = 0
		}
	}()
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(string(raw)), "0x"))
	if err != nil {
		return nil, errors.New("quoter key file must hold one hexadecimal secp256k1 key")
	}
	return key, nil
}

// NewRPC validates the chain before signing and never includes endpoints or key
// material in diagnostics. Dry mode also refuses writes at the transport layer.
func NewRPC(ctx context.Context, cfg *config.Config) (*RPC, error) {
	if cfg == nil || cfg.Service != config.Quoter {
		return nil, errors.New("quoter configuration is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if venueABIError != nil || feedABIError != nil {
		return nil, errors.New("venue ABI is invalid")
	}
	key, err := readKey(cfg.Quoting.KeyFile)
	if err != nil {
		return nil, err
	}
	r := &RPC{key: key, account: crypto.PubkeyToAddress(key.PublicKey), chain: new(big.Int).SetUint64(cfg.ChainID), mode: cfg.Quoting.Mode, completed: map[string]bool{}, now: time.Now}
	var last error
	for _, endpoint := range cfg.RPCURLs {
		attempt, cancel := context.WithTimeout(ctx, requestTimeout)
		client, err := ethclient.DialContext(attempt, endpoint)
		if err == nil {
			chain, readErr := client.ChainID(attempt)
			if readErr == nil && chain.Cmp(r.chain) == 0 {
				r.client = client
				cancel()
				return r, nil
			}
			if readErr == nil {
				last = failed("chain id mismatch")
			} else {
				last = failed("chain request failed")
			}
			client.Close()
		} else {
			last = failed("chain request failed")
		}
		cancel()
	}
	return nil, last
}
func (r *RPC) Close()                  { r.client.Close() }
func (r *RPC) Account() common.Address { return r.account }
func (r *RPC) String() string          { return "perps quoter RPC" }
func (r *RPC) GoString() string        { return r.String() }
func (r *RPC) LogValue() slog.Value    { return slog.StringValue(r.String()) }

func knownPerpsReason(s string) bool { _, ok := venueABI.Errors[s]; return ok }
func rpcError(err error, transaction bool) error {
	var dataErr rpc.DataError
	if errors.As(err, &dataErr) {
		if raw, ok := dataErr.ErrorData().(string); ok {
			data, decodeErr := hex.DecodeString(strings.TrimPrefix(raw, "0x"))
			if decodeErr == nil && len(data) >= 4 {
				for name, e := range venueABI.Errors {
					if string(e.ID[:4]) == string(data[:4]) {
						return &ChainError{Reason: name, Refused: transaction}
					}
				}
			}
		}
	}
	// Classify known transport refusals without returning arbitrary remote text.
	if transaction {
		message := strings.ToLower(err.Error())
		switch {
		case strings.Contains(message, "insufficient funds"):
			return refused("insufficient gas balance")
		case strings.Contains(message, "underpriced"):
			return refused("transaction underpriced")
		case strings.Contains(message, "nonce too"):
			return refused("nonce conflict")
		case strings.Contains(message, "execution reverted"):
			return refused("transaction reverted")
		case strings.Contains(message, "intrinsic gas"), strings.Contains(message, "exceeds block gas"), strings.Contains(message, "invalid sender"):
			return refused("transaction refused")
		}
		if dataErr != nil {
			return refused("transaction reverted")
		}
	}
	return failed("chain request failed")
}

func (r *RPC) call(ctx context.Context, contract abi.ABI, to common.Address, at *big.Int, method string, args ...any) ([]any, error) {
	data, err := contract.Pack(method, args...)
	if err != nil {
		return nil, failed("invalid chain response")
	}
	output, err := r.client.CallContract(ctx, ethereum.CallMsg{From: r.account, To: &to, Data: data}, at)
	if err != nil {
		return nil, rpcError(err, false)
	}
	values, err := contract.Unpack(method, output)
	if err != nil {
		return nil, failed("invalid chain response")
	}
	return values, nil
}
func tuple[T any](values []any) *T { return abi.ConvertType(values[0], new(T)).(*T) }
func (r *RPC) venue(ctx context.Context, at *big.Int, method string, args ...any) ([]any, error) {
	return r.call(ctx, venueABI, venueAddress, at, method, args...)
}
func (r *RPC) ActiveMarkets(ctx context.Context) ([]uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	values, err := r.venue(ctx, nil, "getActiveMarketIds")
	if err != nil {
		return nil, err
	}
	var ids []uint64
	for _, n := range values[0].([]*big.Int) {
		if !n.IsUint64() {
			return nil, failed("invalid chain response")
		}
		ids = append(ids, n.Uint64())
	}
	return ids, nil
}

type marketTuple struct {
	MarketId                                          *big.Int
	Symbol, Name, OracleSymbol                        string
	SzDecimals, PxDecimals                            uint8
	TickSize, LotSize, MaxLeverage                    *big.Int
	TierClass                                         uint8
	ImpactNotionalUsd, OiCapUsd, FundingCapPpmPerHour *big.Int
	Enabled                                           bool
	ListedAt                                          uint64
}
type priceTuple struct {
	IndexPrice, MarkPrice *big.Int
	TimestampMs           uint64
	Height                int64
	Sources               uint32
	Fresh                 bool
}
type quoteTuple struct {
	IndexPrice, MarkPrice, OracleTWAP, FundingRatePerSecond, FundingRate24h, LongOI, ShortOI, MaxLeverage, MaintenanceMarginBps *big.Int
	Enabled, PriceStale                                                                                                         bool
}
type orderTuple struct {
	OrderId                                                                            *big.Int
	User                                                                               common.Address
	MarketId                                                                           *big.Int
	IsLong                                                                             bool
	OrderType, TimeInForce, Status                                                     uint8
	ReduceOnly                                                                         bool
	LimitPrice, TriggerPrice, AcceptablePrice, SizeUsd, SizeBase, FilledBase, Leverage *big.Int
	Deadline                                                                           uint64
	Cloid                                                                              [16]byte
	Sequence                                                                           uint64
}
type positionTuple struct {
	PositionId                                                                                                                                  *big.Int
	User                                                                                                                                        common.Address
	MarketId                                                                                                                                    *big.Int
	IsLong                                                                                                                                      bool
	SizeBase, SizeUsdAtMark, EntryPrice, Leverage, UnrealizedPnlUsd, FundingOwedUsd, LiquidationPrice, TakeProfitPrice, StopLossPrice, OpenedAt *big.Int
}
type accountTuple struct {
	Collateral, EquityUsd, InitialMarginUsd, MaintenanceMarginUsd, Withdrawable *big.Int
	FeeTier                                                                     uint8
	Volume14dUsd, PositionCount, OpenOrderCount                                 *big.Int
}
type summaryTuple struct{ Idle, TotalLocked, PositionCount *big.Int }

func (r *RPC) orders(ctx context.Context, at *big.Int) ([]Order, error) {
	values, err := r.venue(ctx, at, "getUserOrderIds", r.account)
	if err != nil {
		return nil, err
	}
	out := []Order{}
	for _, id := range values[0].([]*big.Int) {
		values, err := r.venue(ctx, at, "getOrderDetails", id)
		if err != nil {
			return nil, err
		}
		o := tuple[orderTuple](values)
		if o.User != r.account || !o.MarketId.IsUint64() || o.FilledBase.Cmp(o.SizeBase) > 0 {
			return nil, failed("invalid chain response")
		}
		if o.Status > 1 {
			continue
		}
		remaining := new(big.Int).Sub(o.SizeBase, o.FilledBase)
		if remaining.Sign() == 0 {
			continue
		}
		_, _, level, round, _ := decodeClientID(o.Cloid)
		out = append(out, Order{ID: clone(id), Market: o.MarketId.Uint64(), Buy: o.IsLong, Level: level, Round: round, Cloid: o.Cloid, Price: o.LimitPrice, SizeUSD: o.SizeUsd, Base: remaining})
	}
	return out, nil
}
func (r *RPC) Orders(ctx context.Context) ([]Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	if err := r.settle(ctx); err != nil {
		return nil, err
	}
	header, err := r.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, rpcError(err, false)
	}
	return r.orders(ctx, header.Number)
}
func (r *RPC) Snapshot(ctx context.Context, id uint64) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	s := Snapshot{PositionBase: new(big.Int)}
	if err := r.settle(ctx); err != nil {
		return s, err
	}
	header, err := r.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return s, rpcError(err, false)
	}
	at := header.Number
	marketID := new(big.Int).SetUint64(id)
	values, err := r.venue(ctx, at, "getMarketConfig", marketID)
	if err != nil {
		return s, err
	}
	m := tuple[marketTuple](values)
	s.Market = Market{ID: id, Tick: m.TickSize, Lot: m.LotSize, MinOrderUSD: clone(minimumOrderUSD), Enabled: m.Enabled}
	values, err = r.venue(ctx, at, "getMarkPriceDetails", marketID)
	if err != nil {
		return s, err
	}
	mark := values[0].(*big.Int)
	venueFresh := values[3].(bool)
	values, err = r.venue(ctx, at, "quoteMarket", marketID)
	if err != nil {
		return s, err
	}
	quote := tuple[quoteTuple](values)
	values, err = r.call(ctx, feedABI, feedAddress, at, "getPrice", marketID)
	if err != nil {
		return s, err
	}
	price := tuple[priceTuple](values)
	if price.TimestampMs > uint64(1<<63-1) {
		return s, failed("invalid chain response")
	}
	s.Price = Price{Mark: mark, Time: time.UnixMilli(int64(price.TimestampMs)).UTC(), Fresh: venueFresh && price.Fresh && !quote.PriceStale && quote.Enabled}
	orders, err := r.orders(ctx, at)
	if err != nil {
		return s, err
	}
	s.Orders = []Order{}
	for _, o := range orders {
		if o.Market == id {
			s.Orders = append(s.Orders, o)
		}
	}
	values, err = r.venue(ctx, at, "getUserMarketPosition", r.account, marketID)
	if err != nil {
		return s, err
	}
	if positionID := values[0].(*big.Int); positionID.Sign() != 0 {
		values, err = r.venue(ctx, at, "getPositionDetails", positionID)
		if err != nil {
			return s, err
		}
		p := tuple[positionTuple](values)
		s.PositionBase = clone(p.SizeBase)
		if !p.IsLong {
			s.PositionBase.Neg(s.PositionBase)
		}
	}
	values, err = r.venue(ctx, at, "getAccount", r.account)
	if err != nil {
		return s, err
	}
	account := tuple[accountTuple](values)
	// Select the explicit-account overload by its ABI input signature.
	summaryMethod := ""
	for name, method := range venueABI.Methods {
		if method.Sig == "getAccountSummary(address,address)" {
			summaryMethod = name
			break
		}
	}
	if summaryMethod == "" {
		return s, failed("invalid chain response")
	}
	values, err = r.venue(ctx, at, summaryMethod, r.account, collateralAddress)
	if err != nil {
		return s, err
	}
	summary := tuple[summaryTuple](values)
	free := new(big.Int).Sub(account.EquityUsd, account.InitialMarginUsd)
	if free.Sign() < 0 {
		free.SetInt64(0)
	}
	free = min(free, new(big.Int).Mul(summary.Idle, big.NewInt(1_000_000_000_000)))
	free = min(free, new(big.Int).Mul(account.Withdrawable, big.NewInt(1_000_000_000_000)))
	s.FreeUSD = free
	values, err = r.venue(ctx, at, "getFeeTier", r.account)
	if err != nil {
		return s, err
	}
	fee := values[1].(*big.Int)
	if !fee.IsUint64() {
		return s, failed("invalid chain response")
	}
	s.TakerFeePPM = fee.Uint64()
	return s, validateSnapshot(s)
}

// orderParams matches the canonical placeOrder tuple. CollateralAmount remains
// zero: this service never calls a deposit, withdrawal or token approval.
type orderParams struct {
	MarketId                                                                       *big.Int
	IsLong                                                                         bool
	OrderType, TimeInForce                                                         uint8
	SizeUsd, Leverage, LimitPrice, TriggerPrice, AcceptablePrice, CollateralAmount *big.Int
	ReduceOnly                                                                     bool
	TakeProfitPrice, StopLossPrice                                                 *big.Int
	Deadline                                                                       uint64
	Cloid                                                                          [16]byte
}

func params(o Order) orderParams {
	return orderParams{MarketId: new(big.Int).SetUint64(o.Market), IsLong: o.Buy, OrderType: 0, TimeInForce: 3, SizeUsd: o.SizeUSD, Leverage: clone(unit), LimitPrice: o.Price, TriggerPrice: new(big.Int), AcceptablePrice: new(big.Int), CollateralAmount: new(big.Int), TakeProfitPrice: new(big.Int), StopLossPrice: new(big.Int), Cloid: o.Cloid}
}
func (r *RPC) Place(ctx context.Context, o Order) error {
	data, err := venueABI.Pack("placeOrder", params(o))
	if err != nil {
		return failed("invalid chain response")
	}
	return r.send(ctx, venueAddress, data, "place:"+hex.EncodeToString(o.Cloid[:]))
}
func (r *RPC) Cancel(ctx context.Context, o Order) error {
	var data []byte
	var err error
	if o.Cloid != ([16]byte{}) {
		data, err = venueABI.Pack("cancelByCloid", new(big.Int).SetUint64(o.Market), o.Cloid)
	} else {
		data, err = venueABI.Pack("cancelOrder", o.ID)
	}
	if err != nil {
		return failed("invalid chain response")
	}
	return r.send(ctx, venueAddress, data, "cancel:"+o.ID.String())
}
func (r *RPC) remember(key string) {
	r.completed[key] = true
	r.completionOrder = append(r.completionOrder, key)
	if len(r.completionOrder) > 4096 {
		delete(r.completed, r.completionOrder[0])
		r.completionOrder = r.completionOrder[1:]
	}
}
func (r *RPC) send(ctx context.Context, to common.Address, data []byte, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.mode != "live" {
		return errors.New("dry mode sends no transactions")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	if err := r.settle(ctx); err != nil {
		return err
	}
	if r.completed[key] {
		return nil
	}
	nonce, minedNonce, err := r.nonceCounts(ctx)
	if err != nil {
		return err
	}
	if nonce != minedNonce {
		return failed("transaction pending")
	}
	gasPrice, err := r.client.SuggestGasPrice(ctx)
	if err != nil {
		return rpcError(err, false)
	}
	message := ethereum.CallMsg{From: r.account, To: &to, Data: data, GasPrice: gasPrice}
	gas, err := r.client.EstimateGas(ctx, message)
	if err != nil {
		return rpcError(err, true)
	}
	gas += gas / 5
	tx := types.NewTransaction(nonce, to, new(big.Int), gas, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(r.chain), r.key)
	if err != nil {
		return failed("transaction refused")
	}
	r.pending = &pendingTx{tx: signed, to: to, data: slicesCopy(data), key: key}
	if err := r.client.SendTransaction(ctx, signed); err != nil {
		classified := rpcError(err, true)
		if isRefusal(classified) {
			r.pending = nil
			return classified
		}
		// An interrupted response may follow an accepted broadcast. Keep the exact
		// signed transaction; a later read waits for it rather than signing another.
	}
	return r.settle(ctx)
}
func slicesCopy(data []byte) []byte { return append([]byte(nil), data...) }

// nonceCounts uses the mined count while the endpoint refuses pending reads.
// The caller holds mu; a retained signed transaction is settled separately.
func (r *RPC) nonceCounts(ctx context.Context) (uint64, uint64, error) {
	var pending uint64
	served := false
	if !r.now().Before(r.pendingTagRetryAt) {
		var err error
		pending, err = r.client.PendingNonceAt(ctx, r.account)
		if err != nil {
			var responseError rpc.Error
			if !errors.As(err, &responseError) {
				return 0, 0, rpcError(err, false)
			}
			r.pendingTagRetryAt = r.now().Add(pendingTagRetryInterval)
		} else {
			served = true
			r.pendingTagRetryAt = time.Time{}
		}
	}
	mined, err := r.client.NonceAt(ctx, r.account, nil)
	if err != nil {
		return 0, 0, rpcError(err, false)
	}
	if !served {
		pending = mined
	}
	return pending, mined, nil
}

func (r *RPC) settle(ctx context.Context) error {
	if r.pending == nil {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			pending, mined, err := r.nonceCounts(ctx)
			if err != nil {
				return err
			}
			if pending == mined {
				return nil
			}
			select {
			case <-ctx.Done():
				return failed("transaction pending")
			case <-ticker.C:
			}
		}
	}
	pending := r.pending
	rebroadcast := time.Now().Add(2 * time.Second)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		receipt, err := r.client.TransactionReceipt(ctx, pending.tx.Hash())
		if err == nil {
			r.pending = nil
			if receipt.Status != types.ReceiptStatusSuccessful {
				_, callErr := r.client.CallContract(ctx, ethereum.CallMsg{From: r.account, To: &pending.to, Data: pending.data, Gas: pending.tx.Gas(), GasPrice: pending.tx.GasPrice()}, receipt.BlockNumber)
				if callErr != nil {
					return rpcError(callErr, true)
				}
				return refused("transaction reverted")
			}
			r.remember(pending.key)
			return nil
		}
		if !errors.Is(err, ethereum.NotFound) && ctx.Err() == nil {
			return failed("transaction pending")
		}
		if time.Now().After(rebroadcast) {
			_ = r.client.SendTransaction(ctx, pending.tx)
			rebroadcast = time.Now().Add(2 * time.Second)
		}
		select {
		case <-ctx.Done():
			return failed("transaction pending")
		case <-ticker.C:
		}
	}
}
