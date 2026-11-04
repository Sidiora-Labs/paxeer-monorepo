package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	srvconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	moduletestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	govkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair683Chain struct {
	app      *baseapp.BaseApp
	ctx      sdk.Context
	keeper   govkeeper.Keeper
	encoding moduletestutil.TestEncodingConfig
	node     local.NodeService
	bank     bankkeeper.BaseKeeper
	config   *config.Config
	genesis  *tmtypes.GenesisDoc
}

func repair683Encoding() moduletestutil.TestEncodingConfig {
	return moduletestutil.MakeTestEncodingConfig(auth.AppModuleBasic{}, bank.AppModuleBasic{}, staking.AppModuleBasic{}, gov.AppModuleBasic{})
}

func newRepair683Chain(t *testing.T) *repair683Chain {
	t.Helper()
	enc := repair683Encoding()
	cfg := config.TestConfig().SetRoot(t.TempDir())
	config.EnsureRoot(cfg.RootDir)
	cfg.Mode = config.ModeFull
	cfg.DBBackend = "goleveldb"
	cfg.TxIndex.Indexer = []string{"kv"}
	opts := viper.New()
	opts.Set("chain-id", "repair-683")
	app := baseapp.NewBaseApp("repair-683", dbm.NewMemDB(), enc.TxConfig.TxDecoder(), cfg, opts)
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
		"mint":                         {authtypes.Minter},
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
	_, err := app.InitChain(t.Context(), &abci.RequestInitChain{ChainId: "repair-683"})
	require.NoError(t, err)
	ctx := app.NewContext(false, tmproto.Header{Height: 1, ChainID: "repair-683", Time: time.Unix(1000, 0).UTC()})
	for name, perms := range permissions {
		ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, perms...))
	}
	bk.SetParams(ctx, banktypes.DefaultParams())
	sk.SetParams(ctx, stakingtypes.DefaultParams())
	k.SetProposalID(ctx, 1)
	k.SetDepositParams(ctx, govtypes.DefaultDepositParams())
	k.SetVotingParams(ctx, govtypes.DefaultVotingParams())
	k.SetTallyParams(ctx, govtypes.DefaultTallyParams())
	validatorKey, err := cryptocodec.ToTmPubKeyInterface(ed25519.GenPrivKeyFromSecret([]byte("repair-683-validator")).PubKey())
	require.NoError(t, err)
	genesis := &tmtypes.GenesisDoc{
		ChainID: "repair-683", GenesisTime: ctx.BlockTime(),
		Validators: []tmtypes.GenesisValidator{{Address: validatorKey.Address(), PubKey: validatorKey, Power: 1}},
	}
	node, err := tmnode.New(t.Context(), cfg, func() {}, app, genesis, nil, tmnode.NoOpMetricsProvider(), tmtypes.ConsensusPolicy{})
	require.NoError(t, err)
	// The node supplies the production stores and local RPC handlers; no consensus
	// service or listening endpoint is started in this integration test.
	chain := &repair683Chain{app: app, ctx: ctx, keeper: k, encoding: enc, node: node, bank: bk, config: cfg, genesis: genesis}
	t.Cleanup(func() {
		if chain.node != nil {
			chain.node.(interface{ OnStop() }).OnStop()
		}
	})
	return chain
}

func (chain *repair683Chain) client(t *testing.T) client.Context {
	t.Helper()
	rpc, err := local.New(chain.node)
	require.NoError(t, err)
	enc := repair683Encoding()
	return client.Context{}.WithClient(rpc).WithCodec(enc.Codec).WithLegacyAmino(enc.Amino).
		WithInterfaceRegistry(enc.InterfaceRegistry).WithTxConfig(enc.TxConfig).
		WithChainID("repair-683").WithOutputFormat("json")
}

func (chain *repair683Chain) commit() {
	chain.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	chain.app.CommitMultiStore().Commit(true)
	chain.ctx = chain.ctx.WithMultiStore(chain.app.CommitMultiStore().CacheMultiStore())
}

func TestRepairM_683_CLIConfig(t *testing.T) {
	cfg := network.DefaultConfig(t)
	integration := NewIntegrationTestSuite(cfg)
	deposits := NewDepositTestSuite(cfg)
	require.Equal(t, cfg.ChainID, integration.cfg.ChainID)
	require.Equal(t, cfg.ChainID, deposits.cfg.ChainID)
	require.NotNil(t, cfg.AppConstructor)
	home := t.TempDir()
	appConfig := srvconfig.DefaultConfig()
	appConfig.StateCommit.Directory = filepath.Join(home, "commit")
	appConfig.StateCommit.MemIAVLConfig.AsyncCommitBuffer = 0
	appConfig.StateStore.DBDirectory = filepath.Join(home, "query")
	appConfig.StateStore.AsyncWriteBuffer = 0
	serverCtx := server.NewDefaultContext()
	serverCtx.Config = config.TestConfig().SetRoot(home)
	app := cfg.AppConstructor(network.Validator{
		Dir: home, AppConfig: appConfig, Ctx: serverCtx,
		ClientCtx: client.Context{}.WithChainID(cfg.ChainID),
	})
	require.IsType(t, &paxapp.App{}, app)
	defer func() { require.NoError(t, app.Close()) }()
	var genesis govtypes.GenesisState
	require.NoError(t, cfg.Codec.UnmarshalAsJSON(cfg.GenesisState[govtypes.ModuleName], &genesis))
	require.Equal(t, govtypes.DefaultDepositParams(), genesis.DepositParams)
	require.Equal(t, govtypes.DefaultVotingParams(), genesis.VotingParams)
	require.Equal(t, govtypes.DefaultTallyParams(), genesis.TallyParams)
	genesisBytes, err := json.Marshal(cfg.GenesisState)
	require.NoError(t, err)
	_, err = app.InitChain(t.Context(), &abci.RequestInitChain{
		Time: time.Unix(1000, 0).UTC(), ChainId: cfg.ChainID, InitialHeight: 1,
		ConsensusParams: paxapp.DefaultConsensusParams, AppStateBytes: genesisBytes,
	})
	require.NoError(t, err)
	ctx := app.(*paxapp.App).NewContext(false, tmproto.Header{Height: 1})
	require.Equal(t, genesis.DepositParams, app.(*paxapp.App).GovKeeper.GetDepositParams(ctx))
	require.Equal(t, genesis.VotingParams, app.(*paxapp.App).GovKeeper.GetVotingParams(ctx))
	require.Equal(t, genesis.TallyParams, app.(*paxapp.App).GovKeeper.GetTallyParams(ctx))
}

func TestRepairM_683_Params(t *testing.T) {
	chain := newRepair683Chain(t)
	chain.commit()
	clientCtx := chain.client(t)
	s := NewIntegrationTestSuite(network.Config{LegacyAmino: chain.encoding.Amino})
	s.SetT(t)
	s.network = &network.Network{Validators: []*network.Validator{{ClientCtx: clientCtx}}}
	s.TestCmdParams()
	s.TestCmdParam()
	// Exercise the same fixture assertions against the keeper's actual defaults,
	// including all expedited fields, in both transport representations.
	for _, parameter := range []struct {
		name     string
		expected fmt.Stringer
	}{
		{"voting", govtypes.DefaultVotingParams()},
		{"tallying", govtypes.DefaultTallyParams()},
		{"deposit", govtypes.DefaultDepositParams()},
	} {
		jsonOut, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryParam(), []string{parameter.name, "--output=json"})
		require.NoError(t, err)
		s.assertParamOutput(jsonOut.String(), parameter.expected, "json")
		textOut, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryParam(), []string{parameter.name, "--output=text"})
		require.NoError(t, err)
		s.assertParamOutput(textOut.String(), parameter.expected, "text")
		require.YAMLEq(t, jsonOut.String(), textOut.String())
	}
}

func TestRepairM_683_CumulativeDeposits(t *testing.T) {
	chain := newRepair683Chain(t)
	coins := func(amount int64) sdk.Coins { return sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, amount)) }
	owners := []sdk.AccAddress{bytes.Repeat([]byte{1}, 20), bytes.Repeat([]byte{2}, 20)}
	for _, owner := range owners {
		require.NoError(t, chain.bank.MintCoins(chain.ctx, "mint", coins(30000000)))
		require.NoError(t, chain.bank.SendCoinsFromModuleToAccount(chain.ctx, "mint", owner, coins(30000000)))
	}
	var txs []tmtypes.Tx
	var results []*abci.TxResultV2
	execute := func(msg sdk.Msg) {
		t.Helper()
		builder := chain.encoding.TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetMemo(fmt.Sprintf("repair-683-%d", len(txs)))
		encoded, err := chain.encoding.TxConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := chain.encoding.TxConfig.TxDecoder()(encoded)
		require.NoError(t, err)
		result, err := chain.app.RunMsgs(chain.ctx, decoded.GetMsgs())
		require.NoError(t, err)
		results = append(results, &abci.TxResultV2{Height: 1, Index: uint32(len(txs)), Tx: encoded,
			Result: abci.ExecTxResult{Data: result.Data, Log: result.Log, Events: sdk.MarkEventsToIndex(result.Events, map[string]struct{}{})}})
		txs = append(txs, encoded)
	}
	initial := govtypes.DefaultMinDepositTokens.Int64() - 20
	msg, err := govtypes.NewMsgSubmitProposal(govtypes.NewTextProposal("Cumulative deposit", "Keep each depositor's entire amount", false), coins(initial), owners[0])
	require.NoError(t, err)
	execute(msg)
	execute(govtypes.NewMsgDeposit(owners[0], 1, coins(50)))
	execute(govtypes.NewMsgDeposit(owners[1], 1, coins(7)))
	execute(govtypes.NewMsgDeposit(owners[1], 1, coins(3)))
	expected := govtypes.Deposits{govtypes.NewDeposit(1, owners[0], coins(initial+50)), govtypes.NewDeposit(1, owners[1], coins(10))}
	block := tmtypes.MakeBlock(1, txs, &tmtypes.Commit{}, nil)
	block.ChainID = "repair-683"
	block.Time = chain.ctx.BlockTime()
	pubKey, err := cryptocodec.ToTmPubKeyInterface(ed25519.GenPrivKeyFromSecret([]byte("repair-683-validator")).PubKey())
	require.NoError(t, err)
	validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pubKey, 1)})
	block.ProposerAddress = pubKey.Address()
	block.ValidatorsHash = validators.Hash()
	block.NextValidatorsHash = validators.Hash()
	require.NoError(t, block.ValidateBasic(tmtypes.ConsensusPolicy{}))
	parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
	require.NoError(t, err)
	chain.node.RPCEnvironment().BlockStore.SaveBlock(block, parts, &tmtypes.Commit{Height: 1})
	require.Len(t, chain.node.RPCEnvironment().EventSinks, 1)
	require.NoError(t, chain.node.RPCEnvironment().EventSinks[0].IndexTxEvents(results))
	chain.commit()
	check := func(t *testing.T) {
		t.Helper()
		clientCtx := chain.client(t)
		deposits := NewDepositTestSuite(network.Config{})
		deposits.SetT(t)
		for i, want := range expected {
			validator := &network.Validator{ClientCtx: clientCtx, Address: owners[i]}
			got := deposits.queryDeposit(validator, "1", false)
			assert.Equal(t, want, *got)
		}
		got := deposits.queryDeposits(&network.Validator{ClientCtx: clientCtx}, "1", false)
		assert.ElementsMatch(t, expected, got, "deposits must remain cumulative per account")
	}
	for i, want := range expected {
		live, found := chain.keeper.GetDeposit(chain.ctx, 1, owners[i])
		require.True(t, found)
		require.Equal(t, want, live)
	}
	t.Run("before_finalization", check)
	proposal, found := chain.keeper.GetProposal(chain.ctx, 1)
	require.True(t, found)
	require.Equal(t, govtypes.StatusVotingPeriod, proposal.Status)
	chain.ctx = chain.ctx.WithBlockTime(proposal.VotingEndTime.Add(time.Second))
	gov.EndBlocker(chain.ctx, chain.keeper)
	proposal, found = chain.keeper.GetProposal(chain.ctx, 1)
	require.True(t, found)
	require.Equal(t, govtypes.StatusRejected, proposal.Status)
	require.Empty(t, chain.keeper.GetDeposits(chain.ctx, 1))
	chain.commit()
	t.Run("after_finalization", check)
	// Stop all index/store handles, reopen the same on-disk node databases,
	// and query with a fresh production RPC client without indexing again.
	chain.node.(interface{ OnStop() }).OnStop()
	chain.node = nil
	reopened, err := tmnode.New(t.Context(), chain.config, func() {}, chain.app,
		chain.genesis, nil,
		tmnode.NoOpMetricsProvider(), tmtypes.ConsensusPolicy{})
	require.NoError(t, err)
	chain.node = reopened
	t.Run("after_index_reopen", check)
}
