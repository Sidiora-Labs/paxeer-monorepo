package utils_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmbytes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	clientcli "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/client/utils"
	clientkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/keeper"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	ibctmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

const repair334Chain = "repair-334"
const repair334Client = "07-tendermint-0"

type repair334Fixture struct {
	ctx            client.Context
	app            *baseapp.BaseApp
	store          *rootmulti.Store
	keeper         clientkeeper.Keeper
	key            *sdk.KVStoreKey
	cdc            *codec.ProtoCodec
	roots          map[int64][]byte
	blocks         []*tmtypes.Block
	vals           *tmtypes.ValidatorSet
	next           *tmtypes.ValidatorSet
	saveValidators func(int64, int64, *tmtypes.ValidatorSet) error
	url            string
}

func repair334ClientState(height uint64) *ibctmtypes.ClientState {
	return ibctmtypes.NewClientState(repair334Chain, ibctmtypes.DefaultTrustLevel,
		24*time.Hour, 48*time.Hour, time.Minute, clienttypes.NewHeight(334, height),
		commitmenttypes.GetSDKSpecs(), []string{"upgrade", "upgradedIBCState"}, false, false)
}

func (f *repair334Fixture) snapshot(t *testing.T, latest uint64) {
	t.Helper()
	cache := f.store.CacheMultiStore()
	ctx := sdk.NewContext(cache, tmproto.Header{ChainID: repair334Chain, Height: f.store.LastCommitID().Version + 1, Time: time.Now().UTC()}, false)
	f.keeper.SetClientState(ctx, repair334Client, repair334ClientState(latest))
	for _, h := range []uint64{3, 7, 9, 12} {
		root := sha256.Sum256([]byte(fmt.Sprintf("consensus-%d", h)))
		cs := ibctmtypes.NewConsensusState(time.Unix(int64(h), 0).UTC(), commitmenttypes.NewMerkleRoot(root[:]), root[:])
		f.keeper.SetClientConsensusState(ctx, repair334Client, clienttypes.NewHeight(334, h), cs)
	}
	cache.Write()
	id := f.store.Commit(true)
	f.roots[id.Version] = bytes.Clone(id.Hash)
}

func repair334NewFixture(t *testing.T, count int) *repair334Fixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	clienttypes.RegisterInterfaces(registry)
	ibctmtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	f := &repair334Fixture{cdc: cdc, key: sdk.NewKVStoreKey(host.StoreKey), roots: make(map[int64][]byte)}
	sc := storageconfig.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.HistoricalProofRateLimit = 0
	f.store = rootmulti.NewStore(t.TempDir(), sc, storageconfig.StateStoreConfig{}, nil)
	options := viper.New()
	options.Set(baseapp.FlagChainID, repair334Chain)
	cfg := config.DefaultConfig().SetRoot(t.TempDir())
	config.EnsureRoot(cfg.RootDir)
	cfg.Mode = config.ModeFull
	cfg.DBBackend = "memdb"
	cfg.P2P.PexReactor = false
	cfg.TxIndex.Indexer = []string{"null"}
	cfg.RPC.ListenAddress = "tcp://127.0.0.1:0"
	cfg.RPC.ExperimentalDisableWebsocket = true
	f.app = baseapp.NewBaseApp("repair334", dbm.NewMemDB(), nil, cfg, options, func(app *baseapp.BaseApp) { app.SetCMS(f.store) })
	t.Cleanup(func() { require.NoError(t, f.app.Close()) })
	f.app.SetInterfaceRegistry(registry)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	f.app.MountStores(f.key, paramsKey, transientKey)
	space := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), paramsKey, transientKey, host.ModuleName)
	f.keeper = clientkeeper.NewKeeper(cdc, f.key, space, nil, nil)
	clienttypes.RegisterQueryServer(f.app.GRPCQueryRouter(), f.keeper)
	require.NoError(t, f.app.LoadLatestVersion())
	_, err := f.app.InitChain(t.Context(), &abci.RequestInitChain{ChainId: repair334Chain, Time: time.Now().UTC()})
	require.NoError(t, err)
	f.snapshot(t, 7)
	f.snapshot(t, 9)
	f.snapshot(t, 12)

	genesis := &tmtypes.GenesisDoc{ChainID: repair334Chain, GenesisTime: time.Now().Add(-time.Hour).UTC()}
	var signers []tmtypes.PrivValidator
	signerDir := t.TempDir()
	for i := 0; i < count; i++ {
		key := ed25519.GenerateSecretKey()
		genesis.Validators = append(genesis.Validators, tmtypes.GenesisValidator{Address: key.Public().Address(), PubKey: key.Public(), Power: 10})
		signers = append(signers, privval.NewFilePV(key, filepath.Join(signerDir, fmt.Sprintf("key%d.json", i)), filepath.Join(signerDir, fmt.Sprintf("state%d.json", i))))
	}
	sort.Sort(tmtypes.PrivValidatorsByAddress(signers))
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	node, err := tmnode.New(ctx, cfg, func() {}, f.app, genesis, nil, tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	env := node.RPCEnvironment()
	t.Cleanup(func() { require.NoError(t, env.StateStore.Close()) })
	state, err := env.StateStore.Load()
	require.NoError(t, err)
	f.vals = state.Validators.Copy()
	next := append([]*tmtypes.Validator(nil), f.vals.Validators...)
	next = append(next, tmtypes.NewValidator(ed25519.GenerateSecretKey().Public(), 10), tmtypes.NewValidator(ed25519.GenerateSecretKey().Public(), 10))
	f.next = tmtypes.NewValidatorSet(next)
	f.saveValidators = env.StateStore.SaveValidatorSets
	require.NoError(t, f.saveValidators(1, 3, f.vals))
	require.NoError(t, f.saveValidators(4, 4, f.next))
	lastCommit := &tmtypes.Commit{}
	for h := int64(1); h <= 3; h++ {
		state.NextValidators = f.vals
		if h == 3 {
			state.NextValidators = f.next
		}
		if h > 1 {
			state.AppHash = f.roots[h-1]
		}
		block := state.MakeBlockAt(h, nil, lastCommit, nil, f.vals.GetProposer().Address, state.LastBlockTime.Add(time.Second))
		parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
		require.NoError(t, err)
		id := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
		votes := tmtypes.NewVoteSet(repair334Chain, h, 0, tmproto.PrecommitType, f.vals)
		commit, err := tmtypes.MakeCommit(t.Context(), id, h, 0, votes, signers, block.Time)
		require.NoError(t, err)
		require.NoError(t, f.vals.VerifyCommitLight(repair334Chain, id, h, commit))
		env.BlockStore.SaveBlock(block, parts, commit)
		f.blocks = append(f.blocks, block)
		lastCommit = commit
		state.LastBlockHeight, state.LastBlockID, state.LastBlockTime = h, id, block.Time
		state.LastValidators = f.vals.Copy()
		state.Validators = state.NextValidators.Copy()
	}
	// Complete the real reactor's sync transition before serving queries for
	// the next validator set, which is unavailable while block sync is pending.
	state.AppHash = f.roots[3]
	reactor, ok := env.ConsensusReactor.Get()
	require.True(t, ok)
	reactor.SwitchToConsensus(state, false)
	require.False(t, reactor.WaitSync())
	listeners, err := env.StartService(ctx, cfg)
	require.NoError(t, err)
	require.Len(t, listeners, 1)
	f.url = "http://" + listeners[0].Addr().String()
	rpc, err := rpchttp.New(f.url)
	require.NoError(t, err)
	f.ctx = client.Context{}.WithClient(rpc).WithChainID(repair334Chain).WithInterfaceRegistry(registry).WithCodec(cdc).WithOutputFormat("json")
	return f
}

// proxy observes real HTTP requests and can change the real backing store
// between requests; it always forwards to the production RPC handlers.
func (f *repair334Fixture) proxy(t *testing.T, before func(*http.Request, []byte), after func(*http.Request, []byte)) client.Context {
	t.Helper()
	target, err := url.Parse(f.url)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(target)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		r.Body = io.NopCloser(bytes.NewReader(body))
		if before != nil {
			before(r, body)
		}
		proxy.ServeHTTP(w, r)
		if after != nil {
			after(r, body)
		}
	}))
	t.Cleanup(server.Close)
	rpc, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	return f.ctx.WithClient(rpc)
}

func repair334RPCRequest(t *testing.T, body []byte) (string, map[string]json.RawMessage) {
	t.Helper()
	var request struct {
		Method string                     `json:"method"`
		Params map[string]json.RawMessage `json:"params"`
	}
	require.NoError(t, json.Unmarshal(body, &request))
	return request.Method, request.Params
}

func repair334CLI(t *testing.T, f *repair334Fixture, ctx client.Context, args ...string) *clienttypes.QueryConsensusStateResponse {
	t.Helper()
	var out bytes.Buffer
	ctx = ctx.WithOutput(&out)
	cmd := clientcli.GetCmdQueryConsensusState()
	cmd.SetArgs(args)
	require.NoError(t, cmd.ExecuteContext(context.WithValue(t.Context(), client.ClientContextKey, &ctx)))
	var response clienttypes.QueryConsensusStateResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(out.Bytes(), &response))
	return &response
}

func repair334AssertProof(t *testing.T, f *repair334Fixture, res *clienttypes.QueryConsensusStateResponse, consensusHeight uint64, version int64) {
	t.Helper()
	require.Equal(t, clienttypes.NewHeight(334, uint64(version+1)), res.ProofHeight)
	var proof commitmenttypes.MerkleProof
	require.NoError(t, f.cdc.Unmarshal(res.Proof, &proof))
	state, err := clienttypes.UnpackConsensusState(res.ConsensusState)
	require.NoError(t, err)
	value, err := clienttypes.MarshalConsensusState(f.cdc, state)
	require.NoError(t, err)
	path := commitmenttypes.NewMerklePath(host.StoreKey, string(host.FullConsensusStateKey(repair334Client, clienttypes.NewHeight(334, consensusHeight))))
	root := commitmenttypes.NewMerkleRoot(f.roots[version])
	require.NoError(t, proof.VerifyMembership(commitmenttypes.GetSDKSpecs(), &root, path, value))
	wrongPath := commitmenttypes.NewMerklePath(host.StoreKey, string(host.FullConsensusStateKey(repair334Client, clienttypes.NewHeight(334, 3))))
	require.Error(t, proof.VerifyMembership(commitmenttypes.GetSDKSpecs(), &root, wrongPath, value))
}

func TestRepair_334_LatestConsensusProof(t *testing.T) {
	f := repair334NewFixture(t, 1)
	plain := repair334CLI(t, f, f.ctx, repair334Client, "--latest-height", "--prove=false")
	proved := repair334CLI(t, f, f.ctx, repair334Client, "--latest-height")
	require.Equal(t, plain.ConsensusState, proved.ConsensusState)
	repair334AssertProof(t, f, proved, 12, 3)
	explicit := repair334CLI(t, f, f.ctx, repair334Client, "334-7")
	repair334AssertProof(t, f, explicit, 7, 3)
	historical := repair334CLI(t, f, f.ctx, repair334Client, "--latest-height", "--height=3")
	repair334AssertProof(t, f, historical, 9, 2)
	_, err := utils.QueryConsensusState(f.ctx, "07-tendermint-999", clienttypes.ZeroHeight(), true, true)
	require.Error(t, err)
}

func TestRepair_334_LatestConsensusPinnedSnapshot(t *testing.T) {
	f := repair334NewFixture(t, 1)
	var advanced atomic.Bool
	ctx := f.proxy(t, func(_ *http.Request, body []byte) {
		method, params := repair334RPCRequest(t, body)
		if method != "abci_query" {
			return
		}
		var path string
		require.NoError(t, json.Unmarshal(params["path"], &path))
		if path != "store/ibc/key" {
			return
		}
		if advanced.CompareAndSwap(false, true) {
			// The first query has not yet executed, so advance only before
			// the second query, after its proof snapshot has been selected.
			return
		}
		f.snapshot(t, 9)
	}, nil)
	res := repair334CLI(t, f, ctx, repair334Client, "--latest-height")
	require.True(t, advanced.Load())
	require.Equal(t, int64(4), f.store.LastCommitID().Version)
	repair334AssertProof(t, f, res, 12, 3)
	current := repair334CLI(t, f, f.ctx, repair334Client, "--latest-height", "--prove=false")
	require.NotEqual(t, current.ConsensusState, res.ConsensusState)
}

func TestRepair_334_HeaderValidatorPagination(t *testing.T) {
	for _, count := range []int{101, 200, 201} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			f := repair334NewFixture(t, count)
			node, err := f.ctx.GetNode()
			require.NoError(t, err)
			h, page, perPage := int64(3), 1, 10_000
			first, err := node.Validators(t.Context(), &h, &page, &perPage)
			require.NoError(t, err)
			require.Equal(t, 100, first.Count)
			require.Equal(t, count, first.Total)
			for _, requested := range []int64{0, 2} {
				header, height, err := utils.QueryTendermintHeader(f.ctx.WithHeight(requested))
				require.NoError(t, err)
				if requested == 0 {
					requested = 3
				}
				require.Equal(t, requested, height)
				require.Len(t, header.ValidatorSet.Validators, count)
				require.NoError(t, header.ValidateBasic())
				signed, err := tmtypes.SignedHeaderFromProto(header.SignedHeader)
				require.NoError(t, err)
				vals, err := tmtypes.ValidatorSetFromProto(header.ValidatorSet)
				require.NoError(t, err)
				require.Equal(t, f.vals.Hash(), vals.Hash())
				require.NoError(t, vals.VerifyCommitLight(repair334Chain, signed.Commit.BlockID, height, signed.Commit))
			}
		})
	}
}

func TestRepair_334_SelfConsensusValidatorPagination(t *testing.T) {
	f := repair334NewFixture(t, 101)
	for _, requested := range []int64{0, 2} {
		state, height, err := utils.QuerySelfConsensusState(f.ctx.WithHeight(requested))
		require.NoError(t, err)
		if requested == 0 {
			requested = 3
		}
		require.Equal(t, requested, height)
		block := f.blocks[height-1]
		require.Equal(t, block.Time, state.Timestamp)
		require.Equal(t, []byte(block.AppHash), state.Root.Hash)
		require.Equal(t, block.NextValidatorsHash, state.NextValidatorsHash)
		require.NoError(t, state.ValidateBasic())
		// Initialize the counterparty light client with the RPC-produced
		// consensus state, then persist and decode it through the real keeper.
		cache := f.store.CacheMultiStore()
		ctx := sdk.NewContext(cache, tmproto.Header{ChainID: repair334Chain, Height: 4, Time: time.Now().UTC()}, false)
		clientState := repair334ClientState(uint64(height))
		id := "07-tendermint-1"
		require.NoError(t, clientState.Initialize(ctx, f.cdc, f.keeper.ClientStore(ctx, id), state))
		f.keeper.SetClientState(ctx, id, clientState)
		f.keeper.SetClientConsensusState(ctx, id, clientState.GetLatestHeight(), state)
		cache.Write()
		f.store.Commit(true)
		readCtx := sdk.NewContext(f.store.CacheMultiStore(), ctx.BlockHeader(), true)
		stored, found := f.keeper.GetLatestClientConsensusState(readCtx, id)
		require.True(t, found)
		require.Equal(t, state, stored)
		next := f.vals
		if height == 3 {
			next = f.next
		}
		require.Equal(t, tmbytes.HexBytes(next.Hash()), stored.(*ibctmtypes.ConsensusState).NextValidatorsHash)
	}
}

func TestRepair_334_RejectInconsistentValidatorPages(t *testing.T) {
	for _, self := range []bool{false, true} {
		for _, failure := range []string{"total", "hash", "missing-page"} {
			t.Run(fmt.Sprintf("self=%t/%s", self, failure), func(t *testing.T) {
				f := repair334NewFixture(t, 101)
				height := int64(3)
				original := f.vals
				if self {
					height++
					original = f.next
				}
				var replacement *tmtypes.ValidatorSet
				switch failure {
				case "total":
					vals := append([]*tmtypes.Validator(nil), original.Validators...)
					vals = append(vals, tmtypes.NewValidator(ed25519.GenerateSecretKey().Public(), 10))
					replacement = tmtypes.NewValidatorSet(vals)
				case "hash":
					replacement = original.Copy()
					replacement.Validators[0].VotingPower++
				case "missing-page":
					replacement = tmtypes.NewValidatorSet(original.Validators[:1])
				}
				var changed atomic.Bool
				ctx := f.proxy(t, func(_ *http.Request, body []byte) {
					method, params := repair334RPCRequest(t, body)
					if method != "validators" {
						return
					}
					var page coretypes.Int64
					require.NoError(t, json.Unmarshal(params["page"], &page))
					if (failure == "hash" || page == 2) && changed.CompareAndSwap(false, true) {
						require.NoError(t, f.saveValidators(height, height, replacement))
					}
				}, nil)
				var err error
				if self {
					_, _, err = utils.QuerySelfConsensusState(ctx)
				} else {
					_, _, err = utils.QueryTendermintHeader(ctx)
				}
				require.True(t, changed.Load())
				require.Error(t, err)
				switch failure {
				case "total":
					require.ErrorContains(t, err, "inconsistent validator total")
				case "hash":
					require.ErrorContains(t, err, "does not match committed hash")
				case "missing-page":
					require.ErrorContains(t, err, "page")
				}
			})
		}
	}
}
