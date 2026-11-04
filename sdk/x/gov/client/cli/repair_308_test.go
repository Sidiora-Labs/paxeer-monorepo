package cli_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	moduletestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module/testutil"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	gcutils "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/utils"
	govkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type repair308Chain struct {
	app      *baseapp.BaseApp
	ctx      sdk.Context
	keeper   govkeeper.Keeper
	encoding moduletestutil.TestEncodingConfig
	node     local.NodeService
	proposal govtypes.Proposal
}

func repair308Encoding() moduletestutil.TestEncodingConfig {
	enc := moduletestutil.MakeTestEncodingConfig(gov.AppModuleBasic{})
	authtypes.RegisterLegacyAminoCodec(enc.Amino)
	authtypes.RegisterInterfaces(enc.InterfaceRegistry)
	banktypes.RegisterLegacyAminoCodec(enc.Amino)
	banktypes.RegisterInterfaces(enc.InterfaceRegistry)
	stakingtypes.RegisterLegacyAminoCodec(enc.Amino)
	stakingtypes.RegisterInterfaces(enc.InterfaceRegistry)
	return enc
}

func newRepair308Chain(t *testing.T, indexer string) *repair308Chain {
	t.Helper()
	enc := repair308Encoding()
	cfg := config.TestConfig().SetRoot(t.TempDir())
	config.EnsureRoot(cfg.RootDir)
	cfg.Mode = config.ModeFull
	cfg.DBBackend = "memdb"
	cfg.TxIndex.Indexer = []string{indexer}
	opts := viper.New()
	opts.Set("chain-id", "repair-308")
	app := baseapp.NewBaseApp("repair-308", dbm.NewMemDB(), enc.TxConfig.TxDecoder(), cfg, opts)
	sc := storageconfig.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.MemIAVLConfig.SnapshotInterval = 1
	sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
	app.SetCMS(storev2rootmulti.NewStore(t.TempDir(), sc, storageconfig.StateStoreConfig{}, nil))
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	app.SetInterfaceRegistry(enc.InterfaceRegistry)
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, govtypes.StoreKey, paramtypes.StoreKey)
	tkeys := sdk.NewTransientStoreKeys(paramtypes.TStoreKey)
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	pk := paramskeeper.NewKeeper(enc.Codec, enc.Amino, keys[paramtypes.StoreKey], tkeys[paramtypes.TStoreKey])
	permissions := map[string][]string{
		govtypes.ModuleName:            {authtypes.Burner},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	}
	ak := authkeeper.NewAccountKeeper(enc.Codec, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, permissions)
	bk := bankkeeper.NewBaseKeeper(enc.Codec, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), map[string]bool{})
	sk := stakingkeeper.NewKeeper(enc.Codec, keys[stakingtypes.StoreKey], ak, bk, pk.Subspace(stakingtypes.ModuleName))
	router := govtypes.NewRouter().AddRoute(govtypes.RouterKey, govtypes.ProposalHandler)
	k := govkeeper.NewKeeper(enc.Codec, keys[govtypes.StoreKey], pk.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable()), ak, bk, sk, pk, router)
	govtypes.RegisterQueryServer(app.GRPCQueryRouter(), k)
	govtypes.RegisterMsgServer(app.MsgServiceRouter(), govkeeper.NewMsgServerImpl(k))
	require.NoError(t, app.LoadLatestVersion())
	_, err := app.InitChain(t.Context(), &abci.RequestInitChain{ChainId: "repair-308"})
	require.NoError(t, err)
	ctx := app.NewContext(false, tmproto.Header{Height: 1, ChainID: "repair-308", Time: time.Unix(1000, 0).UTC()})
	for name, perms := range permissions {
		ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, perms...))
	}
	bk.SetParams(ctx, banktypes.DefaultParams())
	sk.SetParams(ctx, stakingtypes.DefaultParams())
	k.SetProposalID(ctx, 1)
	k.SetDepositParams(ctx, govtypes.DefaultDepositParams())
	k.SetVotingParams(ctx, govtypes.DefaultVotingParams())
	k.SetTallyParams(ctx, govtypes.DefaultTallyParams())
	proposal, err := k.SubmitProposal(ctx, govtypes.NewTextProposal("Historical vote", "Keep the recorded vote queryable after tally", false))
	require.NoError(t, err)
	k.ActivateVotingPeriod(ctx, proposal)
	proposal, found := k.GetProposal(ctx, proposal.ProposalId)
	require.True(t, found)
	node, err := tmnode.New(t.Context(), cfg, func() {}, app, &tmtypes.GenesisDoc{ChainID: "repair-308", GenesisTime: ctx.BlockTime()}, nil, tmnode.NoOpMetricsProvider(), tmtypes.ConsensusPolicy{})
	require.NoError(t, err)
	// The node supplies the production stores and local RPC handlers; no consensus
	// service or listening endpoint is started in this integration test.
	t.Cleanup(node.(interface{ OnStop() }).OnStop)
	return &repair308Chain{app: app, ctx: ctx, keeper: k, encoding: enc, node: node, proposal: proposal}
}

func (chain *repair308Chain) client(t *testing.T) client.Context {
	t.Helper()
	rpc, err := local.New(chain.node)
	require.NoError(t, err)
	enc := repair308Encoding()
	return client.Context{}.WithClient(rpc).WithCodec(enc.Codec).WithLegacyAmino(enc.Amino).
		WithInterfaceRegistry(enc.InterfaceRegistry).WithTxConfig(enc.TxConfig).
		WithChainID("repair-308").WithOutputFormat("json")
}

func (chain *repair308Chain) cast(t *testing.T, msg sdk.Msg, saveBlock bool) {
	t.Helper()
	builder := chain.encoding.TxConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	txBytes, err := chain.encoding.TxConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	decoded, err := chain.encoding.TxConfig.TxDecoder()(txBytes)
	require.NoError(t, err)
	res := chain.app.DeliverTx(chain.ctx, abci.RequestDeliverTxV2{Tx: txBytes}, decoded, sha256.Sum256(txBytes))
	require.Zero(t, res.Code, res.Log)
	env := chain.node.RPCEnvironment()
	if saveBlock {
		block := tmtypes.MakeBlock(1, []tmtypes.Tx{txBytes}, &tmtypes.Commit{}, nil)
		block.ChainID = "repair-308"
		block.Time = chain.ctx.BlockTime()
		pubKey, err := cryptocodec.ToTmPubKeyInterface(ed25519.GenPrivKeyFromSecret([]byte("repair-308-validator")).PubKey())
		require.NoError(t, err)
		validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pubKey, 1)})
		block.ProposerAddress = pubKey.Address()
		block.ValidatorsHash = validators.Hash()
		block.NextValidatorsHash = validators.Hash()
		require.NoError(t, block.ValidateBasic(tmtypes.ConsensusPolicy{}))
		parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
		require.NoError(t, err)
		env.BlockStore.SaveBlock(block, parts, &tmtypes.Commit{Height: 1})
	}
	result := &abci.TxResultV2{Height: 1, Tx: txBytes, Result: abci.ExecTxResult{
		Code: res.Code, Codespace: res.Codespace, Data: res.Data, Log: res.Log,
		Info: res.Info, GasWanted: res.GasWanted, GasUsed: res.GasUsed, Events: res.Events,
	}}
	for _, sink := range env.EventSinks {
		require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{result}))
	}
}

func (chain *repair308Chain) commit() {
	chain.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	chain.app.CommitMultiStore().Commit(true)
	chain.ctx = chain.ctx.WithMultiStore(chain.app.CommitMultiStore().CacheMultiStore())
}

func (chain *repair308Chain) tally(t *testing.T) {
	t.Helper()
	chain.ctx = chain.ctx.WithBlockTime(chain.proposal.VotingEndTime.Add(time.Second))
	gov.EndBlocker(chain.ctx, chain.keeper)
	proposal, found := chain.keeper.GetProposal(chain.ctx, chain.proposal.ProposalId)
	require.True(t, found)
	require.Equal(t, govtypes.StatusRejected, proposal.Status)
	chain.commit()
}

func repair308Query(ctx context.Context, clientCtx client.Context, proposalID uint64, voter sdk.AccAddress) (string, error) {
	var output bytes.Buffer
	clientCtx = clientCtx.WithOutput(&output)
	cmd := cli.GetCmdQueryVote()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{fmt.Sprint(proposalID), voter.String(), "--output=json"})
	err := cmd.ExecuteContext(context.WithValue(ctx, client.ClientContextKey, &clientCtx))
	return output.String(), err
}

func TestRepair_308_HistoricalSingleVoteAfterTallyAndClientRestart(t *testing.T) {
	for _, weighted := range []bool{false, true} {
		t.Run(fmt.Sprintf("weighted=%t", weighted), func(t *testing.T) {
			chain := newRepair308Chain(t, "kv")
			voter := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
			options := govtypes.NewNonSplitVoteOption(govtypes.OptionYes)
			var msg sdk.Msg = govtypes.NewMsgVote(voter, chain.proposal.ProposalId, govtypes.OptionYes)
			if weighted {
				options = govtypes.WeightedVoteOptions{
					{Option: govtypes.OptionYes, Weight: sdk.MustNewDecFromStr("0.7")},
					{Option: govtypes.OptionAbstain, Weight: sdk.MustNewDecFromStr("0.3")},
				}
				msg = govtypes.NewMsgVoteWeighted(voter, chain.proposal.ProposalId, options)
			}
			chain.cast(t, msg, true)
			chain.commit()
			oldClient := chain.client(t)
			ctx, stopClient := context.WithCancel(t.Context())
			output, err := repair308Query(ctx, oldClient, chain.proposal.ProposalId, voter)
			require.NoError(t, err)
			var liveVote govtypes.Vote
			require.NoError(t, oldClient.Codec.UnmarshalAsJSON([]byte(output), &liveVote))
			require.Equal(t, options, govtypes.WeightedVoteOptions(liveVote.Options))
			stopClient()

			chain.tally(t)
			_, found := chain.keeper.GetVote(chain.ctx, chain.proposal.ProposalId, voter)
			require.False(t, found, "the real final tally must remove live vote state")
			freshClient := chain.client(t)
			_, err = govtypes.NewQueryClient(freshClient).Vote(t.Context(), &govtypes.QueryVoteRequest{ProposalId: chain.proposal.ProposalId, Voter: voter.String()})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			output, err = repair308Query(t.Context(), freshClient, chain.proposal.ProposalId, voter)
			require.NoError(t, err)
			var recovered govtypes.Vote
			require.NoError(t, freshClient.Codec.UnmarshalAsJSON([]byte(output), &recovered))
			require.Equal(t, chain.proposal.ProposalId, recovered.ProposalId)
			require.Equal(t, voter.String(), recovered.Voter)
			require.Equal(t, options, govtypes.WeightedVoteOptions(recovered.Options))
		})
	}
}

func TestRepair_308_LiveVoteDoesNotRequireTransactionIndex(t *testing.T) {
	chain := newRepair308Chain(t, "null")
	voter := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	chain.cast(t, govtypes.NewMsgVote(voter, chain.proposal.ProposalId, govtypes.OptionNo), false)
	chain.commit()
	clientCtx := chain.client(t)
	output, err := repair308Query(t.Context(), clientCtx, chain.proposal.ProposalId, voter)
	require.NoError(t, err)
	var vote govtypes.Vote
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON([]byte(output), &vote))
	require.Equal(t, voter.String(), vote.Voter)
	require.Equal(t, govtypes.NewNonSplitVoteOption(govtypes.OptionNo), govtypes.WeightedVoteOptions(vote.Options))
	missing := sdk.AccAddress(bytes.Repeat([]byte{3}, 20))
	output, err = repair308Query(t.Context(), clientCtx, chain.proposal.ProposalId, missing)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Empty(t, output)
}

func TestRepair_308_HistoricalAbsenceAndIndexFailure(t *testing.T) {
	for _, indexer := range []string{"kv", "null"} {
		t.Run(indexer, func(t *testing.T) {
			chain := newRepair308Chain(t, indexer)
			voter := sdk.AccAddress(bytes.Repeat([]byte{4}, 20))
			chain.commit()
			chain.tally(t)
			clientCtx := chain.client(t)
			_, expected := gcutils.QueryVoteByTxQuery(t.Context(), clientCtx, govtypes.NewQueryVoteParams(chain.proposal.ProposalId, voter))
			require.Error(t, expected)
			if indexer == "kv" {
				require.Contains(t, expected.Error(), "did not vote")
			} else {
				require.Contains(t, expected.Error(), "transaction searching is disabled")
			}
			output, err := repair308Query(t.Context(), clientCtx, chain.proposal.ProposalId, voter)
			require.EqualError(t, err, expected.Error())
			require.Empty(t, output)
		})
	}
}

func TestRepair_308_HistoricalTransportErrorIsPreserved(t *testing.T) {
	chain := newRepair308Chain(t, "kv")
	voter := sdk.AccAddress(bytes.Repeat([]byte{5}, 20))
	chain.cast(t, govtypes.NewMsgVote(voter, chain.proposal.ProposalId, govtypes.OptionYes), false)
	chain.commit()
	chain.tally(t)
	clientCtx := chain.client(t)
	// The real index contains the vote, but the proof's block is unavailable.
	// This must remain a query failure rather than become an absent-vote error.
	_, expected := gcutils.QueryVoteByTxQuery(t.Context(), clientCtx, govtypes.NewQueryVoteParams(chain.proposal.ProposalId, voter))
	require.ErrorIs(t, expected, coretypes.ErrHeightNotAvailable)
	output, err := repair308Query(t.Context(), clientCtx, chain.proposal.ProposalId, voter)
	require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
	require.Empty(t, output)
}
