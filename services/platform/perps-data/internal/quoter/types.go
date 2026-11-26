// Package quoter reconciles bounded post-only limits for a dedicated account.
package quoter

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"
)

var unit = big.NewInt(1_000_000_000_000_000_000)
var ppm = big.NewInt(1_000_000)

// MinimumOrderUSD is the supported venue's minimum notional, in 18-decimal USD.
var minimumOrderUSD = new(big.Int).Mul(big.NewInt(10), unit)

// Market carries the live venue tick and base lot, both in 18-decimal units.
type Market struct {
	ID                     uint64
	Tick, Lot, MinOrderUSD *big.Int
	Enabled                bool
}
type Price struct {
	Mark  *big.Int
	Time  time.Time
	Fresh bool
}

// Order contains the original notional and the remaining base size of an order.
type Order struct {
	ID                   *big.Int
	Market               uint64
	Buy                  bool
	Level                int
	Round                uint64
	Cloid                [16]byte
	Price, SizeUSD, Base *big.Int
}

type Snapshot struct {
	Market       Market
	Price        Price
	Orders       []Order
	PositionBase *big.Int // signed: a long is positive
	FreeUSD      *big.Int // bounded by equity less margin, idle and withdrawable USDL
	TakerFeePPM  uint64
}

// Chain reads consistent snapshots and confirms writes before returning success.
// Orders includes every open order of the account, including unselected markets.
type Chain interface {
	ActiveMarkets(context.Context) ([]uint64, error)
	Snapshot(context.Context, uint64) (Snapshot, error)
	Orders(context.Context) ([]Order, error)
	Place(context.Context, Order) error
	Cancel(context.Context, Order) error
}

// ChainError exposes only product reason codes. Transport errors and arbitrary
// remote error text never enter logs or health responses.
type ChainError struct {
	Reason  string
	Refused bool
}

func (e *ChainError) Error() string { return e.Reason }

func reason(err error) string {
	var e *ChainError
	if errors.As(err, &e) && (safeReasons[e.Reason] || knownPerpsReason(e.Reason)) {
		return e.Reason
	}
	return "chain request failed"
}

var safeReasons = map[string]bool{
	"chain request failed": true, "chain id mismatch": true, "invalid chain response": true,
	"transaction pending": true, "transaction reverted": true, "insufficient gas balance": true,
	"transaction underpriced": true, "nonce conflict": true, "transaction refused": true,
}

func failed(text string) error  { return &ChainError{Reason: text} }
func refused(text string) error { return &ChainError{Reason: text, Refused: true} }
func isRefusal(err error) bool  { var e *ChainError; return errors.As(err, &e) && e.Refused }

// StopResult counts chain orders after cancellation; -1 means the read failed.
type StopResult struct {
	OrdersLeft int `json:"ordersLeft"`
}

var ErrOrdersRemain = errors.New("account still has open orders")

func clone(n *big.Int) *big.Int { return new(big.Int).Set(n) }
func min(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return clone(a)
	}
	return clone(b)
}
func ceilDiv(a, b *big.Int) *big.Int {
	return new(big.Int).Quo(new(big.Int).Add(a, new(big.Int).Sub(b, big.NewInt(1))), b)
}
func valid(n *big.Int) bool { return n != nil && n.Sign() > 0 && n.BitLen() <= 256 }

func validateSnapshot(s Snapshot) error {
	if !valid(s.Market.Tick) || !valid(s.Market.Lot) || !valid(s.Market.MinOrderUSD) || s.PositionBase == nil || s.FreeUSD == nil || s.FreeUSD.Sign() < 0 || s.TakerFeePPM > 1_000_000 {
		return failed("invalid chain response")
	}
	for _, o := range s.Orders {
		if o.ID == nil || !valid(o.Base) || o.Price == nil || o.Price.Sign() < 0 {
			return failed("invalid chain response")
		}
	}
	return nil
}

func orderText(o Order) string {
	side := "sell"
	if o.Buy {
		side = "buy"
	}
	return fmt.Sprintf("market=%d side=%s level=%d round=%d cloid=%x price=%s sizeUsd=%s sizeBase=%s", o.Market, side, o.Level, o.Round, o.Cloid, o.Price, o.SizeUSD, o.Base)
}
