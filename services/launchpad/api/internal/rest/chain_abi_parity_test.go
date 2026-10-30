package rest

import (
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func loadChainParityABI(t *testing.T, path string) abi.ABI {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open ABI %s: %v", path, err)
	}
	defer file.Close()
	parsed, err := abi.JSON(file)
	if err != nil {
		t.Fatalf("parse ABI %s: %v", path, err)
	}
	return parsed
}

func TestDevChainMarketCreatedTopicMatchesPrecompileABI(t *testing.T) {
	launchpadABI := loadChainParityABI(t, "../../../../../precompiles/launchpad/abi.json")
	matches := 0
	for _, event := range launchpadABI.Events {
		if event.RawName != "MarketCreated" || len(event.Inputs) != 7 {
			continue
		}
		matches++
		if want := event.ID.Hex(); marketCreatedTopic != want {
			t.Errorf("marketCreatedTopic = %s, want %s for %s", marketCreatedTopic, want, event.Sig)
		}
	}
	if matches != 1 {
		t.Fatalf("found %d seven-input MarketCreated events, want 1", matches)
	}
}

func TestDevStreamDecimalsSelectorMatchesERC20ABI(t *testing.T) {
	erc20ABI := loadChainParityABI(t, "../../../../../precompiles/launchpad/abi/SidioraERC20.json")
	method, ok := erc20ABI.Methods["decimals"]
	if !ok {
		t.Fatal("decimals method missing from SidioraERC20 ABI")
	}
	if want := hexutil.Encode(method.ID); erc20DecimalsSelector != want {
		t.Fatalf("erc20DecimalsSelector = %s, want %s for %s", erc20DecimalsSelector, want, method.Sig)
	}
}
