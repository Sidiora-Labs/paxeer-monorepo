package rest

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

// DevelopmentSeriesModelVersion labels candle documents built from the
// development chart builder state, with receipt-verified post-swap spot prices.
const DevelopmentSeriesModelVersion = "development-positioned-candles-v1"

const hsdTimestamp = "2006-01-02T15:04:05.000Z"

// devHistory serves GET /v2/pools/{poolAddress}/trades and /candles outside
// production. Requests are validated exactly as the production handler
// validates them (accountingRequest: TradesRequest / CandlesRequest).
//
// Candles use the development stream projector's committed history. Legacy
// rows without a canonical projection remain quarantined; empty scopes remain
// valid empty documents. Trades use the same receipt-verified commits as the stream.
func devHistory(st *store.Store, candles bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r, token, err := accountingRequest(req, candles)
		if err != nil {
			WriteV2APIError(w, req, 422, "FIELD_INVALID")
			return
		}
		if token != "" && !candles {
			// The development surface never issues cursors.
			WriteV2APIError(w, req, 400, "CURSOR_INVALID")
			return
		}
		ctx := req.Context()
		indexed, err := st.DevelopmentPoolIndexed(ctx, r.Pool)
		if err != nil {
			noteDevelopmentReadError(req, err)
			WriteV2APIError(w, req, 503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		if !indexed {
			WriteV2APIError(w, req, 404, "NOT_FOUND")
			return
		}
		if candles {
			serveDevelopmentCandles(w, req, st, r, token)
			return
		}
		items, err := st.DevelopmentTrades(ctx, r)
		if err != nil {
			noteDevelopmentReadError(req, err)
			code := "DEPENDENCY_UNAVAILABLE"
			if errors.Is(err, store.ErrDataIntegrity) {
				code = "INTEGRITY_QUARANTINED"
			}
			WriteV2APIError(w, req, 503, code)
			return
		}
		data := map[string]any{"items": items}
		writeHSDEnvelope(w, req, "TradesResponse", data, nil)
	}
}

// writeHSDEnvelope writes a v2 read envelope with no watermark or
// revision (the development stores carry neither) and validates it against
// the named contract document before sending it.
func writeHSDEnvelope(w http.ResponseWriter, req *http.Request, name string, data any, next *string) {
	body, err := json.Marshal(map[string]any{"requestId": responseRequestID(w), "data": data, "watermark": nil, "revision": nil, "quality": "partial", "nextCursor": next, "generatedAt": time.Now().UTC().Format(hsdTimestamp)})
	if err != nil || len(body) > store.PublicJSONMaxBytes || interfaces.ValidateJSON(name, body) != nil {
		WriteV2APIError(w, req, 503, "INTEGRITY_QUARANTINED")
		return
	}
	writeHSDBody(w, body, "no-store")
}

func writeHSDBody(w http.ResponseWriter, body []byte, cache string) {
	w.Header().Set("X-Schema-Version", "2")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", cache)
	_, _ = w.Write(body)
}

type developmentCandleCursor struct {
	Snapshot string `json:"snapshot"`
	Filter   string `json:"filter"`
	After    int64  `json:"after"`
}

func serveDevelopmentCandles(w http.ResponseWriter, req *http.Request, st *store.Store, r store.AccountingPageRequest, token string) {
	filter := hex.EncodeToString(accountingFilter(r, true))
	if token != "" {
		raw, err := base64.RawURLEncoding.DecodeString(token)
		var c developmentCandleCursor
		if err != nil || len(raw) > 512 || json.Unmarshal(raw, &c) != nil || c.Filter != filter || c.After < 0 || c.Snapshot == "" {
			WriteV2APIError(w, req, 400, "CURSOR_INVALID")
			return
		}
		r.ExpectedSnapshot = c.Snapshot
		r.AfterStart = &c.After
	}
	recorded, err := st.DevelopmentCandlesRecorded(req.Context(), r.Pool, r.Interval, (r.From+999)/1000, (r.To+999)/1000)
	if err != nil {
		noteDevelopmentReadError(req, err)
		WriteV2APIError(w, req, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	if !recorded {
		if token != "" {
			WriteV2APIError(w, req, 409, "RESNAPSHOT_REQUIRED")
			return
		}
		writeHSDEnvelope(w, req, "CandlesResponse", map[string]any{"items": []any{}, "snapshotId": uuid.NewString(), "seriesModelVersion": DevelopmentSeriesModelVersion}, nil)
		return
	}
	noteDevelopmentReadOperation(req, "DevelopmentCandlePage")
	page, err := st.DevelopmentCandlePage(req.Context(), r)
	if errors.Is(err, store.ErrDevelopmentCandleSnapshotChanged) {
		WriteV2APIError(w, req, 409, "RESNAPSHOT_REQUIRED")
		return
	}
	if errors.Is(err, store.ErrDataIntegrity) {
		noteDevelopmentReadError(req, err)
		WriteV2APIError(w, req, 503, "INTEGRITY_QUARANTINED")
		return
	}
	if err != nil {
		noteDevelopmentReadError(req, err)
		WriteV2APIError(w, req, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	var next any
	if page.NextStart != nil {
		raw, err := json.Marshal(developmentCandleCursor{Snapshot: page.SnapshotID, Filter: filter, After: *page.NextStart})
		if err != nil {
			noteDevelopmentReadError(req, err)
			WriteV2APIError(w, req, 503, "INTEGRITY_QUARANTINED")
			return
		}
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	body, err := json.Marshal(map[string]any{"requestId": responseRequestID(w), "data": map[string]any{"items": page.Items, "snapshotId": page.SnapshotID, "seriesModelVersion": page.SeriesModelVersion}, "watermark": page.Watermark, "revision": page.Revision, "quality": "partial", "nextCursor": next, "generatedAt": time.Now().UTC().Format(hsdTimestamp)})
	if err != nil || len(body) > store.PublicJSONMaxBytes || interfaces.ValidateJSON("CandlesResponse", body) != nil {
		WriteV2APIError(w, req, 503, "INTEGRITY_QUARANTINED")
		return
	}
	writeHSDBody(w, body, "no-store")
}
