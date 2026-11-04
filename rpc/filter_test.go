package evmrpc_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	rpcfixtures "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/tests"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
)

func TestNewPendingTransactionFilterNotSupported(t *testing.T) {
	t.Parallel()
	resObj := sendRequestGood(t, "newPendingTransactionFilter")
	require.Contains(t, resObj, "error")
	errObj := resObj["error"].(map[string]interface{})
	require.Equal(t, float64(evmrpc.ErrCodeEVMNotSupported), errObj["code"])
	require.Contains(t, errObj["message"].(string), "eth_newPendingTransactionFilter")
}

func TestFilterNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		fromBlock string
		toBlock   string
		blockHash common.Hash
		addrs     []common.Address
		topics    [][]common.Hash
		wantErr   bool
	}{
		{
			name:      "happy path",
			fromBlock: "0x1",
			toBlock:   "0x2",
			addrs:     []common.Address{common.HexToAddress("0x123")},
			topics:    [][]common.Hash{{common.HexToHash("0x456")}},
			wantErr:   false,
		},
		{
			name:      "error: block hash and block range both given",
			fromBlock: "0x1",
			toBlock:   "0x2",
			blockHash: common.HexToHash("0xabc"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterCriteria := map[string]interface{}{
				"fromBlock": tt.fromBlock,
				"toBlock":   tt.toBlock,
				"address":   tt.addrs,
				"topics":    tt.topics,
			}
			if tt.blockHash != (common.Hash{}) {
				filterCriteria["blockHash"] = tt.blockHash.Hex()
			}
			if len(tt.fromBlock) > 0 || len(tt.toBlock) > 0 {
				filterCriteria["fromBlock"] = tt.fromBlock
				filterCriteria["toBlock"] = tt.toBlock
			}
			resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
			_, errExists := resObj["error"]

			if tt.wantErr {
				require.True(t, errExists)
			} else {
				require.False(t, errExists, "error should not exist")
				got := resObj["result"].(string)
				// make sure next filter id is not equal to this one
				resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
				got2 := resObj["result"].(string)
				require.NotEqual(t, got, got2)
			}
		})
	}
}

func TestFilterUninstall(t *testing.T) {
	t.Parallel()
	// uninstall existing filter
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x1",
	}
	resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)
	require.NotEmpty(t, filterId)

	resObj = sendRequest(t, TestPort, "uninstallFilter", filterId)
	uninstallSuccess := resObj["result"].(bool)
	require.True(t, uninstallSuccess)

	// uninstall non-existing filter
	nonExistingFilterId := "100"
	resObj = sendRequest(t, TestPort, "uninstallFilter", nonExistingFilterId)
	uninstallSuccess = resObj["result"].(bool)
	require.False(t, uninstallSuccess)
}

type GetFilterLogTests struct {
	name      string
	blockHash *common.Hash
	fromBlock string
	toBlock   string
	addrs     []common.Address
	topics    [][]common.Hash
	wantErr   bool
	wantLen   int
	check     func(t *testing.T, log map[string]interface{})
}

func getCommonFilterLogTests() []GetFilterLogTests {
	tests := []GetFilterLogTests{
		{
			name:      "filter by single address",
			fromBlock: "0x2",
			toBlock:   "0x2",
			addrs:     []common.Address{common.HexToAddress("0x1111111111111111111111111111111111111112")},
			wantErr:   false,
			check: func(t *testing.T, log map[string]interface{}) {
				require.Equal(t, "0x1111111111111111111111111111111111111112", log["address"].(string))
			},
			wantLen: 2,
		},
		{
			name:      "filter by single topic",
			fromBlock: "0x2",
			toBlock:   "0x2",
			topics:    [][]common.Hash{{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000123")}},
			wantErr:   false,
			check: func(t *testing.T, log map[string]interface{}) {
				require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000123", log["topics"].([]interface{})[0].(string))
			},
			wantLen: 4,
		},
		{
			name:      "filter by single topic with block range",
			fromBlock: "0x8",
			toBlock:   "0x8",
			topics:    [][]common.Hash{{common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")}},
			wantErr:   false,
			check: func(t *testing.T, log map[string]interface{}) {
				require.Equal(t, "0x1111111111111111111111111111111111111111111111111111111111111111", log["topics"].([]interface{})[0].(string))
			},
			wantLen: 1,
		},
		{
			name:      "error with from block ahead of to block",
			fromBlock: "0x3",
			toBlock:   "0x2",
			topics:    [][]common.Hash{{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000123")}},
			wantErr:   true,
		},
		{
			name:      "multiple addresses, multiple topics",
			fromBlock: "0x2",
			toBlock:   "0x2",
			addrs: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111112"),
				common.HexToAddress("0x1111111111111111111111111111111111111113"),
			},
			topics: [][]common.Hash{
				{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000123")},
				{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000456")},
			},
			wantErr: false,
			check: func(t *testing.T, log map[string]interface{}) {
				if log["address"].(string) != "0x1111111111111111111111111111111111111112" && log["address"].(string) != "0x1111111111111111111111111111111111111113" {
					t.Fatalf("address %s not in expected list", log["address"].(string))
				}
				firstTopic := log["topics"].([]interface{})[0].(string)
				secondTopic := log["topics"].([]interface{})[1].(string)
				require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000123", firstTopic)
				require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000456", secondTopic)
			},
			wantLen: 2,
		},
		{
			name:      "wildcard first topic",
			fromBlock: "0x2",
			toBlock:   "0x2",
			topics: [][]common.Hash{
				{},
				{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000456")},
			},
			wantErr: false,
			check: func(t *testing.T, log map[string]interface{}) {
				secondTopic := log["topics"].([]interface{})[1].(string)
				require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000456", secondTopic)
			},
			wantLen: 3,
		},
	}
	return tests
}

func TestFilterGetLogs(t *testing.T) {
	testFilterGetLogs(t, "eth", getCommonFilterLogTests())
}

func TestFilterPaxGetLogs(t *testing.T) {
	// make sure we pass all the eth_ namespace tests
	testFilterGetLogs(t, "pax", getCommonFilterLogTests())

	// test where we get a synthetic log
	testFilterGetLogs(t, "pax", []GetFilterLogTests{
		{
			name:      "filter by single synthetic address",
			fromBlock: "0x64",
			toBlock:   "0x64",
			addrs:     []common.Address{common.HexToAddress("0x1111111111111111111111111111111111111116")},
			wantErr:   false,
			check: func(t *testing.T, log map[string]interface{}) {
				require.Equal(t, "0x1111111111111111111111111111111111111116", log["address"].(string))
			},
			wantLen: 1,
		},
		{
			name:      "filter by single topic, include synethetic logs",
			topics:    [][]common.Hash{{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000234")}},
			wantErr:   false,
			fromBlock: "0x64",
			toBlock:   "0x64",
			check: func(t *testing.T, log map[string]interface{}) {
				require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000234", log["topics"].([]interface{})[0].(string))
			},
			wantLen: 1,
		},
		{
			name:    "filter by single topic with default range, include synethetic logs",
			topics:  [][]common.Hash{{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000234")}},
			wantErr: false,
			check: func(t *testing.T, log map[string]interface{}) {
				require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000234", log["topics"].([]interface{})[0].(string))
			},
			wantLen: 1,
		},
	})
}

func TestFilterEthEndpointReturnsNormalEvmLogEvenIfSyntheticLogIsInSameBlock(t *testing.T) {
	f := newCurrentFilterFixture(t, 100)
	testFilterGetLogsWithFixture(t, f, "eth", []GetFilterLogTests{
		{
			name:      "normal evm log is returned even if synthetic log is in the same block",
			fromBlock: "0x64", // 100
			toBlock:   "0x64",
			wantErr:   false,
			check: func(t *testing.T, log map[string]interface{}) {
				// check that none of the events have the synthetic hash
				syntheticHash := f.syntheticHash
				require.NotEqual(t, syntheticHash.Hex(), log["transactionHash"].(string))
			},
			wantLen: 2,
		},
	})
}

func testFilterGetLogs(t *testing.T, namespace string, tests []GetFilterLogTests) {
	f := newCurrentFilterFixture(t, 100)
	testFilterGetLogsWithFixture(t, f, namespace, tests)
}

func testFilterGetLogsWithFixture(t *testing.T, f *currentFilterFixture, namespace string, tests []GetFilterLogTests) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterCriteria := map[string]interface{}{
				"address": tt.addrs,
				"topics":  tt.topics,
			}
			if tt.blockHash != nil {
				filterCriteria["blockHash"] = tt.blockHash.Hex()
			}
			if len(tt.fromBlock) > 0 || len(tt.toBlock) > 0 {
				filterCriteria["fromBlock"] = tt.fromBlock
				filterCriteria["toBlock"] = tt.toBlock
			}
			var resObj map[string]interface{}
			if namespace == "eth" {
				resObj = f.request(t, namespace+"_getLogs", filterCriteria)
			} else if namespace == "pax" {
				resObj = f.request(t, namespace+"_getLogs", filterCriteria)
			} else {
				panic("unknown namespace")
			}
			if tt.wantErr {
				_, ok := resObj["error"]
				require.True(t, ok)
			} else {
				require.NotContains(t, resObj, "error")
				got := resObj["result"].([]interface{})
				for _, log := range got {
					logObj := log.(map[string]interface{})
					tt.check(t, logObj)
				}
				require.Equal(t, tt.wantLen, len(got))
			}
		})
	}
}

func TestFilterGetFilterLogs(t *testing.T) {
	f := newCurrentFilterFixture(t, 100)
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x2",
		"toBlock":   "0x2",
	}
	resObj := f.request(t, "eth_"+evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)

	resObj = f.request(t, "eth_"+evmrpc.GetFilterLogsMethod, filterId)
	logs := resObj["result"].([]interface{})
	require.Equal(t, 4, len(logs))
	for _, log := range logs {
		logObj := log.(map[string]interface{})
		require.Equal(t, "0x2", logObj["blockNumber"].(string))
	}

	// error: filter id does not exist
	nonexistentFilterId := 1000
	resObj = f.request(t, "eth_"+evmrpc.GetFilterLogsMethod, nonexistentFilterId)
	_, ok := resObj["error"]
	require.True(t, ok)
}

func TestFilterGetFilterChanges(t *testing.T) {
	f := newCurrentFilterFixture(t, 8)
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x2",
	}
	resObj := f.request(t, "eth_"+evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)

	resObj = f.request(t, "eth_"+evmrpc.GetFilterChangesMethod, filterId)
	logs := resObj["result"].([]interface{})
	// After tightening block/receipt matching, fromBlock=0x2 now yields 5 logs total
	require.Equal(t, 5, len(logs))
	logObj := logs[0].(map[string]interface{})
	require.Equal(t, "0x2", logObj["blockNumber"].(string))

	resObj = f.request(t, "eth_"+evmrpc.GetFilterChangesMethod, filterId)
	require.Empty(t, resObj["result"], "polling must advance its cursor")

	// error: filter id does not exist
	nonExistingFilterId := 1000
	resObj = f.request(t, "eth_"+evmrpc.GetFilterChangesMethod, nonExistingFilterId)
	_, ok := resObj["error"]
	require.True(t, ok)
}

func TestFilterBlockFilter(t *testing.T) {
	t.Parallel()
	resObj := sendRequestGood(t, "newBlockFilter")
	blockFilterId := resObj["result"].(string)
	resObj = sendRequestGood(t, evmrpc.GetFilterChangesMethod, blockFilterId)
	hashesInterface := resObj["result"].([]interface{})
	for _, hashInterface := range hashesInterface {
		hash := hashInterface.(string)
		require.Equal(t, 66, len(hash))
		require.Equal(t, "0x", hash[:2])
	}
	// query again to make sure cursor is updated
	resObj = sendRequestGood(t, evmrpc.GetFilterChangesMethod, blockFilterId)
	hashesInterface = resObj["result"].([]interface{})
	for _, hashInterface := range hashesInterface {
		hash := hashInterface.(string)
		require.Equal(t, 66, len(hash))
		require.Equal(t, "0x", hash[:2])
	}
}

func TestFilterExpiration(t *testing.T) {
	t.Parallel()
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x1",
	}
	resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)

	// wait for filter to expire
	time.Sleep(2 * filterTimeoutDuration)

	resObj = sendRequest(t, TestPort, evmrpc.GetFilterLogsMethod, filterId)
	_, ok := resObj["error"]
	require.True(t, ok)
}

func TestFilterGetFilterLogsKeepsFilterAlive(t *testing.T) {
	t.Parallel()
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x1",
	}
	resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)

	for i := 0; i < 5; i++ {
		// should keep filter alive
		resObj = sendRequestGood(t, evmrpc.GetFilterLogsMethod, filterId)
		_, ok := resObj["error"]
		require.False(t, ok)
		time.Sleep(filterTimeoutDuration / 2)
	}
}

func TestFilterGetFilterChangesKeepsFilterAlive(t *testing.T) {
	t.Parallel()
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x1",
	}
	resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)

	for i := 0; i < 5; i++ {
		// should keep filter alive
		resObj = sendRequestGood(t, evmrpc.GetFilterChangesMethod, filterId)
		_, ok := resObj["error"]
		require.False(t, ok)
		time.Sleep(filterTimeoutDuration / 2)
	}
}

func TestGetLogsBlockHashIsNotZero(t *testing.T) {
	f := newCurrentFilterFixture(t, 100)
	// Test that eth_getLogs returns logs with correct blockHash (not zero hash)
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x2",
		"toBlock":   "0x2",
	}
	resObj := f.request(t, "eth_"+"getLogs", filterCriteria)
	logs := resObj["result"].([]interface{})
	require.Greater(t, len(logs), 0, "should have at least one log")

	expectedBlockHash := f.hashes[2].Hex()
	zeroBlockHash := "0x0000000000000000000000000000000000000000000000000000000000000000"

	for i, logInterface := range logs {
		log := logInterface.(map[string]interface{})
		blockHash := log["blockHash"].(string)

		// The main check: blockHash should not be zero
		require.NotEqual(t, zeroBlockHash, blockHash,
			"log %d should not have zero blockHash", i)

		// Additional check: it should be the expected block hash for block 2
		require.Equal(t, expectedBlockHash, blockHash,
			"log %d should have correct blockHash for block 2", i)

		// Verify other expected fields are present
		require.Equal(t, "0x2", log["blockNumber"].(string),
			"log %d should be from block 2", i)
	}
}

func TestGetLogsTransactionIndexConsistency(t *testing.T) {
	t.Parallel()

	// Test that eth_getLogs returns logs with transaction indices that match eth_getBlockByNumber
	// This is a regression test for the transaction index mismatch issue

	// Try multiple blocks to find one with both logs and EVM transactions
	testBlocks := []string{"0x2", "0x8", "0x64", "0x67"} // Block 2, 8, 100, 103

	var logs []interface{}
	var transactions []interface{}
	var blockHex string

	for _, blockNum := range testBlocks {
		// Get logs for this block
		filterCriteria := map[string]interface{}{
			"fromBlock": blockNum,
			"toBlock":   blockNum,
		}
		resObj := sendRequestGood(t, "getLogs", filterCriteria)
		blockLogs := resObj["result"].([]interface{})

		if len(blockLogs) == 0 {
			continue // No logs in this block, try next
		}

		// Get the block to see what transaction indices eth_getBlockByNumber returns
		blockRes := sendRequestGood(t, "getBlockByNumber", blockNum, true)
		block := blockRes["result"].(map[string]interface{})
		blockTxs := block["transactions"].([]interface{})

		if len(blockTxs) > 0 {
			// Found a block with both logs and EVM transactions
			logs = blockLogs
			transactions = blockTxs
			blockHex = blockNum
			t.Logf("Using block %s with %d logs and %d EVM transactions", blockHex, len(logs), len(transactions))
			break
		}
	}

	// Skip test if we can't find a suitable block (this might happen in minimal test environments)
	if len(logs) == 0 || len(transactions) == 0 {
		t.Skip("No block found with both logs and EVM transactions - skipping transaction index consistency test")
		return
	}

	// Create a map of transaction hash to EVM transaction index from the block
	hashToEvmIndex := make(map[string]int)
	for i, txInterface := range transactions {
		tx := txInterface.(map[string]interface{})
		txHash := tx["hash"].(string)
		hashToEvmIndex[txHash] = i
	}

	// Verify that each log's transactionIndex is a valid EVM index (0 to len(transactions)-1)
	for i, logInterface := range logs {
		log := logInterface.(map[string]interface{})
		txHash := log["transactionHash"].(string)
		logTxIndex := log["transactionIndex"].(string)

		// Convert hex string to int for comparison
		logTxIndexInt, err := strconv.ParseInt(logTxIndex[2:], 16, 64)
		require.NoError(t, err, "should be able to parse transaction index from log %d", i)

		// Key assertion: transactionIndex should be a valid EVM transaction index
		require.GreaterOrEqual(t, logTxIndexInt, int64(0),
			"log %d: transactionIndex %d should be >= 0", i, logTxIndexInt)
		require.Less(t, logTxIndexInt, int64(len(transactions)),
			"log %d: transactionIndex %d should be < %d (number of EVM transactions)",
			i, logTxIndexInt, len(transactions))

		// If the transaction exists in the block, verify indices match
		if expectedEvmIndex, exists := hashToEvmIndex[txHash]; exists {
			require.Equal(t, int64(expectedEvmIndex), logTxIndexInt,
				"log %d: transactionIndex from eth_getLogs (%d) should match EVM transaction index from eth_getBlockByNumber (%d) for tx %s",
				i, logTxIndexInt, expectedEvmIndex, txHash)
		}
	}

	// Additional check: ensure transaction indices are reasonable for the block structure
	// Block 8 should have mixed transaction types, so EVM transaction indices should be sequential
	txIndicesFound := make(map[int64]bool)
	for _, logInterface := range logs {
		log := logInterface.(map[string]interface{})
		logTxIndex := log["transactionIndex"].(string)
		logTxIndexInt, _ := strconv.ParseInt(logTxIndex[2:], 16, 64)
		txIndicesFound[logTxIndexInt] = true
	}

	// We should not see indices that are >= number of EVM transactions
	for txIndex := range txIndicesFound {
		require.Less(t, txIndex, int64(len(transactions)),
			"no log should have transactionIndex %d when there are only %d EVM transactions",
			txIndex, len(transactions))
	}
}

func TestCollectLogsEvmTransactionIndex(t *testing.T) {
	t.Parallel()

	// Block 2 fixture includes mixed tx types.
	// Receipt.TransactionIndex values are absolute positions (1, 3, 4), but eth_getLogs
	// must return EVM-only transaction indexes (0, 1, 2).
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x2",
		"toBlock":   "0x2",
	}
	resObj := sendRequestGood(t, "getLogs", filterCriteria)
	logs := resObj["result"].([]interface{})
	require.Greater(t, len(logs), 0, "block 2 should have logs")

	expectedByHash := map[string]int64{
		multiTxBlockTx1.Hash().Hex(): 0,
		multiTxBlockTx2.Hash().Hex(): 1,
		multiTxBlockTx3.Hash().Hex(): 2,
	}
	absoluteIndexByHash := map[string]int64{
		multiTxBlockTx1.Hash().Hex(): 1,
		multiTxBlockTx2.Hash().Hex(): 3,
		multiTxBlockTx3.Hash().Hex(): 4,
	}
	seen := map[string]bool{}

	for i, logInterface := range logs {
		logObj := logInterface.(map[string]interface{})
		txHash := logObj["transactionHash"].(string)
		gotHex := logObj["transactionIndex"].(string)
		gotIndex, err := strconv.ParseInt(gotHex[2:], 16, 64)
		require.NoError(t, err, "log %d has invalid transactionIndex %q", i, gotHex)

		expectedIndex, tracked := expectedByHash[txHash]
		if !tracked {
			continue
		}

		seen[txHash] = true
		require.Equal(t, expectedIndex, gotIndex,
			"log %d tx %s should use EVM tx index %d", i, txHash, expectedIndex)
		require.NotEqual(t, absoluteIndexByHash[txHash], gotIndex,
			"log %d tx %s should not use absolute transaction index %d", i, txHash, absoluteIndexByHash[txHash])
	}

	require.Len(t, seen, len(expectedByHash), "should observe logs for all expected EVM txs in block 2")
}

// TestFilterGetFilterChangesEmptyOnExhaustedBoundedFilter asserts that
// eth_getFilterChanges returns [] (not null) once a bounded filter's range is
// fully consumed (Ethereum JSON-RPC spec requires an array, never null).
func TestFilterGetFilterChangesEmptyOnExhaustedBoundedFilter(t *testing.T) {
	t.Parallel()
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x2",
		"toBlock":   "0x2",
	}
	resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)

	// First call: consumes the range; result may be non-empty, but must be an array.
	resObj = sendRequest(t, TestPort, evmrpc.GetFilterChangesMethod, filterId)
	_, hasErr := resObj["error"]
	require.False(t, hasErr)
	require.NotNil(t, resObj["result"], "first getFilterChanges should return [] not null")
	_, ok := resObj["result"].([]interface{})
	require.True(t, ok, "first getFilterChanges result should be an array")

	// Second call: range is exhausted; must return [] not null.
	resObj = sendRequest(t, TestPort, evmrpc.GetFilterChangesMethod, filterId)
	_, hasErr = resObj["error"]
	require.False(t, hasErr)
	require.NotNil(t, resObj["result"], "exhausted getFilterChanges should return [] not null")
	logs, ok := resObj["result"].([]interface{})
	require.True(t, ok, "exhausted getFilterChanges result should be an array")
	require.Empty(t, logs)
}

// TestFilterGetFilterLogsEmptyResultIsArray asserts that eth_getFilterLogs
// returns [] (not null) when no logs match the filter criteria.
func TestFilterGetFilterLogsEmptyResultIsArray(t *testing.T) {
	t.Parallel()
	// Use an address that is guaranteed to have no logs in the mock data.
	filterCriteria := map[string]interface{}{
		"fromBlock": "0x1",
		"toBlock":   "0x1",
		"address":   "0x0000000000000000000000000000000000000000",
	}
	resObj := sendRequestGood(t, evmrpc.NewFilterMethod, filterCriteria)
	filterId := resObj["result"].(string)

	resObj = sendRequest(t, TestPort, evmrpc.GetFilterLogsMethod, filterId)
	_, hasErr := resObj["error"]
	require.False(t, hasErr)
	require.NotNil(t, resObj["result"], "getFilterLogs with no matching logs should return [] not null")
	logs, ok := resObj["result"].([]interface{})
	require.True(t, ok, "getFilterLogs result should be an array")
	require.Empty(t, logs)
}

// currentFilterFixture records receipts produced by the EVM message server and
// the native CallEVM entry point. Consensus is the committed transcript peer;
// the registered FilterAPI, codec, HTTP stack and client are production code.
type currentFilterFixture struct {
	url           string
	hashes        map[int64]common.Hash
	syntheticHash common.Hash
}

func filterLogCode(topics ...[]common.Hash) []byte {
	var code []byte
	for _, group := range topics {
		for i := len(group) - 1; i >= 0; i-- {
			code = append(code, 0x7f)
			code = append(code, group[i].Bytes()...)
		}
		code = append(code, 0x60, 0x00, 0x60, 0x00, byte(0xa0+len(group)))
	}
	return append(code, 0x00)
}

func newCurrentFilterFixture(t *testing.T, latest int64) *currentFilterFixture {
	t.Helper()
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	k := &a.EvmKeeper
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(latest).WithChainID("hyperpax_125-1").WithClosestUpgradeName(LatestCtxUpgradeName)
	ctx = ctx.WithConsensusParams(a.GetConsensusParams(ctx))
	consensus := &rpcfixtures.MockClient{}
	f := &currentFilterFixture{hashes: make(map[int64]common.Hash)}
	topic1, topic2 := common.HexToHash("0x123"), common.HexToHash("0x456")
	address := func(n byte) common.Address {
		return common.HexToAddress(fmt.Sprintf("0x11111111111111111111111111111111111111%02x", n))
	}
	records := []receiptstore.ReceiptRecord{}
	contexts := make(map[int64]sdk.Context, latest)
	for height := int64(1); height <= latest; height++ {
		// Current bloom keys are version-local, so each recorded block needs its
		// own store view rather than only a different context height.
		blockCtx, _ := ctx.WithBlockHeight(height).WithBlockTime(mockBlockHeader(height).Time).CacheContext()
		contexts[height] = blockCtx
		block := tmtypes.MakeBlock(height, nil, &tmtypes.Commit{Height: height - 1}, nil)
		block.Header = mockBlockHeader(height)
		var result []*abci.ExecTxResult
		var blooms, evmBlooms []ethtypes.Bloom
		emit := func(to common.Address, code []byte) {
			k.SetCode(blockCtx, to, code)
			k.SetNonce(blockCtx, to, 1)
			key, err := crypto.GenerateKey()
			require.NoError(t, err)
			from := crypto.PubkeyToAddress(key.PublicKey)
			coins := sdk.NewCoins(sdk.NewInt64Coin(k.GetBaseDenom(blockCtx), 10000000))
			require.NoError(t, a.BankKeeper.MintCoins(blockCtx, "evm", coins))
			require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(blockCtx, "evm", sdk.AccAddress(from.Bytes()), coins))
			chainID := k.ChainID(blockCtx)
			tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
				ChainID: chainID, Gas: 200000, GasFeeCap: big.NewInt(1000000000000), To: &to,
			})
			require.NoError(t, err)
			data, err := ethtx.NewDynamicFeeTx(tx)
			require.NoError(t, err)
			msg, err := evmtypes.NewMsgEVMTransaction(data)
			require.NoError(t, err)
			builder := TxConfig.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			raw, err := Encoder(builder.GetTx())
			require.NoError(t, err)
			index := len(block.Txs)
			execution := blockCtx.WithTxIndex(index).WithIsEVM(true).WithEVMNonce(0).WithEVMSenderAddress(from).WithEVMTxHash(tx.Hash())
			require.NoError(t, ante.Preprocess(execution, msg, chainID, false))
			execution, err = ante.NewEVMFeeCheckDecorator(k, &a.UpgradeKeeper).AnteHandle(execution, builder.GetTx(), false,
				func(next sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return next, nil })
			require.NoError(t, err)
			response, err := evmkeeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(execution), msg)
			require.NoError(t, err)
			require.Empty(t, response.VmError)
			receipt, err := k.GetTransientReceipt(execution, tx.Hash(), uint64(index))
			require.NoError(t, err)
			require.Equal(t, uint32(1), receipt.Status)
			records = append(records, receiptstore.ReceiptRecord{TxHash: tx.Hash(), Receipt: receipt})
			bloom := ethtypes.BytesToBloom(receipt.LogsBloom)
			blooms = append(blooms, bloom)
			evmBlooms = append(evmBlooms, bloom)
			block.Txs = append(block.Txs, raw)
			result = append(result, &abci.ExecTxResult{Code: abci.CodeTypeOK, GasUsed: int64(response.GasUsed)})
		}
		switch height {
		case 2:
			emit(address(0x12), filterLogCode([]common.Hash{topic1, topic2}, []common.Hash{topic1}))
			emit(address(0x13), filterLogCode([]common.Hash{topic1, topic2}))
			emit(address(0x14), filterLogCode([]common.Hash{topic1, topic2}))
		case 8:
			emit(address(0x15), filterLogCode([]common.Hash{common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")}))
		case 100:
			emit(address(0x12), filterLogCode([]common.Hash{topic1}))
			emit(address(0x13), filterLogCode([]common.Hash{topic1}))
			to := address(0x16)
			k.SetCode(blockCtx, to, filterLogCode([]common.Hash{common.HexToHash("0x234")}))
			native := &wasmtypes.MsgExecuteContract{Sender: sdk.AccAddress(to.Bytes()).String(), Contract: sdk.AccAddress(to.Bytes()).String(), Msg: []byte(`{}`)}
			builder := TxConfig.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(native))
			raw, err := Encoder(builder.GetTx())
			require.NoError(t, err)
			sum := sha256.Sum256(raw)
			f.syntheticHash = common.Hash(sum)
			nativeCtx := blockCtx.WithIsEVM(false).WithTxIndex(len(block.Txs)).WithTxSum(sum)
			_, err = k.CallEVM(nativeCtx, common.HexToAddress("0x1292"), &to, nil, nil)
			require.NoError(t, err)
			receipt, err := k.GetTransientReceipt(nativeCtx, sum, uint64(len(block.Txs)))
			require.NoError(t, err)
			require.Len(t, receipt.Logs, 1)
			records = append(records, receiptstore.ReceiptRecord{TxHash: sum, Receipt: receipt})
			blooms = append(blooms, ethtypes.BytesToBloom(receipt.LogsBloom))
			block.Txs = append(block.Txs, raw)
			result = append(result, &abci.ExecTxResult{Code: abci.CodeTypeOK, GasUsed: int64(receipt.GasUsed)})
		}
		k.SetBlockBloom(blockCtx, blooms)
		k.SetEvmOnlyBlockBloom(blockCtx, evmBlooms)
		// MakeBlock's cached data hash predates the appended producer transactions.
		block.DataHash = block.Txs.Hash()
		hash := common.BytesToHash(block.Hash())
		f.hashes[height] = hash
		consensus.RecordCommittedBlock(&coretypes.ResultBlock{BlockID: tmtypes.BlockID{Hash: hash.Bytes()}, Block: block},
			&coretypes.ResultBlockResults{Height: height, TxsResults: result})
	}
	require.NoError(t, k.ReceiptStore().SetReceipts(ctx, records))
	require.Eventually(t, func() bool {
		for _, record := range records {
			if _, err := k.GetReceipt(ctx, record.TxHash); err != nil {
				return false
			}
		}
		return true
	}, 5*time.Second, time.Millisecond)
	require.NoError(t, k.ReceiptStore().SetLatestVersion(latest))
	require.NoError(t, k.ReceiptStore().SetEarliestVersion(1))
	provider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return contexts[latest]
		}
		return contexts[height]
	}
	wm := evmrpc.NewWatermarkManager(consensus, provider, nil, k.ReceiptStore())
	apis := []gethrpc.API{}
	for _, namespace := range []string{"eth", "pax"} {
		api := evmrpc.NewFilterAPI(consensus, k, provider, func(int64) client.TxConfig { return TxConfig }, evmrpc.NewFilterConfig(time.Minute, 0, 0),
			evmrpc.ConnectionTypeHTTP, namespace, make(chan struct{}, evmrpc.MaxDBReadConcurrency), evmrpc.NewBlockCache(3000), &sync.Mutex{}, evmrpc.NewLogSlicePool(), wm)
		t.Cleanup(func() { evmrpc.ShutdownFilterAPIForTest(api) })
		apis = append(apis, gethrpc.API{Namespace: namespace, Service: api, Public: true})
	}
	server := evmrpc.NewHTTPServer(gethrpc.HTTPTimeouts{})
	require.NoError(t, server.EnableRPC(apis, evmrpc.HTTPConfig{Modules: []string{"eth", "pax"}, Vhosts: []string{"*"}, PaxLegacyAllowlist: evmrpc.BuildPaxLegacyEnabledSet(evmrpc.PaxLegacyAllGatedMethodNames())}))
	t.Cleanup(server.Stop)
	transport := httptest.NewServer(server)
	t.Cleanup(transport.Close)
	f.url = transport.URL
	return f
}

func (f *currentFilterFixture) request(t *testing.T, method string, params ...any) map[string]interface{} {
	t.Helper()
	if params == nil {
		params = []any{}
	}
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, f.url, strings.NewReader(string(raw)))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	return result
}
