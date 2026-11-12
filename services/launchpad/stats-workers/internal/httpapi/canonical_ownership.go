package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const defaultOwnershipSchema = "stats_transfer_holdings"

type ownershipCursor struct {
	Scope    string `json:"scope"`
	Balance  string `json:"balance"`
	Address  string `json:"address"`
	Revision int64  `json:"revision"`
	Rank     int    `json:"rank"`
}

type ownershipRead struct {
	Holders      []walletEntry
	Next         *string
	Distribution holdings.Distribution
	Supply       string
	Coverage     string
	Revision     int64
	Offset       int
	Limit        int
	Creator      string
}

func ownershipSchema(schemas []string) string {
	if len(schemas) == 0 {
		return defaultOwnershipSchema
	}
	if len(schemas) != 1 {
		return ""
	}
	return schemas[0]
}

func readOwnership(ctx context.Context, st *store.Store, schema, pool string, request *http.Request, whalesOnly bool) (out ownershipRead, status int, err error) {
	status = 503
	if _, err = holdings.SchemaSQL(schema); err != nil || st == nil || st.Pool() == nil {
		return out, status, errors.New("canonical ownership dependency unavailable")
	}
	scope := "distribution:" + strings.ToLower(pool)
	def, maximum, minimum := 200, legacyPageMaximum, 0
	if whalesOnly {
		scope = "whales:" + strings.ToLower(pool)
		def, maximum, minimum = 50, whalePageMaximum, 1
	}
	q := request.URL.Query()
	rawCursor, cursorExists := q[valueCursor]
	q.Del(valueCursor)
	page, e := boundedPage(q, def, maximum, minimum, scope)
	if e != nil {
		return out, 400, e
	}
	var cursor *ownershipCursor
	if cursorExists {
		if len(rawCursor) != 1 || len(rawCursor[0]) > 1024 || page.Offset != 0 || len(q[valueOffset]) > 0 {
			return out, 400, errors.New("invalid ownership cursor")
		}
		raw, e := base64.RawURLEncoding.Strict().DecodeString(rawCursor[0])
		if e != nil {
			return out, 400, e
		}
		cursor = &ownershipCursor{}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(cursor) != nil || decoder.Decode(new(any)) != io.EOF || cursor.Scope != scope || cursor.Revision < 0 || cursor.Rank < 1 || cursor.Rank > int(^uint(0)>>1)-legacyPageMaximum-1 {
			return out, 400, errors.New("invalid ownership cursor")
		}
		if _, e = holdings.UInt(cursor.Balance); e != nil {
			return out, 400, e
		}
		if address, parseErr := holdings.Address(cursor.Address); parseErr != nil || address != cursor.Address {
			return out, 400, errors.New("invalid ownership cursor address")
		}
	}
	out.Limit, out.Offset = page.Limit, page.Offset
	row, e := st.GetPoolStats(ctx, strings.ToLower(pool))
	if e != nil {
		return out, status, e
	}
	if row == nil {
		return out, 404, pgx.ErrNoRows
	}
	if row.CreatorAddress != nil {
		out.Creator = strings.ToLower(*row.CreatorAddress)
	}
	schema = pgx.Identifier{schema}.Sanitize()
	tx, e := st.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, status, e
	}
	defer rollbackTransaction(context.WithoutCancel(ctx), tx)
	e = tx.QueryRow(ctx, "SELECT supply::text,coverage,revision FROM "+schema+".tokens WHERE token=$1", row.TokenAddress).Scan(&out.Supply, &out.Coverage, &out.Revision)
	if e != nil {
		return out, status, e
	}
	if out.Coverage == valueQuarantined {
		return out, status, errors.New("canonical ownership quarantined")
	}
	if cursor != nil && cursor.Revision != out.Revision {
		return out, 409, holdings.ErrRestartRequired
	}
	supply, e := holdings.UInt(out.Supply)
	if e != nil {
		return out, status, e
	}
	out.Distribution = holdings.Distribution{Policy: holdings.DistributionPolicy, Supply: out.Supply, InclusiveDenominator: holdings.Denominator{Raw: out.Supply, Policy: "indexed_supply_v1", ExcludedCategories: []string{}}, CirculatingDenominator: holdings.Denominator{Policy: "verified_external_wallets_v1", ExcludedCategories: []string{valueUnknown, "pool_reserve", "burn", "treasury", "router"}}}
	for i := range out.Distribution.Bands {
		out.Distribution.Bands[i].Raw = "0"
	}
	if !whalesOnly {
		rows, e := tx.Query(ctx, "SELECT category,band,population,raw::text FROM "+schema+".aggregates WHERE token=$1 ORDER BY category,band", row.TokenAddress)
		if e != nil {
			return out, status, e
		}
		total, circulating := new(big.Int), new(big.Int)
		for rows.Next() {
			var category, raw string
			var band int
			var count uint64
			if e = rows.Scan(&category, &band, &count, &raw); e != nil {
				rows.Close()
				return out, status, e
			}
			amount, parseErr := holdings.UInt(raw)
			if parseErr != nil || band < 0 || band > 9 {
				rows.Close()
				return out, status, holdings.ErrIntegrity
			}
			total.Add(total, amount)
			out.Distribution.AllPositiveCount += count
			out.Distribution.Bands[band].Count += count
			prior, _ := new(big.Int).SetString(out.Distribution.Bands[band].Raw, 10)
			out.Distribution.Bands[band].Raw = prior.Add(prior, amount).String()
			if category == "external_wallet" {
				out.Distribution.ExternalWalletCount += count
				circulating.Add(circulating, amount)
			}
			if category == valueUnknown {
				out.Distribution.UnknownCount += count
			}
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return out, status, e
		}
		if total.Cmp(supply) != 0 {
			return out, status, holdings.ErrIntegrity
		}
		out.Distribution.CirculatingRaw = circulating.String()
		out.Distribution.CirculatingDenominator.Raw = circulating.String()
		var raw string
		if e = tx.QueryRow(ctx, "SELECT COALESCE(sum(balance),0)::text FROM (SELECT balance FROM "+schema+".balances WHERE token=$1 ORDER BY balance DESC,address ASC LIMIT 10) leaders", row.TokenAddress).Scan(&raw); e != nil {
			return out, status, e
		}
		out.Distribution.Top10 = holdings.BasisPoints(raw, out.Supply)
		if e = tx.QueryRow(ctx, "SELECT COALESCE(sum(balance),0)::text FROM (SELECT balance FROM "+schema+".balances WHERE token=$1 AND category='external_wallet' ORDER BY balance DESC,address ASC LIMIT 10) leaders", row.TokenAddress).Scan(&raw); e != nil {
			return out, status, e
		}
		out.Distribution.CirculatingTop10 = holdings.BasisPoints(raw, out.Distribution.CirculatingRaw)
		out.Distribution.Creator = holdings.Ratio{Reason: "unknown_creator"}
		out.Distribution.CirculatingCreator = out.Distribution.Creator
		if out.Creator != "" {
			raw, category := "0", valueUnknown
			e = tx.QueryRow(ctx, "SELECT balance::text,category FROM "+schema+".balances WHERE token=$1 AND address=$2", row.TokenAddress, out.Creator).Scan(&raw, &category)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return out, status, e
			}
			out.Distribution.Creator = holdings.BasisPoints(raw, out.Supply)
			if category != "external_wallet" {
				raw = "0"
			}
			out.Distribution.CirculatingCreator = holdings.BasisPoints(raw, out.Distribution.CirculatingRaw)
		}
	}
	threshold := "0"
	if whalesOnly {
		threshold = new(big.Int).Quo(new(big.Int).Add(supply, big.NewInt(99)), big.NewInt(100)).String()
	}
	balance, address := "0", holdings.Zero
	rank := page.Offset
	if cursor != nil {
		balance, address = cursor.Balance, cursor.Address
		rank = cursor.Rank
	}
	rows, e := tx.Query(ctx, "SELECT address,balance::text FROM "+schema+".balances WHERE token=$1 AND balance>=$2::numeric AND ($5 OR balance<$3::numeric OR (balance=$3::numeric AND address>$4)) ORDER BY balance DESC,address ASC LIMIT $6 OFFSET $7", row.TokenAddress, threshold, balance, address, cursor == nil, page.Limit+1, page.Offset)
	if e != nil {
		return out, status, e
	}
	out.Holders = []walletEntry{}
	for rows.Next() {
		var item walletEntry
		if e = rows.Scan(&item.Address, &item.Balance); e != nil {
			rows.Close()
			return out, status, e
		}
		item.Rank = rank + len(out.Holders) + 1
		ratio := holdings.BasisPoints(item.Balance, out.Supply)
		if ratio.BasisPoints != nil {
			item.PctBps, e = strconv.ParseInt(*ratio.BasisPoints, 10, 64)
			if e != nil {
				rows.Close()
				return out, status, e
			}
		}
		out.Holders = append(out.Holders, item)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return out, status, e
	}
	if len(out.Holders) > page.Limit {
		out.Holders = out.Holders[:page.Limit]
		if page.Limit > 0 {
			last := out.Holders[len(out.Holders)-1]
			raw, _ := json.Marshal(ownershipCursor{Scope: scope, Balance: last.Balance, Address: last.Address, Revision: out.Revision, Rank: last.Rank})
			next := base64.RawURLEncoding.EncodeToString(raw)
			out.Next = &next
		}
	}
	if out.Coverage != valueComplete {
		unavailable := holdings.Ratio{Reason: out.Coverage}
		out.Distribution.Top10 = unavailable
		out.Distribution.CirculatingTop10 = unavailable
		out.Distribution.Creator = unavailable
		out.Distribution.CirculatingCreator = unavailable
	}
	if e = tx.Commit(ctx); e != nil {
		return out, status, e
	}
	return out, 200, nil
}

func canonicalDistribution(st *store.Store, schema string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, status, err := readOwnership(r.Context(), st, schema, chi.URLParam(r, valuePoolAddress), r, false)
		if err != nil {
			sharedhttp.WriteError(w, status, http.StatusText(status), "canonical distribution unavailable")
			return
		}
		sharedhttp.WriteJSON(w, 200, map[string]any{valueCoverage: out.Coverage, valueOwnershipSource: "canonical_transfers", "totalHolders": out.Distribution.AllPositiveCount, "distribution": out.Distribution, "walletMap": out.Holders, "walletMapTotal": out.Distribution.AllPositiveCount, "walletMapLimit": out.Limit, "walletMapOffset": out.Offset, valueNextCursor: out.Next, valueRevision: strconv.FormatInt(out.Revision, 10)})
	}
}
func canonicalWhales(st *store.Store, schema string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := chi.URLParam(r, valuePoolAddress)
		out, status, err := readOwnership(r.Context(), st, schema, pool, r, true)
		if err != nil {
			sharedhttp.WriteError(w, status, http.StatusText(status), "canonical whales unavailable")
			return
		}
		whales := []whaleEntry{}
		for _, item := range out.Holders {
			whales = append(whales, whaleEntry{Rank: item.Rank, HolderAddress: item.Address, Balance: item.Balance, PctOfSupply: item.PctBps, PctOfSupplyHuman: humanPct(float64(item.PctBps)), IsCreator: strings.EqualFold(item.Address, out.Creator)})
		}
		sharedhttp.WriteJSON(w, 200, map[string]any{valueCoverage: out.Coverage, valueOwnershipSource: "canonical_transfers", valuePoolAddress: pool, "whaleThresholdPct": 1, "whaleCount": len(whales), "whales": whales, valueLimit: out.Limit, valueOffset: out.Offset, valueNextCursor: out.Next, valueRevision: strconv.FormatInt(out.Revision, 10)})
	}
}
