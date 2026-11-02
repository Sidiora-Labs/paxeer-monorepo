package streams

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestAdaptV2StreamListMatchesFrozenSchema(t *testing.T) {
	const requestID = "33333333-3333-4333-8333-333333333333"
	at := time.Date(2026, time.October, 26, 15, 4, 5, 123000000, time.FixedZone("offset", 3600))
	viewers := int64(17)
	operations := []Operation{
		{ID: "11111111-1111-4111-8111-111111111111", Pool: "0x" + strings.Repeat("1", 40), Wallet: "0x" + strings.Repeat("2", 40), Title: "First stream", State: "LIVE", ProviderID: "provider_1", ProviderTime: &at, Revision: 9},
		{ID: "22222222-2222-4222-8222-222222222222", Pool: "0x" + strings.Repeat("3", 40), Wallet: "0x" + strings.Repeat("4", 40), Title: "Second stream", State: "LIVE", ProviderID: "provider_2", ProviderTime: &at, Revision: 10},
	}
	items := make([]V2Stream, 0, len(operations))
	for _, operation := range operations {
		item, err := AdaptV2Stream(operation, "https://livepeercdn.studio/hls/one/index.m3u8", &viewers, "verified")
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, item)
	}
	cursor := "next-page-cursor"
	for _, tc := range []struct {
		name     string
		items    []V2Stream
		cursor   *string
		revision string
	}{
		{name: "empty"},
		{name: "empty_slice", items: []V2Stream{}},
		{name: "page_with_cursor", items: items, cursor: &cursor, revision: "10"},
		{name: "maximum_first", items: []V2Stream{items[1], items[0]}, revision: "10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := AdaptV2StreamList(requestID, tc.items, tc.cursor, at)
			if err != nil {
				t.Fatal(err)
			}
			if err := v2.ValidateJSON("StreamListResponse", raw); err != nil {
				t.Fatal(err)
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Fatal(err)
			}
			fields := []string{"requestId", "data", "watermark", "revision", "quality", "nextCursor", "generatedAt"}
			if len(envelope) != len(fields) {
				t.Fatalf("unexpected envelope fields: %s", raw)
			}
			for _, field := range fields {
				if _, present := envelope[field]; !present {
					t.Fatalf("missing envelope field %s: %s", field, raw)
				}
			}
			for key, want := range map[string]string{
				"requestId":   requestID,
				"quality":     "complete",
				"generatedAt": "2026-10-26T14:04:05.123Z",
			} {
				var got string
				if err := json.Unmarshal(envelope[key], &got); err != nil || got != want {
					t.Fatalf("%s = %s, want %q (error %v)", key, envelope[key], want, err)
				}
			}
			if string(envelope["watermark"]) != "null" {
				t.Fatalf("watermark = %s, want null", envelope["watermark"])
			}
			wantRevision := "null"
			if tc.revision != "" {
				wantRevision = `"` + tc.revision + `"`
			}
			if string(envelope["revision"]) != wantRevision {
				t.Fatalf("revision = %s, want %s", envelope["revision"], wantRevision)
			}
			wantCursor, err := json.Marshal(tc.cursor)
			if err != nil {
				t.Fatal(err)
			}
			if string(envelope["nextCursor"]) != string(wantCursor) {
				t.Fatalf("nextCursor = %s, want %s", envelope["nextCursor"], wantCursor)
			}
			var data map[string]json.RawMessage
			if err := json.Unmarshal(envelope["data"], &data); err != nil {
				t.Fatal(err)
			}
			wantItems := tc.items
			if wantItems == nil {
				wantItems = []V2Stream{}
			}
			var gotItems []V2Stream
			if err := json.Unmarshal(data["items"], &gotItems); err != nil {
				t.Fatal(err)
			}
			if len(data) != 1 || !reflect.DeepEqual(gotItems, wantItems) {
				t.Fatalf("data.items = %s, want %#v in input order", data["items"], wantItems)
			}
		})
	}
	for _, revision := range []string{"", "0", "-1", "01", "1.5", "not-a-revision", "115792089237316195423570985008687907853269984665640564039457584007913129639936"} {
		t.Run("invalid_revision_"+revision, func(t *testing.T) {
			item := items[0]
			item.Revision = revision
			raw, err := AdaptV2StreamList(requestID, []V2Stream{item}, nil, at)
			if !errors.Is(err, ErrInvalid) || raw != nil {
				t.Fatalf("invalid revision %q returned bytes %s and error %v", revision, raw, err)
			}
		})
	}
}
