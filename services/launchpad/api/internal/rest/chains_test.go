package rest_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

const chainHolderA = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const chainHolderB = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func newChainReadsFixture(t *testing.T) poolReadsFixture {
	t.Helper()
	f := newPoolReadsFixture(t, "")
	rows := []struct{ chain, chainID, pool, token, nft, block string }{
		{"7", "1007", f.first, "0x" + strings.Repeat("3", 40), "77", "101"},
		{"8", "1008", poolReadsSecond, "0x" + strings.Repeat("4", 40), "78", "102"},
		{"9", "1009", poolReadsStateless, "0x" + strings.Repeat("6", 40), "79", "103"},
	}
	for _, row := range rows {
		_, err := f.pool.Exec(context.Background(), `INSERT INTO indexer.launched_chains(chain,chain_id,pool,token,fee_right,creator,block_number,tx_hash,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active')`, row.chain, row.chainID, row.pool, row.token, row.nft, poolReadsCreator, row.block, "0x"+strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, epoch := range []struct{ number, amount, snapshot string }{{"1", "1000", "95"}, {"2", "2000", "96"}} {
		if _, err := f.pool.Exec(context.Background(), `INSERT INTO indexer.chain_revenue_epochs(chain,epoch,amount,snapshot_block,eligible_supply,opened_block) VALUES(7,$1,$2,$3,'10000',100)`, epoch.number, epoch.amount, epoch.snapshot); err != nil {
			t.Fatal(err)
		}
	}
	for _, claim := range []struct{ epoch, holder, amount string }{{"1", chainHolderA, "100"}, {"2", chainHolderA, "200"}, {"2", chainHolderB, "300"}} {
		if _, err := f.pool.Exec(context.Background(), `INSERT INTO indexer.chain_revenue_claims(chain,epoch,holder,amount,block_number,tx_hash) VALUES(7,$1,$2,$3,110,$4)`, claim.epoch, claim.holder, claim.amount, "0x"+strings.Repeat("b", 64)); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func chainData(t *testing.T, got v2Response) map[string]any {
	t.Helper()
	if got.status != 200 {
		t.Fatalf("status=%d body=%s", got.status, got.body)
	}
	var body map[string]any
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	if body["requestId"] != got.header.Get("X-Request-ID") || got.header.Get("X-Schema-Version") != "2" || body["quality"] != "complete" {
		t.Fatalf("envelope=%v headers=%v", body, got.header)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%v", body["data"])
	}
	return data
}

func TestChainsListPagesNewestFirst(t *testing.T) {
	f := newChainReadsFixture(t)
	first := getV2(t, f.srv.URL+"/v2/chains?limit=2")
	items := chainData(t, first)["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["chain"] != "9" || items[1].(map[string]any)["chain"] != "8" {
		t.Fatalf("first page=%v", items)
	}
	var envelope map[string]any
	if err := json.Unmarshal(first.body, &envelope); err != nil {
		t.Fatal(err)
	}
	cursor, ok := envelope["nextCursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("next cursor=%v", envelope["nextCursor"])
	}
	second := getV2(t, f.srv.URL+"/v2/chains?limit=2&cursor="+cursor)
	items = chainData(t, second)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["chain"] != "7" {
		t.Fatalf("second page=%v", items)
	}
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/chains?cursor=bad"), 400, "CURSOR_INVALID")
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO indexer.launched_chains(chain,chain_id,pool,token,fee_right,creator,block_number,tx_hash,status) VALUES(10,1010,$1,$2,'80',$3,104,$4,'active')`, poolReadsUnknown, "0x"+strings.Repeat("7", 40), poolReadsCreator, "0x"+strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/chains?limit=2&cursor="+cursor), 409, "RESNAPSHOT_REQUIRED")
}

func TestChainsDetailAndNotFound(t *testing.T) {
	f := newChainReadsFixture(t)
	chain := chainData(t, getV2(t, f.srv.URL+"/v2/chains/7"))["chain"].(map[string]any)
	if want := []string{"blockNumber", "chain", "chainId", "creator", "nftId", "pool", "status", "token"}; !reflect.DeepEqual(sortedKeys(chain), want) {
		t.Fatalf("chain fields=%v want %v", sortedKeys(chain), want)
	}
	if chain["chain"] != "7" || chain["chainId"] != "1007" || chain["nftId"] != "77" || chain["pool"] != f.first || chain["status"] != "active" {
		t.Fatalf("chain=%v", chain)
	}
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/chains/11"), 404, "NOT_FOUND")
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/chains/0"), 422, "FIELD_INVALID")
}

func TestChainsRevenueWithAndWithoutHolder(t *testing.T) {
	f := newChainReadsFixture(t)
	base := f.srv.URL + "/v2/chains/7/revenue"
	plain := chainData(t, getV2(t, base))
	epochs := plain["epochs"].([]any)
	if len(epochs) != 2 || epochs[0].(map[string]any)["epoch"] != "2" || epochs[0].(map[string]any)["epochAmount"] != "2000" || epochs[1].(map[string]any)["epoch"] != "1" {
		t.Fatalf("epochs=%v", epochs)
	}
	if _, present := plain["claims"]; present {
		t.Fatalf("claims without holder=%v", plain["claims"])
	}
	withHolder := chainData(t, getV2(t, base+"?holder="+chainHolderA))
	claims := withHolder["claims"].([]any)
	if len(claims) != 2 || claims[0].(map[string]any)["amount"] != "200" || claims[1].(map[string]any)["amount"] != "100" {
		t.Fatalf("claims=%v", claims)
	}
	other := chainData(t, getV2(t, base+"?holder="+chainHolderB))["claims"].([]any)
	if len(other) != 1 || other[0].(map[string]any)["amount"] != "300" {
		t.Fatalf("other holder claims=%v", other)
	}
	firstPage := getV2(t, base+"?holder="+chainHolderA+"&limit=1")
	if len(chainData(t, firstPage)["epochs"].([]any)) != 1 {
		t.Fatalf("first revenue page=%s", firstPage.body)
	}
	var pageEnvelope map[string]any
	if err := json.Unmarshal(firstPage.body, &pageEnvelope); err != nil {
		t.Fatal(err)
	}
	cursor := pageEnvelope["nextCursor"].(string)
	secondPage := chainData(t, getV2(t, base+"?holder="+chainHolderA+"&limit=1&cursor="+cursor))
	if secondPage["epochs"].([]any)[0].(map[string]any)["epoch"] != "1" || len(secondPage["claims"].([]any)) != 1 {
		t.Fatalf("second revenue page=%v", secondPage)
	}
	assertV2Error(t, getV2(t, base+"?holder="+chainHolderB+"&limit=1&cursor="+cursor), 409, "RESNAPSHOT_REQUIRED")
	assertV2Error(t, getV2(t, f.srv.URL+"/v2/chains/11/revenue"), 404, "NOT_FOUND")
}

func TestChainsFieldNamesMatchTheAbi(t *testing.T) {
	f := newChainReadsFixture(t)
	raw, err := os.ReadFile(filepath.Join(internaltest.RepositoryRoot(t), "precompiles", "launchpad", "abi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var abi []struct {
		Name    string `json:"name"`
		Outputs []struct {
			Name string `json:"name"`
		} `json:"outputs"`
		Inputs []struct {
			Name string `json:"name"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &abi); err != nil {
		t.Fatal(err)
	}
	abiNames := map[string]bool{}
	for _, entry := range abi {
		if entry.Name != "launchedChain" && entry.Name != "ChainRevenueEpochOpened" {
			continue
		}
		for _, field := range entry.Outputs {
			abiNames[field.Name] = true
		}
		for _, field := range entry.Inputs {
			abiNames[field.Name] = true
		}
	}
	chain := chainData(t, getV2(t, f.srv.URL+"/v2/chains/7"))["chain"].(map[string]any)
	for _, field := range []string{"chainId", "pool", "token", "nftId", "creator", "blockNumber"} {
		if !abiNames[field] {
			t.Fatalf("ABI lacks %s", field)
		}
		if _, ok := chain[field]; !ok {
			t.Fatalf("chain lacks %s", field)
		}
	}
	if !abiNames["registryNumber"] {
		t.Fatal("ABI lacks registryNumber")
	}
	if chain["chain"] != "7" {
		t.Fatalf("registry chain=%v", chain["chain"])
	}
	epoch := chainData(t, getV2(t, f.srv.URL+"/v2/chains/7/revenue"))["epochs"].([]any)[0].(map[string]any)
	if want := []string{"eligibleSupply", "epoch", "epochAmount", "snapshotBlock"}; !reflect.DeepEqual(sortedKeys(epoch), want) {
		t.Fatalf("epoch fields=%v want %v", sortedKeys(epoch), want)
	}
	for _, field := range []string{"epoch", "epochAmount", "snapshotBlock", "eligibleSupply"} {
		if !abiNames[field] {
			t.Fatalf("ABI lacks %s", field)
		}
		if _, ok := epoch[field]; !ok {
			t.Fatalf("epoch lacks %s", field)
		}
	}
}

func TestPoolDetailCarriesHomeChain(t *testing.T) {
	f := newChainReadsFixture(t)
	root := assertV2Success(t, getV2(t, f.srv.URL+"/v2/pools/"+f.first), "PoolDetailResponse")
	rootPool := root["data"].(map[string]any)["pool"].(map[string]any)
	if rootPool["homeChain"] != "0" {
		t.Fatalf("root homeChain=%v", rootPool["homeChain"])
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE indexer.pools SET home_chain=7 WHERE lower(pool_address)=$1`, f.first); err != nil {
		t.Fatal(err)
	}
	data := assertV2Success(t, getV2(t, f.srv.URL+"/v2/pools/"+f.first), "PoolDetailResponse")
	pool := data["data"].(map[string]any)["pool"].(map[string]any)
	if pool["homeChain"] != "7" {
		t.Fatalf("launched homeChain=%v", pool["homeChain"])
	}
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
