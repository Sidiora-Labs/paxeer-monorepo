package localnode

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

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
