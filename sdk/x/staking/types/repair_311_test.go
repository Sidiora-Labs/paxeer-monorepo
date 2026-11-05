package types_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"path/filepath"
	"sort"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmstate "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	stakingprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/client/cli"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair311App struct {
	app  *paxapp.App
	ctx  sdk.Context
	dir  string
	stop func()
}

func repair311Open(t *testing.T, dir string, initialize bool) repair311App {
	t.Helper()
	db, err := dbm.NewGoLevelDB("staking-repair", dir)
	require.NoError(t, err)
	encoding := paxapp.MakeEncodingConfig()
	app := paxapp.New(db, nil, true, map[int64]bool{}, filepath.Join(dir, "node"), 1, false,
		config.TestConfig(), encoding, wasm.EnableAllProposals, paxapp.TestAppOpts{}, paxapp.EmptyWasmOpts, nil)
	closed := false
	stop := func() {
		if !closed {
			require.NoError(t, app.Close())
			closed = true
		}
	}
	t.Cleanup(stop)
	header := tmproto.Header{ChainID: "pax-test", Height: 2, Time: time.Unix(1700000000, 0).UTC()}
	if initialize {
		genesis, err := json.Marshal(paxapp.NewDefaultGenesisState(encoding.Marshaler))
		require.NoError(t, err)
		_, err = app.InitChain(context.Background(), &abci.RequestInitChain{
			Time: header.Time, ChainId: header.ChainID, ConsensusParams: paxapp.DefaultConsensusParams, AppStateBytes: genesis,
		})
		require.NoError(t, err)
		app.NewContext(false, header).MultiStore().(sdk.CacheMultiStore).Write()
		require.Equal(t, int64(1), app.CommitMultiStore().Commit(true).Version)
	}
	ctx := sdk.NewContext(app.CommitMultiStore().CacheMultiStore(), header, false)
	return repair311App{app: app, ctx: ctx, dir: dir, stop: stop}
}

func (h repair311App) reopen(t *testing.T) repair311App {
	t.Helper()
	h.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	committed := h.app.CommitMultiStore().Commit(true)
	require.Positive(t, committed.Version)
	h.stop()
	reopened := repair311Open(t, h.dir, false)
	require.Equal(t, committed, reopened.app.CommitMultiStore().LastCommitID())
	return reopened
}

func repair311Address(b byte) sdk.AccAddress {
	return sdk.AccAddress(bytes.Repeat([]byte{b}, 20))
}

func repair311Fund(t *testing.T, h repair311App, addr sdk.AccAddress, amount sdk.Int) {
	t.Helper()
	require.NoError(t, apptesting.FundAccount(h.app.BankKeeper, h.ctx, addr,
		sdk.NewCoins(sdk.NewCoin(h.app.StakingKeeper.BondDenom(h.ctx), amount))))
}

func repair311Validator(t *testing.T, h repair311App, addr sdk.AccAddress, power int64) stakingtypes.Validator {
	t.Helper()
	return repair311ValidatorWithKey(t, h, addr, power, ed25519.GenPrivKey().PubKey())
}

func repair311ValidatorWithKey(t *testing.T, h repair311App, addr sdk.AccAddress, power int64, pubkey cryptotypes.PubKey) stakingtypes.Validator {
	t.Helper()
	amount := h.app.StakingKeeper.PowerReduction(h.ctx).MulRaw(power)
	repair311Fund(t, h, addr, amount)
	msg, err := stakingtypes.NewMsgCreateValidator(sdk.ValAddress(addr), pubkey,
		sdk.NewCoin(h.app.StakingKeeper.BondDenom(h.ctx), amount), stakingtypes.Description{Moniker: "repair-311-validator"},
		stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.OneDec(), sdk.OneDec()), sdk.OneInt())
	require.NoError(t, err)
	require.NoError(t, msg.ValidateBasic())
	_, err = stakingkeeper.NewMsgServerImpl(h.app.StakingKeeper).CreateValidator(sdk.WrapSDKContext(h.ctx), msg)
	require.NoError(t, err)
	validator, found := h.app.StakingKeeper.GetValidator(h.ctx, sdk.ValAddress(addr))
	require.True(t, found)
	return validator
}

func repair311CLIGrant(t *testing.T, h repair311App, granter, grantee sdk.AccAddress, policy, validator string) {
	t.Helper()
	encoding := paxapp.MakeEncodingConfig()
	clientCtx := client.Context{}.WithCodec(encoding.Marshaler).WithInterfaceRegistry(encoding.InterfaceRegistry).
		WithTxConfig(encoding.TxConfig).WithLegacyAmino(encoding.Amino).
		WithFrom(granter.String()).WithFromAddress(granter).WithChainID("pax-test").WithGenerateOnly(true)
	out, err := clitestutil.ExecTestCLICmd(clientCtx, authzcli.NewCmdGrantAuthorization(), []string{
		grantee.String(), "delegate", "--" + policy + "=" + validator,
		"--" + authzcli.FlagSpendLimit + "=100" + h.app.StakingKeeper.BondDenom(h.ctx),
		fmt.Sprintf("--%s=%d", authzcli.FlagExpiration, h.ctx.BlockTime().Add(time.Hour).Unix()),
		"--" + flags.FlagGenerateOnly,
	})
	require.NoError(t, err, out.String())
	tx, err := encoding.TxConfig.TxJSONDecoder()(out.Bytes())
	require.NoError(t, err, out.String())
	require.Len(t, tx.GetMsgs(), 1)
	grant, ok := tx.GetMsgs()[0].(*authz.MsgGrant)
	require.True(t, ok)
	require.NoError(t, grant.ValidateBasic())
	_, err = h.app.AuthzKeeper.Grant(sdk.WrapSDKContext(h.ctx), grant)
	require.NoError(t, err)
}

func repair311Exec(t *testing.T, h repair311App, grantee sdk.AccAddress, msg sdk.Msg) error {
	t.Helper()
	exec := authz.NewMsgExec(grantee, []sdk.Msg{msg})
	// Decode the transaction envelope so MsgExec routes unpacked Any messages.
	bz := h.app.AppCodec().MustMarshal(&exec)
	var decoded authz.MsgExec
	require.NoError(t, h.app.AppCodec().Unmarshal(bz, &decoded))
	cacheCtx, write := h.ctx.CacheContext()
	_, err := h.app.AuthzKeeper.Exec(sdk.WrapSDKContext(cacheCtx), &decoded)
	if err == nil {
		write()
	}
	return err
}

func TestRepair_311_DenyListGrants(t *testing.T) {
	for _, policy := range []string{authzcli.FlagDenyValidators, authzcli.FlagAllowedValidators} {
		t.Run(policy, func(t *testing.T) {
			h := repair311Open(t, t.TempDir(), true)
			granter, grantee := repair311Address(1), repair311Address(2)
			excluded := repair311Validator(t, h, repair311Address(3), 10)
			permitted := repair311Validator(t, h, repair311Address(4), 10)
			repair311Fund(t, h, granter, sdk.NewInt(200))
			listed := excluded.GetOperator().String()
			if policy == authzcli.FlagAllowedValidators {
				listed = permitted.GetOperator().String()
			}
			repair311CLIGrant(t, h, granter, grantee, policy, listed)
			h = h.reopen(t)

			grants, err := h.app.AuthzKeeper.Grants(sdk.WrapSDKContext(h.ctx), &authz.QueryGrantsRequest{
				Granter: granter.String(), Grantee: grantee.String(), MsgTypeUrl: sdk.MsgTypeURL(&stakingtypes.MsgDelegate{}),
			})
			require.NoError(t, err)
			require.Len(t, grants.Grants, 1)
			var decoded authz.Authorization
			require.NoError(t, h.app.AppCodec().UnpackAny(grants.Grants[0].Authorization, &decoded))
			stake, ok := decoded.(*stakingtypes.StakeAuthorization)
			require.True(t, ok)
			if policy == authzcli.FlagDenyValidators {
				require.Nil(t, stake.GetAllowList())
				require.Equal(t, []string{listed}, stake.GetDenyList().Address)
			} else {
				require.Nil(t, stake.GetDenyList())
				require.Equal(t, []string{listed}, stake.GetAllowList().Address)
			}
			require.Equal(t, sdk.NewInt(100), stake.MaxTokens.Amount)
			denom := h.app.StakingKeeper.BondDenom(h.ctx)
			before := h.app.BankKeeper.GetBalance(h.ctx, granter, denom)
			err = repair311Exec(t, h, grantee, stakingtypes.NewMsgDelegate(granter, excluded.GetOperator(), sdk.NewInt64Coin(denom, 1)))
			require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
			_, found := h.app.StakingKeeper.GetDelegation(h.ctx, granter, excluded.GetOperator())
			require.False(t, found)
			require.Equal(t, before, h.app.BankKeeper.GetBalance(h.ctx, granter, denom))
			require.ErrorIs(t, repair311Exec(t, h, grantee, stakingtypes.NewMsgUndelegate(granter, permitted.GetOperator(), sdk.NewInt64Coin(denom, 1))), sdkerrors.ErrUnauthorized)

			require.NoError(t, repair311Exec(t, h, grantee, stakingtypes.NewMsgDelegate(granter, permitted.GetOperator(), sdk.NewInt64Coin(denom, 60))))
			delegation, found := h.app.StakingKeeper.GetDelegation(h.ctx, granter, permitted.GetOperator())
			require.True(t, found)
			require.True(t, sdk.NewDec(60).Equal(delegation.Shares))
			saved, _ := h.app.AuthzKeeper.GetCleanAuthorization(h.ctx, grantee, granter, sdk.MsgTypeURL(&stakingtypes.MsgDelegate{}))
			require.Equal(t, sdk.NewInt(40), saved.(*stakingtypes.StakeAuthorization).MaxTokens.Amount)
			h = h.reopen(t)
			require.NoError(t, repair311Exec(t, h, grantee, stakingtypes.NewMsgDelegate(granter, permitted.GetOperator(), sdk.NewInt64Coin(denom, 40))))
			saved, _ = h.app.AuthzKeeper.GetCleanAuthorization(h.ctx, grantee, granter, sdk.MsgTypeURL(&stakingtypes.MsgDelegate{}))
			require.Nil(t, saved, "exhausting the token limit must remove the grant")
			require.ErrorIs(t, repair311Exec(t, h, grantee, stakingtypes.NewMsgDelegate(granter, permitted.GetOperator(), sdk.NewInt64Coin(denom, 1))), sdkerrors.ErrUnauthorized)
			require.Equal(t, sdk.NewInt(100), h.app.BankKeeper.GetBalance(h.ctx, granter, denom).Amount)
		})
	}
}

func TestRepair_311_PrecompileShareUnits(t *testing.T) {
	h := repair311Open(t, t.TempDir(), true)
	source := repair311Validator(t, h, repair311Address(11), 100)
	destination := repair311Validator(t, h, repair311Address(12), 100)
	_, err := h.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(h.ctx)
	require.NoError(t, err)
	destination, found := h.app.StakingKeeper.GetValidator(h.ctx, destination.GetOperator())
	require.True(t, found)
	require.Equal(t, stakingtypes.Bonded, destination.Status)
	consAddr, err := destination.GetConsAddr()
	require.NoError(t, err)
	reduction := h.app.StakingKeeper.PowerReduction(h.ctx)
	h.app.StakingKeeper.Slash(h.ctx, consAddr, h.ctx.BlockHeight(), destination.ConsensusPower(reduction), sdk.NewDecWithPrec(5, 1))
	destination, found = h.app.StakingKeeper.GetValidator(h.ctx, destination.GetOperator())
	require.True(t, found)
	require.True(t, destination.DelegatorShares.Equal(destination.Tokens.ToDec().MulInt64(2)))

	granter := sdk.AccAddress(source.GetOperator())
	amount := reduction.MulRaw(10)
	_, err = stakingkeeper.NewMsgServerImpl(h.app.StakingKeeper).BeginRedelegate(sdk.WrapSDKContext(h.ctx),
		stakingtypes.NewMsgBeginRedelegate(granter, source.GetOperator(), destination.GetOperator(), sdk.NewCoin(h.app.StakingKeeper.BondDenom(h.ctx), amount)))
	require.NoError(t, err)
	h = h.reopen(t)
	querier := stakingkeeper.Querier{Keeper: h.app.StakingKeeper}
	request := &stakingtypes.QueryRedelegationsRequest{DelegatorAddr: granter.String(), SrcValidatorAddr: source.OperatorAddress, DstValidatorAddr: destination.OperatorAddress}
	response, err := querier.Redelegations(sdk.WrapSDKContext(h.ctx), request)
	require.NoError(t, err)
	require.Len(t, response.RedelegationResponses, 1)
	require.Len(t, response.RedelegationResponses[0].Entries, 1)
	entry := response.RedelegationResponses[0].Entries[0]
	require.True(t, amount.Equal(entry.Balance))
	require.True(t, amount.ToDec().MulInt64(2).Equal(entry.RedelegationEntry.SharesDst))
	require.False(t, entry.Balance.ToDec().Equal(entry.RedelegationEntry.SharesDst))

	precompile, err := stakingprecompile.NewPrecompile(&paxapp.PrecompileKeepers{
		StakingKeeper: stakingkeeper.NewMsgServerImpl(h.app.StakingKeeper), StakingQuerier: querier, EVMKeeper: &h.app.EvmKeeper,
		BankKeeper: h.app.BankKeeper, DistributionKeeper: h.app.DistrKeeper,
	})
	require.NoError(t, err)
	method := precompile.ABI.Methods["redelegations"]
	input, err := precompile.ABI.Pack("redelegations", granter.String(), source.OperatorAddress, destination.OperatorAddress, []byte{})
	require.NoError(t, err)
	evm := &vm.EVM{StateDB: evmstate.NewDBImpl(h.ctx, &h.app.EvmKeeper, true)}
	output, _, err := precompile.RunAndCalculateGas(evm, common.Address{}, common.Address{}, input, math.MaxUint64, big.NewInt(0), nil, true, false)
	require.NoError(t, err)
	values, err := method.Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, values, 1)
	decoded := abi.ConvertType(values[0], new(stakingprecompile.RedelegationsResponse)).(*stakingprecompile.RedelegationsResponse)
	require.Len(t, decoded.Redelegations, 1)
	require.Len(t, decoded.Redelegations[0].Entries, 1)
	abiEntry := decoded.Redelegations[0].Entries[0]
	require.Equal(t, entry.RedelegationEntry.SharesDst.String(), abiEntry.SharesDst)
	require.NotEqual(t, entry.Balance.String(), abiEntry.SharesDst)
	require.Equal(t, entry.RedelegationEntry.InitialBalance.String(), abiEntry.InitialBalance)
	require.Equal(t, entry.RedelegationEntry.CreationHeight, abiEntry.CreationHeight)
	require.Equal(t, entry.RedelegationEntry.CompletionTime.Unix(), abiEntry.CompletionTime)
}

func TestRepair_311_HistoricalSortContract(t *testing.T) {
	h := repair311Open(t, t.TempDir(), true)
	lowKey, highKey := ed25519.GenPrivKey().PubKey(), ed25519.GenPrivKey().PubKey()
	if bytes.Compare(lowKey.Address(), highKey.Address()) > 0 {
		lowKey, highKey = highKey, lowKey
	}
	validators := []stakingtypes.Validator{
		repair311Validator(t, h, repair311Address(21), 1),
		repair311Validator(t, h, repair311Address(22), 7),
		repair311ValidatorWithKey(t, h, repair311Address(23), 4, highKey),
		repair311ValidatorWithKey(t, h, repair311Address(24), 4, lowKey),
	}
	_, err := h.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(h.ctx)
	require.NoError(t, err)
	for i := range validators {
		var found bool
		validators[i], found = h.app.StakingKeeper.GetValidator(h.ctx, validators[i].GetOperator())
		require.True(t, found)
		require.Equal(t, stakingtypes.Bonded, validators[i].Status)
	}
	reduction := h.app.StakingKeeper.PowerReduction(h.ctx)
	reductions := []sdk.Int{reduction, reduction.MulRaw(10)}
	for i, r := range reductions {
		height := int64(10 + i)
		header := h.ctx.BlockHeader()
		header.Height = height
		produced := stakingtypes.NewHistoricalInfo(header, append(stakingtypes.Validators(nil), validators...), r)
		h.app.StakingKeeper.SetHistoricalInfo(h.ctx, height, &produced)
	}
	h = h.reopen(t)
	for i, r := range reductions {
		height := int64(10 + i)
		recovered, found := h.app.StakingKeeper.GetHistoricalInfo(h.ctx, height)
		require.True(t, found)
		require.Equal(t, height, recovered.Header.Height)
		require.Len(t, recovered.Valset, 4)
		require.NoError(t, stakingtypes.ValidateBasic(recovered, r))
		if i == 0 {
			require.Equal(t, validators[1].OperatorAddress, recovered.Valset[0].OperatorAddress)
			require.Equal(t, validators[0].OperatorAddress, recovered.Valset[3].OperatorAddress)
			require.False(t, sort.IsSorted(stakingtypes.Validators(recovered.Valset)))
			left, err := recovered.Valset[1].GetConsAddr()
			require.NoError(t, err)
			right, err := recovered.Valset[2].GetConsAddr()
			require.NoError(t, err)
			require.Negative(t, bytes.Compare(left, right))
			require.Positive(t, bytes.Compare(recovered.Valset[1].GetOperator(), recovered.Valset[2].GetOperator()))
		}
		for j := 1; j < len(recovered.Valset); j++ {
			previous, current := recovered.Valset[j-1], recovered.Valset[j]
			require.GreaterOrEqual(t, previous.ConsensusPower(r), current.ConsensusPower(r))
			if previous.ConsensusPower(r) == current.ConsensusPower(r) {
				pa, err := previous.GetConsAddr()
				require.NoError(t, err)
				ca, err := current.GetConsAddr()
				require.NoError(t, err)
				require.Negative(t, bytes.Compare(pa, ca))
			}
		}
		reversed := recovered
		reversed.Valset = append([]stakingtypes.Validator(nil), recovered.Valset...)
		reversed.Valset[0], reversed.Valset[len(reversed.Valset)-1] = reversed.Valset[len(reversed.Valset)-1], reversed.Valset[0]
		require.ErrorIs(t, stakingtypes.ValidateBasic(reversed, r), stakingtypes.ErrInvalidHistoricalInfo)
	}
	require.ErrorIs(t, stakingtypes.ValidateBasic(stakingtypes.HistoricalInfo{}, reduction), stakingtypes.ErrInvalidHistoricalInfo)
}
