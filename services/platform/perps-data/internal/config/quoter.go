package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

// QuoterConfig bounds a dedicated account's resting orders. USD amounts use
// the venue's 18-decimal scale; quoting always uses 1x leverage.
type QuoterConfig struct {
	KeyFile                 string        `json:"-"`
	Markets                 []uint64      `json:"markets"`
	Levels                  int           `json:"levels"`
	SpreadBPS               uint64        `json:"spreadBps"`
	StepBPS                 uint64        `json:"stepBps"`
	LevelUSD                *big.Int      `json:"levelUsd"`
	RefreshBPS              uint64        `json:"refreshBps"`
	MaxQuoteAge             time.Duration `json:"maxQuoteAge"`
	MaxPositionUSD          *big.Int      `json:"maxPositionUsd"`
	MaxPriceAge             time.Duration `json:"maxPriceAge"`
	Interval                time.Duration `json:"interval"`
	RefusalLimit            int           `json:"refusalLimit"`
	Cooldown                time.Duration `json:"cooldown"`
	MaxTransactionsPerRound int           `json:"maxTransactionsPerRound"`
	MaxOpenReads            int           `json:"maxOpenReads"`
	Mode                    string        `json:"mode"`
	Stop                    bool          `json:"stop"`
	Handover                bool          `json:"handover"`
}

const DefaultMaxOpenReads = 2

const quoterPrefix = "PERPS_QUOTER_"

// DefaultQuoterConfig is safe to inspect; KeyFile must be supplied at startup.
func DefaultQuoterConfig() QuoterConfig {
	return QuoterConfig{Levels: 2, SpreadBPS: 10, StepBPS: 10, LevelUSD: usdWhole(25),
		RefreshBPS: 5, MaxQuoteAge: 30 * time.Second, MaxPositionUSD: usdWhole(500),
		MaxPriceAge: 10 * time.Second, Interval: 2 * time.Second, RefusalLimit: 3,
		Cooldown: 30 * time.Second, MaxTransactionsPerRound: 8, MaxOpenReads: DefaultMaxOpenReads, Mode: "dry"}
}

func usdWhole(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(1_000_000_000_000_000_000))
}

// ParseUSD accepts exact decimal USD amounts, without floating-point rounding.
func ParseUSD(raw string) (*big.Int, error) {
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" {
		return nil, errors.New("must be a positive decimal USD amount with at most 18 decimal places")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" {
			return nil, errors.New("must be a decimal USD amount")
		}
	}
	if len(fraction) > 18 {
		return nil, errors.New("must have at most 18 decimal places")
	}
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return nil, errors.New("must be a decimal USD amount")
			}
		}
	}
	n, ok := new(big.Int).SetString(parts[0]+fraction+strings.Repeat("0", 18-len(fraction)), 10)
	if !ok || n.Sign() <= 0 || n.BitLen() > 128 {
		return nil, errors.New("must be a positive USD amount below 2^128 units")
	}
	return n, nil
}

func loadQuoter() (*QuoterConfig, error) {
	q := DefaultQuoterConfig()
	q.KeyFile = os.Getenv(quoterPrefix + "KEY_FILE")
	if raw := os.Getenv(quoterPrefix + "MARKETS"); raw != "" && raw != "all" {
		seen := map[uint64]bool{}
		for _, p := range strings.Split(raw, ",") {
			id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64)
			if err != nil || seen[id] {
				return nil, errors.New(quoterPrefix + "MARKETS must be all or distinct unsigned market ids")
			}
			seen[id] = true
			q.Markets = append(q.Markets, id)
		}
	}
	for _, v := range []struct {
		name   string
		target *uint64
	}{{"SPREAD_BPS", &q.SpreadBPS}, {"STEP_BPS", &q.StepBPS}, {"REFRESH_BPS", &q.RefreshBPS}} {
		if raw, set := os.LookupEnv(quoterPrefix + v.name); set {
			n, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s%s must be an unsigned integer", quoterPrefix, v.name)
			}
			*v.target = n
		}
	}
	for _, v := range []struct {
		name   string
		target *int
	}{{"LEVELS", &q.Levels}, {"REFUSAL_LIMIT", &q.RefusalLimit}, {"MAX_TRANSACTIONS_PER_ROUND", &q.MaxTransactionsPerRound}, {"MAX_OPEN_READS", &q.MaxOpenReads}} {
		if raw, set := os.LookupEnv(quoterPrefix + v.name); set {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("%s%s must be an integer", quoterPrefix, v.name)
			}
			*v.target = n
		}
	}
	for _, v := range []struct {
		name   string
		target **big.Int
	}{{"LEVEL_USD", &q.LevelUSD}, {"MAX_POSITION_USD", &q.MaxPositionUSD}} {
		if raw, set := os.LookupEnv(quoterPrefix + v.name); set {
			n, err := ParseUSD(raw)
			if err != nil {
				return nil, fmt.Errorf("%s%s %w", quoterPrefix, v.name, err)
			}
			*v.target = n
		}
	}
	for _, v := range []struct {
		name   string
		target *time.Duration
	}{{"MAX_QUOTE_AGE_SECONDS", &q.MaxQuoteAge}, {"MAX_PRICE_AGE_SECONDS", &q.MaxPriceAge}, {"INTERVAL_SECONDS", &q.Interval}, {"COOLDOWN_SECONDS", &q.Cooldown}} {
		if raw, set := os.LookupEnv(quoterPrefix + v.name); set {
			n, err := strconv.ParseUint(raw, 10, 32)
			if err != nil || n == 0 || n > 86400 {
				return nil, fmt.Errorf("%s%s must be in [1,86400] seconds", quoterPrefix, v.name)
			}
			*v.target = time.Duration(n) * time.Second
		}
	}
	if raw, set := os.LookupEnv(quoterPrefix + "MODE"); set {
		q.Mode = raw
	}
	for _, v := range []struct {
		name   string
		target *bool
	}{{"STOP", &q.Stop}, {"HANDOVER", &q.Handover}} {
		if raw, set := os.LookupEnv(quoterPrefix + v.name); set {
			if raw != "true" && raw != "false" {
				return nil, fmt.Errorf("%s%s must be true or false", quoterPrefix, v.name)
			}
			*v.target = raw == "true"
		}
	}
	return &q, q.Validate()
}

func (q QuoterConfig) Validate() error {
	if strings.TrimSpace(q.KeyFile) == "" {
		return errors.New(quoterPrefix + "KEY_FILE is required")
	}
	if q.Mode != "dry" && q.Mode != "live" {
		return errors.New(quoterPrefix + "MODE must be dry or live")
	}
	if q.Levels < 1 || q.Levels > 16 {
		return errors.New(quoterPrefix + "LEVELS must be in [1,16]")
	}
	if q.SpreadBPS < 1 || q.SpreadBPS >= 10000 || q.StepBPS < 1 || q.StepBPS >= 10000 || q.SpreadBPS+uint64(q.Levels-1)*q.StepBPS >= 10000 {
		return errors.New("quoter ladder distances must be in [1,9999] basis points")
	}
	if q.RefreshBPS < 1 || q.RefreshBPS >= 10000 {
		return errors.New(quoterPrefix + "REFRESH_BPS must be in [1,9999]")
	}
	for _, n := range []*big.Int{q.LevelUSD, q.MaxPositionUSD} {
		if n == nil || n.Sign() <= 0 || n.BitLen() > 128 {
			return errors.New("quoter USD bounds must be positive and below 2^128 units")
		}
	}
	for _, d := range []time.Duration{q.MaxQuoteAge, q.MaxPriceAge, q.Interval, q.Cooldown} {
		if d < time.Second || d > 24*time.Hour || d%time.Second != 0 {
			return errors.New("quoter durations must be whole seconds in [1,86400]")
		}
	}
	if q.RefusalLimit < 1 || q.RefusalLimit > 100 {
		return errors.New(quoterPrefix + "REFUSAL_LIMIT must be in [1,100]")
	}
	if q.MaxTransactionsPerRound < 1 || q.MaxTransactionsPerRound > 256 {
		return errors.New(quoterPrefix + "MAX_TRANSACTIONS_PER_ROUND must be in [1,256]")
	}
	if q.MaxOpenReads < 1 || q.MaxOpenReads > 256 {
		return errors.New(quoterPrefix + "MAX_OPEN_READS must be in [1,256]")
	}
	seen := map[uint64]bool{}
	for _, id := range q.Markets {
		if seen[id] {
			return errors.New("quoter market ids must be distinct")
		}
		seen[id] = true
	}
	return nil
}

func (q QuoterConfig) String() string {
	type public QuoterConfig
	raw, _ := json.Marshal(public(q))
	return string(raw)
}
func (q QuoterConfig) GoString() string     { return q.String() }
func (q QuoterConfig) LogValue() slog.Value { return slog.StringValue(q.String()) }
