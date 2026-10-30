package streams_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/streams"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const streamsPlan = "streams-test-merge"
const streamsStart = uint64(1700000000)

type streamsHarness struct {
	t                   *testing.T
	keeper              *paykeeper.Keeper
	precompile          *agentcommon.Precompile
	db                  *state.DBImpl
	evm                 *vm.EVM
	hostKey             sdk.StoreKey
	payer, payee, other streamsActor
	asset               [32]byte
	token               common.Address
}

func newStreamsHarness(t *testing.T) *streamsHarness {
	t.Helper()
	application := testkeeper.EVMTestApp
	ctx, _ := application.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(100).WithBlockTime(time.Unix(int64(streamsStart), 0)).WithEVMTxHash(common.HexToHash("0x2351"))
	payKey, hostKey := sdk.NewKVStoreKey(paytypes.StoreKey), sdk.NewKVStoreKey("lxphost")
	parent := ctx.MultiStore()
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	for _, key := range parent.StoreKeys() {
		var kv storetypes.KVStore
		func() { defer func() { _ = recover() }(); kv = parent.GetKVStore(key) }()
		if kv != nil {
			stores[key] = kv
			keys[key.Name()] = key
		}
	}
	for _, key := range []*sdk.KVStoreKey{payKey, hostKey} {
		stores[key] = dbadapter.Store{DB: dbm.NewMemDB()}
		keys[key.Name()] = key
	}
	ctx = ctx.WithMultiStore(cachemulti.NewStore(dbm.NewMemDB(), stores, keys, nil, nil, nil, 0))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	keeper, err := paykeeper.NewKeeper(application.AppCodec(), payKey, hostKey, host, &application.EvmKeeper, application.LayerXCustodyKeeper, "pay-governance")
	require.NoError(t, err)
	require.NoError(t, keeper.InitGenesis(ctx, *paytypes.DefaultGenesis()))
	keeper.SetParams(ctx, paytypes.Params{StreamEnabled: true})
	for _, module := range []uint16{paytypes.EscrowModuleID, paytypes.BudgetModuleID, paytypes.StreamModuleID, paytypes.ServiceModuleID} {
		key, err := paytypes.KernelModuleEnableKey(module)
		require.NoError(t, err)
		var enabled [32]byte
		enabled[31] = 1
		ctx.KVStore(hostKey).Set(key, enabled[:])
	}
	application.UpgradeKeeper.SetDone(ctx, streamsPlan)
	p, err := streams.NewPrecompile(&keeper, &application.UpgradeKeeper, streamsPlan)
	require.NoError(t, err)
	h := &streamsHarness{t: t, keeper: &keeper, precompile: p, hostKey: hostKey, token: common.HexToAddress("0x2351")}
	h.db = state.NewDBImpl(ctx, &application.EvmKeeper, false)
	block, err := application.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))
	h.evm = vm.NewEVM(*block, h.db, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{p.Address(): p})
	h.payer = streamsRegister(t, h, [32]byte{0x35, 1}, common.HexToAddress("0xa351"), true)
	h.payee = streamsRegister(t, h, [32]byte{0x35, 2}, common.HexToAddress("0xa352"), true)
	h.other = streamsRegister(t, h, [32]byte{0x35, 3}, common.HexToAddress("0xa353"), true)
	h.fund()
	require.NoError(t, application.LayerXCustodyKeeper.SetAsset(h.db.Ctx(), custodytypes.AssetMapping{AssetId: hex.EncodeToString(h.asset[:]), Denom: "streamtoken", Pointer: h.token.Hex(), Enabled: true}))
	coins := sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), sdk.NewInt(100)))
	require.NoError(t, application.BankKeeper.MintCoins(h.db.Ctx(), "evm", coins))
	require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(h.db.Ctx(), "evm", application.EvmKeeper.GetPaxAddressOrDefault(h.db.Ctx(), h.payer.caller), coins))
	return h
}

func (h *streamsHarness) input(name string, args ...interface{}) []byte {
	h.t.Helper()
	input, err := h.precompile.GetABI().Pack(name, args...)
	require.NoError(h.t, err)
	return input
}

func (h *streamsHarness) call(caller common.Address, name string, args ...interface{}) ([]byte, error) {
	h.t.Helper()
	out, _, err := h.evm.Call(caller, h.precompile.Address(), h.input(name, args...), 2000000, uint256.NewInt(0))
	require.NoError(h.t, h.db.Err())
	return out, err
}

func (h *streamsHarness) write(caller common.Address, name string, args ...interface{}) []interface{} {
	h.t.Helper()
	out, err := h.call(caller, name, args...)
	require.NoError(h.t, err, "%x", out)
	result, err := h.precompile.GetABI().Methods[name].Outputs.Unpack(out)
	require.NoError(h.t, err)
	return result
}

func (h *streamsHarness) view(name string, args ...interface{}) []interface{} {
	h.t.Helper()
	logs := len(h.db.GetAllLogs())
	before := h.keeper.ExportGenesis(h.db.Ctx())
	out, _, err := h.evm.StaticCall(h.other.caller, h.precompile.Address(), h.input(name, args...), 2000000)
	require.NoError(h.t, err, "%x", out)
	require.Equal(h.t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(h.t, h.db.GetAllLogs(), logs)
	result, err := h.precompile.GetABI().Methods[name].Outputs.Unpack(out)
	require.NoError(h.t, err)
	return result
}

func (h *streamsHarness) open() *big.Int {
	h.t.Helper()
	return h.write(h.payer.caller, "open", h.payee.caller, h.token, big.NewInt(2), streamsStart, streamsStart+100, big.NewInt(100))[0].(*big.Int)
}

func (h *streamsHarness) advance(seconds int64) {
	h.db.WithCtx(h.db.Ctx().WithBlockHeight(h.db.Ctx().BlockHeight() + 1).WithBlockTime(time.Unix(int64(streamsStart)+seconds, 0)))
}

func (h *streamsHarness) record(id *big.Int) paytypes.StreamRecord {
	h.t.Helper()
	var key [32]byte
	id.FillBytes(key[:])
	record, err := h.keeper.GetStream(h.db.Ctx(), key)
	require.NoError(h.t, err)
	return record
}

func requireStreamsRevert(t *testing.T, output []byte, err error, code string) {
	t.Helper()
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, decodeErr := abi.UnpackRevert(output)
	require.NoError(t, decodeErr)
	require.True(t, reason == code || strings.HasPrefix(reason, code+": "), reason)
}

func (h *streamsHarness) log(index int, name, topic string, indexed []common.Hash, values ...interface{}) {
	h.t.Helper()
	entry := h.db.GetAllLogs()[index]
	require.Equal(h.t, h.precompile.Address(), entry.Address)
	require.Equal(h.t, append([]common.Hash{common.HexToHash(topic)}, indexed...), entry.Topics)
	data, err := h.precompile.GetABI().Events[name].Inputs.NonIndexed().Pack(values...)
	require.NoError(h.t, err)
	require.Equal(h.t, data, entry.Data)
}

type streamsActor struct {
	private ed25519.PrivateKey
	public  [32]byte
	did     []byte
	actor   [32]byte
	caller  common.Address
	account [32]byte
}

func streamsExecute(t *testing.T, h *streamsHarness, a streamsActor, kind uint32, payload []byte, key [32]byte) {
	t.Helper()
	ctx, host := h.db.Ctx(), h.keeper.Host()
	last, err := host.ReadLastSequence(ctx, a.did)
	require.NoError(t, err)
	seq, err := host.ReadIdentitySequence(ctx, a.did)
	require.NoError(t, err)
	activity := lxphost.Activity{ProtocolVersion: 3, Type: kind, ActorDID: a.did, AuthorityKey: a.public,
		AccountSequence: seq, NotBefore: uint64(ctx.BlockTime().UnixMilli()),
		NotAfter: uint64(ctx.BlockTime().UnixMilli()) + host.TimestampWindowMs(), IdempotencyKey: key, Payload: payload}
	digest, err := host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(a.private, digest[:])
	receipt, err := host.HostExecute(lxphost.WithSequence(ctx, last+1), activity, lxphost.KernelAuthority{Actor: a.actor, Principal: a.actor, VerifiedKey: a.public})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
}

func streamsRegister(t *testing.T, h *streamsHarness, seed [32]byte, caller common.Address, bind bool) streamsActor {
	t.Helper()
	a := streamsActor{private: ed25519.NewKeyFromSeed(seed[:]), caller: caller}
	copy(a.public[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte(evmtypes.LayerXDid(a.public))
	var err error
	a.actor, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	if bind {
		keeper := &testkeeper.EVMTestApp.EvmKeeper
		message, err := evmtypes.LayerXBindMessage(keeper.ChainID(h.db.Ctx()), caller, 0)
		require.NoError(t, err)
		var signature [64]byte
		copy(signature[:], ed25519.Sign(a.private, message))
		_, err = keeper.BindLayerX(h.db.Ctx(), caller, a.public, signature)
		require.NoError(t, err)
	}
	payload := append([]byte{0x71, 1, 0, 2}, a.actor[:]...)
	payload = append(payload, a.public[:]...)
	streamsExecute(t, h, a, 0x00070001, payload, sha256.Sum256(a.did))
	return a
}

func streamsMain(t *testing.T, h *streamsHarness, a streamsActor, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := lxphost.MainAccountID(a.public)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	row := field(nil, id[:])
	row = field(row, []byte(name))
	row = append(row, 1)
	row = append(row, make([]byte, 16)...)
	row = field(row, asset[:])
	row = append(row, 1)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = binary.BigEndian.AppendUint64(row, 1)
	row = append(row, 0, 0)
	row = field(row, a.public[:])
	row = append(row, 1)
	h.db.Ctx().KVStore(h.hostKey).Set(append([]byte{0x06}, id[:]...), row)
	return id
}

func (h *streamsHarness) fund() {
	t := h.t
	t.Helper()
	salt := [32]byte{0x28, 0x04}
	h.asset = sha256.Sum256(append(append([]byte("LX:ASSET:v1"), h.payer.actor[:]...), salt[:]...))
	h.payer.account = streamsMain(t, h, h.payer, h.asset)
	h.payee.account = streamsMain(t, h, h.payee, h.asset)
	h.other.account = streamsMain(t, h, h.other, h.asset)
	assetName, assetAccount, err := lxphost.AssetAccountID(h.payer.public, h.asset)
	require.NoError(t, err)
	payload := append([]byte{0, 1}, h.asset[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 2000)
	payload = append(payload, 1, 0)
	streamsExecute(t, h, h.payer, 0x00010001, payload, [32]byte{0x11})
	streamsExecute(t, h, h.payer, 0x00010004, append([]byte{0, 1}, h.asset[:]...), [32]byte{0x12})
	payload = append(append([]byte{0, 1}, h.asset[:]...), assetAccount[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	streamsExecute(t, h, h.payer, 0x0001000a, payload, [32]byte{0x13})
	amount := (codec.U128{Lo: 1000}).Bytes()
	key := [32]byte{0x14}
	material := append(append(append(append([]byte(nil), assetAccount[:]...), h.payer.account[:]...), h.asset[:]...), amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(bytes.Clone(material), key[:]...))
	require.NoError(t, err)
	account, err := h.keeper.Host().ReadAccount(h.db.Ctx(), assetName, assetAccount)
	require.NoError(t, err)
	transfer := binary.BigEndian.AppendUint64(material, account.NextSequence)
	transfer = append(transfer, key[:]...)
	transfer = binary.BigEndian.AppendUint64(transfer, uint64(h.db.Ctx().BlockTime().UnixMilli())+10000)
	transfer = append(transfer, contextHash[:]...)
	transfer = append(transfer, 0)
	suffix := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint32(nil, 7), 3)
	message := append([]byte{0x53, 1}, transfer...)
	message = append(message, 1)
	message = append(message, assetAccount[:]...)
	message = append(message, contextHash[:]...)
	message = append(message, suffix...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	payload = append([]byte{0x53, 1, 0, 10}, transfer...)
	payload = append(payload, 1)
	payload = append(payload, assetAccount[:]...)
	payload = append(payload, h.payer.public[:]...)
	payload = append(payload, ed25519.Sign(h.payer.private, digest[:])...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	streamsExecute(t, h, h.payer, 0x00010005, payload, key)
}
