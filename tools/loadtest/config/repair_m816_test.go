package config_test

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	loadtypes "github.com/paxeer-network/pax-load/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_816_RandomGasUint64Boundary(t *testing.T) {
	for _, field := range []string{"gasPicker", "gasFeeCapPicker", "gasTipCapPicker"} {
		for _, bounds := range []struct {
			name     string
			min, max uint64
		}{
			{"full uint64", 0, math.MaxUint64},
			{"upper endpoints", math.MaxUint64 - 1, math.MaxUint64},
			{"lower endpoints", 0, 1},
			{"above int64", 1 << 63, math.MaxUint64},
		} {
			t.Run(field+"/"+bounds.name, func(t *testing.T) {
				fields := map[string]json.RawMessage{
					"name":            json.RawMessage(`"EVMTransfer"`),
					"gasPicker":       json.RawMessage(`{"Name":"fixed","Gas":21000}`),
					"gasFeeCapPicker": json.RawMessage(`{"Name":"fixed","Gas":18446744073709551615}`),
					"gasTipCapPicker": json.RawMessage(`{"Name":"fixed","Gas":0}`),
				}
				fields[field] = json.RawMessage(fmt.Sprintf(`{"Name":"random","Min":%d,"Max":%d}`, bounds.min, bounds.max))
				data, err := json.Marshal(struct {
					ChainID   int64                        `json:"chainId"`
					Scenarios []map[string]json.RawMessage `json:"scenarios"`
				}{125, []map[string]json.RawMessage{fields}})
				require.NoError(t, err)
				var cfg config.LoadConfig
				require.NoError(t, json.Unmarshal(data, &cfg))
				account, err := loadtypes.NewAccount()
				require.NoError(t, err)
				scenario := scenarios.NewEVMTransferScenario(cfg.Scenarios[0]).(*scenarios.EVMTransferScenario)
				input := &loadtypes.TxScenario{Name: scenarios.EVMTransfer, Sender: account, Receiver: common.HexToAddress("0x1234")}
				seen := map[uint64]bool{}
				for range 128 {
					tx, err := scenario.CreateTransaction(&cfg, input)
					if err != nil {
						// Zero is a valid sample, but the transaction consumer refuses a zero gas limit.
						require.Equal(t, "gasPicker", field)
						require.Zero(t, bounds.min)
						require.EqualError(t, err, "gasPicker: gas limit must be greater than zero")
						seen[0] = true
						continue
					}
					loadTx := loadtypes.CreateTxFromEthTx(tx, input)
					var request loadtypes.JSONRPCRequest
					require.NoError(t, json.Unmarshal(loadTx.JSONRPCPayload, &request))
					require.Equal(t, "eth_sendRawTransaction", request.Method)
					var params []string
					require.NoError(t, json.Unmarshal(request.Params, &params))
					require.Len(t, params, 1)
					var decoded ethtypes.Transaction
					require.NoError(t, decoded.UnmarshalBinary(common.FromHex(params[0])))
					require.Equal(t, tx.Hash(), decoded.Hash())
					sender, err := ethtypes.Sender(ethtypes.NewCancunSigner(cfg.GetChainID()), &decoded)
					require.NoError(t, err)
					require.Equal(t, account.Address, sender)
					value := decoded.Gas()
					if field == "gasFeeCapPicker" {
						value = decoded.GasFeeCap().Uint64()
						require.True(t, decoded.GasFeeCap().IsUint64())
					}
					if field == "gasTipCapPicker" {
						value = decoded.GasTipCap().Uint64()
						require.True(t, decoded.GasTipCap().IsUint64())
					}
					require.GreaterOrEqual(t, value, bounds.min)
					require.LessOrEqual(t, value, bounds.max)
					seen[value] = true
				}
				require.Greater(t, len(seen), 1, "sampler must not collapse the admitted range")
				if bounds.max-bounds.min == 1 {
					require.Contains(t, seen, bounds.min)
					require.Contains(t, seen, bounds.max)
				}
			})
		}
	}
	for _, interval := range []string{`"Min":0,"Max":0`, `"Min":18446744073709551615,"Max":18446744073709551615`, `"Min":18446744073709551615,"Max":0`} {
		var picker config.GasPicker
		require.NoError(t, json.Unmarshal([]byte(`{"Name":"random",`+interval+`}`), &picker))
		value, err := picker.GenerateGas()
		require.ErrorContains(t, err, "invalid random gas range")
		require.Zero(t, value)
	}
}

func TestRepairM_816_ProfileCLI(t *testing.T) {
	if proofBoundsCLIChild(t) {
		return
	}
	TestProofPaxloadBoundsCallerGap(t)
}

func TestRepairM_816_ConfigurationPrecisionAndPrecedence(t *testing.T) {
	t.Run("precision and precedence", TestProofPaxloadBounds)
	t.Run("JSON keys", TestProofPaxloadBoundsJSONKeys)
	t.Run("malformed producer", TestProofPaxloadBoundsRefusesMalformedProducer)
	t.Run("latency boundary", TestProofPaxloadBoundsLatencyBoundary)
}
