package evmrpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/common"
)

// WithPxIndexerURL configures the asset enumeration authority. It is independent
// of the hosted gateway: metadata is always read from this node's pinned state.
func WithPxIndexerURL(endpoint string) ServerOption { return pxIndexerOption{endpoint: endpoint} }

type pxIndexerOption struct {
	endpoint string
	url      *url.URL
}

func (pxIndexerOption) serverOption() {}
func pxIndexerForServer(endpoint string, opts []ServerOption) (*pxIndexerOption, error) {
	if endpoint != "" {
		opts = append(append([]ServerOption{}, opts...), WithPxIndexerURL(endpoint))
	}
	var configured *pxIndexerOption
	for _, opt := range opts {
		value, ok := opt.(pxIndexerOption)
		if !ok {
			continue
		}
		parsed, err := url.Parse(value.endpoint)
		if configured != nil || err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, pxInvalidParams()
		}
		value.url = parsed
		configured = &value
	}
	return configured, nil
}

type pxAsset struct {
	AssetID      string  `json:"asset_id"`
	Denom        *string `json:"denom"`
	Kind         string  `json:"kind"`
	Name         *string `json:"name"`
	Symbol       *string `json:"symbol"`
	Decimals     *uint8  `json:"decimals"`
	Supply       *string `json:"supply"`
	Paused       *bool   `json:"paused,omitempty"`
	Verification *string `json:"verification,omitempty"`
}

func (a *pxAccountAPI) assetRecords(ctx context.Context) ([]pxAsset, error) {
	if a.indexer == nil {
		return nil, pxUnavailable("indexer_not_configured")
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	records := make([]pxAsset, 0, 16)
	cursor := uint64(0)
	for {
		endpoint := *a.indexer.url
		endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/v1/assets"
		query := url.Values{"limit": {"500"}}
		if cursor != 0 {
			query.Set("cursor", strconv.FormatUint(cursor, 10))
		}
		endpoint.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(bounded, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, pxUnavailable("indexer_unreachable")
		}
		req.Header.Set("Accept", "application/json")
		response, err := client.Do(req)
		if err != nil {
			return nil, pxUnavailable("indexer_unreachable")
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
		response.Body.Close()
		contentType, _, typeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if response.StatusCode != http.StatusOK || typeErr != nil || contentType != "application/json" {
			return nil, pxUnavailable("indexer_assets_unavailable")
		}
		if readErr != nil || len(body) > 4*1024*1024 {
			return nil, pxUnavailable("invalid_indexer_response")
		}
		var document struct {
			Items json.RawMessage `json:"items"`
			Next  json.RawMessage `json:"next_cursor"`
		}
		if json.Unmarshal(body, &document) != nil || len(document.Items) == 0 || string(document.Items) == "null" || len(document.Next) == 0 {
			return nil, pxUnavailable("invalid_indexer_response")
		}
		var rows []map[string]json.RawMessage
		if json.Unmarshal(document.Items, &rows) != nil || len(rows) > 500 {
			return nil, pxUnavailable("invalid_indexer_response")
		}
		for _, row := range rows {
			var asset, kind string
			var denom *string
			if len(row["denom"]) == 0 || json.Unmarshal(row["asset"], &asset) != nil || asset == "" || json.Unmarshal(row["kind"], &kind) != nil || string(row["kind"]) == "null" || json.Unmarshal(row["denom"], &denom) != nil {
				return nil, pxUnavailable("invalid_indexer_response")
			}
			records = append(records, pxAsset{AssetID: asset, Denom: denom, Kind: kind})
		}
		// Retain the globally smallest sixteen records while collecting every page.
		sort.SliceStable(records, func(i, j int) bool { return records[i].AssetID < records[j].AssetID })
		if len(records) > 16 {
			records = records[:16]
		}
		if string(document.Next) == "null" {
			break
		}
		var next string
		if json.Unmarshal(document.Next, &next) != nil || next == "" {
			return nil, pxUnavailable("invalid_indexer_response")
		}
		nextCursor, err := strconv.ParseUint(next, 10, 64)
		if err != nil || nextCursor <= cursor {
			return nil, pxUnavailable("invalid_indexer_response")
		}
		cursor = nextCursor
	}
	return records, nil
}
func (a *pxAccountAPI) assetMetadata(ctx context.Context, height int64, record pxAsset) pxAsset {
	if record.Kind == "layerx" || record.Kind == "ledger" {
		raw, err := hex.DecodeString(strings.TrimPrefix(record.AssetID, "0x"))
		if err != nil || len(raw) != 32 {
			return record
		}
		var id [32]byte
		copy(id[:], raw)
		if id == ([32]byte{}) {
			return record
		}
		values, err := a.call(ctx, height, common.HexToAddress("0x101a"), layerxledger.GetABI(), "getAsset", id)
		if err != nil || values[0].([32]byte) != id {
			return record
		}
		symbol, name, decimals, paused := values[1].(string), values[2].(string), values[3].(uint8), values[10].(bool)
		supply := values[11].(*big.Int).String()
		verification := "authenticated_committed_snapshot"
		record.Name = &name
		record.Symbol = &symbol
		record.Decimals = &decimals
		record.Supply = &supply
		record.Paused = &paused
		record.Verification = &verification
		return record
	}
	if record.Denom == nil || *record.Denom == "" {
		return record
	}
	values := make([]interface{}, 0, 4)
	for _, method := range []string{"name", "symbol", "decimals", "supply"} {
		output, err := a.call(ctx, height, common.HexToAddress("0x1001"), bank.GetABI(), method, *record.Denom)
		if err != nil {
			return record
		}
		values = append(values, output[0])
	}
	name, symbol, decimals, supply := values[0].(string), values[1].(string), values[2].(uint8), values[3].(*big.Int).String()
	record.Name = &name
	record.Symbol = &symbol
	record.Decimals = &decimals
	record.Supply = &supply
	return record
}
func (a *pxAccountAPI) ListAssets(ctx context.Context) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_listAssets", a.state.connectionType, start, returnErr, recover())
	}()
	records, err := a.assetRecords(ctx)
	if err != nil {
		return nil, err
	}
	height, err := a.height(ctx)
	if err != nil {
		return nil, err
	}
	for i, record := range records {
		records[i] = a.assetMetadata(ctx, height, record)
	}
	return map[string]interface{}{"height": height, "assets": records, "joined_limit": 16}, nil
}
