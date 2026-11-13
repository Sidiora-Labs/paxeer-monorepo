package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestRESTDatasetMeasurementReportsCommittedRows(t *testing.T) {
	model, db := productionAccountingFixture(t)
	insertProductionFill(t, db, 1, "")
	fill, _, _ := productionFill(t, productionAddress, productionBase, 1, "")
	var factDigest string
	if err := db.QueryRow(context.Background(), `SELECT source_fact_digest FROM stats.accounting_market_commits_v1 WHERE pool=$1`, productionAddress).Scan(&factDigest); err != nil {
		t.Fatal(err)
	}
	framed, err := json.Marshal([]any{productionAddress, int64(1), fill.SourceFillID, factDigest})
	if err != nil {
		t.Fatal(err)
	}
	actualDigest := sha256.Sum256(append(framed, '\n'))
	measurement, err := model.MeasureRESTDataset(context.Background(), productionAddress, 10_000_000, fmt.Sprintf("%x", actualDigest))
	if !errors.Is(err, store.ErrDataIntegrity) || measurement.Rows != 1 || measurement.HotRows != 1 || measurement.SHA256 != fmt.Sprintf("%x", actualDigest) || measurement.LastPageAfter != 0 {
		t.Fatalf("committed dataset measurement: %+v, %v", measurement, err)
	}
	for _, tc := range []struct {
		pool string
		rows int64
		digest string
	}{
		{"invalid", 10_000_000, strings.Repeat("0", 64)},
		{productionAddress, 1, strings.Repeat("0", 64)},
		{productionAddress, 10_000_000, "invalid"},
	} {
		if _, err := model.MeasureRESTDataset(context.Background(), tc.pool, tc.rows, tc.digest); !errors.Is(err, store.ErrDataIntegrity) {
			t.Fatalf("invalid dataset parameters accepted: %+v, %v", tc, err)
		}
	}
}
