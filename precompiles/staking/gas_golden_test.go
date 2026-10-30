package staking_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStakingGasGolden(t *testing.T) {
	const planHeight int64 = 100
	const suppliedGas uint64 = 20_000_000
	a := testkeeper.EVMTestApp
	fixedTime := time.Unix(1_800_000_000, 0).UTC()
	base, _ := a.GetContextForDeliverTx(nil).WithBlockHeight(8).
		WithBlockTime(fixedTime).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).CacheContext()
	val1 := setupValidator(t, base, a, stakingtypes.Unbonded, ed25519.GenPrivKeyFromSecret([]byte("staking-gas-validator-1")).PubKey())
	val2 := setupValidator(t, base, a, stakingtypes.Unbonded, ed25519.GenPrivKeyFromSecret([]byte("staking-gas-validator-2")).PubKey())
	delegator := sdk.AccAddress(val1)
	caller := common.BytesToAddress(delegator)
	creator := common.BytesToAddress(bytes.Repeat([]byte{0x33}, 20))
	address := common.HexToAddress(staking.StakingAddress)
	for _, account := range []common.Address{caller, creator, address} {
		paxAddress := sdk.AccAddress(account.Bytes())
		a.EvmKeeper.SetAddressMapping(base, paxAddress, account)
		funds := sdk.NewCoins(sdk.NewInt64Coin(a.StakingKeeper.BondDenom(base), 1_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(base, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(base, "mint", paxAddress, funds))
	}
	a.StakingKeeper.SetUnbondingDelegationEntry(base, delegator, val1, 7, fixedTime.Add(time.Hour), sdk.NewInt(10))
	a.StakingKeeper.SetRedelegationEntry(base, delegator, val1, val2, 7, fixedTime.Add(time.Hour), sdk.NewInt(10), sdk.NewDec(10), sdk.NewDec(10))
	validator1, found := a.StakingKeeper.GetValidator(base, val1)
	require.True(t, found)
	validator2, found := a.StakingKeeper.GetValidator(base, val2)
	require.True(t, found)
	a.StakingKeeper.SetHistoricalInfo(base, 7, &stakingtypes.HistoricalInfo{
		Header: tmtypes.Header{Height: 7, Time: fixedTime},
		Valset: []stakingtypes.Validator{validator1, validator2},
	})
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(2), "v6.4.0")
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(planHeight), app.MergeUpgrade)
	require.Equal(t, planHeight, a.UpgradeKeeper.GetDoneHeight(base, app.MergeUpgrade))
	precompile, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	contractABI := precompile.GetABI()
	pubkey := hex.EncodeToString(ed25519.GenPrivKeyFromSecret([]byte("staking-gas-validator-3")).PubKey().Bytes())
	rows := []struct {
		method string
		args   []interface{}
		caller common.Address
		value  *big.Int
		gas    uint64
	}{
		{"createValidator", []interface{}{pubkey, "gas-validator", "0.05", "0.20", "0.01", big.NewInt(1)}, creator, big.NewInt(100_000_000_000_000), 124217},
		{"delegate", []interface{}{val1.String()}, caller, big.NewInt(10_000_000_000_000), 102259},
		{"delegation", []interface{}{caller, val1.String()}, caller, nil, 5371},
		{"delegatorDelegations", []interface{}{caller, []byte{}}, caller, nil, 4914},
		{"delegatorUnbondingDelegations", []interface{}{caller, []byte{}}, caller, nil, 2131},
		{"delegatorValidator", []interface{}{caller, val1.String()}, caller, nil, 4302},
		{"delegatorValidators", []interface{}{caller, []byte{}}, caller, nil, 3845},
		{"editValidator", []interface{}{"gas-validator-edited", "", big.NewInt(2)}, caller, nil, 12439},
		{"historicalInfo", []interface{}{int64(7)}, caller, nil, 2338},
		{"params", nil, caller, nil, 8828},
		{"pool", nil, caller, nil, 6134},
		{"redelegate", []interface{}{val1.String(), val2.String(), big.NewInt(10)}, caller, nil, 135282},
		{"redelegations", []interface{}{delegator.String(), val1.String(), val2.String(), []byte{}}, caller, nil, 3440},
		{"unbondingDelegation", []interface{}{caller, val1.String()}, caller, nil, 2597},
		{"undelegate", []interface{}{val1.String(), big.NewInt(10)}, caller, nil, 99364},
		{"validator", []interface{}{val1.String()}, caller, nil, 1696},
		{"validatorDelegations", []interface{}{val1.String(), []byte{}}, caller, nil, 4304},
		{"validatorUnbondingDelegations", []interface{}{val1.String(), []byte{}}, caller, nil, 1792},
		{"validators", []interface{}{"BOND_STATUS_UNBONDED", []byte{}}, caller, nil, 2178},
	}
	require.Len(t, rows, len(contractABI.Methods))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		require.False(t, seen[row.method], "duplicate golden row %s", row.method)
		seen[row.method] = true
		t.Run(row.method, func(t *testing.T) {
			method, exists := contractABI.Methods[row.method]
			require.True(t, exists)
			input, err := contractABI.Pack(row.method, row.args...)
			require.NoError(t, err)
			var charged [2]uint64
			var outputs [2][]byte
			for i, height := range []int64{planHeight - 1, planHeight} {
				t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
					ctx, _ := base.WithBlockHeight(height).CacheContext()
					require.Equal(t, height >= planHeight, a.UpgradeKeeper.IsUpgradeActiveAtHeight(ctx, app.MergeUpgrade, height))
					served, exists := a.EvmKeeper.CustomPrecompiles(ctx)[address]
					require.True(t, exists)
					p, ok := served.(*pcommon.DynamicGasPrecompile)
					require.True(t, ok, "staking must use dynamic gas")
					require.Equal(t, contractABI, p.GetABI())
					evm := &vm.EVM{StateDB: state.NewDBImpl(ctx, &a.EvmKeeper, false)}
					ret, remaining, err := p.RunAndCalculateGas(evm, row.caller, row.caller, input, suppliedGas, row.value, nil, method.IsConstant(), false)
					require.NoError(t, err, "staking call reverted: %x", ret)
					require.Less(t, remaining, suppliedGas)
					charged[i] = suppliedGas - remaining
					outputs[i] = ret
					values, err := method.Outputs.Unpack(ret)
					require.NoError(t, err)
					require.NotEmpty(t, values)
					if !method.IsConstant() {
						require.Equal(t, []interface{}{true}, values)
					}
					assert.Equal(t, row.gas, charged[i], "golden gas for %s at height %d", row.method, height)
				})
			}
			assert.Equal(t, charged[0], charged[1], "merge changed dynamic gas")
			assert.Equal(t, outputs[0], outputs[1], "merge changed staking result")
		})
	}
	for name := range contractABI.Methods {
		require.True(t, seen[name], "ABI method %s has no gas golden", name)
	}
}
