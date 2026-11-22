package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
)

type composedChartSeriesRead struct {
	document     map[string]any
	items        []engine.SeriesCandle
	began, ended time.Time
}

func decodeComposedChartSeries(raw []byte, began, ended time.Time) (composedChartSeriesRead, error) {
	read := composedChartSeriesRead{began: began, ended: ended}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&read.document); err != nil {
		return read, err
	}
	var body struct {
		Items []engine.SeriesCandle `json:"items"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return read, err
	}
	read.items = body.Items
	items, ok := read.document["items"].([]any)
	if !ok {
		return read, fmt.Errorf("items: expected an array")
	}
	for i, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			return read, fmt.Errorf("items[%d]: expected an object", i)
		}
		delete(fields, "completenessStatus")
	}
	return read, nil
}

func compareComposedChartSeries(first, second composedChartSeriesRead) error {
	if err := composedChartFieldDifference("series", first.document, second.document); err != nil {
		return err
	}
	for n, read := range []composedChartSeriesRead{first, second} {
		if read.ended.Before(read.began) {
			return fmt.Errorf("read[%d]: request ended before it began", n)
		}
		for i, candle := range read.items {
			field := fmt.Sprintf("read[%d].items[%d].completenessStatus", n, i)
			if candle.IntervalEnd <= 0 {
				return fmt.Errorf("read[%d].items[%d].intervalEnd: expected a positive timestamp", n, i)
			}
			elapsed := "interval_elapsed"
			if candle.SourceWatermarkTimestamp >= candle.IntervalEnd {
				elapsed = "complete_through_watermark"
			}
			liveAllowed := read.began.Unix() < candle.IntervalEnd
			elapsedAllowed := read.ended.Unix() >= candle.IntervalEnd
			if (liveAllowed && candle.CompletenessStatus == "live_interval") || (elapsedAllowed && candle.CompletenessStatus == elapsed) {
				continue
			}
			return fmt.Errorf("%s: %q outside request interval [%s, %s] with intervalEnd=%d and sourceWatermarkTimestamp=%d", field, candle.CompletenessStatus, read.began.Format(time.RFC3339Nano), read.ended.Format(time.RFC3339Nano), candle.IntervalEnd, candle.SourceWatermarkTimestamp)
		}
	}
	return nil
}

func composedChartFieldDifference(path string, first, second any) error {
	switch a := first.(type) {
	case map[string]any:
		b, ok := second.(map[string]any)
		if !ok {
			break
		}
		keys := make(map[string]bool, len(a)+len(b))
		for key := range a {
			keys[key] = true
		}
		for key := range b {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			av, aok := a[key]
			bv, bok := b[key]
			field := path + "." + key
			if aok != bok {
				return fmt.Errorf("%s: field presence changed", field)
			}
			if err := composedChartFieldDifference(field, av, bv); err != nil {
				return err
			}
		}
		return nil
	case []any:
		b, ok := second.([]any)
		if !ok {
			break
		}
		if len(a) != len(b) {
			return fmt.Errorf("%s: length changed from %d to %d", path, len(a), len(b))
		}
		for i := range a {
			if err := composedChartFieldDifference(fmt.Sprintf("%s[%d]", path, i), a[i], b[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if !reflect.DeepEqual(first, second) {
		return fmt.Errorf("%s: changed from %v to %v", path, first, second)
	}
	return nil
}

func TestComposedChartSeriesComparisonAcrossIntervalEnd(t *testing.T) {
	const end = int64(1700000100)
	before := time.Unix(end, 0).Add(-time.Nanosecond)
	atEnd := time.Unix(end, 0)
	after := atEnd.Add(time.Second)
	wire := func(status string, watermark int64) []byte {
		t.Helper()
		candle := engine.SeriesCandle{
			AlgorithmVersion: engine.SeriesAlgorithm, IntervalEnd: end,
			SourceWatermarkHeight: 9007199254740992, SourceWatermarkTimestamp: watermark,
			CompletenessStatus: status, CorrectionReason: "delayed_ingestion",
		}
		raw, err := json.Marshal(struct {
			SchemaVersion int                   `json:"schemaVersion"`
			Algorithm     string                `json:"algorithmVersion"`
			Items         []engine.SeriesCandle `json:"items"`
			NoData        bool                  `json:"noData"`
		}{2, engine.SeriesAlgorithm, []engine.SeriesCandle{candle}, false})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	decode := func(raw []byte, began, ended time.Time) composedChartSeriesRead {
		t.Helper()
		read, err := decodeComposedChartSeries(raw, began, ended)
		if err != nil {
			t.Fatal(err)
		}
		return read
	}
	for _, tc := range []struct {
		name, firstStatus, secondStatus string
		firstBegan, firstEnded          time.Time
		secondBegan, secondEnded        time.Time
		watermark                       int64
		valid                           bool
	}{
		{"interval ends between reads", "live_interval", "interval_elapsed", before, before, atEnd, after, end - 1, true},
		{"both reads before end", "live_interval", "interval_elapsed", before, before, before, before, end - 1, false},
		{"live after end", "live_interval", "live_interval", atEnd, after, atEnd, after, end - 1, false},
		{"live within crossing read", "live_interval", "live_interval", before, atEnd, before, after, end - 1, true},
		{"elapsed within crossing read", "live_interval", "interval_elapsed", before, atEnd, before, after, end - 1, true},
		{"covered interval", "live_interval", "complete_through_watermark", before, before, atEnd, after, end, true},
		{"covered within crossing read", "live_interval", "complete_through_watermark", before, before, before, after, end + 1, true},
		{"uncovered interval", "live_interval", "complete_through_watermark", before, before, atEnd, after, end - 1, false},
		{"covered before end", "complete_through_watermark", "complete_through_watermark", before, before, before, before, end, false},
		{"covered interval requires covered status", "interval_elapsed", "interval_elapsed", atEnd, after, atEnd, after, end, false},
		{"unexpected status", "backfill_incomplete", "backfill_incomplete", atEnd, after, atEnd, after, end, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := decode(wire(tc.firstStatus, tc.watermark), tc.firstBegan, tc.firstEnded)
			second := decode(wire(tc.secondStatus, tc.watermark), tc.secondBegan, tc.secondEnded)
			err := compareComposedChartSeries(first, second)
			if (err == nil) != tc.valid {
				t.Fatalf("comparison valid=%t, want %t: %v", err == nil, tc.valid, err)
			}
			if err != nil && !strings.Contains(err.Error(), "completenessStatus") {
				t.Fatalf("status diagnostic: %v", err)
			}
		})
	}

	first := decode(wire("live_interval", end-1), before, before)
	raw := wire("interval_elapsed", end-1)
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	compareChanged := func(t *testing.T, field string) {
		t.Helper()
		changed, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		err = compareComposedChartSeries(first, decode(changed, atEnd, after))
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Fatalf("%s difference diagnostic: %v", field, err)
		}
	}
	var visit func(string, any)
	visit = func(path string, value any) {
		switch fields := value.(type) {
		case map[string]any:
			for key, original := range fields {
				field := path + "." + key
				if field == "series.items[0].completenessStatus" {
					continue
				}
				switch v := original.(type) {
				case map[string]any, []any:
					visit(field, v)
				default:
					t.Run(field, func(t *testing.T) {
						switch v := original.(type) {
						case string:
							fields[key] = v + "changed"
						case json.Number:
							n, err := strconv.ParseInt(v.String(), 10, 64)
							if err != nil {
								t.Fatal(err)
							}
							fields[key] = json.Number(strconv.FormatInt(n+1, 10))
						case bool:
							fields[key] = !v
						default:
							t.Fatalf("unhandled field type %T", original)
						}
						defer func() { fields[key] = original }()
						compareChanged(t, field)
					})
				}
			}
		case []any:
			for i, item := range fields {
				visit(fmt.Sprintf("%s[%d]", path, i), item)
			}
		}
	}
	visit("series", document)
	t.Run("additional field", func(t *testing.T) {
		document["additionalField"] = "value"
		defer delete(document, "additionalField")
		compareChanged(t, "series.additionalField")
	})
	t.Run("missing field", func(t *testing.T) {
		value := document["noData"]
		delete(document, "noData")
		defer func() { document["noData"] = value }()
		compareChanged(t, "series.noData")
	})
}
