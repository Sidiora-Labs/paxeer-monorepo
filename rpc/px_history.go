package evmrpc

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/Sidiora-Labs/Paxeer-X-Network/src/modules/history"
	"github.com/ethereum/go-ethereum/common"
)

type pxHistoryAPI struct{ account *pxAccountAPI }

func pxHistoryError(err error) error {
	var failure *history.Failure
	if errors.As(err, &failure) {
		if failure.Refusal != nil {
			data, ok := failure.Refusal.(map[string]interface{})
			if !ok {
				return pxInvalidParams()
			}
			return &pxError{-32602, "Invalid params", data}
		}
		return pxHistoryUnavailable(failure.Code)
	}
	return pxHistoryUnavailable("invalid_indexer_response")
}
func pxHistoryUnavailable(code string) error {
	return &pxError{-32001, "History unavailable", map[string]interface{}{"code": code}}
}
func pxHistoryQuery(cursor *string, limit *int, kind *string) (history.Query, error) {
	q := history.Query{Cursor: cursor, Limit: 50, Kind: kind}
	if limit != nil {
		q.Limit = *limit
	}
	if (cursor != nil && !history.ValidCursor(*cursor)) || q.Limit < 1 || q.Limit > 100 || (kind != nil && !history.ValidKind(*kind)) {
		return q, pxInvalidParams()
	}
	return q, nil
}
func (a *pxHistoryAPI) GetHistory(ctx context.Context, selector string, cursor *string, limit *int, kind *string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getHistory", a.account.state.connectionType, start, returnErr, recover())
	}()
	return a.read(ctx, selector, cursor, limit, kind, false)
}
func (a *pxHistoryAPI) GetUnifiedHistory(ctx context.Context, selector string, cursor *string, limit *int, kind *string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getUnifiedHistory", a.account.state.connectionType, start, returnErr, recover())
	}()
	return a.read(ctx, selector, cursor, limit, kind, true)
}
func (a *pxHistoryAPI) read(ctx context.Context, selector string, cursor *string, limit *int, kind *string, unified bool) (map[string]interface{}, error) {
	q, err := pxHistoryQuery(cursor, limit, kind)
	if err != nil {
		return nil, err
	}
	// History accepts only the OpenRPC EVM/DID selector, never a bare native ID.
	if !((strings.HasPrefix(selector, "0x") && len(selector) == 42 && common.IsHexAddress(selector)) || (strings.HasPrefix(selector, "did:layerx:") && pxHistoryDID(selector))) {
		return nil, pxInvalidParams()
	}
	account, height, err := a.account.snapshot(ctx, selector)
	if err != nil {
		return nil, err
	}
	if account.EVMAddress == nil {
		return nil, pxInvalidParams()
	}
	if a.account.indexer == nil {
		return nil, pxHistoryUnavailable("indexer_not_configured")
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	accounts := []history.Account{{Side: "paxeer", Account: *account.EVMAddress}}
	if unified && account.Bound {
		owned, err := layerxledger.ListOwnedAccounts(a.account.state.ctxProvider(height).WithContext(bounded), *account.LayerXDid, 15)
		if err != nil {
			return nil, pxHistoryUnavailable("native_account_enumeration_unavailable")
		}
		// The binding's canonical main account is part of the unified identity even
		// before its first kernel activity has materialized a registry record.
		seen := map[string]bool{*account.LayerXAccount: true}
		accounts = append(accounts, history.Account{Side: "layerx", Account: *account.LayerXAccount})
		for _, id := range owned {
			if !seen[id] {
				accounts = append(accounts, history.Account{Side: "layerx", Account: id})
				seen[id] = true
			}
		}
		if len(accounts) > 16 {
			return nil, pxHistoryUnavailable("native_account_enumeration_unavailable")
		}
	}
	source := history.Client{Endpoint: a.account.indexer.url}
	pages := make([]history.Page, 0, len(accounts))
	for _, key := range accounts {
		page, err := source.Page(bounded, key, q)
		if err != nil {
			return nil, pxHistoryError(err)
		}
		pages = append(pages, page)
	}
	page, err := history.Merge(pages, accounts, q.Limit, unified)
	if err != nil {
		return nil, pxHistoryError(err)
	}
	if err := a.joinAssets(bounded, source, height, page.Items); err != nil {
		return nil, err
	}
	status, freshness, err := source.Get(bounded, "/readyz", nil)
	if err != nil || (status != 200 && status != 503) || !history.ValidReadiness(freshness) {
		return nil, pxHistoryUnavailable("invalid_indexer_readiness")
	}
	result := map[string]interface{}{"account": *account.EVMAddress, "items": page.Items, "next_cursor": page.Next, "indexedHeight": page.IndexedHeight, "chainHeight": height, "freshness": freshness}
	if unified {
		result["account"] = account
		result["accounts"] = accounts
	}
	return result, nil
}
func pxHistoryDID(selector string) bool {
	raw := strings.TrimPrefix(selector, "did:layerx:")
	if len(raw) != 64 {
		return false
	}
	for _, c := range raw {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
func (a *pxHistoryAPI) ListHolderRecords(ctx context.Context, address, kind string, cursor *string, limit *int) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_listHolderRecords", a.account.state.connectionType, start, returnErr, recover())
	}()
	q, err := pxHistoryQuery(cursor, limit, &kind)
	if err != nil || (kind != "budget" && kind != "grant") || !strings.HasPrefix(address, "0x") || len(address) != 42 || !common.IsHexAddress(address) {
		return nil, pxInvalidParams()
	}
	if a.account.indexer == nil {
		return nil, pxHistoryUnavailable("indexer_not_configured")
	}
	height, err := a.account.height(ctx)
	if err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result, err = (history.Client{Endpoint: a.account.indexer.url}).Holders(bounded, strings.ToLower(address), q)
	if err != nil {
		return nil, pxHistoryError(err)
	}
	result["chainHeight"] = height
	return result, nil
}

func (a *pxHistoryAPI) joinAssets(ctx context.Context, source history.Client, height int64, items []history.Row) error {
	records := map[string]map[string]interface{}{}
	needsRegistry := false
	for _, item := range items {
		if _, exists := records[item.Asset]; exists {
			continue
		}
		status, body, err := source.Get(ctx, "/v1/assets/"+url.PathEscape(item.Asset), nil)
		if err != nil {
			return pxHistoryError(err)
		}
		if status == 404 {
			records[item.Asset] = nil
			continue
		}
		if status != 200 {
			return pxHistoryUnavailable("indexer_unavailable")
		}
		record, ok := body["asset"].(map[string]interface{})
		if !ok || record["asset"] != item.Asset {
			return pxHistoryError(history.Invalid())
		}
		kind, kindOK := record["kind"].(string)
		chain, chainOK := record["chain"].(string)
		if !kindOK || kind == "" || !chainOK || (chain != "paxeer" && chain != "layerx") {
			return pxHistoryError(history.Invalid())
		}
		if kind == "layerx" && !pxLowerHex(item.Asset, 32) {
			return pxHistoryError(history.Invalid())
		}
		if kind == "pointer" {
			address, ok := record["address"].(string)
			if !ok || len(address) != 42 || !strings.HasPrefix(address, "0x") || !common.IsHexAddress(address) || strings.ToLower(address) != address {
				return pxHistoryError(history.Invalid())
			}
		}
		for _, key := range []string{"denom", "address"} {
			if value := record[key]; value != nil {
				if _, ok := value.(string); !ok {
					return pxHistoryError(history.Invalid())
				}
			}
		}
		records[item.Asset] = record
		needsRegistry = needsRegistry || record["denom"] != nil
	}
	registry := map[string]map[string]interface{}{}
	if needsRegistry {
		cursor := uint64(0)
		for {
			query := url.Values{"limit": {"500"}}
			if cursor != 0 {
				query.Set("cursor", strconv.FormatUint(cursor, 10))
			}
			status, body, err := source.Get(ctx, "/v1/assets", query)
			if err != nil {
				return pxHistoryError(err)
			}
			if status != 200 {
				return pxHistoryUnavailable("registry_unavailable")
			}
			rows, ok := body["items"].([]interface{})
			if !ok || len(rows) > 500 {
				return pxHistoryError(history.Invalid())
			}
			for _, value := range rows {
				row, ok := value.(map[string]interface{})
				if !ok {
					return pxHistoryError(history.Invalid())
				}
				if row["kind"] != "layerx" {
					continue
				}
				id, ok := row["asset"].(string)
				if !ok || !pxLowerHex(id, 32) {
					return pxHistoryError(history.Invalid())
				}
				if denom, ok := row["denom"].(string); ok {
					registry[denom] = row
				}
			}
			next, exists := body["next_cursor"]
			if !exists {
				return pxHistoryError(history.Invalid())
			}
			if next == nil {
				break
			}
			text, ok := next.(string)
			if !ok {
				return pxHistoryError(history.Invalid())
			}
			value, err := strconv.ParseUint(text, 10, 64)
			if err != nil || value <= cursor {
				return pxHistoryError(history.Invalid())
			}
			cursor = value
		}
	}
	joined := map[string]interface{}{}
	for asset, record := range records {
		if record == nil {
			joined[asset] = nil
			continue
		}
		kind := record["kind"].(string)
		metadata, _ := record["metadata"].(map[string]interface{})
		var symbol, decimals, nativeID, pointer interface{}
		if value, ok := metadata["symbol"].(string); ok {
			symbol = value
		}
		if value, ok := history.Uint(metadata["decimals"]); ok {
			decimals = value
		}
		var denom *string
		if value, ok := record["denom"].(string); ok {
			denom = &value
		}
		registryRecord := registry[pxHistoryDenom(denom)]
		if kind == "layerx" {
			nativeID = asset
		} else if registryRecord != nil {
			nativeID = registryRecord["asset"]
		}
		if registryRecord != nil {
			registryMetadata, _ := registryRecord["metadata"].(map[string]interface{})
			if symbol == nil {
				if value, ok := registryMetadata["symbol"].(string); ok {
					symbol = value
				}
			}
			if decimals == nil {
				if value, ok := history.Uint(registryMetadata["decimals"]); ok {
					decimals = value
				}
			}
		}
		// Prefer indexer metadata, then metadata read from the local pinned registry.
		local := a.account.assetMetadata(ctx, height, pxAsset{AssetID: asset, Kind: kind, Denom: denom})
		if symbol == nil && local.Symbol != nil {
			symbol = *local.Symbol
		}
		if decimals == nil && local.Decimals != nil {
			decimals = *local.Decimals
		}
		if decimals == nil && kind == "native" {
			decimals = uint64(18)
		}
		if kind == "pointer" {
			pointer = record["address"]
		}
		joined[asset] = map[string]interface{}{"asset": asset, "chain": record["chain"], "kind": kind, "address": record["address"], "denom": record["denom"], "symbol": symbol, "decimals": decimals, "native_id": nativeID, "pointer": pointer, "metadata": record["metadata"]}
	}
	for i := range items {
		items[i].AssetMetadata = joined[items[i].Asset]
	}
	return nil
}
func pxHistoryDenom(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
