package chainreader

import (
	"bytes"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestMarketCreatedTopicMatchesFactoryABI(t *testing.T) {
	data, err := os.ReadFile("../../../../../precompiles/launchpad/abi/SidioraFactory.json")
	if err != nil {
		t.Fatalf("read factory ABI: %v", err)
	}
	factoryABI, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse factory ABI: %v", err)
	}
	event, ok := factoryABI.Events["MarketCreated"]
	if !ok {
		t.Fatal("factory ABI is missing MarketCreated")
	}
	if marketCreatedFactoryTopic != event.ID {
		t.Fatalf("market creation topic = %s, factory ABI MarketCreated ID = %s", marketCreatedFactoryTopic.Hex(), event.ID.Hex())
	}
}
