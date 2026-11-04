package operations

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const RequestTimeout = 5 * time.Second

type MetricsClient struct {
	RPC, REST string
	Client    *http.Client
}

type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d from %s", e.Status, e.URL) }

func (c MetricsClient) get(ctx context.Context, endpoint string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: RequestTimeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &HTTPError{res.StatusCode, endpoint}
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20+1))
	if err != nil {
		return err
	}
	if len(raw) > 16<<20 {
		return errors.New("metric response exceeds 16 MiB")
	}
	return decodeJSON(raw, out)
}

type BlockInfo struct {
	Height      int       `json:"height"`
	Timestamp   time.Time `json:"timestamp"`
	NumberOfTxs int       `json:"number_of_txs"`
}

func (c MetricsClient) Block(ctx context.Context, height int) (map[string]any, error) {
	var body map[string]json.RawMessage
	if err := c.get(ctx, c.RPC+"/block?height="+strconv.Itoa(height), &body); err != nil {
		return nil, err
	}
	var version string
	if err := json.Unmarshal(body["jsonrpc"], &version); err != nil || version != "2.0" {
		return nil, errors.New("Invalid block JSON-RPC response")
	}
	if raw, ok := body["error"]; ok && string(raw) != "null" {
		var failure map[string]any
		if err := decodeJSON(raw, &failure); err != nil || failure == nil {
			return nil, errors.New("Invalid block JSON-RPC error")
		}
		return nil, fmt.Errorf("Block JSON-RPC error %v: %v (%v)", failure["code"], failure["message"], failure["data"])
	}
	var result map[string]map[string]any
	if err := decodeJSON(body["result"], &result); err != nil || result["block"] == nil {
		return nil, errors.New("Missing result.block in block JSON-RPC response")
	}
	block := result["block"]
	data, ok := block["data"].(map[string]any)
	if !ok {
		return nil, errors.New("Missing result.block.data.txs")
	}
	txValue, ok := data["txs"]
	if !ok {
		return nil, errors.New("Missing result.block.data.txs")
	}
	if txValue == nil {
		data["txs"] = []any{}
		return block, nil
	}
	txs, ok := txValue.([]any)
	if !ok {
		return nil, errors.New("Invalid result.block.data.txs")
	}
	for _, tx := range txs {
		if _, ok := tx.(string); !ok {
			return nil, errors.New("Invalid result.block.data.txs")
		}
	}
	return block, nil
}

func (c MetricsClient) BlockInfo(ctx context.Context, height int) (BlockInfo, error) {
	block, err := c.Block(ctx, height)
	if err != nil {
		return BlockInfo{}, err
	}
	header, ok := block["header"].(map[string]any)
	if !ok {
		return BlockInfo{}, errors.New("missing block header")
	}
	stamp, ok := header["time"].(string)
	if !ok {
		return BlockInfo{}, errors.New("missing block timestamp")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return BlockInfo{}, fmt.Errorf("invalid RFC3339 timestamp: %w", err)
	}
	return BlockInfo{height, timestamp, len(block["data"].(map[string]any)["txs"].([]any))}, nil
}

func (c MetricsClient) Breakdown(ctx context.Context, height int) (map[string]int, error) {
	block, err := c.Block(ctx, height)
	if err != nil {
		return nil, err
	}
	mapping := map[string]int{}
	for _, tx := range block["data"].(map[string]any)["txs"].([]any) {
		raw, err := base64.StdEncoding.DecodeString(tx.(string))
		if err != nil {
			return nil, err
		}
		name := "other"
		for _, kind := range []struct{ message, module string }{{"MsgSend", "bank"}, {"MsgAggregateExchangeRateVote", "oracle"}, {"MsgDelegate", "staking"}, {"MsgCreateDenom", "tokenfactory"}, {"MsgPlaceOrder", "dex"}} {
			if strings.Contains(string(raw), kind.message) {
				name = kind.module
				break
			}
		}
		mapping[name]++
	}
	return mapping, nil
}

func (c MetricsClient) Height(ctx context.Context, hash string) (*int, error) {
	var body map[string]json.RawMessage
	if err := c.get(ctx, c.REST+"/txs/"+url.PathEscape(hash), &body); err != nil {
		return nil, err
	}
	if _, ok := body["height"]; !ok {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(body["height"], &value); err != nil {
		value = string(body["height"])
	}
	height, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	return &height, nil
}

// BestStats leaves unavailable measurements null, including intervals of zero
// or negative duration; positive submillisecond intervals retain nanoseconds.
func (c MetricsClient) BestStats(ctx context.Context, blocks []BlockInfo) ([]any, error) {
	var best []any = []any{nil, nil, nil}
	maxTPS := -1.0
	for _, block := range blocks {
		next, err := c.BlockInfo(ctx, block.Height+1)
		if err != nil {
			return nil, err
		}
		duration := next.Timestamp.Sub(block.Timestamp)
		if duration <= 0 {
			continue
		}
		tps := float64(block.NumberOfTxs) / duration.Seconds()
		if tps > maxTPS {
			maxTPS = tps
			best = []any{tps, block.Height, float64(duration) / float64(time.Millisecond)}
		}
	}
	return best, nil
}

func (c MetricsClient) Metrics(ctx context.Context, hashesPath string) (map[string]any, error) {
	raw, err := os.ReadFile(hashesPath)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	for _, hash := range strings.Fields(string(raw)) {
		height, err := c.Height(ctx, hash)
		if err != nil {
			return nil, err
		}
		if height != nil {
			seen[*height] = true
		}
	}
	heights := make([]int, 0, len(seen))
	for height := range seen {
		heights = append(heights, height)
	}
	sort.Ints(heights)
	if len(heights) <= 2 {
		return nil, errors.New("not enough blocks for meaningful metrics")
	}
	blocks := make([]BlockInfo, 0, len(heights))
	counts := make([]int, 0, len(heights))
	for _, height := range heights {
		block, err := c.BlockInfo(ctx, height)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
		counts = append(counts, block.NumberOfTxs)
	}
	var duration time.Duration
	var total int
	available := true
	for _, block := range blocks[1 : len(blocks)-1] {
		next, err := c.BlockInfo(ctx, block.Height+1)
		if err != nil {
			return nil, err
		}
		delta := next.Timestamp.Sub(block.Timestamp)
		if delta <= 0 {
			available = false
		}
		duration += delta
		total += block.NumberOfTxs
	}
	var average, tps any
	if available && duration > 0 {
		average = duration.Seconds() / float64(len(blocks)-2)
		tps = float64(total) / duration.Seconds()
	}
	best, err := c.BestStats(ctx, blocks)
	if err != nil {
		return nil, err
	}
	var mapping any
	if best[1] != nil {
		mapping, err = c.Breakdown(ctx, best[1].(int))
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"Summary (excl. edge block)": map[string]any{"average_block_time": average, "average_throughput_per_sec": tps, "average_throughput_per_block": float64(total) / float64(len(blocks)-2), "number_of_full_blocks": len(blocks) - 2, "full_blocks": heights[1 : len(heights)-1], "total_txs_num": total},
		"Detail (incl. edge blocks)": map[string]any{"blocks": heights, "txs_per_block": counts},
		"Best block":                 map[string]any{"height": best[1], "tps": best[0], "block_time_ms": best[2], "tx_mapping": mapping},
	}, nil
}
