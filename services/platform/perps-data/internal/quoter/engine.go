package quoter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
)

type RestingOrder struct {
	ID       string `json:"id"`
	ClientID string `json:"clientId"`
	Side     string `json:"side"`
	Price    string `json:"price"`
	Base     string `json:"sizeBase"`
}
type MarketHealth struct {
	Market      uint64         `json:"market"`
	Orders      []RestingOrder `json:"restingOrders"`
	Position    string         `json:"positionBase"`
	LastRefusal string         `json:"lastRefusal,omitempty"`
	Refusals    uint64         `json:"refusals"`
	Consecutive int            `json:"consecutiveRefusals"`
	RestUntil   time.Time      `json:"restUntil,omitempty"`
	LastError   string         `json:"lastError,omitempty"`
	LastRound   time.Time      `json:"lastRound"`
}
type Health struct {
	Mode                  string         `json:"mode"`
	LastRound             time.Time      `json:"lastRound"`
	Markets               []MarketHealth `json:"markets"`
	Stopped               bool           `json:"stopped"`
	OrdersLeft            int            `json:"ordersLeft"`
	TransactionsLastRound int            `json:"transactionsLastRound"`
}
type marketRound struct {
	anchor *big.Int
	born   time.Time
	round  uint64
	seen   map[[16]byte]bool
	health MarketHealth
}

// Engine serializes rounds and shutdown while allowing concurrent health reads.
type Engine struct {
	chain      Chain
	cfg        config.QuoterConfig
	log        *slog.Logger
	work       sync.Mutex
	mu         sync.RWMutex
	health     Health
	states     map[uint64]*marketRound
	generation uint64
}

func New(chain Chain, q config.QuoterConfig, logger *slog.Logger) (*Engine, error) {
	if chain == nil || logger == nil {
		return nil, errors.New("quoter requires a chain and a logger")
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	q.Markets = slices.Clone(q.Markets)
	q.LevelUSD = clone(q.LevelUSD)
	q.MaxPositionUSD = clone(q.MaxPositionUSD)
	return &Engine{chain: chain, cfg: q, log: logger, states: map[uint64]*marketRound{}, health: Health{Mode: q.Mode, Markets: []MarketHealth{}, OrdersLeft: -1}}, nil
}
func (e *Engine) state(id uint64) *marketRound {
	s := e.states[id]
	if s == nil {
		s = &marketRound{health: MarketHealth{Market: id, Orders: []RestingOrder{}, Position: "0"}}
		e.states[id] = s
	}
	return s
}
func (e *Engine) renew(s *marketRound, price *big.Int, now time.Time) {
	generation := uint64(now.UnixMilli())
	if generation <= e.generation {
		generation = e.generation + 1
	}
	e.generation = generation
	s.anchor = clone(price)
	s.born = now
	s.round = generation
	s.seen = map[[16]byte]bool{}
}
func (e *Engine) publish(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.health.LastRound = now
	e.health.Markets = nil
	ids := make([]uint64, 0, len(e.states))
	for id := range e.states {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		h := e.states[id].health
		h.Orders = slices.Clone(h.Orders)
		e.health.Markets = append(e.health.Markets, h)
	}
}
func (e *Engine) observe(s *marketRound, snap Snapshot, now time.Time) {
	s.health.Position = snap.PositionBase.String()
	s.health.LastRound = now
	s.health.Orders = []RestingOrder{}
	for _, o := range snap.Orders {
		side := "sell"
		if o.Buy {
			side = "buy"
		}
		s.health.Orders = append(s.health.Orders, RestingOrder{ID: o.ID.String(), ClientID: hex.EncodeToString(o.Cloid[:]), Side: side, Price: o.Price.String(), Base: o.Base.String()})
	}
}
func (e *Engine) record(s *marketRound, err error, now time.Time) {
	r := reason(err)
	s.health.LastError = r
	if isRefusal(err) {
		s.health.Refusals++
		s.health.Consecutive++
		s.health.LastRefusal = r
		if s.health.Consecutive >= e.cfg.RefusalLimit {
			s.health.RestUntil = now.Add(e.cfg.Cooldown)
		}
	}
	e.log.Warn("quoter market request refused", "market", s.health.Market, "reason", r, "refusals", s.health.Refusals, "consecutive", s.health.Consecutive)
}
func fresh(s Snapshot, now time.Time, maxAge time.Duration) bool {
	return s.Market.Enabled && s.Price.Fresh && valid(s.Price.Mark) && !s.Price.Time.IsZero() && !s.Price.Time.After(now) && now.Sub(s.Price.Time) <= maxAge
}

// Round reads the account on every market and before every live placement.
// The result is the planned ladder, including orders already resting.
func (e *Engine) Round(ctx context.Context, now time.Time) (map[uint64][]Order, error) {
	e.work.Lock()
	defer e.work.Unlock()
	started := time.Now()
	transactions := 0
	defer func() {
		e.mu.Lock()
		e.health.TransactionsLastRound = transactions
		e.mu.Unlock()
		e.log.Info("quoter round", "mode", e.cfg.Mode, "transactions", transactions)
	}()
	e.mu.RLock()
	stopped := e.health.Stopped
	e.mu.RUnlock()
	if stopped {
		return nil, errors.New("quoter is stopped")
	}
	ids := slices.Clone(e.cfg.Markets)
	var result error
	if len(ids) == 0 {
		var err error
		ids, err = e.chain.ActiveMarkets(ctx)
		if err != nil {
			return nil, failed(reason(err))
		}
	}
	// Include markets removed from the active set so their orders are withdrawn.
	all, err := e.chain.Orders(ctx)
	if err != nil {
		return nil, failed(reason(err))
	}
	for _, o := range all {
		if !slices.Contains(ids, o.Market) {
			ids = append(ids, o.Market)
		}
	}
	plans := map[uint64][]Order{}
	drySpent := new(big.Int)
	for _, id := range ids {
		state := e.state(id)
		snap, err := e.chain.Snapshot(ctx, id)
		if err == nil {
			err = validateSnapshot(snap)
		}
		if err != nil {
			e.record(state, err, now)
			// With unreadable risk or price, only cancellations are permitted.
			cancelErr := e.cancelMarket(ctx, id, now, &transactions)
			result = errors.Join(result, failed(reason(err)), cancelErr)
			continue
		}
		e.observe(state, snap, now)
		state.health.LastError = ""
		selected := len(e.cfg.Markets) == 0 || slices.Contains(e.cfg.Markets, id)
		cooling := now.Before(state.health.RestUntil)
		if !cooling && !state.health.RestUntil.IsZero() {
			state.health.RestUntil = time.Time{}
			state.health.Consecutive = 0
			state.anchor = nil
		}
		var wanted []Order
		if selected && fresh(snap, now.Add(time.Since(started)), e.cfg.MaxPriceAge) && !cooling {
			renew := state.anchor == nil || now.Sub(state.born) >= e.cfg.MaxQuoteAge
			if !renew {
				renew = priceMoved(state.anchor, snap.Price.Mark, e.cfg.RefreshBPS)
			}
			if !renew {
				for cloid := range state.seen {
					found := false
					for _, o := range snap.Orders {
						if o.Cloid == cloid {
							found = true
							break
						}
					}
					if !found {
						renew = true
						break
					}
				}
				for _, o := range snap.Orders {
					if o.Round != state.round {
						continue
					}
					for _, target := range Ladder(snap.Market, state.anchor, e.cfg, state.round) {
						if o.Cloid == target.Cloid && !sameOrder(o, target) {
							renew = true
						}
					}
				}
			}
			if renew {
				e.renew(state, snap.Price.Mark, now)
			}
			wanted = withinCap(Ladder(snap.Market, state.anchor, e.cfg, state.round), snap.PositionBase, snap.Price.Mark, e.cfg.MaxPositionUSD)
		} else {
			state.anchor = nil
		}
		plans[id] = wanted
		var kept []Order
		cancelFailed := false
		for _, actual := range snap.Orders {
			keep := false
			for _, target := range wanted {
				if sameOrder(actual, target) {
					keep = true
					state.seen[target.Cloid] = true
					break
				}
			}
			if keep {
				kept = append(kept, actual)
				continue
			}
			e.log.Info("quoter cancel", "mode", e.cfg.Mode, "order", orderText(actual))
			if e.cfg.Mode == "live" {
				if transactions >= e.cfg.MaxTransactionsPerRound {
					cancelFailed = true
					continue
				}
				transactions++
				if err := e.chain.Cancel(ctx, actual); err != nil {
					e.record(state, err, now)
					result = errors.Join(result, failed(reason(err)))
					cancelFailed = true
				} else {
					state.health.Consecutive = 0
				}
			}
		}
		if cancelFailed {
			current, readErr := e.chain.Snapshot(ctx, id)
			if readErr == nil {
				readErr = validateSnapshot(current)
			}
			if readErr != nil {
				e.record(state, readErr, now)
				result = errors.Join(result, failed(reason(readErr)))
			} else {
				e.observe(state, current, now)
			}
			continue
		}
		budget := new(big.Int).Sub(snap.FreeUSD, drySpent)
		for _, target := range wanted {
			found := false
			for _, o := range kept {
				if sameOrder(o, target) {
					found = true
					break
				}
			}
			if found {
				continue
			}
			if e.cfg.Mode == "live" && transactions >= e.cfg.MaxTransactionsPerRound {
				break
			}
			current := snap
			current.Orders = kept
			if e.cfg.Mode == "live" {
				current, err = e.chain.Snapshot(ctx, id)
				if err == nil {
					err = validateSnapshot(current)
				}
				if err != nil {
					e.record(state, err, now)
					result = errors.Join(result, failed(reason(err)))
					break
				}
				budget = clone(current.FreeUSD)
				if !fresh(current, now.Add(time.Since(started)), e.cfg.MaxPriceAge) {
					result = errors.Join(result, e.cancelMarket(ctx, id, now, &transactions))
					state.anchor = nil
					break
				}
			}
			if !fitsPosition(current, target, e.cfg.MaxPositionUSD) || !noSelfCross(target, current.Orders) {
				continue
			}
			reserve := reserveUSD(target, current.Price.Mark, current.TakerFeePPM)
			if budget.Cmp(reserve) < 0 {
				e.log.Info("quoter level omitted", "market", id, "level", target.Level, "reason", "free collateral bound")
				continue
			}
			e.log.Info("quoter place", "mode", e.cfg.Mode, "order", orderText(target))
			if e.cfg.Mode == "live" {
				transactions++
				if err := e.chain.Place(ctx, target); err != nil {
					e.record(state, err, now)
					result = errors.Join(result, failed(reason(err)))
					break
				}
				state.seen[target.Cloid] = true
				state.health.Consecutive = 0
			} else {
				budget.Sub(budget, reserve)
				drySpent.Add(drySpent, reserve)
			}
			kept = append(kept, target)
		}
		if now.Before(state.health.RestUntil) && e.cfg.Mode == "live" {
			result = errors.Join(result, e.cancelMarket(ctx, id, now, &transactions))
		}
		// Report actual chain orders, including dry mode's unchanged book.
		current, err := e.chain.Snapshot(ctx, id)
		if err == nil {
			err = validateSnapshot(current)
		}
		if err != nil {
			e.record(state, err, now)
			result = errors.Join(result, failed(reason(err)))
		} else {
			e.observe(state, current, now)
		}
	}
	e.publish(now)
	return plans, result
}

func fitsPosition(s Snapshot, target Order, capUSD *big.Int) bool {
	capBase := new(big.Int).Quo(new(big.Int).Mul(capUSD, unit), s.Price.Mark)
	exposure := clone(s.PositionBase)
	if !target.Buy {
		exposure.Neg(exposure)
	}
	exposure.Add(exposure, target.Base)
	for _, o := range s.Orders {
		if o.Buy == target.Buy {
			exposure.Add(exposure, o.Base)
		}
	}
	return exposure.Cmp(capBase) <= 0
}

func (e *Engine) cancelMarket(ctx context.Context, id uint64, now time.Time, transactions *int) error {
	orders, err := e.chain.Orders(ctx)
	if err != nil {
		return failed(reason(err))
	}
	var result error
	for _, o := range orders {
		if o.Market != id {
			continue
		}
		e.log.Info("quoter cancel", "mode", e.cfg.Mode, "order", orderText(o))
		if e.cfg.Mode == "live" {
			if *transactions >= e.cfg.MaxTransactionsPerRound {
				break
			}
			*transactions++
			if err := e.chain.Cancel(ctx, o); err != nil {
				e.record(e.state(id), err, now)
				result = errors.Join(result, failed(reason(err)))
			}
		}
	}
	return result
}

// Stop ignores selection, staleness and cooldown. A successful live shutdown
// requires an independent read showing zero orders across the whole account.
func (e *Engine) Stop(ctx context.Context) (StopResult, error) {
	e.work.Lock()
	defer e.work.Unlock()
	result := StopResult{OrdersLeft: -1}
	var last error
	for pass := 0; pass < 3 && ctx.Err() == nil; pass++ {
		orders, err := e.chain.Orders(ctx)
		if err != nil {
			last = failed(reason(err))
			continue
		}
		result.OrdersLeft = len(orders)
		if len(orders) == 0 {
			last = nil
			break
		}
		for _, o := range orders {
			e.log.Info("quoter stop cancel", "mode", e.cfg.Mode, "order", orderText(o))
			if e.cfg.Mode == "live" {
				if err := e.chain.Cancel(ctx, o); err != nil {
					last = failed(reason(err))
					e.record(e.state(o.Market), err, time.Now())
				}
			}
		}
		if e.cfg.Mode == "dry" {
			break
		}
	}
	if orders, err := e.chain.Orders(ctx); err != nil {
		result.OrdersLeft = -1
		last = failed(reason(err))
	} else {
		result.OrdersLeft = len(orders)
		if len(orders) == 0 {
			last = nil
		}
	}
	if result.OrdersLeft != 0 {
		last = errors.Join(last, ErrOrdersRemain)
	}
	e.publish(time.Now())
	e.mu.Lock()
	e.health.Stopped = true
	e.health.OrdersLeft = result.OrdersLeft
	if result.OrdersLeft == 0 {
		for i := range e.health.Markets {
			e.health.Markets[i].Orders = []RestingOrder{}
		}
	}
	e.mu.Unlock()
	e.log.Info("quoter stopped", "mode", e.cfg.Mode, "ordersLeft", result.OrdersLeft)
	return result, last
}

func (e *Engine) Health() Health {
	e.mu.RLock()
	defer e.mu.RUnlock()
	h := e.health
	h.Markets = slices.Clone(h.Markets)
	for i := range h.Markets {
		h.Markets[i].Orders = slices.Clone(h.Markets[i].Orders)
	}
	return h
}
func (e *Engine) Handler() http.Handler {
	mux := http.NewServeMux()
	handler := func(w http.ResponseWriter, r *http.Request) {
		h := e.Health()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/healthz/ready" && (h.Stopped || h.LastRound.IsZero() || time.Since(h.LastRound) > 2*e.cfg.Interval+time.Minute) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(h)
	}
	mux.HandleFunc("GET /healthz", handler)
	mux.HandleFunc("GET /healthz/live", handler)
	mux.HandleFunc("GET /healthz/ready", handler)
	return mux
}

// Run checks the stop variable before each round and uses a fresh context for
// cancellation after the signal context has ended.
func (e *Engine) Run(ctx context.Context, stop func() bool) error {
	ticker := time.NewTicker(e.cfg.Interval)
	defer ticker.Stop()
	for ctx.Err() == nil && !e.cfg.Stop && (stop == nil || !stop()) {
		if _, err := e.Round(ctx, time.Now().UTC()); err != nil {
			e.log.Warn("quoter round incomplete", "reason", reason(err))
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, err := e.Stop(cleanup)
	return err
}
