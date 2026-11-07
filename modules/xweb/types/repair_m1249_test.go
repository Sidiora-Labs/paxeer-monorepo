package types_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	pxweb "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// This fixture constructs production keepers directly. No application VM,
// external process, endpoint or native guest artifact is started by the tests.
type repair1249Store struct {
	home     string
	store    sdk.CommitMultiStore
	ctx      sdk.Context
	keys     map[string]*sdk.KVStoreKey
	registry codectypes.InterfaceRegistry
	cdc      *codec.ProtoCodec
	bank     bankkeeper.BaseKeeper
	evm      *evmkeeper.Keeper
	upgrades upgradekeeper.Keeper
	web      keeper.Keeper
	closed   bool
}

func repair1249Open(t *testing.T, home string, initialize bool) *repair1249Store {
	t.Helper()
	if _, exists := sdk.GetDenomUnit(types.ProgramFeeDenomPAX); !exists {
		require.NoError(t, sdk.RegisterDenom(types.ProgramFeeDenomPAX, sdk.OneDec()))
	}
	require.Equal(t, types.ProgramFeeDenomPAX, sdk.MustGetBaseDenom())
	db, err := dbm.NewGoLevelDB("state", home)
	require.NoError(t, err)
	s := &repair1249Store{home: home, store: store.NewCommitMultiStore(db), keys: map[string]*sdk.KVStoreKey{}, registry: codectypes.NewInterfaceRegistry()}
	for _, name := range []string{types.StoreKey, "auth", "bank", "evm", "params", "upgrade"} {
		s.keys[name] = sdk.NewKVStoreKey(name)
		s.store.MountStoreWithDB(s.keys[name], sdk.StoreTypeDB, nil)
	}
	paramsTransient := sdk.NewTransientStoreKey("params_transient")
	evmTransient := sdk.NewTransientStoreKey("evm_transient")
	s.store.MountStoreWithDB(paramsTransient, sdk.StoreTypeTransient, nil)
	s.store.MountStoreWithDB(evmTransient, sdk.StoreTypeTransient, nil)
	require.NoError(t, s.store.LoadLatestVersion())
	s.ctx = sdk.NewContext(s.store.CacheMultiStore(), tmproto.Header{ChainID: "pax-test", Height: s.store.LastCommitID().Version + 1,
		Time: time.Unix(1800000000, 0)}, false).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	authtypes.RegisterInterfaces(s.registry)
	types.RegisterInterfaces(s.registry)
	s.cdc = codec.NewProtoCodec(s.registry)
	amino := codec.NewLegacyAmino()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(s.cdc, amino, s.keys["params"], paramsTransient, name)
	}
	accounts := authkeeper.NewAccountKeeper(s.cdc, s.keys["auth"], subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{"repair1249": {authtypes.Minter}, authtypes.FeeCollectorName: nil})
	s.bank = bankkeeper.NewBaseKeeper(s.cdc, s.keys["bank"], accounts, subspace("bank"), nil)
	s.upgrades = upgradekeeper.NewKeeper(nil, s.keys["upgrade"], s.cdc, home, nil)
	s.evm = evmkeeper.NewKeeper(s.keys["evm"], evmTransient, subspace("evm"), nil, s.bank, &accounts,
		nil, transferkeeper.Keeper{}, nil, nil, &s.upgrades)
	s.web = keeper.NewKeeper(s.keys[types.StoreKey], s.bank, s.evm)
	if initialize {
		s.bank.InitGenesis(s.ctx, banktypes.DefaultGenesisState())
		s.evm.SetParams(s.ctx, evmtypes.DefaultParams())
		s.upgrades.SetDone(s.ctx, keeper.ProgramOriginPlan)
		g := types.DefaultGenesis()
		g.Paused = false
		g.Params.KernelNetworkId = 125
		g.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX, types.ProgramFeeDenomSID, types.ProgramFeeDenomUSDC, types.ProgramFeeDenomUSDL}
		g.Attestors = xwebtestutil.Set(xwebtestutil.Attestors(3), 3)
		s.web.InitGenesis(s.ctx, *g)
	}
	t.Cleanup(func() { s.close(t) })
	return s
}

func (s *repair1249Store) close(t *testing.T) {
	t.Helper()
	if !s.closed {
		require.NoError(t, s.store.Close())
		s.closed = true
	}
}

func (s *repair1249Store) commit() {
	s.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	s.store.Commit(true)
	s.ctx = s.ctx.WithMultiStore(s.store.CacheMultiStore()).WithBlockHeight(s.store.LastCommitID().Version + 1)
}

func (s *repair1249Store) client() types.QueryClient {
	router := baseapp.NewQueryServerTestHelper(s.ctx, s.registry)
	xweb.NewAppModule(s.web).RegisterServices(module.NewConfigurator(s.cdc, router, router))
	return types.NewQueryClient(router)
}

func (s *repair1249Store) fund(t *testing.T, account sdk.AccAddress, denom string, amount int64) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewInt64Coin(denom, amount))
	require.NoError(t, s.bank.MintCoins(s.ctx, "repair1249", coins))
	require.NoError(t, s.bank.SendCoinsFromModuleToAccount(s.ctx, "repair1249", account, coins))
}

func repair1249Program() types.Hash32 { return types.Hash32{0x49} }
func repair1249Payload() []byte       { return []byte("https://example.test/repair1249") }

func (s *repair1249Store) request(t *testing.T, id uint64, denom string) {
	t.Helper()
	program, asset := repair1249Program(), types.Hash32{byte(id), 0x49}
	principal := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	s.fund(t, principal, denom, 7)
	data := binary.BigEndian.AppendUint64(nil, id)
	data = append(data, types.KindFetch)
	data = binary.BigEndian.AppendUint32(data, uint32(len(repair1249Payload())))
	data = append(data, repair1249Payload()...)
	branch, write := s.ctx.CacheContext()
	require.NoError(t, s.bank.SendCoins(branch, principal, keeper.ProgramFeeAccount(asset), sdk.NewCoins(sdk.NewInt64Coin(denom, 7))))
	call := &keeper.ProgramCall{ProgramID: program, Principal: principal, Sequence: id,
		Legs: []keeper.ProgramLeg{{From: principal, To: keeper.ProgramFeeAccount(asset), Asset: asset, Denom: denom, Amount: sdk.NewInt(7)}}}
	require.NoError(t, s.web.RequestFromProgram(branch, call, []byte(keeper.ProgramRequestTopic), data))
	write()
}

func repair1249Observation(t *testing.T, id uint64, response []byte) (types.Observation, []byte) {
	t.Helper()
	observation := types.Observation{Origin: types.OriginProgram, NetworkID: 125, ProgramID: repair1249Program(), RequestID: id,
		Kind: types.KindFetch, PayloadHash: types.Keccak(repair1249Payload()), ContentDigest: types.Keccak(response),
		FullLength: uint32(len(response)), Response: response}
	digest := types.Digest(types.Attestation{Origin: observation.Origin, NetworkID: big.NewInt(125), Requester: observation.ProgramID,
		RequestID: id, Kind: observation.Kind, PayloadHash: observation.PayloadHash, ContentDigest: observation.ContentDigest,
		ResponseHash: types.Keccak(response), FullLength: observation.FullLength})
	observation.Signatures = xwebtestutil.Sign(digest, xwebtestutil.Attestors(3)...)
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	return observation, raw
}

func repair1249JSON(t *testing.T, g types.GenesisState) []byte {
	t.Helper()
	raw, err := json.Marshal(g)
	require.NoError(t, err)
	return raw
}

// Export through the module, stop the producer, import both its actual bank
// ledger and web JSON, then close and reopen every store and keeper.
func repair1249Recover(t *testing.T, source *repair1249Store) *repair1249Store {
	t.Helper()
	webJSON := xweb.NewAppModule(source.web).ExportGenesis(source.ctx, source.cdc)
	bankJSON, err := json.Marshal(source.bank.ExportGenesis(source.ctx))
	require.NoError(t, err)
	source.commit()
	source.close(t)
	targetHome := filepath.Join(t.TempDir(), "imported")
	target := repair1249Open(t, targetHome, true)
	var bankGenesis banktypes.GenesisState
	require.NoError(t, json.Unmarshal(bankJSON, &bankGenesis))
	target.bank.InitGenesis(target.ctx, &bankGenesis)
	am := xweb.NewAppModule(target.web)
	require.NoError(t, am.ValidateGenesis(target.cdc, nil, webJSON))
	require.NotPanics(t, func() { am.InitGenesis(target.ctx, target.cdc, webJSON) })
	require.JSONEq(t, string(webJSON), string(am.ExportGenesis(target.ctx, target.cdc)))
	target.commit()
	target.close(t)
	return repair1249Open(t, targetHome, false)
}

func repair1249Snapshot(t *testing.T, s *repair1249Store) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	it := s.ctx.KVStore(s.keys[types.StoreKey]).Iterator(nil, nil)
	defer func() { require.NoError(t, it.Close()) }()
	for ; it.Valid(); it.Next() {
		out[string(it.Key())] = bytes.Clone(it.Value())
	}
	return out
}

func repair1249Reject(t *testing.T, s *repair1249Store, g types.GenesisState) {
	t.Helper()
	raw := repair1249JSON(t, g)
	var decoded types.GenesisState
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.ErrorIs(t, decoded.Validate(), types.ErrInvalidGenesis)
	am := xweb.NewAppModule(s.web)
	require.ErrorIs(t, am.ValidateGenesis(s.cdc, nil, raw), types.ErrInvalidGenesis)
	before := repair1249Snapshot(t, s)
	require.Panics(t, func() { am.InitGenesis(s.ctx, s.cdc, raw) })
	require.Equal(t, before, repair1249Snapshot(t, s), "invalid import must write no state")
}

func TestRepairM_1249_StorageReceiptErrorName(t *testing.T) {
	s := repair1249Open(t, filepath.Join(t.TempDir(), "source"), true)
	valid, err := types.EncodeStorageReceiptPayload(types.StorageReceiptPayload{ObjectDigest: types.Keccak([]byte("object")), BaseURL: "https://storage.example/bucket"})
	require.NoError(t, err)
	cases := map[string][]byte{
		"short digest":   bytes.Clone(valid[:31]),
		"zero digest":    append(make([]byte, 32), valid[32:]...),
		"http":           bytes.Replace(valid, []byte("https:"), []byte("http:"), 1),
		"query":          append(bytes.Clone(valid), '?'),
		"fragment":       append(bytes.Clone(valid), '#'),
		"trailing slash": append(bytes.Clone(valid), '/'),
		"invalid utf8":   append(bytes.Clone(valid), 0xff),
		"missing url":    bytes.Clone(valid[:32]),
		"oversized url":  append(bytes.Clone(valid[:32]), []byte("https://storage.example/"+strings.Repeat("x", 513))...),
	}
	precompile := pxweb.NewPrecompileWithKeeper(&s.web, s.bank, s.evm)
	require.Equal(t, common.HexToAddress("0x1019"), precompile.Address())
	caller := common.HexToAddress("0xa1249")
	payer := s.evm.GetPaxAddressOrDefault(s.ctx, caller)
	s.fund(t, payer, types.ProgramFeeDenomPAX, 2_000_000)
	const name = "LXP_XWEB_INVALID_PAYLOAD"
	for label, payload := range cases {
		t.Run(label, func(t *testing.T) {
			before := repair1249Snapshot(t, s)
			balance := s.bank.GetBalance(s.ctx, payer, types.ProgramFeeDenomPAX)
			_, directErr := s.web.Request(s.ctx, caller, types.KindStorageReceipt, payload, 100_000, s.web.Fee(s.ctx))
			require.ErrorIs(t, directErr, types.ErrInvalidPayload)
			code, found := types.LXPName(directErr)
			require.True(t, found)
			require.Equal(t, name, code)
			input, err := precompile.GetABI().Pack(pxweb.RequestMethod, types.KindStorageReceipt, payload, uint64(100_000))
			require.NoError(t, err)
			evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(s.ctx.BlockHeight()), CanTransfer: core.CanTransfer, Transfer: core.Transfer},
				state.NewDBImpl(s.ctx, s.evm, false), evmtypes.DefaultChainConfig().EthereumConfig(big.NewInt(125)), vm.Config{},
				map[common.Address]vm.PrecompiledContract{precompile.Address(): precompile})
			output, _, err := evm.Call(caller, precompile.Address(), input, 2_000_000, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Equal(t, name, strings.SplitN(reason, ":", 2)[0])
			sharedReason, err := abi.UnpackRevert(pcommon.LXPReason(directErr))
			require.NoError(t, err)
			require.Equal(t, name, strings.SplitN(sharedReason, ":", 2)[0])
			registeredReason, err := abi.UnpackRevert(pcommon.LXPRevert(name, ""))
			require.NoError(t, err)
			require.Equal(t, name+": invalid storage receipt payload", registeredReason)
			require.Equal(t, before, repair1249Snapshot(t, s))
			require.Equal(t, balance, s.bank.GetBalance(s.ctx, payer, types.ProgramFeeDenomPAX))
		})
	}
	// Both SDK consumers default unlisted terminal codes to no retry. Inspect
	// their committed maps without invoking another language or toolchain.
	root := filepath.Join("..", "..", "..")
	python, err := os.ReadFile(filepath.Join(root, "services/agent/sdk/python/layerx_sdk/revert.py"))
	require.NoError(t, err)
	require.Contains(t, string(python), "LXP_RETRYABLE.get(code, False)")
	rows := regexp.MustCompile(`"(LXP_[A-Z0-9_]+)": (True|False)`).FindAllStringSubmatch(string(python), -1)
	require.NotEmpty(t, rows)
	retryable := map[string]bool{}
	for _, row := range rows {
		retryable[row[1]] = row[2] == "True"
	}
	require.False(t, retryable[name])
	typescript, err := os.ReadFile(filepath.Join(root, "services/agent/sdk/typescript/src/revert.ts"))
	require.NoError(t, err)
	start := strings.Index(string(typescript), "// BEGIN GENERATED")
	end := strings.Index(string(typescript), "// END GENERATED")
	require.Greater(t, end, start)
	require.NotContains(t, string(typescript)[start:end], `"`+name+`"`)
	require.Contains(t, string(typescript), "this.retryable = LXP_RETRYABLE.has(code)")
	// The canonical producer's unmodified receipt still reaches admission.
	id, err := s.web.Request(s.ctx, caller, types.KindStorageReceipt, valid, 100_000, s.web.Fee(s.ctx))
	require.NoError(t, err)
	request, found := s.web.GetRequest(s.ctx, id)
	require.True(t, found)
	require.Equal(t, types.Keccak(valid), request.PayloadHash)
}

func TestRepairM_1249_PendingGenesisFeeDenom(t *testing.T) {
	s := repair1249Open(t, filepath.Join(t.TempDir(), "source"), true)
	denoms := []string{types.ProgramFeeDenomPAX, types.ProgramFeeDenomSID, types.ProgramFeeDenomUSDC, types.ProgramFeeDenomUSDL}
	for i, denom := range denoms {
		s.request(t, uint64(i+1), denom)
	}
	original := s.web.ExportGenesis(s.ctx)
	require.NoError(t, original.Validate())
	for i := range original.ProgramFeeDenoms {
		bad := s.web.ExportGenesis(s.ctx)
		bad.ProgramFeeDenoms = append(bad.ProgramFeeDenoms[:i], bad.ProgramFeeDenoms[i+1:]...)
		repair1249Reject(t, s, bad)
	}
	s = repair1249Recover(t, s)
	program := repair1249Program()
	keys := xwebtestutil.Attestors(3)
	for i, denom := range denoms {
		id := uint64(i + 1)
		answer, err := s.client().ProgramRequest(context.Background(), &types.QueryProgramRequestRequest{ProgramId: program[:], RequestId: id})
		require.NoError(t, err)
		require.Equal(t, denom, answer.FeeDenom)
		record, err := keeper.DecodeProgramPendingRecord(answer.Record)
		require.NoError(t, err)
		require.False(t, record.Fulfilled)
		require.Equal(t, sdk.NewInt(7), record.Amount)
		require.Equal(t, original.ProgramPending[i].Record, answer.Record)
		_, raw := repair1249Observation(t, id, []byte("recovered"))
		require.NoError(t, s.web.FulfilProgram(s.ctx, raw))
		for index, key := range keys {
			share := int64(2)
			if index == 0 {
				share = 3
			}
			require.Equal(t, sdk.NewInt(share), s.bank.GetBalance(s.ctx, key.Payout, denom).Amount)
		}
		require.True(t, s.bank.GetBalance(s.ctx, keeper.ProgramFeeAccount(record.FeeAsset), denom).IsZero())
		require.ErrorContains(t, s.web.FulfilProgram(s.ctx, raw), "LXP_SEQUENCE_REUSED")
	}
	fulfilled := s.web.ExportGenesis(s.ctx)
	require.NoError(t, fulfilled.Validate())
	bad := s.web.ExportGenesis(s.ctx)
	bad.ProgramFeeDenoms = nil
	repair1249Reject(t, s, bad)
	s.commit()
	home := s.home
	s.close(t)
	s = repair1249Open(t, home, false)
	require.Equal(t, fulfilled, s.web.ExportGenesis(s.ctx))
	for i, denom := range denoms {
		answer, err := s.client().ProgramRequest(context.Background(), &types.QueryProgramRequestRequest{ProgramId: program[:], RequestId: uint64(i + 1)})
		require.NoError(t, err)
		require.Equal(t, denom, answer.FeeDenom)
		for index, key := range keys {
			share := int64(2)
			if index == 0 {
				share = 3
			}
			require.Equal(t, sdk.NewInt(share), s.bank.GetBalance(s.ctx, key.Payout, denom).Amount)
		}
	}
}

func TestRepairM_1249_ZeroFeeGenesisPolicy(t *testing.T) {
	// Preserve only the existing exemption: an unpaid historical pending
	// fixture needs no mapping. Fulfilled records still require one.
	g := programGenesis(1, 0)
	g.ProgramPending[0].Record[113] = 0
	g.ProgramFeeDenoms = nil
	require.NoError(t, g.Validate())
	for byteIndex := 98; byteIndex < 114; byteIndex++ {
		g.ProgramPending[0].Record[byteIndex] = 1
		require.ErrorIs(t, g.Validate(), types.ErrInvalidGenesis, "amount byte %d", byteIndex)
		g.ProgramPending[0].Record[byteIndex] = 0
	}
	g = programGenesis(0, 1)
	g.ProgramPending[0].Record[113] = 0
	require.NoError(t, g.Validate())
	g.ProgramFeeDenoms = nil
	require.ErrorIs(t, g.Validate(), types.ErrInvalidGenesis)
}

func TestRepairM_1249_ProgramAnswerGenesisRecovery(t *testing.T) {
	require.Equal(t, keeper.ProgramAnswerChunkBytes, types.ProgramAnswerChunkBytes)
	s := repair1249Open(t, filepath.Join(t.TempDir(), "source"), true)
	lengths := []int{0, 1, 1023, 1024, 1025, 2048, 3073, 4096}
	observations := make([]types.Observation, len(lengths))
	for i, length := range lengths {
		id := uint64(i + 1)
		s.request(t, id, types.ProgramFeeDenomUSDL)
		var body []byte
		if length > 0 {
			body = bytes.Repeat([]byte{byte(id)}, length)
		}
		observation, raw := repair1249Observation(t, id, body)
		observations[i] = observation
		require.NoError(t, s.web.FulfilProgram(s.ctx, raw))
	}
	before := s.web.ExportGenesis(s.ctx)
	require.NoError(t, before.Validate())
	s = repair1249Recover(t, s)
	require.Equal(t, before, s.web.ExportGenesis(s.ctx))
	for _, observation := range observations {
		answer, err := s.client().ProgramResult(context.Background(), &types.QueryProgramResultRequest{ProgramId: observation.ProgramID[:], RequestId: observation.RequestID})
		require.NoError(t, err)
		require.Equal(t, observation.ContentDigest, answer.ContentDigest)
		require.Equal(t, observation.FullLength, answer.FullLength)
		require.Equal(t, uint32(len(observation.Response)), answer.ResponseLength)
		require.Equal(t, observation.Response, answer.Response)
	}
	message, broken := keeper.ProgramCountersInvariant(s.web)(s.ctx)
	require.False(t, broken, message)
}

func TestRepairM_1249_ProgramAnswerGenesisRejectsCorruption(t *testing.T) {
	s := repair1249Open(t, filepath.Join(t.TempDir(), "source"), true)
	s.request(t, 1, types.ProgramFeeDenomUSDL)
	_, raw := repair1249Observation(t, 1, bytes.Repeat([]byte{0x41}, 2049))
	require.NoError(t, s.web.FulfilProgram(s.ctx, raw))
	mutations := []struct {
		name  string
		apply func(*types.GenesisState)
	}{
		{"missing first chunk", func(g *types.GenesisState) { g.ProgramAnswers = append(g.ProgramAnswers[:1], g.ProgramAnswers[2:]...) }},
		{"missing middle chunk", func(g *types.GenesisState) { g.ProgramAnswers = append(g.ProgramAnswers[:2], g.ProgramAnswers[3:]...) }},
		{"missing last chunk", func(g *types.GenesisState) { g.ProgramAnswers = g.ProgramAnswers[:3] }},
		{"no chunks", func(g *types.GenesisState) { g.ProgramAnswers = g.ProgramAnswers[:1] }},
		{"missing header", func(g *types.GenesisState) { g.ProgramAnswers = g.ProgramAnswers[1:] }},
		{"short header", func(g *types.GenesisState) { g.ProgramAnswers[0].Value = g.ProgramAnswers[0].Value[:39] }},
		{"long header", func(g *types.GenesisState) { g.ProgramAnswers[0].Value = append(g.ProgramAnswers[0].Value, 0) }},
		{"response over bound", func(g *types.GenesisState) {
			binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[32:36], 4097)
			binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[36:40], 4097)
		}},
		{"response exceeds full length", func(g *types.GenesisState) { binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[32:36], 2048) }},
		{"max response length", func(g *types.GenesisState) {
			binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[32:36], math.MaxUint32)
			binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[36:40], math.MaxUint32)
		}},
		{"short middle chunk", func(g *types.GenesisState) { g.ProgramAnswers[2].Value = g.ProgramAnswers[2].Value[:1023] }},
		{"long first chunk", func(g *types.GenesisState) { g.ProgramAnswers[1].Value = append(g.ProgramAnswers[1].Value, 0) }},
		{"empty chunk", func(g *types.GenesisState) { g.ProgramAnswers[3].Value = nil }},
		{"long final chunk", func(g *types.GenesisState) { g.ProgramAnswers[3].Value = []byte{1, 2} }},
		{"extra chunk", func(g *types.GenesisState) {
			extra := g.ProgramAnswers[3]
			extra.Part++
			g.ProgramAnswers = append(g.ProgramAnswers, extra)
		}},
		{"extra empty chunk", func(g *types.GenesisState) {
			extra := g.ProgramAnswers[3]
			extra.Part = 255
			extra.Value = nil
			g.ProgramAnswers = append(g.ProgramAnswers, extra)
		}},
		{"duplicate chunk", func(g *types.GenesisState) { g.ProgramAnswers = append(g.ProgramAnswers, g.ProgramAnswers[1]) }},
		{"duplicate header", func(g *types.GenesisState) { g.ProgramAnswers = append(g.ProgramAnswers, g.ProgramAnswers[0]) }},
		{"unreferenced request", func(g *types.GenesisState) { g.ProgramAnswers[1].RequestID++ }},
		{"unreferenced program", func(g *types.GenesisState) { g.ProgramAnswers[1].ProgramID[0]++ }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			g := s.web.ExportGenesis(s.ctx)
			mutation.apply(&g)
			repair1249Reject(t, s, g)
		})
	}
	// The audited one-byte response with no part 1 must fail before import.
	g := s.web.ExportGenesis(s.ctx)
	binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[32:36], 1)
	binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[36:40], 1)
	g.ProgramAnswers = g.ProgramAnswers[:1]
	repair1249Reject(t, s, g)
	// Genesis entry order is immaterial; only the canonical key set matters.
	g = s.web.ExportGenesis(s.ctx)
	for i, j := 0, len(g.ProgramAnswers)-1; i < j; i, j = i+1, j-1 {
		g.ProgramAnswers[i], g.ProgramAnswers[j] = g.ProgramAnswers[j], g.ProgramAnswers[i]
	}
	require.NoError(t, g.Validate())
	// Empty answers have exactly the header and no chunks, even empty ones.
	s.request(t, 2, types.ProgramFeeDenomUSDL)
	_, raw = repair1249Observation(t, 2, nil)
	require.NoError(t, s.web.FulfilProgram(s.ctx, raw))
	g = s.web.ExportGenesis(s.ctx)
	require.NoError(t, g.Validate())
	g.ProgramAnswers = append(g.ProgramAnswers, types.ProgramAnswerEntry{ProgramID: repair1249Program(), RequestID: 2, Part: 1})
	repair1249Reject(t, s, g)
	// Retain every pre-existing genesis assertion with the newly mapped fixture.
	t.Run("existing genesis assertions", TestGenesisValidateRefusesCountersThatDisagreeWithRecords)
	t.Run("pending capacity", TestGenesisValidateRefusesPendingOverCapacity)
	t.Run("committed capacity", TestGenesisValidateRefusesCommittedOverCapacity)
	t.Logf("validated %d producer-derived corruption cases before any import writes", len(mutations)+2)
}
