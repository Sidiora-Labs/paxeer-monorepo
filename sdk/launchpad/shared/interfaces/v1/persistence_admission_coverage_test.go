package v1

import (
	"errors"
	"testing"
	"time"
)

func TestPersistenceRecordsRejectIncompleteAndOverflowingBindings(t *testing.T) {
	payload, err := NewCanonicalPayload([]byte(`{"value":125}`))
	if err != nil {
		t.Fatal(err)
	}
	max := MaxPersistenceCounter + 1
	cases := map[string]interface{ Validate() error }{
		"outbox source":         OutboxIntent{SourceOrder: PersistenceSourceOrder{BlockNumber: max}},
		"consumer source":       ConsumerInput{PartitionSequence: 1, SourceOrder: PersistenceSourceOrder{BlockNumber: max}},
		"consumer partition":    ConsumerInput{PartitionSequence: 0},
		"consumer inbox":        ConsumerInput{Partition: "pool", PartitionSequence: 1, Payload: payload},
		"checkpoint authority":  CheckpointFence{PreviousHeight: 1},
		"ingest fence":          IngestBatch{LastCompleteHeight: 1},
		"ingest regression":     IngestBatch{BatchID: "batch", LastCompleteHeight: 1, Fence: CheckpointFence{PreviousHeight: 2, Lease: "lease", Generation: "g1"}},
		"commit regression":     IngestCommit{BatchID: "batch", PreviousHeight: 2, LastCompleteHeight: 1, CommittedAt: time.Now()},
		"publication authority": PublishAck{Sequence: 1},
		"partition authority":   PartitionWatermark{Sequence: 1},
		"snapshot authority":    SnapshotWatermark{BlockNumber: 1},
		"snapshot partition":    SnapshotWatermark{Generation: "g1", Partitions: []PartitionWatermark{{Sequence: 1}}},
	}
	for name, record := range cases {
		t.Run(name, func(t *testing.T) {
			want := ErrInvalidPersistenceRecord
			if name == "outbox source" || name == "consumer source" {
				want = ErrPersistenceOverflow
			}
			if err := record.Validate(); !errors.Is(err, want) {
				t.Fatalf("unbound persistence record: %v", err)
			}
		})
	}
}
