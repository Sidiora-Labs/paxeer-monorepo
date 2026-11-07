package hostmigration_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type migrationHarness struct {
	host       *lxphost.Host
	key        *sdk.KVStoreKey
	private    ed25519.PrivateKey
	public     [32]byte
	did        []byte
	authority  lxphost.KernelAuthority
	state      *state.DBImpl
	evm        *vm.EVM
	precompile *pcommon.Precompile
}

func newMigrationHarness(t *testing.T) *migrationHarness {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(100).WithBlockTime(time.UnixMilli(1700000000123)).
		WithEVMTxHash(common.HexToHash("0x223142"))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	h := &migrationHarness{key: sdk.NewKVStoreKey("migration_binding")}
	ctx = xwebtestutil.WithXWebStore(ctx, h.key)
	h.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	copy(h.public[:], h.private.Public().(ed25519.PublicKey))
	caller := common.HexToAddress("0xa111")
	message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(ctx), caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(h.private, message))
	_, err = app.EvmKeeper.BindLayerX(ctx, caller, h.public, signature)
	require.NoError(t, err)
	h.did = []byte(evmtypes.LayerXDid(h.public))
	actor, err := lxphost.DIDID(h.did)
	require.NoError(t, err)
	digest, err := lxphost.AuthorityHash(lxphost.AuthorityKindOwner, [32]byte{}, h.public)
	require.NoError(t, err)
	h.authority = lxphost.KernelAuthority{Kind: lxphost.AuthorityKindOwner,
		Actor: actor, Principal: actor, VerifiedKey: h.public, AuthorityHash: digest, Bound: true}
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	asset := [32]byte{0x91}
	fee = append(fee, asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store := ctx.KVStore(h.key)
	store.Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), fee)
	store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)...), fee)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, actor[:]...)
	store.Set(append([]byte{1, 0, 9}, []byte("progmet/active/v1")...), metering)
	store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)...), metering)
	h.host, err = lxphost.NewHost(lxphost.Config{StoreKey: h.key, NetworkID: 7,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(h.host.Close)
	h.precompile, err = lp.NewPrecompile(h.host, &app.EvmKeeper, &app.UpgradeKeeper, "v7.0")
	require.NoError(t, err)
	h.state = state.NewDBImpl(ctx, &app.EvmKeeper, false)
	block, err := app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(ctx))
	h.evm = vm.NewEVM(*block, h.state, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{
		h.precompile.Address(): h.precompile,
	})
	return h
}

func (h *migrationHarness) dump() map[string][]byte {
	values := map[string][]byte{}
	iterator := h.state.Ctx().KVStore(h.key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

func migrationPayload(t *testing.T) ([]byte, [32]byte, []byte) {
	t.Helper()
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	hash := sha256.Sum256(wasm)
	payload, err := lp.EncodeUpgradePayload([32]byte{1}, 1, 1, [32]byte{2}, hash,
		[]byte{0}, nil, wasm)
	require.NoError(t, err)
	return payload, hash, wasm
}

func TestHostBoundMigrationPrecompile(t *testing.T) {
	h := newMigrationHarness(t)
	_, hash, wasm := migrationPayload(t)
	before := h.dump()
	input, err := h.precompile.Pack("upgrade", [32]byte{1}, uint16(1), uint8(1),
		[32]byte{2}, hash, []byte{0}, []byte{}, wasm, big.NewInt(0))
	require.NoError(t, err)
	data, _, err := h.evm.Call(common.HexToAddress("0xa111"), h.precompile.Address(),
		input, 4000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_UNKNOWN_FIELD")
	require.ErrorIs(t, h.state.GetPrecompileError(), lxphost.Code(-7))
	require.Equal(t, before, h.dump())
	require.Empty(t, h.state.GetAllLogs())
}

func TestHostBoundMigrationNative(t *testing.T) {
	h := newMigrationHarness(t)
	payload, _, _ := migrationPayload(t)
	ctx := h.state.Ctx()
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x00090002,
		ActorDID: h.did, AuthorityKey: h.public, NotBefore: timestamp, NotAfter: timestamp,
		IdempotencyKey: [32]byte{1}, Payload: payload}
	digest, err := h.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(h.private, digest[:])
	authority := h.authority
	authority.Bound = false
	receipt, err := h.host.Execute(lxphost.WithSequence(ctx, 1), activity, authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(-7), receipt.ResultCode)
	require.Equal(t, uint64(1), receipt.GlobalSequence)
	require.True(t, receipt.FeeCharged.IsZero())
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(3), decoded.ProtocolVersion)
}

func TestHostBoundMigrationTransportRefusals(t *testing.T) {
	for _, variant := range []string{"actor", "principal", "key", "signed payload"} {
		t.Run(variant, func(t *testing.T) {
			h := newMigrationHarness(t)
			payload, _, _ := migrationPayload(t)
			ctx := h.state.Ctx()
			timestamp := uint64(ctx.BlockTime().UnixMilli())
			activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x00090002,
				ActorDID: h.did, AuthorityKey: h.public, NotBefore: timestamp, NotAfter: timestamp,
				IdempotencyKey: [32]byte{1}, Payload: payload}
			authority := h.authority
			expected := lxphost.Code(-204)
			switch variant {
			case "actor":
				authority.Actor[0] ^= 1
			case "principal":
				authority.Principal[0] ^= 1
			case "key":
				authority.VerifiedKey[0] ^= 1
				expected = lxphost.Code(-201)
			case "signed payload":
				digest, err := h.host.SigningHash(activity)
				require.NoError(t, err)
				activity.Signature = ed25519.Sign(h.private, digest[:])
				activity.Payload[0] ^= 1
				authority.Bound = false
				expected = lxphost.Code(-201)
			}
			before := h.dump()
			receipt, err := h.host.Execute(lxphost.WithSequence(ctx, 1), activity, authority)
			require.ErrorIs(t, err, expected)
			require.Equal(t, lxphost.Receipt{}, receipt)
			require.Equal(t, before, h.dump())
		})
	}
}
