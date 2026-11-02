package account

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestAdaptV2PreferencesMatchesFrozenSchema(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.Revision = "42"
	preferences.Locale = "fr-CA"
	preferences.Timezone = "America/Toronto"
	preferences.QuietHours = QuietHours{Timezone: "America/Toronto", Start: "21:30", End: "06:45"}
	preferences.Channels = []string{"inapp", "push"}
	preferences.AnalyticsConsent = true
	preferences.NotificationCategories = map[string]bool{"trade": true, "mention": true, "reward": false}
	for _, update := range []bool{false, true} {
		schema := "PreferencesReadResponse"
		if update {
			schema = "PreferencesUpdateResponse"
		}
		t.Run(schema, func(t *testing.T) {
			for _, revision := range []string{"0", "42"} {
				preferences.Revision = revision
				raw := v2OwnerTestEnvelope(t, map[string]Preferences{"preferences": preferences}, revision)
				body, err := AdaptV2Preferences(raw, update)
				if err != nil {
					t.Fatal(err)
				}
				v2OwnerTestSchema(t, schema, raw, body)
				var response v2.PreferencesReadResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				got := response.Data.Preferences
				if string(got.Revision) != revision || got.Locale != preferences.Locale || got.Timezone != preferences.Timezone || !got.AnalyticsConsent || !reflect.DeepEqual(got.Channels, []v2.Channel{"inapp", "push"}) {
					t.Fatalf("preferences changed: %+v", got)
				}
				if got.QuietHours.Start != "21:30" || got.QuietHours.End != "06:45" || got.QuietHours.Timezone != preferences.QuietHours.Timezone {
					t.Fatalf("quiet hours changed: %+v", got.QuietHours)
				}
				expected := v2.NotificationCategories{Trade: true, Mention: true}
				if got.NotificationCategories != expected {
					t.Fatalf("categories = %+v, want %+v", got.NotificationCategories, expected)
				}
				for _, mutation := range []struct {
					path  []string
					value json.RawMessage
				}{
					{[]string{"data", "preferences", "locale"}, json.RawMessage(`"invalid_locale"`)},
					{[]string{"data", "preferences", "revision"}, json.RawMessage(`"01"`)},
					{[]string{"data", "preferences", "channels"}, json.RawMessage(`["sms"]`)},
					{[]string{"data", "preferences", "analyticsConsent"}, nil},
					{[]string{"data", "preferences", "notificationCategories"}, json.RawMessage(`{"trade":null}`)},
					{[]string{"data", "preferences", "notificationCategories"}, json.RawMessage(`{"unknown":true}`)},
					{[]string{"data", "preferences", "notificationCategories"}, json.RawMessage(`null`)},
					{[]string{"data", "preferences", "quietHours", "timezone"}, json.RawMessage(`"bad zone"`)},
				} {
					invalid := v2OwnerTestMutation(t, raw, mutation.path, mutation.value)
					if output, err := AdaptV2Preferences(invalid, update); err == nil || output != nil {
						t.Fatalf("accepted invalid preferences at %v: %s", mutation.path, output)
					}
				}
				v2OwnerTestRejectEnvelope(t, raw, func(input []byte) ([]byte, error) { return AdaptV2Preferences(input, update) })
			}
		})
	}
	t.Run("default preferences", func(t *testing.T) {
		raw := v2OwnerTestEnvelope(t, map[string]Preferences{"preferences": DefaultPreferences()}, "0")
		body, err := AdaptV2Preferences(raw, false)
		if err != nil {
			t.Fatal(err)
		}
		v2OwnerTestSchema(t, "PreferencesReadResponse", raw, body)
		var response v2.PreferencesReadResponse
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatal(err)
		}
		if response.Data.Preferences.NotificationCategories != (v2.NotificationCategories{}) || len(response.Data.Preferences.Channels) != 0 {
			t.Fatalf("default preferences enabled notifications: %+v", response.Data.Preferences)
		}
	})
}

func TestAdaptV2DeviceRegisterMatchesFrozenSchema(t *testing.T) {
	deviceID := uuid.New()
	raw := v2OwnerTestEnvelope(t, map[string]any{"deviceId": deviceID, "state": "active", "revision": "17"}, "17")
	body, err := AdaptV2DeviceRegister(raw)
	if err != nil {
		t.Fatal(err)
	}
	v2OwnerTestSchema(t, "DeviceRegisterResponse", raw, body)
	var response v2.DeviceRegisterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.DeviceId != deviceID.String() || response.Data.State != "active" || response.Data.Revision != "17" {
		t.Fatalf("device registration changed: %+v", response.Data)
	}
	for _, mutation := range []struct {
		name  string
		value json.RawMessage
	}{
		{"deviceId", json.RawMessage(`"not-a-uuid"`)},
		{"deviceId", nil},
		{"state", json.RawMessage(`"revoked"`)},
		{"revision", json.RawMessage(`17`)},
		{"revision", nil},
	} {
		invalid := v2OwnerTestMutation(t, raw, []string{"data", mutation.name}, mutation.value)
		if output, err := AdaptV2DeviceRegister(invalid); err == nil || output != nil {
			t.Fatalf("accepted invalid device %s: %s", mutation.name, output)
		}
	}
	v2OwnerTestRejectEnvelope(t, raw, AdaptV2DeviceRegister)
}

func TestAdaptV2AccountJobsMatchFrozenSchema(t *testing.T) {
	at := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	for _, kind := range []string{"export", "deletion"} {
		for _, status := range []bool{false, true} {
			schema := "AccountExportResponse"
			if kind == "deletion" {
				schema = "AccountDeleteResponse"
			}
			if status {
				if kind == "export" {
					schema = "AccountExportStatusResponse"
				} else {
					schema = "AccountDeleteStatusResponse"
				}
			}
			t.Run(schema, func(t *testing.T) {
				job := Job{ID: uuid.New(), AccountID: uuid.New(), Kind: kind, State: "requested", RequestedAt: at}
				var data any = map[string]any{"job": wireJob(job)}
				if kind == "deletion" {
					job.State = "processing"
					data = deletionProjection(job)
					if status {
						projection := deletionProjection(job)
						data = map[string]any{"job": projection.Job, "downloadUrl": projection.Job.DownloadURL}
					}
				} else if status {
					data = map[string]any{"job": wireJob(job), "downloadUrl": nil}
				}
				raw := v2OwnerTestEnvelope(t, data, "0")
				body, err := AdaptV2AccountJob(raw, kind, status)
				if err != nil {
					t.Fatal(err)
				}
				v2OwnerTestSchema(t, schema, raw, body)
				var response Envelope[struct {
					Job              v2.AccountJob        `json:"job"`
					RetentionSummary []v2.RetentionReason `json:"retentionSummary"`
				}]
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				got := response.Data.Job
				if got.JobId != job.ID.String() || got.AccountId != job.AccountID.String() || got.Kind != kind || got.RequestedAt != "2026-10-12T09:30:00.000Z" || got.ExpiresAt != nil || got.DownloadUrl != nil {
					t.Fatalf("job changed: %+v", got)
				}
				if kind == "export" {
					if got.State != "queued" || len(got.DomainResults) != 0 || len(got.RetainedReasons) != 0 {
						t.Fatalf("queued export changed: %+v", got)
					}
				} else {
					if got.State != "running" || len(got.DomainResults) != 4 || got.DomainResults[2].State != "pending" || got.DomainResults[2].CompletedAt != nil || len(got.RetainedReasons) != 2 || !got.RetainedReasons[0].IrreversibleChainData {
						t.Fatalf("deletion progress changed: %+v", got)
					}
					if !status && !reflect.DeepEqual(response.Data.RetentionSummary, got.RetainedReasons) {
						t.Fatalf("retention summary changed: %+v", response.Data.RetentionSummary)
					}
				}
				for _, mutation := range []struct {
					name  string
					value json.RawMessage
				}{
					{"jobId", json.RawMessage(`"invalid"`)},
					{"state", json.RawMessage(`"processing"`)},
					{"requestedAt", json.RawMessage(`"2026-02-30T09:30:00.000Z"`)},
					{"expiresAt", nil},
					{"domainResults", json.RawMessage(`null`)},
					{"retainedReasons", json.RawMessage(`null`)},
					{"downloadUrl", json.RawMessage(`"http://example.com/export"`)},
				} {
					invalid := v2OwnerTestMutation(t, raw, []string{"data", "job", mutation.name}, mutation.value)
					if output, err := AdaptV2AccountJob(invalid, kind, status); err == nil || output != nil {
						t.Fatalf("accepted invalid job %s: %s", mutation.name, output)
					}
				}
				otherKind := "deletion"
				if kind == "deletion" {
					otherKind = "export"
				}
				if output, err := AdaptV2AccountJob(raw, otherKind, status); err == nil || output != nil {
					t.Fatalf("accepted mismatched job kind: %s", output)
				}
				if output, err := AdaptV2AccountJob(raw, "unknown", status); err == nil || output != nil {
					t.Fatalf("accepted unknown job kind: %s", output)
				}
				if kind == "export" && status || kind == "deletion" && !status {
					field := "downloadUrl"
					if kind == "deletion" {
						field = "retentionSummary"
					}
					invalid := v2OwnerTestMutation(t, raw, []string{"data", field}, nil)
					if output, err := AdaptV2AccountJob(invalid, kind, status); err == nil || output != nil {
						t.Fatalf("accepted missing %s: %s", field, output)
					}
				}
				v2OwnerTestRejectEnvelope(t, raw, func(input []byte) ([]byte, error) { return AdaptV2AccountJob(input, kind, status) })
				if kind == "export" && status {
					job.State = "ready"
					ready := wireJob(job)
					expires := "2026-10-13T09:30:00.000Z"
					url := "https://example.com/v2/me/exports/" + job.ID.String() + "/download"
					ready.ExpiresAt, ready.DownloadURL = &expires, &url
					raw := v2OwnerTestEnvelope(t, map[string]any{"job": ready, "downloadUrl": url}, "0")
					body, err := AdaptV2AccountJob(raw, kind, status)
					if err != nil {
						t.Fatal(err)
					}
					v2OwnerTestSchema(t, schema, raw, body)
					var completed v2.AccountExportStatusResponse
					if err := json.Unmarshal(body, &completed); err != nil {
						t.Fatal(err)
					}
					if completed.Data.DownloadUrl == nil || *completed.Data.DownloadUrl != url || completed.Data.Job.DownloadUrl == nil || *completed.Data.Job.DownloadUrl != url || completed.Data.Job.ExpiresAt == nil || *completed.Data.Job.ExpiresAt != expires || completed.Data.Job.State != "completed" {
						t.Fatalf("completed export changed: %+v", completed.Data)
					}
				}
			})
		}
	}
}

func v2OwnerTestEnvelope(t *testing.T, data any, revision string) []byte {
	t.Helper()
	at := time.Date(2026, 10, 12, 11, 30, 0, 123456789, time.FixedZone("offset", 2*60*60))
	envelope, err := Response(uuid.New(), data, revision, at)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func v2OwnerTestSchema(t *testing.T, schema string, before, after []byte) {
	t.Helper()
	if err := v2.ValidateJSON(schema, after); err != nil {
		t.Fatalf("%s: %v\n%s", schema, err, after)
	}
	var original, adapted map[string]json.RawMessage
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &adapted); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"requestId", "watermark", "revision", "quality", "nextCursor", "generatedAt"} {
		if !reflect.DeepEqual(original[name], adapted[name]) {
			t.Fatalf("%s changed: %s -> %s", name, original[name], adapted[name])
		}
	}
}

func v2OwnerTestMutation(t *testing.T, raw []byte, path []string, value json.RawMessage) []byte {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	if len(path) > 1 {
		object[path[0]] = v2OwnerTestMutation(t, object[path[0]], path[1:], value)
	} else if value == nil {
		delete(object, path[0])
	} else {
		object[path[0]] = value
	}
	result, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func v2OwnerTestRejectEnvelope(t *testing.T, raw []byte, adapt func([]byte) ([]byte, error)) {
	t.Helper()
	for _, name := range []string{"requestId", "data", "watermark", "revision", "quality", "nextCursor", "generatedAt"} {
		invalid := v2OwnerTestMutation(t, raw, []string{name}, nil)
		if output, err := adapt(invalid); err == nil || output != nil {
			t.Fatalf("accepted missing envelope %s: %s", name, output)
		}
	}
	for _, invalid := range [][]byte{[]byte(`null`), []byte(`[]`), []byte(`{`), []byte(`{} {}`), v2OwnerTestMutation(t, raw, []string{"data"}, json.RawMessage(`null`)), v2OwnerTestMutation(t, raw, []string{"generatedAt"}, json.RawMessage(`"not-a-timestamp"`))} {
		if output, err := adapt(invalid); err == nil || output != nil {
			t.Fatalf("accepted malformed envelope: %s", output)
		}
	}
}
