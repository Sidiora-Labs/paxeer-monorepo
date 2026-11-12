package l16

import (
	"errors"
	"strings"
	"testing"
)

func TestFactorySupplyRequiresCanonicalMintAndPoolTransfer(t *testing.T) {
	factory := "0x" + strings.Repeat("a", 40)
	pool := "0x" + strings.Repeat("b", 40)
	tx := "0x" + strings.Repeat("c", 64)
	mint := movement{TransactionHash: tx, From: zeroAddress, To: factory, Amount: "100", LogIndex: 1}
	intoPool := movement{TransactionHash: tx, From: factory, To: pool, Amount: "100", LogIndex: 2}
	supply, err := factoryCreationSupply([]movement{mint, intoPool}, tx, factory, pool)
	if err != nil || supply.String() != "100" {
		t.Fatal(supply, err)
	}
	for _, tc := range []struct {
		name, factory string
		inputs        []movement
	}{
		{"factory identity", "invalid", []movement{mint, intoPool}},
		{"ambiguous creation", factory, []movement{mint, intoPool, intoPool}},
		{"noncanonical amount", factory, func() []movement { m, p := mint, intoPool; m.Amount = "bad"; p.Amount = "bad"; return []movement{m, p} }()},
		{"zero supply", factory, func() []movement { m, p := mint, intoPool; m.Amount = "0"; p.Amount = "0"; return []movement{m, p} }()},
		{"preexisting movement", factory, []movement{{TransactionHash: "0x" + strings.Repeat("d", 64)}, mint, intoPool}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := factoryCreationSupply(tc.inputs, tx, tc.factory, pool)
			if !errors.Is(err, ErrBaselineUnavailable) {
				t.Fatal(actual, err)
			}
		})
	}
}
