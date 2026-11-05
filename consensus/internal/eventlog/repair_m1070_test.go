package eventlog

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventlog/cursor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// f004: An Add after the head snapshot must notify that snapshot's waiter,
// even though Add installs a new ready channel before the wait begins.
func TestRepairM_1070_SnapshotSubscriptionBoundary(t *testing.T) {
	for _, seeded := range []bool{false, true} {
		name := "empty"
		if seeded {
			name = "existing-head"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				lg := repairM1070Log(t)
				if seeded {
					repairM1070Add(t, lg, "previous")
				}
				st := lg.state()
				previous := st.info()
				repairM1070Add(t, lg, "boundary")
				want := lg.Info()
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				updated, err := lg.waitStateChange(ctx, st)
				if err != nil {
					t.Fatalf("Add between snapshot and subscription was missed: %v", err)
				}
				if got := updated.info(); got != want || got.Newest == previous.Newest {
					t.Fatalf("notification state = %+v, want %+v with a new head", got, want)
				}
				repairM1070Scan(t, lg, ctx, previous.Newest, want, "boundary")
			})
		})
	}
}

func TestRepairM_1070_WaitScanBroadcastAndResubscribe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lg := repairM1070Log(t)
		var previous cursor.Cursor
		for _, payload := range []types.EventDataString{"first", "second", "third"} {
			const readers = 4
			finished := make(chan struct{}, readers)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			for range readers {
				go func() {
					defer func() { finished <- struct{}{} }()
					var items []*Item
					info, err := lg.WaitScan(ctx, previous, func(item *Item) error {
						items = append(items, item)
						return ErrStopScan
					})
					if err != nil {
						t.Errorf("WaitScan: %v", err)
						return
					}
					if info.Newest == previous || len(items) != 1 {
						t.Errorf("WaitScan returned info %+v and %d items", info, len(items))
						return
					}
					if items[0].Cursor != info.Newest || items[0].Data != payload || items[0].Type != types.EventNewRoundStepValue {
						t.Errorf("WaitScan item = %+v, want payload %q at %v", items[0], payload, info.Newest)
					}
				}()
			}
			synctest.Wait()
			select {
			case <-finished:
				t.Fatal("WaitScan returned before Add changed the head")
			default:
			}
			repairM1070Add(t, lg, payload)
			for range readers {
				<-finished
			}
			cancel()
			previous = lg.Info().Newest
		}
	})
}

func TestRepairM_1070_WaitScanCancellationAndImmediateScan(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lg := repairM1070Log(t)
		for _, seeded := range []bool{false, true} {
			if seeded {
				repairM1070Add(t, lg, "available")
			}
			want := lg.Info()
			ctx, cancel := context.WithCancel(t.Context())
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				info, err := lg.WaitScan(ctx, want.Newest, func(*Item) error {
					t.Error("canceled WaitScan invoked the callback without a new head")
					return nil
				})
				if !errors.Is(err, context.Canceled) || info != want {
					t.Errorf("canceled WaitScan = (%+v, %v), want (%+v, context.Canceled)", info, err, want)
				}
			}()
			synctest.Wait()
			cancel()
			<-finished
			if seeded {
				repairM1070Scan(t, lg, ctx, cursor.Cursor{}, want, "available")
				callbackErr := errors.New("consumer error")
				info, err := lg.WaitScan(t.Context(), cursor.Cursor{}, func(*Item) error { return callbackErr })
				if !errors.Is(err, callbackErr) || info != want {
					t.Fatalf("callback error scan = (%+v, %v), want (%+v, %v)", info, err, want, callbackErr)
				}
			}
		}
	})
}

func repairM1070Log(t *testing.T) *Log {
	t.Helper()
	lg, err := New(LogSettings{WindowSize: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return lg
}

func repairM1070Add(t *testing.T, lg *Log, payload types.EventDataString) {
	t.Helper()
	if err := lg.Add(types.EventNewRoundStepValue, payload); err != nil {
		t.Fatalf("Add: %v", err)
	}
}

func repairM1070Scan(t *testing.T, lg *Log, ctx context.Context, previous cursor.Cursor, want Info, payload types.EventDataString) {
	t.Helper()
	var items []*Item
	info, err := lg.WaitScan(ctx, previous, func(item *Item) error {
		items = append(items, item)
		return ErrStopScan
	})
	if err != nil || info != want || len(items) != 1 {
		t.Fatalf("WaitScan = (%+v, %v), %d items; want (%+v, nil), 1 item", info, err, len(items), want)
	}
	if items[0].Cursor != want.Newest || items[0].Data != payload || items[0].Type != types.EventNewRoundStepValue {
		t.Fatalf("WaitScan item = %+v, want payload %q at %v", items[0], payload, want.Newest)
	}
	if eventType, ok := FindType(items[0].Events); !ok || eventType != types.EventNewRoundStepValue {
		t.Fatalf("event metadata = (%q, %v), want %q", eventType, ok, types.EventNewRoundStepValue)
	}
}
