package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clientconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/genesis"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_636_FirstLastRanges(t *testing.T) {
	for _, backend := range []string{"memory", "goleveldb"} {
		t.Run(backend, func(t *testing.T) {
			var db dbm.DB
			if backend == "memory" {
				db = dbm.NewMemDB()
			} else {
				var err error
				db, err = dbm.NewGoLevelDB("ranges", t.TempDir())
				require.NoError(t, err)
			}
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			st := dbadapter.Store{DB: db}
			for _, key := range []string{"a", "b", "d", "g", "z", "\xff\x00", "\xff\xff"} {
				st.Set([]byte(key), []byte("value-"+key))
			}
			for _, tc := range []struct {
				name, start, end, first, last string
			}{
				{"bounded", "b", "g", "b", "d"},
				{"absent-start", "c", "z", "d", "g"},
				{"exclusive-end", "b", "d", "b", "b"},
				{"unbounded", "", "", "a", "\xff\xff"},
				{"no-lower-bound", "", "g", "a", "d"},
				{"no-upper-bound", "g", "", "g", "\xff\xff"},
				{"equal-existing", "b", "b", "", ""},
				{"reversed-existing", "g", "b", "", ""},
				{"empty-gap", "c", "d", "", ""},
				{"before-first", "\x00", "a", "", ""},
				{"after-last", "\xff\xff\x01", "", "", ""},
				{"binary", "\xff\x00", "\xff\xff", "\xff\x00", "\xff\x00"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var start, end []byte
					if tc.start != "" {
						start = []byte(tc.start)
					}
					if tc.end != "" {
						end = []byte(tc.end)
					}
					for _, op := range []struct {
						name string
						call func(store.KVStore, []byte, []byte) (kv.Pair, bool)
						want string
					}{
						{"First", store.First, tc.first},
						{"Last", store.Last, tc.last},
					} {
						t.Run(op.name, func(t *testing.T) {
							pair, ok := op.call(st, start, end)
							require.Equal(t, op.want != "", ok)
							if !ok {
								require.Equal(t, kv.Pair{}, pair)
								return
							}
							require.Equal(t, []byte(op.want), pair.Key)
							require.Equal(t, []byte("value-"+op.want), pair.Value)
							require.Equal(t, st.Get(pair.Key), pair.Value)
						})
					}
				})
			}
			for _, key := range st.GetAllKeyStrsInRange(nil, nil) {
				st.Delete([]byte(key))
			}
			for _, call := range []func(store.KVStore, []byte, []byte) (kv.Pair, bool){store.First, store.Last} {
				pair, ok := call(st, nil, nil)
				require.False(t, ok)
				require.Equal(t, kv.Pair{}, pair)
			}
		})
	}
}

func TestRepairM_636_FirstLastCloses(t *testing.T) {
	db, err := dbm.NewGoLevelDB("closes", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	st := dbadapter.Store{DB: db}
	for i := 0; i < 128; i++ {
		st.Set([]byte(fmt.Sprintf("k%03d", i)), []byte(fmt.Sprint(i)))
	}
	alive := func() string {
		count, err := db.DB().GetProperty("leveldb.aliveiters")
		require.NoError(t, err)
		return count
	}
	require.Equal(t, "0", alive())
	for _, bounds := range []struct {
		name       string
		start, end []byte
	}{
		{"bounded", []byte("k010"), []byte("k100")},
		{"unbounded", nil, nil},
		{"upper-only", nil, []byte("k010")},
		{"lower-only", []byte("k100"), nil},
		{"empty", []byte("x"), []byte("z")},
		{"equal-existing", []byte("k010"), []byte("k010")},
		{"reversed-existing", []byte("k100"), []byte("k010")},
	} {
		t.Run(bounds.name, func(t *testing.T) {
			// Even an empty real backend iterator holds a counted resource until Close.
			probe, err := db.ReverseIterator(bounds.start, bounds.end)
			require.NoError(t, err)
			require.Equal(t, "1", alive())
			require.NoError(t, probe.Close())
			require.Equal(t, "0", alive())
			for i := 0; i < 3; i++ {
				store.First(st, bounds.start, bounds.end)
				require.Equal(t, "0", alive(), "First leaked its iterator")
				store.Last(st, bounds.start, bounds.end)
				require.Equal(t, "0", alive(), "Last leaked its iterator")
			}
		})
	}
}

func TestRepairM_636_SDKAPIContract(t *testing.T) {
	data, err := os.ReadFile("../../contracts/cosmos-fork.md")
	require.NoError(t, err)
	doc := string(data)
	for _, heading := range []string{
		"## Go API", "## CLI", "## Errors", "## Error codes", "## Crypto",
		"### Store ranges and bulk keys", "### Historical stores and recovery", "### Genesis streams",
	} {
		require.Contains(t, doc, heading+"\n")
	}
	for _, contract := range []string{
		"ReverseIterator(start, end)", "[start,end)", "GetAllKeyStrsInRange(start, end)",
		"sdk/storev2/rootmulti", "ErrVersionedStateUnsupported", "RollbackUnsupportedError",
		"genesis.genesis-stream-file", "Wait() error", "ABCIError(codespace, code, log)",
		"config <key> [value]", "does not certify upgrade readiness",
	} {
		require.Contains(t, doc, contract)
	}
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	for _, anchor := range []string{"go-api", "store-ranges-and-bulk-keys", "historical-stores-and-recovery", "genesis-streams", "cli", "errors"} {
		require.Contains(t, string(readme), "../../contracts/cosmos-fork.md#"+anchor)
	}
}

func TestRepairM_636_BulkKeyNamespace(t *testing.T) {
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	parent := dbadapter.Store{DB: db}
	st := prefix.NewStore(prefix.NewStore(parent, []byte("tenant/")), []byte("bank/"))
	for _, key := range []string{"a", "b", "c", "d"} {
		st.Set([]byte(key), []byte("balance-"+key))
	}
	parent.Set([]byte("tenant/staking/b"), []byte("sibling"))
	parent.Set([]byte("other/bank/b"), []byte("other-tenant"))
	require.Equal(t, []string{"b", "c"}, st.GetAllKeyStrsInRange([]byte("b"), []byte("d")))
	for _, key := range st.GetAllKeyStrsInRange(nil, nil) {
		require.Equal(t, []byte("balance-"+key), st.Get([]byte(key)))
	}
	cache := st.CacheWrap(storetypes.NewKVStoreKey("bank")).(storetypes.CacheKVStore)
	require.NoError(t, cache.DeleteAll([]byte("b"), []byte("d")))
	require.Equal(t, []string{"a", "d"}, cache.GetAllKeyStrsInRange(nil, nil))
	require.Equal(t, []byte("balance-b"), st.Get([]byte("b")))
	cache.Write()
	require.Equal(t, []string{"a", "d"}, st.GetAllKeyStrsInRange(nil, nil))
	require.NoError(t, st.DeleteAll([]byte("d"), nil))
	require.Equal(t, []string{"a"}, st.GetAllKeyStrsInRange(nil, nil))
	require.Equal(t, []byte("sibling"), parent.Get([]byte("tenant/staking/b")))
	require.Equal(t, []byte("other-tenant"), parent.Get([]byte("other/bank/b")))
}

func TestRepairM_636_ErrorTransport(t *testing.T) {
	for _, sentinel := range []*sdkerrors.Error{sdkerrors.ErrInvalidRequest, sdkerrors.ErrUnauthorized, sdkerrors.ErrOutOfGas} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			response := sdkerrors.QueryResult(sdkerrors.Wrap(sentinel, "store request refused"))
			wire, err := response.Marshal()
			require.NoError(t, err)
			var received abci.ResponseQuery
			require.NoError(t, received.Unmarshal(wire))
			require.Equal(t, sentinel.Codespace(), received.Codespace)
			require.Equal(t, sentinel.ABCICode(), received.Code)
			require.Contains(t, received.Log, "store request refused")
			require.True(t, sentinel.Is(sdkerrors.ABCIError(received.Codespace, received.Code, received.Log)))
			require.False(t, sentinel.Is(sdkerrors.ABCIError("unregistered-m636", received.Code, received.Log)))
		})
	}
	space, code, log := sdkerrors.ABCIInfo(nil, false)
	require.Empty(t, space)
	require.Zero(t, code)
	require.Empty(t, log)
}

func TestRepairM_636_CLIContract(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "config")
	require.NoError(t, os.MkdirAll(configPath, 0700))
	require.NoError(t, clientconfig.SetClientConfig("chain-id", "m636-consensus", configPath, &clientconfig.ClientConfig{}))
	execute := func(args ...string) (string, error) {
		clientCtx := client.Context{Viper: viper.New()}.WithHomeDir(home)
		cmd := clientconfig.Cmd()
		cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return output.String(), err
	}
	for _, setting := range []struct{ key, value string }{
		{"chain-id", "m636-consensus"}, {"keyring-backend", "memory"},
		{"output", "json"}, {"node", "tcp://localhost:26657"}, {"broadcast-mode", "sync"},
	} {
		_, err := execute(setting.key, setting.value)
		require.NoError(t, err)
		out, err := execute(setting.key)
		require.NoError(t, err)
		require.Equal(t, setting.value+"\n", out)
	}
	_, err := execute("unsupported-key", "value")
	require.ErrorContains(t, err, "unknown configuration key")
	conf, err := clientconfig.GetClientConfig(configPath, viper.New())
	require.NoError(t, err)
	require.Equal(t, "tcp://localhost:26657", conf.Node)
	require.Equal(t, "m636-consensus", conf.ChainID)
}

func TestRepairM_636_HistoricalStoreContract(t *testing.T) {
	ms := store.NewCommitMultiStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, ms.Close()) })
	key := storetypes.NewKVStoreKey("bank")
	ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, ms.LoadLatestVersion())
	for i, value := range []string{"old", "latest"} {
		cache := ms.CacheMultiStore()
		cache.GetKVStore(key).Set([]byte("balance"), []byte(value))
		cache.Write()
		require.Equal(t, int64(i+1), ms.Commit(true).Version)
	}
	commit := ms.LastCommitID()
	for _, version := range []int64{1, 2, 3} {
		for _, load := range []func(int64) (storetypes.CacheMultiStore, error){ms.CacheMultiStoreWithVersion, ms.CacheMultiStoreForExport} {
			cache, err := load(version)
			require.ErrorIs(t, err, rootmulti.ErrVersionedStateUnsupported)
			require.Nil(t, cache)
		}
		response := ms.(storetypes.Queryable).Query(context.Background(), abci.RequestQuery{
			Path: "/bank/key", Data: []byte("balance"), Height: version, Prove: true,
		})
		require.Equal(t, sdkerrors.ErrInvalidRequest.ABCICode(), response.Code)
		require.Contains(t, response.Log, rootmulti.ErrVersionedStateUnsupported.Error())
		require.Empty(t, response.Value)
		require.Nil(t, response.ProofOps)
	}
	err := ms.RollbackToVersion(1)
	var unsupported *rootmulti.RollbackUnsupportedError
	require.ErrorAs(t, err, &unsupported)
	require.ErrorIs(t, err, rootmulti.ErrVersionedStateUnsupported)
	require.Equal(t, commit, ms.LastCommitID())
	require.Equal(t, []byte("latest"), ms.CacheMultiStore().GetKVStore(key).Get([]byte("balance")))
}

type repairM636Genesis struct {
	ms      storetypes.CommitMultiStore
	ctx     sdk.Context
	keeper  paramkeeper.Keeper
	cdc     *codec.ProtoCodec
	manager *module.Manager
	closed  bool
}

func repairM636OpenGenesis(t *testing.T, dir string) *repairM636Genesis {
	t.Helper()
	db, err := dbm.NewGoLevelDB("genesis", dir)
	require.NoError(t, err)
	ms := store.NewCommitMultiStore(db)
	key := storetypes.NewKVStoreKey(paramtypes.StoreKey)
	tkey := storetypes.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(tkey, storetypes.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	keeper := paramkeeper.NewKeeper(cdc, codec.NewLegacyAmino(), key, tkey)
	f := &repairM636Genesis{ms: ms, ctx: sdk.NewContext(ms, tmproto.Header{Height: 1, Time: time.Unix(1, 0)}, false), keeper: keeper, cdc: cdc, manager: module.NewManager(params.NewAppModule(keeper))}
	t.Cleanup(func() { f.close(t) })
	return f
}

func (f *repairM636Genesis) close(t *testing.T) {
	t.Helper()
	if !f.closed {
		require.NoError(t, f.ms.Close())
		f.closed = true
	}
}

func TestRepairM_636_GenesisStreamContract(t *testing.T) {
	producer := repairM636OpenGenesis(t, t.TempDir())
	state := paramtypes.DefaultGenesis()
	state.CosmosGasParams.CosmosGasMultiplierDenominator = 636
	producer.keeper.InitGenesis(producer.ctx, state)
	var exported bytes.Buffer
	encoder := json.NewEncoder(&exported)
	require.NoError(t, producer.manager.ProcessGenesisPerModule(producer.ctx, producer.cdc, func(name string, data json.RawMessage) error {
		return encoder.Encode(module.ModuleState{AppState: module.AppState{Module: name, Data: data}})
	}))
	metadata, err := json.Marshal(server.GenesisDocNoAppState{
		ChainID: "m636-consensus", InitialHeight: 1, GenesisTime: time.Unix(1, 0).UTC(), ConsensusParams: tmtypes.DefaultConsensusParams(),
	})
	require.NoError(t, err)
	exported.Write(metadata)
	path := filepath.Join(t.TempDir(), "genesis.stream")
	require.NoError(t, os.WriteFile(path, exported.Bytes(), 0600))
	dir := t.TempDir()
	consumer := repairM636OpenGenesis(t, dir)
	stream := genesis.IngestGenesisFileLineByLine(path)
	consumer.manager.InitGenesisStream(consumer.ctx, consumer.cdc, stream)
	require.NoError(t, stream.Wait())
	require.Equal(t, uint64(636), consumer.keeper.ExportGenesis(consumer.ctx).CosmosGasParams.CosmosGasMultiplierDenominator)
	commit := consumer.ms.Commit(true)
	consumer.close(t)
	reopened := repairM636OpenGenesis(t, dir)
	require.Equal(t, commit, reopened.ms.LastCommitID())
	require.Equal(t, uint64(636), reopened.keeper.ExportGenesis(reopened.ctx).CosmosGasParams.CosmosGasMultiplierDenominator)
	missing := genesis.IngestGenesisFileLineByLine(filepath.Join(t.TempDir(), "missing"))
	require.ErrorIs(t, missing.Consume(func(string) error { t.Fatal("missing stream emitted a record"); return nil }), os.ErrNotExist)
	// A real truncated export must fail before publishing its cached state.
	badPath := filepath.Join(t.TempDir(), "truncated.stream")
	require.NoError(t, os.WriteFile(badPath, []byte(strings.TrimSuffix(exported.String(), "}")), 0600))
	rejected := repairM636OpenGenesis(t, t.TempDir())
	rejected.keeper.InitGenesis(rejected.ctx, paramtypes.DefaultGenesis())
	before := rejected.keeper.ExportGenesis(rejected.ctx)
	require.Panics(t, func() {
		rejected.manager.InitGenesisStream(rejected.ctx, rejected.cdc, genesis.IngestGenesisFileLineByLine(badPath))
	})
	require.Equal(t, before, rejected.keeper.ExportGenesis(rejected.ctx))
}
