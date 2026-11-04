package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
)

const (
	MaxWebhookBodyBytes = 1 << 20
	MaxWebhookEvents    = 1000
)

var errWebhookTooLarge = errors.New("webhook request exceeds limits")

type webhookBodyKey struct{}

// WebhookEvent is a single event in a webhook batch.
type WebhookEvent struct {
	EventName      string                 `json:"eventName"`
	BlockNumber    int64                  `json:"blockNumber"`
	BlockTimestamp int64                  `json:"blockTimestamp"`
	TxHash         string                 `json:"txHash"`
	LogIndex       int                    `json:"logIndex"`
	Args           map[string]interface{} `json:"args"`
}

// WebhookBody is the expected POST body for /webhooks/events.
type WebhookBody struct {
	Events []WebhookEvent `json:"events"`
}

// WebhookDeps holds the dependencies for the webhook route.
type WebhookDeps struct {
	Builder *engine.Builder
	Logger  *slog.Logger
	Secret  string
}

// RegisterWebhook registers the HMAC-authenticated webhook receiver.
func RegisterWebhook(r chi.Router, deps WebhookDeps) {
	r.With(webhookAuth(deps.Secret)).Post("/webhooks/events", webhookHandler(deps))
}

func webhookHandler(deps WebhookDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wb, ok := r.Context().Value(webhookBodyKey{}).(*WebhookBody)
		if !ok {
			sharedhttp.WriteError(w, 400, "Bad Request", "authenticated webhook body required")
			return
		}

		processed := 0
		errCount := 0
		retryable := false

		for _, event := range wb.Events {
			if event.EventName != "Swap" {
				continue
			}

			if !validWebhookSwap(event) {
				deps.Logger.Warn("rejecting invalid swap", slog.String("txHash", event.TxHash))
				errCount++
				continue
			}

			swap := engine.SwapEvent{
				PoolID:         asString(event.Args["poolId"]),
				PoolAddress:    asString(event.Args["poolAddress"]),
				Sender:         asString(event.Args["sender"]),
				IsBuy:          asBool(event.Args["isBuy"]),
				AmountIn:       asString(event.Args["amountIn"]),
				AmountOut:      asString(event.Args["amountOut"]),
				Fee:            asString(event.Args["fee"]),
				Price:          asString(event.Args["price"]),
				BlockTimestamp: event.BlockTimestamp,
				TxHash:         event.TxHash,
				LogIndex:       event.LogIndex,
			}

			if err := deps.Builder.ProcessSwap(r.Context(), swap); err != nil {
				errCount++
				retryable = true
				deps.Logger.Error("failed to process swap for candles",
					slog.String("err", err.Error()), slog.String("txHash", event.TxHash))
			} else {
				processed++
			}
		}

		status := http.StatusOK
		if retryable {
			status = http.StatusServiceUnavailable
			w.Header().Set("Retry-After", "1")
		} else if errCount > 0 {
			status = http.StatusUnprocessableEntity
		}
		sharedhttp.WriteJSON(w, status, map[string]interface{}{
			"ok":        errCount == 0,
			"processed": processed,
			"errors":    errCount,
			"retryable": retryable,
		})
	}
}

func validWebhookSwap(event WebhookEvent) bool {
	if event.BlockTimestamp < 1704067200 || event.TxHash == "" || event.LogIndex < 0 || asString(event.Args["poolAddress"]) == "" || asString(event.Args["sender"]) == "" {
		return false
	}
	if _, ok := event.Args["isBuy"].(bool); !ok {
		return false
	}
	for _, key := range []string{"amountIn", "amountOut", "price"} {
		if _, err := engine.RawUint256(asString(event.Args[key])); err != nil {
			return false
		}
	}
	return true
}

// Decode incrementally so an oversized batch never allocates an unbounded event slice.
func decodeWebhookBody(body []byte) (*WebhookBody, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("body must be an object")
	}
	var wb WebhookBody
	seenEvents := false
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if key != "events" {
			var ignored json.RawMessage
			if err := dec.Decode(&ignored); err != nil {
				return nil, err
			}
			continue
		}
		if seenEvents {
			return nil, errors.New("duplicate events field")
		}
		seenEvents = true
		token, err = dec.Token()
		if err != nil || token != json.Delim('[') {
			return nil, errors.New("events must be an array")
		}
		wb.Events = make([]WebhookEvent, 0)
		for dec.More() {
			if len(wb.Events) == MaxWebhookEvents {
				return nil, errWebhookTooLarge
			}
			var event WebhookEvent
			if err := dec.Decode(&event); err != nil {
				return nil, err
			}
			wb.Events = append(wb.Events, event)
		}
		if _, err = dec.Token(); err != nil {
			return nil, err
		}
	}
	if _, err = dec.Token(); err != nil {
		return nil, err
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, errors.New("unexpected trailing JSON")
	}
	if !seenEvents {
		return nil, errors.New("events must be an array")
	}
	return &wb, nil
}

// webhookAuth is the HMAC verification middleware for /webhooks/* routes.
func webhookAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sig := r.Header.Get("X-Sidiora-Signature")
			ts := r.Header.Get("X-Sidiora-Timestamp")
			if sig == "" {
				sig = r.Header.Get("X-Hub-Signature-256")
			}

			r.Body = http.MaxBytesReader(w, r.Body, MaxWebhookBodyBytes)
			defer r.Body.Close()
			// Limit bytes before buffering, parsing or HMAC verification.
			body, err := io.ReadAll(r.Body)
			if err != nil {
				var oversized *http.MaxBytesError
				if errors.As(err, &oversized) {
					sharedhttp.WriteError(w, 413, "Payload Too Large", errWebhookTooLarge.Error())
					return
				}
				sharedhttp.WriteError(w, 400, "Bad Request", "failed to read body")
				return
			}
			wb, err := decodeWebhookBody(body)
			if err != nil {
				if errors.Is(err, errWebhookTooLarge) {
					sharedhttp.WriteError(w, 413, "Payload Too Large", errWebhookTooLarge.Error())
				} else {
					sharedhttp.WriteError(w, 400, "Bad Request", "invalid JSON body or events array")
				}
				return
			}

			if err := auth.VerifyWebhook(secret, ts, string(body), sig, time.Now(), 0); err != nil {
				sharedhttp.WriteError(w, 401, "Unauthorized", "invalid webhook signature")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), webhookBodyKey{}, wb)))
		})
	}
}
