package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestObservedFeeBpsAccountingAndLegacyKeys(t *testing.T) {
	model, db := productionAccountingFixture(t)
	ctx := context.Background()
	const secondPool = "0x2222222222222222222222222222222222222222"
	fee := func(value uint16) *uint16 { return &value }
	previous := map[string]string{}
	for _, row := range []struct {
		pool     string
		sequence int
		fee      *uint16
		quality  string
		legacy   bool
	}{
		{productionAddress, 1, fee(30), "qualified_execution", true},
		{productionAddress, 2, fee(100), "qualified_execution", false},
		{secondPool, 1, fee(40), "qualified_execution", false},
		{secondPool, 2, fee(75), "qualified_execution", true},
		{secondPool, 3, nil, "qualified_execution", false},
		{secondPool, 4, fee(300), "unavailable", false},
	} {
		fill, position, publication := productionFill(t, row.pool, productionBase, row.sequence, previous[row.pool])
		previous[row.pool] = position.EffectIdentity
		fill.Execution = accounting.ExecutionPolicy{Quality: row.quality, FeeBPS: row.fee, Evidence: []accounting.EvidenceRef{}}
		raw, err := json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		if row.legacy {
			raw = bytes.Replace(raw, []byte(`"feeBPS":`), []byte(`"feeBps":`), 1)
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(raw))
		position.PayloadDigest = accounting.Digest(digest)
		positionRaw, err := json.Marshal(position)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision) VALUES($1,$2,$3,$4,$5,1699999980,$6,$7,$8,$8)`, publication.PublicationID, fill.SourceFillID, digest, productionProjection, row.pool, positionRaw, raw, row.sequence); err != nil {
			t.Fatal(err)
		}
	}
	got, err := model.ObservedFeeBps(ctx, []string{productionAddress, secondPool})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("observed fees: got %v, want both pools", got)
	}
	for pool, want := range map[string]int{productionAddress: 100, secondPool: 75} {
		if value, ok := got[pool]; !ok || value != want {
			t.Errorf("observed fee for %s: got %d (present=%t), want newest qualified fee %d", pool, value, ok, want)
		}
	}
}
