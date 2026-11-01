package localnode

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestLocalNodeV6120CometEnv(t *testing.T) {
	endpoint := os.Getenv("PERPS_DATA_TEST_COMET")
	if endpoint == "" {
		t.Fatal("PERPS_DATA_TEST_COMET is required; start v6120-node.sh and source its env file")
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
	if result.NodeInfo.Network != "hyperpax_125-1" {
		t.Fatalf("Comet network = %q, want hyperpax_125-1", result.NodeInfo.Network)
	}
}

func TestLocalNodeV6120Serves(t *testing.T) {
	endpoint := os.Getenv("PERPS_DATA_TEST_RPC")
	if endpoint == "" {
		t.Fatal("PERPS_DATA_TEST_RPC is required; start v6120-node.sh and source its env file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatalf("dial local node: %v", err)
	}
	defer client.Close()
	chain, err := client.ChainID(ctx)
	if err != nil {
		t.Fatalf("read chain ID: %v", err)
	}
	if chain.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("chain ID = %s, want 125", chain)
	}
	cases := []struct {
		method  string
		address string
		output  string
	}{
		{"totalMarkets", "0x0000000000000000000000000000000000001015", "uint256"},
		{"getRegisteredTokens", "0x0000000000000000000000000000000000001025", "address[]"},
	}
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			contract, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"` + tc.method + `","inputs":[],"outputs":[{"type":"` + tc.output + `"}],"stateMutability":"view"}]`))
			if err != nil {
				t.Fatalf("parse ABI: %v", err)
			}
			data, err := contract.Pack(tc.method)
			if err != nil {
				t.Fatalf("pack %s: %v", tc.method, err)
			}
			address := common.HexToAddress(tc.address)
			result, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
			if err != nil {
				t.Fatalf("call %s at %s: %v", tc.method, address, err)
			}
			values, err := contract.Unpack(tc.method, result)
			if err != nil || len(values) != 1 {
				t.Fatalf("decode %s: values=%v error=%v", tc.method, values, err)
			}
			switch tc.method {
			case "totalMarkets":
				count, ok := values[0].(*big.Int)
				if !ok || count.Cmp(big.NewInt(18)) != 0 {
					t.Fatalf("totalMarkets = %v, want 18", values[0])
				}
			case "getRegisteredTokens":
				tokens, ok := values[0].([]common.Address)
				if !ok || len(tokens) != 3 {
					t.Fatalf("getRegisteredTokens = %v, want 3 tokens", values[0])
				}
			}
		})
	}
}

func TestLocalNodeV6120PerpsMarket(t *testing.T) {
	endpoint := os.Getenv("LOCAL_NODE_RPC")
	keyHex := os.Getenv("LOCAL_NODE_KEY")
	if endpoint == "" || keyHex == "" {
		t.Fatal("LOCAL_NODE_RPC and LOCAL_NODE_KEY are required; start v6120-node.sh and source its env file")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(keyHex, "0x"))
	if err != nil {
		t.Fatalf("decode local node key: %v", err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatalf("dial local node: %v", err)
	}
	defer client.Close()
	contract, err := perpsabi.ABI()
	if err != nil {
		t.Fatalf("parse perps ABI: %v", err)
	}
	perps := common.HexToAddress("0x0000000000000000000000000000000000001015")
	usdl := common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
	call := func(method string, args ...interface{}) []interface{} {
		t.Helper()
		data, err := contract.Pack(method, args...)
		if err != nil {
			t.Fatalf("pack %s: %v", method, err)
		}
		result, err := client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &perps, Data: data}, nil)
		if err != nil {
			t.Fatalf("call %s: %v", method, err)
		}
		values, err := contract.Unpack(method, result)
		if err != nil || len(values) != len(contract.Methods[method].Outputs) {
			t.Fatalf("decode %s: values=%v error=%v", method, values, err)
		}
		return values
	}
	var market struct {
		Config struct {
			MarketId             *big.Int
			Symbol               string
			Name                 string
			OracleSymbol         string
			SzDecimals           uint8
			PxDecimals           uint8
			TickSize             *big.Int
			LotSize              *big.Int
			MaxLeverage          *big.Int
			TierClass            uint8
			ImpactNotionalUsd    *big.Int
			OiCapUsd             *big.Int
			FundingCapPpmPerHour *big.Int
			Enabled              bool
			ListedAt             uint64
		}
	}
	if err := contract.Methods["getMarketConfig"].Outputs.Copy(&market, call("getMarketConfig", big.NewInt(11))); err != nil {
		t.Fatalf("decode market config: %v", err)
	}
	cfg := market.Config
	if cfg.MarketId == nil || cfg.MarketId.Cmp(big.NewInt(11)) != 0 || cfg.Symbol != "PAX" || cfg.OracleSymbol != "PAX" || !cfg.Enabled {
		t.Fatalf("market 11 config = %+v, want enabled PAX with PAX oracle", cfg)
	}
	prices := call("getMarkPriceDetails", big.NewInt(11))
	mark, ok := prices[0].(*big.Int)
	if !ok || mark == nil || mark.Cmp(big.NewInt(10_000_000_000_000)) < 0 {
		t.Fatalf("market 11 mark = %v, want at least one 1e13 tick", prices[0])
	}
	if fresh, ok := prices[3].(bool); !ok || !fresh {
		t.Fatalf("market 11 fresh = %v, want true", prices[3])
	}
	code, err := client.CodeAt(ctx, usdl, nil)
	if err != nil {
		t.Fatalf("read USDL code: %v", err)
	}
	if len(code) == 0 {
		t.Fatal("USDL has no deployed runtime")
	}
	balances := call("getBalance", sender, usdl)
	collateral, ok := balances[0].(*big.Int)
	if !ok || collateral == nil || collateral.Cmp(big.NewInt(1_000_000_000)) < 0 {
		t.Fatalf("perps collateral for %s = %v, want at least 1000 USDL (1000000000 base units)", sender, balances[0])
	}
}

func localNodePendingCall(ctx context.Context, call func() error) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := call()
		if err == nil {
			return nil
		}
		if err.Error() != "pending block unavailable: sequencer: no block is open" && err.Error() != "sequencer: no block is open" {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func TestLocalNodeV6120SequencerReceipt(t *testing.T) {
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		home := os.Getenv("LOCAL_NODE_HOME")
		if home == "" {
			t.Log("LOCAL_NODE_HOME is unset; cannot read node.log")
			return
		}
		for _, name := range []string{"node.log"} {
			path := filepath.Join(home, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Logf("read node log %s: %v", path, err)
				continue
			}
			lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			for i, line := range lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "crashed") || strings.Contains(lower, "panic") {
					end := min(i+41, len(lines))
					t.Logf("panic context at %s:%d:\n%s", path, i+1, strings.Join(lines[i:end], "\n"))
				}
			}
			var replication []string
			for _, line := range lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "sequencer") || strings.Contains(lower, "replicat") {
					replication = append(replication, line)
				}
			}
			t.Logf("replication and sequencer lines of %s:\n%s", path, strings.Join(replication, "\n"))
			if len(lines) > 150 {
				lines = lines[len(lines)-150:]
			}
			t.Logf("last 150 lines of %s:\n%s", path, strings.Join(lines, "\n"))
		}
		for _, profile := range []struct{ env, name string }{
			{"LOCAL_NODE_PROFILE_RPC", "node-goroutines.txt"},
		} {
			endpoint := os.Getenv(profile.env)
			if endpoint == "" {
				t.Logf("%s is unset; cannot capture goroutines", profile.env)
				continue
			}
			response, err := (&http.Client{Timeout: 3 * time.Second}).Get(endpoint + "/debug/pprof/goroutine?debug=2")
			if err != nil {
				t.Logf("capture %s: %v", profile.name, err)
				continue
			}
			data, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusOK {
				t.Logf("capture %s: status %d, error %v", profile.name, response.StatusCode, err)
				continue
			}
			path := filepath.Join(home, profile.name)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Logf("retain goroutines %s: %v", path, err)
			}
			t.Logf("goroutines; evidence %s:\n%s", path, data)
		}
	})
	endpoint := os.Getenv("LOCAL_NODE_SEQUENCER_RPC")
	address := os.Getenv("LOCAL_NODE_ADDRESS")
	keyHex := os.Getenv("LOCAL_NODE_KEY")
	if endpoint == "" || address == "" || keyHex == "" {
		t.Fatal("sequencer RPC, address and key are required; start v6120-node.sh --sequencer and source its env file")
	}
	if endpoint != os.Getenv("LOCAL_NODE_RPC") {
		t.Fatal("sequencer must use the primary EVM RPC")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(keyHex, "0x"))
	if err != nil {
		t.Fatalf("decode local node key: %v", err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	if !common.IsHexAddress(address) || common.HexToAddress(address) != sender {
		t.Fatalf("LOCAL_NODE_ADDRESS = %q, want test key address %s", address, sender)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatalf("dial sequencer: %v", err)
	}
	defer client.Close()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		t.Fatalf("read sequencer chain ID: %v", err)
	}
	if chainID.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("chain ID = %s, want 125", chainID)
	}
	var nonce uint64
	err = localNodePendingCall(ctx, func() error {
		var err error
		nonce, err = client.PendingNonceAt(ctx, sender)
		return err
	})
	if err != nil {
		t.Fatalf("read sender nonce: %v", err)
	}
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatalf("read latest block: %v", err)
	}
	if header.BaseFee == nil {
		t.Fatal("latest block has no base fee")
	}
	gasPrice := new(big.Int).Add(new(big.Int).Mul(header.BaseFee, big.NewInt(2)), big.NewInt(1_000_000_000))
	recipient := common.HexToAddress("0x000000000000000000000000000000000000beef")
	var before *big.Int
	err = localNodePendingCall(ctx, func() error {
		var err error
		before, err = client.PendingBalanceAt(ctx, recipient)
		return err
	})
	if err != nil {
		t.Fatalf("read recipient balance before transfer: %v", err)
	}
	tx, err := types.SignTx(types.NewTx(&types.LegacyTx{
		Nonce: nonce, GasPrice: gasPrice, Gas: 21000, To: &recipient, Value: big.NewInt(1),
	}), types.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatalf("sign one-wei transfer: %v", err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("encode signed transfer: %v", err)
	}
	var hash common.Hash
	if err := localNodePendingCall(ctx, func() error {
		return client.Client().CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Encode(raw))
	}); err != nil {
		t.Fatalf("send one-wei transfer: %v", err)
	}
	if hash != tx.Hash() {
		t.Fatalf("submitted hash = %s, want %s", hash, tx.Hash())
	}
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		var receipt *evmrpc.SequencerReceiptResult
		if err := client.Client().CallContext(ctx, &receipt, "pax_sequencerReceipt", hash); err != nil {
			t.Fatalf("read sequencer receipt for %s: %v", hash, err)
		}
		if receipt != nil && receipt.Rung != evmrpc.RungSubmitted {
			if receipt.Rung != evmrpc.RungReceipted {
				t.Fatalf("receipt rung = %q, want receipted", receipt.Rung)
			}
			if receipt.SequencerReceiptView == nil || len(receipt.Receipt) == 0 {
				t.Fatal("receipted result has no signed receipt")
			}
			if receipt.TxHash != hash || uint64(receipt.Nonce) != nonce || receipt.From != sender || receipt.To != recipient || receipt.Status != 1 {
				t.Fatalf("receipt does not describe the successful one-wei transfer: %+v", receipt)
			}
			if err := receipt.Verify(); err != nil {
				t.Fatalf("verify signed receipt and its JSON view: %v", err)
			}
			var after *big.Int
			err := localNodePendingCall(ctx, func() error {
				var err error
				after, err = client.PendingBalanceAt(ctx, recipient)
				return err
			})
			if err != nil {
				t.Fatalf("read recipient balance after transfer: %v", err)
			}
			if delta := new(big.Int).Sub(after, before); delta.Cmp(big.NewInt(1)) != 0 {
				t.Fatalf("recipient balance increased by %s wei, want 1", delta)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for signed sequencer receipt for %s: %v", hash, ctx.Err())
		case <-tick.C:
		}
	}
}
