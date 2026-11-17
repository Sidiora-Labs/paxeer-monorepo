package ranker

import (
	"context"
	"strings"
	"testing"
	"time"

	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
)

type databaseClockRefreshSource struct {
	input      InputSnapshot
	called     bool
	usedLegacy bool
}

func (s *databaseClockRefreshSource) FreezeAtDatabaseClock(_ context.Context, runID string) (InputSnapshot, error) {
	s.called = true
	in := s.input
	in.RunID = runID
	return in, nil
}

func (s *databaseClockRefreshSource) Freeze(_ context.Context, _ string, _ ...time.Time) (InputSnapshot, error) {
	s.usedLegacy = true
	return InputSnapshot{}, ErrUnavailable
}

type databaseClockRefreshSink struct{ completed int }

func (s *databaseClockRefreshSink) Complete(_ context.Context, snapshots []Snapshot) error {
	s.completed = len(snapshots)
	return nil
}

func TestRefreshUsesDatabaseClock(t *testing.T) {
	in := fixture(0)
	in.CutoffTimeUTC = 100000
	in.Provenance = &Provenance{
		Watermark:      v1.Watermark{ChainId: "125", Height: "125", BlockHash: "0x" + strings.Repeat("a", 64), ProjectionRevision: "1", ObservedAt: time.Unix(99999, 0).UTC().Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"},
		UniverseDigest: CanonicalUniverse(in.Candidates),
		StatsRevision:  "stats:1000", ModerationRevision: "moderation", ActivityRevision: "activity:1000",
		WindowStart: 100000 - 86400, WindowEnd: 100000,
	}
	source := &databaseClockRefreshSource{input: in}
	sink := &databaseClockRefreshSink{}
	coordinator, err := NewCoordinator(source, sink)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = coordinator.Refresh(context.Background(), "be0ebdd7-e6e2-4f9a-b856-68e7fb31bf22", false); err != nil {
		t.Fatal(err)
	}
	if !source.called || source.usedLegacy || sink.completed != len(HeavyCategoryNames()) {
		t.Fatalf("database-clock refresh used legacy=%t called=%t completed=%d", source.usedLegacy, source.called, sink.completed)
	}
}
