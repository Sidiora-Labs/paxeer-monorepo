package app_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func awaitTradeHead(t *testing.T, f *repair372Fixture, path, height string) (int, []byte) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		response, err := http.Get(f.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var ready struct {
			Watermark *struct {
				Height string `json:"height"`
			} `json:"watermark"`
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("head read: %v, close: %v", readErr, closeErr)
		}
		if response.StatusCode == 200 && json.Unmarshal(body, &ready) == nil && ready.Watermark != nil && ready.Watermark.Height == height {
			return response.StatusCode, body
		}
		if time.Now().After(deadline) {
			t.Fatalf("head watermark was not ready: status %d body %s", response.StatusCode, body)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRepair_22_39_376_trade_read_carries_the_head_watermark(t *testing.T) {
	f := newRepair372Fixture(t)
	f.createPool(t)
	first := f.swap(t, true, 1000000, 400000000000000000, 10000)
	second := f.swap(t, false, 200000000000000000, 500000, 5000)
	f.awaitTrade(t, first)
	f.awaitTrade(t, second)

	path := "/v2/pools/" + dstPool + "/trades"
	var revision string
	for read := 0; read < 2; read++ {
		status, raw := awaitTradeHead(t, f, path, second.BlockNumber.String())
		if status != 200 {
			t.Fatalf("trade read %d status = %d: %s", read, status, raw)
		}
		if err := interfaces.ValidateJSON("TradesResponse", raw); err != nil {
			t.Fatalf("trade read %d contract: %v: %s", read, err, raw)
		}
		var page struct {
			Quality   string  `json:"quality"`
			Revision  *string `json:"revision"`
			Watermark *struct {
				Integrity string `json:"integrity"`
				Height    string `json:"height"`
				BlockHash string `json:"blockHash"`
			} `json:"watermark"`
			Data struct {
				Items []struct {
					Position struct {
						Height    string `json:"height"`
						BlockHash string `json:"blockHash"`
					} `json:"position"`
				} `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			t.Fatal(err)
		}
		if page.Quality != "partial" || page.Watermark == nil || page.Watermark.Integrity != "healthy" || page.Revision == nil || *page.Revision == "" {
			t.Fatalf("trade read %d lacks its partial, healthy head envelope: %s", read, raw)
		}
		if read == 0 {
			revision = *page.Revision
		} else if *page.Revision != revision {
			t.Fatalf("revision changed at one head: %q then %q", revision, *page.Revision)
		}
		watermarkHeight, err := strconv.ParseUint(page.Watermark.Height, 10, 64)
		if err != nil || !strings.EqualFold(page.Watermark.BlockHash, second.BlockHash.Hex()) || page.Watermark.Height != second.BlockNumber.String() || revision != page.Watermark.Height {
			t.Fatalf("head watermark or revision differs from the committed head: %s", raw)
		}
		if len(page.Data.Items) != 2 {
			t.Fatalf("trade read %d returned %d swaps, want 2: %s", read, len(page.Data.Items), raw)
		}
		for _, trade := range page.Data.Items {
			height, err := strconv.ParseUint(trade.Position.Height, 10, 64)
			if err != nil || height > watermarkHeight || height == watermarkHeight && !strings.EqualFold(trade.Position.BlockHash, page.Watermark.BlockHash) {
				t.Fatalf("trade position is above or outside its head watermark: %s", raw)
			}
		}
	}

	// A cursor behind the latest committed swap must not publish that swap
	// above the page watermark.
	f.exec(t, `UPDATE indexer.cursors SET last_processed_block=$1, updated_at=now() WHERE chain_id=125`, first.BlockNumber.Int64())
	status, raw := awaitTradeHead(t, f, path, first.BlockNumber.String())
	if status != 200 || interfaces.ValidateJSON("TradesResponse", raw) != nil {
		t.Fatalf("lagging head trade read = %d: %s", status, raw)
	}
	var lagging struct {
		Watermark struct {
			Height    string `json:"height"`
			BlockHash string `json:"blockHash"`
		} `json:"watermark"`
		Data struct {
			Items []struct {
				Position struct {
					Height    string `json:"height"`
					BlockHash string `json:"blockHash"`
				} `json:"position"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &lagging); err != nil {
		t.Fatal(err)
	}
	if lagging.Watermark.Height != first.BlockNumber.String() || !strings.EqualFold(lagging.Watermark.BlockHash, first.BlockHash.Hex()) || len(lagging.Data.Items) != 1 || lagging.Data.Items[0].Position.Height != first.BlockNumber.String() || !strings.EqualFold(lagging.Data.Items[0].Position.BlockHash, first.BlockHash.Hex()) {
		t.Fatalf("trade above lagging head escaped its watermark: %s", raw)
	}
}
