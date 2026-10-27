package layerxauth_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type precompileViewFixture struct {
	*harness
	host                           *lxphost.Host
	private                        ed25519.PrivateKey
	public, didID, delegate, asset [32]byte
	capabilityKey                  [32]byte
	purpose                        [32]byte
	did                            []byte
	timestamp                      uint64
	sessionID, capabilityID        [32]byte
	sessionBody, capabilityBody    []byte
	sessionActivity                lxphost.Activity
	sessionReceipt                 lxphost.Receipt
}

func newPrecompileViewFixture(t *testing.T) *precompileViewFixture {
	t.Helper()
	f := &precompileViewFixture{harness: newHarness(t)}
	var err error
	f.host, err = lxphost.NewHost(lxphost.Config{
		StoreKey: f.hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(f.host.Close)
	f.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, ed25519.SeedSize))
	copy(f.public[:], f.private.Public().(ed25519.PublicKey))
	delegate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize))
	copy(f.delegate[:], delegate.Public().(ed25519.PublicKey))
	capabilityKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, ed25519.SeedSize))
	copy(f.capabilityKey[:], capabilityKey.Public().(ed25519.PublicKey))
	f.did = []byte(evmtypes.LayerXDid(f.public))
	f.didID, err = lxphost.DIDID(f.did)
	require.NoError(t, err)
	f.timestamp = uint64(f.ctx.BlockTime().UnixMilli())
	f.asset, f.purpose = [32]byte{0x11}, [32]byte{0x22}
	registration := append(append([]byte{0x71, 1, 0, 2}, f.didID[:]...), f.public[:]...)
	f.execute(t, 41, f.activity(t, 0x00070001, 0, registration))
	f.sessionBody = f.grantBody(2)
	f.sessionID, err = codec.DomainHash(codec.DomainAuthorityHash, f.sessionBody)
	require.NoError(t, err)
	payload, err := types.EncodeSessionPayload(f.sessionBody, 100, [32]byte{0x33}, nil)
	require.NoError(t, err)
	f.sessionActivity = f.activity(t, 0x00070005, 1, payload)
	f.sessionReceipt = f.execute(t, 42, f.sessionActivity)
	f.capabilityBody = f.grantBody(3)
	f.capabilityID, err = codec.DomainHash(codec.DomainAuthorityHash, f.capabilityBody)
	require.NoError(t, err)
	payload, err = types.EncodeCapabilityPayload(f.capabilityBody)
	require.NoError(t, err)
	f.execute(t, 43, f.activity(t, 0x00070008, 2, payload))
	return f
}

func (f *precompileViewFixture) grantBody(kind byte) []byte {
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(value []byte) {
		body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
	}
	u64 := func(value uint64) { body = binary.BigEndian.AppendUint64(body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(f.didID[:])
	field(f.didID[:])
	body = append(body, kind)
	if kind == 2 {
		field(f.delegate[:])
	} else {
		field(f.capabilityKey[:])
	}
	u64(1 << 9)
	body = binary.BigEndian.AppendUint16(body, 4)
	body = binary.BigEndian.AppendUint16(body, 5)
	if kind == 2 {
		field(make([]byte, 32))
		body = append(body, make([]byte, 96)...)
		field(make([]byte, 32))
	} else {
		field(f.asset[:])
		u128(10)
		u128(20)
		u128(0)
		u64(1000)
		u128(15)
		u128(0)
		u64(f.timestamp - 1000)
		field(f.purpose[:])
	}
	u64(f.timestamp - 1000)
	u64(f.timestamp + 1000)
	u64(41)
	body = append(body, 0)
	u64(0)
	field(make([]byte, ed25519.SignatureSize))
	return body
}

func (f *precompileViewFixture) activity(t *testing.T, activityType uint32, sequence uint64, payload []byte) lxphost.Activity {
	t.Helper()
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: f.did, AuthorityKey: f.public,
		AccountSequence: sequence, NotBefore: f.timestamp - 1000, NotAfter: f.timestamp + 1000,
		IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(append([]byte(nil), f.did...), sequence)),
		Payload:        payload,
	}
	hash, err := f.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(f.private, hash[:])
	return activity
}

func (f *precompileViewFixture) execute(t *testing.T, sequence uint64, activity lxphost.Activity) lxphost.Receipt {
	t.Helper()
	receipt, err := f.host.Execute(lxphost.WithSequence(f.ctx, sequence), activity, lxphost.KernelAuthority{
		Actor: f.didID, Principal: f.didID, VerifiedKey: f.public,
	})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	return receipt
}

func (f *precompileViewFixture) rawView(t *testing.T, caller common.Address, method string, args ...interface{}) []byte {
	t.Helper()
	a := f.precompile.GetABI()
	input, err := a.Pack(method, args...)
	require.NoError(t, err)
	evm, db := f.evm()
	before := hostSnapshot(t, db.Ctx(), f.hostKey)
	data, err := f.precompile.Run(evm, caller, f.precompile.Address(), input, nil, true, false, nil)
	require.Equal(t, before, hostSnapshot(t, db.Ctx(), f.hostKey))
	require.Empty(t, db.Ctx().EventManager().Events())
	require.NoError(t, err, "staticcall %s: %x", method, data)
	return data
}

func (f *precompileViewFixture) view(t *testing.T, caller common.Address, method string, args ...interface{}) []interface{} {
	t.Helper()
	a := f.precompile.GetABI()
	data := f.rawView(t, caller, method, args...)
	output, err := a.Methods[method].Outputs.Unpack(data)
	require.NoError(t, err)
	return output
}

func TestPrecompileAuthViews(t *testing.T) {
	f := newPrecompileViewFixture(t)
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true, IdempotencyRetention: 4096}))
	grant, err := f.keeper.GetGrant(f.ctx, f.capabilityID)
	require.NoError(t, err)
	require.Equal(t, f.capabilityBody, grant)
	summary, err := f.keeper.SessionKey(f.ctx, f.sessionID)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(summary, []byte("LXGS2")))
	scope, err := f.keeper.CapabilityScope(f.ctx, f.capabilityID)
	require.NoError(t, err)
	require.Equal(t, f.capabilityBody[114:294], scope)
	for _, caller := range []struct {
		name    string
		address common.Address
	}{{"bound", f.caller}, {"unbound", f.unbound}} {
		t.Run(caller.name, func(t *testing.T) {
			t.Run("getGrant", func(t *testing.T) {
				require.Equal(t, []interface{}{grant, false, uint64(0)}, f.view(t, caller.address, "getGrant", f.capabilityID))
			})
			t.Run("sessionKey", func(t *testing.T) {
				require.Equal(t, []interface{}{summary}, f.view(t, caller.address, "sessionKey", f.sessionID))
			})
			t.Run("capabilityScope", func(t *testing.T) {
				expected := []interface{}{
					uint64(1 << 9), uint16(4), uint16(5), f.asset, big.NewInt(10), big.NewInt(20), big.NewInt(0),
					uint64(1000), big.NewInt(15), big.NewInt(0), f.timestamp - 1000, f.purpose,
				}
				a := f.precompile.GetABI()
				encoded, err := a.Methods["capabilityScope"].Outputs.Pack(expected...)
				require.NoError(t, err)
				require.Equal(t, encoded, f.rawView(t, caller.address, "capabilityScope", f.capabilityID))
			})
		})
	}
	for _, name := range []string{"getGrant", "sessionKey", "capabilityScope"} {
		t.Run(name+"Missing", func(t *testing.T) {
			a := f.precompile.GetABI()
			input, err := a.Pack(name, [32]byte{0xff})
			require.NoError(t, err)
			evm, db := f.evm()
			before := hostSnapshot(t, db.Ctx(), f.hostKey)
			data, err := f.precompile.Run(evm, f.caller, f.precompile.Address(), input, nil, true, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLxpRevert(t, data, layerxauth.KernelCodeText(lxphost.Code(-7)))
			require.Equal(t, before, hostSnapshot(t, db.Ctx(), f.hostKey))
		})
	}
}

func TestPrecompileIsLive(t *testing.T) {
	f := newPrecompileViewFixture(t)
	for _, tc := range []struct {
		name      string
		timestamp uint64
		live      bool
		reason    string
	}{
		{"live", f.timestamp, true, ""},
		{"notYetValid", f.timestamp - 1001, false, "LXP_NOT_YET_VALID"},
		{"expired", f.timestamp + 1000, false, "LXP_AUTH_EXPIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, []interface{}{tc.live, tc.reason}, f.view(t, f.unbound, "isLive", f.capabilityID, uint64(43), tc.timestamp))
		})
	}
	f.execute(t, 44, f.activity(t, 0x00070006, 3, types.EncodeRevokePayload(f.capabilityID, 1, 44)))
	require.Equal(t, []interface{}{false, "LXP_AUTH_REVOKED"}, f.view(t, f.unbound, "isLive", f.capabilityID, uint64(44), f.timestamp))
	body, err := f.keeper.GetGrant(f.ctx, f.capabilityID)
	require.NoError(t, err)
	require.Equal(t, []interface{}{body, true, uint64(44)}, f.view(t, f.unbound, "getGrant", f.capabilityID))
	a := f.precompile.GetABI()
	input, err := a.Pack("isLive", f.capabilityID, uint64(44), uint64(math.MaxUint64))
	require.NoError(t, err)
	evm, db := f.evm()
	data, err := f.precompile.Run(evm, f.unbound, f.precompile.Address(), input, nil, true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLxpRevert(t, data, types.ErrNonCanonical.Error())
	require.ErrorIs(t, db.GetPrecompileError(), types.ErrNonCanonical)
}

func TestPrecompileCheckActivity(t *testing.T) {
	f := newPrecompileViewFixture(t)
	for _, tc := range []struct {
		name         string
		activityType uint32
		asset        [32]byte
		amount       *big.Int
		reason       string
	}{
		{"inside", 0x00090004, f.asset, big.NewInt(5), ""},
		{"boundary", 0x00090005, f.asset, big.NewInt(10), ""},
		{"module", 0x00080004, f.asset, big.NewInt(5), "LXP_AUTH_SCOPE"},
		{"lowOrdinal", 0x00090003, f.asset, big.NewInt(5), "LXP_AUTH_SCOPE"},
		{"highOrdinal", 0x00090006, f.asset, big.NewInt(5), "LXP_AUTH_SCOPE"},
		{"asset", 0x00090004, [32]byte{2}, big.NewInt(5), "LXP_AUTH_SCOPE"},
		{"amount", 0x00090004, f.asset, big.NewInt(11), "LXP_AUTH_SCOPE"},
		{"maxUint128", 0x00090004, f.asset, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)), "LXP_AUTH_SCOPE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, []interface{}{tc.reason}, f.view(t, f.unbound, "checkActivity", f.capabilityID, tc.activityType, tc.asset, tc.amount))
		})
	}
	f.execute(t, 44, f.activity(t, 0x00070006, 3, types.EncodeRevokePayload(f.capabilityID, 1, 44)))
	require.Equal(t, []interface{}{"LXP_AUTH_REVOKED"}, f.view(t, f.unbound, "checkActivity", f.capabilityID, uint32(0x00090004), f.asset, big.NewInt(5)))
}

func TestPrecompileResultByIdempotency(t *testing.T) {
	f := newPrecompileViewFixture(t)
	stored, err := f.keeper.ResultByIdempotency(f.ctx, f.public, f.sessionActivity.IdempotencyKey)
	require.NoError(t, err)
	require.NotEmpty(t, stored)
	recorded, err := f.host.ReadIdempotentReceipt(f.ctx, f.did, f.sessionActivity.IdempotencyKey)
	require.NoError(t, err)
	require.Equal(t, recorded.Encoded, stored)
	output := f.view(t, f.unbound, "resultByIdempotency", f.public, f.sessionActivity.IdempotencyKey)
	require.Equal(t, []interface{}{stored}, output)
	receipt, err := codec.DecodeReceipt(output[0].([]byte))
	require.NoError(t, err)
	require.Equal(t, uint64(42), receipt.GlobalSequence)
	for _, tc := range []struct {
		name string
		did  [32]byte
		key  [32]byte
	}{
		{"unknownKey", f.public, [32]byte{0xff}},
		{"otherIdentity", f.delegate, f.sessionActivity.IdempotencyKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := f.view(t, f.unbound, "resultByIdempotency", tc.did, tc.key)
			require.Len(t, output, 1)
			require.Empty(t, output[0].([]byte))
		})
	}
}
