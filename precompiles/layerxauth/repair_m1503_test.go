package layerxauth_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth/internal/repair1503"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

func TestRepairM_1503_10_SuccessorSpendSurvivesReopen(t *testing.T) {
	TestPrecompileGrantSessionSuccessor(t)
}

func TestRepairM_1503_12_LifecycleReceiptLogs(t *testing.T) {
	TestPrecompileRecordLifecycleRegistersDid(t)
}

func TestRepairM_1503_11_ChargedSuccessorParity(t *testing.T) {
	repairM1503SuccessorParity(t)
}

func repairM1503OwnerAuthority(t *testing.T, f *grantFixture) lxphost.KernelAuthority {
	t.Helper()
	authority := f.actor.Authority
	authority.Kind = lxphost.AuthorityKindOwner
	var err error
	authority.AuthorityHash, err = lxphost.AuthorityHash(authority.Kind, authority.GrantID, authority.VerifiedKey)
	require.NoError(t, err)
	return authority
}

func repairM1503Execute(t *testing.T, f *grantFixture, ctx sdk.Context, activityType uint32, payload []byte, limit uint64, grantID [32]byte) (lxphost.Receipt, error) {
	t.Helper()
	authority := repairM1503OwnerAuthority(t, f)
	if grantID != [32]byte{} {
		payloadHash, err := codec.DomainHash(codec.DomainPayloadHash, payload)
		require.NoError(t, err)
		resolved, err := f.host.ReadCheckActivity(ctx, f.actor.DID, grantID, payloadHash)
		require.NoError(t, err)
		authority = lxphost.KernelAuthority{Actor: resolved.Actor, Principal: resolved.Principal,
			VerifiedKey: resolved.VerifiedKey, Bound: true, Kind: resolved.Kind,
			GrantID: resolved.GrantID, AuthorityHash: resolved.AuthorityHash, Scope: &resolved.Scope}
	}
	activity := repairM1503Activity(t, f, ctx, activityType, payload, limit, authority.VerifiedKey)
	sequence, err := lxphost.NextSequence(ctx, f.hostKey)
	require.NoError(t, err)
	return f.host.Execute(lxphost.WithSequence(ctx, sequence), activity, authority)
}

func repairM1503Activity(t *testing.T, f *grantFixture, ctx sdk.Context, activityType uint32, payload []byte, limit uint64, key [32]byte) lxphost.Activity {
	t.Helper()
	sequence, err := f.host.ReadIdentitySequence(ctx, f.actor.DID)
	require.NoError(t, err)
	material := binary.BigEndian.AppendUint32(nil, activityType)
	material = binary.BigEndian.AppendUint64(material, sequence)
	material = append(material, payload...)
	activity := lxphost.Activity{ProtocolVersion: 3, Type: activityType, ActorDID: f.actor.DID,
		AuthorityKey: key, AccountSequence: sequence, NotBefore: uint64(ctx.BlockTime().UnixMilli()),
		NotAfter: uint64(ctx.BlockTime().UnixMilli()), IdempotencyKey: sha256.Sum256(material), Payload: payload}
	binary.BigEndian.PutUint64(activity.FeeLimit[8:], limit)
	return activity
}

func repairM1503FeesAndProgram(t *testing.T, f *grantFixture) ([]byte, uint64) {
	t.Helper()
	// A primary key can also hold a session grant. This keeps the real host's
	// main-account signer binding intact during the metered predecessor call.
	f.key = f.actor.DIDKey
	salt := bytes.Repeat([]byte{0x55}, 32)
	material := append([]byte("LX:ASSET:v1"), f.actor.Authority.Actor[:]...)
	f.asset = sha256.Sum256(append(material, salt...))
	rows, err := repair1503.GovernedRecords(f.asset, f.timestamp)
	require.NoError(t, err)
	for _, row := range rows {
		f.ctx.KVStore(f.hostKey).Set(row.Key, row.Value)
	}
	schedule, err := f.host.ReadFeeSchedule(f.ctx)
	require.NoError(t, err)
	require.Len(t, schedule, 217)
	require.Equal(t, f.asset[:], schedule[65:97])
	history, err := f.host.ReadFeeScheduleAt(f.ctx, 1)
	require.NoError(t, err)
	require.Equal(t, schedule, history)
	execute := func(kind uint32, payload []byte) {
		t.Helper()
		receipt, err := repairM1503Execute(t, f, f.ctx, kind, payload, 0, [32]byte{})
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
	}
	registration := append([]byte{0, 1}, f.asset[:]...)
	registration = append(registration, salt...)
	registration = append(registration, 1, 'T', 1, 'T', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 1_000_000_000)
	execute(0x00010001, append(registration, 1, 0))
	execute(0x00010004, append([]byte{0, 1}, f.asset[:]...))
	_, payer, err := lxphost.AssetAccountID(f.actor.DIDKey, f.asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, f.asset[:]...)
	mint = append(mint, payer[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 500_000_000)
	execute(0x0001000a, mint)
	balance, err := f.host.ReadBalance(f.ctx, payer, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(500_000_000), balance)

	// Account peers are committed fixtures; grants, fees and spend are produced
	// by their production implementations, never by writing charge records.
	account := func(id [32]byte, name string, kind byte, amount uint64, asset, authority [32]byte, hasAuthority byte) {
		field := func(out, value []byte) []byte {
			return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
		}
		row := field(nil, id[:])
		row = field(row, []byte(name))
		row = append(row, kind)
		row = binary.BigEndian.AppendUint64(row, 0)
		row = binary.BigEndian.AppendUint64(row, amount)
		row = field(row, asset[:])
		row = append(row, 1)
		row = binary.BigEndian.AppendUint64(row, 0)
		row = binary.BigEndian.AppendUint64(row, 1)
		row = append(row, 0, 0)
		row = field(row, authority[:])
		f.ctx.KVStore(f.hostKey).Set(append([]byte{6}, id[:]...), append(row, hasAuthority))
	}
	mainName, mainID, err := lxphost.MainAccountID(f.actor.DIDKey)
	require.NoError(t, err)
	account(mainID, mainName, 1, 100_000_000, [32]byte{0x91}, f.actor.DIDKey, 1)
	treasury, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	account(treasury, "system:fees", 10, 0, f.asset, [32]byte{}, 0)

	section := func(id byte, body []byte) []byte { return append([]byte{id, byte(len(body))}, body...) }
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
	body := []byte{0, 0x41, 0, 0x41, 1, 0x41, 1, 0x41, 1, 0x10, 0, 0x1a, 0x41, 0, 0x0b}
	code := []byte{2, 4, 0, 0x41, 16, 0x0b, byte(len(body))}
	wasm = append(wasm, section(10, append(code, body...))...)
	wasm = append(wasm, section(11, []byte{1, 0, 0x41, 0, 0x0b, 2, 't', 'd'})...)
	program := [32]byte{0x71}
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = append(iface, 4, 0, 0, 0, 0)
	deploy, err := lp.EncodeDeployPayload(program, 1, 1, f.actor.Authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	execute(0x00090001, deploy)
	payload, err := lp.EncodeCallPayload(program, 1, "layerx_call", []byte{0, 0xff}, []byte{0, 1, 3},
		[]byte("LayerX/programs/access-declaration/v1\x00\x00"), 64,
		[7]uint64{1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096})
	require.NoError(t, err)
	activity := repairM1503Activity(t, f, f.ctx, 0x00090003, payload, 100_000_000, f.actor.DIDKey)
	sequence, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	before := hostSnapshot(t, f.ctx, f.hostKey)
	forecast, err := f.host.Simulate(lxphost.WithSequence(f.ctx, sequence), activity, repairM1503OwnerAuthority(t, f))
	require.NoError(t, err)
	require.Zero(t, forecast.ResultCode)
	require.NotNil(t, forecast.ProgramOutcome)
	require.Zero(t, forecast.ProgramOutcome.Status)
	require.True(t, forecast.FeeCharged.IsPositive())
	require.Equal(t, before, hostSnapshot(t, f.ctx, f.hostKey))
	decoded, err := codec.DecodeReceipt(forecast.Encoded)
	require.NoError(t, err)
	require.NotNil(t, decoded.ProgramOutcome)
	usage := decoded.ProgramOutcome
	budget := [7]uint64{usage.CPUFuel, usage.MemoryBytes, usage.StorageReadBytes,
		usage.StorageWriteBytes, uint64(usage.OutputValues), usage.OutputBytes, 0}
	var ceiling uint64
	for i, units := range budget[:6] {
		price := binary.BigEndian.Uint64(schedule[9+8*i : 17+8*i])
		ceiling += units * price
	}
	require.Equal(t, forecast.FeeCharged, sdk.NewIntFromUint64(ceiling))
	// Sign exactly the measured budget so the exhausted grant can admit this
	// call without reserving unused resource headroom against its fee limits.
	payload, err = lp.EncodeCallPayload(program, 1, "layerx_call", []byte{0, 0xff}, []byte{0, 1, 3},
		[]byte("LayerX/programs/access-declaration/v1\x00\x00"), 64, budget)
	require.NoError(t, err)
	return payload, forecast.FeeCharged.Uint64()
}

func repairM1503Body(f *grantFixture, key [32]byte, revocation, fee uint64) []byte {
	body := f.feeSessionBody(key, revocation)
	binary.BigEndian.PutUint16(body[122:124], 3)
	binary.BigEndian.PutUint16(body[124:126], 3)
	binary.BigEndian.PutUint64(body[302:310], f.timestamp+10_000)
	binary.BigEndian.PutUint64(body[439:447], fee)
	binary.BigEndian.PutUint64(body[455:463], 2*fee)
	binary.BigEndian.PutUint64(body[495:503], fee)
	return body
}

func repairM1503Commitment(grant lxphost.AuthorityGrant) [32]byte {
	preimage := append([]byte("LXP/session-fee-replacement/v1\x00"), grant.GrantID[:]...)
	preimage = binary.BigEndian.AppendUint64(preimage, grant.RevokedAtSequence)
	preimage = append(preimage, grant.GrantID[:]...)
	for _, amount := range []sdk.Int{grant.FeeBudget.SpentTotal, grant.FeeBudget.SpentThisPeriod} {
		var encoded [16]byte
		amount.BigInt().FillBytes(encoded[:])
		preimage = append(preimage, encoded[:]...)
	}
	return sha256.Sum256(binary.BigEndian.AppendUint64(preimage, grant.FeeBudget.PeriodStart))
}

func repairM1503Charge(t *testing.T, f *grantFixture, ctx sdk.Context, id [32]byte, payload []byte, fee uint64) lxphost.AuthorityGrant {
	t.Helper()
	_, payer, err := lxphost.AssetAccountID(f.actor.DIDKey, f.asset)
	require.NoError(t, err)
	balance, err := f.host.ReadBalance(ctx, payer, f.asset)
	require.NoError(t, err)
	before, err := f.host.ReadGrant(ctx, id)
	require.NoError(t, err)
	receipt, err := repairM1503Execute(t, f, ctx, 0x00090003, payload, fee, id)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.NotNil(t, receipt.ProgramOutcome)
	require.Zero(t, receipt.ProgramOutcome.Status)
	require.Equal(t, sdk.NewIntFromUint64(fee), receipt.FeeCharged)
	require.NotEmpty(t, receipt.Encoded)
	after, err := f.host.ReadGrant(ctx, id)
	require.NoError(t, err)
	require.Equal(t, before.FeeBudget.SpentTotal.Add(receipt.FeeCharged), after.FeeBudget.SpentTotal)
	require.Equal(t, receipt.FeeCharged, after.FeeBudget.SpentThisPeriod)
	remaining, err := f.host.ReadBalance(ctx, payer, f.asset)
	require.NoError(t, err)
	require.Equal(t, receipt.FeeCharged, balance.Sub(remaining))
	return after
}

func repairM1503Reopen(t *testing.T, f *grantFixture, ctx sdk.Context) {
	t.Helper()
	before := hostSnapshot(t, ctx, f.hostKey)
	directory := t.TempDir()
	keys := ctx.MultiStore().StoreKeys()
	open := func() sdk.CommitMultiStore {
		db, err := tmdb.NewDB("auth-spend", tmdb.GoLevelDBBackend, directory)
		require.NoError(t, err)
		cms := store.NewCommitMultiStore(db)
		for _, key := range keys {
			switch key.(type) {
			case *sdk.KVStoreKey:
				cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
			case *sdk.TransientStoreKey:
				cms.MountStoreWithDB(key, sdk.StoreTypeTransient, nil)
			case *sdk.MemoryStoreKey:
				cms.MountStoreWithDB(key, sdk.StoreTypeMemory, nil)
			}
		}
		require.NoError(t, cms.LoadLatestVersion())
		return cms
	}
	cms := open()
	for _, key := range keys {
		if _, permanent := key.(*sdk.KVStoreKey); !permanent {
			continue
		}
		source, ok := mounted(ctx.MultiStore(), key)
		if !ok {
			continue
		}
		iterator := source.Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			cms.GetKVStore(key).Set(bytes.Clone(iterator.Key()), bytes.Clone(iterator.Value()))
		}
		require.NoError(t, iterator.Close())
	}
	commit := cms.Commit(true)
	require.Positive(t, commit.Version)
	f.host.Close()
	require.NoError(t, cms.Close())
	cms = open()
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	require.Equal(t, commit, cms.LastCommitID())
	f.ctx = ctx.WithMultiStore(cms.CacheMultiStore()).WithEventManager(sdk.NewEventManager())
	var err error
	f.host, err = lxphost.NewHost(lxphost.Config{StoreKey: f.hostKey, NetworkID: 7,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1_000_000})
	require.NoError(t, err)
	t.Cleanup(f.host.Close)
	var authKey sdk.StoreKey
	for _, key := range keys {
		if key.Name() == types.StoreKey {
			authKey = key
		}
	}
	require.NotNil(t, authKey)
	app := testkeeper.EVMTestApp
	f.keeper = authkeeper.NewKeeper(app.AppCodec(), authKey, f.host, &app.AuthzKeeper, &app.EvmKeeper, "grant-test-authority")
	f.keepers.auth = f.keeper
	f.precompile, err = layerxauth.NewPrecompile(f.keepers)
	require.NoError(t, err)
	actor, err := f.keeper.Actor(f.ctx, f.caller)
	require.NoError(t, err)
	require.Equal(t, f.actor, actor)
	require.Equal(t, before, hostSnapshot(t, f.ctx, f.hostKey))
}

func repairM1503AssertExhausted(t *testing.T, f *grantFixture, id [32]byte, fee uint64, total bool) {
	t.Helper()
	grant, err := f.host.ReadGrant(f.ctx, id)
	require.NoError(t, err)
	require.True(t, grant.FeeBudget.Present)
	require.Equal(t, sdk.NewIntFromUint64(fee), grant.FeeBudget.SpentThisPeriod)
	require.Equal(t, uint64(f.ctx.BlockTime().UnixMilli()), grant.FeeBudget.PeriodStart)
	before := hostSnapshot(t, f.ctx, f.hostKey)
	require.ErrorIs(t, repair1503.FeeAdmission(before, id, grant.FeeBudget.PeriodStart, fee), lxphost.Code(-211))
	nextPeriod := grant.FeeBudget.PeriodStart + grant.FeeBudget.PeriodLength
	if total {
		require.Equal(t, grant.FeeBudget.MaximumTotal, grant.FeeBudget.SpentTotal)
		require.ErrorIs(t, repair1503.FeeAdmission(before, id, nextPeriod, fee), lxphost.Code(-211))
	} else {
		require.Equal(t, sdk.NewIntFromUint64(fee), grant.FeeBudget.SpentTotal)
		require.NoError(t, repair1503.FeeAdmission(before, id, nextPeriod, fee))
	}
	require.Equal(t, before, hostSnapshot(t, f.ctx, f.hostKey))
	evm, _ := f.evm()
	require.Equal(t, []interface{}{true, ""}, grantCall(t, f, evm, "isLive", id, uint64(90), uint64(f.ctx.BlockTime().UnixMilli())))
	summary := grantCall(t, f, evm, "sessionKey", id)[0].([]byte)
	native, err := f.keeper.SessionKey(f.ctx, id)
	require.NoError(t, err)
	require.Equal(t, native, summary)
}
