package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

// RegisterPoolTransactions registers GET /stats/{poolAddress}/transactions.
func RegisterPoolTransactions(r chi.Router, st *store.Store) {
	r.Get("/stats/{poolAddress}/transactions", poolTransactions(st))
}

// poolTransactions serves a pool's transaction history, newest first, optionally
// filtered by side (type=buy|sell|all). Ports the TS pool-transactions route.
func poolTransactions(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolAddress := strings.ToLower(chi.URLParam(r, valuePoolAddress))
		q := r.URL.Query()
		page, err := boundedPage(q, 50, legacyPageMaximum, 1, "transactions:"+poolAddress+":"+q.Get("type"))
		if err != nil {
			sharedhttp.WriteError(w, 400, "Bad Request", err.Error())
			return
		}
		if page.After != nil {
			sharedhttp.WriteError(w, 400, "Bad Request", "cursor is not supported on this route")
			return
		}
		limit, offset := page.Limit, page.Offset
		txType := q.Get("type")
		if txType == "" {
			txType = "all"
		}

		txs, err := st.ListTransactions(r.Context(), poolAddress, limit, offset, txType)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "transactions lookup failed")
			return
		}

		sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
			valueTransactions: txs,
			valueLimit:        limit,
			valueOffset:       offset,
		})
	}
}
