// Package httpapi registers the indexer's read/operational HTTP routes.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/go-chi/chi/v5"
)

type StatusMode string

const (
	StatusCanonical StatusMode = "canonical"
	StatusLegacy    StatusMode = "legacy"
)

// DeliveryHealth is implemented by the durable delivery Store.
type DeliveryHealth interface {
	Stats(context.Context) (delivery.Stats, error)
}

// DeadLetterPolicy is implemented by security/policy.DeadLetters. Its handler
// owns authentication, scoped authorization, durable inspection and audit.
type DeadLetterPolicy interface {
	Handler() http.Handler
}

type StatusDeps struct {
	Store             *store.Store
	ChainID           int32
	Publisher         *publisher.Publisher
	StartTime         time.Time
	Mode              StatusMode
	CanonicalStreamID string
	Delivery          DeliveryHealth
	MaxDeliveryLag    time.Duration
	DeadLetters       DeadLetterPolicy
}

func RegisterStatus(r chi.Router, d StatusDeps) {
	r.Get("/status", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
		defer cancel()
		body := map[string]any{
			"chainId": d.ChainID, "mode": d.Mode,
			"lastProcessedBlock": nil, "totalPoolsIndexed": nil,
			"totalSwapsIndexed": nil,
			"uptime":            time.Since(d.StartTime).Round(time.Second).String(),
			"status":            "ok",
		}
		failures := []string{}
		unavailable := func(component string) { failures = append(failures, component); body["status"] = "unavailable" }
		if d.Store == nil {
			unavailable("store")
		} else {
			switch d.Mode {
			case StatusCanonical:
				body["canonicalStreamId"] = d.CanonicalStreamID
				height, err := d.Store.GetCanonicalWatermark(ctx, d.ChainID, d.CanonicalStreamID)
				if err != nil {
					unavailable("canonicalWatermark")
				} else {
					body["lastProcessedBlock"] = height
				}
			case StatusLegacy:
				height, err := d.Store.GetCursor(ctx, d.ChainID)
				if err != nil {
					unavailable("legacyCursor")
				} else {
					body["lastProcessedBlock"] = height
				}
			default:
				unavailable("mode")
			}
			pools, err := d.Store.PoolCount(ctx)
			if err != nil {
				unavailable("poolCount")
			} else {
				body["totalPoolsIndexed"] = pools
			}
			swaps, err := d.Store.SwapCount(ctx)
			if err != nil {
				unavailable("swapCount")
			} else {
				body["totalSwapsIndexed"] = swaps
			}
		}
		if d.Mode == StatusCanonical {
			body["delivery"] = nil
			if d.Delivery == nil || d.MaxDeliveryLag <= 0 {
				unavailable("deliveryConfiguration")
			} else {
				health, err := d.Delivery.Stats(ctx)
				if err != nil {
					unavailable("delivery")
				} else {
					body["delivery"] = map[string]any{
						"pendingRows": health.PendingRows, "pendingBytes": health.PendingBytes,
						"quarantinedRows":             health.QuarantinedRows,
						"oldestUnpublishedAgeSeconds": health.OldestUnpublishedAgeSeconds,
						"leaseExpiryCount":            health.LeaseExpiryCount,
						"maxAcknowledgementLatencyMs": health.MaxAcknowledgementLatencyMS,
					}
					if body["status"] == "ok" && (health.QuarantinedRows > 0 || health.OldestUnpublishedAgeSeconds > d.MaxDeliveryLag.Seconds()) {
						body["status"] = "degraded"
					}
				}
			}
		} else if d.Mode == StatusLegacy && d.Publisher != nil {
			m := d.Publisher.Snapshot()
			body["webhooks"] = map[string]any{
				"totalEventsPublished": m.TotalEventsPublished, "totalDeliveries": m.TotalDeliveries,
				"totalFailures": m.TotalFailures, "totalDeadLettered": m.TotalDeadLettered,
				"totalDeduplicated": m.TotalDeduplicated, "totalRedisPublishes": m.TotalRedisPublishes,
				"totalRedisFailures": m.TotalRedisFailures, "currentInflight": m.CurrentInflight,
				"deadLetterQueueSize": d.Publisher.DeadLetterCount(),
			}
		}
		if len(failures) > 0 {
			body["unavailableComponents"] = failures
		}
		code := http.StatusOK
		if body["status"] != "ok" {
			code = http.StatusServiceUnavailable
		}
		sharedhttp.WriteJSON(w, code, body)
	})

	var operations http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "dead-letter policy unavailable", http.StatusServiceUnavailable)
	})
	if d.DeadLetters != nil {
		if handler := d.DeadLetters.Handler(); handler != nil {
			operations = handler
		}
	}
	r.Method(http.MethodGet, "/status/dead-letter", operations)
	r.Method(http.MethodPost, "/status/dead-letter/drain", operations)
}
