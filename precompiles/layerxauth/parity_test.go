package layerxauth_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativeSessionFixture struct {
	activity  lxphost.Activity
	network   uint32
	id        [32]byte
	receipt   *codec.Receipt
	body      []byte
	expiry    uint64
	action    [32]byte
	successor *authtypes.SessionSuccessor
}

func readNativeSessionFixture(t *testing.T, name string) nativeSessionFixture {
	t.Helper()
	dir := filepath.Join("..", "..", "tests", "fixtures", "authority", "native-sessions", name)
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		return data
	}
	raw := read("grant.activity")
	r := bytes.NewReader(raw)
	value := func(out interface{}) {
		require.NoError(t, binary.Read(r, binary.BigEndian, out))
	}
	field := func() []byte {
		var size uint32
		value(&size)
		require.LessOrEqual(t, uint64(size), uint64(r.Len()))
		out := make([]byte, int(size))
		_, err := io.ReadFull(r, out)
		require.NoError(t, err)
		return out
	}
	tag := func(want byte) {
		got, err := r.ReadByte()
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	var f nativeSessionFixture
	var version, structure uint16
	value(&version)
	value(&structure)
	require.Equal(t, uint16(3), version)
	require.Equal(t, uint16(0x1001), structure)
	tag(12)
	tag(1)
	value(&f.activity.ProtocolVersion)
	require.Equal(t, version, f.activity.ProtocolVersion)
	tag(2)
	value(&f.network)
	tag(3)
	value(&f.activity.Type)
	require.Equal(t, uint32(0x00070005), f.activity.Type)
	tag(4)
	f.activity.ActorDID = field()
	tag(5)
	key := field()
	require.Len(t, key, 32)
	copy(f.activity.AuthorityKey[:], key)
	require.Equal(t, evmtypes.LayerXDid(f.activity.AuthorityKey), string(f.activity.ActorDID))
	tag(6)
	value(&f.activity.AccountSequence)
	tag(7)
	value(&f.activity.NotBefore)
	value(&f.activity.NotAfter)
	tag(8)
	idempotency := field()
	require.Len(t, idempotency, 32)
	copy(f.activity.IdempotencyKey[:], idempotency)
	tag(9)
	value(&f.activity.FeeLimit)
	tag(10)
	payloadHash := field()
	tag(11)
	f.activity.Payload = field()
	gotPayloadHash, err := codec.DomainHash(codec.DomainPayloadHash, f.activity.Payload)
	require.NoError(t, err)
	require.Equal(t, gotPayloadHash[:], payloadHash)
	tag(12)
	f.activity.Signature = field()
	require.Len(t, f.activity.Signature, 64)
	require.Zero(t, r.Len())
	id := read("grant.activity-id")
	require.Len(t, id, 32)
	copy(f.id[:], id)
	gotID, err := codec.DomainHash(codec.DomainActivityID, raw)
	require.NoError(t, err)
	require.Equal(t, f.id, gotID)
	f.receipt, err = codec.DecodeReceipt(read("grant.receipt"))
	require.NoError(t, err)
	require.Equal(t, f.id, f.receipt.ActivityID)
	require.Equal(t, f.activity.ProtocolVersion, f.receipt.ProtocolVersion)

	r = bytes.NewReader(f.activity.Payload)
	tag(0x71)
	tag(5)
	variant, err := r.ReadByte()
	require.NoError(t, err)
	switch variant {
	case 1:
		tag(3)
	case 2:
		tag(5)
		f.successor = &authtypes.SessionSuccessor{}
	default:
		t.Fatalf("unknown native session variant %d", variant)
	}
	f.body = field()
	require.GreaterOrEqual(t, len(f.body), 5)
	value(&f.expiry)
	fixed := func(out *[32]byte) {
		data := field()
		require.Len(t, data, len(out))
		copy(out[:], data)
	}
	fixed(&f.action)
	if f.successor != nil {
		fixed(&f.successor.PredecessorID)
		fixed(&f.successor.ExpectedCommitment)
	}
	require.Zero(t, r.Len())
	payload, err := authtypes.EncodeSessionPayload(f.body, f.expiry, f.action, f.successor)
	require.NoError(t, err)
	require.Equal(t, f.activity.Payload, payload)
	return f
}

func nativeSessionHost(t *testing.T, f nativeSessionFixture) (*harness, *lxphost.Host, authkeeper.Actor) {
	t.Helper()
	h := newHarness(t)
	h.ctx = h.ctx.WithBlockTime(time.UnixMilli(int64(f.activity.NotBefore)).UTC())
	evmKeeper := &testkeeper.EVMTestApp.EvmKeeper
	_, _, err := evmKeeper.UnbindLayerX(h.ctx, h.caller)
	require.NoError(t, err)
	bindings := h.ctx.KVStore(evmKeeper.GetStoreKey())
	bindings.Set(evmtypes.EVMAddressToLayerXDidKey(h.caller), f.activity.AuthorityKey[:])
	bindings.Set(evmtypes.LayerXDidToEVMAddressKey(f.activity.AuthorityKey), h.caller[:])
	require.NoError(t, evmKeeper.ValidateLayerXBindings(h.ctx))
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: h.hostKey, NetworkID: f.network, ParameterVersion: f.receipt.ParameterVersion,
		TimestampWindowMs: f.activity.NotAfter - f.activity.NotBefore, GasLimit: 1_000_000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	var authKey sdk.StoreKey
	for _, key := range h.ctx.MultiStore().StoreKeys() {
		if key.Name() == authtypes.StoreKey {
			authKey = key
		}
	}
	require.NotNil(t, authKey)
	h.keeper = authkeeper.NewKeeper(testkeeper.EVMTestApp.AppCodec(), authKey, host,
		&testkeeper.EVMTestApp.AuthzKeeper, evmKeeper, "native-session-parity")
	h.keepers.auth = h.keeper
	h.precompile, err = layerxauth.NewPrecompile(h.keepers)
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x101C"), h.precompile.Address())
	actor, err := h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	require.Equal(t, f.activity.ActorDID, actor.DID)
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), actor.DIDKey[:]...)
	registered, err := host.Execute(lxphost.WithSequence(h.ctx, 3), lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID, AuthorityKey: actor.DIDKey,
		NotBefore: f.activity.NotBefore, NotAfter: f.activity.NotBefore,
		IdempotencyKey: [32]byte{1}, Payload: registration,
	}, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), registered.ResultCode)
	if f.body[4] == 2 {
		require.Len(t, f.body, 527)
		var policyKey, policy [32]byte
		copy(policyKey[:], "native-fee-authority-version")
		policy[31] = 2
		h.ctx.KVStore(h.hostKey).Set(append([]byte{1, 0, 7}, policyKey[:]...), policy[:])
		fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
		for i := 0; i < 7; i++ {
			fee = binary.BigEndian.AppendUint64(fee, 1)
		}
		fee = append(fee, f.body[399:431]...)
		for _, n := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
			fee = binary.BigEndian.AppendUint64(fee, n)
		}
		fee = append(fee, bytes.Repeat([]byte{0xa5}, 32)...)
		fee = append(fee, make([]byte, 16)...)
		require.Len(t, fee, 217)
		h.ctx.KVStore(h.hostKey).Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), fee)
	}
	return h, host, actor
}

func nativeSessionReceiptResult(t *testing.T, host *lxphost.Host) (int32, uint64) {
	t.Helper()
	node := reflect.ValueOf(host).Elem().FieldByName("node")
	require.False(t, node.IsNil())
	receipt := node.Elem().FieldByName("receipt")
	require.Equal(t, reflect.Struct, receipt.Kind())
	require.Equal(t, uint64(3), receipt.FieldByName("protocol_version").Uint())
	return int32(receipt.FieldByName("result_code").Int()), receipt.FieldByName("global_sequence").Uint()
}

func TestAuthPrecompileKernelParity(t *testing.T) {
	for _, name := range []string{"authentication", "fee", "legacy", "replacement"} {
		t.Run(name, func(t *testing.T) {
			if name == "replacement" {
				repairM1503SuccessorParity(t)
				return
			}
			f := readNativeSessionFixture(t, name)
			grantID, err := codec.DomainHash(codec.DomainAuthorityHash, f.body)
			require.NoError(t, err)
			h, host, _ := nativeSessionHost(t, f)
			h.ctx.KVStore(h.hostKey).Set([]byte{2}, binary.BigEndian.AppendUint64(nil, f.receipt.GlobalSequence-1))
			a := h.precompile.GetABI()
			method := "grantSession"
			args := []interface{}{f.body, f.expiry, f.action}
			if f.successor != nil {
				method = "grantSessionSuccessor"
				args = append(args, f.successor.PredecessorID, f.successor.ExpectedCommitment)
			}
			input, err := a.Pack(method, args...)
			require.NoError(t, err)
			evm, db := h.evm()
			output, callErr := h.precompile.Run(evm, h.caller, h.precompile.Address(), input, nil, false, false, nil)
			assert.NoError(t, callErr, "%s returned %x; precompile error: %v", method, output, db.GetPrecompileError())
			if callErr == nil {
				values, err := a.Unpack(method, output)
				require.NoError(t, err)
				require.Len(t, values, 1)
				assert.Equal(t, grantID, values[0], "0x101C grant id must equal the fixture grant-body authority hash")
			}
			resultCode, sequence := nativeSessionReceiptResult(t, host)
			assert.Equal(t, f.receipt.GlobalSequence, sequence, "native fixture receipt sequence")
			assert.Equal(t, f.receipt.ResultCode, resultCode, "native fixture receipt result")
			require.NoError(t, callErr, "%s returned %x", method, output)
		})
	}
}

func repairM1503SuccessorParity(t *testing.T) {
	t.Helper()
	f := newGrantFixture(t)
	payload, fee := repairM1503FeesAndProgram(t, f)
	evm, db := f.evm()
	predecessorBody := repairM1503Body(f, f.key, 41, fee)
	predecessorID := grantCall(t, f, evm, "grantSession", predecessorBody, uint64(100), [32]byte{0x33})[0].([32]byte)
	charged := repairM1503Charge(t, f, db.Ctx(), predecessorID, payload, fee)
	require.Equal(t, sdk.NewIntFromUint64(fee), charged.FeeBudget.SpentTotal)
	require.Equal(t, charged.FeeBudget.SpentTotal, charged.FeeBudget.SpentThisPeriod)
	sequence, err := lxphost.NextSequence(db.Ctx(), f.hostKey)
	require.NoError(t, err)
	revoked, err := f.keeper.RevokeGrant(db.Ctx(), f.actor, predecessorID, 1, sequence, [32]byte{0x35})
	require.NoError(t, err)
	require.Zero(t, revoked.ResultCode)
	predecessor, err := f.host.ReadGrant(db.Ctx(), predecessorID)
	require.NoError(t, err)
	require.True(t, predecessor.Revoked)
	require.Equal(t, charged.FeeBudget, predecessor.FeeBudget)
	require.Equal(t, revoked.GlobalSequence, predecessor.RevokedAtSequence)
	commitment := repairM1503Commitment(predecessor)
	repairM1503Reopen(t, f, db.Ctx())
	persisted, err := f.host.ReadGrant(f.ctx, predecessorID)
	require.NoError(t, err)
	require.Equal(t, predecessor, persisted)
	require.Equal(t, commitment, repairM1503Commitment(persisted))
	var successorKey [32]byte
	copy(successorKey[:], ed25519.NewKeyFromSeed(bytes.Repeat([]byte{75}, ed25519.SeedSize)).Public().(ed25519.PublicKey))
	successorBody := repairM1503Body(f, successorKey, predecessor.RevokedAtSequence, fee)
	successorID, err := codec.DomainHash(codec.DomainAuthorityHash, successorBody)
	require.NoError(t, err)
	action := [32]byte{0x34}
	a := f.precompile.GetABI()
	method := a.Methods["grantSessionSuccessor"]
	packed, err := method.Inputs.Pack(successorBody, uint64(120), action, predecessorID, commitment)
	require.NoError(t, err)
	preimage := append([]byte("layerxauth/grants/v1"), f.actor.DIDKey[:]...)
	preimage = append(preimage, method.ID...)
	idempotency := sha256.Sum256(append(preimage, packed...))
	nativePayload, err := authtypes.EncodeSessionPayload(successorBody, 120, action, &authtypes.SessionSuccessor{
		PredecessorID: predecessorID, ExpectedCommitment: commitment})
	require.NoError(t, err)
	nativeCtx, _ := f.ctx.CacheContext()
	activity := repairM1503Activity(t, f, nativeCtx, 0x00070005, nativePayload, 0, f.actor.DIDKey)
	activity.IdempotencyKey = idempotency
	sequence, err = lxphost.NextSequence(nativeCtx, f.hostKey)
	require.NoError(t, err)
	receipt, err := f.host.Execute(lxphost.WithSequence(nativeCtx, sequence), activity, f.actor.Authority)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.Equal(t, sequence, receipt.GlobalSequence)
	nativeGrant, err := f.host.ReadGrant(nativeCtx, successorID)
	require.NoError(t, err)
	nativeSummary, err := f.host.ReadSessionKey(nativeCtx, successorID)
	require.NoError(t, err)
	events, err := authkeeper.AuthEvents(receipt)
	require.NoError(t, err)
	require.Len(t, events, 5)
	nativeState := hostSnapshot(t, nativeCtx, f.hostKey)
	evm, db = f.evm()
	result := grantCall(t, f, evm, "grantSessionSuccessor", successorBody, uint64(120), action, predecessorID, commitment)
	require.Equal(t, []interface{}{successorID}, result)
	code, globalSequence := nativeSessionReceiptResult(t, f.host)
	require.Equal(t, int32(receipt.ResultCode), code)
	require.Equal(t, receipt.GlobalSequence, globalSequence)
	evmGrant, err := f.host.ReadGrant(db.Ctx(), successorID)
	require.NoError(t, err)
	require.Equal(t, nativeGrant, evmGrant)
	require.False(t, evmGrant.Revoked)
	require.Equal(t, predecessor.FeeBudget, evmGrant.FeeBudget)
	require.Equal(t, nativeState, hostSnapshot(t, db.Ctx(), f.hostKey))
	summary := grantCall(t, f, evm, "sessionKey", successorID)[0].([]byte)
	require.Equal(t, nativeSummary, summary)
	grantLogs(t, f, db, successorID, successorBody, "SessionGranted", summary)
	logs := db.GetAllLogs()
	require.Len(t, logs, len(events))
	for i, event := range events {
		var indexed common.Hash
		var values []interface{}
		switch event.Name {
		case "SessionGranted":
			indexed = common.Hash(event.GrantID)
			values = []interface{}{event.Body}
		case "GrantBody":
			indexed = common.Hash(event.GrantID)
			values = []interface{}{event.Sub, event.Chunk, event.Body}
		case "IdentityStateUpdated":
			indexed = common.Hash(event.DIDID)
			values = []interface{}{event.Body}
		default:
			t.Fatalf("unexpected receipt event %s", event.Name)
		}
		require.Equal(t, []common.Hash{a.Events[event.Name].ID, indexed}, logs[i].Topics)
		encoded, err := a.Events[event.Name].Inputs.NonIndexed().Pack(values...)
		require.NoError(t, err)
		require.Equal(t, encoded, logs[i].Data)
	}
	repairM1503Reopen(t, f, db.Ctx())
	recovered, err := f.host.ReadGrant(f.ctx, successorID)
	require.NoError(t, err)
	require.Equal(t, nativeGrant, recovered)
	repairM1503AssertExhausted(t, f, successorID, fee, false)
}
