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
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/common"
)

// WithPxIndexerURL configures the single indexer for px_listAssets and history reads.
// Asset metadata is read from this node's pinned state.
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

// pxAssetCursor accepts only the published, canonical signed-64-bit row ID.
func pxAssetCursor(value string) (int64, bool) {
	n, err := strconv.ParseInt(value, 10, 64)
	return n, err == nil && n > 0 && strconv.FormatInt(n, 10) == value
}

func (a *pxAccountAPI) assetRecords(ctx context.Context, cursor *string) ([]pxAsset, *string, error) {
	if a.indexer == nil {
		return nil, nil, pxUnavailable("indexer_not_configured")
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	endpoint := *a.indexer.url
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/v1/assets"
	query := url.Values{"limit": {"16"}}
	if cursor != nil {
		query.Set("cursor", *cursor)
	}
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(bounded, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, nil, pxUnavailable("indexer_unreachable")
	}
	req.Header.Set("Accept", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return nil, nil, pxUnavailable("indexer_unreachable")
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	contentType, _, typeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || typeErr != nil || contentType != "application/json" {
		return nil, nil, pxUnavailable("indexer_assets_unavailable")
	}
	if readErr != nil || len(body) > 4*1024*1024 {
		return nil, nil, pxUnavailable("invalid_indexer_response")
	}
	var document struct {
		Version json.RawMessage `json:"version"`
		Items   json.RawMessage `json:"items"`
		Next    json.RawMessage `json:"next_cursor"`
	}
	if json.Unmarshal(body, &document) != nil || string(document.Version) != "1" || len(document.Items) == 0 || string(document.Items) == "null" || len(document.Next) == 0 {
		return nil, nil, pxUnavailable("invalid_indexer_response")
	}
	var rows []map[string]json.RawMessage
	if json.Unmarshal(document.Items, &rows) != nil || len(rows) > 16 {
		return nil, nil, pxUnavailable("invalid_indexer_response")
	}
	var next *string
	if string(document.Next) != "null" {
		var value string
		if json.Unmarshal(document.Next, &value) != nil {
			return nil, nil, pxUnavailable("invalid_indexer_response")
		}
		nextID, valid := pxAssetCursor(value)
		var lower int64
		if cursor != nil {
			lower, _ = pxAssetCursor(*cursor)
		}
		if !valid || nextID <= lower || len(rows) != 16 {
			return nil, nil, pxUnavailable("invalid_indexer_response")
		}
		next = &value
	}
	records := make([]pxAsset, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		var asset, kind string
		var denom *string
		if len(row["denom"]) == 0 || json.Unmarshal(row["asset"], &asset) != nil || asset == "" || seen[asset] || json.Unmarshal(row["kind"], &kind) != nil || kind == "" || json.Unmarshal(row["denom"], &denom) != nil {
			return nil, nil, pxUnavailable("invalid_indexer_response")
		}
		seen[asset] = true
		records = append(records, pxAsset{AssetID: asset, Denom: denom, Kind: kind})
	}
	return records, next, nil
}

func (a *pxAccountAPI) assetMetadata(ctx context.Context, height int64, record pxAsset) pxAsset {
	if ctx.Err() != nil {
		return record
	}
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
		if err != nil || ctx.Err() != nil || values[0].([32]byte) != id {
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
		if ctx.Err() != nil {
			return record
		}
		output, err := a.call(ctx, height, common.HexToAddress("0x1001"), bank.GetABI(), method, *record.Denom)
		if err != nil || ctx.Err() != nil {
			return record
		}
		values = append(values, output[0])
	}
	// The bank precompile reports base-unit decimals. Asset listings describe
	// the display unit recorded in the same committed bank metadata.
	metadata, found := a.state.keeper.BankKeeper().GetDenomMetaData(a.state.ctxProvider(height).WithContext(ctx), *record.Denom)
	if !found || ctx.Err() != nil {
		return record
	}
	var decimals *uint8
	for _, unit := range metadata.DenomUnits {
		if unit != nil && unit.Denom == metadata.Display {
			if decimals != nil || unit.Exponent > 255 {
				return record
			}
			value := uint8(unit.Exponent)
			decimals = &value
		}
	}
	if decimals == nil {
		return record
	}
	name, symbol, supply := values[0].(string), values[1].(string), values[3].(*big.Int).String()
	record.Name = &name
	record.Symbol = &symbol
	record.Decimals = decimals
	record.Supply = &supply
	return record
}
func (a *pxAccountAPI) ListAssets(ctx context.Context, cursor *string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_listAssets", a.state.connectionType, start, returnErr, recover())
	}()
	if cursor != nil {
		if _, valid := pxAssetCursor(*cursor); !valid {
			return nil, pxInvalidParams()
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	deadline := func() error {
		if ctx.Err() != nil {
			return pxUnavailable("asset_list_deadline_exceeded")
		}
		return nil
	}
	records, next, err := a.assetRecords(ctx, cursor)
	if expired := deadline(); expired != nil {
		return nil, expired
	}
	if err != nil {
		return nil, err
	}
	height, err := a.height(ctx)
	if expired := deadline(); expired != nil {
		return nil, expired
	}
	if err != nil {
		return nil, err
	}
	for i, record := range records {
		if err := deadline(); err != nil {
			return nil, err
		}
		records[i] = a.assetMetadata(ctx, height, record)
		if err := deadline(); err != nil {
			return nil, err
		}
	}
	return map[string]interface{}{"height": height, "assets": records, "joined_limit": 16, "next_cursor": next, "order": "registration"}, nil
}
