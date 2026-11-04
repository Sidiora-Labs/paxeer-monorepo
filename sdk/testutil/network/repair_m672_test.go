package network

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	srvconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_672_AppConfigRestart(t *testing.T) {
	cfg := DefaultConfig(t)
	encoding := app.MakeEncodingConfig()
	constructor := NewAppConstructor(t, encoding)
	home := t.TempDir()
	appConfig := srvconfig.DefaultConfig()
	appConfig.MinGasPrices = "0.123uhpx"
	appConfig.ConcurrencyWorkers = 2
	appConfig.OccEnabled = false
	appConfig.StateCommit.Directory = filepath.Join(home, "custom-commit")
	appConfig.StateCommit.MemIAVLConfig.AsyncCommitBuffer = 0
	appConfig.StateCommit.MemIAVLConfig.SnapshotInterval = 0
	appConfig.StateStore.DBDirectory = filepath.Join(home, "custom-query")
	appConfig.StateStore.AsyncWriteBuffer = 0
	ctx := server.NewDefaultContext()
	ctx.Config = tmconfig.DefaultConfig().SetRoot(home)
	val := Validator{Dir: home, AppConfig: appConfig, Ctx: ctx,
		ClientCtx: client.Context{}.WithChainID("pax-test")}

	address := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 987654321))
	var authGenesis authtypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[authtypes.ModuleName], &authGenesis)
	accounts, err := authtypes.PackAccounts([]authtypes.GenesisAccount{authtypes.NewBaseAccountWithAddress(address)})
	require.NoError(t, err)
	authGenesis.Accounts = accounts
	cfg.GenesisState[authtypes.ModuleName] = cfg.Codec.MustMarshalJSON(&authGenesis)
	var bankGenesis banktypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[banktypes.ModuleName], &bankGenesis)
	bankGenesis.Balances = []banktypes.Balance{{Address: address.String(), Coins: coins}}
	bankGenesis.Supply = coins
	cfg.GenesisState[banktypes.ModuleName] = cfg.Codec.MustMarshalJSON(&bankGenesis)
	var distribution distrtypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[distrtypes.ModuleName], &distribution)
	distribution.Params.CommunityTax = sdk.MustNewDecFromStr("0.125")
	cfg.GenesisState[distrtypes.ModuleName] = cfg.Codec.MustMarshalJSON(&distribution)
	genesis, err := json.Marshal(cfg.GenesisState)
	require.NoError(t, err)

	a := constructor(val).(*app.App)
	t.Cleanup(func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	})
	require.Same(t, encoding.Marshaler, a.AppCodec())
	require.Same(t, encoding.InterfaceRegistry, a.InterfaceRegistry())
	require.Same(t, encoding.TxConfig, a.GetTxConfig())
	require.Equal(t, int64(0), a.LastBlockHeight())
	at := time.Now().UTC()
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: at, ChainId: "pax-test", InitialHeight: 1,
		ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
	})
	require.NoError(t, err)
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: at},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	committed := a.LastCommitID()
	require.Equal(t, int64(1), committed.Version)
	require.NotEmpty(t, committed.Hash)
	info, err := a.Info(context.Background(), &abci.RequestInfo{})
	require.NoError(t, err)
	require.Equal(t, appConfig.GetMinGasPrices().String(), info.MinimumGasPrices)
	require.NoError(t, a.Close())
	a = nil
	require.DirExists(t, filepath.Join(home, "data", "application.db"))
	require.DirExists(t, appConfig.StateCommit.Directory)
	require.DirExists(t, appConfig.StateStore.DBDirectory)

	a = constructor(val).(*app.App)
	require.Equal(t, committed, a.LastCommitID())
	info, err = a.Info(context.Background(), &abci.RequestInfo{})
	require.NoError(t, err)
	require.Equal(t, int64(1), info.LastBlockHeight)
	require.Equal(t, appConfig.GetMinGasPrices().String(), info.MinimumGasPrices)
	require.Same(t, encoding.Marshaler, a.AppCodec())
	require.Same(t, encoding.InterfaceRegistry, a.InterfaceRegistry())
	require.Same(t, encoding.TxConfig, a.GetTxConfig())
	require.Eventually(t, func() bool {
		queryCtx, queryErr := a.CreateQueryContext(0, false)
		return queryErr == nil && a.BankKeeper.GetAllBalances(queryCtx, address).IsEqual(coins) &&
			a.DistrKeeper.GetParams(queryCtx).CommunityTax.Equal(distribution.Params.CommunityTax)
	}, 5*time.Second, 10*time.Millisecond, "custom genesis balances and parameters must survive reopening the configured stores")
}

func TestRepairM_672_DefaultConfigLiveness(t *testing.T) {
	cfg := DefaultConfig(t)
	cfg.TimeoutCommit = 100 * time.Millisecond
	n := New(t, cfg)
	t.Cleanup(n.Cleanup)
	height, err := n.WaitForHeightWithTimeout(10, time.Minute)
	require.NoError(t, err, "expected to reach 10 blocks; got %d", height)
	require.GreaterOrEqual(t, height, int64(10))
}

func TestRepairM_672_SharedGenesis(t *testing.T) {
	cfg := DefaultConfig(t)
	cfg.NumValidators = 3
	cfg.TimeoutCommit = 100 * time.Millisecond
	cfg.BondedTokens = sdk.TokensFromConsensusPower(137, sdk.DefaultPowerReduction)
	n := New(t, cfg)
	t.Cleanup(n.Cleanup)
	height, err := n.WaitForHeightWithTimeout(3, time.Minute)
	require.NoError(t, err, "all three validators must agree to commit; got height %d", height)
	require.GreaterOrEqual(t, height, int64(3))

	var genesisHash, diskHash [sha256.Size]byte
	var blockHash []byte
	for i, val := range n.Validators {
		data, err := os.ReadFile(val.Ctx.Config.GenesisFile())
		require.NoError(t, err)
		genesis, err := tmtypes.GenesisDocFromFile(val.Ctx.Config.GenesisFile())
		require.NoError(t, err)
		require.Len(t, genesis.Validators, cfg.NumValidators)
		for j, validator := range genesis.Validators {
			require.Equal(t, n.Validators[j].Moniker, validator.Name)
			require.Equal(t, []byte(n.Validators[j].PubKey.Address()), []byte(validator.Address))
			require.Equal(t, int64(137), validator.Power)
		}
		rpc, err := local.New(val.tmNode.(local.NodeService))
		require.NoError(t, err)
		require.NoError(t, rpc.InitGenesisChunks())
		callCtx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		var wireGenesis []byte
		for chunk := uint(0); ; chunk++ {
			result, err := rpc.GenesisChunked(callCtx, chunk)
			require.NoError(t, err)
			decoded, err := base64.StdEncoding.DecodeString(result.Data)
			require.NoError(t, err)
			wireGenesis = append(wireGenesis, decoded...)
			if int(chunk)+1 == result.TotalChunks {
				break
			}
		}
		var consumed tmtypes.GenesisDoc
		require.NoError(t, tmjson.Unmarshal(wireGenesis, &consumed))
		require.NoError(t, consumed.ValidateAndComplete())
		require.Equal(t, genesis.ValidatorHash(), consumed.ValidatorHash())
		require.JSONEq(t, string(genesis.AppState), string(consumed.AppState))
		require.Eventually(t, func() bool {
			status, statusErr := rpc.Status(callCtx)
			return statusErr == nil && status.SyncInfo.LatestBlockHeight >= 3
		}, 10*time.Second, 20*time.Millisecond)
		blockHeight := int64(3)
		block, err := rpc.Block(callCtx, &blockHeight)
		require.NoError(t, err)
		require.Equal(t, genesis.ValidatorHash(), []byte(block.Block.ValidatorsHash))
		if i == 0 {
			genesisHash = sha256.Sum256(wireGenesis)
			diskHash = sha256.Sum256(data)
			blockHash = append([]byte(nil), block.BlockID.Hash...)
		} else {
			require.Equal(t, genesisHash, sha256.Sum256(wireGenesis), "consensus must consume identical genesis")
			require.Equal(t, diskHash, sha256.Sum256(data), "persisted genesis must be identical")
			require.Equal(t, blockHash, []byte(block.BlockID.Hash), "validators must commit the same block")
		}
	}
}
