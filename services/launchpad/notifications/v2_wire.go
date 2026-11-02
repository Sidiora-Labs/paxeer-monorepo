package notifications

import (
	"encoding/json"
	"fmt"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

type V2NotificationItem struct {
	InboxItem
	V2ID, Wallet, DeliveryState string
	ReadAt                      *time.Time
}

type V2NotificationsPage struct {
	Items      []V2NotificationItem
	NextCursor *string
}

type v2NotificationItem struct {
	v2.Notification
	Summary string `json:"summary"`
	Read    bool   `json:"read"`
}

type v2NotificationEnvelope[T any] struct {
	RequestID   string              `json:"requestId"`
	Data        T                   `json:"data"`
	Watermark   *v2.Watermark       `json:"watermark"`
	Revision    *v2.PositiveUInt256 `json:"revision"`
	Quality     string              `json:"quality"`
	NextCursor  *string             `json:"nextCursor"`
	GeneratedAt string              `json:"generatedAt"`
}

func AdaptV2Notifications(requestID string, page V2NotificationsPage, generatedAt time.Time) ([]byte, error) {
	items := make([]v2NotificationItem, len(page.Items))
	for i, item := range page.Items {
		if item.Read != (item.ReadAt != nil) {
			return nil, fmt.Errorf("NotificationsResponse item %d: read state and read timestamp disagree", i)
		}
		var readAt *string
		if item.ReadAt != nil {
			value := item.ReadAt.UTC().Format("2006-01-02T15:04:05.000Z")
			readAt = &value
		}
		kind := item.Category
		if kind == "watchlist" {
			kind = "price_alert"
		}
		items[i] = v2NotificationItem{
			Notification: v2.Notification{
				NotificationId: item.V2ID,
				Wallet:         item.Wallet,
				Kind:           kind,
				EntityId:       item.ResourceID,
				CreatedAt:      item.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
				ReadAt:         readAt,
				DeliveryState:  item.DeliveryState,
			},
			Summary: item.Summary,
			Read:    item.Read,
		}
	}
	data := struct {
		Items []v2NotificationItem `json:"items"`
	}{Items: items}
	return marshalV2NotificationEnvelope("NotificationsResponse", requestID, data, page.NextCursor, generatedAt)
}

func AdaptV2NotificationRead(requestID string, readAt, generatedAt time.Time) ([]byte, error) {
	return marshalV2NotificationEnvelope("NotificationReadResponse", requestID, v2.NotificationReadData{
		ReadAt: readAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}, nil, generatedAt)
}

func marshalV2NotificationEnvelope[T any](schema, requestID string, data T, nextCursor *string, generatedAt time.Time) ([]byte, error) {
	raw, err := json.Marshal(v2NotificationEnvelope[T]{
		RequestID: requestID, Data: data, Quality: "complete", NextCursor: nextCursor,
		GeneratedAt: generatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", schema, err)
	}
	if err := v2.ValidateJSON(schema, raw); err != nil {
		return nil, fmt.Errorf("%s: %w", schema, err)
	}
	return raw, nil
}
