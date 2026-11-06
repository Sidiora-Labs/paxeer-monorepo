package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

const vectorV7Name = "vector-v7-named-payee"

type v7Account struct {
	Name       string      `json:"name"`
	Identifier common.Hash `json:"identifier"`
}

type v7Parties struct {
	PayerDID          string      `json:"payer_did"`
	PayeeDID          string      `json:"payee_did"`
	PayeeMainAccount  v7Account   `json:"payee_main_account"`
	PayeeAssetAccount v7Account   `json:"payee_asset_account"`
	Asset             common.Hash `json:"asset"`
	Amount            string      `json:"amount"`
	Reference         common.Hash `json:"reference"`
}

type v7Vector struct {
	v4Vector
	Parties v7Parties `json:"parties"`
}

func init() {
	registerVector(vectorV7Name, recordV7)
}

func recordV7(n *node) (any, error) {
	var parties v7Parties
	recorded, err := recordV4Payment(n, &parties)
	if err != nil {
		return nil, err
	}
	v4, ok := recorded.(*v4Vector)
	if !ok {
		return nil, fmt.Errorf("V7 payment recorder returned %T", recorded)
	}
	v4.Name = vectorV7Name
	v4.Provenance += " Parties retain the bound payer and payee DIDs and the payee's native main and asset account names and identifiers used by the buyer human-evidence rung."
	return &v7Vector{v4Vector: *v4, Parties: parties}, nil
}
