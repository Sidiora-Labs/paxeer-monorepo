package configuration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "embed"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"
)

type Currency struct {
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type Fixture struct {
	ArchiveSHA256  string   `json:"archive_sha256"`
	ChainID        string   `json:"chain_id"`
	Address        string   `json:"address"`
	Currency       Currency `json:"currency"`
	SnapshotHeight int64    `json:"snapshot_height"`
	Balance        string   `json:"balance"`
	FaucetAmount   string   `json:"faucet_amount"`
	Node           string   `json:"node"`
}

type BootstrapBalance struct {
	AccountIdentifier struct {
		Address string `json:"address"`
	} `json:"account_identifier"`
	Currency Currency `json:"currency"`
	Value    string   `json:"value"`
}

//go:embed transfer.ros.tmpl
var transferTemplate string

func ValidateAddress(address string) error {
	if len(address) != 45 || address != strings.ToLower(address) {
		return errors.New("invalid Cosmos account address")
	}
	hrp, raw, err := bech32.DecodeAndConvert(address)
	if err != nil || hrp != "cosmos" || len(raw) != 20 {
		return errors.New("invalid Cosmos account address")
	}
	return nil
}

func positiveInteger(value string) bool {
	n, ok := new(big.Int).SetString(value, 10)
	return ok && n.Sign() > 0 && n.String() == value
}

func (f Fixture) Validate() error {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(f.ArchiveSHA256) || ValidateAddress(f.Address) != nil ||
		!regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9/:._-]{2,127}$`).MatchString(f.Currency.Symbol) || f.Currency.Decimals != 0 ||
		!regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`).MatchString(f.ChainID) || f.SnapshotHeight < 1 ||
		!positiveInteger(f.Balance) || !positiveInteger(f.FaucetAmount) || !strings.HasPrefix(f.Node, "tcp://") {
		return errors.New("invalid fixture metadata")
	}
	return nil
}

func LoadFixture(dir string) (Fixture, error) {
	var f Fixture
	b, err := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if err != nil {
		return f, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&f); err != nil {
		return f, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return f, errors.New("trailing fixture data")
	}
	return f, f.Validate()
}

func Metadata(f Fixture) (map[string][]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	balance := BootstrapBalance{Currency: f.Currency, Value: f.Balance}
	balance.AccountIdentifier.Address = f.Address
	bootstrap, err := json.MarshalIndent([]BootstrapBalance{balance}, "", "  ")
	if err != nil {
		return nil, err
	}
	manifest, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	currency, err := json.Marshal(f.Currency)
	if err != nil {
		return nil, err
	}
	transfer := strings.NewReplacer("@CURRENCY@", string(currency), "@GAS_PRICE@", "1"+f.Currency.Symbol).Replace(transferTemplate)
	return map[string][]byte{"fixture.json": append(manifest, '\n'), "bootstrap.json": append(bootstrap, '\n'), "transfer.ros": []byte(transfer)}, nil
}

func WriteMetadata(dir string, f Fixture) error {
	files, err := Metadata(f)
	if err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}
