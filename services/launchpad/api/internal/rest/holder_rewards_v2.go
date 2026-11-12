package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func holderRewardsStats(st *store.Store) http.HandlerFunc {
	if st == nil {
		return holderRewardsStatsFrom(nil)
	}
	return holderRewardsStatsFrom(st.HolderRewardsStats)
}

// holderRewardsStatsFrom serves the stats document from load; a nil load is
// an unavailable dependency.
func holderRewardsStatsFrom(load func(context.Context) (*store.HolderRewardsStats, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Query()) != 0 {
			WriteV2APIError(w, r, 400, literalQUERYINVALID)
			return
		}
		if load == nil {
			WriteV2NotConfiguredError(w, r)
			return
		}
		stats, err := load(r.Context())
		if err != nil {
			writeComputedReadError(w, r, err)
			return
		}
		generated := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		data := map[string]any{"cumulativeDistributedRaw": string(stats.CumulativeDistributedRaw), "walletsRewarded": stats.WalletsRewarded, literalWatermark: stats.Watermark, literalGeneratedAt: generated}
		writeComputedEnvelope(w, r, "HolderRewardsStatsResponse", data, stats.Watermark, stats.Watermark.ProjectionRevision, "complete", nil)
	}
}

func holderRewardsPools(st *store.Store) http.HandlerFunc {
	if st == nil {
		return holderRewardsPoolsFrom(nil)
	}
	return holderRewardsPoolsFrom(st.HolderRewardPools)
}

// holderRewardsPoolsFrom serves the ranked pool list from load; a nil load is
// an unavailable dependency.
func holderRewardsPoolsFrom(load func(context.Context, string) (*store.HolderRewardPoolsSnapshot, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for key, values := range q {
			if (key != literalSort && key != literalCursor && key != literalLimit) || len(values) != 1 {
				WriteV2APIError(w, r, 422, "FIELD_UNKNOWN")
				return
			}
		}
		limit, err := boundedInteger(q.Get(literalLimit), 50, 1, 100)
		if err != nil {
			WriteV2APIError(w, r, 422, literalFIELDINVALID)
			return
		}
		sortKey := "rewards"
		if values, present := q[literalSort]; present {
			sortKey = values[0]
		}
		shape := map[string]any{literalSort: sortKey, literalLimit: limit}
		token, hasCursor := q[literalCursor]
		if hasCursor {
			if token[0] == "" {
				WriteV2APIError(w, r, 400, literalCURSORINVALID)
				return
			}
			shape[literalCursor] = token[0]
		}
		raw, _ := json.Marshal(shape)
		if interfaces.ValidateJSON("HolderRewardsPoolsRequest", raw) != nil {
			WriteV2APIError(w, r, 422, literalFIELDINVALID)
			return
		}
		if load == nil {
			WriteV2NotConfiguredError(w, r)
			return
		}
		snapshot, err := load(r.Context(), sortKey)
		if err != nil {
			writeComputedReadError(w, r, err)
			return
		}
		offset := 0
		if hasCursor {
			cursor, cursorErr := decodePoolListCursor(token[0])
			if cursorErr != nil {
				WriteV2APIError(w, r, 400, literalCURSORINVALID)
				return
			}
			if cursor.SnapshotID != snapshot.SnapshotID {
				WriteV2APIError(w, r, 409, "RESNAPSHOT_REQUIRED")
				return
			}
			if cursor.Offset > len(snapshot.Items) {
				WriteV2APIError(w, r, 400, literalCURSORINVALID)
				return
			}
			offset = cursor.Offset
		}
		end := offset + limit
		if end > len(snapshot.Items) {
			end = len(snapshot.Items)
		}
		items := []map[string]any{}
		for _, item := range snapshot.Items[offset:end] {
			items = append(items, item.Public())
		}
		var next any
		if end < len(snapshot.Items) {
			next = encodePoolListCursor(poolListCursor{SnapshotID: snapshot.SnapshotID, Offset: end})
		}
		writeComputedEnvelope(w, r, "HolderRewardsPoolsResponse", map[string]any{literalItems: items}, snapshot.Watermark, snapshot.Watermark.ProjectionRevision, snapshot.Quality, next)
	}
}
