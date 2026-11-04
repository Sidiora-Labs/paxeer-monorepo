package simulation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	cryptoproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	govkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	govsimulation "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/simulation"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/simulation"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const repair684Chain = "repair684"

var repair684Time = time.Unix(1700000000, 0).UTC()

type repair684Fixture struct {
	app      *baseapp.BaseApp
	cdc      *codec.ProtoCodec
	keys     map[string]*sdk.KVStoreKey
	auth     authkeeper.AccountKeeper
	bank     bankkeeper.BaseKeeper
	gov      govkeeper.Keeper
	params   paramskeeper.Keeper
	accounts []simtypes.Account
}

func newRepair684Fixture(t *testing.T) *repair684Fixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	govtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	proposal.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	gen := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	app := baseapp.NewBaseApp(repair684Chain, dbm.NewMemDB(), gen.TxDecoder(), nil, &testutil.TestAppOpts{})
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	keys := sdk.NewKVStoreKeys(govtypes.StoreKey, authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		app.MountStores(key)
	}
	transient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	app.MountStores(transient)
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterLegacyAminoCodec(amino)
	govtypes.RegisterLegacyAminoCodec(amino)
	stakingtypes.RegisterLegacyAminoCodec(amino)
	proposal.RegisterLegacyAminoCodec(amino)
	pk := paramskeeper.NewKeeper(cdc, amino, keys[paramtypes.StoreKey], transient)
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		govtypes.ModuleName:            {authtypes.Burner},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	})
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, pk.Subspace(stakingtypes.ModuleName))
	gk := govkeeper.NewKeeper(cdc, keys[govtypes.StoreKey], pk.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable()), ak, bk, sk, pk,
		govtypes.NewRouter().AddRoute(govtypes.RouterKey, govtypes.ProposalHandler).AddRoute(proposal.RouterKey, params.NewParamChangeProposalHandler(pk)))
	f := &repair684Fixture{app: app, cdc: cdc, keys: keys, auth: ak, bank: bk, gov: gk, params: pk}
	for i := 0; i < 5; i++ {
		key := secp256k1.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair684 account %d", i)))
		f.accounts = append(f.accounts, simtypes.Account{Address: sdk.AccAddress(key.PubKey().Address()), PrivKey: key, PubKey: key.PubKey()})
	}
	app.SetInterfaceRegistry(registry)
	govtypes.RegisterMsgServer(app.MsgServiceRouter(), govkeeper.NewMsgServerImpl(gk))
	app.SetAnteHandler(sdk.ChainAnteDecorators(ante.NewDefaultSetUpContextDecorator(), ante.NewSigVerificationDecorator(ak, gen.SignModeHandler()), ante.NewIncrementSequenceDecorator(ak)))
	app.SetInitChainer(func(ctx sdk.Context, _ abci.RequestInitChain) abci.ResponseInitChain {
		ak.SetParams(ctx, authtypes.DefaultParams())
		bankGenesis := banktypes.DefaultGenesisState()
		for _, account := range f.accounts {
			acc := ak.NewAccountWithAddress(ctx, account.Address)
			require.NoError(t, acc.SetPubKey(account.PubKey))
			ak.SetAccount(ctx, acc)
			bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{Address: account.Address.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1000000))})
		}
		bk.InitGenesis(ctx, bankGenesis)
		genesis := govtypes.DefaultGenesisState()
		genesis.StartingProposalId = 0x010203
		genesis.DepositParams.MinDeposit = sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1))
		genesis.DepositParams.MinExpeditedDeposit = sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 2))
		genesis.VotingParams = govtypes.NewVotingParams(12*time.Second, 6*time.Second)
		gov.InitGenesis(ctx, ak, bk, gk, genesis)
		return abci.ResponseInitChain{Validators: []abci.ValidatorUpdate{{PubKey: cryptoproto.PublicKey{Sum: &cryptoproto.PublicKey_Ed25519{Ed25519: bytes.Repeat([]byte{7}, 32)}}, Power: 10}}}
	})
	return f
}

func (f *repair684Fixture) start(t *testing.T) sdk.Context {
	t.Helper()
	require.NoError(t, f.app.LoadLatestVersion())
	_, err := f.app.InitChain(context.Background(), &abci.RequestInitChain{ChainId: repair684Chain, Time: repair684Time})
	require.NoError(t, err)
	*f.app.DeliverContext() = f.app.DeliverContext().WithBlockHeight(1)
	return *f.app.DeliverContext()
}

func repair684Content(_ *rand.Rand, _ sdk.Context, _ []simtypes.Account) simtypes.Content {
	return govtypes.NewTextProposal("Simulation repair", "Exercise the submitted proposal and its scheduled votes", false)
}

func TestRepairM_684_DecoderEndian(t *testing.T) {
	f := newRepair684Fixture(t)
	ctx := f.start(t)
	ids := []uint64{0x0102030405060708, 0x1020304050607080}
	end := ctx.BlockTime().Add(time.Hour)
	for _, prefix := range [][]byte{govtypes.ProposalIDKey, govtypes.ActiveProposalQueuePrefix, govtypes.InactiveProposalQueuePrefix} {
		pairs := make([]kv.Pair, 2)
		for i, id := range ids {
			var key []byte
			switch prefix[0] {
			case govtypes.ProposalIDKey[0]:
				f.gov.SetProposalID(ctx, id)
				key = govtypes.ProposalIDKey
			case govtypes.ActiveProposalQueuePrefix[0]:
				f.gov.InsertActiveProposalQueue(ctx, id, end)
				key = govtypes.ActiveProposalQueueKey(id, end)
			case govtypes.InactiveProposalQueuePrefix[0]:
				f.gov.InsertInactiveProposalQueue(ctx, id, end)
				key = govtypes.InactiveProposalQueueKey(id, end)
			}
			pairs[i] = kv.Pair{Key: key, Value: ctx.KVStore(f.keys[govtypes.StoreKey]).Get(key)}
			require.Len(t, pairs[i].Value, 8)
		}
		require.Equal(t, fmt.Sprintf("proposalIDA: %d\nProposalIDB: %d", ids[0], ids[1]), govsimulation.NewDecodeStore(f.cdc)(pairs[0], pairs[1]))
	}
}

func TestRepairM_684_ScheduledVotesTargetSubmittedProposal(t *testing.T) {
	f := newRepair684Fixture(t)
	ctx := f.start(t)
	submitted, err := f.gov.GetProposalID(ctx)
	require.NoError(t, err)
	msg, future, err := govsimulation.SimulateMsgSubmitProposal(f.auth, f.bank, f.gov, repair684Content)(rand.New(rand.NewSource(684)), f.app, ctx, f.accounts, repair684Chain)
	require.NoError(t, err)
	require.True(t, msg.OK)
	require.NotEmpty(t, future)
	next, err := f.gov.GetProposalID(ctx)
	require.NoError(t, err)
	require.Equal(t, submitted+1, next)
	p, found := f.gov.GetProposal(ctx, submitted)
	require.True(t, found)
	require.Equal(t, govtypes.StatusVotingPeriod, p.Status)
	require.Empty(t, f.gov.GetVotes(ctx, submitted))
	for _, scheduled := range future {
		require.NotNil(t, scheduled.Op)
		vote, nested, err := scheduled.Op(rand.New(rand.NewSource(685)), f.app, ctx.WithBlockTime(scheduled.BlockTime), f.accounts, repair684Chain)
		require.NoError(t, err)
		require.True(t, vote.OK)
		require.Empty(t, nested)
	}
	votes := f.gov.GetVotes(ctx, submitted)
	require.Len(t, votes, len(future))
	for _, vote := range votes {
		require.Equal(t, submitted, vote.ProposalId)
		require.Len(t, vote.Options, 1)
		require.True(t, vote.Options[0].Weight.Equal(sdk.OneDec()))
	}
	require.Empty(t, f.gov.GetVotes(ctx, next))
	_, found = f.gov.GetProposal(ctx, next)
	require.False(t, found)
}

func TestRepairM_684_ScheduleThroughExecutor(t *testing.T) {
	f := newRepair684Fixture(t)
	submit := govsimulation.SimulateMsgSubmitProposal(f.auth, f.bank, f.gov, repair684Content)
	scheduledCount := 0
	var proposalID uint64
	operation := func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chain string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		if ctx.BlockHeight() != 1 {
			return simtypes.NoOpMsg(govtypes.ModuleName, "tick", "advance scheduled votes"), nil, nil
		}
		var err error
		proposalID, err = f.gov.GetProposalID(ctx)
		require.NoError(t, err)
		msg, future, err := submit(r, app, ctx, accs, chain)
		require.NoError(t, err)
		require.True(t, msg.OK)
		require.NotEmpty(t, future)
		scheduledCount = len(future)
		for _, next := range future {
			require.NotNil(t, next.Op)
			require.Zero(t, next.BlockHeight)
			require.False(t, next.BlockTime.Before(ctx.BlockTime()))
			require.True(t, next.BlockTime.Before(ctx.BlockTime().Add(12*time.Second)))
		}
		return msg, future, err
	}
	result, err := simulation.Simulate(t, f.app, simulation.SimulationConfig{
		Seed: 684, NumBlocks: 4, BlockSize: 1, Params: simulation.RandomParams(rand.New(rand.NewSource(684))),
		Genesis:  abci.RequestInitChain{ChainId: repair684Chain, Time: repair684Time},
		Accounts: f.accounts, Operations: simulation.WeightedOperations{simulation.NewWeightedOperation(1, operation)},
		BeginBlock: func(sdk.Context, abci.RequestBeginBlock) error { return nil }, LogDirectory: t.TempDir(),
	})
	require.NoError(t, err)
	require.Equal(t, 4, result.Blocks)
	require.Equal(t, 4+scheduledCount, result.Operations)
	ctx := f.app.NewUncachedContext(false, tmproto.Header{ChainID: repair684Chain, Height: f.app.LastBlockHeight()})
	require.Len(t, f.gov.GetVotes(ctx, proposalID), scheduledCount)
	require.Empty(t, f.gov.GetVotes(ctx, proposalID+1))
	log, err := os.ReadFile(result.LogPath)
	require.NoError(t, err)
	require.Equal(t, scheduledCount, bytes.Count(log, []byte(`"entry_kind":"queued_msg"`)))
}

func TestRepairM_684_VetoFieldThroughProposalHandler(t *testing.T) {
	f := newRepair684Fixture(t)
	ctx := f.start(t)
	changes := govsimulation.ParamChanges(rand.New(rand.NewSource(684)))
	var tally simtypes.ParamChange
	for _, change := range changes {
		if change.Key() == string(govtypes.ParamStoreKeyTallyParams) {
			tally = change
		}
	}
	require.NotNil(t, tally)
	r := rand.New(rand.NewSource(684))
	applied := 0
	for i := 0; i < 32; i++ {
		value := tally.SimValue()(r)
		var fields map[string]string
		require.NoError(t, json.Unmarshal([]byte(value), &fields))
		require.NotContains(t, fields, "veto")
		before := f.gov.GetTallyParams(ctx)
		content := proposal.NewParameterChangeProposal("Generated tally change", "Apply the simulation's generated fields", []proposal.ParamChange{{Subspace: tally.Subspace(), Key: tally.Key(), Value: value}}, false)
		require.NoError(t, content.ValidateBasic())
		require.NoError(t, f.gov.Router().GetRoute(content.ProposalRoute())(ctx, content))
		after := f.gov.GetTallyParams(ctx)
		if veto, ok := fields["veto_threshold"]; ok {
			require.True(t, after.VetoThreshold.Equal(sdk.MustNewDecFromStr(veto)))
			require.False(t, after.VetoThreshold.Equal(govtypes.DefaultVetoThreshold))
			applied++
		} else {
			require.True(t, after.VetoThreshold.Equal(before.VetoThreshold))
		}
	}
	require.Positive(t, applied, "generated tally changes must exercise the stored veto threshold")
}
