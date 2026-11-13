package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestDurableMarketRecoveryPreservesCommittedSourceCoordinates(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	partitionDDL, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, partitionDDL); err != nil {
		t.Fatal(err)
	}
	ddl, err := os.ReadFile("../../../../../migrations/190001.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	markets, err := ReadDiscoveredMarkets(ctx, pool)
	if err != nil || len(markets) != 0 {
		t.Fatalf("empty market recovery: %+v %v", markets, err)
	}
	input := repair53InputsFromProducer(t)
	for _, market := range input.Markets {
		if market.Creation == nil {
			t.Fatal("source creation proof absent")
		}
		body, err := json.Marshal(market.Creation.Provenance)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations VALUES($1,$2,$1,$1,$1,1,$3,$4,$5,$6,$7,1,$8,'{}')`, strings.ToLower(market.Token.Hex()), strings.ToLower(market.Pool.Hex()), int64(market.Height), strings.ToLower(market.Creation.BlockHash.Hex()), strings.ToLower(market.Creation.TransactionHash.Hex()), int64(market.TransactionIndex), int64(market.Creation.LogIndex), body); err != nil {
			t.Fatal(err)
		}
	}
	markets, err = ReadDiscoveredMarkets(ctx, pool)
	if err != nil || !reflect.DeepEqual(markets, input.Markets) {
		t.Fatalf("source-bound markets: %+v/%+v %v", markets, input.Markets, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_canonical_creations SET provenance='{}'`); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadDiscoveredMarkets(ctx, pool); !errors.Is(err, ErrQualification) {
		t.Fatalf("unproven market accepted: %v", err)
	}
	if _, err = ReadDiscoveredMarkets(ctx, nil); !errors.Is(err, ErrQualification) {
		t.Fatalf("missing store: %v", err)
	}
	pool.Close()
	if _, err = ReadDiscoveredMarkets(ctx, pool); err == nil {
		t.Fatal("closed store read succeeded")
	}
}
