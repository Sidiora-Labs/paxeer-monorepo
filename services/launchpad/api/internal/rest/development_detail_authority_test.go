package rest_test

import (
	"context"
	"testing"
	"time"
)

func TestDevelopmentPoolDetailRequiresCommittedStateAndCreation(t *testing.T) {
	f := newMktMarketFixture(t, true)
	receipt := f.createPool(t, mktPoolB, mktTokenB, "0x0000000000000000000000000000000000000000", time.Now().Unix())
	endpoint := f.srv.URL + "/v2/pools/" + mktPoolB
	f.setCursor(t, f.chain.head(t).Number.Uint64())
	assertV2Error(t, getV2(t, endpoint), 503, "DEPENDENCY_UNAVAILABLE")
	f.insertState(t, mktPoolB, receipt, "1000000000", "0", "1000000000000000000000000")
	assertV2Error(t, getV2(t, endpoint), 503, "DEPENDENCY_UNAVAILABLE")
	f.insertPoolStats(t, mktPoolB, mktTokenB, "0", "0", 0, 0, 0, time.Now().Unix(), time.Now().Unix())
	f.insertSwap(t, mktPoolB, mktTraderOne, true, "1000000", "1000000000000000000", "10000", receipt.BlockNumber.Int64(), time.Now().Unix(), 1)
	if tag, err := f.pool.Exec(context.Background(), `UPDATE indexer.pools SET created_block=999999 WHERE lower(pool_address)=$1`, mktPoolB); err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("change indexed creation: %v, %v", tag, err)
	}
	assertV2Error(t, getV2(t, endpoint), 503, "DEPENDENCY_UNAVAILABLE")
	if tag, err := f.pool.Exec(context.Background(), `UPDATE indexer.pools SET created_block=$2 WHERE lower(pool_address)=$1`, mktPoolB, receipt.BlockNumber.Int64()); err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("restore indexed creation: %v, %v", tag, err)
	}
	result := assertV2Success(t, waitForV2Ready(t, func() v2Response { return getV2(t, endpoint) }), "PoolDetailResponse")
	if mktData(result)["pool"].(map[string]any)["poolAddress"] != mktPoolB {
		t.Fatalf("restored committed pool detail: %v", result)
	}
}
