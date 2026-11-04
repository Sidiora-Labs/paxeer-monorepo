package types_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM688Keepers struct {
	ctx      sdk.Context
	cdc      *codec.ProtoCodec
	cms      storetypes.CommitMultiStore
	staking  stakingkeeper.Keeper
	authz    authzkeeper.Keeper
	bank     bankkeeper.BaseKeeper
	accounts authkeeper.AccountKeeper
	stop     func()
}

func repairM688Open(t *testing.T, dir string, initialize bool) repairM688Keepers {
	t.Helper()
	db, err := dbm.NewGoLevelDB("repair-m688", dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys("accounts", "bank", "staking", "authz", "params")
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	transient := sdk.NewTransientStoreKey("transient_params")
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "repair-m688", Height: 10, Time: time.Unix(1700000000, 0).UTC()}, false)
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	authz.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	params := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), keys["params"], transient)
	accounts := authkeeper.NewAccountKeeper(cdc, keys["accounts"], params.Subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{types.BondedPoolName: {authtypes.Burner, authtypes.Staking}, types.NotBondedPoolName: {authtypes.Burner, authtypes.Staking}, "faucet": {authtypes.Minter}})
	bank := bankkeeper.NewBaseKeeper(cdc, keys["bank"], accounts, params.Subspace("bank"), map[string]bool{})
	staking := stakingkeeper.NewKeeper(cdc, keys["staking"], accounts, bank, params.Subspace("staking"))
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterMsgServer(router, stakingkeeper.NewMsgServerImpl(staking))
	authorizations := authzkeeper.NewKeeper(keys["authz"], cdc, router)
	if initialize {
		accounts.SetParams(ctx, authtypes.DefaultParams())
		bank.SetParams(ctx, banktypes.DefaultParams())
		staking.SetParams(ctx, types.DefaultParams())
		for _, name := range []string{types.BondedPoolName, types.NotBondedPoolName, "faucet"} {
			require.NotNil(t, accounts.GetModuleAccount(ctx, name))
		}
	}
	closed := false
	stop := func() {
		if !closed {
			require.NoError(t, cms.Close())
			closed = true
		}
	}
	t.Cleanup(stop)
	return repairM688Keepers{ctx: ctx, cdc: cdc, cms: cms, staking: staking, authz: authorizations, bank: bank, accounts: accounts, stop: stop}
}

func repairM688Exec(t *testing.T, h repairM688Keepers, grantee sdk.AccAddress, msg sdk.Msg) error {
	t.Helper()
	exec := authz.NewMsgExec(grantee, []sdk.Msg{msg})
	var decoded authz.MsgExec
	require.NoError(t, h.cdc.Unmarshal(h.cdc.MustMarshal(&exec), &decoded))
	_, err := h.authz.Exec(sdk.WrapSDKContext(h.ctx), &decoded)
	return err
}

func TestRepairM_688_StakeLimitPanics(t *testing.T) {
	h := repairM688Open(t, t.TempDir(), true)
	granter := sdk.AccAddress(bytes.Repeat([]byte{11}, 20))
	grantee := sdk.AccAddress(bytes.Repeat([]byte{12}, 20))
	validator := createValidators(t)[0]
	h.staking.SetValidator(h.ctx, validator)
	h.accounts.SetAccount(h.ctx, h.accounts.NewAccountWithAddress(h.ctx, granter))
	h.accounts.SetAccount(h.ctx, h.accounts.NewAccountWithAddress(h.ctx, grantee))
	denom := h.staking.BondDenom(h.ctx)
	funds := sdk.NewCoins(sdk.NewInt64Coin(denom, 500), sdk.NewInt64Coin("other", 500))
	require.NoError(t, h.bank.MintCoins(h.ctx, "faucet", funds))
	require.NoError(t, h.bank.SendCoinsFromModuleToAccount(h.ctx, "faucet", granter, funds))
	operations := []struct {
		name    string
		kind    types.AuthorizationType
		message func(sdk.Coin) sdk.Msg
	}{
		{"delegate", types.AuthorizationType_AUTHORIZATION_TYPE_DELEGATE, func(c sdk.Coin) sdk.Msg { return types.NewMsgDelegate(granter, validator.GetOperator(), c) }},
		{"undelegate", types.AuthorizationType_AUTHORIZATION_TYPE_UNDELEGATE, func(c sdk.Coin) sdk.Msg { return types.NewMsgUndelegate(granter, validator.GetOperator(), c) }},
		{"redelegate", types.AuthorizationType_AUTHORIZATION_TYPE_REDELEGATE, func(c sdk.Coin) sdk.Msg {
			return types.NewMsgBeginRedelegate(granter, sdk.ValAddress(grantee), validator.GetOperator(), c)
		}},
	}
	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			limit := sdk.NewInt64Coin(denom, 100)
			authorization, err := types.NewStakeAuthorization([]sdk.ValAddress{validator.GetOperator()}, nil, op.kind, &limit)
			require.NoError(t, err)
			grant, err := authz.NewMsgGrant(granter, grantee, authorization, h.ctx.BlockTime().Add(time.Hour))
			require.NoError(t, err)
			var decoded authz.MsgGrant
			require.NoError(t, h.cdc.Unmarshal(h.cdc.MustMarshal(grant), &decoded))
			_, err = h.authz.Grant(sdk.WrapSDKContext(h.ctx), &decoded)
			require.NoError(t, err)
			savedBefore := h.cdc.MustMarshal(h.authz.ExportGenesis(h.ctx))
			balancesBefore := h.bank.GetAccountsBalances(h.ctx)
			validatorBefore, found := h.staking.GetValidator(h.ctx, validator.GetOperator())
			require.True(t, found)
			for _, tc := range []struct {
				name   string
				amount sdk.Coin
				want   error
			}{
				{"overspend", sdk.NewInt64Coin(denom, 101), sdkerrors.ErrInsufficientFunds},
				{"wrong-denom", sdk.NewInt64Coin("other", 1), sdkerrors.ErrUnauthorized},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var execErr error
					require.NotPanics(t, func() { execErr = repairM688Exec(t, h, grantee, op.message(tc.amount)) })
					require.ErrorIs(t, execErr, tc.want)
					require.Equal(t, savedBefore, h.cdc.MustMarshal(h.authz.ExportGenesis(h.ctx)))
					require.Equal(t, balancesBefore, h.bank.GetAccountsBalances(h.ctx))
					stored, _ := h.authz.GetCleanAuthorization(h.ctx, grantee, granter, authorization.MsgTypeURL())
					require.Equal(t, &limit, stored.(*types.StakeAuthorization).MaxTokens)
					after, found := h.staking.GetValidator(h.ctx, validator.GetOperator())
					require.True(t, found)
					require.True(t, validatorBefore.Equal(&after))
					_, found = h.staking.GetDelegation(h.ctx, granter, validator.GetOperator())
					require.False(t, found)
				})
			}
		})
	}
	// Successful execution proves the same real handler moves funds and persists the reduced allowance.
	require.NoError(t, repairM688Exec(t, h, grantee, types.NewMsgDelegate(granter, validator.GetOperator(), sdk.NewInt64Coin(denom, 40))))
	stored, _ := h.authz.GetCleanAuthorization(h.ctx, grantee, granter, sdk.MsgTypeURL(&types.MsgDelegate{}))
	require.Equal(t, sdk.NewInt(60), stored.(*types.StakeAuthorization).MaxTokens.Amount)
	require.Equal(t, sdk.NewInt(460), h.bank.GetBalance(h.ctx, granter, denom).Amount)
	require.NoError(t, repairM688Exec(t, h, grantee, types.NewMsgDelegate(granter, validator.GetOperator(), sdk.NewInt64Coin(denom, 60))))
	stored, _ = h.authz.GetCleanAuthorization(h.ctx, grantee, granter, sdk.MsgTypeURL(&types.MsgDelegate{}))
	require.Nil(t, stored)
	require.Equal(t, sdk.NewInt(400), h.bank.GetBalance(h.ctx, granter, denom).Amount)
	require.Equal(t, sdk.NewInt(100), h.bank.GetBalance(h.ctx, h.accounts.GetModuleAddress(types.BondedPoolName), denom).Amount)
	delegation, found := h.staking.GetDelegation(h.ctx, granter, validator.GetOperator())
	require.True(t, found)
	require.True(t, sdk.NewDec(100).Equal(delegation.Shares))
}

func TestRepairM_688_InvalidStakeCoins(t *testing.T) {
	h := repairM688Open(t, t.TempDir(), true)
	validator := createValidators(t)[0].GetOperator()
	limit := sdk.NewInt64Coin("stake", 100)
	for _, coin := range []sdk.Coin{{Denom: "stake"}, {Denom: "stake", Amount: sdk.NewInt(-1)}, {Denom: "!", Amount: sdk.OneInt()}} {
		authorization, err := types.NewStakeAuthorization([]sdk.ValAddress{validator}, nil, types.AuthorizationType_AUTHORIZATION_TYPE_DELEGATE, &coin)
		require.NoError(t, err)
		require.NotPanics(t, func() { require.ErrorIs(t, authorization.ValidateBasic(), sdkerrors.ErrInvalidCoins) })
		require.NotPanics(t, func() {
			_, err = authorization.Accept(h.ctx, types.NewMsgDelegate(sdk.AccAddress(validator), validator, limit))
			require.ErrorIs(t, err, sdkerrors.ErrInvalidCoins)
		})
		for _, allowance := range []*sdk.Coin{nil, &limit} {
			authorization, err = types.NewStakeAuthorization([]sdk.ValAddress{validator}, nil, types.AuthorizationType_AUTHORIZATION_TYPE_DELEGATE, allowance)
			require.NoError(t, err)
			require.NotPanics(t, func() {
				response, acceptErr := authorization.Accept(h.ctx, types.NewMsgDelegate(sdk.AccAddress(validator), validator, coin))
				require.ErrorIs(t, acceptErr, sdkerrors.ErrInvalidCoins)
				require.False(t, response.Accept)
				require.Nil(t, response.Updated)
			})
		}
	}
}

func TestRepairM_688_HistoricalSortFixture(t *testing.T) {
	TestHistoricalInfo(t)
	TestValidateBasic(t)
	h := repairM688Open(t, t.TempDir(), true)
	validators := createValidators(t)
	for _, validator := range validators {
		h.staking.SetValidator(h.ctx, validator)
		h.staking.SetLastValidatorPower(h.ctx, validator.GetOperator(), validator.ConsensusPower(sdk.DefaultPowerReduction))
	}
	h.staking.TrackHistoricalInfo(h.ctx)
	historical, found := h.staking.GetHistoricalInfo(h.ctx, h.ctx.BlockHeight())
	require.True(t, found)
	repairM688AssertHistoricalOrder(t, historical)
	require.NoError(t, types.ValidateBasic(historical, sdk.DefaultPowerReduction))
}

func repairM688MalformedQueueKeys(valid []byte) map[string][]byte {
	cases := map[string][]byte{}
	for n := 0; n < 8; n++ {
		cases[fmt.Sprintf("height-%d-bytes", n)] = append([]byte(nil), valid[:len(valid)-8+n]...)
	}
	cases["trailing"] = append(append([]byte(nil), valid...), 0)
	for _, length := range []uint64{0, 1, math.MaxUint64} {
		key := append([]byte(nil), valid...)
		binary.BigEndian.PutUint64(key[len(types.ValidatorQueueKey):], length)
		cases[fmt.Sprintf("time-length-%d", length)] = key
	}
	badTime := append([]byte(nil), valid...)
	badTime[len(types.ValidatorQueueKey)+8] = '!'
	cases["invalid-time"] = badTime
	noncanonical := append([]byte(nil), valid...)
	noncanonical[len(types.ValidatorQueueKey)+8+19] = ','
	cases["noncanonical-time"] = noncanonical
	badPrefix := append([]byte(nil), valid...)
	badPrefix[0] ^= 0xff
	cases["invalid-prefix"] = badPrefix
	badHeight := append([]byte(nil), valid...)
	binary.BigEndian.PutUint64(badHeight[len(badHeight)-8:], math.MaxUint64)
	cases["height-overflow"] = badHeight
	for n := 0; n < len(types.ValidatorQueueKey)+8; n++ {
		cases[fmt.Sprintf("header-%d-bytes", n)] = append([]byte(nil), valid[:n]...)
	}
	return cases
}

func repairM688AssertQueueRejected(t *testing.T, key []byte) {
	t.Helper()
	require.NotPanics(t, func() {
		ts, height, err := types.ParseValidatorQueueKey(key)
		require.Error(t, err)
		require.True(t, ts.IsZero())
		require.Zero(t, height)
	})
}

func TestRepairM_688_ValidatorQueueKeyBounds(t *testing.T) {
	dir := t.TempDir()
	h := repairM688Open(t, dir, true)
	ts, height := h.ctx.BlockTime(), int64(1024)
	operator := createValidators(t)[0].OperatorAddress
	h.staking.SetUnbondingValidatorsQueue(h.ctx, ts, height, []string{operator})
	valid := types.GetValidatorQueueKey(ts, height)
	value := h.ctx.KVStore(h.staking.GetStoreKey()).Get(valid)
	cases := repairM688MalformedQueueKeys(valid)
	// Persist corrupted height suffixes alongside the producer's valid key, then close and reopen the store.
	for name, key := range cases {
		if len(key) > 0 {
			h.ctx.KVStore(h.staking.GetStoreKey()).Set(key, value)
		}
		t.Run(name, func(t *testing.T) { repairM688AssertQueueRejected(t, key) })
	}
	committed := h.cms.Commit(true)
	h.stop()
	reopened := repairM688Open(t, dir, false)
	require.Equal(t, committed, reopened.cms.LastCommitID())
	require.Equal(t, []string{operator}, reopened.staking.GetUnbondingValidators(reopened.ctx, ts, height))
	stored := reopened.ctx.KVStore(reopened.staking.GetStoreKey())
	for name, key := range cases {
		if len(key) > 0 {
			require.Equal(t, value, stored.Get(key))
			t.Run("recovered-"+name, func(t *testing.T) { repairM688AssertQueueRejected(t, key) })
		}
	}
	for _, height := range []int64{0, 1024, math.MaxInt64} {
		parsedTime, parsedHeight, err := types.ParseValidatorQueueKey(types.GetValidatorQueueKey(ts, height))
		require.NoError(t, err)
		require.Equal(t, ts, parsedTime)
		require.Equal(t, height, parsedHeight)
	}
}

func TestRepairM_688_NilMinimumCommission(t *testing.T) {
	defaults := types.DefaultParams()
	for _, tc := range []struct {
		name  string
		rate  sdk.Dec
		valid bool
	}{
		{"uninitialized", sdk.Dec{}, false}, {"negative", sdk.NewDecWithPrec(-1, 18), false}, {"above-one", sdk.OneDec().Add(sdk.NewDecWithPrec(1, 18)), false},
		{"zero", sdk.ZeroDec(), true}, {"one", sdk.OneDec(), true}, {"default", defaults.MinCommissionRate, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := types.NewParams(defaults.UnbondingTime, defaults.MaxValidators, defaults.MaxEntries, defaults.HistoricalEntries, defaults.BondDenom, tc.rate, defaults.MaxVotingPowerRatio, defaults.MaxVotingPowerEnforcementThreshold)
			require.NotPanics(t, func() {
				err := params.Validate()
				if tc.valid {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
			checked := false
			for _, pair := range params.ParamSetPairs() {
				if bytes.Equal(pair.Key, types.KeyMinCommissionRate) {
					checked = true
					require.NotPanics(t, func() {
						err := pair.ValidatorFn(tc.rate)
						if tc.valid {
							require.NoError(t, err)
						} else {
							require.Error(t, err)
						}
					})
				}
			}
			require.True(t, checked, "minimum commission parameter validator must be registered")
		})
	}
}
