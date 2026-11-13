package rest

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

// DevelopmentSeriesModelVersion labels candle documents built from the
// development chart builder state, with receipt-verified post-swap spot prices.
const DevelopmentSeriesModelVersion = "development-positioned-candles-v1"

const hsdTimestamp = "2006-01-02T15:04:05.000Z"

// DevelopmentHistoryOptions supplies the development store and the same chain
// head source used by the development market reads.
type DevelopmentHistoryOptions struct {
	Store       *store.Store
	Admission   *Admission
	ChainRPCURL string
}

// devHistory serves GET /v2/pools/{poolAddress}/trades and /candles outside
// production. Requests are validated exactly as the production handler
// validates them (accountingRequest: TradesRequest / CandlesRequest).
//
// Candles use the development stream projector's committed history. Legacy
// rows without a canonical projection remain quarantined; empty scopes remain
// valid empty documents. Trades use the same receipt-verified commits as the stream.
func devHistory(d *developmentMarket, candles bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r, token, err := accountingRequest(req, candles)
		if err != nil {
			WriteV2APIError(w, req, 422, literalFIELDINVALID)
			return
		}
		if token != "" && !candles {
			// The development surface never issues cursors.
			WriteV2APIError(w, req, 400, literalCURSORINVALID)
			return
		}
		ctx := req.Context()
		indexed, err := d.st.DevelopmentPoolIndexed(ctx, r.Pool)
		if err != nil {
			noteDevelopmentReadError(req, err)
			WriteV2APIError(w, req, 503, literalDEPENDENCYUNAVAILABLE)
			return
		}
		if !indexed {
			WriteV2APIError(w, req, 404, literalNOTFOUND)
			return
		}
		if candles {
			serveDevelopmentCandles(w, req, d.st, r, token)
			return
		}
		items, err := d.st.DevelopmentTrades(ctx, r)
		if err != nil {
			noteDevelopmentReadError(req, err)
			code := literalDEPENDENCYUNAVAILABLE
			if errors.Is(err, store.ErrDataIntegrity) {
				code = literalINTEGRITYQUARANTINED
			}
			WriteV2APIError(w, req, 503, code)
			return
		}
		head, headErr := d.st.DevelopmentIndexerHead(ctx)
		wm := d.optionalWatermark(ctx, head, headErr)
		if wm != nil {
			filtered := make([]json.RawMessage, 0, len(items))
			height, err := strconv.ParseUint(wm.Height, 10, 64)
			if err != nil {
				WriteV2APIError(w, req, 503, literalINTEGRITYQUARANTINED)
				return
			}
			for _, raw := range items {
				var trade struct {
					Position struct {
						Height    string `json:"height"`
						BlockHash string `json:"blockHash"`
					} `json:"position"`
				}
				if json.Unmarshal(raw, &trade) != nil {
					WriteV2APIError(w, req, 503, literalINTEGRITYQUARANTINED)
					return
				}
				tradeHeight, err := strconv.ParseUint(trade.Position.Height, 10, 64)
				if err != nil {
					WriteV2APIError(w, req, 503, literalINTEGRITYQUARANTINED)
					return
				}
				if tradeHeight < height || tradeHeight == height && strings.EqualFold(trade.Position.BlockHash, wm.BlockHash) {
					filtered = append(filtered, raw)
				}
			}
			items = filtered
		}
		data := map[string]any{literalItems: items}
		writeMarketEnvelope(w, req, "TradesResponse", data, wm, literalPartial, nil)
	}
}

// writeHSDEnvelope writes a v2 read envelope with no watermark or
// revision (the development stores carry neither) and validates it against
// the named contract document before sending it.
func writeHSDEnvelope(w http.ResponseWriter, req *http.Request, name string, data any, next *string) {
	body, err := json.Marshal(map[string]any{literalRequestId: responseRequestID(w), literalData: data, literalWatermark: nil, literalRevision: nil, literalQuality: literalPartial, literalNextCursor: next, literalGeneratedAt: time.Now().UTC().Format(hsdTimestamp)})
	if err != nil || len(body) > store.PublicJSONMaxBytes || interfaces.ValidateJSON(name, body) != nil {
		WriteV2APIError(w, req, 503, literalINTEGRITYQUARANTINED)
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
			WriteV2APIError(w, req, 400, literalCURSORINVALID)
			return
		}
		r.ExpectedSnapshot = c.Snapshot
		r.AfterStart = &c.After
	}
	recorded, err := st.DevelopmentCandlesRecorded(req.Context(), r.Pool, r.Interval, (r.From+999)/1000, (r.To+999)/1000)
	if err != nil {
		noteDevelopmentReadError(req, err)
		WriteV2APIError(w, req, 503, literalDEPENDENCYUNAVAILABLE)
		return
	}
	if !recorded {
		if token != "" {
			WriteV2APIError(w, req, 409, "RESNAPSHOT_REQUIRED")
			return
		}
		writeHSDEnvelope(w, req, "CandlesResponse", map[string]any{literalItems: []any{}, literalSnapshotId: uuid.NewString(), literalSeriesModelVersion: DevelopmentSeriesModelVersion}, nil)
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
		WriteV2APIError(w, req, 503, literalINTEGRITYQUARANTINED)
		return
	}
	if err != nil {
		noteDevelopmentReadError(req, err)
		WriteV2APIError(w, req, 503, literalDEPENDENCYUNAVAILABLE)
		return
	}
	var next any
	if page.NextStart != nil {
		raw, err := json.Marshal(developmentCandleCursor{Snapshot: page.SnapshotID, Filter: filter, After: *page.NextStart})
		if err != nil {
			noteDevelopmentReadError(req, err)
			WriteV2APIError(w, req, 503, literalINTEGRITYQUARANTINED)
			return
		}
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	body, err := json.Marshal(map[string]any{literalRequestId: responseRequestID(w), literalData: map[string]any{literalItems: page.Items, literalSnapshotId: page.SnapshotID, literalSeriesModelVersion: page.SeriesModelVersion}, literalWatermark: page.Watermark, literalRevision: page.Revision, literalQuality: literalPartial, literalNextCursor: next, literalGeneratedAt: time.Now().UTC().Format(hsdTimestamp)})
	if err != nil || len(body) > store.PublicJSONMaxBytes || interfaces.ValidateJSON("CandlesResponse", body) != nil {
		WriteV2APIError(w, req, 503, literalINTEGRITYQUARANTINED)
		return
	}
	writeHSDBody(w, body, "no-store")
}
