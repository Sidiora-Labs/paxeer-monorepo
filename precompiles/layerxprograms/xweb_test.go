package layerxprograms

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ibctransferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type webCallHarness struct {
	ctx       sdk.Context
	key       *sdk.KVStoreKey
	webKey    *sdk.KVStoreKey
	bankKey   *sdk.KVStoreKey
	authKey   *sdk.KVStoreKey
	host      *lxphost.Host
	executor  *PrecompileExecutor
	web       xwebkeeper.Keeper
	caller    common.Address
	publicKey [32]byte
	evm       *vm.EVM
}

func newWebCallHarness(t *testing.T) *webCallHarness {
	t.Helper()
	keys := sdk.NewKVStoreKeys("host", "web", "bank", "auth", "evm", "params", "upgrade")
	transient := sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 100, ChainID: "paxeer-1", Time: time.UnixMilli(1700000000123)}, false).WithEVMTxHash(common.HexToHash("0x12345"))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := sdkcodec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, sdkcodec.NewLegacyAmino(), keys["params"], transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys["auth"], subspace("auth"), authtypes.ProtoBaseAccount, map[string][]string{})
	accounts.SetParams(ctx, authtypes.DefaultParams())
	bank := bankkeeper.NewBaseKeeper(cdc, keys["bank"], &accounts, subspace("bank"), nil)
	bank.SetParams(ctx, banktypes.DefaultParams())
	upgrades := upgradekeeper.NewKeeper(nil, keys["upgrade"], cdc, t.TempDir(), nil)
	upgrades.SetDone(ctx, xwebkeeper.ProgramOriginPlan)
	evm := evmkeeper.NewKeeper(keys["evm"], transient, subspace("evm"), nil, bank, &accounts, nil, ibctransferkeeper.Keeper{}, nil, nil, &upgrades)
	evm.SetParams(ctx, evmtypes.DefaultParams())
	web := xwebkeeper.NewKeeper(keys["web"], bank, evm)
	genesis := xwebtypes.DefaultGenesis()
	genesis.Params.KernelNetworkId = 7
	genesis.Params.ProgramFeeDenoms = []string{xwebtypes.ProgramFeeDenomPAX}
	genesis.Paused = false
	web.InitGenesis(ctx, *genesis)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: keys["host"], NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	h := &webCallHarness{ctx: ctx, key: keys["host"], webKey: keys["web"], bankKey: keys["bank"], authKey: keys["auth"], host: host, executor: &PrecompileExecutor{host: host, evmKeeper: evm}, web: web, caller: common.HexToAddress("0xa111")}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	copy(h.publicKey[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(ctx), h.caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evm.BindLayerX(ctx, h.caller, h.publicKey, signature)
	require.NoError(t, err)
	configureXWeb(h.executor, &h.web)
	statedb := state.NewDBImpl(ctx, evm, false)
	t.Cleanup(statedb.Cleanup)
	h.evm = &vm.EVM{StateDB: statedb}
	return h
}

func webStoreDump(ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	values := map[string][]byte{}
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

func webProgramsCallFixture(t *testing.T) (*webCallHarness, []interface{}, []byte) {
	t.Helper()
	h := newWebCallHarness(t)
	ctx := h.ctx
	authority, did, err := PrincipalOf(ctx, h.executor.evmKeeper, h.caller)
	require.NoError(t, err)
	moduleKey := func(key []byte) []byte { return append([]byte{1, 0, 9}, key...) }
	store := ctx.KVStore(h.key)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Actor[:]...)
	store.Set(moduleKey([]byte("progmet/active/v1")), metering)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)

	execute := func(activityType uint32, payload []byte) {
		sequence, err := h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		last, err := h.host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, last+1), h.host, authority, activityType, payload, sequence)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
	}
	salt := bytes.Repeat([]byte{0x55}, 32)
	assetMaterial := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(assetMaterial, salt...))
	register := append([]byte{0, 1}, asset[:]...)
	register = append(register, salt...)
	register = append(register, 1, 'T', 1, 'T', 6)
	register = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(register, 0), 1000000000)
	execute(0x00010001, append(register, 1, 0))
	execute(0x00010004, append([]byte{0, 1}, asset[:]...))
	_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, asset[:]...)
	mint = append(mint, payer[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 500000000)
	execute(0x0001000a, mint)

	mainName, mainID, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	main := field(nil, mainID[:])
	main = field(main, []byte(mainName))
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 100000000)
	mainAsset := [32]byte{0x91}
	main = field(main, mainAsset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, h.publicKey[:])
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	mainAccount, err := h.host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, int64(100000000), mainAccount.Balance.Int64())

	treasuryID, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	treasury := field(nil, treasuryID[:])
	treasury = field(treasury, []byte("system:fees"))
	treasury = append(treasury, 10)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = field(treasury, asset[:])
	treasury = append(treasury, 1)
	treasury = append(treasury, make([]byte, 16)...)
	treasury = append(treasury, 0, 0)
	treasury = field(treasury, make([]byte, 32))
	store.Set(append([]byte{6}, treasuryID[:]...), append(treasury, 0))
	treasuryAccount, err := h.host.ReadAccount(ctx, "system:fees", treasuryID)
	require.NoError(t, err)
	require.False(t, treasuryAccount.Frozen)
	require.True(t, treasuryAccount.Balance.IsZero())
	require.Equal(t, asset, treasuryAccount.AssetID)

	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, authority.Actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store.Set(moduleKey([]byte("progfee/active/v1")), fee)
	store.Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)

	section := func(id byte, body []byte) []byte {
		return append([]byte{id, byte(len(body))}, body...)
	}
	label := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	wasm = append(wasm, section(1, []byte{3, 0x60, 4, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	imports := append([]byte{1}, label("layerx_v1")...)
	imports = append(imports, label("event_emit")...)
	wasm = append(wasm, section(2, append(imports, 0, 0))...)
	wasm = append(wasm, section(3, []byte{2, 1, 2})...)
	wasm = append(wasm, section(5, []byte{1, 1, 1, 1})...)
	exports := append([]byte{3}, label("layerx_reserve")...)
	exports = append(exports, 0, 1)
	exports = append(exports, label("layerx_call")...)
	exports = append(exports, 0, 2)
	exports = append(exports, label("memory")...)
	wasm = append(wasm, section(7, append(exports, 2, 0))...)
	request := binary.BigEndian.AppendUint64(nil, 257)
	request = append(request, xwebtypes.KindSearch)
	request = binary.BigEndian.AppendUint32(request, 4)
	request = append(request, []byte("pax\x00")...)
	topic := []byte(xwebkeeper.ProgramRequestTopic)
	body := []byte{0, 0x41, 0, 0x41, byte(len(topic)), 0x41, byte(len(topic)), 0x41, byte(len(request)), 0x10, 0, 0x1a, 0x41, 0, 0x0b}
	code := []byte{2, 4, 0, 0x41, 48, 0x0b, byte(len(body))}
	wasm = append(wasm, section(10, append(code, body...))...)
	data := append(topic, request...)
	wasm = append(wasm, section(11, append([]byte{1, 0, 0x41, 0, 0x0b, byte(len(data))}, data...))...)
	program := [32]byte{0x71}
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = append(iface, 4, 0, 0, 0, 0)
	deploy, err := EncodeDeployPayload(program, 1, 1, authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	execute(0x00090001, deploy)

	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	capabilities := []byte{0, 1, 3}
	access := []byte("LayerX/programs/access-declaration/v1\x00\x00")
	args := []interface{}{program, uint16(1), "layerx_call", []byte{0, 0xff}, capabilities, access, uint32(64), budget, big.NewInt(100000000)}
	payload, err := EncodeCallPayload(program, 1, "layerx_call", args[3].([]byte), capabilities, access, 64, budget)
	require.NoError(t, err)
	return h, args, payload
}

func TestProgramsCallForwardsGuestWebRequest(t *testing.T) {
	h, args, payload := webProgramsCallFixture(t)
	ctx := h.ctx.WithValue(xwebContextKey{}, &h.web)
	method := programsABI.Methods["call"]
	reference, write := ctx.CacheContext()
	var limit [16]byte
	args[8].(*big.Int).FillBytes(limit[:])
	receipt, err := submit(reference, h.executor, callContext{caller: h.caller, method: &method, evm: h.evm}, 0x00090003, payload, limit)
	require.NoError(t, err)
	require.NotNil(t, receipt.ProgramOutcome)
	require.Equal(t, uint8(1), receipt.ProgramOutcome.Kind)
	_, did, err := principalOf(ctx, h.executor.evmKeeper, h.caller)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	_, main, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	principal := lxphost.BankAddress(main)
	asset := xwebtypes.Hash32{9}
	call := &xwebkeeper.ProgramCall{ProgramID: xwebtypes.Hash32(args[0].([32]byte)), Principal: principal, Sequence: sequence,
		Legs: []xwebkeeper.ProgramLeg{
			{From: principal, To: xwebkeeper.ProgramFeeAccount(asset), Asset: asset, Denom: xwebtypes.ProgramFeeDenomPAX, Amount: sdk.NewInt(23)},
			{From: principal, To: xwebkeeper.ProgramFeeAccount(asset), Asset: asset, Denom: xwebtypes.ProgramFeeDenomPAX, Amount: sdk.NewInt(52)},
		}}
	bank := h.executor.evmKeeper.BankKeeper()
	require.NoError(t, bank.AddCoins(ctx, principal, sdk.NewCoins(sdk.NewInt64Coin(xwebtypes.ProgramFeeDenomPAX, 100)), true))
	require.NoError(t, commitWebRequestReceipt(reference, h.executor, call, receipt))
	write()
	record, found, err := h.web.GetProgramPendingRecord(ctx, call.ProgramID, 257)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, xwebtypes.KindSearch, record.Kind)
	require.Equal(t, xwebtypes.Keccak([]byte("pax\x00")), record.PayloadHash)
	require.Equal(t, sequence, record.Sequence)
	require.Equal(t, asset, record.FeeAsset)
	require.Equal(t, xwebkeeper.ProgramFeeAccountID(asset), record.FeeAccount)
	require.True(t, sdk.NewInt(75).Equal(record.Amount))
	require.False(t, record.Fulfilled)
	require.Equal(t, uint64(1), h.web.ProgramPendingCount(ctx))
	require.True(t, sdk.NewInt(25).Equal(bank.GetBalance(ctx, principal, xwebtypes.ProgramFeeDenomPAX).Amount))
	require.True(t, sdk.NewInt(75).Equal(bank.GetBalance(ctx, xwebkeeper.ProgramFeeAccount(asset), xwebtypes.ProgramFeeDenomPAX).Amount))
	afterSequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	require.Equal(t, sequence+1, afterSequence)
}

func TestProgramsCallDiscardedWhenXWebRefuses(t *testing.T) {
	h, args, _ := webProgramsCallFixture(t)
	ctx := h.ctx
	require.NoError(t, h.web.Pause(ctx, xwebtypes.MsgPause{Authority: h.web.GetParams(ctx).Authority}))
	before := map[sdk.StoreKey]map[string][]byte{}
	for _, key := range []sdk.StoreKey{h.key, h.webKey, h.bankKey, h.authKey, h.executor.evmKeeper.GetStoreKey()} {
		before[key] = webStoreDump(ctx, key)
	}
	eventsBefore := append(sdk.Events(nil), ctx.EventManager().Events()...)
	method := programsABI.Methods["call"]
	output, err := h.executor.Execute(ctx, &method, h.caller, Address, args, big.NewInt(0), false, h.evm, nil)
	require.Nil(t, output)
	require.ErrorIs(t, err, xwebtypes.ErrPaused)
	reason, decodeErr := abi.UnpackRevert(pcommon.LXPReason(err))
	require.NoError(t, decodeErr)
	require.Contains(t, reason, "LXP_XWEB_PAUSED")
	for key, snapshot := range before {
		require.Equal(t, snapshot, webStoreDump(ctx, key), key.Name())
	}
	require.Equal(t, eventsBefore, ctx.EventManager().Events())
	require.Empty(t, state.GetDBImpl(h.evm.StateDB).GetAllLogs())
	require.Zero(t, h.web.ProgramPendingCount(ctx))
	_, found, recordErr := h.web.GetProgramPendingRecord(ctx, xwebtypes.Hash32(args[0].([32]byte)), 257)
	require.NoError(t, recordErr)
	require.False(t, found)
}
