package config

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"time"
)

// LoadConfig stores the configuration for load-related settings.
type LoadConfig struct {
	ChainID int64 `json:"chainId,omitempty"`
	// PaxChainID is the textual chain ID used for tagging metric collection.
	PaxChainID string         `json:"paxChainID,omitempty"`
	Endpoints  []string       `json:"endpoints"`
	Accounts   *AccountConfig `json:"accounts,omitempty"`
	Scenarios  []Scenario     `json:"scenarios,omitempty"`
	MockDeploy bool           `json:"mockDeploy,omitempty"`
	Settings   *Settings      `json:"settings,omitempty"`
	Bounds     *BoundsConfig  `json:"bounds,omitempty"`
	// Path to write a JSON report of the load test. A nonempty settings.reportPath
	// overrides this path; --report-path overrides both, including an empty flag.
	ReportPath string `json:"reportPath,omitempty"`
}

// MergeReportPath supplies the top-level report path when settings has no path.
// LoadSettings and ResolveSettings subsequently apply --report-path precedence.
func MergeReportPath(cfg *LoadConfig, s *Settings) {
	if s.ReportPath == "" {
		s.ReportPath = cfg.ReportPath
	}
}

type BoundsConfig struct {
	MinAverageTPS   float64 `json:"minAverageTps,omitempty"`
	MaxP99LatencyMs int64   `json:"maxP99LatencyMs,omitempty"`
	MinTotalTxs     uint64  `json:"minTotalTxs,omitempty"`
}

// UnmarshalJSON rejects millisecond bounds that would overflow MergeBounds.
func (b *BoundsConfig) UnmarshalJSON(data []byte) error {
	type boundsJSON BoundsConfig
	var decoded boundsJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.MaxP99LatencyMs > math.MaxInt64/int64(time.Millisecond) ||
		decoded.MaxP99LatencyMs < math.MinInt64/int64(time.Millisecond) {
		return fmt.Errorf("maxP99LatencyMs is outside the time.Duration range: %d", decoded.MaxP99LatencyMs)
	}
	*b = BoundsConfig(decoded)
	return nil
}

func MergeBounds(cfg *LoadConfig, s *Settings) {
	if cfg.Bounds == nil {
		return
	}
	if s.MinTPS == 0 && cfg.Bounds.MinAverageTPS != 0 {
		s.MinTPS = cfg.Bounds.MinAverageTPS
	}
	if s.MaxP99Latency == 0 && cfg.Bounds.MaxP99LatencyMs != 0 {
		s.MaxP99Latency = Duration(time.Duration(cfg.Bounds.MaxP99LatencyMs) * time.Millisecond)
	}
	if s.MinTotalTxs == 0 && cfg.Bounds.MinTotalTxs != 0 {
		s.MinTotalTxs = cfg.Bounds.MinTotalTxs
	}
}

// Duration wraps time.Duration to provide JSON unmarshaling support
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler for Duration
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	*d = Duration(parsed)
	return nil
}

// ToDuration converts Duration back to time.Duration
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// MarshalJSON implements json.Marshaler for Duration
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// GetChainID returns the chain ID as a big.Int.
func (c *LoadConfig) GetChainID() *big.Int {
	return big.NewInt(c.ChainID)
}

// AccountConfig stores the configuration for account generation.
type AccountConfig struct {
	NewAccountRate float64 `json:"newAccountRate,omitempty"`
	Accounts       int     `json:"count,omitempty"`
}

// Scenario represents each scenario in the load configuration.
type Scenario struct {
	Name            string         `json:"name,omitempty"`
	Weight          int            `json:"weight,omitempty"`
	Accounts        *AccountConfig `json:"accounts,omitempty"`
	GasPicker       *GasPicker     `json:"gasPicker,omitempty"`
	GasFeeCapPicker *GasPicker     `json:"gasFeeCapPicker,omitempty"`
	GasTipCapPicker *GasPicker     `json:"gasTipCapPicker,omitempty"`
}
