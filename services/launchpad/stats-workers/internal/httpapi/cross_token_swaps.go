package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

// RegisterCrossTokenSwaps registers the multihop swap-history routes. The static
// /token/ segment is registered before the {wallet} param route so chi resolves
// it first.
func RegisterCrossTokenSwaps(r chi.Router, st *store.Store) {
	r.Get("/stats/cross-token-swaps/token/{tokenAddress}", crossSwapsByToken(st))
	r.Get("/stats/cross-token-swaps/{wallet}", crossSwapsByWallet(st))
}

// crossSwapsByWallet serves a wallet's multihop swap history. Ports the TS route.
func crossSwapsByWallet(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wallet := chi.URLParam(r, valueWallet)
		scope := "cross-wallet:" + strings.ToLower(wallet)
		page, err := boundedPage(r.URL.Query(), 50, legacyPageMaximum, 1, scope)
		if err != nil {
			sharedhttp.WriteError(w, 400, "Bad Request", err.Error())
			return
		}
		limit, offset := page.Limit, page.Offset

		swaps, err := st.CrossTokenSwapPage(r.Context(), strings.ToLower(wallet), false, limit+1, offset, page.After)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "cross-token swaps lookup failed")
			return
		}
		var next any
		if len(swaps) > limit {
			swaps = swaps[:limit]
			last := swaps[len(swaps)-1]
			next = nextHistoryCursor(scope, store.HistoryPosition{Timestamp: last.BlockTimestamp, ID: last.ID}, nil)
		}
		sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
			valueNextCursor: next,
			valueWallet:     wallet,
			"swaps":         swaps,
			valueLimit:      limit,
			valueOffset:     offset,
		})
	}
}

// crossSwapsByToken serves all multihop swaps involving a token. Ports the TS route.
func crossSwapsByToken(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenAddress := chi.URLParam(r, valueTokenAddress)
		scope := "cross-token:" + strings.ToLower(tokenAddress)
		page, err := boundedPage(r.URL.Query(), 50, legacyPageMaximum, 1, scope)
		if err != nil {
			sharedhttp.WriteError(w, 400, "Bad Request", err.Error())
			return
		}
		limit, offset := page.Limit, page.Offset

		swaps, err := st.CrossTokenSwapPage(r.Context(), strings.ToLower(tokenAddress), true, limit+1, offset, page.After)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "cross-token swaps lookup failed")
			return
		}
		var next any
		if len(swaps) > limit {
			swaps = swaps[:limit]
			last := swaps[len(swaps)-1]
			next = nextHistoryCursor(scope, store.HistoryPosition{Timestamp: last.BlockTimestamp, ID: last.ID}, nil)
		}
		sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
			valueNextCursor:   next,
			valueTokenAddress: tokenAddress,
			"swaps":           swaps,
			valueLimit:        limit,
			valueOffset:       offset,
		})
	}
}
