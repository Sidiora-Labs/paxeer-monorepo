package notifications

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestAdaptV2NotificationsMatchFrozenSchema(t *testing.T) {
	requestID := "00000000-0000-4000-8000-000000000123"
	wallet := "0x0000000000000000000000000000000000000001"
	zone := time.FixedZone("offset", 2*60*60)
	generatedAt := time.Date(2026, time.October, 26, 14, 34, 56, 789123456, zone)
	createdAt := generatedAt.Add(-time.Hour)
	readAt := generatedAt.Add(-time.Minute)
	cursor := "opaque-next-page"
	page := V2NotificationsPage{NextCursor: &cursor, Items: []V2NotificationItem{
		{InboxItem: InboxItem{ID: "internal-key-one", Category: "watchlist", Summary: "A watched pool crossed your threshold", ResourceID: "pool-one", CreatedAt: createdAt}, V2ID: "00000000-0000-7000-8000-000000000001", Wallet: wallet, DeliveryState: "pending"},
		{InboxItem: InboxItem{ID: "internal-key-two", Category: "mention", Summary: "You were mentioned", ResourceID: "post-two", CreatedAt: createdAt, Read: true}, V2ID: "00000000-0000-7000-8000-000000000002", Wallet: wallet, DeliveryState: "sent", ReadAt: &readAt},
		{InboxItem: InboxItem{ID: "internal-key-three", Category: "reward", Summary: "A reward is available", ResourceID: "reward-three", CreatedAt: createdAt}, V2ID: "00000000-0000-7000-8000-000000000003", Wallet: wallet, DeliveryState: "failed"},
	}}

	for _, tc := range []struct {
		name string
		page V2NotificationsPage
	}{
		{name: "empty", page: V2NotificationsPage{}},
		{name: "three items and next cursor", page: page},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := AdaptV2Notifications(requestID, tc.page, generatedAt)
			if err != nil {
				t.Fatal(err)
			}
			if err := v2.ValidateJSON("NotificationsResponse", raw); err != nil {
				t.Fatal(err)
			}
			var got v2.NotificationsResponse
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if got.RequestId != requestID || got.GeneratedAt != "2026-10-26T12:34:56.789Z" || got.Quality != "complete" || got.Revision != nil || got.Watermark != nil || !reflect.DeepEqual(got.NextCursor, tc.page.NextCursor) {
				t.Fatalf("unexpected list envelope: %#v", got)
			}
			if got.Data.Items == nil || len(got.Data.Items) != len(tc.page.Items) {
				t.Fatalf("items = %#v; want %d items in an array", got.Data.Items, len(tc.page.Items))
			}
			var extended struct {
				Data struct {
					Items []struct {
						Summary string `json:"summary"`
						Read    bool   `json:"read"`
					} `json:"items"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &extended); err != nil {
				t.Fatal(err)
			}
			for i, item := range tc.page.Items {
				kind := []string{"price_alert", "mention", "reward"}[i]
				var wantReadAt *string
				if item.Read {
					value := "2026-10-26T12:33:56.789Z"
					wantReadAt = &value
				}
				want := v2.Notification{NotificationId: item.V2ID, Wallet: wallet, Kind: kind, EntityId: item.ResourceID, CreatedAt: "2026-10-26T11:34:56.789Z", ReadAt: wantReadAt, DeliveryState: item.DeliveryState}
				if !reflect.DeepEqual(got.Data.Items[i], want) {
					t.Errorf("item %d = %#v; want %#v", i, got.Data.Items[i], want)
				}
				if extended.Data.Items[i].Summary != item.Summary || extended.Data.Items[i].Read != item.Read {
					t.Errorf("item %d lost summary or read state: %#v", i, extended.Data.Items[i])
				}
			}
		})
	}

	t.Run("read result", func(t *testing.T) {
		raw, err := AdaptV2NotificationRead(requestID, readAt, generatedAt)
		if err != nil {
			t.Fatal(err)
		}
		if err := v2.ValidateJSON("NotificationReadResponse", raw); err != nil {
			t.Fatal(err)
		}
		var got v2.NotificationReadResponse
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		want := v2.NotificationReadResponse{
			RequestId: requestID, Data: v2.NotificationReadData{ReadAt: "2026-10-26T12:33:56.789Z"},
			Quality: "complete", GeneratedAt: "2026-10-26T12:34:56.789Z",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("read response = %#v; want %#v", got, want)
		}
	})
}
