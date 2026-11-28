package localnode

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
)

func TestLocalNodeV6120SequencerMadeABlockAfterItsRestart(t *testing.T) {
	decimal := regexp.MustCompile(`^[1-9][0-9]*$`)
	heights := make(map[string]int64)
	for _, name := range []string{"LOCAL_NODE_SEQUENCER_RESTART_HEIGHT", "LOCAL_NODE_SEQUENCER_FIRST_HEIGHT"} {
		value := os.Getenv(name)
		if value == "" {
			t.Fatalf("%s is required; start v6120-node.sh with --sequencer and source its env file", name)
		}
		if !decimal.MatchString(value) {
			t.Fatalf("%s = %q, want a decimal integer above 0", name, value)
		}
		height, err := strconv.ParseInt(value, 10, 64)
		if err != nil || height <= 0 {
			t.Fatalf("%s = %q, want a decimal int64 above 0: %v", name, value, err)
		}
		heights[name] = height
	}
	restart := heights["LOCAL_NODE_SEQUENCER_RESTART_HEIGHT"]
	first := heights["LOCAL_NODE_SEQUENCER_FIRST_HEIGHT"]
	if first <= restart {
		t.Fatalf("sequencer first height = %d, want above restart height %d", first, restart)
	}
	endpoint := os.Getenv("PERPS_DATA_TEST_COMET")
	if endpoint == "" {
		t.Fatal("PERPS_DATA_TEST_COMET is required; start v6120-node.sh with --sequencer and source its env file")
	}
	match := regexp.MustCompile(`^http://127\.0\.0\.1:([1-9][0-9]{0,4})$`).FindStringSubmatch(endpoint)
	if match == nil {
		t.Fatalf("PERPS_DATA_TEST_COMET = %q, want http://127.0.0.1:<port>", endpoint)
	}
	port, err := strconv.Atoi(match[1])
	if err != nil || port > 65535 {
		t.Fatalf("PERPS_DATA_TEST_COMET port = %q, want 1 through 65535", match[1])
	}
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Get(endpoint + "/status")
	if err != nil {
		t.Fatalf("GET Comet status: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET Comet status: HTTP %d, want %d", response.StatusCode, http.StatusOK)
	}
	var result coretypes.ResultStatus
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode Comet status: %v", err)
	}
	if result.SyncInfo.LatestBlockHeight < first {
		t.Fatalf("Comet latest block height = %d, want at least sequencer first height %d", result.SyncInfo.LatestBlockHeight, first)
	}
}
