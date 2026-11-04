package layerxpay_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"

	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestRepairRampCustomerPayment2231211(t *testing.T) {
	f := newEscrowTestFixture(t)
	h := f.h
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{BudgetEnabled: true, EscrowEnabled: true})
	id := [32]byte{0xc7}
	args := budgetTestArgs(t, f, id)
	a := h.precompile.GetABI()
	create, err := a.Pack("budgetCreate", args...)
	require.NoError(t, err)
	_, _, err = h.evm.Call(f.owner.caller, common.HexToAddress(lp.PayAddress), create, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	dir := os.Getenv("RAMP_REPAIR_EXCHANGE")
	require.NotEmpty(t, dir)
	// Publish actual customer-owned budget bindings for the Rust compiler.
	getBudget, err := a.Pack("getBudget", id)
	require.NoError(t, err)
	budgetABI, _, err := h.evm.StaticCall(f.owner.caller, common.HexToAddress(lp.PayAddress), getBudget, 2000000)
	require.NoError(t, err)
	purpose := args[3].([32]byte)
	_, operatorAccount, err := lxphost.MainAccountID(f.beneficiary.public)
	require.NoError(t, err)
	bindings, err := json.Marshal(map[string]interface{}{
		"customer":         "agent:" + string(f.owner.did) + ":main",
		"operator":         "agent:" + string(f.beneficiary.did) + ":main",
		"operator_account": hex.EncodeToString(operatorAccount[:]),
		"asset":            hex.EncodeToString(f.asset[:]),
		"budget":           hex.EncodeToString(id[:]),
		"purpose":          hex.EncodeToString(purpose[:]),
		"expiry_ms":        args[9],
		"budget_abi":       hex.EncodeToString(budgetABI),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "budget.json"), bindings, 0600))
	deadline := time.Now().Add(20 * time.Second)
	var input []byte
	for {
		input, err = os.ReadFile(filepath.Join(dir, "calldata"))
		if err == nil {
			break
		}
		require.True(t, os.IsNotExist(err))
		require.True(t, time.Now().Before(deadline), "Rust compiler must supply customer payment")
		time.Sleep(10 * time.Millisecond)
	}
	call, err := hex.DecodeString(string(input))
	require.NoError(t, err)
	require.True(t, bytes.Equal(call[:4], a.Methods["budgetSpend"].ID))
	before := budgetTestRecord(t, h, id)
	_, _, err = h.evm.Call(f.owner.caller, common.HexToAddress(lp.PayAddress), call, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	after := budgetTestRecord(t, h, id)
	require.NotEqual(t, before.SpentThisPeriod, after.SpentThisPeriod)
	logs := h.db.GetAllLogs()
	require.NotEmpty(t, logs)
	log := logs[len(logs)-1]
	encoded, err := json.Marshal(log)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "payment.json"), encoded, 0600))
}
