package receipt

import (
	"bytes"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestGetReservesSelectorMatchesPoolABI(t *testing.T) {
	poolABIFile, err := os.Open("../../../../../precompiles/launchpad/abi/SidioraPool.json")
	if err != nil {
		t.Fatalf("open pool ABI: %v", err)
	}
	defer poolABIFile.Close()

	poolABI, err := abi.JSON(poolABIFile)
	if err != nil {
		t.Fatalf("parse pool ABI: %v", err)
	}
	method, ok := poolABI.Methods["getReserves"]
	if !ok {
		t.Fatal("pool ABI does not declare getReserves")
	}
	if !bytes.Equal(getReservesSelector, method.ID) {
		t.Fatalf("getReserves selector = %x, pool ABI method ID = %x", getReservesSelector, method.ID)
	}
}
