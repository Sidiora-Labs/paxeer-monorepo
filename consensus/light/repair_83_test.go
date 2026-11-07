package light_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	blockstore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	tmmath "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/math"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider"
	lighthttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store"
	dbs "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store/db"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repair83Produce(t *testing.T, key crypto.PrivKey, appHash []byte, heights ...int64) []*types.LightBlock {
	t.Helper()
	state, err := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID: chainID, GenesisTime: bTime, AppHash: appHash,
		Validators: []types.GenesisValidator{{Address: key.Public().Address(), PubKey: key.Public(), Power: 10}},
	})
	require.NoError(t, err)
	dir := t.TempDir()
	signer := privval.NewFilePV(key, filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
	lastCommit := &types.Commit{Signatures: []types.CommitSig{}}
	blocks := make([]*types.LightBlock, 0, len(heights))
	for _, height := range heights {
		blockTime := bTime.Add(time.Duration(height) * time.Second)
		block := state.MakeBlockAt(height, nil, lastCommit, nil, key.Public().Address(), blockTime)
		parts, err := block.MakePartSet(types.BlockPartSizeBytes)
		require.NoError(t, err)
		blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
		votes := types.NewVoteSet(chainID, height, 0, tmproto.PrecommitType, state.Validators)
		commit, err := types.MakeCommit(t.Context(), blockID, height, 0, votes, []types.PrivValidator{signer}, blockTime)
		require.NoError(t, err)
		lb := &types.LightBlock{SignedHeader: &types.SignedHeader{Header: &block.Header, Commit: commit}, ValidatorSet: state.Validators.Copy()}
		require.NoError(t, lb.ValidateBasic(chainID))
		require.NoError(t, lb.ValidatorSet.VerifyCommitLight(chainID, blockID, height, commit))
		blocks = append(blocks, lb)
		state.LastBlockID = blockID
		state.LastBlockHeight = height
		state.LastBlockTime = blockTime
		lastCommit = commit
	}
	return blocks
}

func repair83Environment(t *testing.T, blocks ...*types.LightBlock) *core.Environment {
	t.Helper()
	bs := blockstore.NewBlockStore(dbm.NewMemDB())
	ss := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, bs.Close()); require.NoError(t, ss.Close()) })
	for _, lb := range blocks {
		require.NoError(t, bs.SaveSignedHeader(lb.SignedHeader, lb.Commit.BlockID))
		require.NoError(t, bs.SaveSeenCommit(lb.Height, lb.Commit))
		validators := make([]types.GenesisValidator, len(lb.ValidatorSet.Validators))
		for i, val := range lb.ValidatorSet.Validators {
			validators[i] = types.GenesisValidator{Address: val.Address, PubKey: val.PubKey, Power: val.VotingPower}
		}
		state, err := sm.MakeGenesisState(&types.GenesisDoc{
			ChainID: chainID, InitialHeight: lb.Height, GenesisTime: bTime, Validators: validators,
		})
		require.NoError(t, err)
		require.NoError(t, ss.Save(state))
	}
	return &core.Environment{BlockStore: bs, StateStore: ss}
}

func repair83Provider(t *testing.T, env *core.Environment,
	commit func(context.Context, *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error)) provider.Provider {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"commit":     rpcserver.NewRPCFunc(commit),
		"validators": rpcserver.NewRPCFunc(env.Validators),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	p, err := lighthttp.New(chainID, server.URL)
	require.NoError(t, err)
	return p
}

func repair83Client(t *testing.T, primary provider.Provider, witnesses []provider.Provider, trusted store.Store, root *types.LightBlock) *light.Client {
	t.Helper()
	client, err := light.NewClient(t.Context(), chainID,
		light.TrustOptions{Period: 24 * time.Hour, Height: root.Height, Hash: root.Hash()},
		primary, witnesses, trusted, time.Hour)
	require.NoError(t, err)
	return client
}

func TestRepair_83_BackwardWorkLimit(t *testing.T) {
	blocks := repair83Produce(t, ed25519.GenerateSecretKey(), hash("canonical"), 1, 10002)
	env := repair83Environment(t, blocks...)
	var requests atomic.Int64
	p := repair83Provider(t, env, func(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error) {
		requests.Add(1)
		return env.Commit(ctx, req)
	})
	trusted := dbs.New(dbm.NewMemDB())
	client := repair83Client(t, p, []provider.Provider{p}, trusted, blocks[1])
	before := requests.Load()
	_, err := client.VerifyLightBlockAtHeight(t.Context(), 1, blocks[1].Time)
	var failure light.ErrVerificationFailed
	require.ErrorAs(t, err, &failure)
	require.EqualValues(t, 10002, failure.From)
	require.EqualValues(t, 1, failure.To)
	require.EqualValues(t, 1, requests.Load()-before, "the work limit must refuse before walking the chain")
	_, err = client.TrustedLightBlock(1)
	require.ErrorIs(t, err, store.ErrLightBlockNotFound)
	require.EqualValues(t, 1, trusted.Size())
	root, err := client.TrustedLightBlock(0)
	require.NoError(t, err)
	require.Equal(t, blocks[1].Hash(), root.Hash())
}

func TestRepair_83_SubstitutedHistoricalHeader(t *testing.T) {
	key := ed25519.GenerateSecretKey()
	blocks := repair83Produce(t, key, hash("canonical"), 1, 2, 3)
	forged := repair83Produce(t, key, hash("substituted"), 1)[0]
	require.NotEqual(t, blocks[0].Hash(), forged.Hash())
	env := repair83Environment(t, blocks...)
	fork := repair83Environment(t, forged)
	var targetRequests atomic.Int64
	p := repair83Provider(t, env, func(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error) {
		if req.Height != nil && int64(*req.Height) == 1 && targetRequests.Add(1) == 1 {
			return fork.Commit(ctx, req)
		}
		return env.Commit(ctx, req)
	})
	trusted := dbs.New(dbm.NewMemDB())
	client := repair83Client(t, p, []provider.Provider{p}, trusted, blocks[2])
	_, err := client.VerifyLightBlockAtHeight(t.Context(), 1, blocks[2].Time)
	var invalid light.ErrInvalidHeader
	require.ErrorAs(t, err, &invalid)
	require.EqualValues(t, 2, targetRequests.Load(), "the provider substituted the target between fetch and verification")
	_, err = client.TrustedLightBlock(1)
	require.ErrorIs(t, err, store.ErrLightBlockNotFound)
	require.EqualValues(t, 1, trusted.Size())
	verified, err := client.VerifyLightBlockAtHeight(t.Context(), 1, blocks[2].Time)
	require.NoError(t, err)
	require.Equal(t, blocks[0].Hash(), verified.Hash())
	saved, err := client.TrustedLightBlock(1)
	require.NoError(t, err)
	require.Equal(t, blocks[0].Hash(), saved.Hash())
}

func TestRepair_83_BackwardLinkedHeaders(t *testing.T) {
	blocks := repair83Produce(t, ed25519.GenerateSecretKey(), hash("canonical"), 1, 2, 3, 4)
	env := repair83Environment(t, blocks...)
	p := repair83Provider(t, env, env.Commit)
	trusted := dbs.New(dbm.NewMemDB())
	client := repair83Client(t, p, []provider.Provider{p}, trusted, blocks[3])
	verified, err := client.VerifyLightBlockAtHeight(t.Context(), 1, blocks[3].Time)
	require.NoError(t, err)
	require.Equal(t, blocks[0].Hash(), verified.Hash())
	require.EqualValues(t, 2, trusted.Size())
	_, err = client.TrustedLightBlock(2)
	require.ErrorIs(t, err, store.ErrLightBlockNotFound)
	_, err = client.TrustedLightBlock(3)
	require.ErrorIs(t, err, store.ErrLightBlockNotFound)
	saved, err := client.TrustedLightBlock(1)
	require.NoError(t, err)
	require.Equal(t, blocks[0].Hash(), saved.Hash())
}

func TestRepair_83_ReopenDivergenceQuarantine(t *testing.T) {
	for _, constructor := range []string{"trusted_store", "new_client"} {
		t.Run(constructor, func(t *testing.T) {
			blocks := repair83Produce(t, ed25519.GenerateSecretKey(), hash("canonical"), 1, 2, 3)
			divergent := repair83Produce(t, ed25519.GenerateSecretKey(), hash("divergent"), 2)[0]
			env := repair83Environment(t, blocks...)
			badEnv := repair83Environment(t, blocks[0], divergent)
			primary := repair83Provider(t, env, env.Commit)
			good := repair83Provider(t, env, env.Commit)
			var badRequests atomic.Int64
			bad := repair83Provider(t, badEnv, func(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error) {
				badRequests.Add(1)
				return badEnv.Commit(ctx, req)
			})
			dir := t.TempDir()
			db, err := dbm.NewDB("trusted", dbm.GoLevelDBBackend, dir)
			require.NoError(t, err)
			client := repair83Client(t, primary, []provider.Provider{good, bad}, dbs.New(db), blocks[0])
			require.NoError(t, db.Close())
			client = nil

			db, err = dbm.NewDB("trusted", dbm.GoLevelDBBackend, dir)
			require.NoError(t, err)
			trusted := dbs.New(db)
			if constructor == "trusted_store" {
				client, err = light.NewClientFromTrustedStore(chainID, 24*time.Hour, primary, []provider.Provider{good, bad}, trusted, time.Hour)
			} else {
				client, err = light.NewClient(t.Context(), chainID, light.TrustOptions{Period: 24 * time.Hour}, primary, []provider.Provider{good, bad}, trusted, time.Hour)
			}
			require.NoError(t, err)
			recovered, err := client.TrustedLightBlock(0)
			require.NoError(t, err)
			require.Equal(t, blocks[0].Hash(), recovered.Hash())
			before := badRequests.Load()
			verified, err := client.VerifyLightBlockAtHeight(t.Context(), 2, blocks[2].Time)
			require.NoError(t, err)
			require.Equal(t, blocks[1].Hash(), verified.Hash())
			require.Greater(t, badRequests.Load(), before, "the real divergence handler must query the witness")
			require.Equal(t, []provider.Provider{good}, client.Witnesses())
			client.AddProvider(bad)
			require.Equal(t, []provider.Provider{good}, client.Witnesses(), "quarantine must prevent re-adding the divergent provider")
			before = badRequests.Load()
			_, err = client.VerifyLightBlockAtHeight(t.Context(), 3, blocks[2].Time)
			require.NoError(t, err)
			require.Equal(t, before, badRequests.Load())
			require.NoError(t, db.Close())
			client = nil

			db, err = dbm.NewDB("trusted", dbm.GoLevelDBBackend, dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			client, err = light.NewClientFromTrustedStore(chainID, 24*time.Hour, primary, []provider.Provider{good}, dbs.New(db), time.Hour)
			require.NoError(t, err)
			for _, block := range blocks {
				recovered, err = client.TrustedLightBlock(block.Height)
				require.NoError(t, err)
				require.Equal(t, block.Hash(), recovered.Hash())
			}
		})
	}
}

func TestRepair_83_RestoredConfiguration(t *testing.T) {
	blocks := repair83Produce(t, ed25519.GenerateSecretKey(), hash("canonical"), 1)
	env := repair83Environment(t, blocks...)
	p := repair83Provider(t, env, env.Commit)
	trusted := dbs.New(dbm.NewMemDB())
	repair83Client(t, p, []provider.Provider{p}, trusted, blocks[0])
	for _, tc := range []struct {
		name      string
		chain     string
		period    time.Duration
		primary   provider.Provider
		witnesses []provider.Provider
		trusted   store.Store
		options   []light.Option
	}{
		{name: "no_witnesses", chain: chainID, period: time.Hour, primary: p, trusted: trusted},
		{name: "zero_period", chain: chainID, primary: p, witnesses: []provider.Provider{p}, trusted: trusted},
		{name: "negative_period", chain: chainID, period: -time.Hour, primary: p, witnesses: []provider.Provider{p}, trusted: trusted},
		{name: "wrong_chain", chain: "other-chain", period: time.Hour, primary: p, witnesses: []provider.Provider{p}, trusted: trusted},
		{name: "nil_primary", chain: chainID, period: time.Hour, witnesses: []provider.Provider{p}, trusted: trusted},
		{name: "nil_store", chain: chainID, period: time.Hour, primary: p, witnesses: []provider.Provider{p}},
		{name: "nil_witness", chain: chainID, period: time.Hour, primary: p, witnesses: []provider.Provider{nil}, trusted: trusted},
		{name: "invalid_trust_level", chain: chainID, period: time.Hour, primary: p, witnesses: []provider.Provider{p}, trusted: trusted, options: []light.Option{light.SkippingVerification(tmmath.Fraction{Numerator: 1, Denominator: 4})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := light.NewClientFromTrustedStore(tc.chain, tc.period, tc.primary, tc.witnesses, tc.trusted, time.Hour, tc.options...)
			require.Error(t, err)
			require.Nil(t, client)
		})
	}
	require.EqualValues(t, 1, trusted.Size())
}
