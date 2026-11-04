package module_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/genesis"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Task 22.31.295 record: stream-import-invalid-lines is proved by
// TestRepair_295_RoundTripRestartRecovery, TestRepair_295_RejectCorruptionAtomically,
// TestRepair_295_RejectIncompleteTransport, and TestRepair_295_RejectSourceErrors.
// No out-of-path wiring is required.
// The inherited attempt failed in RejectCorruptionAtomically while constructing
// missing_data; RoundTripRestartRecovery and RejectSourceErrors passed there.
// The explicit implementation-first gate order forbids the embedded baseline
// verification step; no unchanged-tree test result is claimed.

type repair295State struct {
	ctx     sdk.Context
	cms     storetypes.CommitMultiStore
	cdc     *codec.ProtoCodec
	params  paramskeeper.Keeper
	auth    authkeeper.AccountKeeper
	manager *module.Manager
	keys    []sdk.StoreKey
	closed  bool
}

func repair295Open(t *testing.T, directory string) *repair295State {
	t.Helper()
	db, err := dbm.NewDB("repair295", dbm.GoLevelDBBackend, directory)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	paramsKey := sdk.NewKVStoreKey(paramstypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramstypes.TStoreKey)
	cms.MountStoreWithDB(paramsKey, sdk.StoreTypeIAVL, nil)
	cms.MountStoreWithDB(authKey, sdk.StoreTypeIAVL, nil)
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	pk := paramskeeper.NewKeeper(cdc, amino, paramsKey, transientKey)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil})
	mm := module.NewManager(auth.NewAppModule(cdc, ak, nil), params.NewAppModule(pk))
	mm.SetOrderInitGenesis(paramstypes.ModuleName, authtypes.ModuleName)
	mm.SetOrderExportGenesis(authtypes.ModuleName, paramstypes.ModuleName)
	state := &repair295State{
		ctx: sdk.NewContext(cms, tmproto.Header{ChainID: "repair295", Height: 1}, false),
		cms: cms, cdc: cdc, params: pk, auth: ak, manager: mm,
		keys: []sdk.StoreKey{paramsKey, authKey, transientKey},
	}
	t.Cleanup(func() { state.close(t) })
	return state
}

func (s *repair295State) close(t *testing.T) {
	t.Helper()
	if !s.closed {
		require.NoError(t, s.cms.Close())
		s.closed = true
	}
}

func repair295Address(value byte) sdk.AccAddress {
	return sdk.AccAddress(bytes.Repeat([]byte{value}, 20))
}

func (s *repair295State) seed(account byte, denominator uint64) {
	pg := paramstypes.DefaultGenesis()
	pg.FeesParams.GlobalMinimumGasPrices = sdk.DecCoins{sdk.NewDecCoinFromDec("uhpx", sdk.NewDecWithPrec(125, 3))}
	pg.CosmosGasParams.CosmosGasMultiplierDenominator = denominator
	ag := authtypes.NewGenesisState(authtypes.DefaultParams(), authtypes.GenesisAccounts{
		authtypes.NewBaseAccount(repair295Address(account), nil, 0, uint64(account)),
	})
	s.manager.InitGenesis(s.ctx, s.cdc, map[string]json.RawMessage{
		paramstypes.ModuleName: s.cdc.MustMarshalJSON(pg),
		authtypes.ModuleName:   s.cdc.MustMarshalJSON(ag),
	}, genesis.GenesisImportConfig{})
}

func (s *repair295State) records(t *testing.T) []string {
	t.Helper()
	var records []string
	require.NoError(t, s.manager.ProcessGenesisPerModule(s.ctx, s.cdc, func(name string, data json.RawMessage) error {
		record, err := json.Marshal(module.ModuleState{AppState: module.AppState{Module: name, Data: data}})
		if err == nil {
			records = append(records, string(record))
		}
		return err
	}))
	return records
}

func repair295Records(t *testing.T) []string {
	t.Helper()
	first := repair295Open(t, t.TempDir())
	first.seed(1, 9)
	second := repair295Open(t, t.TempDir())
	second.seed(2, 9)
	firstRecords := first.records(t)
	secondRecords := second.records(t)
	require.Len(t, firstRecords, 2)
	require.Len(t, secondRecords, 2)
	// Both adjacent auth chunks come from real module exports, with disjoint
	// user accounts so retaining both chunks is observable after reopening.
	header, err := json.Marshal(server.GenesisDocNoAppState{
		GenesisTime: time.Unix(1700000000, 0).UTC(), ChainID: "repair295", InitialHeight: 1,
		ConsensusParams: tmtypes.DefaultConsensusParams(),
	})
	require.NoError(t, err)
	first.close(t)
	second.close(t)
	return []string{firstRecords[0], firstRecords[1], secondRecords[1], string(header)}
}

func repair295File(t *testing.T, records []string, trailingNewline bool) string {
	t.Helper()
	data := strings.Join(records, "\n")
	if trailingNewline {
		data += "\n"
	}
	filename := filepath.Join(t.TempDir(), "genesis.jsonl")
	require.NoError(t, os.WriteFile(filename, []byte(data), 0o600))
	return filename
}

func (s *repair295State) importFile(filename string) {
	s.manager.InitGenesis(s.ctx, s.cdc, nil, genesis.GenesisImportConfig{
		StreamGenesisImport: true, GenesisStreamFile: filename,
	})
}

func (s *repair295State) snapshot(t *testing.T) map[string]map[string]string {
	t.Helper()
	result := make(map[string]map[string]string)
	for _, key := range s.keys {
		values := make(map[string]string)
		iterator := s.ctx.MultiStore().GetKVStore(key).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			values[string(iterator.Key())] = string(iterator.Value())
		}
		require.NoError(t, iterator.Close())
		result[key.Name()] = values
	}
	return result
}

func TestRepair_295_RoundTripRestartRecovery(t *testing.T) {
	records := repair295Records(t)
	for _, trailingNewline := range []bool{false, true} {
		name := "metadata_without_newline"
		if trailingNewline {
			name = "metadata_with_newline"
		}
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			target := repair295Open(t, directory)
			require.NotPanics(t, func() { target.importFile(repair295File(t, records, trailingNewline)) })
			target.cms.Commit(true)
			before := target.snapshot(t)
			target.close(t)
			recovered := repair295Open(t, directory)
			require.Equal(t, before, recovered.snapshot(t))
			for _, account := range []byte{1, 2} {
				got := recovered.auth.GetAccount(recovered.ctx, repair295Address(account))
				require.NotNil(t, got, "account from chunk %d must survive import and restart", account)
				require.Equal(t, uint64(account), got.GetSequence())
			}
			require.Equal(t, uint64(9), recovered.params.GetCosmosGasParams(recovered.ctx).CosmosGasMultiplierDenominator)
			require.Equal(t, "0.125000000000000000uhpx", recovered.params.GetFeesParams(recovered.ctx).GlobalMinimumGasPrices.String())
		})
	}
}

func repair295Replace(t *testing.T, record string, path []string, value json.RawMessage) string {
	t.Helper()
	var fields map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(record))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&fields))
	parent := fields
	for _, name := range path[:len(path)-1] {
		child, ok := parent[name].(map[string]interface{})
		require.True(t, ok, "fixture field %q must be an object", name)
		parent = child
	}
	name := path[len(path)-1]
	if len(value) == 0 {
		delete(parent, name)
	} else {
		require.True(t, json.Valid(value), "replacement must be valid JSON")
		parent[name] = value
	}
	result, err := json.Marshal(fields)
	require.NoError(t, err)
	return string(result)
}

func TestRepair_295_RejectCorruptionAtomically(t *testing.T) {
	r := repair295Records(t)
	replace := func(record string, path []string, value string) string {
		return repair295Replace(t, record, path, json.RawMessage(value))
	}
	cases := map[string][]string{
		"empty":                        {},
		"malformed_first":              {"{", r[1], r[2], r[3]},
		"malformed_after_valid_chunks": {r[0], r[1], "{", r[2], r[3]},
		"unknown_module":               {r[0], r[1], replace(r[2], []string{"app_state", "module"}, `"crisis"`), r[3]},
		"empty_module":                 {r[0], r[1], replace(r[2], []string{"app_state", "module"}, `""`), r[3]},
		"null_state":                   {r[0], r[1], `{"app_state":null}`, r[3]},
		"missing_data":                 {r[0], r[1], replace(r[2], []string{"app_state", "data"}, ""), r[3]},
		"null_data":                    {r[0], r[1], replace(r[2], []string{"app_state", "data"}, "null"), r[3]},
		"scalar_data":                  {r[0], r[1], replace(r[2], []string{"app_state", "data"}, `"bad"`), r[3]},
		"array_data":                   {r[0], r[1], replace(r[2], []string{"app_state", "data"}, `[]`), r[3]},
		"extra_record_field":           {r[0], r[1], replace(r[2], []string{"unexpected"}, `true`), r[3]},
		"extra_state_field":            {r[0], r[1], replace(r[2], []string{"app_state", "unexpected"}, `true`), r[3]},
		"duplicate_state_field":        {r[0], r[1], `{"app_state":{"module":"auth","module":"auth","data":{}}}`, r[3]},
		"duplicate_record_field":       {r[0], r[1], `{"app_state":{},"app_state":{}}`, r[3]},
		"trailing_json":                {r[0], r[1], r[2] + `{}`, r[3]},
		"blank_record":                 {r[0], r[1], "", r[2], r[3]},
		"reordered_groups":             {r[1], r[2], r[0], r[3]},
		"repeated_group":               {r[0], r[1], r[0], r[2], r[3]},
		"missing_first_group":          {r[1], r[2], r[3]},
		"missing_last_group":           {r[0], r[3]},
		"missing_metadata":             {r[0], r[1], r[2]},
		"invalid_metadata":             {r[0], r[1], r[2], `{}`},
		"missing_header_field":         {r[0], r[1], r[2], replace(r[3], []string{"initial_height"}, "")},
		"null_header_field":            {r[0], r[1], r[2], replace(r[3], []string{"chain_id"}, "null")},
		"empty_chain_id":               {r[0], r[1], r[2], replace(r[3], []string{"chain_id"}, `""`)},
		"negative_height":              {r[0], r[1], r[2], replace(r[3], []string{"initial_height"}, `"-1"`)},
		"invalid_time":                 {r[0], r[1], r[2], replace(r[3], []string{"genesis_time"}, `"0001-01-01T00:00:00Z"`)},
		"unknown_metadata_field":       {r[0], r[1], r[2], replace(r[3], []string{"unexpected"}, `true`)},
		"truncated_metadata":           {r[0], r[1], r[2], r[3][:len(r[3])-1]},
		"duplicate_metadata":           {r[0], r[1], r[2], r[3], r[3]},
		"data_after_metadata":          {r[0], r[1], r[2], r[3], r[2]},
		"metadata_before_groups":       {r[3], r[0], r[1], r[2]},
		"invalid_module_state":         {replace(r[0], []string{"app_state", "data", "cosmos_gas_params", "cosmos_gas_multiplier_denominator"}, `"0"`), r[1], r[2], r[3]},
	}
	for name, records := range cases {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			target := repair295Open(t, directory)
			target.seed(7, 3)
			target.cms.Commit(true)
			before := target.snapshot(t)
			events := append(sdk.Events{}, target.ctx.EventManager().Events()...)
			require.Panics(t, func() { target.importFile(repair295File(t, records, false)) })
			require.Equal(t, before, target.snapshot(t), "a rejected stream must not publish any store writes")
			require.Equal(t, events, target.ctx.EventManager().Events())
			target.cms.Commit(true)
			target.close(t)
			recovered := repair295Open(t, directory)
			require.Equal(t, before, recovered.snapshot(t), "rejection must preserve state across restart")
			require.Equal(t, uint64(3), recovered.params.GetCosmosGasParams(recovered.ctx).CosmosGasMultiplierDenominator)
			require.NotNil(t, recovered.auth.GetAccount(recovered.ctx, repair295Address(7)))
			require.Nil(t, recovered.auth.GetAccount(recovered.ctx, repair295Address(1)))
			require.Nil(t, recovered.auth.GetAccount(recovered.ctx, repair295Address(2)))
		})
	}
}

func TestRepair_295_RejectSourceErrors(t *testing.T) {
	for _, name := range []string{"missing_file", "unreadable_directory"} {
		t.Run(name, func(t *testing.T) {
			target := repair295Open(t, t.TempDir())
			target.seed(7, 3)
			before := target.snapshot(t)
			filename := t.TempDir()
			if name == "missing_file" {
				filename = filepath.Join(filename, "missing.jsonl")
			}
			require.Panics(t, func() { target.importFile(filename) })
			require.Equal(t, before, target.snapshot(t))
		})
	}
}

func TestRepair_295_RejectIncompleteTransport(t *testing.T) {
	records := repair295Records(t)
	directory := t.TempDir()
	target := repair295Open(t, directory)
	target.seed(7, 3)
	target.cms.Commit(true)
	before := target.snapshot(t)
	events := append(sdk.Events{}, target.ctx.EventManager().Events()...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reader, writer := io.Pipe()
	stream := genesis.NewStream(ctx, reader)
	defer stream.Close()
	failure := errors.New("genesis transport interrupted after metadata")
	producerDone := make(chan error, 1)
	go func() {
		_, err := io.WriteString(writer, strings.Join(records, "\n")+"\n")
		_ = writer.CloseWithError(failure)
		producerDone <- err
	}()
	var caught interface{}
	func() {
		defer func() { caught = recover() }()
		target.manager.InitGenesisStream(target.ctx, target.cdc, stream)
	}()
	err, ok := caught.(error)
	require.True(t, ok, "transport failure must panic with an error, got %v", caught)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, stream.Wait(), failure)
	require.NoError(t, <-producerDone)
	require.Equal(t, before, target.snapshot(t))
	require.Equal(t, events, target.ctx.EventManager().Events())
	target.cms.Commit(true)
	target.close(t)
	recovered := repair295Open(t, directory)
	require.Equal(t, before, recovered.snapshot(t))
	require.Equal(t, uint64(3), recovered.params.GetCosmosGasParams(recovered.ctx).CosmosGasMultiplierDenominator)
	require.NotNil(t, recovered.auth.GetAccount(recovered.ctx, repair295Address(7)))
	require.Nil(t, recovered.auth.GetAccount(recovered.ctx, repair295Address(1)))
	require.Nil(t, recovered.auth.GetAccount(recovered.ctx, repair295Address(2)))
}
