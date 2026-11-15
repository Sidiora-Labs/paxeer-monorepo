package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEntryTimeLimitConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ttlDuration time.Duration
		ttlBlocks   int64
		wantNone    bool
	}{
		{name: "neither", wantNone: true},
		{name: "duration", ttlDuration: time.Second},
		{name: "blocks", ttlBlocks: 1},
		{name: "both", ttlDuration: time.Second, ttlBlocks: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &MempoolConfig{TTLDuration: tc.ttlDuration, TTLNumBlocks: tc.ttlBlocks}
			require.Equal(t, tc.wantNone, cfg.HasNoEntryTimeLimit())
		})
	}
}
