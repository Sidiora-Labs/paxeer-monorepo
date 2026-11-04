//go:build cgo

package lxphost

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestReadProgramRecordFields(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	ctx := chain.context(1, 10000, 41)
	actor := newTestActor(t, 61)
	seedProgramsFeeRecord(t, chain, actor.id)
	id := repeated(0xa1)
	before := chain.dump()
	_, err := host.ReadProgram(ctx, id)
	require.Equal(t, Code(-7), err)
	_, err = host.ReadProgramInterface(ctx, id)
	require.Equal(t, Code(-7), err)
	_, err = host.ReadProgramStateHead(ctx, id)
	require.Equal(t, Code(-7), err)
	require.Equal(t, before, chain.dump())

	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, actor.id[:]...)
	store := ctx.KVStore(chain.key)
	store.Set(moduleKVKey(9, []byte("progmet/active/v1")), metering)
	store.Set(moduleKVKey(9, binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)

	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	codeHash := sha256.Sum256(wasm)
	encoding := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	encoding = binary.BigEndian.AppendUint16(encoding, 1)
	encoding = binary.BigEndian.AppendUint16(encoding, 1)
	encoding = binary.BigEndian.AppendUint16(encoding, 11)
	encoding = append(encoding, []byte("layerx_call")...)
	encoding = append(encoding, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)
	payload := append(id[:0:0], id[:]...)
	payload = append(payload, 0, 1, 1, 0)
	payload = append(payload, actor.id[:]...)
	payload = append(payload, codeHash[:]...)
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(wasm)))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(encoding)))
	payload = append(payload, encoding...)
	payload = append(payload, wasm...)
	registered := execute(t, host, chain.context(1, 10000, 40), actor.register(t, host, 0, 10000), actor.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	receipt := execute(t, host, ctx, actor.activity(t, host, 0x00090001, 1, 10000, payload), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	chain.commit()
	before = chain.dump()
	expected := ProgramRecord{Policy: 1, Authority: actor.id, CodeHash: codeHash, ABIVersion: 1, Version: 1, Active: true}
	record, err := host.ReadProgram(ctx, id)
	require.NoError(t, err)
	require.Equal(t, expected, record)
	iface, err := host.ReadProgramInterface(ctx, id)
	require.NoError(t, err)
	expectedInterface := ProgramInterface{Version: 1, Digest: sha256.Sum256(encoding), Encoding: encoding}
	require.Equal(t, expectedInterface, iface)
	require.Equal(t, before, chain.dump())

	head := binary.BigEndian.AppendUint16(nil, 1)
	head = binary.BigEndian.AppendUint32(head, 0)
	manifestDigest := sha256.Sum256([]byte{0, 1, 0, 0, 0, 0})
	head = append(head, manifestDigest[:]...)
	ctx.KVStore(chain.key).Set(moduleKVKey(9, append(programKey("progstor", id), 1)), head)
	before = chain.dump()
	fresh := chain.host(t)
	actualHead, err := fresh.ReadProgramStateHead(ctx, id)
	require.NoError(t, err)
	require.Equal(t, head, actualHead)
	actualHead[0] ^= 0xff
	actualHead, err = host.ReadProgramStateHead(ctx, id)
	require.NoError(t, err)
	require.Equal(t, head, actualHead)
	iface.Encoding[0] ^= 0xff
	iface, err = fresh.ReadProgramInterface(ctx, id)
	require.NoError(t, err)
	require.Equal(t, expectedInterface, iface)
	record, err = fresh.ReadProgram(ctx, id)
	require.NoError(t, err)
	require.Equal(t, expected, record)
	require.Equal(t, before, chain.dump())

	branch, _ := ctx.CacheContext()
	branch.KVStore(chain.key).Set(moduleKVKey(9, programKey("program\x00", id)), []byte{1})
	_, err = fresh.ReadProgram(branch, id)
	require.Equal(t, codeFatalInvariant, err)
	record, err = fresh.ReadProgram(ctx, id)
	require.NoError(t, err)
	require.Equal(t, expected, record)
	require.Equal(t, before, chain.dump())
	host.Close()
	_, err = host.ReadProgram(ctx, id)
	require.Equal(t, codeNonCanonical, err)
}

// programReadAccountFixture is an opaque-copy/corruption fixture, not producer proof.
func programReadAccountFixture(t *testing.T, chain *testChain, ctx sdk.Context) ProgramAccount {
	t.Helper()
	program, asset, principal := repeated(0x71), repeated(0xaa), repeated(0x31)
	seed := []byte("vault\x00reserve")
	material := append([]byte("LayerX/programs/program-account/v1\x00"), program[:]...)
	material = binary.BigEndian.AppendUint32(material, uint32(len(seed)))
	accountID := sha256.Sum256(append(material, seed...))
	seedDigest := sha256.Sum256(seed)
	event := append([]byte("LXPA1"), program[:]...)
	event = append(event, accountID[:]...)
	event = append(event, asset[:]...)
	event = binary.BigEndian.AppendUint16(event, uint16(len(seed)))
	event = append(event, seedDigest[:]...)
	event = binary.BigEndian.AppendUint64(event, 42)
	digest := sha256.Sum256(event)
	binding := append([]byte{2}, program[:]...)
	binding = append(binding, accountID[:]...)
	binding = append(binding, asset[:]...)
	binding = binary.BigEndian.AppendUint16(binding, uint16(len(seed)))
	binding = binary.BigEndian.AppendUint64(binding, 42)
	binding = append(binding, digest[:]...)
	binding = append(binding, seed...)
	store := ctx.KVStore(chain.key)
	primary := append(programKey("program-account\x00p", program), seedDigest[:]...)
	store.Set(moduleKVKey(9, primary), binding)
	store.Set(moduleKVKey(9, programKey("program-account\x00r", accountID)), binding)
	store.Set(moduleKVKey(9, programKey("program-owner\x00", program)), append([]byte{1}, principal[:]...))
	store.Set(moduleKVKey(9, programKey("program-account-profile\x00", program)), append([]byte{2}, principal[:]...))

	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	name := []byte("module:programs:value:" + hex.EncodeToString(accountID[:]))
	account := field(nil, accountID[:])
	account = field(account, name)
	account = append(account, 13)
	account = binary.BigEndian.AppendUint64(account, 3)
	account = binary.BigEndian.AppendUint64(account, 250)
	account = field(account, asset[:])
	account = append(account, 1)
	account = binary.BigEndian.AppendUint64(account, 7)
	account = binary.BigEndian.AppendUint64(account, 42)
	account = append(account, 1, 0)
	account = field(account, make([]byte, 32))
	account = append(account, 0)
	store.Set(accountKey(accountID), account)
	return ProgramAccount{RecordVersion: 2, ProgramID: program, AccountID: accountID, AssetID: asset,
		Seed: seed, RegisteredSequence: 42, RegistrationEventDigest: digest}
}

func TestReadProgramAccountsOpaqueCopy(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	ctx := chain.context(1, 10000, 42)
	program, asset := repeated(0x71), repeated(0xaa)
	_, err := host.ReadProgramAccount(ctx, program, asset, []byte("vault"))
	require.Equal(t, Code(-7), err)
	_, err = host.ReadProgramAccountByID(ctx, repeated(1))
	require.Equal(t, Code(-7), err)
	_, err = host.ReadAccountProfile(ctx, program)
	require.Equal(t, Code(-7), err)
	value, err := host.ReadValueAccount(ctx, program, repeated(1))
	require.Equal(t, Code(-7), err)
	require.Equal(t, sdk.ZeroInt(), value.Balance)
	expected := programReadAccountFixture(t, chain, ctx)
	before := chain.dump()
	for _, reader := range []*Host{host, chain.host(t)} {
		bySeed, err := reader.ReadProgramAccount(ctx, program, asset, expected.Seed)
		require.NoError(t, err)
		require.Equal(t, expected, bySeed)
		byID, err := reader.ReadProgramAccountByID(ctx, expected.AccountID)
		require.NoError(t, err)
		require.Equal(t, expected, byID)
		byID.Seed[0] ^= 0xff
		again, err := reader.ReadProgramAccountByID(ctx, expected.AccountID)
		require.NoError(t, err)
		require.Equal(t, expected, again)
		profile, err := reader.ReadAccountProfile(ctx, program)
		require.NoError(t, err)
		require.Equal(t, AccountProfile{Profile: 2, Principal: repeated(0x31)}, profile)
		value, err = reader.ReadValueAccount(ctx, program, expected.AccountID)
		require.NoError(t, err)
		amount, ok := sdk.NewIntFromString("55340232221128655098")
		require.True(t, ok)
		require.Equal(t, ValueAccount{Balance: amount, Frozen: true}, value)
	}
	_, err = host.ReadProgramAccount(ctx, program, repeated(2), expected.Seed)
	require.Equal(t, Code(-402), err)
	_, err = host.ReadProgramAccount(ctx, program, asset, make([]byte, 129))
	require.Equal(t, Code(-5), err)
	_, err = host.ReadValueAccount(ctx, repeated(2), expected.AccountID)
	require.Equal(t, Code(-213), err)
	require.Equal(t, before, chain.dump())
	host.Close()
	_, err = host.ReadAccountProfile(ctx, program)
	require.Equal(t, codeNonCanonical, err)
}

func TestReadWindDownAndRetirementOpaqueCopy(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	ctx := chain.context(1, 10000, 42)
	program, lease := repeated(0x71), repeated(0x51)
	before := chain.dump()
	_, err := host.ReadWindDown(ctx, program)
	require.Equal(t, Code(-7), err)
	_, err = host.ReadExitRoute(ctx, program, repeated(1))
	require.Equal(t, Code(-7), err)
	for kind := uint8(0); kind < 3; kind++ {
		absent, err := host.ReadSandbox(ctx, lease, kind)
		require.Equal(t, Code(-7), err)
		require.Nil(t, absent)
	}
	absent, err := host.ReadRetirementControl(ctx)
	require.Equal(t, Code(-7), err)
	require.Nil(t, absent)
	require.Equal(t, before, chain.dump())

	binding := programReadAccountFixture(t, chain, ctx)
	store := ctx.KVStore(chain.key)
	wind := append([]byte{1, 2}, program[:]...)
	wind = binary.BigEndian.AppendUint64(wind, 20000)
	wind = binary.BigEndian.AppendUint64(wind, 43)
	wind = binary.BigEndian.AppendUint16(wind, 1)
	wind = binary.BigEndian.AppendUint16(wind, 1)
	store.Set(moduleKVKey(9, programKey("wind-down\x00s", program)), wind)
	destination := repeated(0x91)
	route := append([]byte{1}, binding.AssetID[:]...)
	route = append(route, destination[:]...)
	route = binary.BigEndian.AppendUint16(route, uint16(len(binding.Seed)))
	route = append(route, binding.Seed...)
	routeKey := append(programKey("wind-down\x00r", program), binding.AccountID[:]...)
	store.Set(moduleKVKey(9, routeKey), route)
	sandbox := [][]byte{{0, 1, 0, 2, 3}, {0, 2, 0xff}, {0, 3, 0, 0, 5}}
	for kind, record := range sandbox {
		store.Set(moduleKVKey(9, append([]byte{'s', byte(kind)}, lease[:]...)), record)
	}
	control := make([]byte, 251)
	copy(control, "LXRT1")
	binary.BigEndian.PutUint16(control[5:7], 1)
	binary.BigEndian.PutUint32(control[7:11], testNetworkID)
	copy(control[11:43], repeatedBytes(0x31))
	store.Set(moduleKVKey(9, []byte("progretire/v1")), control)
	chain.commit()
	before = chain.dump()
	for _, reader := range []*Host{host, chain.host(t)} {
		actualWind, err := reader.ReadWindDown(ctx, program)
		require.NoError(t, err)
		require.Equal(t, WindDown{ProgramID: program, Status: 2, ExitProgram: program, Deadline: 20000,
			EffectiveSequence: 43, ValueAccountCount: 1, LiveValueAccountCount: 1}, actualWind)
		actualRoute, err := reader.ReadExitRoute(ctx, program, binding.AccountID)
		require.NoError(t, err)
		expectedRoute := ExitRoute{ProgramID: program, AccountID: binding.AccountID, AssetID: binding.AssetID,
			Destination: destination, Seed: binding.Seed}
		require.Equal(t, expectedRoute, actualRoute)
		actualRoute.Seed[0] ^= 0xff
		actualRoute, err = reader.ReadExitRoute(ctx, program, binding.AccountID)
		require.NoError(t, err)
		require.Equal(t, expectedRoute, actualRoute)
		for kind, record := range sandbox {
			actual, err := reader.ReadSandbox(ctx, lease, uint8(kind))
			require.NoError(t, err)
			require.Equal(t, record, actual)
			actual[0] ^= 0xff
			actual, err = reader.ReadSandbox(ctx, lease, uint8(kind))
			require.NoError(t, err)
			require.Equal(t, record, actual)
		}
		actual, err := reader.ReadRetirementControl(ctx)
		require.NoError(t, err)
		require.Equal(t, control, actual)
		actual[0] ^= 0xff
		actual, err = reader.ReadRetirementControl(ctx)
		require.NoError(t, err)
		require.Equal(t, control, actual)
	}
	_, err = host.ReadSandbox(ctx, lease, 3)
	require.Equal(t, codeNonCanonical, err)
	require.Equal(t, before, chain.dump())
	host.Close()
	_, err = host.ReadRetirementControl(ctx)
	require.Equal(t, codeNonCanonical, err)
}

func repeatedBytes(value byte) []byte {
	id := repeated(value)
	return id[:]
}

func TestReadProgramLifecycleProducers(t *testing.T) {
	testReadProgramLifecycleProducers(t)
}

func testReadProgramLifecycleProducers(t *testing.T) {
	t.Helper()
	f := repairM909Funded(t)
	ctx := f.chain.context(2, resolutionBlockTime, 8)
	called := execute(t, f.host, ctx, f.call, f.authority)
	require.Zero(t, called.ResultCode)
	head, err := f.host.ReadProgramStateHead(ctx, f.program)
	require.NoError(t, err)
	require.Len(t, head, 38)
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(head[:2]))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(head[2:6]))

	// Account activities require an ABI 2 program and bind its owner at deploy.
	program := repeated(0x78)
	wasm := resolutionGuest("layerx_v1", "event_emit", 4)
	hash := sha256.Sum256(wasm)
	deploy := append([]byte(nil), program[:]...)
	deploy = append(deploy, 0, 2, 1, 0)
	deploy = append(deploy, f.actor.id[:]...)
	deploy = append(deploy, hash[:]...)
	deploy = binary.BigEndian.AppendUint32(deploy, uint32(len(wasm)))
	deploy = append(deploy, wasm...)
	identitySequence, globalSequence := uint64(7), uint64(9)
	run := func(kind uint32, payload []byte) Receipt {
		t.Helper()
		activity := f.actor.activity(t, f.host, kind, identitySequence, resolutionBlockTime, payload)
		receipt := execute(t, f.host, f.chain.context(2, resolutionBlockTime, globalSequence), activity, f.authority)
		require.Zero(t, receipt.ResultCode, "activity %x: %s", kind, receipt.ResultCode.Error())
		identitySequence++
		globalSequence++
		return receipt
	}
	run(resolutionDeployType, deploy)
	lease := repeated(0x51)
	seed := append([]byte("sandbox-lease-escrow/v1\x00"), lease[:]...)
	accountPayload := append([]byte(nil), program[:]...)
	accountPayload = append(accountPayload, "LXPA2"...)
	accountPayload = append(accountPayload, f.asset.id[:]...)
	accountPayload = binary.BigEndian.AppendUint32(accountPayload, uint32(len(seed)))
	accountPayload = append(accountPayload, seed...)
	registered := run(0x00090006, accountPayload)
	ctx = f.chain.context(2, resolutionBlockTime, globalSequence)
	binding, err := f.host.ReadProgramAccount(ctx, program, f.asset.id, seed)
	require.NoError(t, err)
	material := append([]byte("LayerX/programs/program-account/v1\x00"), program[:]...)
	material = binary.BigEndian.AppendUint32(material, uint32(len(seed)))
	require.Equal(t, sha256.Sum256(append(material, seed...)), binding.AccountID)
	require.Equal(t, registered.GlobalSequence, binding.RegisteredSequence)
	require.Equal(t, uint8(2), binding.RecordVersion)
	var accountEvent []byte
	for _, event := range registered.Events {
		if len(event.Body) > 5 && string(event.Body[:5]) == "LXPA1" {
			accountEvent = event.Body
		}
	}
	require.NotEmpty(t, accountEvent)
	require.Equal(t, sha256.Sum256(accountEvent), binding.RegistrationEventDigest)

	var amount [16]byte
	binary.BigEndian.PutUint64(amount[8:], 5)
	requestEvidence, err := codec.DecodeReceipt(registered.Encoded)
	require.NoError(t, err)
	fund, err := EncodeSandboxFunding(SandboxFunding{
		LeaseID: lease, Tenant: f.actor.id, ProgramID: program, ImageCodeHash: hash,
		Asset: f.asset.id, Source: f.asset.account, Amount: amount,
		Limits: [8]uint64{1, 1, 1, 1, 1, 1, 1, 1}, OpenedAt: 2, Expiry: 200,
		FeeScheduleVersion: 1, Prices: [7]uint64{1, 1, 1, 1, 1, 1, 1},
		RequestActivityID: requestEvidence.ActivityID, RequestReceiptDigest: requestEvidence.Digest(),
	})
	require.NoError(t, err)
	requestLength := int(binary.BigEndian.Uint32(fund[248:252]))
	stateOffset := requestLength - 112
	require.Equal(t, binding.AccountID[:], fund[100:132])
	run(0x00090009, fund)
	ctx = f.chain.context(2, resolutionBlockTime, globalSequence)
	sandbox := make([][]byte, 3)
	for kind, domain := range []string{"LayerX/programs/sandbox/lease-state/v3\x00", "LayerX/programs/sandbox/escrow-state/v1\x00", "LayerX/programs/sandbox/usage-ledger/v2\x00"} {
		sandbox[kind], err = f.host.ReadSandbox(ctx, lease, uint8(kind))
		require.NoError(t, err)
		require.Greater(t, len(sandbox[kind]), len(domain))
		require.Equal(t, domain, string(sandbox[kind][:len(domain)]))
	}
	require.Equal(t, byte(1), sandbox[0][stateOffset], "FUND must produce Funded state")
	require.Equal(t, byte(2), sandbox[0][stateOffset+1], "FUND must append its native transition to the request acknowledgement")
	require.Equal(t, fund[252:252+stateOffset], sandbox[0][:stateOffset], "native FUND must preserve the encoded lease terms")
	value, err := f.host.ReadValueAccount(ctx, program, binding.AccountID)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(5), value.Balance)

	route := append([]byte(nil), program[:]...)
	route = append(route, 1)
	route = append(route, binding.AccountID[:]...)
	route = append(route, f.asset.id[:]...)
	route = append(route, f.payee[:]...)
	route = binary.BigEndian.AppendUint16(route, uint16(len(seed)))
	route = append(route, seed...)
	run(0x00090007, route)
	deprecate := append([]byte(nil), program[:]...)
	deprecate = append(deprecate, 2)
	deprecate = append(deprecate, program[:]...)
	deprecate = binary.BigEndian.AppendUint64(deprecate, uint64(resolutionBlockTime+1000))
	deprecated := run(0x00090007, deprecate)
	control, err := f.host.ReadRetirementControl(ctx)
	require.NoError(t, err)
	require.Len(t, control, 251)
	require.Equal(t, "LXRT1", string(control[:5]))
	require.Equal(t, uint32(testNetworkID), binary.BigEndian.Uint32(control[7:11]))
	require.Equal(t, f.actor.id[:], control[11:43])
	profileHash := sha256.Sum256(append([]byte("LXP/programs-retirement-profile/v1\x00"), control[:147]...))
	require.Equal(t, profileHash[:], control[147:179])
	f.reopen(t)
	ctx = f.chain.context(3, resolutionBlockTime, globalSequence)
	before := f.chain.dump()
	bySeed, err := f.host.ReadProgramAccount(ctx, program, f.asset.id, seed)
	require.NoError(t, err)
	require.Equal(t, binding, bySeed)
	byID, err := f.host.ReadProgramAccountByID(ctx, binding.AccountID)
	require.NoError(t, err)
	require.Equal(t, binding, byID)
	profile, err := f.host.ReadAccountProfile(ctx, program)
	require.NoError(t, err)
	require.Equal(t, AccountProfile{Profile: 2, Principal: f.actor.id}, profile)
	value, err = f.host.ReadValueAccount(ctx, program, binding.AccountID)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(5), value.Balance)
	wind, err := f.host.ReadWindDown(ctx, program)
	require.NoError(t, err)
	require.Equal(t, WindDown{ProgramID: program, Status: 2, ExitProgram: program, Deadline: uint64(resolutionBlockTime + 1000), EffectiveSequence: deprecated.GlobalSequence, ValueAccountCount: 1, LiveValueAccountCount: 1}, wind)
	actualRoute, err := f.host.ReadExitRoute(ctx, program, binding.AccountID)
	require.NoError(t, err)
	require.Equal(t, ExitRoute{ProgramID: program, AccountID: binding.AccountID, AssetID: f.asset.id, Destination: f.payee, Seed: seed}, actualRoute)
	for kind, expected := range sandbox {
		actual, err := f.host.ReadSandbox(ctx, lease, uint8(kind))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		actual[0] ^= 0xff
		again, err := f.host.ReadSandbox(ctx, lease, uint8(kind))
		require.NoError(t, err)
		require.Equal(t, expected, again)
	}
	actualHead, err := f.host.ReadProgramStateHead(ctx, f.program)
	require.NoError(t, err)
	require.Equal(t, head, actualHead)
	actualControl, err := f.host.ReadRetirementControl(ctx)
	require.NoError(t, err)
	require.Equal(t, control, actualControl)
	require.Equal(t, before, f.chain.dump())
}
