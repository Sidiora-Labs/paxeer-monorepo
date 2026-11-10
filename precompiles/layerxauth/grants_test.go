package layerxauth_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type grantFixture struct {
	*harness
	host      *lxphost.Host
	actor     authkeeper.Actor
	key       [32]byte
	asset     [32]byte
	purpose   [32]byte
	timestamp uint64
}

func newGrantFixture(t *testing.T) *grantFixture {
	t.Helper()
	h := newHarness(t)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: h.hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	actor, err := h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	f := &grantFixture{harness: h, host: host, actor: actor, timestamp: uint64(h.ctx.BlockTime().UnixMilli()), asset: [32]byte{0x11}, purpose: [32]byte{0x22}}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	copy(f.key[:], private.Public().(ed25519.PublicKey))
	grantee := common.Address{0x21, 0x11, 0x03}
	evmKeeper := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evmKeeper.ChainID(h.ctx), grantee, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evmKeeper.BindLayerX(h.ctx, grantee, f.key, signature)
	require.NoError(t, err)
	registry := cdctypes.NewInterfaceRegistry()
	authz.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	authzKey := sdk.NewKVStoreKey("grant_authz")
	h.ctx = withStore(h.ctx, authzKey)
	az := authzkeeper.NewKeeper(authzKey, sdkcodec.NewProtoCodec(registry), baseapp.NewMsgServiceRouter())
	var authKey sdk.StoreKey
	for _, key := range h.ctx.MultiStore().StoreKeys() {
		if key.Name() == types.StoreKey {
			authKey = key
		}
	}
	require.NotNil(t, authKey)
	h.keeper = authkeeper.NewKeeper(sdkcodec.NewProtoCodec(registry), authKey, host, az, evmKeeper, "grant-test-authority")
	h.keepers.auth = h.keeper
	h.precompile, err = layerxauth.NewPrecompile(h.keepers)
	require.NoError(t, err)
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), actor.DIDKey[:]...)
	activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID, AuthorityKey: actor.DIDKey,
		NotBefore: f.timestamp, NotAfter: f.timestamp, IdempotencyKey: [32]byte{0x41}, Payload: registration}
	receipt, err := host.Execute(lxphost.WithSequence(h.ctx, 41), activity, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	return f
}

func (f *grantFixture) body(kind byte, key [32]byte) []byte {
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(value []byte) {
		body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
	}
	u64 := func(value uint64) { body = binary.BigEndian.AppendUint64(body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(f.actor.Authority.Actor[:])
	field(f.actor.Authority.Actor[:])
	body = append(body, kind)
	field(key[:])
	moduleMask := uint64(1 << 9)
	if kind == 3 {
		moduleMask = 1 << 1
	}
	u64(moduleMask)
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
	field(make([]byte, 64))
	return body
}

func grantCall(t *testing.T, f *grantFixture, evm *vm.EVM, method string, args ...interface{}) []interface{} {
	t.Helper()
	a := f.precompile.GetABI()
	input, err := a.Pack(method, args...)
	require.NoError(t, err)
	data, err := f.precompile.Run(evm, f.caller, f.precompile.Address(), input, nil, a.Methods[method].IsConstant(), false, nil)
	require.NoError(t, err, "%s returned %x", method, data)
	values, err := a.Methods[method].Outputs.Unpack(data)
	require.NoError(t, err)
	return values
}

func grantLogs(t *testing.T, f *grantFixture, db *state.DBImpl, id [32]byte, body []byte, kind string, summary []byte) {
	t.Helper()
	a := f.precompile.GetABI()
	logs := db.GetAllLogs()
	chunks := (len(body) + 255) / 256
	require.Len(t, logs, chunks+2)
	for _, log := range logs {
		require.Equal(t, f.precompile.Address(), log.Address)
		require.Len(t, log.Topics, 2)
	}
	require.Equal(t, []common.Hash{a.Events[kind].ID, common.Hash(id)}, logs[0].Topics)
	data, err := a.Events[kind].Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	if kind == "SessionGranted" {
		require.Equal(t, []interface{}{summary}, data)
	} else {
		require.Empty(t, data)
	}
	var reconstructed []byte
	sub := uint8(5)
	if kind == "CapabilityGranted" {
		sub = 8
	}
	for i, log := range logs[1 : chunks+1] {
		require.Equal(t, []common.Hash{a.Events["GrantBody"].ID, common.Hash(id)}, log.Topics)
		data, err := a.Events["GrantBody"].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, sub, data[0])
		require.Equal(t, uint8(i), data[1])
		chunk := data[2].([]byte)
		size := len(body) - i*256
		if size > 256 {
			size = 256
		}
		require.Len(t, chunk, size)
		reconstructed = append(reconstructed, chunk...)
	}
	require.Equal(t, body, reconstructed)
	require.Equal(t, []common.Hash{a.Events["IdentityStateUpdated"].ID, common.Hash(f.actor.Authority.Actor)}, logs[chunks+1].Topics)
}

func TestPrecompileGrantSession(t *testing.T) {
	f := newGrantFixture(t)
	evm, db := f.evm()
	body := f.body(2, f.key)
	id, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	action := [32]byte{0x33}
	values := grantCall(t, f, evm, "grantSession", body, uint64(100), action)
	require.Equal(t, []interface{}{id}, values)
	values = grantCall(t, f, evm, "sessionKey", id)
	summary := values[0].([]byte)
	require.Len(t, summary, 209)
	require.Equal(t, []byte("LXGS2"), summary[:5])
	require.Equal(t, id[:], summary[5:37])
	require.Equal(t, f.actor.Authority.Actor[:], summary[37:69])
	require.Equal(t, f.actor.DIDKey[:], summary[69:101])
	require.Equal(t, action[:], summary[101:133])
	require.Equal(t, f.key[:], summary[133:165])
	require.Equal(t, uint64(100), binary.BigEndian.Uint64(summary[165:173]))
	want, err := f.keeper.SessionKey(db.Ctx(), id)
	require.NoError(t, err)
	require.Equal(t, want, summary)
	grantLogs(t, f, db, id, body, "SessionGranted", summary)
	before := hostSnapshot(t, db.Ctx(), f.hostKey)
	input, err := f.precompile.GetABI().Pack("grantSession", body, uint64(100), action)
	require.NoError(t, err)
	snapshot := db.Snapshot()
	data, err := f.precompile.Run(evm, f.caller, f.precompile.Address(), input, nil, false, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLxpRevert(t, data, "LXP_SEQUENCE_REUSED")
	require.ErrorIs(t, db.GetPrecompileError(), types.ErrSequenceReused)
	db.RevertToSnapshot(snapshot)
	require.Equal(t, before, hostSnapshot(t, db.Ctx(), f.hostKey))
	require.Len(t, db.GetAllLogs(), 4)
}

func (f *grantFixture) feeSessionBody(key [32]byte, revocationSequence uint64) []byte {
	body := f.body(2, key)
	body[4] = 2
	const revocationOffset = 114 + 180 + 2*8
	binary.BigEndian.PutUint64(body[revocationOffset:revocationOffset+8], revocationSequence)
	body = binary.BigEndian.AppendUint32(body, 32)
	body = append(body, f.asset[:]...)
	for _, value := range []uint64{10, 100, 0} {
		body = binary.BigEndian.AppendUint64(body, 0)
		body = binary.BigEndian.AppendUint64(body, value)
	}
	body = binary.BigEndian.AppendUint64(body, 1000)
	for _, value := range []uint64{50, 0} {
		body = binary.BigEndian.AppendUint64(body, 0)
		body = binary.BigEndian.AppendUint64(body, value)
	}
	return binary.BigEndian.AppendUint64(body, f.timestamp-1000)
}

func TestPrecompileGrantSessionSuccessor(t *testing.T) {
	for _, count := range []int{1, 2} {
		name := "period exhausted"
		if count == 2 {
			name = "total exhausted"
		}
		t.Run(name, func(t *testing.T) {
			f := newGrantFixture(t)
			payload, fee := repairM1503FeesAndProgram(t, f)
			evm, db := f.evm()
			body := repairM1503Body(f, f.key, 41, fee)
			predecessor := grantCall(t, f, evm, "grantSession", body, uint64(100), [32]byte{0x33})[0].([32]byte)
			live, reason, err := f.keeper.IsLive(db.Ctx(), predecessor, 90, f.timestamp)
			require.NoError(t, err)
			require.True(t, live)
			require.Empty(t, reason)
			issued, err := f.host.ReadGrant(db.Ctx(), predecessor)
			require.NoError(t, err)
			require.True(t, issued.FeeBudget.Present)
			require.True(t, issued.FeeBudget.SpentTotal.IsZero())
			predecessorSummary := grantCall(t, f, evm, "sessionKey", predecessor)[0].([]byte)
			grantLogs(t, f, db, predecessor, body, "SessionGranted", predecessorSummary)
			ctx := db.Ctx()
			var charged lxphost.AuthorityGrant
			for i := 0; i < count; i++ {
				ctx = ctx.WithBlockTime(time.UnixMilli(int64(f.timestamp) + int64(i)*1000).UTC())
				charged = repairM1503Charge(t, f, ctx, predecessor, payload, fee)
				before := hostSnapshot(t, ctx, f.hostKey)
				refused, err := repairM1503Execute(t, f, ctx, 0x00090003, payload, fee, predecessor)
				require.ErrorIs(t, err, lxphost.Code(-211))
				require.Empty(t, refused.Encoded)
				require.Equal(t, before, hostSnapshot(t, ctx, f.hostKey))
			}
			sequence, err := lxphost.NextSequence(ctx, f.hostKey)
			require.NoError(t, err)
			revoke, err := f.keeper.RevokeGrant(ctx, f.actor, predecessor, 1, sequence, [32]byte{0x35})
			require.NoError(t, err)
			require.Equal(t, sequence, revoke.GlobalSequence)
			grant, err := f.host.ReadGrant(ctx, predecessor)
			require.NoError(t, err)
			require.True(t, grant.Revoked)
			require.Equal(t, revoke.GlobalSequence, grant.RevokedAtSequence)
			require.Equal(t, charged.FeeBudget, grant.FeeBudget)
			revocationSequence, err := f.keeper.RevocationSequence(ctx, f.actor.DIDKey)
			require.NoError(t, err)
			require.Equal(t, revoke.GlobalSequence, revocationSequence)
			require.Equal(t, sdk.NewIntFromUint64(uint64(count)*fee), grant.FeeBudget.SpentTotal)
			require.Equal(t, sdk.NewIntFromUint64(fee), grant.FeeBudget.SpentThisPeriod)
			require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), grant.FeeBudget.PeriodStart)
			repairM1503Reopen(t, f, ctx)
			persisted, err := f.host.ReadGrant(f.ctx, predecessor)
			require.NoError(t, err)
			require.Equal(t, grant, persisted)
			private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{75}, ed25519.SeedSize))
			var key [32]byte
			copy(key[:], private.Public().(ed25519.PublicKey))
			successorBody := repairM1503Body(f, key, revocationSequence, fee)
			id, err := codec.DomainHash(codec.DomainAuthorityHash, successorBody)
			require.NoError(t, err)
			commitment := repairM1503Commitment(persisted)
			evm, db = f.evm()
			t.Run("wrong expected commitment", func(t *testing.T) {
				before := hostSnapshot(t, db.Ctx(), f.hostKey)
				wrong := commitment
				wrong[0] ^= 1
				input, err := f.precompile.GetABI().Pack("grantSessionSuccessor", successorBody, uint64(120), [32]byte{0x34}, predecessor, wrong)
				require.NoError(t, err)
				snapshot := db.Snapshot()
				data, err := f.precompile.Run(evm, f.caller, f.precompile.Address(), input, nil, false, false, nil)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				assertLxpRevert(t, data, "LXP_AUTH_SCOPE")
				require.ErrorIs(t, db.GetPrecompileError(), types.ErrAuthScope)
				db.RevertToSnapshot(snapshot)
				require.Equal(t, before, hostSnapshot(t, db.Ctx(), f.hostKey))
				require.Empty(t, db.GetAllLogs())
			})
			values := grantCall(t, f, evm, "grantSessionSuccessor", successorBody, uint64(120), [32]byte{0x34}, predecessor, commitment)
			require.Equal(t, []interface{}{id}, values)
			summary := grantCall(t, f, evm, "sessionKey", id)[0].([]byte)
			require.Len(t, summary, 209)
			require.Equal(t, key[:], summary[133:165])
			require.Equal(t, uint64(120), binary.BigEndian.Uint64(summary[165:173]))
			require.Equal(t, revocationSequence, binary.BigEndian.Uint64(summary[201:209]))
			grantLogs(t, f, db, id, successorBody, "SessionGranted", summary)
			successor, err := f.host.ReadGrant(db.Ctx(), id)
			require.NoError(t, err)
			require.False(t, successor.Revoked)
			require.Equal(t, grant.FeeBudget, successor.FeeBudget)
			sequence, err = f.host.ReadLastSequence(db.Ctx(), f.actor.DID)
			require.NoError(t, err)
			require.Greater(t, sequence, revoke.GlobalSequence)
			live, reason, err = f.keeper.IsLive(db.Ctx(), id, sequence, uint64(ctx.BlockTime().UnixMilli()))
			require.NoError(t, err)
			require.True(t, live)
			require.Empty(t, reason)
			live, reason, err = f.keeper.IsLive(db.Ctx(), predecessor, sequence, uint64(ctx.BlockTime().UnixMilli()))
			require.NoError(t, err)
			require.False(t, live)
			require.Equal(t, "LXP_AUTH_REVOKED", reason)
			repairM1503Reopen(t, f, db.Ctx())
			recovered, err := f.host.ReadGrant(f.ctx, id)
			require.NoError(t, err)
			require.Equal(t, successor, recovered)
			repairM1503AssertExhausted(t, f, id, fee, count == 2)
		})
	}
}

func TestPrecompileGrantCapability(t *testing.T) {
	f := newGrantFixture(t)
	evm, db := f.evm()
	body := f.body(3, f.key)
	t.Run("unsupported SDK scope", func(t *testing.T) {
		unsupported := bytes.Clone(body)
		binary.BigEndian.PutUint64(unsupported[114:122], 1<<9)
		before := hostSnapshot(t, db.Ctx(), f.hostKey)
		input, err := f.precompile.GetABI().Pack("grantCapability", unsupported)
		require.NoError(t, err)
		snapshot := db.Snapshot()
		data, err := f.precompile.Run(evm, f.caller, f.precompile.Address(), input, nil, false, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLxpRevert(t, data, "LXP_AUTH_SCOPE")
		require.ErrorIs(t, db.GetPrecompileError(), types.ErrAuthScope)
		db.RevertToSnapshot(snapshot)
		require.Equal(t, before, hostSnapshot(t, db.Ctx(), f.hostKey))
		require.Empty(t, db.GetAllLogs())
	})
	id, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	values := grantCall(t, f, evm, "grantCapability", body)
	require.Equal(t, []interface{}{id}, values)
	grantLogs(t, f, db, id, body, "CapabilityGranted", nil)
	before := hostSnapshot(t, db.Ctx(), f.hostKey)
	values = grantCall(t, f, evm, "capabilityScope", id)
	want := []interface{}{uint64(1 << 1), uint16(4), uint16(5), f.asset, big.NewInt(10), big.NewInt(20), big.NewInt(0), uint64(1000), big.NewInt(15), big.NewInt(0), f.timestamp - 1000, f.purpose}
	require.Len(t, values, len(want))
	for i, expected := range want {
		if amount, ok := expected.(*big.Int); ok {
			require.IsType(t, amount, values[i])
			require.Zero(t, amount.Cmp(values[i].(*big.Int)), "scope field %d", i)
		} else {
			require.Equal(t, expected, values[i], "scope field %d", i)
		}
	}
	require.Equal(t, []interface{}{""}, grantCall(t, f, evm, "checkActivity", id, uint32(0x00010005), f.asset, big.NewInt(5)))
	require.Equal(t, []interface{}{"LXP_AUTH_SCOPE"}, grantCall(t, f, evm, "checkActivity", id, uint32(0x00090004), f.asset, big.NewInt(5)))
	require.Equal(t, before, hostSnapshot(t, db.Ctx(), f.hostKey))
	stored, err := f.keeper.GetGrant(db.Ctx(), id)
	require.NoError(t, err)
	require.Equal(t, body, stored)
}
