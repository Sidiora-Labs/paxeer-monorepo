package testfixture

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

type Record struct {
	StoredBar    json.RawMessage              `json:"storedBar"`
	Fills        map[string]accounting.FillV1 `json:"fills"`
	PublicCandle json.RawMessage              `json:"publicCandle"`
}

func StoredBar(fill accounting.FillV1, interval string, checkpoint engine.SeriesCheckpoint, now time.Time) ([]byte, error) {
	folded, err := engine.FoldQualified(nil, fill, interval, false)
	if err != nil {
		return nil, err
	}
	bar, err := engine.DescribeSeries(folded, checkpoint, now)
	if err != nil {
		return nil, err
	}
	bar.DisplayRevision = 1
	return json.Marshal(bar)
}

func CommittedBar(fill accounting.FillV1, interval string) ([]byte, error) {
	return StoredBar(fill, interval, engine.SeriesCheckpoint{Height: 110, Timestamp: 1700000600}, time.Unix(1700000601, 0))
}

func Records() ([]Record, error) {
	pool := "0x" + strings.Repeat("1", 40)
	fill := accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: "125", Height: "110", TransactionIndex: "0", LogIndex: "0", TransactionHash: fmt.Sprintf("0x%024x%s", 1, pool[2:]), BlockHash: fmt.Sprintf("0x%064x", 0xb10c0000+110)}, ActualEmitter: pool, PoolAddress: pool, Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6}, Side: "buy", TokenAmountRaw: "1000000000000000000", USDLAmountRaw: "1000000", PaidInputRaw: "1000000", FeeRaw: "3000", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl", Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Provenance: accounting.Provenance{DeploymentID: "fixture125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch1", PoolCodeEpoch: "epoch1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)}, BlockTimestampSeconds: "1700000000"}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	fills := map[string]accounting.FillV1{fill.SourceFillID: fill}
	out := []Record{}
	for i, interval := range []string{"1m", "5m"} {
		now := time.Unix(1700000601, 0)
		if i == 1 {
			now = time.Unix(1700000001, 0)
		}
		raw, err := StoredBar(fill, interval, engine.SeriesCheckpoint{Height: 110, Timestamp: 1700000600}, now)
		if err != nil {
			return nil, err
		}
		var bar engine.SeriesCandle
		if err = json.Unmarshal(raw, &bar); err != nil {
			return nil, err
		}
		mapped, err := engine.CanonicalCandle(bar, fills)
		if err != nil {
			return nil, err
		}
		public, err := json.Marshal(mapped)
		if err != nil {
			return nil, err
		}
		out = append(out, Record{StoredBar: raw, Fills: fills, PublicCandle: public})
	}
	return out, nil
}
