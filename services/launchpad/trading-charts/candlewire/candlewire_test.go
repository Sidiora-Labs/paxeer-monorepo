package candlewire_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/candlewire/testfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
)

func TestFromStoredBarRejectsInvalidInput(t *testing.T) {
	for _, raw := range []string{"{", `{"trades":"invalid"}`} {
		if _, err := candlewire.FromStoredBar([]byte(raw), nil); err == nil {
			t.Fatalf("malformed bar accepted: %s", raw)
		}
	}
	if _, err := candlewire.FromStoredBar([]byte(`{}`), nil); !errors.Is(err, engine.ErrInvalidFill) {
		t.Fatalf("invalid bar: %v", err)
	}
}

func identityJSON(t *testing.T, raw []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestCandleIdentityRecord(t *testing.T) {
	raw, err := os.ReadFile("testdata/candle_identity.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []testfixture.Record
	if err = json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	generated, err := testfixture.Records()
	if err != nil {
		t.Fatal(err)
	}
	current, err := json.Marshal(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(identityJSON(t, raw), identityJSON(t, current)) {
		t.Fatal("renew candle record from trading-charts: GOWORK=off go run ./candlewire/testdata/generate.go")
	}
	if len(records) < 2 {
		t.Fatal("two candle states required")
	}
	for i, record := range records {
		var bar engine.SeriesCandle
		if err = json.Unmarshal(record.StoredBar, &bar); err != nil {
			t.Fatal(err)
		}
		mapped, err := candlewire.FromStoredBar(record.StoredBar, record.Fills)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := engine.CanonicalCandle(bar, record.Fills)
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []any{mapped, canonical} {
			actual, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(identityJSON(t, actual), identityJSON(t, record.PublicCandle)) {
				t.Fatalf("candle differs for %s", bar.Interval)
			}
		}
		if mapped.SeriesModelVersion != candlewire.SeriesModelVersion {
			t.Fatal(mapped)
		}
		if i == 0 && (mapped.CompletenessStatus != "complete_through_watermark" || mapped.Coverage != "complete" || mapped.CorrectionReason == "") {
			t.Fatal(mapped)
		}
		if i == 1 && (mapped.CompletenessStatus != "live_interval" || mapped.Coverage != "partial" || mapped.CorrectionReason != "") {
			t.Fatal(mapped)
		}
	}
}
