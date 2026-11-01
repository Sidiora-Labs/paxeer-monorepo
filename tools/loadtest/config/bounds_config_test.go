package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigBoundsMergeIntoSettings(t *testing.T) {
	const profile = `{"bounds":{"minAverageTps":400,"maxP99LatencyMs":250,"minTotalTxs":5000}}`
	var cfg LoadConfig
	require.NoError(t, json.Unmarshal([]byte(profile), &cfg))
	require.Equal(t, &BoundsConfig{
		MinAverageTPS:   400,
		MaxP99LatencyMs: 250,
		MinTotalTxs:     5000,
	}, cfg.Bounds)

	tests := []struct {
		name     string
		profile  string
		settings Settings
		want     Settings
	}{
		{
			name:    "profile fills zero settings",
			profile: profile,
			want:    Settings{MinTPS: 400, MaxP99Latency: Duration(250 * time.Millisecond), MinTotalTxs: 5000},
		},
		{
			name:     "all flags take precedence",
			profile:  profile,
			settings: Settings{MinTPS: 600, MaxP99Latency: Duration(100 * time.Millisecond), MinTotalTxs: 7000},
			want:     Settings{MinTPS: 600, MaxP99Latency: Duration(100 * time.Millisecond), MinTotalTxs: 7000},
		},
		{
			name:     "TPS flag takes precedence independently",
			profile:  profile,
			settings: Settings{MinTPS: 600, Workers: 3},
			want:     Settings{MinTPS: 600, MaxP99Latency: Duration(250 * time.Millisecond), MinTotalTxs: 5000, Workers: 3},
		},
		{
			name:     "latency flag takes precedence independently",
			profile:  profile,
			settings: Settings{MaxP99Latency: Duration(100 * time.Millisecond)},
			want:     Settings{MinTPS: 400, MaxP99Latency: Duration(100 * time.Millisecond), MinTotalTxs: 5000},
		},
		{
			name:     "transaction flag takes precedence independently",
			profile:  profile,
			settings: Settings{MinTotalTxs: 7000},
			want:     Settings{MinTPS: 400, MaxP99Latency: Duration(250 * time.Millisecond), MinTotalTxs: 7000},
		},
		{
			name:    "profile with only TPS and latency",
			profile: `{"bounds":{"minAverageTps":400,"maxP99LatencyMs":250}}`,
			want:    Settings{MinTPS: 400, MaxP99Latency: Duration(250 * time.Millisecond)},
		},
		{
			name:     "zero profile bounds preserve flags",
			profile:  `{"bounds":{"minAverageTps":0,"maxP99LatencyMs":0,"minTotalTxs":0}}`,
			settings: Settings{MinTPS: 600, MaxP99Latency: Duration(100 * time.Millisecond), MinTotalTxs: 7000},
			want:     Settings{MinTPS: 600, MaxP99Latency: Duration(100 * time.Millisecond), MinTotalTxs: 7000},
		},
		{
			name:    "empty bounds leave settings zero",
			profile: `{"bounds":{}}`,
		},
		{
			name:     "profile without bounds preserves settings",
			profile:  `{"endpoints":["http://localhost:8545"],"settings":{"workers":3}}`,
			settings: Settings{Workers: 3, MinTPS: 600},
			want:     Settings{Workers: 3, MinTPS: 600},
		},
		{
			name:    "null bounds leave settings zero",
			profile: `{"bounds":null}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg LoadConfig
			require.NoError(t, json.Unmarshal([]byte(tt.profile), &cfg))
			MergeBounds(&cfg, &tt.settings)
			require.Equal(t, tt.want, tt.settings)
		})
	}
}
