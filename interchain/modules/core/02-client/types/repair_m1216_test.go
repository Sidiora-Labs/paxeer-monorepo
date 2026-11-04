package types_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	client "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client"
	clientkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	coretypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
	ibctmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	localhosttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/09-localhost/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

// Task 22.33.1216 repair record:
// client-sequence-zero: TestRepairM_1216_SequenceZeroValidation and
// TestRepairM_1216_ImportRestartAllocation preserve imported state across recovery.
// client-parameter-destination: TestRepairM_1216_ParameterDestination exercises
// SetParamSet, GetParamSet and GetParamSetIfExists through the SDK store and codec.
// Baseline: not run; the controlling gate order forbids verification before the
// complete implementation and forbids invoking verify_cmd outside hf-done.
// No pre-change test failure is claimed. No out-of-path wiring is required.
// Qualification repair: Amino recovers an empty list as a nil slice; assert its
// exact representation as well as its stored bytes and allowlist behavior.

type repairM1216Store struct {
	root   *rootmulti.Store
	ctx    sdk.Context
	keeper clientkeeper.Keeper
	params paramtypes.Subspace
	cdc    *codec.ProtoCodec
}

func repairM1216Open(t *testing.T, dir string) *repairM1216Store {
	t.Helper()
	db, err := dbm.NewGoLevelDB("client", dir)
	require.NoError(t, err)
	root := rootmulti.NewStore(db)
	clientKey := sdk.NewKVStoreKey(host.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	root.MountStoreWithDB(clientKey, sdk.StoreTypeIAVL, nil)
	root.MountStoreWithDB(paramsKey, sdk.StoreTypeIAVL, nil)
	root.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, root.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	ibctmtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	table := types.ParamKeyTable().RegisterParamSet(&coretypes.Params{})
	params := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), paramsKey, transientKey, host.StoreKey).WithKeyTable(table)
	ctx := sdk.NewContext(root.CacheMultiStore(), tmproto.Header{
		ChainID: "recovery-1", Height: root.LastCommitID().Version + 1,
		Time: time.Unix(1700000000, 0).UTC(),
	}, false)
	return &repairM1216Store{
		root: root, ctx: ctx, params: params, cdc: cdc,
		// Import, export and creation do not use staking or upgrade keepers.
		keeper: clientkeeper.NewKeeper(cdc, clientKey, params, nil, nil),
	}
}

func (s *repairM1216Store) commit(t *testing.T) {
	t.Helper()
	s.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	commit := s.root.Commit(true)
	require.Positive(t, commit.Version)
}

func repairM1216Client(height uint64) (*ibctmtypes.ClientState, *ibctmtypes.ConsensusState) {
	state := ibctmtypes.NewClientState(
		"counterparty-1", ibctmtypes.DefaultTrustLevel, time.Hour, 2*time.Hour,
		time.Minute, types.NewHeight(1, height), commitmenttypes.GetSDKSpecs(),
		[]string{"upgrade", "upgradedIBCState"}, false, false,
	)
	consensus := ibctmtypes.NewConsensusState(time.Unix(1699999900, 0).UTC(),
		commitmenttypes.NewMerkleRoot(bytes.Repeat([]byte{byte(height)}, 32)),
		bytes.Repeat([]byte{1}, 32))
	return state, consensus
}

func TestRepairM_1216_SequenceZeroValidation(t *testing.T) {
	state, _ := repairM1216Client(10)
	require.NoError(t, state.Validate())
	for _, tc := range []struct {
		name      string
		sequences []uint64
		next      uint64
		valid     bool
	}{
		{"empty-zero", nil, 0, true},
		{"zero-collision", []uint64{0}, 0, false},
		{"zero-successor", []uint64{0}, 1, true},
		{"below-maximum", []uint64{0, 2}, 1, false},
		{"equal-maximum", []uint64{0, 2}, 2, false},
		{"above-maximum", []uint64{0, 2}, 3, true},
		{"maximum-uint64", []uint64{math.MaxUint64}, math.MaxUint64, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs := types.DefaultGenesisState()
			gs.NextClientSequence = tc.next
			for _, sequence := range tc.sequences {
				gs.Clients = append(gs.Clients, types.NewIdentifiedClientState(
					types.FormatClientIdentifier(state.ClientType(), sequence), state))
			}
			if tc.valid {
				require.NoError(t, gs.Validate())
			} else {
				require.EqualError(t, gs.Validate(), fmt.Sprintf(
					"next client identifier sequence %d must be greater than the maximum sequence used in the provided client identifiers %d",
					tc.next, tc.sequences[len(tc.sequences)-1]))
			}
		})
	}
	gs := types.DefaultGenesisState()
	gs.Params = types.NewParams(exported.Tendermint, exported.Localhost)
	gs.Clients = []types.IdentifiedClientState{
		types.NewIdentifiedClientState(types.FormatClientIdentifier(exported.Tendermint, 0), state),
		types.NewIdentifiedClientState(types.FormatClientIdentifier(exported.Localhost, 0),
			localhosttypes.NewClientState("local-1", types.NewHeight(1, 10))),
	}
	require.ErrorContains(t, gs.Validate(), "must be greater than the maximum sequence")
	gs.NextClientSequence = 1
	require.NoError(t, gs.Validate())
}

func TestRepairM_1216_ImportRestartAllocation(t *testing.T) {
	source := repairM1216Open(t, t.TempDir())
	defer func() { require.NoError(t, source.root.Close()) }()
	client.InitGenesis(source.ctx, source.keeper, types.DefaultGenesisState())
	coreParams := coretypes.DefaultParams()
	source.params.SetParamSet(source.ctx, &coreParams)
	original, consensus := repairM1216Client(10)
	require.NoError(t, original.Validate())
	require.NoError(t, consensus.ValidateBasic())
	oldID, err := source.keeper.CreateClient(source.ctx, original, consensus)
	require.NoError(t, err)
	require.Equal(t, types.FormatClientIdentifier(exported.Tendermint, 0), oldID)
	exportedGenesis := client.ExportGenesis(source.ctx, source.keeper)
	require.Len(t, exportedGenesis.Clients, 1)
	require.Len(t, exportedGenesis.ClientsMetadata, 1)
	require.NotEmpty(t, exportedGenesis.ClientsMetadata[0].ClientMetadata)
	wire, err := source.cdc.MarshalAsJSON(&exportedGenesis)
	require.NoError(t, err)

	dir := t.TempDir()
	recovered := repairM1216Open(t, dir)
	defer func() { require.NoError(t, recovered.root.Close()) }()
	var imported types.GenesisState
	require.NoError(t, recovered.cdc.UnmarshalAsJSON(wire, &imported))
	require.Equal(t, uint64(1), imported.NextClientSequence)
	imported.NextClientSequence = 0
	require.EqualError(t, imported.Validate(), "next client identifier sequence 0 must be greater than the maximum sequence used in the provided client identifiers 0")
	imported.NextClientSequence = exportedGenesis.NextClientSequence
	require.NoError(t, imported.Validate())
	client.InitGenesis(recovered.ctx, recovered.keeper, imported)
	recovered.params.SetParamSet(recovered.ctx, &coreParams)
	originalBytes := append([]byte(nil), recovered.keeper.ClientStore(recovered.ctx, oldID).Get(host.ClientStateKey())...)
	recovered.commit(t)
	require.NoError(t, recovered.root.Close())
	recovered = repairM1216Open(t, dir)
	require.Equal(t, uint64(1), recovered.keeper.GetNextClientSequence(recovered.ctx))
	before, found := recovered.keeper.GetClientState(recovered.ctx, oldID)
	require.True(t, found)
	require.Equal(t, original, before)

	nextState, nextConsensus := repairM1216Client(20)
	require.NoError(t, nextState.Validate())
	require.NoError(t, nextConsensus.ValidateBasic())
	newID, err := recovered.keeper.CreateClient(recovered.ctx, nextState, nextConsensus)
	require.NoError(t, err)
	require.Equal(t, types.FormatClientIdentifier(exported.Tendermint, 1), newID)
	require.NotEqual(t, oldID, newID)
	recovered.commit(t)
	require.NoError(t, recovered.root.Close())
	recovered = repairM1216Open(t, dir)
	require.Equal(t, uint64(2), recovered.keeper.GetNextClientSequence(recovered.ctx))
	require.Equal(t, originalBytes, recovered.keeper.ClientStore(recovered.ctx, oldID).Get(host.ClientStateKey()))
	for id, expected := range map[string]*ibctmtypes.ClientState{oldID: original, newID: nextState} {
		actual, found := recovered.keeper.GetClientState(recovered.ctx, id)
		require.True(t, found)
		require.Equal(t, expected, actual)
	}
	oldConsensus, found := recovered.keeper.GetClientConsensusState(recovered.ctx, oldID, original.GetLatestHeight())
	require.True(t, found)
	require.Equal(t, consensus, oldConsensus)
	newStoredConsensus, found := recovered.keeper.GetClientConsensusState(recovered.ctx, newID, nextState.GetLatestHeight())
	require.True(t, found)
	require.Equal(t, nextConsensus, newStoredConsensus)
	for _, metadata := range imported.ClientsMetadata[0].ClientMetadata {
		require.Equal(t, metadata.Value, recovered.keeper.ClientStore(recovered.ctx, oldID).Get(metadata.Key))
	}
	require.NoError(t, client.ExportGenesis(recovered.ctx, recovered.keeper).Validate())
}

func TestRepairM_1216_ParameterDestination(t *testing.T) {
	s := repairM1216Open(t, t.TempDir())
	defer func() { require.NoError(t, s.root.Close()) }()
	require.False(t, s.params.Has(s.ctx, types.KeyAllowedClients))
	var absent types.Params
	s.params.GetParamSetIfExists(s.ctx, &absent)
	require.Nil(t, absent.AllowedClients)
	untouched := types.NewParams(exported.Localhost)
	s.params.GetParamSetIfExists(s.ctx, &untouched)
	require.Equal(t, []string{exported.Localhost}, untouched.AllowedClients)
	require.False(t, s.params.Has(s.ctx, types.KeyAllowedClients))
	for _, tc := range []struct {
		name    string
		allowed []string
		decoded []string
	}{
		{"both-clients", []string{exported.Tendermint, exported.Solomachine}, []string{exported.Tendermint, exported.Solomachine}},
		{"one-client", []string{exported.Solomachine}, []string{exported.Solomachine}},
		{"empty-list", []string{}, nil},
		{"nil-list", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			written := types.Params{AllowedClients: tc.allowed}
			expected := types.Params{AllowedClients: tc.decoded}
			require.NoError(t, written.Validate())
			s.params.SetParamSet(s.ctx, &written)
			require.True(t, s.params.Has(s.ctx, types.KeyAllowedClients))
			wire, err := codec.NewLegacyAmino().MarshalAsJSON(written.AllowedClients)
			require.NoError(t, err)
			require.Equal(t, wire, s.params.GetRaw(s.ctx, types.KeyAllowedClients))
			var fresh types.Params
			s.params.GetParamSet(s.ctx, &fresh)
			require.Equal(t, expected, fresh)
			existing := types.NewParams(exported.Localhost)
			s.params.GetParamSetIfExists(s.ctx, &existing)
			require.Equal(t, expected, existing)
			require.Equal(t, expected, s.keeper.GetParams(s.ctx))
			for _, clientType := range []string{exported.Tendermint, exported.Solomachine, exported.Localhost} {
				require.Equal(t, written.IsAllowedClient(clientType), fresh.IsAllowedClient(clientType))
				require.Equal(t, written.IsAllowedClient(clientType), existing.IsAllowedClient(clientType))
			}
		})
	}
}
