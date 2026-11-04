package scenarios_test

// Task 22.31.323 repair record:
// contract-gas-pickers is proved by TestRepair_323_SignedProfileSelections,
// TestRepair_323_RandomProfileSelections and TestRepair_323_InvalidSelections.
// Wiring request: domain 16 owns tools/loadtest/generator/utils/utils.go;
// move the scenario-options helper there when that path is authorized.
// Baseline verification was not run: the explicit gate order requires the whole
// implementation before hf-build and hf-done, and prohibits a manual verify run.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator"
	"github.com/paxeer-network/pax-load/generator/bindings"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/types"
)

var repair323Scenarios = []struct {
	name       string
	defaultGas uint64
	metadata   *bind.MetaData
	method     string
}{
	{scenarios.ERC20, 72156, bindings.ERC20MetaData, "transfer"},
	{scenarios.ERC20Conflict, 100000, bindings.ERC20ConflictMetaData, "transfer"},
	{scenarios.ERC20Noop, 22460, bindings.ERC20NoopMetaData, "transfer"},
	{scenarios.ERC721, 100000, bindings.ERC721MetaData, "mint"},
	{scenarios.Disperse, 200000, bindings.DisperseMetaData, "disperseEtherFixed"},
	{scenarios.EVMTransfer, 21000, nil, ""},
	{scenarios.EVMTransferNoop, 21000, nil, ""},
}

func repair323Profile(t *testing.T, name, selections string) *config.LoadConfig {
	t.Helper()
	var cfg config.LoadConfig
	profile := fmt.Sprintf(`{"chainId":125,"mockDeploy":true,"accounts":{"count":1},"scenarios":[{"name":%q,"weight":1%s}]}`, name, selections)
	if err := json.Unmarshal([]byte(profile), &cfg); err != nil {
		t.Fatal(err)
	}
	return &cfg
}

func repair323Decode(t *testing.T, load *types.LoadTx) *ethtypes.Transaction {
	t.Helper()
	var request struct {
		Version string   `json:"jsonrpc"`
		Method  string   `json:"method"`
		Params  []string `json:"params"`
	}
	if err := json.Unmarshal(load.JSONRPCPayload, &request); err != nil {
		t.Fatal(err)
	}
	if request.Version != "2.0" || request.Method != "eth_sendRawTransaction" || len(request.Params) != 1 {
		t.Fatalf("invalid transaction transport: %+v", request)
	}
	raw, err := hexutil.Decode(request.Params[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, load.Payload) {
		t.Fatal("JSON-RPC payload differs from binary payload")
	}
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		t.Fatal(err)
	}
	if tx.Hash() != load.EthTx.Hash() || tx.Type() != ethtypes.DynamicFeeTxType || tx.ChainId().Cmp(big.NewInt(125)) != 0 {
		t.Fatal("signed transaction identity, type or chain was lost in transport")
	}
	sender, err := ethtypes.Sender(ethtypes.NewCancunSigner(big.NewInt(125)), &tx)
	if err != nil {
		t.Fatal(err)
	}
	if sender != load.Scenario.Sender.Address {
		t.Fatalf("signature recovered %s, want %s", sender, load.Scenario.Sender.Address)
	}
	return &tx
}

func repair323AssertFees(t *testing.T, tx *ethtypes.Transaction, gas, tip, fee uint64) {
	t.Helper()
	if tx.Gas() != gas || tx.GasTipCap().Cmp(new(big.Int).SetUint64(tip)) != 0 || tx.GasFeeCap().Cmp(new(big.Int).SetUint64(fee)) != 0 {
		t.Fatalf("signed gas/tip/fee = %d/%s/%s, want %d/%d/%d", tx.Gas(), tx.GasTipCap(), tx.GasFeeCap(), gas, tip, fee)
	}
}

func TestRepair_323_SignedProfileSelections(t *testing.T) {
	for _, sc := range repair323Scenarios {
		t.Run(sc.name, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				selections string
				gas        uint64
				tip        uint64
				fee        uint64
			}{
				{"defaults", "", sc.defaultGas, 2000000000, 20000000000},
				{"all", `,"gasPicker":{"Name":"fixed","Gas":345678},"gasTipCapPicker":{"Name":"fixed","Gas":3000000000},"gasFeeCapPicker":{"Name":"fixed","Gas":40000000000}`, 345678, 3000000000, 40000000000},
				{"gas_only", `,"gasPicker":{"Name":"fixed","Gas":345678}`, 345678, 2000000000, 20000000000},
				{"tip_only", `,"gasTipCapPicker":{"Name":"fixed","Gas":3000000000}`, sc.defaultGas, 3000000000, 20000000000},
				{"fee_only", `,"gasFeeCapPicker":{"Name":"fixed","Gas":40000000000}`, sc.defaultGas, 2000000000, 40000000000},
				{"unsigned_fees", `,"gasTipCapPicker":{"Name":"fixed","Gas":9223372036854775815},"gasFeeCapPicker":{"Name":"fixed","Gas":18446744073709551615}`, sc.defaultGas, 9223372036854775815, math.MaxUint64},
				{"zero_fees", `,"gasTipCapPicker":{"Name":"fixed","Gas":0},"gasFeeCapPicker":{"Name":"fixed","Gas":0}`, sc.defaultGas, 0, 0},
			} {
				t.Run(tc.name, func(t *testing.T) {
					gen, err := generator.NewConfigBasedGenerator(repair323Profile(t, sc.name, tc.selections))
					if err != nil {
						t.Fatal(err)
					}
					for nonce := uint64(0); nonce < 2; nonce++ {
						load, ok := gen.Generate()
						if !ok || load == nil {
							t.Fatal("generator did not emit a transaction")
						}
						tx := repair323Decode(t, load)
						repair323AssertFees(t, tx, tc.gas, tc.tip, tc.fee)
						if tx.Nonce() != nonce || tx.To() == nil {
							t.Fatalf("nonce or destination lost: %d / %v", tx.Nonce(), tx.To())
						}
						if sc.metadata == nil {
							wantTo := load.Scenario.Receiver
							if sc.name == scenarios.EVMTransferNoop {
								wantTo = load.Scenario.Sender.Address
								if tx.Value().Sign() != 0 {
									t.Fatal("noop transfer has a nonzero value")
								}
							} else if tx.Value().Sign() <= 0 {
								t.Fatal("transfer value was lost")
							}
							if *tx.To() != wantTo || len(tx.Data()) != 0 {
								t.Fatal("transfer destination or data changed")
							}
							continue
						}
						contractABI, err := sc.metadata.GetAbi()
						if err != nil {
							t.Fatal(err)
						}
						if len(tx.Data()) < 4 {
							t.Fatal("contract calldata is missing")
						}
						method, err := contractABI.MethodById(tx.Data()[:4])
						if err != nil || method.Name != sc.method {
							t.Fatalf("contract method = %v, error = %v", method, err)
						}
						args, err := method.Inputs.Unpack(tx.Data()[4:])
						if err != nil {
							t.Fatal(err)
						}
						if sc.name == scenarios.Disperse {
							if len(args) != 1 || len(args[0].([]common.Address)) != scenarios.DisperseBatchSize {
								t.Fatalf("disperse recipients were lost: %v", args)
							}
							continue
						}
						wantAmount := uint64(1)
						if sc.name == scenarios.ERC721 {
							wantAmount = nonce + 1
						}
						if len(args) != 2 || args[0].(common.Address) != load.Scenario.Receiver || args[1].(*big.Int).Cmp(new(big.Int).SetUint64(wantAmount)) != 0 {
							t.Fatalf("contract arguments changed: %v", args)
						}
					}
				})
			}
		})
	}
}

func TestRepair_323_RandomProfileSelections(t *testing.T) {
	const selections = `,"gasPicker":{"Name":"random","Min":300000,"Max":400000},"gasTipCapPicker":{"Name":"random","Min":3000000000,"Max":4000000000},"gasFeeCapPicker":{"Name":"random","Min":30000000000,"Max":40000000000}`
	for _, sc := range repair323Scenarios {
		t.Run(sc.name, func(t *testing.T) {
			gen, err := generator.NewConfigBasedGenerator(repair323Profile(t, sc.name, selections))
			if err != nil {
				t.Fatal(err)
			}
			for range 8 {
				load, ok := gen.Generate()
				if !ok || load == nil {
					t.Fatal("generator did not emit a transaction")
				}
				tx := repair323Decode(t, load)
				if tx.Gas() < 300000 || tx.Gas() > 400000 || tx.GasTipCap().Cmp(big.NewInt(3000000000)) < 0 || tx.GasTipCap().Cmp(big.NewInt(4000000000)) > 0 || tx.GasFeeCap().Cmp(big.NewInt(30000000000)) < 0 || tx.GasFeeCap().Cmp(big.NewInt(40000000000)) > 0 {
					t.Fatalf("signed transaction does not preserve random picker bounds: %d / %s / %s", tx.Gas(), tx.GasTipCap(), tx.GasFeeCap())
				}
			}
		})
	}
}

func TestRepair_323_InvalidSelections(t *testing.T) {
	for _, sc := range repair323Scenarios {
		t.Run(sc.name, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				selections string
				field      string
			}{
				{"gas_range", `,"gasPicker":{"Name":"random","Min":4,"Max":3}`, "gasPicker"},
				{"tip_range", `,"gasTipCapPicker":{"Name":"random","Min":4,"Max":3}`, "gasTipCapPicker"},
				{"fee_range", `,"gasFeeCapPicker":{"Name":"random","Min":4,"Max":3}`, "gasFeeCapPicker"},
				{"empty_picker", `,"gasPicker":{}`, "gasPicker"},
				{"zero_gas", `,"gasPicker":{"Name":"fixed","Gas":0}`, "gasPicker"},
				{"fee_below_tip", `,"gasTipCapPicker":{"Name":"fixed","Gas":5},"gasFeeCapPicker":{"Name":"fixed","Gas":4}`, "gasFeeCapPicker"},
				{"tip_above_default_fee", `,"gasTipCapPicker":{"Name":"fixed","Gas":30000000000}`, "gasTipCapPicker"},
				{"fee_below_default_tip", `,"gasFeeCapPicker":{"Name":"fixed","Gas":1}`, "gasFeeCapPicker"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cfg := repair323Profile(t, sc.name, tc.selections)
					consumer := scenarios.CreateScenario(cfg.Scenarios[0])
					if gen, err := generator.NewConfigBasedGenerator(cfg); gen != nil || err == nil || !strings.Contains(err.Error(), tc.field) {
						t.Fatalf("invalid construction produced generator %v, error %v; want %s error", gen, err, tc.field)
					}
					sender, err := types.NewAccount()
					if err != nil {
						t.Fatal(err)
					}
					tx, err := consumer.(scenarios.ScenarioDeployer).CreateTransaction(cfg, &types.TxScenario{
						Name: sc.name, Sender: sender, Receiver: common.HexToAddress("0x324"),
					})
					if tx != nil || err == nil || !strings.Contains(err.Error(), tc.field) {
						t.Fatalf("invalid selection produced tx %v, error %v; want %s error", tx, err, tc.field)
					}
				})
			}
		})
	}
}
