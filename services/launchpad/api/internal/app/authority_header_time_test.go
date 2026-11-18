package app

import (
	"math"
	"testing"
	"testing/synctest"
	"time"
)

func TestRecentAuthorityHeaderTimestampBounds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, reader, _ := newAuthorityReadinessState(t)
		now := uint64(time.Now().Unix())
		for _, tc := range []struct {
			name      string
			timestamp uint64
			accepted  bool
		}{
			{"current", now, true},
			{"recent", now - 1, true},
			{"age boundary", now - uint64(authorityRecentHeadMaxAge/time.Second), true},
			{"stale", now - uint64(authorityRecentHeadMaxAge/time.Second) - 1, false},
			{"future", now + 1, false},
			{"zero", 0, false},
			{"largest signed timestamp", math.MaxInt64, false},
			{"above signed range", uint64(math.MaxInt64) + 1, false},
			{"largest unsigned timestamp", math.MaxUint64, false},
		} {
			reader.header.Time = tc.timestamp
			hash := reader.header.Hash()
			header, err := recentAuthorityHeader(t.Context(), reader, hash)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if (header != nil) != tc.accepted {
				t.Fatalf("%s: timestamp %d accepted = %t, want %t", tc.name, tc.timestamp, header != nil, tc.accepted)
			}
			if header != nil && (header.Hash() != hash || header.Time != tc.timestamp || header.Number.Cmp(reader.header.Number) != 0) {
				t.Fatalf("%s: accepted header differs from the committed block: %+v", tc.name, header)
			}
		}
	})
}
