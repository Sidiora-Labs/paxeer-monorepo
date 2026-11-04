package distribution_test

import (
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const repair183ActivationHeight int64 = 10

type repair183Coin struct {
	Amount   *big.Int
	Decimals *big.Int
	Denom    string
}

type repair183Reward struct {
	Coins            []repair183Coin
	ValidatorAddress string
}

type repair183Rewards struct {
	Rewards []repair183Reward
	Total   []repair183Coin
}

type repair183Env struct {
	app        *node.App
	ctx        sdk.Context
	key        *ecdsa.PrivateKey
	caller     common.Address
	delegator  sdk.AccAddress
	precompile *pcommon.DynamicGasPrecompile
	expected   repair183Rewards
}

func newRepair183Env(t *testing.T, height int64, applied bool) *repair183Env {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.NewContext(false, tmtypes.Header{Height: 2, Time: time.Unix(1, 0).UTC()}).CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	if applied {
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair183ActivationHeight), pcommon.MergeUpgrade)
	}
	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[31] = 183
	validator := setupValidator(t, ctx, a, stakingtypes.Unbonded, key.PubKey())
	delegator := sdk.AccAddress(validator)
	ecdsaKey, err := crypto.ToECDSA(key.Bytes())
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, delegator, caller)
	val, found := a.StakingKeeper.GetValidator(ctx, validator)
	require.True(t, found)
	a.DistrKeeper.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(sdk.NewInt64Coin("uhpx", 1000)))
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 100_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, delegator, funds))
	ctx = ctx.WithBlockHeight(height)
	precompile, err := distribution.NewPrecompile(&node.PrecompileKeepers{DistributionKeeper: a.DistrKeeper, EVMKeeper: &a.EvmKeeper})
	require.NoError(t, err)
	require.Equal(t, "view", precompile.Methods[distribution.RewardsMethod].StateMutability)
	response, err := a.DistrKeeper.DelegationTotalRewards(sdk.WrapSDKContext(ctx), &distrtypes.QueryDelegationTotalRewardsRequest{DelegatorAddress: delegator.String()})
	require.NoError(t, err)
	require.NotEmpty(t, response.Rewards)
	require.True(t, response.Total.AmountOf("uhpx").IsPositive())
	expected := repair183Rewards{Rewards: make([]repair183Reward, 0), Total: make([]repair183Coin, 0)}
	for _, reward := range response.Rewards {
		row := repair183Reward{ValidatorAddress: reward.ValidatorAddress, Coins: make([]repair183Coin, 0)}
		for _, coin := range reward.Reward {
			row.Coins = append(row.Coins, repair183Coin{Amount: coin.Amount.BigInt(), Denom: coin.Denom, Decimals: big.NewInt(sdk.Precision)})
		}
		expected.Rewards = append(expected.Rewards, row)
	}
	for _, coin := range response.Total {
		expected.Total = append(expected.Total, repair183Coin{Amount: coin.Amount.BigInt(), Denom: coin.Denom, Decimals: big.NewInt(sdk.Precision)})
	}
	return &repair183Env{app: a, ctx: ctx, key: ecdsaKey, caller: caller, delegator: delegator, precompile: precompile, expected: expected}
}

func (e *repair183Env) transaction(t *testing.T, value *big.Int) *ethtypes.Transaction {
	t.Helper()
	input, err := e.precompile.Pack(distribution.RewardsMethod, e.caller)
	require.NoError(t, err)
	to := e.precompile.Address()
	signer := ethtypes.LatestSignerForChainID(e.app.EvmKeeper.ChainID(e.ctx))
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{GasPrice: big.NewInt(1_000_000_000_000), Gas: 1_000_000, To: &to, Value: value, Data: input}), signer, e.key)
	require.NoError(t, err)
	raw, err := tx.MarshalBinary()
	require.NoError(t, err)
	decoded := new(ethtypes.Transaction)
	require.NoError(t, decoded.UnmarshalBinary(raw))
	require.Equal(t, tx.Hash(), decoded.Hash())
	return decoded
}

func (e *repair183Env) evm(t *testing.T) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	k := &e.app.EvmKeeper
	db := state.NewDBImpl(e.ctx, k, false)
	t.Cleanup(db.Cleanup)
	block, err := k.GetVMBlockContext(db.Ctx(), k.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(e.ctx)), vm.Config{}, map[common.Address]vm.PrecompiledContract{e.precompile.Address(): e.precompile})
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db
}

func (e *repair183Env) assertRewards(t *testing.T, output []byte) {
	t.Helper()
	decoded, err := e.precompile.Methods[distribution.RewardsMethod].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	actual := abi.ConvertType(decoded[0], new(repair183Rewards)).(*repair183Rewards)
	require.Equal(t, e.expected, *actual)
}

func TestRepair_183_PositiveValueRawCALL(t *testing.T) {
	for _, height := range []int64{repair183ActivationHeight, repair183ActivationHeight + 1} {
		for _, value := range []int64{1, 1_000_000_000_000} {
			t.Run(big.NewInt(height).String()+"/"+big.NewInt(value).String(), func(t *testing.T) {
				e := newRepair183Env(t, height, true)
				tx := e.transaction(t, big.NewInt(value))
				msg, err := core.TransactionToMessage(tx, ethtypes.LatestSignerForChainID(tx.ChainId()), nil)
				require.NoError(t, err)
				evm, db := e.evm(t)
				callerBefore := db.GetBalance(msg.From).ToBig()
				precompileBefore := db.GetBalance(*msg.To).ToBig()
				output, _, err := evm.Call(msg.From, *msg.To, msg.Data, msg.GasLimit, uint256.MustFromBig(msg.Value))
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Empty(t, output)
				require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
				require.Equal(t, callerBefore, db.GetBalance(msg.From).ToBig())
				require.Equal(t, precompileBefore, db.GetBalance(*msg.To).ToBig())
				_, err = db.Finalize()
				require.NoError(t, err)
				reopened := state.NewDBImpl(e.ctx, &e.app.EvmKeeper, true)
				t.Cleanup(reopened.Cleanup)
				require.Equal(t, callerBefore, reopened.GetBalance(msg.From).ToBig())
				require.Equal(t, precompileBefore, reopened.GetBalance(*msg.To).ToBig())
			})
		}
	}
}

func TestRepair_183_ZeroValueRealRewards(t *testing.T) {
	for _, height := range []int64{repair183ActivationHeight - 1, repair183ActivationHeight} {
		for _, static := range []bool{false, true} {
			e := newRepair183Env(t, height, true)
			tx := e.transaction(t, big.NewInt(0))
			evm, db := e.evm(t)
			callerBefore := db.GetBalance(e.caller).ToBig()
			precompileBefore := db.GetBalance(*tx.To()).ToBig()
			var output []byte
			var err error
			if static {
				output, _, err = evm.StaticCall(e.caller, *tx.To(), tx.Data(), tx.Gas())
			} else {
				output, _, err = evm.Call(e.caller, *tx.To(), tx.Data(), tx.Gas(), uint256.NewInt(0))
			}
			require.NoError(t, err)
			e.assertRewards(t, output)
			require.Equal(t, callerBefore, db.GetBalance(e.caller).ToBig())
			require.Equal(t, precompileBefore, db.GetBalance(*tx.To()).ToBig())
		}
	}
}

func TestRepair_183_HistoricalReplay(t *testing.T) {
	for _, applied := range []bool{false, true} {
		height := repair183ActivationHeight - 1
		if !applied {
			height = repair183ActivationHeight + 1
		}
		e := newRepair183Env(t, height, applied)
		require.False(t, pcommon.MergePlanActive(e.ctx, &e.app.EvmKeeper))
		tx := e.transaction(t, big.NewInt(1_000_000_000_001))
		evm, db := e.evm(t)
		callerBefore := db.GetBalance(e.caller).ToBig()
		precompileBefore := db.GetBalance(*tx.To()).ToBig()
		output, _, err := evm.Call(e.caller, *tx.To(), tx.Data(), tx.Gas(), uint256.MustFromBig(tx.Value()))
		require.NoError(t, err)
		e.assertRewards(t, output)
		require.Equal(t, new(big.Int).Sub(callerBefore, tx.Value()), db.GetBalance(e.caller).ToBig())
		require.Equal(t, new(big.Int).Add(precompileBefore, tx.Value()), db.GetBalance(*tx.To()).ToBig())
	}
}

func TestRepair_183_RawTransactionReverts(t *testing.T) {
	e := newRepair183Env(t, repair183ActivationHeight, true)
	tx := e.transaction(t, big.NewInt(1))
	data, err := ethtx.NewLegacyTx(tx)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	ctx := e.ctx.WithIsEVM(true).WithEVMNonce(tx.Nonce()).WithEVMSenderAddress(e.caller).WithEVMTxHash(tx.Hash())
	k := &e.app.EvmKeeper
	require.NoError(t, ante.Preprocess(ctx, msg, k.ChainID(ctx), false))
	builder := e.app.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	before := state.NewDBImpl(ctx, k, true)
	callerBefore := before.GetBalance(e.caller).ToBig()
	precompileBefore := before.GetBalance(*tx.To()).ToBig()
	before.Cleanup()
	executionCtx, err := ante.NewEVMFeeCheckDecorator(k, &e.app.UpgradeKeeper).AnteHandle(ctx, builder.GetTx(), false, func(next sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return next, nil })
	require.NoError(t, err)
	response, err := keeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(executionCtx), msg)
	require.NoError(t, err)
	require.Equal(t, vm.ErrExecutionReverted.Error(), response.VmError)
	require.Empty(t, response.ReturnData)
	after := state.NewDBImpl(ctx, k, true)
	defer after.Cleanup()
	fee := new(big.Int).Mul(new(big.Int).SetUint64(response.GasUsed), tx.GasPrice())
	require.Positive(t, response.GasUsed)
	require.Equal(t, callerBefore, new(big.Int).Add(after.GetBalance(e.caller).ToBig(), fee))
	require.Equal(t, precompileBefore, after.GetBalance(*tx.To()).ToBig())
	receipt, err := k.GetTransientReceipt(ctx, tx.Hash(), uint64(ctx.TxIndex()))
	require.NoError(t, err)
	require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
}
