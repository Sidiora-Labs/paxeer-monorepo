package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/google/uuid"

	"github.com/go-chi/chi/v5"

	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

var (
	bigTotalSupply = func() *big.Int { v, _ := new(big.Int).SetString(shareddb.TotalSupplyRaw, 10); return v }()
	bigTenThousand = big.NewInt(10000)
)

// holderWithRank is a holder row plus its 1-based rank (parity with the TS
// `{ ...h, rank }` enrichment; embedded fields marshal inline before rank).
type holderWithRank struct {
	store.HolderRow
	Rank int `json:"rank"`
}

// walletEntry is one entry of the distribution walletMap.
type walletEntry struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
	PctBps  int64  `json:"pctBps"`
	Rank    int    `json:"rank"`
}

// bracketOut is one distribution bracket in the response.
type bracketOut struct {
	Label              string `json:"label"`
	Count              int    `json:"count"`
	TotalBalancePctBps int64  `json:"totalBalancePctBps"`
}

// RegisterPoolHolders registers the holder list + distribution routes.
func RegisterPoolHolders(r chi.Router, st *store.Store, schemas ...string) {
	r.Get("/stats/{poolAddress}/holders/distribution", canonicalDistribution(st, ownershipSchema(schemas)))
	r.Get("/stats/{poolAddress}/holders", holderList(st))
}

// holderList serves a paginated, rank-enriched holder list. Ports the TS
// GET /stats/:pool/holders route.
func holderList(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolAddress := chi.URLParam(r, valuePoolAddress)
		ctx := r.Context()
		page, err := boundedPage(r.URL.Query(), 50, legacyPageMaximum, 1, "holders:"+poolAddress)
		if err != nil {
			sharedhttp.WriteError(w, 400, "Bad Request", err.Error())
			return
		}
		if page.After != nil {
			sharedhttp.WriteError(w, 400, "Bad Request", "use the canonical holder route for cursor pagination")
			return
		}
		limit, offset := page.Limit, page.Offset

		holders, err := st.ListHolders(ctx, poolAddress, limit, offset)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "holders lookup failed")
			return
		}
		total, err := st.CountHolders(ctx, poolAddress)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "holders count failed")
			return
		}

		enriched := make([]holderWithRank, len(holders))
		for i, h := range holders {
			enriched[i] = holderWithRank{HolderRow: h, Rank: offset + i + 1}
		}

		sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
			valueCoverage:        valueLegacy_incomplete,
			valueOwnershipSource: valueSwap_sender_legacy,
			"holders":            enriched,
			"total":              total,
			valueLimit:           limit,
			valueOffset:          offset,
		})
	}
}

// pctOfSupplyBps returns Number((balance * 10000) / TOTAL_SUPPLY_RAW) as an
// integer (range [0,10000]), matching the TS distribution math.
func pctOfSupplyBps(balance *big.Int) int64 {
	if bigTotalSupply.Sign() <= 0 {
		return 0
	}
	v := new(big.Int).Mul(balance, bigTenThousand)
	v.Quo(v, bigTotalSupply)
	return v.Int64()
}

// holderDistribution serves the detailed holder distribution: brackets by % of
// supply, top-10/20/50 concentration, and a paginated wallet map. Ports the TS
// GET /stats/:pool/holders/distribution route (S-6 pagination).
func holderDistribution(st *store.Store) http.HandlerFunc {
	return canonicalDistribution(st, defaultOwnershipSchema)
}

func RegisterTransferHolders(router chi.Router, st *store.Store, repository *holdings.Repository) {
	router.Get("/v1/pools/{poolAddress}/holders", TransferHolders(st, repository))
}
func TransferHolders(st *store.Store, repository *holdings.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		originStarted := time.Now()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=1")
		requestID := uuid.NewString()
		fail := func(status int, code string) {
			w.Header().Set("Cache-Control", "private, no-store")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{valueCode: code, valueMessage: code, valueRequestId: requestID, valueRetryable: status == 503, valueRetryAfterMs: nil, valueFieldErrors: []any{}, valueDetails: nil})
		}
		if repository == nil || st == nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		pool, err := holdings.Address(chi.URLParam(r, valuePoolAddress))
		if err != nil {
			fail(422, "FIELD_INVALID")
			return
		}
		query := r.URL.Query()
		for key, values := range query {
			if (key != valueLimit && key != valueCursor) || len(values) != 1 {
				fail(422, "FIELD_UNKNOWN")
				return
			}
		}
		limit := 50
		if value, ok := query[valueLimit]; ok {
			limit, err = strconv.Atoi(value[0])
			if err != nil || limit < 1 || limit > 100 {
				fail(422, "FIELD_INVALID")
				return
			}
		}
		context, err := st.GetPoolStats(r.Context(), pool)
		if err != nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		if context == nil {
			fail(404, "NOT_FOUND")
			return
		}
		token, err := holdings.Address(context.TokenAddress)
		if err != nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		creator := ""
		if context.CreatorAddress != nil {
			creator = *context.CreatorAddress
		}
		cursor := query.Get(valueCursor)
		if _, exists := query[valueCursor]; exists && cursor == "" {
			fail(400, "CURSOR_INVALID")
			return
		}
		if cursor == "" {
			cursor, err = repository.Materialize(r.Context(), token, pool, creator)
			if err != nil {
				if errors.Is(err, holdings.ErrUnknownToken) {
					fail(503, "DEPENDENCY_UNAVAILABLE")
				} else {
					fail(503, "DEPENDENCY_UNAVAILABLE")
				}
				return
			}
		}
		page, err := repository.ReadPage(r.Context(), token, pool, cursor, limit)
		if err != nil {
			if errors.Is(err, holdings.ErrRestartRequired) {
				fail(409, "RESNAPSHOT_REQUIRED")
			} else if errors.Is(err, holdings.ErrCursor) {
				fail(400, "CURSOR_INVALID")
			} else {
				fail(503, "DEPENDENCY_UNAVAILABLE")
			}
			return
		}
		items := []map[string]any{}
		for _, balance := range page.Snapshot.Balances {
			var share any
			ratio := holdings.BasisPoints(balance.Raw, page.Snapshot.Supply)
			if page.Snapshot.Coverage == valueComplete && ratio.BasisPoints != nil {
				share = map[string]any{"numerator": balance.Raw, "denominator": page.Snapshot.Supply, "basisPoints": *ratio.BasisPoints, "denominatorPolicy": "indexed_supply_v1", "excludedCategories": []string{}}
			}
			items = append(items, map[string]any{valueWallet: balance.Address, valueTokenAddress: token, "balanceRaw": balance.Raw, "share": share})
		}
		coverage := valuePartial
		if page.Snapshot.Coverage == valueComplete {
			coverage = valueComplete
		}
		if page.Snapshot.Coverage == valueQuarantined {
			coverage = "unavailable"
		}
		revision := strconv.FormatInt(page.Snapshot.Revision, 10)
		var wireRevision any
		if revision != "0" {
			wireRevision = revision
		}
		data := map[string]any{valueItems: items, valueCoverage: coverage, "coverageReason": page.Snapshot.Coverage, "fromPosition": page.Snapshot.StartPosition, "toPosition": page.Snapshot.Position, "exclusionPolicyVersion": "inclusive_indexed_supply_v1", "excludedProtocolAddresses": []string{}, valueTokenAddress: token, valuePoolAddress: pool, "decimals": page.Snapshot.Decimals, "supplyRaw": page.Snapshot.Supply, "snapshotId": page.SnapshotID, "distribution": page.Distribution, "lastTransferProvenance": page.Provenance, valueStale: page.Stale, "staleReason": page.StaleReason}
		var watermark any
		if page.Snapshot.Position != nil && page.Snapshot.Revision > 0 {
			position := page.Snapshot.Position
			integrity := "healthy"
			if page.Snapshot.Coverage == valueQuarantined {
				integrity = valueQuarantined
			}
			watermark = map[string]any{"chainId": "125", "height": position.Height, "blockHash": position.BlockHash, "projectionRevision": revision, "observedAt": page.ObservedAt, "integrity": integrity}
		}
		quality := coverageQuality(coverage)
		if page.Stale && quality == valueComplete {
			quality = valueStale
		}
		if page.Stale {
			w.Header().Set("Cache-Control", "private, no-store")
		}
		body, err := json.Marshal(map[string]any{valueRequestId: requestID, valueData: data, valueWatermark: watermark, valueRevision: wireRevision, valueQuality: quality, valueNextCursor: page.Next, valueGeneratedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z")})
		if err != nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		elapsed := time.Since(originStarted).Nanoseconds()
		w.Header().Set("Server-Timing", "origin;dur="+strconv.FormatInt(elapsed/1000000, 10)+"."+fmt.Sprintf("%06d", elapsed%1000000))
		w.WriteHeader(200)
		_, _ = w.Write(append(body, '\n'))
	}
}
func coverageQuality(coverage string) string {
	if coverage == valueComplete {
		return valueComplete
	}
	if coverage == "unavailable" {
		return valueQuarantined
	}
	return valuePartial
}

// devHoldersCursorPrefix tags the opaque development holder cursor; the cursor
// is base64url("stats-dev-holders:v1:<offset>").
const devHoldersCursorPrefix = "stats-dev-holders:v1:"

// maxDevHoldersOffset bounds how deep the development holder cursor may page.
const maxDevHoldersOffset = 1_000_000

// RegisterDevPoolHolders registers GET /v1/pools/{poolAddress}/holders outside
// production. Production serves this route from the transfer-coverage holdings
// repository (RegisterTransferHolders), whose schema is filled only by the
// production canonical catch-up job; the development pipeline never populates
// it. Development therefore answers the same IF_HOLDERS (HoldersResponse)
// contract from the webhook-driven ledger the dev consumers do write
// (stats.pool_stats for pool identity, stats.pool_holders for balances), with
// coverage "partial", no chain watermark and no supply share.
func RegisterDevPoolHolders(router chi.Router, st *store.Store) {
	router.Get("/v1/pools/{poolAddress}/holders", DevHolders(st))
}

// DevHolders is the development IF_HOLDERS handler; see RegisterDevPoolHolders.
// Request validation and error envelopes match TransferHolders.
func DevHolders(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=1")
		requestID := uuid.NewString()
		fail := func(status int, code string) {
			w.Header().Set("Cache-Control", "private, no-store")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{valueCode: code, valueMessage: code, valueRequestId: requestID, valueRetryable: status == 503, valueRetryAfterMs: nil, valueFieldErrors: []any{}, valueDetails: nil})
		}
		if st == nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		pool, err := holdings.Address(chi.URLParam(r, valuePoolAddress))
		if err != nil {
			fail(422, "FIELD_INVALID")
			return
		}
		query := r.URL.Query()
		for key, values := range query {
			if (key != valueLimit && key != valueCursor) || len(values) != 1 {
				fail(422, "FIELD_UNKNOWN")
				return
			}
		}
		limit := 50
		if value, ok := query[valueLimit]; ok {
			limit, err = strconv.Atoi(value[0])
			if err != nil || limit < 1 || limit > 100 {
				fail(422, "FIELD_INVALID")
				return
			}
		}
		offset := 0
		if _, exists := query[valueCursor]; exists {
			offset, err = decodeDevHoldersCursor(query.Get(valueCursor))
			if err != nil {
				fail(400, "CURSOR_INVALID")
				return
			}
		}
		stats, err := st.FindPoolStats(r.Context(), pool)
		if err != nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		if stats == nil {
			fail(404, "NOT_FOUND")
			return
		}
		token, err := holdings.Address(stats.TokenAddress)
		if err != nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		rows, err := st.ListHolderBalances(r.Context(), stats.PoolAddress, limit+1, offset)
		if err != nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		var next any
		if len(rows) > limit {
			rows = rows[:limit]
			next = encodeDevHoldersCursor(offset + limit)
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, map[string]any{valueWallet: row.Wallet, valueTokenAddress: token, "balanceRaw": row.BalanceRaw, "share": nil})
		}
		data := map[string]any{valueItems: items, valueCoverage: valuePartial, "coverageReason": "development_swap_ledger", valueOwnershipSource: valueSwap_sender_legacy, "fromPosition": nil, "toPosition": nil, "exclusionPolicyVersion": "inclusive_indexed_supply_v1", "excludedProtocolAddresses": []string{}, valueTokenAddress: token, valuePoolAddress: pool, valueStale: false, "staleReason": nil}
		body, err := json.Marshal(map[string]any{valueRequestId: requestID, valueData: data, valueWatermark: nil, valueRevision: nil, valueQuality: valuePartial, valueNextCursor: next, valueGeneratedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z")})
		if err != nil {
			fail(503, "DEPENDENCY_UNAVAILABLE")
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write(append(body, '\n'))
	}
}

func encodeDevHoldersCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(devHoldersCursorPrefix + strconv.Itoa(offset)))
}

func decodeDevHoldersCursor(cursor string) (int, error) {
	if cursor == "" || len(cursor) > 128 {
		return 0, errors.New("cursor invalid")
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || !strings.HasPrefix(string(raw), devHoldersCursorPrefix) {
		return 0, errors.New("cursor invalid")
	}
	digits := strings.TrimPrefix(string(raw), devHoldersCursorPrefix)
	offset, err := strconv.Atoi(digits)
	if err != nil || offset < 1 || offset > maxDevHoldersOffset || strconv.Itoa(offset) != digits {
		return 0, errors.New("cursor invalid")
	}
	return offset, nil
}
