package v65_test

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	stakingv65 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking/legacy/v65"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	distrkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const repairM1212Height int64 = 100
const repairM1212Gas uint64 = 2_000_000

var repairM1212Address = common.HexToAddress(staking.StakingAddress)

type repairM1212Env struct {
	app     *repairM1212Keepers
	keeper  evmkeeper.Keeper
	ctx     sdk.Context
	key     *ecdsa.PrivateKey
	caller  common.Address
	payer   sdk.AccAddress
	current *pcommon.DynamicGasPrecompile
	value   *big.Int
	stake   sdk.Int
}

type repairM1212Keepers struct {
	utils.Keepers
	BankKeeper         bankkeeper.BaseKeeper
	AccountKeeper      authkeeper.AccountKeeper
	StakingKeeper      stakingkeeper.Keeper
	UpgradeKeeper      upgradekeeper.Keeper
	EvmKeeper          *evmkeeper.Keeper
	DistributionKeeper distrkeeper.Keeper
}

func (k *repairM1212Keepers) BankK() utils.BankKeeper { return k.BankKeeper }
func (k *repairM1212Keepers) EVMK() utils.EVMKeeper   { return k.EvmKeeper }
func (k *repairM1212Keepers) StakingK() utils.StakingKeeper {
	return stakingkeeper.NewMsgServerImpl(k.StakingKeeper)
}
func (k *repairM1212Keepers) StakingQ() utils.StakingQuerier {
	return stakingkeeper.Querier{Keeper: k.StakingKeeper}
}
func (k *repairM1212Keepers) DistributionK() utils.DistributionKeeper { return k.DistributionKeeper }
func (k *repairM1212Keepers) GetPrecompileKeepers() utils.Keepers     { return k }

func repairM1212Fixture(t *testing.T) (*repairM1212Keepers, sdk.Context) {
	t.Helper()
	if _, registered := sdk.GetDenomUnit("uhpx"); !registered {
		require.NoError(t, sdk.RegisterDenom("uhpx", sdk.OneDec()))
	}
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	multi := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		upgradetypes.StoreKey, evmtypes.StoreKey, paramtypes.StoreKey, distrtypes.StoreKey)
	for _, key := range keys {
		multi.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKeys(paramtypes.TStoreKey, evmtypes.TransientStoreKey)
	for _, key := range transient {
		multi.MountStoreWithDB(key, sdk.StoreTypeTransient, nil)
	}
	require.NoError(t, multi.LoadLatestVersion())
	ctx := sdk.NewContext(multi, tmproto.Header{ChainID: "repair-m1212", Height: repairM1212Height,
		Time: time.Unix(1_800_000_000, 0).UTC()}, false).
		WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, amino, keys[paramtypes.StoreKey], transient[paramtypes.TStoreKey], name)
	}
	permissions := map[string][]string{
		"mint": {authtypes.Minter}, evmtypes.ModuleName: {authtypes.Minter, authtypes.Burner},
		authtypes.FeeCollectorName: {}, distrtypes.ModuleName: {},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	}
	k := &repairM1212Keepers{}
	k.AccountKeeper = authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, permissions)
	k.AccountKeeper.SetParams(ctx, authtypes.DefaultParams())
	for name, perms := range permissions {
		k.AccountKeeper.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, perms...))
	}
	k.BankKeeper = bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], k.AccountKeeper, subspace(banktypes.ModuleName), map[string]bool{})
	k.BankKeeper.InitGenesis(ctx, banktypes.DefaultGenesisState())
	k.StakingKeeper = stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], k.AccountKeeper, k.BankKeeper, subspace(stakingtypes.ModuleName))
	params := stakingtypes.DefaultParams()
	params.BondDenom = "uhpx"
	k.StakingKeeper.SetParams(ctx, params)
	k.DistributionKeeper = distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], subspace(distrtypes.ModuleName), k.AccountKeeper, k.BankKeeper, k.StakingKeeper, authtypes.FeeCollectorName, map[string]bool{})
	k.DistributionKeeper.InitGenesis(ctx, *distrtypes.DefaultGenesisState())
	k.StakingKeeper.SetHooks(k.DistributionKeeper.Hooks())
	k.UpgradeKeeper = upgradekeeper.NewKeeper(nil, keys[upgradetypes.StoreKey], cdc, "", nil)
	k.EvmKeeper = evmkeeper.NewKeeper(keys[evmtypes.StoreKey], transient[evmtypes.TransientStoreKey], subspace(evmtypes.ModuleName), nil,
		k.BankKeeper, &k.AccountKeeper, &k.StakingKeeper, transferkeeper.Keeper{}, nil, nil, &k.UpgradeKeeper)
	k.EvmKeeper.SetParams(ctx, evmtypes.DefaultParams())
	feeCollector := k.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	k.EvmKeeper.SetAddressMapping(ctx, feeCollector, common.BytesToAddress(feeCollector))
	return k, ctx
}

func newRepairM1212Env(t *testing.T) *repairM1212Env {
	t.Helper()
	a, ctx := repairM1212Fixture(t)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	private := &secp256k1.PrivKey{Key: make([]byte, 32)}
	private.Key[30], private.Key[31] = 12, 12
	key, err := crypto.ToECDSA(private.Bytes())
	require.NoError(t, err)
	payer := sdk.AccAddress(private.PubKey().Address())
	caller := crypto.PubkeyToAddress(key.PublicKey)
	a.EvmKeeper.SetAddressMapping(ctx, payer, caller)
	stake := sdk.NewInt(1_000_000)
	funds := sdk.NewCoins(sdk.NewCoin(a.StakingKeeper.BondDenom(ctx), stake.MulRaw(10)))
	require.Equal(t, sdk.MustGetBaseDenom(), funds[0].Denom)
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer, funds))
	current, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	return &repairM1212Env{app: a, keeper: *a.EvmKeeper, ctx: ctx, key: key, caller: caller,
		payer: payer, current: current, stake: stake, value: new(big.Int).Mul(stake.BigInt(), big.NewInt(1_000_000_000_000))}
}

func (e *repairM1212Env) activate() {
	e.app.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(repairM1212Height), pcommon.MergeUpgrade)
}

func (e *repairM1212Env) input(t *testing.T, key string) []byte {
	t.Helper()
	input, err := e.current.Pack(staking.CreateValidatorMethod, key, "repair-m1212", "0.05", "0.20", "0.01", big.NewInt(1))
	require.NoError(t, err)
	return input
}

func (e *repairM1212Env) message(t *testing.T, key string) *core.Message {
	t.Helper()
	signer := ethtypes.LatestSignerForChainID(e.keeper.ChainID(e.ctx))
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		GasPrice: big.NewInt(1_000_000_000_000), Gas: repairM1212Gas,
		To: &repairM1212Address, Value: e.value, Data: e.input(t, key),
	}), signer, e.key)
	require.NoError(t, err)
	wire, err := tx.MarshalBinary()
	require.NoError(t, err)
	decoded := new(ethtypes.Transaction)
	require.NoError(t, decoded.UnmarshalBinary(wire))
	require.Equal(t, tx.Hash(), decoded.Hash())
	msg, err := core.TransactionToMessage(decoded, signer, nil)
	require.NoError(t, err)
	require.Equal(t, e.caller, msg.From)
	return msg
}

func (e *repairM1212Env) evm(t *testing.T, contracts map[common.Address]vm.PrecompiledContract) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	db := state.NewDBImpl(e.ctx, &e.keeper, false)
	t.Cleanup(db.Cleanup)
	block, err := e.keeper.GetVMBlockContext(db.Ctx(), e.keeper.GetGasPool())
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(e.keeper.ChainID(e.ctx)), vm.Config{}, contracts)
	evm.SetTxContext(vm.TxContext{Origin: e.caller})
	return evm, db
}

type repairM1212State struct {
	Balances    []sdk.Coins
	Wei         []sdk.Int
	Validators  []stakingtypes.Validator
	Delegations []stakingtypes.Delegation
	Supply      sdk.Coin
}

func (e *repairM1212Env) state(ctx sdk.Context) repairM1212State {
	addresses := []sdk.AccAddress{e.payer, e.keeper.GetPaxAddressOrDefault(ctx, repairM1212Address),
		e.app.AccountKeeper.GetModuleAddress(stakingtypes.NotBondedPoolName),
		e.app.AccountKeeper.GetModuleAddress(stakingtypes.BondedPoolName)}
	result := repairM1212State{Validators: e.app.StakingKeeper.GetAllValidators(ctx),
		Delegations: e.app.StakingKeeper.GetAllDelegations(ctx),
		Supply:      e.app.BankKeeper.GetSupply(ctx, sdk.MustGetBaseDenom())}
	for _, address := range addresses {
		result.Balances = append(result.Balances, e.app.BankKeeper.GetAllBalances(ctx, address))
		result.Wei = append(result.Wei, e.app.BankKeeper.GetWeiBalance(ctx, address))
	}
	return result
}

func TestRepairM_1212_KeyLengthRefusal(t *testing.T) {
	cases := []struct{ name, key, reason string }{
		{"empty", "", "invalid public key length: expected 32 bytes, got 0"},
		{"one_byte", "ab", "invalid public key length: expected 32 bytes, got 1"},
		{"31_bytes", strings.Repeat("ab", 31), "invalid public key length: expected 32 bytes, got 31"},
		{"33_bytes", strings.Repeat("ab", 33), "invalid public key length: expected 32 bytes, got 33"},
		{"64_bytes", strings.Repeat("ab", 64), "invalid public key length: expected 32 bytes, got 64"},
		{"odd_hex", strings.Repeat("a", 63), "invalid public key hex format"},
		{"non_hex", strings.Repeat("zz", 32), "invalid public key hex format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRepairM1212Env(t)
			e.activate()
			msg := e.message(t, tc.key)
			evm, db := e.evm(t, map[common.Address]vm.PrecompiledContract{repairM1212Address: e.current})
			before := e.state(db.Ctx())
			require.NotPanics(t, func() {
				_, left, err := evm.Call(msg.From, *msg.To, msg.Data, msg.GasLimit, uint256.MustFromBig(msg.Value))
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.LessOrEqual(t, left, msg.GasLimit)
			})
			require.EqualError(t, db.GetPrecompileError(), tc.reason)
			require.Equal(t, before, e.state(db.Ctx()))
			require.Empty(t, db.GetAllLogs())
			_, err := db.Finalize()
			require.NoError(t, err)
			require.Equal(t, before, e.state(e.ctx))
		})
	}
}

func TestRepairM_1212_BeforePayment(t *testing.T) {
	for _, size := range []int{0, 31, 33} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			e := newRepairM1212Env(t)
			e.activate()
			precompile := e.keeper.GetPaxAddressOrDefault(e.ctx, repairM1212Address)
			require.NoError(t, e.app.BankKeeper.SendCoins(e.ctx, e.payer, precompile,
				sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), e.stake))))
			evm, db := e.evm(t, map[common.Address]vm.PrecompiledContract{repairM1212Address: e.current})
			before := e.state(db.Ctx())
			payments := 0
			hooks := &tracing.Hooks{OnEnter: func(int, byte, common.Address, common.Address, []byte, uint64, *big.Int) { payments++ }}
			_, _, err := e.current.RunAndCalculateGas(evm, e.caller, e.caller,
				e.input(t, strings.Repeat("ab", size)), repairM1212Gas, e.value, hooks, false, false)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.EqualError(t, db.GetPrecompileError(), fmt.Sprintf("invalid public key length: expected 32 bytes, got %d", size))
			require.Zero(t, payments, "the payment refund must never start")
			require.Equal(t, before, e.state(db.Ctx()), "no outer EVM revert masks a premature payment")
			require.Empty(t, db.GetAllLogs())
		})
	}
}

func TestRepairM_1212_ValidKey(t *testing.T) {
	public := ed25519.GenPrivKeyFromSecret([]byte("repair-m1212-consensus")).PubKey()
	for _, upper := range []bool{false, true} {
		t.Run(fmt.Sprintf("uppercase_%t", upper), func(t *testing.T) {
			e := newRepairM1212Env(t)
			e.activate()
			key := hex.EncodeToString(public.Bytes())
			if upper {
				key = strings.ToUpper(key)
			}
			msg := e.message(t, key)
			evm, db := e.evm(t, map[common.Address]vm.PrecompiledContract{repairM1212Address: e.current})
			before := e.state(db.Ctx())
			out, left, err := evm.Call(msg.From, *msg.To, msg.Data, msg.GasLimit, uint256.MustFromBig(msg.Value))
			require.NoError(t, err)
			require.Positive(t, left)
			decoded, err := e.current.Methods[staking.CreateValidatorMethod].Outputs.Unpack(out)
			require.NoError(t, err)
			require.Equal(t, []interface{}{true}, decoded)
			require.Nil(t, db.GetPrecompileError())
			require.Len(t, db.GetAllLogs(), 1)
			_, err = db.Finalize()
			require.NoError(t, err)
			validator, found := e.app.StakingKeeper.GetValidator(e.ctx, sdk.ValAddress(e.payer))
			require.True(t, found)
			actual, err := validator.ConsPubKey()
			require.NoError(t, err)
			require.Equal(t, public.Bytes(), actual.Bytes())
			require.Equal(t, e.stake, validator.Tokens)
			require.Equal(t, "repair-m1212", validator.Description.Moniker)
			delegation, found := e.app.StakingKeeper.GetDelegation(e.ctx, e.payer, sdk.ValAddress(e.payer))
			require.True(t, found)
			require.Equal(t, e.stake.ToDec(), delegation.Shares)
			after := e.state(e.ctx)
			coin := sdk.NewCoin(sdk.MustGetBaseDenom(), e.stake)
			require.Equal(t, before.Balances[0].Sub(sdk.NewCoins(coin)), after.Balances[0])
			require.Equal(t, before.Balances[1], after.Balances[1])
			require.Equal(t, before.Balances[2].Add(coin), after.Balances[2])
			require.Equal(t, before.Supply, after.Supply)
		})
	}
}

func TestRepairM_1212_HistoricalReplay(t *testing.T) {
	for _, phase := range []string{"unscheduled", "scheduled", "applied"} {
		for _, height := range []int64{repairM1212Height - 1, repairM1212Height, repairM1212Height + 1} {
			for _, trace := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height_%d/tracing_%t", phase, height, trace), func(t *testing.T) {
					e := newRepairM1212Env(t)
					e.app.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(1), "v6.4.0")
					if phase != "unscheduled" {
						require.NoError(t, e.app.UpgradeKeeper.ScheduleUpgrade(e.ctx.WithBlockHeight(2),
							upgradetypes.Plan{Name: pcommon.MergeUpgrade, Height: repairM1212Height}))
					}
					if phase == "applied" {
						e.activate()
					}
					e.ctx = e.ctx.WithBlockHeight(height).WithIsTracing(trace)
					versions := staking.GetVersioned(pcommon.MergeUpgrade, e.app.GetPrecompileKeepers())
					// v65 is an unregistered archive; replay uses the real v6.4.0
					// table entry, while explicit archived calls below prove v65.
					require.NotContains(t, versions, "v6.5.0")
					e.keeper.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{repairM1212Address: versions}, pcommon.MergeUpgrade)
					served := e.keeper.CustomPrecompiles(e.ctx)[repairM1212Address]
					require.NotNil(t, served)
					active := phase == "applied" && height >= repairM1212Height
					wantVersion := "v6.4.0"
					if active {
						wantVersion = pcommon.MergeUpgrade
					}
					require.Equal(t, wantVersion, e.keeper.GetCustomPrecompilesVersions(e.ctx)[repairM1212Address])
					if trace {
						require.Same(t, versions[wantVersion], served)
					}
					evm, db := e.evm(t, map[common.Address]vm.PrecompiledContract{repairM1212Address: served})
					msg := e.message(t, strings.Repeat("ab", 31))
					call := func() {
						_, _, err := evm.Call(msg.From, *msg.To, msg.Data, msg.GasLimit, uint256.MustFromBig(msg.Value))
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
					}
					if active {
						require.NotPanics(t, call)
						require.EqualError(t, db.GetPrecompileError(), "invalid public key length: expected 32 bytes, got 31")
					} else {
						require.PanicsWithValue(t, "pubkey is incorrect size", call)
					}
					legacy, err := stakingv65.NewPrecompile(e.app.GetPrecompileKeepers())
					require.NoError(t, err)
					legacyCtx, _ := e.ctx.CacheContext()
					e.ctx = legacyCtx
					legacyEVM, _ := e.evm(t, map[common.Address]vm.PrecompiledContract{repairM1212Address: legacy})
					require.PanicsWithValue(t, "pubkey is incorrect size", func() {
						_, _, _ = legacyEVM.Call(msg.From, *msg.To, msg.Data, msg.GasLimit, uint256.MustFromBig(msg.Value))
					})
				})
			}
		}
	}
}

// Produce the entry through BeginRedelegation, after a real slash has changed
// the destination's exchange rate. No query response or ABI output is supplied
// by the fixture: both are produced by the real keepers and precompile.
func repair1212RedelegationState(t *testing.T) (*repairM1212Keepers, sdk.Context, *stakingtypes.QueryRedelegationsRequest) {
	t.Helper()
	a, ctx := repairM1212Fixture(t)
	ctx = ctx.WithBlockHeight(20)
	k := a.StakingKeeper
	validators := make([]sdk.ValAddress, 2)
	for i := range validators {
		pubkey := ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair-1212-validator-%d", i))).PubKey()
		address := sdk.ValAddress(pubkey.Address())
		stake := k.TokensFromConsensusPower(ctx, 3)
		funds := sdk.NewCoins(sdk.NewCoin(k.BondDenom(ctx), stake))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sdk.AccAddress(address), funds))
		msg, err := stakingtypes.NewMsgCreateValidator(address, pubkey, funds[0],
			stakingtypes.Description{Moniker: fmt.Sprintf("repair-1212-validator-%d", i)},
			stakingtypes.NewCommissionRates(sdk.MustNewDecFromStr("0.05"), sdk.OneDec(), sdk.OneDec()), sdk.OneInt())
		require.NoError(t, err)
		_, err = stakingkeeper.NewMsgServerImpl(k).CreateValidator(sdk.WrapSDKContext(ctx), msg)
		require.NoError(t, err)
		validator, found := k.GetValidator(ctx, address)
		require.True(t, found)
		// Unbonding validators and newly created validators both use the
		// not-bonded pool; the future completion keeps the redelegation live.
		validator = validator.UpdateStatus(stakingtypes.Unbonding)
		validator.UnbondingTime = ctx.BlockTime().Add(24 * time.Hour)
		validator.UnbondingHeight = ctx.BlockHeight()
		k.SetValidator(ctx, validator)
		validators[i] = address
	}
	dst, found := k.GetValidator(ctx, validators[1])
	require.True(t, found)
	consensusAddress, err := dst.GetConsAddr()
	require.NoError(t, err)
	k.Slash(ctx, consensusAddress, ctx.BlockHeight(), 3, sdk.MustNewDecFromStr("0.4"))
	dst, found = k.GetValidator(ctx, validators[1])
	require.True(t, found)
	require.Equal(t, "0.600000000000000000", dst.TokensFromShares(sdk.OneDec()).String())

	delegator := sdk.AccAddress(validators[0])
	_, err = k.BeginRedelegation(ctx, delegator, validators[0], validators[1], sdk.NewDec(7))
	require.NoError(t, err)
	return a, ctx, &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr: delegator.String(), SrcValidatorAddr: validators[0].String(),
		DstValidatorAddr: validators[1].String(), Pagination: &query.PageRequest{},
	}
}

func repair1212Query(t *testing.T, a *repairM1212Keepers, ctx sdk.Context, req *stakingtypes.QueryRedelegationsRequest) *stakingtypes.QueryRedelegationsResponse {
	t.Helper()
	response, err := (stakingkeeper.Querier{Keeper: a.StakingKeeper}).Redelegations(sdk.WrapSDKContext(ctx), req)
	require.NoError(t, err)
	require.Len(t, response.RedelegationResponses, 1)
	require.Len(t, response.RedelegationResponses[0].Entries, 1)
	return response
}

func repair1212SharesCall(t *testing.T, ctx sdk.Context, k *evmkeeper.Keeper, runner vm.DynamicGasPrecompiledContract, contractABI abi.ABI, req *stakingtypes.QueryRedelegationsRequest) ([]byte, staking.RedelegationsResponse) {
	t.Helper()
	input, err := contractABI.Pack(staking.RedelegationsMethod, req.DelegatorAddr, req.SrcValidatorAddr, req.DstValidatorAddr, req.Pagination.Key)
	require.NoError(t, err)
	require.Equal(t, "7ac3ee49", common.Bytes2Hex(input[:4]), "frozen redelegations selector")
	db := state.NewDBImpl(ctx, k, true)
	t.Cleanup(db.Cleanup)
	evm := &vm.EVM{StateDB: db}
	ret, _, err := runner.RunAndCalculateGas(evm, common.Address{}, common.Address{}, input, 20_000_000, nil, nil, true, false)
	require.NoError(t, err, "redelegations reverted: %x", ret)
	values, err := contractABI.Methods[staking.RedelegationsMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Len(t, values, 1)
	decoded := abi.ConvertType(values[0], new(staking.RedelegationsResponse)).(*staking.RedelegationsResponse)
	require.Len(t, decoded.Redelegations, 1)
	require.Len(t, decoded.Redelegations[0].Entries, 1)
	return ret, *decoded
}

func TestRepairM_1212_RedelegationShares(t *testing.T) {
	a, base, request := repair1212RedelegationState(t)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(repairM1212Height), pcommon.MergeUpgrade)
	precompile, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	for _, sourceIndex := range []bool{false, true} {
		t.Run(fmt.Sprintf("source_index_%t", sourceIndex), func(t *testing.T) {
			req := *request
			if sourceIndex {
				req.DelegatorAddr, req.DstValidatorAddr = "", ""
			}
			ctx, _ := base.WithBlockHeight(repairM1212Height).CacheContext()
			producer := repair1212Query(t, a, ctx, &req)
			entry := producer.RedelegationResponses[0].Entries[0]
			// SharesFromTokens uses Dec.QuoInt, which truncates to 18 places.
			delegator, err := sdk.AccAddressFromBech32(request.DelegatorAddr)
			require.NoError(t, err)
			src, err := sdk.ValAddressFromBech32(request.SrcValidatorAddr)
			require.NoError(t, err)
			dst, err := sdk.ValAddressFromBech32(request.DstValidatorAddr)
			require.NoError(t, err)
			stored, found := a.StakingKeeper.GetRedelegation(ctx, delegator, src, dst)
			require.True(t, found)
			require.Len(t, stored.Entries, 1)
			require.Equal(t, stored.Entries[0], entry.RedelegationEntry)
			require.Equal(t, "11.666666666666666666", entry.RedelegationEntry.SharesDst.String())
			require.NotEqual(t, entry.Balance.ToDec(), entry.RedelegationEntry.SharesDst)
			_, consumer := repair1212SharesCall(t, ctx, a.EvmKeeper, precompile, precompile.GetABI(), &req)
			redel := consumer.Redelegations[0]
			require.Equal(t, request.DelegatorAddr, redel.DelegatorAddress)
			require.Equal(t, request.SrcValidatorAddr, redel.ValidatorSrcAddress)
			require.Equal(t, request.DstValidatorAddr, redel.ValidatorDstAddress)
			require.Equal(t, entry.RedelegationEntry.SharesDst.String(), redel.Entries[0].SharesDst)
			require.NotEqual(t, entry.Balance.String(), redel.Entries[0].SharesDst)
			require.Equal(t, entry.RedelegationEntry.InitialBalance.String(), redel.Entries[0].InitialBalance)
			require.Equal(t, entry.RedelegationEntry.CreationHeight, redel.Entries[0].CreationHeight)
			require.Equal(t, entry.RedelegationEntry.CompletionTime.Unix(), redel.Entries[0].CompletionTime)
			require.True(t, bytes.Equal(producer.Pagination.NextKey, consumer.NextKey), "pagination key must survive ABI encoding")
		})
	}
}

func TestRepairM_1212_SharesUpgradeReplay(t *testing.T) {
	a, base, req := repair1212RedelegationState(t)
	k := a.EvmKeeper
	address := common.HexToAddress(staking.StakingAddress)
	versions := staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers())
	require.NotContains(t, versions, "v6.5.0")
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(1), "v6.4.0")
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(base, pcommon.MergeUpgrade))
	legacy, err := stakingv65.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	current := versions[pcommon.MergeUpgrade].(*pcommon.DynamicGasPrecompile)
	contractABI := current.GetABI()
	producer := repair1212Query(t, a, base, req)
	entry := producer.RedelegationResponses[0].Entries[0]

	for _, phase := range []string{"unscheduled", "scheduled", "applied"} {
		switch phase {
		case "scheduled":
			require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(base, upgradetypes.Plan{
				Name: pcommon.MergeUpgrade, Height: repairM1212Height,
			}))
		case "applied":
			a.UpgradeKeeper.SetDone(base.WithBlockHeight(repairM1212Height), pcommon.MergeUpgrade)
		}
		for _, height := range []int64{1, repairM1212Height - 1, repairM1212Height, repairM1212Height + 1} {
			for _, tracing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height_%d/tracing_%t", phase, height, tracing), func(t *testing.T) {
					ctx, _ := base.WithBlockHeight(height).WithIsTracing(tracing).CacheContext()
					served, found := k.CustomPrecompiles(ctx)[address]
					require.True(t, found)
					runner, ok := served.(vm.DynamicGasPrecompiledContract)
					require.True(t, ok)
					wantVersion := "v6.4.0"
					if phase == "applied" && height >= repairM1212Height {
						wantVersion = pcommon.MergeUpgrade
					}
					require.Equal(t, wantVersion, k.GetCustomPrecompilesVersions(ctx)[address])
					actual, decoded := repair1212SharesCall(t, ctx, k, runner, contractABI, req)
					legacyCtx, _ := ctx.CacheContext()
					historical, historicalDecoded := repair1212SharesCall(t, legacyCtx, k, legacy, contractABI, req)
					require.Equal(t, entry.Balance.String(), historicalDecoded.Redelegations[0].Entries[0].SharesDst,
						"archived implementation must preserve historical bytes even after activation")
					if phase == "applied" && height >= repairM1212Height {
						require.Equal(t, entry.RedelegationEntry.SharesDst.String(), decoded.Redelegations[0].Entries[0].SharesDst)
						expected := historicalDecoded
						expected.Redelegations[0].Entries[0].SharesDst = entry.RedelegationEntry.SharesDst.String()
						expectedBytes, err := contractABI.Methods[staking.RedelegationsMethod].Outputs.Pack(expected)
						require.NoError(t, err)
						require.Equal(t, expectedBytes, actual, "only the sharesDst projection changes")
					} else {
						require.Equal(t, entry.Balance.String(), decoded.Redelegations[0].Entries[0].SharesDst)
						require.Equal(t, historical, actual, "pre-activation replay must be byte identical")
					}
				})
			}
		}
	}
}

func TestRepairM_1212_ABICompatibility(t *testing.T) {
	a, _ := repairM1212Fixture(t)
	current, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	historical, err := stakingv65.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, historical.Address(), current.Address())
	for version, registered := range staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers()) {
		require.NotEqual(t, reflect.TypeOf(historical), reflect.TypeOf(registered), "v65 must remain unregistered: %s", version)
	}
	currentABI, historicalABI := current.GetABI(), historical.GetABI()
	require.Len(t, currentABI.Methods, len(historicalABI.Methods))
	for name, method := range historicalABI.Methods {
		active, found := currentABI.Methods[name]
		require.True(t, found, name)
		require.Equal(t, method.Sig, active.Sig)
		require.Equal(t, method.ID, active.ID)
		require.Equal(t, method.Inputs, active.Inputs)
		require.Equal(t, method.Outputs, active.Outputs)
	}
	require.Len(t, currentABI.Events, len(historicalABI.Events))
	for name, event := range historicalABI.Events {
		active, found := currentABI.Events[name]
		require.True(t, found, name)
		require.Equal(t, event.Sig, active.Sig)
		require.Equal(t, event.ID, active.ID)
		require.Equal(t, event.Inputs, active.Inputs)
	}
	// Consumers keep the same tuple schema on both sides of the semantic change.
	require.Equal(t, historicalABI.Methods[staking.RedelegationsMethod].Outputs,
		currentABI.Methods[staking.RedelegationsMethod].Outputs)
}
