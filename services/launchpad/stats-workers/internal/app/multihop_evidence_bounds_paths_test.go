package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
)

func TestMultihopRouteChecksEveryBoundedEvidenceField(t *testing.T) {
	f := newRepair70Fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, sql string
		edit      func(*canonicalMultihopLog)
	}{
		{name: "route shape", edit: func(l *canonicalMultihopLog) { l.Topics = nil }},
		{name: "sender topic", edit: func(l *canonicalMultihopLog) { l.Topics[1] = "invalid" }},
		{name: "input topic", edit: func(l *canonicalMultihopLog) { l.Topics[2] = "invalid" }},
		{name: "output topic", edit: func(l *canonicalMultihopLog) { l.Topics[3] = "invalid" }},
		{name: "equal tokens", edit: func(l *canonicalMultihopLog) { l.Topics[3] = l.Topics[2] }},
		{name: "missing creation", sql: `DELETE FROM indexer.indexer_canonical_creations`},
		{name: "invalid pool", sql: `UPDATE indexer.indexer_canonical_creations SET pool='invalid' WHERE token='0xed9e01c0e704566017556794f9d89daf4363099d'`},
		{name: "malformed topics", sql: `UPDATE indexer.indexer_raw_logs SET topics='{}' WHERE log_index=2`},
		{name: "changed transaction order", sql: `UPDATE indexer.indexer_raw_logs SET transaction_index=1 WHERE log_index=2`},
		{name: "missing mirror", sql: `UPDATE indexer.indexer_raw_logs SET topics='[]' WHERE log_index=13`},
		{name: "mirror shape", sql: `UPDATE indexer.indexer_raw_logs SET data='\x01' WHERE log_index=13`},
		{name: "mirror pool", sql: `UPDATE indexer.indexer_raw_logs SET topics=jsonb_set(topics,'{2}','"invalid"') WHERE log_index=13`},
		{name: "mirror sender", sql: `UPDATE indexer.indexer_raw_logs SET topics=jsonb_set(topics,'{3}','"invalid"') WHERE log_index=13`},
		{name: "mirror input", sql: `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,0,1) WHERE log_index=13`},
		{name: "mirror output", sql: `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,32,1) WHERE log_index=13`},
		{name: "timestamp overflow", sql: `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,160,1) WHERE log_index=13`},
		{name: "leg shape", sql: `UPDATE indexer.indexer_raw_logs SET data='\x01' WHERE log_index=2`},
		{name: "leg pool", sql: `UPDATE indexer.indexer_raw_logs SET topics=jsonb_set(topics,'{1}','"invalid"') WHERE log_index=2`},
		{name: "leg wallet", sql: `UPDATE indexer.indexer_raw_logs SET topics=jsonb_set(topics,'{2}','"invalid"') WHERE log_index=2`},
		{name: "leg fee", sql: `UPDATE indexer.indexer_raw_logs SET data=set_byte(data,96,1) WHERE log_index=2`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.sql != "" {
				if _, err := tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			route := f.route
			route.Topics = append([]string(nil), f.route.Topics...)
			route.Data = append([]byte(nil), f.route.Data...)
			if tc.edit != nil {
				tc.edit(&route)
			}
			if err := f.projection.projectRoute(ctx, tx, route); !errors.Is(err, consumer.ErrLegacyBridgeIntegrity) {
				t.Fatalf("changed route evidence accepted: %v", err)
			}
		})
	}
	if rows, checkpoints := f.counts(t); rows != 0 || checkpoints != 0 {
		t.Fatalf("invalid route persisted: %d %d", rows, checkpoints)
	}
	if _, err := multihopAddress("0x" + strings.Repeat("0", 64)); !errors.Is(err, consumer.ErrLegacyBridgeIntegrity) {
		t.Fatal(err)
	}
}
