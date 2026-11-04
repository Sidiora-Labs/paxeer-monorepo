package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
)

// WebhookEvent is one event in a webhook batch.
type WebhookEvent struct {
	EventName      string         `json:"eventName"`
	BlockNumber    int64          `json:"blockNumber"`
	BlockTimestamp int64          `json:"blockTimestamp"`
	TxHash         string         `json:"txHash"`
	LogIndex       int            `json:"logIndex"`
	Args           map[string]any `json:"args"`
}

// WebhookBody is the POST body for /webhooks/events.
type WebhookBody struct {
	Events []WebhookEvent `json:"events"`
}

// WebhookDeps holds the consumers the webhook receiver dispatches to.
type WebhookDeps struct {
	Swap     *consumer.SwapConsumer
	Market   *consumer.MarketConsumer
	State    *consumer.StateConsumer
	Holder   *consumer.HolderTracker
	Multihop *consumer.MultihopConsumer
	Logger   *slog.Logger
	Secret   string
}

// RegisterWebhook registers the HMAC-authenticated POST /webhooks/events receiver.
func RegisterWebhook(r chi.Router, deps WebhookDeps) {
	r.With(webhookAuth(deps.Secret)).Post("/webhooks/events", webhookHandler(deps))
}

// WebhookMaxBodyBytes and WebhookMaxEvents bound the wire payload and decoded
// batch independently. Senders retry the unchanged batch on non-2xx responses;
// swap and holder effects share the durable txHash-logIndex inbox transaction.
const (
	WebhookMaxBodyBytes = 1 << 20 // 1 MiB, including JSON whitespace
	WebhookMaxEvents    = 1000
)

type webhookBodyKey struct{}

var errWebhookEventLimit = errors.New("webhook event limit exceeded")

// webhookHandler acknowledges a batch only when every event succeeded. A 503
// asks the existing HTTP sender to retry the whole batch, including after restart.
func webhookHandler(deps WebhookDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := r.Context().Value(webhookBodyKey{}).([]byte)
		if !ok {
			sharedhttp.WriteError(w, http.StatusUnauthorized, "Unauthorized", "authenticated webhook body required")
			return
		}
		wb, err := decodeWebhookBody(body)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errWebhookEventLimit) {
				status = http.StatusRequestEntityTooLarge
			}
			sharedhttp.WriteError(w, status, http.StatusText(status), err.Error())
			return
		}

		ctx := r.Context()
		processed, errCount := 0, 0
		for _, ev := range wb.Events {
			if err := dispatchEvent(ctx, deps, ev); err != nil {
				errCount++
				deps.Logger.Error("failed to process webhook event",
					slog.String("event", ev.EventName), slog.String("txHash", ev.TxHash), slog.Any("err", err))
				continue
			}
			processed++
		}

		status := http.StatusOK
		if errCount != 0 {
			status = http.StatusServiceUnavailable
			w.Header().Set("Retry-After", "1")
		}
		sharedhttp.WriteJSON(w, status, map[string]any{
			"ok":        errCount == 0,
			"processed": processed,
			"errors":    errCount,
		})
	}
}

// dispatchEvent routes one event to its consumer(s). Unknown event names are a
// no-op success (parity).
func dispatchEvent(ctx context.Context, deps WebhookDeps, ev WebhookEvent) error {
	switch ev.EventName {
	case "Swap":
		return deps.Swap.ProcessWebhookEvent(ctx, consumer.SwapEvent{
			PoolAddress:    asString(ev.Args["poolAddress"]),
			Sender:         asString(ev.Args["sender"]),
			IsBuy:          asBool(ev.Args["isBuy"]),
			AmountIn:       asString(ev.Args["amountIn"]),
			AmountOut:      asString(ev.Args["amountOut"]),
			Price:          asString(ev.Args["price"]),
			Fee:            asString(ev.Args["fee"]),
			BlockTimestamp: ev.BlockTimestamp,
			TxHash:         ev.TxHash,
			LogIndex:       ev.LogIndex,
		}, deps.Holder)
	case "MarketCreated":
		return deps.Market.ProcessEvent(ctx, consumer.MarketEvent{
			Pool:    asString(ev.Args["pool"]),
			Token:   asString(ev.Args["token"]),
			Creator: asStringPtr(ev.Args["creator"]),
		})
	case "PoolStateUpdated":
		return deps.State.ProcessEvent(ctx, consumer.StateEvent{
			PoolAddress: asString(ev.Args["poolAddress"]),
			Price:       asString(ev.Args["price"]),
		})
	case "MultihopSwap":
		// Native Router cross-token swap (Router.swapTokenForToken). The on-chain
		// event emits sender/tokenIn/tokenOut/amountIn/intermediateUsdl/amountOut;
		// poolIn/poolOut/feeIn/feeOut are indexer-enriched (all NOT NULL).
		return deps.Multihop.ProcessEvent(ctx, consumer.MultihopEvent{
			Sender:           asString(ev.Args["sender"]),
			TokenIn:          asString(ev.Args["tokenIn"]),
			TokenOut:         asString(ev.Args["tokenOut"]),
			PoolIn:           asString(ev.Args["poolIn"]),
			PoolOut:          asString(ev.Args["poolOut"]),
			AmountIn:         asString(ev.Args["amountIn"]),
			IntermediateUsdl: asString(ev.Args["intermediateUsdl"]),
			AmountOut:        asString(ev.Args["amountOut"]),
			FeeIn:            asString(ev.Args["feeIn"]),
			FeeOut:           asString(ev.Args["feeOut"]),
			BlockTimestamp:   ev.BlockTimestamp,
			TxHash:           ev.TxHash,
			LogIndex:         ev.LogIndex,
		})
	default:
		return nil
	}
}

// webhookAuth verifies the HMAC signature on /webhooks/* requests, mirroring the
// shared registerWebhookAuth (header names + replay window). The bounded raw
// bytes are authenticated once and passed directly to the decoder in context.
func webhookAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sig := r.Header.Get("X-Sidiora-Signature")
			ts := r.Header.Get("X-Sidiora-Timestamp")
			if sig == "" {
				sig = r.Header.Get("X-Hub-Signature-256")
			}

			if sig == "" || ts == "" {
				sharedhttp.WriteError(w, http.StatusUnauthorized, "Unauthorized", "missing webhook signature or timestamp")
				return
			}
			if r.ContentLength > WebhookMaxBodyBytes {
				sharedhttp.WriteError(w, http.StatusRequestEntityTooLarge, "Request Entity Too Large", "webhook body exceeds 1 MiB")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, WebhookMaxBodyBytes)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				status := http.StatusBadRequest
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					status = http.StatusRequestEntityTooLarge
				}
				sharedhttp.WriteError(w, status, http.StatusText(status), "failed to read bounded webhook body")
				return
			}

			if err := auth.VerifyWebhook(secret, ts, string(body), sig, time.Now(), 0); err != nil {
				sharedhttp.WriteError(w, http.StatusUnauthorized, "Unauthorized", "invalid webhook signature")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), webhookBodyKey{}, body)))
		})
	}
}

// Decode incrementally so even a compact batch cannot allocate more than the
// declared event count. Reject the entire batch before applying any event.
func decodeWebhookBody(body []byte) (WebhookBody, error) {
	var result WebhookBody
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return result, fmt.Errorf("invalid JSON body")
	}
	seen := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return result, fmt.Errorf("invalid JSON body")
		}
		if key != "events" {
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return result, fmt.Errorf("invalid JSON body")
			}
			continue
		}
		if seen {
			return result, fmt.Errorf("duplicate events field")
		}
		seen = true
		token, err := decoder.Token()
		if err != nil || token != json.Delim('[') {
			return result, fmt.Errorf("events must be an array")
		}
		result.Events = []WebhookEvent{}
		for decoder.More() {
			if len(result.Events) == WebhookMaxEvents {
				return result, errWebhookEventLimit
			}
			var event WebhookEvent
			if err := decoder.Decode(&event); err != nil {
				return result, fmt.Errorf("invalid webhook event")
			}
			result.Events = append(result.Events, event)
		}
		if _, err := decoder.Token(); err != nil {
			return result, fmt.Errorf("invalid JSON body")
		}
	}
	if _, err := decoder.Token(); err != nil {
		return result, fmt.Errorf("invalid JSON body")
	}
	if !seen {
		return result, fmt.Errorf("events must be an array")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return result, fmt.Errorf("invalid JSON body")
	}
	return result, nil
}
