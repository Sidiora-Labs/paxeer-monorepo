// Package history reads the configured indexer's history interface. It owns no
// index and never derives transfers or finality by scanning node logs.
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Failure struct {
	Code    string
	Refusal interface{}
}

func (e *Failure) Error() string { return e.Code }
func Invalid() error             { return &Failure{Code: "invalid_indexer_response"} }

type Client struct{ Endpoint *url.URL }

// Get refuses redirects and bounds each response as well as the caller's total
// read budget. Numbers in opaque decoded payloads retain their precision.
func (c Client) Get(ctx context.Context, path string, query url.Values) (int, map[string]interface{}, error) {
	endpoint := *c.Endpoint
	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		return 0, nil, Invalid()
	}
	escapedPath := strings.TrimRight(endpoint.EscapedPath(), "/") + path
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + decodedPath
	endpoint.RawPath = escapedPath
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, nil, &Failure{Code: "indexer_unreachable"}
	}
	request.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, &Failure{Code: "indexer_unreachable"}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	contentType, _, typeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || typeErr != nil || contentType != "application/json" || len(body) > 4*1024*1024 {
		return 0, nil, Invalid()
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var document map[string]interface{}
	if decoder.Decode(&document) != nil || document == nil {
		return 0, nil, Invalid()
	}
	var extra interface{}
	if decoder.Decode(&extra) != io.EOF {
		return 0, nil, Invalid()
	}
	return response.StatusCode, document, nil
}

var kindPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var cursorPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func ValidKind(value string) bool { return kindPattern.MatchString(value) }
func ValidCursor(value string) bool {
	if !cursorPattern.MatchString(value) {
		return false
	}
	height, err := strconv.ParseUint(value[:16], 16, 64)
	return err == nil && height <= math.MaxInt64
}

type Query struct {
	Cursor *string
	Limit  int
	Kind   *string
}

func (q Query) Values() url.Values {
	values := url.Values{"limit": {strconv.Itoa(q.Limit)}}
	if q.Cursor != nil {
		values.Set("cursor", *q.Cursor)
	}
	if q.Kind != nil {
		values.Set("kind", *q.Kind)
	}
	return values
}

type Account struct {
	Side    string `json:"side"`
	Account string `json:"account"`
}
type Stability struct {
	Level             string  `json:"level"`
	Source            string  `json:"source"`
	FinalizedBoundary *string `json:"finalized_boundary"`
}
type Settlement struct {
	Level  string  `json:"level"`
	Source *string `json:"source"`
	Reason string  `json:"reason"`
}
type Row struct {
	ID            string      `json:"id"`
	Height        string      `json:"height_or_seq"`
	Chain         string      `json:"chain"`
	Kind          string      `json:"kind"`
	Direction     string      `json:"direction"`
	Account       string      `json:"account"`
	Counterparty  *string     `json:"counterparty"`
	Asset         string      `json:"asset"`
	Amount        string      `json:"amount"`
	TxID          string      `json:"tx_id"`
	TxHash        string      `json:"tx_hash"`
	Ordinal       string      `json:"ordinal"`
	Final         bool        `json:"final"`
	FinalBasis    string      `json:"final_basis"`
	Stability     Stability   `json:"stability"`
	Settlement    Settlement  `json:"settlement"`
	Decoded       interface{} `json:"decoded"`
	AssetMetadata interface{} `json:"asset_metadata"`
	Side          string      `json:"side,omitempty"`
	BlockHeight   uint64      `json:"blockHeight"`
	TxIndex       uint32      `json:"txIndex"`
	LogIndex      uint32      `json:"logIndex"`
	TxIndexText   string      `json:"tx_index"`
	LogIndexText  string      `json:"log_index"`
}

func (r Row) Position() string {
	return fmt.Sprintf("%016x%08x%08x", r.BlockHeight, r.TxIndex, r.LogIndex)
}
func text(row map[string]interface{}, key string) (string, bool) {
	value, ok := row[key].(string)
	return value, ok
}
func Uint(value interface{}) (uint64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseUint(string(number), 10, 64)
	return parsed, err == nil
}
func decimal(row map[string]interface{}, key string, bits int) (uint64, bool) {
	value, ok := text(row, key)
	if !ok || !decimalPattern.MatchString(value) {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, bits)
	return parsed, err == nil
}

// historyID accepts the two ID namespaces emitted by the indexer without
// changing the externally visible ID or admitting noncanonical decimals.
func historyID(value string) (uint64, bool, bool) {
	number, imported := strings.CutPrefix(value, "explorer:")
	if !decimalPattern.MatchString(number) || number == "0" {
		return 0, imported, false
	}
	id, err := strconv.ParseUint(number, 10, 64)
	return id, imported, err == nil
}

func rowFrom(document map[string]interface{}, account Account, q Query) (Row, error) {
	for _, field := range []string{"id", "height_or_seq", "chain", "kind", "direction", "account", "asset", "amount", "tx_id", "ordinal", "tx_index", "log_index"} {
		if _, ok := text(document, field); !ok {
			return Row{}, Invalid()
		}
	}
	for _, field := range []string{"counterparty", "decoded", "final"} {
		if _, ok := document[field]; !ok {
			return Row{}, Invalid()
		}
	}
	encoded, _ := json.Marshal(document)
	row := Row{
		FinalBasis: "block_commit",
		Settlement: Settlement{Level: "unverified", Reason: "no_verified_checkpoint_evidence"},
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	if decoder.Decode(&row) != nil {
		return Row{}, Invalid()
	}
	row.TxHash = row.TxID
	if row.FinalBasis != "block_commit" && row.FinalBasis != "explorer_api" {
		return Row{}, Invalid()
	}
	// Decode opaque payloads with UseNumber instead of passing through float64.
	row.Decoded = document["decoded"]
	height, hOK := decimal(document, "height_or_seq", 63)
	tx, tOK := decimal(document, "tx_index", 32)
	log, lOK := decimal(document, "log_index", 32)
	ordinal, oOK := decimal(document, "ordinal", 64)
	_, _, idOK := historyID(row.ID)
	_, finalOK := document["final"].(bool)
	counterpartyOK := document["counterparty"] == nil
	if _, ok := document["counterparty"].(string); ok {
		counterpartyOK = true
	}
	expectedLog := uint64(0)
	if ordinal > 0 {
		expectedLog = ordinal - 1
	}
	if !hOK || !tOK || !lOK || !oOK || !idOK || !finalOK || !counterpartyOK || expectedLog != log || !decimalPattern.MatchString(row.Amount) || !ValidKind(row.Kind) || (row.Chain != "paxeer" && row.Chain != "layerx") || (row.Direction != "in" && row.Direction != "out") || row.Account != account.Account || row.Asset == "" || row.TxID == "" || (q.Kind != nil && row.Kind != *q.Kind) {
		return Row{}, Invalid()
	}
	row.Stability.Source = row.FinalBasis
	if row.FinalBasis == "explorer_api" {
		row.Stability.Level = "explorer_recorded"
		row.Stability.FinalizedBoundary = nil
	} else {
		row.Stability.Level = "reversible"
		if row.Final {
			row.Stability.Level = "depth_stable"
		}
		if boundary := row.Stability.FinalizedBoundary; boundary != nil {
			if !decimalPattern.MatchString(*boundary) {
				return Row{}, Invalid()
			}
			value, err := strconv.ParseUint(*boundary, 10, 63)
			if err != nil || row.Final != (height <= value) {
				return Row{}, Invalid()
			}
		}
	}
	if row.Settlement.Level != "unverified" || row.Settlement.Source != nil || row.Settlement.Reason != "no_verified_checkpoint_evidence" {
		return Row{}, Invalid()
	}
	row.BlockHeight = height
	row.TxIndex = uint32(tx)
	row.LogIndex = uint32(log)
	row.Side = ""
	row.AssetMetadata = nil
	return row, nil
}

type Page struct {
	Items         []Row
	Next          *string
	IndexedHeight *uint64
}

func pageFields(body map[string]interface{}) (*string, *uint64, error) {
	version, ok := Uint(body["version"])
	if !ok || version != 1 {
		return nil, nil, Invalid()
	}
	var next *string
	value, exists := body["next_cursor"]
	if !exists {
		return nil, nil, Invalid()
	}
	if value != nil {
		cursor, ok := value.(string)
		if !ok || !ValidCursor(cursor) {
			return nil, nil, Invalid()
		}
		next = &cursor
	}
	var indexed *uint64
	value, exists = body["indexedHeight"]
	if !exists {
		return nil, nil, Invalid()
	}
	if value != nil {
		height, ok := Uint(value)
		if !ok || height > math.MaxInt64 {
			return nil, nil, Invalid()
		}
		indexed = &height
	}
	return next, indexed, nil
}
func (c Client) Page(ctx context.Context, account Account, q Query) (Page, error) {
	status, body, err := c.Get(ctx, "/v1/history/"+url.PathEscape(account.Account), q.Values())
	if err != nil {
		return Page{}, err
	}
	if status == 400 {
		return Page{}, &Failure{Code: "indexer_refused", Refusal: body}
	}
	if status != 200 {
		return Page{}, &Failure{Code: "indexer_unavailable"}
	}
	next, indexed, err := pageFields(body)
	if err != nil {
		return Page{}, err
	}
	rows, ok := body["items"].([]interface{})
	if !ok || !ValidReadiness(body["freshness"]) {
		return Page{}, Invalid()
	}
	page := Page{Items: make([]Row, 0, len(rows)), Next: next, IndexedHeight: indexed}
	for _, item := range rows {
		document, ok := item.(map[string]interface{})
		if !ok {
			return Page{}, Invalid()
		}
		row, err := rowFrom(document, account, q)
		if err != nil {
			return Page{}, err
		}
		position := row.Position()
		if (q.Cursor != nil && position <= *q.Cursor) || (len(page.Items) > 0 && page.Items[len(page.Items)-1].Position() > position) {
			return Page{}, Invalid()
		}
		if len(page.Items) >= q.Limit && page.Items[q.Limit-1].Position() != position {
			return Page{}, Invalid()
		}
		page.Items = append(page.Items, row)
	}
	if len(page.Items) > 100 {
		return Page{}, &Failure{Code: "history_position_page_too_large"}
	}
	if next != nil && (len(page.Items) == 0 || *next != page.Items[len(page.Items)-1].Position()) {
		return Page{}, Invalid()
	}
	return page, nil
}

// Merge keeps every row at the last position together, matching the indexer's
// strictly-after cursor. Splitting ties would silently drop self-transfer legs.
func Merge(pages []Page, accounts []Account, limit int, tagged bool) (Page, error) {
	result := Page{Items: []Row{}}
	more := false
	seen := map[string]bool{}
	for i, page := range pages {
		if i == 0 {
			result.IndexedHeight = page.IndexedHeight
		} else if result.IndexedHeight != nil {
			if page.IndexedHeight == nil {
				result.IndexedHeight = nil
			} else if *page.IndexedHeight < *result.IndexedHeight {
				result.IndexedHeight = page.IndexedHeight
			}
		}
		more = more || page.Next != nil
		for _, row := range page.Items {
			if tagged {
				row.Side = accounts[i].Side
			}
			key := row.Position() + ":" + row.Ordinal + ":" + row.ID
			if !seen[key] {
				result.Items = append(result.Items, row)
				seen[key] = true
			}
		}
	}
	sort.SliceStable(result.Items, func(i, j int) bool {
		a, b := result.Items[i], result.Items[j]
		if a.Position() != b.Position() {
			return a.Position() < b.Position()
		}
		ao, _ := strconv.ParseUint(a.Ordinal, 10, 64)
		bo, _ := strconv.ParseUint(b.Ordinal, 10, 64)
		if ao != bo {
			return ao < bo
		}
		ai, aImported, _ := historyID(a.ID)
		bi, bImported, _ := historyID(b.ID)
		if aImported != bImported {
			return !aImported
		}
		return ai < bi
	})
	if len(result.Items) > limit {
		boundary := result.Items[limit-1].Position()
		end := limit
		for end < len(result.Items) && result.Items[end].Position() == boundary {
			end++
		}
		more = more || end < len(result.Items)
		result.Items = result.Items[:end]
	}
	if len(result.Items) > 100 {
		return Page{}, &Failure{Code: "history_position_page_too_large"}
	}
	if more && len(result.Items) > 0 {
		cursor := result.Items[len(result.Items)-1].Position()
		result.Next = &cursor
	}
	return result, nil
}
func (c Client) Holders(ctx context.Context, address string, q Query) (map[string]interface{}, error) {
	status, body, err := c.Get(ctx, "/v1/holders/"+url.PathEscape(address), q.Values())
	if err != nil {
		return nil, err
	}
	if status == 400 {
		return nil, &Failure{Code: "indexer_refused", Refusal: body}
	}
	if status != 200 {
		return nil, &Failure{Code: "indexer_unavailable"}
	}
	next, indexed, err := pageFields(body)
	if err != nil {
		return nil, err
	}
	rows, ok := body["items"].([]interface{})
	if !ok || len(rows) > q.Limit {
		return nil, Invalid()
	}
	for _, item := range rows {
		row, ok := item.(map[string]interface{})
		if !ok || row["holder"] != address || q.Kind == nil || row["kind"] != *q.Kind {
			return nil, Invalid()
		}
	}
	if next != nil && (len(rows) == 0 || (q.Cursor != nil && *next <= *q.Cursor)) {
		return nil, Invalid()
	}
	return map[string]interface{}{"items": rows, "next_cursor": next, "indexedHeight": indexed}, nil
}
func ValidReadiness(value interface{}) bool {
	body, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	version, ok := Uint(body["version"])
	if !ok || version != 1 || body["database"] != "ok" {
		return false
	}
	sources, ok := body["sources"].([]interface{})
	if !ok {
		return false
	}
	rank := 0
	if len(sources) == 0 {
		rank = 2
	}
	names := map[string]bool{}
	for _, value := range sources {
		source, ok := value.(map[string]interface{})
		if !ok {
			return false
		}
		name, ok := source["source"].(string)
		if !ok || names[name] || (name != "layerx" && name != "paxeer" && name != "comet") {
			return false
		}
		names[name] = true
		for _, key := range []string{"source_head", "indexed_cursor", "finalized_cursor", "last_success_at", "freshness_secs", "last_error_at"} {
			value, exists := source[key]
			if !exists {
				return false
			}
			if value != nil {
				if _, ok := Uint(value); !ok {
					return false
				}
			}
		}
		reconciled, ok := source["reconciled"].(bool)
		if !ok {
			return false
		}
		failures, ok := Uint(source["consecutive_failures"])
		if !ok {
			return false
		}
		switch source["state"] {
		case "ready":
			_, success := Uint(source["last_success_at"])
			_, fresh := Uint(source["freshness_secs"])
			if !reconciled || !success || !fresh || failures != 0 || source["source_head"] != source["indexed_cursor"] {
				return false
			}
		case "catching_up", "retrying", "failing":
			if rank < 1 {
				rank = 1
			}
		case "starting", "stalled":
			rank = 2
		default:
			return false
		}
	}
	return (rank == 0 && body["status"] == "ready") || (rank == 1 && body["status"] == "degraded") || (rank == 2 && body["status"] == "unavailable")
}
