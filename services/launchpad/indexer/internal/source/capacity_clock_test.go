package source

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCapacityClockSelectsFirstBoundedSample(t *testing.T) {
	base := time.Unix(1700000000, 0).UTC()
	slow := capacityClockReading{base, base.Add(time.Second), base.Add(5*time.Millisecond + time.Nanosecond)}
	good := capacityClockReading{base.Add(time.Second), base.Add(2 * time.Second), base.Add(time.Second + 5*time.Millisecond)}
	want := CapacityClockSample{good.after, int64(995 * time.Millisecond), int64(time.Second)}
	for _, tc := range []struct {
		name     string
		readings []capacityClockReading
		calls    int
		accepted bool
	}{
		{"first within bound", []capacityClockReading{good, slow, slow}, 1, true},
		{"slow then within bound", []capacityClockReading{slow, good, slow}, 2, true},
		{"third within bound", []capacityClockReading{slow, slow, good}, 3, true},
		{"three slow samples", []capacityClockReading{slow, slow, slow}, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got, err := sampleCapacityClock(context.Background(), func(context.Context) (capacityClockReading, error) {
				if calls >= len(tc.readings) {
					t.Fatal("clock sample count exceeded")
				}
				reading := tc.readings[calls]
				calls++
				return reading, nil
			})
			if calls != tc.calls {
				t.Fatalf("clock reads = %d, want %d", calls, tc.calls)
			}
			if tc.accepted {
				if err != nil || got != want {
					t.Fatalf("clock sample = %+v, %v; want %+v", got, err, want)
				}
				return
			}
			var sourceErr *SourceError
			if !errors.As(err, &sourceErr) || sourceErr.Kind != TimeoutError || sourceErr.SourceID != "capacity-clock-uncertainty" || got != (CapacityClockSample{}) {
				t.Fatalf("clock sample = %+v, %v", got, err)
			}
		})
	}
}

func TestCapacityClockPreservesReadErrorAndContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	got, err := sampleCapacityClock(ctx, func(readCtx context.Context) (capacityClockReading, error) {
		calls++
		if readCtx != ctx {
			t.Fatal("clock read context changed")
		}
		return capacityClockReading{}, readCtx.Err()
	})
	if calls != 1 || err != context.Canceled || got != (CapacityClockSample{}) {
		t.Fatalf("clock sample = %+v, %v; reads = %d", got, err, calls)
	}
}
