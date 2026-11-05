package layerxledger_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type ledgerParityActor struct {
	public [32]byte
	did    []byte
	id     [32]byte
}

func newLedgerParityActor(t *testing.T, seedByte byte) ledgerParityActor {
	t.Helper()
	var seed [ed25519.SeedSize]byte
	seed[0] = seedByte
	private := ed25519.NewKeyFromSeed(seed[:])
	var actor ledgerParityActor
	copy(actor.public[:], private.Public().(ed25519.PublicKey))
	actor.did = []byte("did:layerx:" + hex.EncodeToString(actor.public[:]))
	var err error
	actor.id, err = lxphost.DIDID(actor.did)
	require.NoError(t, err)
	return actor
}

func ledgerParityReceipt(t *testing.T, host *lxphost.Host, encodedLength int) *codec.Receipt {
	t.Helper()
	node := reflect.ValueOf(host).Elem().FieldByName("node")
	require.False(t, node.IsNil())
	arena := node.Elem().FieldByName("arena")
	buffer := arena.FieldByName("buffer")
	require.Equal(t, reflect.Ptr, buffer.Kind())
	require.False(t, buffer.IsNil())
	require.Positive(t, encodedLength)
	require.LessOrEqual(t, uint64(encodedLength), arena.FieldByName("capacity").Uint())
	require.GreaterOrEqual(t, arena.FieldByName("offset").Uint(), uint64(encodedLength))
	// Read the allocator's actual buffer; cgo's duplicated node declarations
	// do not agree on the offset of the later arena_bytes field.
	encoded := bytes.Clone(unsafe.Slice((*byte)(buffer.UnsafePointer()), encodedLength))
	receipt, err := codec.DecodeReceipt(encoded)
	require.NoError(t, err, "native receipt prefix: %x", encoded[:min(8, len(encoded))])
	return receipt
}

func assertLedgerKernelParity(t *testing.T, h *harness, caller common.Address, actor ledgerParityActor,
	method string, activityType uint32, payload []byte, key [32]byte, args ...interface{}) {
	t.Helper()
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), h.precompile.Address())
	direct, err := lxphost.NewHost(lxphost.Config{StoreKey: h.hostKey, NetworkID: 125, Epoch: 1,
		ParameterVersion: 1, TimestampWindowMs: 60_000, GasLimit: 10_000_000})
	require.NoError(t, err)
	t.Cleanup(direct.Close)
	before := storeSnapshot(t, h.ctx.KVStore(h.hostKey))
	branch, _ := h.ctx.CacheContext()
	sequence, err := lxphost.NextSequence(branch, h.hostKey)
	require.NoError(t, err)
	accountSequence, err := direct.ReadIdentitySequence(branch, actor.did)
	require.NoError(t, err)
	now := uint64(branch.BlockTime().UnixMilli())
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: accountSequence, NotBefore: now, NotAfter: now,
		IdempotencyKey: key, Payload: payload,
	}
	authority := lxphost.KernelAuthority{
		Bound: true, Actor: actor.id, Principal: actor.id, VerifiedKey: actor.public,
	}
	want, err := direct.Execute(lxphost.WithSequence(branch, sequence), activity, authority)
	if method == "issueGrant" || method == "draw" || method == "revokeGrant" {
		require.ErrorIs(t, err, lxphost.Code(-106))
		require.Empty(t, want.Encoded)
		require.Equal(t, before, storeSnapshot(t, branch.KVStore(h.hostKey)))
		logs := len(h.stateDB.GetAllLogs())
		data, err := assetCall(t, h, caller, method, args...)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_NO_PAYER_GRANT")
		require.Equal(t, before, storeSnapshot(t, h.ctx.KVStore(h.hostKey)))
		require.Len(t, h.stateDB.GetAllLogs(), logs)
		return
	}
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), want.ResultCode)
	require.Equal(t, before, storeSnapshot(t, h.ctx.KVStore(h.hostKey)))
	_, err = assetCall(t, h, caller, method, args...)
	require.NoError(t, err)
	got := ledgerParityReceipt(t, h.host, len(want.Encoded))
	require.NotEmpty(t, got.CanonicalBytes())
	require.Equal(t, int32(0), got.ResultCode)
	require.Equal(t, sequence, got.GlobalSequence)
	require.Equal(t, want.Timestamp, got.Timestamp)
	require.Equal(t, want.PreviousStateRoot, got.PreviousStateRoot)
	require.NotEqual(t, [32]byte{}, got.ResultingStateRoot)
	require.Equal(t, want.ResultingStateRoot, got.ResultingStateRoot)
	require.Equal(t, want.Encoded, got.CanonicalBytes())
}

func TestLedgerPrecompileKernelParity(t *testing.T) {
	t.Run("register", func(t *testing.T) {
		h := newAssetHarness(t)
		actor := newLedgerParityActor(t, 1)
		salt := [32]byte{0x61}
		material := append(append([]byte("LX:ASSET:v1"), actor.id[:]...), salt[:]...)
		asset := sha256.Sum256(material)
		payload, err := types.EncodeRegisterPayload(types.RegisterArgs{
			AssetID: asset, Salt: salt, Symbol: "PAR", Name: "Parity token", Decimals: 6,
			SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative,
		})
		require.NoError(t, err)
		assertLedgerKernelParity(t, h, h.caller, actor, "register", 0x00010001, payload, [32]byte{},
			asset, salt, "PAR", "Parity token", uint8(6), big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{})
	})

	t.Run("openAccount", func(t *testing.T) {
		h, _, asset := newTransferHarness(t)
		payload := types.EncodeOpenAccountPayload(asset)
		assertLedgerKernelParity(t, h, h.caller, newLedgerParityActor(t, 1), "openAccount", 0x00010004,
			payload, sha256.Sum256(payload), asset)
	})

	t.Run("mint", func(t *testing.T) {
		f := newPaymentFixture(t)
		payload, err := types.EncodeSupplyPayload(f.asset, f.from, sdk.NewInt(25))
		require.NoError(t, err)
		assertLedgerKernelParity(t, f.harness, f.caller, newLedgerParityActor(t, 1), "mint", 0x0001000a,
			payload, [32]byte{}, f.asset, f.from, big.NewInt(25))
	})

	t.Run("send", func(t *testing.T) {
		h, payer, asset := newTransferHarness(t)
		payee := transferRecipient(t, h)
		from := openTransferAccount(t, h, h.caller, payer, asset)
		to := openTransferAccount(t, h, common.Address{0x23, 2}, payee, asset)
		_, err := runTransferMethod(t, h, "mint", asset, from, big.NewInt(250))
		require.NoError(t, err)
		key := [32]byte{0x63}
		payload := transferEnvelope(t, h, payer, from, to, asset, 25, 0, key)
		assertLedgerKernelParity(t, h, h.caller, newLedgerParityActor(t, 1), "send", 0x00010005,
			payload, key, payload)
	})

	t.Run("issueGrant", func(t *testing.T) {
		f := newPaymentFixture(t)
		assertLedgerKernelParity(t, f.harness, f.caller, newLedgerParityActor(t, 1), "issueGrant", 0x00010007,
			f.grant, sha256.Sum256(f.grant), f.grant)
	})

	t.Run("draw", func(t *testing.T) {
		f := newPaymentFixture(t)
		data, err := f.call(t, f.caller, "issueGrant", f.grant)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_NO_PAYER_GRANT")
		key := [32]byte{0x64}
		payload, err := types.EncodeReceivePayload(types.ReceiveArgs{
			From: f.from, To: f.to, Asset: f.asset, Amount: sdk.NewInt(15), GrantID: f.grantID,
			ReceiverSequence: 0, IdempotencyKey: key, ContextHash: f.context, PublicKey: f.payee.DIDKey,
			NetworkID: 125, ProtocolVersion: codec.StateCommitmentProtocolVersion, PayerGrant: f.grant,
		})
		require.NoError(t, err)
		assertLedgerKernelParity(t, f.harness, f.payeeCaller, newLedgerParityActor(t, 2), "draw", 0x00010006,
			payload, key, f.grantID, f.to, big.NewInt(15), key, uint64(0), f.purpose, f.context)
	})

	t.Run("revokeGrant", func(t *testing.T) {
		f := newPaymentFixture(t)
		data, err := f.call(t, f.caller, "issueGrant", f.grant)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_NO_PAYER_GRANT")
		sequence, err := f.host.ReadIdentitySequence(f.ctx, f.payer.DID)
		require.NoError(t, err)
		payload := types.EncodeRevokeGrantPayload(f.grantID, sequence)
		assertLedgerKernelParity(t, f.harness, f.caller, newLedgerParityActor(t, 1), "revokeGrant", 0x00010008,
			payload, sha256.Sum256(payload), f.grantID, sequence)
	})
}
