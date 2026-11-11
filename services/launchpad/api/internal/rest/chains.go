package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func chainParameter(w http.ResponseWriter, r *http.Request) (uint32, bool) {
	raw := chi.URLParam(r, "chain")
	number, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || number == 0 || strconv.FormatUint(number, 10) != raw {
		WriteV2APIError(w, r, 422, "FIELD_INVALID")
		return 0, false
	}
	return uint32(number), true
}

func chainPageRequest(w http.ResponseWriter, r *http.Request, holderAllowed bool) (int, poolListCursor, string, bool) {
	q := r.URL.Query()
	for key, values := range q {
		if len(values) != 1 || key != "limit" && key != "cursor" && !(holderAllowed && key == "holder") {
			WriteV2APIError(w, r, 422, "FIELD_UNKNOWN")
			return 0, poolListCursor{}, "", false
		}
	}
	limit, err := boundedInteger(q.Get("limit"), 50, 1, 100)
	if err != nil {
		WriteV2APIError(w, r, 422, "FIELD_INVALID")
		return 0, poolListCursor{}, "", false
	}
	var cursor poolListCursor
	if tokens, present := q["cursor"]; present {
		cursor, err = decodePoolListCursor(tokens[0])
		if err != nil {
			WriteV2APIError(w, r, 400, "CURSOR_INVALID")
			return 0, poolListCursor{}, "", false
		}
	}
	holder := ""
	if values, present := q["holder"]; present {
		holder, err = store.Address(values[0])
		if err != nil {
			WriteV2APIError(w, r, 422, "FIELD_INVALID")
			return 0, poolListCursor{}, "", false
		}
	}
	return limit, cursor, holder, true
}

func writeChainEnvelope(w http.ResponseWriter, r *http.Request, data any, next any) {
	body, err := json.Marshal(map[string]any{
		"requestId": responseRequestID(w), "data": data,
		"revision": nil, "watermark": nil, "quality": "complete",
		"nextCursor": next, "generatedAt": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
	if err != nil || store.ValidatePublicJSON(body) != nil {
		WriteV2APIError(w, r, 503, "INTEGRITY_QUARANTINED")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

func chainsList(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, cursor, _, ok := chainPageRequest(w, r, false)
		if !ok {
			return
		}
		if st == nil {
			WriteV2NotConfiguredError(w, r)
			return
		}
		page, err := st.LaunchedChains(r.Context(), limit, cursor.Offset)
		if err != nil {
			writeComputedReadError(w, r, err)
			return
		}
		snapshotID := "chains:" + page.SnapshotID
		if cursor.SnapshotID != "" && cursor.SnapshotID != snapshotID {
			WriteV2APIError(w, r, 409, "RESNAPSHOT_REQUIRED")
			return
		}
		if cursor.Offset > page.Total {
			WriteV2APIError(w, r, 400, "CURSOR_INVALID")
			return
		}
		var next any
		if end := cursor.Offset + len(page.Items); end < page.Total {
			next = encodePoolListCursor(poolListCursor{SnapshotID: snapshotID, Offset: end})
		}
		writeChainEnvelope(w, r, map[string]any{"items": page.Items}, next)
	}
}

func chainsDetail(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chain, ok := chainParameter(w, r)
		if !ok {
			return
		}
		if len(r.URL.Query()) != 0 {
			WriteV2APIError(w, r, 400, "QUERY_INVALID")
			return
		}
		if st == nil {
			WriteV2NotConfiguredError(w, r)
			return
		}
		row, err := st.LaunchedChain(r.Context(), chain)
		if err != nil {
			writeComputedReadError(w, r, err)
			return
		}
		if row == nil {
			WriteV2APIError(w, r, 404, "NOT_FOUND")
			return
		}
		writeChainEnvelope(w, r, map[string]any{"chain": row}, nil)
	}
}

func chainsRevenue(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chain, ok := chainParameter(w, r)
		if !ok {
			return
		}
		limit, cursor, holder, ok := chainPageRequest(w, r, true)
		if !ok {
			return
		}
		if st == nil {
			WriteV2NotConfiguredError(w, r)
			return
		}
		row, err := st.LaunchedChain(r.Context(), chain)
		if err != nil {
			writeComputedReadError(w, r, err)
			return
		}
		if row == nil {
			WriteV2APIError(w, r, 404, "NOT_FOUND")
			return
		}
		page, err := st.ChainRevenue(r.Context(), chain, holder, limit, cursor.Offset)
		if err != nil {
			writeComputedReadError(w, r, err)
			return
		}
		snapshotID := "revenue:" + strconv.FormatUint(uint64(chain), 10) + ":" + holder + ":" + page.SnapshotID
		if cursor.SnapshotID != "" && cursor.SnapshotID != snapshotID {
			WriteV2APIError(w, r, 409, "RESNAPSHOT_REQUIRED")
			return
		}
		if cursor.Offset > page.Total {
			WriteV2APIError(w, r, 400, "CURSOR_INVALID")
			return
		}
		var next any
		if end := cursor.Offset + len(page.Epochs); end < page.Total {
			next = encodePoolListCursor(poolListCursor{SnapshotID: snapshotID, Offset: end})
		}
		data := map[string]any{"chain": row.Chain, "epochs": page.Epochs}
		if holder != "" {
			data["holder"] = holder
			data["claims"] = page.Claims
		}
		writeChainEnvelope(w, r, data, next)
	}
}
