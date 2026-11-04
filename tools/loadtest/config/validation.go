package config

import (
	"fmt"
	"math"
	"net/url"
	"strings"
)

// Admission limits bound allocation before account generation and worker startup.
const (
	MaxAccounts                 = 100_000
	MaxEndpoints                = 256
	MaxScenarios                = 256
	MaxWorkers                  = 1024
	MaxParticipants             = 4096
	MaxBufferSize               = 100_000
	MaxQueuedTransactions       = 1_000_000
	MaxScenarioWeight     int64 = 1<<31 - 1
)

func validateAccounts(name string, a *AccountConfig) error {
	if a == nil {
		return fmt.Errorf("%s: account configuration is required", name)
	}
	if math.IsNaN(a.NewAccountRate) || math.IsInf(a.NewAccountRate, 0) || a.NewAccountRate < 0 || a.NewAccountRate > 1 {
		return fmt.Errorf("%s.newAccountRate: must be finite and in [0,1]", name)
	}
	if a.Accounts < 0 || a.Accounts > MaxAccounts {
		return fmt.Errorf("%s.count: must be in [0,%d]", name, MaxAccounts)
	}
	if a.NewAccountRate < 1 && a.Accounts == 0 {
		return fmt.Errorf("%s.count: fixed pool must be positive", name)
	}
	return nil
}

// Validate admits profile structure without constructing generators or services.
// Live endpoint identity is checked by the executable's eth_chainId preflight.
func (c *LoadConfig) Validate() error {
	if c.ChainID <= 0 {
		return fmt.Errorf("chainId: must be positive")
	}
	if len(c.Endpoints) == 0 || len(c.Endpoints) > MaxEndpoints {
		return fmt.Errorf("endpoints: require 1..%d entries", MaxEndpoints)
	}
	for i, endpoint := range c.Endpoints {
		u, err := url.Parse(endpoint)
		if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("endpoints[%d]: require an http(s) JSON-RPC URL without user information or fragment", i)
		}
	}
	if len(c.Scenarios) == 0 || len(c.Scenarios) > MaxScenarios {
		return fmt.Errorf("scenarios: require 1..%d entries", MaxScenarios)
	}
	totalAccounts := 0
	if c.Accounts != nil {
		if err := validateAccounts("accounts", c.Accounts); err != nil {
			return err
		}
		totalAccounts = c.Accounts.Accounts
	}
	var total int64
	for i, s := range c.Scenarios {
		prefix := fmt.Sprintf("scenarios[%d]", i)
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("%s.name: must not be empty", prefix)
		}
		if s.Weight < 0 || int64(s.Weight) > MaxScenarioWeight-total {
			return fmt.Errorf("%s.weight: nonnegative weights must total at most %d", prefix, MaxScenarioWeight)
		}
		total += int64(s.Weight)
		a := c.Accounts
		if s.Accounts != nil {
			a = s.Accounts
			totalAccounts += a.Accounts
		}
		if err := validateAccounts(prefix+".accounts", a); err != nil {
			return err
		}
		if totalAccounts > MaxAccounts {
			return fmt.Errorf("accounts.count: combined pools exceed %d", MaxAccounts)
		}
		// Defaults match the production transaction options, without sampling ranges.
		gasMin, _, err := pickerRange(s.GasPicker, 21_000, prefix+".gasPicker", true)
		if err != nil {
			return err
		}
		feeMin, _, err := pickerRange(s.GasFeeCapPicker, 20_000_000_000, prefix+".gasFeeCapPicker", false)
		if err != nil {
			return err
		}
		_, tipMax, err := pickerRange(s.GasTipCapPicker, 2_000_000_000, prefix+".gasTipCapPicker", false)
		if err != nil {
			return err
		}
		if gasMin == 0 {
			return fmt.Errorf("%s.gasPicker: gas must be positive", prefix)
		}
		if feeMin < tipMax {
			return fmt.Errorf("%s.gasFeeCapPicker: minimum fee cap must cover maximum tip cap", prefix)
		}
	}
	if total == 0 {
		return fmt.Errorf("scenarios.weight: total must be positive")
	}
	return nil
}

func pickerRange(p *GasPicker, fallback uint64, name string, positive bool) (uint64, uint64, error) {
	if p == nil {
		return fallback, fallback, nil
	}
	var lo, hi uint64
	switch g := p.delegate.(type) {
	case *FixedGasGenerator:
		lo, hi = g.Gas, g.Gas
	case *RandomGasGenerator:
		lo, hi = g.Min, g.Max
		if lo >= hi {
			return 0, 0, fmt.Errorf("%s: min must be less than max", name)
		}
	default:
		return 0, 0, fmt.Errorf("%s: require a fixed or random picker", name)
	}
	if positive && lo == 0 {
		return 0, 0, fmt.Errorf("%s: gas must be positive", name)
	}
	return lo, hi, nil
}

// Validate checks resolved settings, after profile and explicit flag precedence.
func (s Settings) Validate(endpoints int) error {
	if s.Workers <= 0 || s.Workers > MaxWorkers || endpoints > MaxParticipants/s.Workers {
		return fmt.Errorf("workers: require 1..%d and at most %d participants", MaxWorkers, MaxParticipants)
	}
	if s.BufferSize <= 0 || s.BufferSize > MaxBufferSize || endpoints > MaxQueuedTransactions/s.BufferSize {
		return fmt.Errorf("bufferSize: require 1..%d and at most %d queued transactions", MaxBufferSize, MaxQueuedTransactions)
	}
	if math.IsNaN(s.TPS) || math.IsInf(s.TPS, 0) || s.TPS < 0 {
		return fmt.Errorf("tps: must be finite and nonnegative")
	}
	if s.StatsInterval <= 0 {
		return fmt.Errorf("statsInterval: must be positive")
	}
	if math.IsNaN(s.MinTPS) || math.IsInf(s.MinTPS, 0) || s.MinTPS < 0 {
		return fmt.Errorf("minTps: must be finite and nonnegative")
	}
	// A negative latency bound is retained as an explicit always-failing bound,
	// as supported by the existing bounds contract; it is never used as a timer.
	if s.TxsDir != "" {
		if s.TargetGas == 0 {
			return fmt.Errorf("targetGas: must be positive in transaction-file mode")
		}
		if s.NumBlocksToWrite <= 0 {
			return fmt.Errorf("numBlocksToWrite: must be positive in transaction-file mode")
		}
	}
	return nil
}
