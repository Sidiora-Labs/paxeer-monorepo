package types_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	clientkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/keeper"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	connection "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection"
	connectionkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	channelkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/keeper"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	tmclient "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	capabilitykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/keeper"
	paramsmodule "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

type repairM1217Chain struct {
	store       *rootmulti.Store
	cache       storetypes.CacheMultiStore
	ctx         sdk.Context
	cdc         *codec.ProtoCodec
	amino       *codec.LegacyAmino
	key         *storetypes.KVStoreKey
	params      paramskeeper.Keeper
	space       paramtypes.Subspace
	clients     clientkeeper.Keeper
	connections connectionkeeper.Keeper
}

func repairM1217Bool(value interface{}) error {
	if _, ok := value.(bool); !ok {
		return fmt.Errorf("expected bool, got %T", value)
	}
	return nil
}

func repairM1217Open(t *testing.T, dir string) *repairM1217Chain {
	t.Helper()
	cfg := config.DefaultStateCommitConfig()
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	ms := rootmulti.NewStore(dir, cfg, config.StateStoreConfig{}, nil)
	key := sdk.NewKVStoreKey(host.StoreKey)
	paramsKey := sdk.NewKVStoreKey("params")
	transientKey := sdk.NewTransientStoreKey("transient_params")
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(paramsKey, sdk.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	clienttypes.RegisterInterfaces(registry)
	tmclient.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	pk := paramskeeper.NewKeeper(cdc, amino, paramsKey, transientKey)
	space := pk.Subspace("connection").WithKeyTable(types.ParamKeyTable().RegisterType(
		paramtypes.NewParamSetPair(connectionkeeper.KeyOutboundEnabled, new(bool), repairM1217Bool),
	))
	clientSpace := pk.Subspace("client").WithKeyTable(clienttypes.ParamKeyTable().RegisterType(
		paramtypes.NewParamSetPair(clientkeeper.KeyInboundEnabled, new(bool), repairM1217Bool),
	))
	ck := clientkeeper.NewKeeper(cdc, key, clientSpace, nil, nil)
	cache := ms.CacheMultiStore()
	h := &repairM1217Chain{
		store: ms, cache: cache, cdc: cdc, amino: amino, key: key, params: pk, space: space, clients: ck,
		ctx:         sdk.NewContext(cache, tmproto.Header{ChainID: "local-0", Height: 10, Time: time.Unix(1000, 0)}, false),
		connections: connectionkeeper.NewKeeper(cdc, key, space, ck),
	}
	t.Cleanup(func() { h.close(t) })
	return h
}

func (h *repairM1217Chain) close(t *testing.T) {
	t.Helper()
	if h.store != nil {
		h.cache.Close()
		require.NoError(t, h.store.Close())
		h.store = nil
	}
}

func (h *repairM1217Chain) commit() storetypes.CommitID {
	h.cache.Write()
	return h.store.Commit(true)
}

func (h *repairM1217Chain) createClient(t *testing.T, root []byte, height clienttypes.Height) string {
	t.Helper()
	clientSpace, found := h.params.GetSubspace("client")
	require.True(t, found)
	clientSpace.Set(h.ctx, clientkeeper.KeyInboundEnabled, true)
	h.clients.SetParams(h.ctx, clienttypes.DefaultParams())
	h.clients.SetNextClientSequence(h.ctx, 0)
	state := tmclient.NewClientState("remote-0", tmclient.DefaultTrustLevel,
		time.Hour, 2*time.Hour, time.Minute, height, commitmenttypes.GetSDKSpecs(), nil, false, false)
	consensus := tmclient.NewConsensusState(h.ctx.BlockTime(), commitmenttypes.NewMerkleRoot(root), make([]byte, 32))
	require.NoError(t, state.Validate())
	require.NoError(t, consensus.ValidateBasic())
	id, err := h.clients.CreateClient(h.ctx, state, consensus)
	require.NoError(t, err)
	return id
}

func repairM1217Counterparty() types.Counterparty {
	return types.NewCounterparty("07-tendermint-1", "", commitmenttypes.NewMerklePrefix([]byte(host.StoreKey)))
}

func repairM1217ProduceGenesis(t *testing.T) (*repairM1217Chain, types.GenesisState) {
	t.Helper()
	h := repairM1217Open(t, t.TempDir())
	connection.InitGenesis(h.ctx, h.connections, types.DefaultGenesisState())
	h.space.Set(h.ctx, connectionkeeper.KeyOutboundEnabled, true)
	id := h.createClient(t, make([]byte, 32), clienttypes.NewHeight(0, 1))
	connectionID, err := h.connections.ConnOpenInit(h.ctx, id, repairM1217Counterparty(), nil, uint64(5*time.Second))
	require.NoError(t, err)
	require.Equal(t, "connection-0", connectionID)
	return h, connection.ExportGenesis(h.ctx, h.connections)
}

func TestRepairM_1217_ConnectionSequenceZero(t *testing.T) {
	_, genesis := repairM1217ProduceGenesis(t)
	require.NoError(t, genesis.Validate())
	require.Len(t, genesis.Connections, 1)
	genesis.NextConnectionSequence = 0
	require.ErrorContains(t, genesis.Validate(), "next connection sequence 0 must be greater than maximum sequence used in connection identifier 0")
	genesis.NextConnectionSequence = 1
	require.NoError(t, genesis.Validate())
	genesis.Connections[0].Id = types.FormatConnectionIdentifier(10)
	genesis.NextConnectionSequence = 10
	require.Error(t, genesis.Validate())
	genesis.NextConnectionSequence = 11
	require.NoError(t, genesis.Validate())
	require.NoError(t, types.DefaultGenesisState().Validate())
}

func TestRepairM_1217_GenesisImportRestartOpen(t *testing.T) {
	source, genesis := repairM1217ProduceGenesis(t)
	wire, err := source.cdc.Marshal(&genesis)
	require.NoError(t, err)
	source.close(t)
	dir := t.TempDir()
	recovered := repairM1217Open(t, dir)
	var decoded types.GenesisState
	require.NoError(t, recovered.cdc.Unmarshal(wire, &decoded))
	require.Equal(t, genesis, decoded)
	require.NoError(t, decoded.Validate())
	id := recovered.createClient(t, make([]byte, 32), clienttypes.NewHeight(0, 1))
	require.Equal(t, decoded.Connections[0].ClientId, id)
	connection.InitGenesis(recovered.ctx, recovered.connections, decoded)
	recovered.space.Set(recovered.ctx, connectionkeeper.KeyOutboundEnabled, true)
	original, found := recovered.connections.GetConnection(recovered.ctx, "connection-0")
	require.True(t, found)
	require.Equal(t, decoded.Connections[0], types.NewIdentifiedConnection("connection-0", original))
	originalBytes := append([]byte(nil), recovered.ctx.KVStore(recovered.key).Get(host.ConnectionKey("connection-0"))...)
	commit := recovered.commit()
	recovered.close(t)

	reopened := repairM1217Open(t, dir)
	require.Equal(t, commit, reopened.store.LastCommitID())
	require.Equal(t, uint64(1), reopened.connections.GetNextConnectionSequence(reopened.ctx))
	require.Equal(t, decoded.Params, reopened.connections.GetParams(reopened.ctx))
	newID, err := reopened.connections.ConnOpenInit(reopened.ctx, id, repairM1217Counterparty(), nil, 0)
	require.NoError(t, err)
	require.Equal(t, "connection-1", newID)
	require.Equal(t, uint64(2), reopened.connections.GetNextConnectionSequence(reopened.ctx))
	retained, found := reopened.connections.GetConnection(reopened.ctx, "connection-0")
	require.True(t, found)
	require.Equal(t, original, retained)
	require.Equal(t, originalBytes, reopened.ctx.KVStore(reopened.key).Get(host.ConnectionKey("connection-0")))
	created, found := reopened.connections.GetConnection(reopened.ctx, newID)
	require.True(t, found)
	require.Equal(t, types.INIT, created.State)
	paths, found := reopened.connections.GetClientConnectionPaths(reopened.ctx, id)
	require.True(t, found)
	require.Equal(t, []string{"connection-0", "connection-1"}, paths)
}

func TestRepairM_1217_ParameterDestination(t *testing.T) {
	h := repairM1217Open(t, t.TempDir())
	untouched := types.NewParams(17)
	h.space.GetParamSetIfExists(h.ctx, &untouched)
	require.Equal(t, uint64(17), untouched.MaxExpectedTimePerBlock)
	require.False(t, h.space.Has(h.ctx, types.KeyMaxExpectedTimePerBlock))
	written := types.NewParams(uint64(7 * time.Second))
	h.space.SetParamSet(h.ctx, &written)
	var read types.Params
	h.space.GetParamSet(h.ctx, &read)
	require.Equal(t, written, read)
	readIfExists := types.NewParams(23)
	h.space.GetParamSetIfExists(h.ctx, &readIfExists)
	require.Equal(t, written, readIfExists)
	require.Equal(t, written, h.connections.GetParams(h.ctx))
}

func TestRepairM_1217_ZeroBlockDelayGovernance(t *testing.T) {
	remote := repairM1217Open(t, t.TempDir())
	channels := channelkeeper.NewKeeper(remote.cdc, remote.key, paramtypes.Subspace{}, remote.clients, remote.connections, nil, capabilitykeeper.ScopedKeeper{})
	packet := channeltypes.NewPacket([]byte("delay regression"), 1, "transfer", "channel-0", "transfer", "channel-1", clienttypes.NewHeight(0, 100), 0)
	commitment := channeltypes.CommitPacket(remote.cdc, packet)
	channels.SetPacketCommitment(remote.ctx, packet.SourcePort, packet.SourceChannel, packet.Sequence, commitment)
	commit := remote.commit()
	response := remote.store.Query(context.Background(), abci.RequestQuery{
		Path: "/" + host.StoreKey + "/key", Data: host.PacketCommitmentKey(packet.SourcePort, packet.SourceChannel, packet.Sequence), Height: commit.Version, Prove: true,
	})
	require.Zero(t, response.Code, response.Log)
	require.Equal(t, commitment, response.Value)
	merkleProof, err := commitmenttypes.ConvertProofs(response.ProofOps)
	require.NoError(t, err)
	proof, err := remote.cdc.Marshal(&merkleProof)
	require.NoError(t, err)
	height := clienttypes.NewHeight(0, uint64(commit.Version)+1)

	h := repairM1217Open(t, t.TempDir())
	id := h.createClient(t, commit.Hash, height)
	h.connections.SetParams(h.ctx, types.NewParams(uint64(2*time.Second)))
	conn := types.NewConnectionEnd(types.OPEN, id, repairM1217Counterparty(), types.ExportedVersionsToProto(types.GetCompatibleVersions()), uint64(5*time.Second))
	verify := func(ctx sdk.Context, value []byte) error {
		return h.connections.VerifyPacketCommitment(ctx, conn, height, proof, packet.SourcePort, packet.SourceChannel, packet.Sequence, value)
	}
	atTwoBlocks := h.ctx.WithBlockTime(h.ctx.BlockTime().Add(5 * time.Second)).WithBlockHeight(h.ctx.BlockHeight() + 2)
	require.ErrorIs(t, verify(atTwoBlocks, commitment), tmclient.ErrDelayPeriodNotPassed)
	require.NoError(t, verify(atTwoBlocks.WithBlockHeight(h.ctx.BlockHeight()+3), commitment))

	before := append([]byte(nil), h.space.GetRaw(h.ctx, types.KeyMaxExpectedTimePerBlock)...)
	zero := types.NewParams(0)
	require.Error(t, zero.Validate())
	require.Error(t, h.space.Validate(h.ctx, types.KeyMaxExpectedTimePerBlock, uint64(0)))
	require.Error(t, h.space.Validate(h.ctx, types.KeyMaxExpectedTimePerBlock, int64(1)))
	require.Panics(t, func() { h.space.SetParamSet(h.ctx, &zero) })
	require.Error(t, h.space.Update(h.ctx, types.KeyMaxExpectedTimePerBlock, h.amino.MustMarshalJSON(uint64(0))))
	handler := paramsmodule.NewParamChangeProposalHandler(h.params)
	change := func(value uint64) *proposal.ParameterChangeProposal {
		return proposal.NewParameterChangeProposal("Connection timing", "Update expected block time", []proposal.ParamChange{
			proposal.NewParamChange(h.space.Name(), string(types.KeyMaxExpectedTimePerBlock), string(h.amino.MustMarshalJSON(value))),
		}, false)
	}
	require.Error(t, handler(h.ctx, change(0)))
	require.Equal(t, before, h.space.GetRaw(h.ctx, types.KeyMaxExpectedTimePerBlock))
	require.ErrorIs(t, verify(atTwoBlocks, commitment), tmclient.ErrDelayPeriodNotPassed)
	require.NoError(t, handler(h.ctx, change(uint64(5*time.Second))))
	var updated types.Params
	h.space.GetParamSet(h.ctx, &updated)
	require.Equal(t, uint64(5*time.Second), updated.MaxExpectedTimePerBlock)
	require.NoError(t, updated.Validate())
	require.NoError(t, verify(atTwoBlocks, commitment))
	require.ErrorIs(t, verify(atTwoBlocks.WithBlockTime(atTwoBlocks.BlockTime().Add(-time.Nanosecond)), commitment), tmclient.ErrDelayPeriodNotPassed)
	require.ErrorIs(t, verify(atTwoBlocks.WithBlockHeight(h.ctx.BlockHeight()), commitment), tmclient.ErrDelayPeriodNotPassed)
	require.NoError(t, verify(atTwoBlocks.WithBlockHeight(h.ctx.BlockHeight()+1), commitment))
	require.Error(t, verify(atTwoBlocks, []byte("different commitment")))
}
