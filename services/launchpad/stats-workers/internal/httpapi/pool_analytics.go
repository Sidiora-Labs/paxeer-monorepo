package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

// whaleThresholdBps is 1% of supply in basis points (parity WHALE_THRESHOLD_BPS).
const whaleThresholdBps = 100

// jsNumber mirrors JavaScript Number(string): "" -> 0, non-numeric -> NaN.
func jsNumber(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

// humanPct formats a basis-point value as "X.YY%" (parity with (bps/100).toFixed(2)).
func humanPct(bps float64) string {
	return strconv.FormatFloat(bps/100, 'f', 2, 64) + "%"
}

type whaleEntry struct {
	Rank             int     `json:"rank"`
	HolderAddress    string  `json:"holderAddress"`
	Balance          string  `json:"balance"`
	PctOfSupply      int64   `json:"pctOfSupply"`
	PctOfSupplyHuman string  `json:"pctOfSupplyHuman"`
	USDLValue        *string `json:"usdlValue"`
	LastUpdated      int64   `json:"lastUpdated"`
	IsCreator        bool    `json:"isCreator"`
}

// RegisterPoolAnalytics registers the whales / creator-activity / risk routes.
func RegisterPoolAnalytics(r chi.Router, st *store.Store, schemas ...string) {
	r.Get("/stats/{poolAddress}/whales", canonicalWhales(st, ownershipSchema(schemas)))
	r.Get("/stats/{poolAddress}/creator-activity", creatorActivity(st))
	r.Get("/stats/{poolAddress}/risk", riskBreakdown(st))
}

// whales reads a bounded numeric page of canonical holders at or above 1% of supply.
func whales(st *store.Store) http.HandlerFunc {
	return canonicalWhales(st, defaultOwnershipSchema)
}

// creatorActivity returns the creator's bounded transaction history, current balance
// and a buy/sell summary. Ports the TS creator-activity route.
func creatorActivity(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolAddress := chi.URLParam(r, "poolAddress")
		scope := "creator:" + strings.ToLower(poolAddress)
		page, err := boundedPage(r.URL.Query(), 50, creatorPageMaximum, 1, scope)
		if err != nil {
			sharedhttp.WriteError(w, 400, "Bad Request", err.Error())
			return
		}
		if page.After != nil && page.Head == nil {
			sharedhttp.WriteError(w, 400, "Bad Request", "creator cursor requires a history boundary")
			return
		}
		ctx := r.Context()

		row, err := st.GetPoolStats(ctx, poolAddress)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "creator activity lookup failed")
			return
		}
		if row == nil {
			sharedhttp.WriteError(w, http.StatusNotFound, "Not Found", "Pool not found")
			return
		}

		if row.CreatorAddress == nil {
			sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
				"coverage":        "legacy_incomplete",
				"ownershipSource": "swap_sender_legacy",
				"poolAddress":     poolAddress,
				"creatorAddress":  nil,
				"message":         "Creator address not recorded for this pool",
				"transactions":    []store.TransactionRow{},
				"summary":         nil,
			})
			return
		}

		creator := *row.CreatorAddress
		creatorLower := strings.ToLower(creator)

		history, err := st.CreatorTransactionPage(ctx, poolAddress, creatorLower, page.Limit+1, page.Offset, page.After, page.Head)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "creator activity lookup failed")
			return
		}

		currentBalance := "0"
		if bal, ok, err := st.GetHolderBalance(ctx, poolAddress, creatorLower); err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "creator activity lookup failed")
			return
		} else if ok {
			currentBalance = bal
		}

		txs := history.Transactions
		var next any
		if len(txs) > page.Limit {
			txs = txs[:page.Limit]
			last := txs[len(txs)-1]
			next = nextHistoryCursor(scope, store.HistoryPosition{Timestamp: last.BlockTimestamp, ID: last.ID}, history.Head)
		}

		creatorHoldingsBps := jsNumber(row.CreatorHoldingsPct)

		sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
			"coverage":                "legacy_incomplete",
			"ownershipSource":         "swap_sender_legacy",
			"poolAddress":             poolAddress,
			"creatorAddress":          creator,
			"createdAt":               row.CreatedAt,
			"currentBalance":          currentBalance,
			"currentHoldingsPct":      int64(creatorHoldingsBps),
			"currentHoldingsPctHuman": humanPct(creatorHoldingsBps),
			"summary":                 history.Summary,
			"limit":                   page.Limit,
			"offset":                  page.Offset,
			"nextCursor":              next,
			"transactions":            txs,
		})
	}
}

// riskBreakdown returns the detailed risk breakdown for a pool. Ports the TS
// risk route.
func riskBreakdown(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolAddress := chi.URLParam(r, "poolAddress")

		row, err := st.GetPoolStats(r.Context(), poolAddress)
		if err != nil {
			sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "risk lookup failed")
			return
		}
		if row == nil {
			sharedhttp.WriteError(w, http.StatusNotFound, "Not Found", "Pool not found")
			return
		}

		factorsRaw := "[]"
		if row.RiskFactors != nil {
			factorsRaw = *row.RiskFactors
		}
		var riskFactors []string
		if err := json.Unmarshal([]byte(factorsRaw), &riskFactors); err != nil {
			riskFactors = []string{}
		}

		riskLevel := "low"
		switch {
		case row.RiskRating >= 70:
			riskLevel = "high"
		case row.RiskRating >= 40:
			riskLevel = "medium"
		}

		top10Bps := jsNumber(row.Top10Concentration)
		creatorBps := jsNumber(row.CreatorHoldingsPct)

		var ageSeconds any
		if row.CreatedAt != 0 {
			ageSeconds = shareddb.NowSeconds() - row.CreatedAt
		}
		var creatorAddress any
		if row.CreatorAddress != nil {
			creatorAddress = *row.CreatorAddress
		}

		sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{
			"coverage":        "legacy_incomplete",
			"ownershipSource": "swap_sender_legacy",
			"poolAddress":     poolAddress,
			"riskRating":      row.RiskRating,
			"riskLevel":       riskLevel,
			"riskFactors":     riskFactors,
			"details": map[string]any{
				"holderCount":           row.HolderCount,
				"top10ConcentrationPct": humanPct(top10Bps),
				"creatorHoldingsPct":    humanPct(creatorBps),
				"creatorAddress":        creatorAddress,
				"hasCreatorSold":        containsStr(riskFactors, "creator_sold"),
				"isNew":                 containsStr(riskFactors, "new"),
				"ageSeconds":            ageSeconds,
			},
		})
	}
}

func containsStr(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}
