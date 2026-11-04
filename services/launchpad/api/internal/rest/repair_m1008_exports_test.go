package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

// These test-only entrypoints exercise the production serializers and chain
// consumer without requiring the API's database peers in the bounded gate.
func RepairM1008RankingsHandler(value store.PublicProjection) http.Handler {
	return ResponseMetadata(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := rankingsRequest(w, r); ok {
			writeRankingsProjection(w, r, value, rankingsLimit(r))
		}
	}))
}

func RepairM1008CreationHandler(ctx context.Context, rpcURL string, row store.DevelopmentPool) (http.Handler, func(), error) {
	chain, err := newDevelopmentChain(ctx, rpcURL)
	if err != nil {
		return nil, nil, err
	}
	h := ResponseMetadata(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wm, err := chain.headWatermark(r.Context(), store.DevelopmentHead{Height: row.CreatedHeight, UpdatedAt: time.Unix(row.CreatedAt, 0).UTC()})
		if err != nil {
			WriteV2APIError(w, r, 503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		created, ok := chain.creationPositions(r.Context(), []store.DevelopmentPool{row})[row.Pool]
		if !ok {
			WriteV2APIError(w, r, 503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		document, err := developmentPoolDocument(row, created)
		if err != nil {
			writePoolReadError(w, r, err)
			return
		}
		writeMarketEnvelope(w, r, "PoolListResponse", map[string]any{"items": []any{document}}, &wm, "partial", nil)
	}))
	return h, chain.client.Close, nil
}
