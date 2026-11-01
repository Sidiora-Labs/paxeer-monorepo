package layerxprograms

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type sandboxActivationFixture struct {
	ctx      sdk.Context
	key      *sdk.KVStoreKey
	executor *PrecompileExecutor
	call     callContext
	leaseID  [32]byte
	funded   []byte
	digest   [32]byte
	state    int
}

func newSandboxActivationFixture(t *testing.T) sandboxActivationFixture {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	key, evmKey := sdk.NewKVStoreKey("sandbox_activation"), sdk.NewKVStoreKey(evmtypes.StoreKey)
	transient := sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	cms.MountStoreWithDB(evmKey, sdk.StoreTypeIAVL, db)
	cms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "sandbox-activation", Height: 100, Time: time.UnixMilli(1700000000123)}, false)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager()).
		WithEVMTxHash(common.HexToHash("0x132703"))
	evm := evmkeeper.NewKeeper(evmKey, transient, paramtypes.NewSubspace(nil, nil, evmKey, transient, "evm"),
		nil, nil, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	caller := common.HexToAddress("0xa111")
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(ctx), caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evm.BindLayerX(ctx, caller, public, signature)
	require.NoError(t, err)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	method := programsABI.Methods["sandboxActivate"]
	f := sandboxActivationFixture{ctx: ctx, key: key, executor: &PrecompileExecutor{host: host, evmKeeper: evm},
		call: callContext{caller: caller, method: &method, evm: &vm.EVM{}}, leaseID: [32]byte{1}}
	authority, _, err := principalOf(ctx, evm, caller)
	require.NoError(t, err)
	f.funded, f.state = sandboxActivationFundedLease(t, f.leaseID, authority.Actor)
	f.digest = sha256.Sum256(f.funded)
	ctx.KVStore(key).Set(append([]byte{1, 0, 9, 's', 0}, f.leaseID[:]...), f.funded)
	return f
}

func sandboxActivationFundedLease(t *testing.T, leaseID, tenant [32]byte) ([]byte, int) {
	t.Helper()
	host, image, asset := [32]byte{3}, [32]byte{4}, [32]byte{9}
	namespace := sha256.Sum256(append(append([]byte("LayerX/programs/sandbox/namespace/v1\x00"), host[:]...), leaseID[:]...))
	seed := append([]byte("sandbox-lease-escrow/v1\x00"), leaseID[:]...)
	accountPreimage := append([]byte("LayerX/programs/program-account/v1\x00"), host[:]...)
	accountPreimage = binary.BigEndian.AppendUint32(accountPreimage, uint32(len(seed)))
	escrow := sha256.Sum256(append(accountPreimage, seed...))
	fees, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	terms := make([]byte, 0)
	for _, field := range [][32]byte{leaseID, tenant, host, image, namespace, asset, escrow, fees} {
		terms = append(terms, field[:]...)
	}
	amount := make([]byte, 16)
	amount[15] = 100
	terms = append(terms, amount...)
	for i := 0; i < 8; i++ {
		terms = binary.BigEndian.AppendUint64(terms, 1)
	}
	terms = binary.BigEndian.AppendUint64(terms, 1)
	terms = binary.BigEndian.AppendUint64(terms, 200)
	schedule := binary.BigEndian.AppendUint32(nil, 1)
	for i := 0; i < 7; i++ {
		schedule = binary.BigEndian.AppendUint64(schedule, 1)
	}
	requestPreimage := append([]byte("LayerX/programs/sandbox/request/v3\x00"), terms...)
	requestBinding := sha256.Sum256(append(requestPreimage, schedule...))
	lease := append([]byte("LayerX/programs/sandbox/lease-state/v3\x00"), terms...)
	lease = append(lease, make([]byte, 64)...)
	lease = append(lease, schedule...)
	lease = append(lease, make([]byte, 16)...)
	stateOffset := len(lease)
	lease = append(lease, 0, 1, 0, 0, 0)
	requestID := [32]byte{8}
	lease = append(lease, requestID[:]...)
	lease = append(lease, requestBinding[:]...)
	requestReceipt := sha256.Sum256(lease)
	lease = append(lease, requestReceipt[:]...)
	lease = binary.BigEndian.AppendUint64(lease, 1)
	lease = append(lease, 0, 0, 0)
	requestedDigest := sha256.Sum256(lease)
	fundID := [32]byte{7}
	commitment := append([]byte("LayerX/programs/sandbox/intrinsic-transition/v1\x00"), requestedDigest[:]...)
	commitment = append(commitment, fundID[:]...)
	commitment = append(commitment, 1)
	commitment = binary.BigEndian.AppendUint64(commitment, 2)
	fundReceipt := sha256.Sum256(commitment)
	lease = lease[:len(lease)-3]
	lease[stateOffset], lease[stateOffset+1] = 1, 2
	lease = append(lease, 1, 0, 1)
	lease = append(lease, fundID[:]...)
	lease = append(lease, make([]byte, 32)...)
	lease = append(lease, fundReceipt[:]...)
	lease = binary.BigEndian.AppendUint64(lease, 2)
	return append(lease, 0, 0, 0), stateOffset
}

func (f sandboxActivationFixture) dump(ctx sdk.Context) map[string][]byte {
	out := make(map[string][]byte)
	iterator := ctx.KVStore(f.key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		out[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return out
}

func (f sandboxActivationFixture) args(t *testing.T, sequence uint64) []interface{} {
	t.Helper()
	packed, err := f.call.method.Inputs.Pack(f.leaseID, f.digest, sequence)
	require.NoError(t, err)
	args, err := f.call.method.Inputs.Unpack(packed)
	require.NoError(t, err)
	return args
}

func TestProgramsSandboxActivateLease(t *testing.T) {
	f := newSandboxActivationFixture(t)
	payload, err := EncodeSandboxActivatePayload(f.leaseID, f.digest, 3)
	require.NoError(t, err)
	require.Len(t, payload, 76)
	require.Equal(t, []byte{1, 3, 0, 0}, payload[:4])
	require.Equal(t, f.leaseID[:], payload[4:36])
	require.Equal(t, f.digest[:], payload[36:68])
	require.Equal(t, uint64(3), binary.BigEndian.Uint64(payload[68:]))
	reference, _ := f.ctx.CacheContext()
	receipt, err := submit(reference, f.executor, f.call, 0x00090009, payload, [16]byte{})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.Equal(t, uint64(1), receipt.GlobalSequence)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(9), decoded.ModuleID)
	before := f.dump(f.ctx)
	branch, _ := f.ctx.CacheContext()
	out, err := sandboxActivateHandler(branch, f.executor, f.call, f.args(t, 3))
	require.NoError(t, err)
	require.Empty(t, out)
	require.Equal(t, f.dump(reference), f.dump(branch))
	require.Equal(t, before, f.dump(f.ctx))
	active, err := f.executor.host.ReadSandbox(branch, f.leaseID, 0)
	require.NoError(t, err)
	require.Len(t, active, len(f.funded)+107)
	require.Equal(t, byte(2), active[f.state])
	require.Equal(t, byte(3), active[f.state+1])
	require.Equal(t, []byte{2, 1, 2}, active[len(f.funded)-3:len(f.funded)])
	require.Equal(t, decoded.ActivityID[:], active[len(f.funded):len(f.funded)+32])
	_, did, err := principalOf(branch, f.executor.evmKeeper, f.call.caller)
	require.NoError(t, err)
	sequence, err := f.executor.host.ReadIdentitySequence(branch, did)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	for _, kind := range []uint8{1, 2} {
		_, err := f.executor.host.ReadSandbox(branch, f.leaseID, kind)
		require.ErrorIs(t, err, lxphost.Code(-7))
	}
	t.Run("executor dispatch", func(t *testing.T) {
		dispatched, _ := f.ctx.CacheContext()
		out, err := f.executor.Execute(dispatched, f.call.method, f.call.caller, Address,
			f.args(t, 3), big.NewInt(0), false, &vm.EVM{}, nil)
		require.NoError(t, err)
		require.Empty(t, out)
		require.Equal(t, f.dump(branch), f.dump(dispatched))
	})
	t.Run("staticcall refuses activation", func(t *testing.T) {
		branch, _ := f.ctx.CacheContext()
		out, err := f.executor.Execute(branch, f.call.method, f.call.caller, Address,
			f.args(t, 3), big.NewInt(0), true, f.call.evm, nil)
		require.Nil(t, out)
		require.ErrorIs(t, err, ErrReadOnlyWrite)
		require.Equal(t, before, f.dump(branch))
	})
}

func TestProgramsSandboxActivateWrongSequenceReverts(t *testing.T) {
	f := newSandboxActivationFixture(t)
	before := f.dump(f.ctx)
	for _, sequence := range []uint64{9, 0, 1, 2, 4, math.MaxUint64} {
		branch, _ := f.ctx.CacheContext()
		out, err := sandboxActivateHandler(branch, f.executor, f.call, f.args(t, sequence))
		require.Nil(t, out)
		require.ErrorIs(t, err, lxphost.Code(-3))
		require.EqualError(t, err, "LXP_NON_CANONICAL: sandboxActivate: LXP_ERR_NON_CANONICAL")
		reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
		require.NoError(t, unpackErr)
		require.Equal(t, err.Error(), reason)
		lease, readErr := f.executor.host.ReadSandbox(branch, f.leaseID, 0)
		require.NoError(t, readErr)
		require.Equal(t, f.funded, lease)
		require.Equal(t, before, f.dump(f.ctx))
		dispatched, _ := f.ctx.CacheContext()
		out, dispatchErr := f.executor.Execute(dispatched, f.call.method, f.call.caller, Address,
			f.args(t, sequence), big.NewInt(0), false, f.call.evm, nil)
		require.Nil(t, out)
		require.ErrorIs(t, dispatchErr, lxphost.Code(-3))
		require.EqualError(t, dispatchErr, err.Error())
		require.Equal(t, f.dump(branch), f.dump(dispatched))
	}
	branch, _ := f.ctx.CacheContext()
	_, err := sandboxActivateHandler(branch, f.executor, f.call, f.args(t, 3))
	require.NoError(t, err)
	active, err := f.executor.host.ReadSandbox(branch, f.leaseID, 0)
	require.NoError(t, err)
	require.Equal(t, byte(2), active[f.state])
}
