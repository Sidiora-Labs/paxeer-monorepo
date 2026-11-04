//go:build cgo

package lxphost

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost/internal/m909fixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM909Open(t *testing.T, directory string) (*testChain, *Host, func()) {
	t.Helper()
	db, err := dbm.NewDB("m909", dbm.GoLevelDBBackend, directory)
	require.NoError(t, err)
	key := sdk.NewKVStoreKey("lxphost")
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	chain := &testChain{key: key, cms: cms}
	host := chain.host(t)
	closed := false
	closeStore := func() {
		if !closed {
			host.Close()
			require.NoError(t, cms.Close())
			closed = true
		}
	}
	t.Cleanup(closeStore)
	return chain, host, closeStore
}

func repairM909Records(t *testing.T, chain *testChain, asset, controller, program [32]byte) map[string][]byte {
	t.Helper()
	first, second := newTestActor(t, 201), newTestActor(t, 202)
	records, err := m909fixture.Records(asset, controller, first.public, second.public, program)
	require.NoError(t, err)
	require.Contains(t, records, "progfee/active/v1")
	require.Contains(t, records, "progretire/v1")
	for key, value := range records {
		chain.context(1, resolutionBlockTime, 1).KVStore(chain.key).Set(moduleKVKey(9, []byte(key)), value)
	}
	return records
}

func repairM909Send(t *testing.T, host *Host, actor testActor, asset testAsset, to, reference [32]byte, identitySequence, accountSequence uint64) (Activity, KernelAuthority) {
	t.Helper()
	args := ledgertypes.SendArgs{From: asset.account, To: to, Asset: asset.id, Amount: sdk.NewInt(3), Sequence: accountSequence,
		IdempotencyKey: reference, ExpiresAt: uint64(resolutionBlockTime + 1000), PublicKey: actor.public, NetworkID: testNetworkID, ProtocolVersion: stateCommitment}
	payload, err := ledgertypes.EncodeSendPayload(args)
	require.NoError(t, err)
	args.ContextHash, err = codec.DomainHash(codec.DomainContextHash, append(append([]byte(nil), payload[4:116]...), reference[:]...))
	require.NoError(t, err)
	payload, err = ledgertypes.EncodeSendPayload(args)
	require.NoError(t, err)
	activity := actor.activity(t, host, 0x00010005, identitySequence, resolutionBlockTime, payload)
	activity.IdempotencyKey, activity.Signature = reference, nil
	authority := actor.authority()
	authority.Bound = true
	return activity, authority
}

type repairM909Fixture struct {
	chain     *testChain
	host      *Host
	close     func()
	directory string
	actor     testActor
	asset     testAsset
	payee     [32]byte
	treasury  [32]byte
	program   [32]byte
	paid      Receipt
	call      Activity
	authority KernelAuthority
}

func repairM909Funded(t *testing.T) *repairM909Fixture {
	t.Helper()
	f := &repairM909Fixture{directory: t.TempDir(), actor: newTestActor(t, 0x63), program: repeated(0x71)}
	f.chain, f.host, f.close = repairM909Open(t, f.directory)
	f.authority = f.actor.authority()
	f.authority.AuthorityHash = sha256.Sum256(f.actor.public[:])
	recipient := newTestActor(t, 0x64)
	f.asset = newTestAsset(t, f.actor, 0x55)
	repairM909Records(t, f.chain, f.asset.id, f.actor.id, f.program)
	for i, step := range []struct {
		actor    testActor
		activity Activity
	}{
		{f.actor, f.actor.register(t, f.host, 0, resolutionBlockTime)},
		{f.actor, f.actor.registerAsset(t, f.host, 1, resolutionBlockTime, f.asset, 0x55, 1000000000)},
		{f.actor, f.actor.openAssetAccount(t, f.host, 2, resolutionBlockTime, f.asset)},
		{f.actor, f.actor.mint(t, f.host, 3, resolutionBlockTime, f.asset, 500000000)},
		{recipient, recipient.openAssetAccount(t, f.host, 0, resolutionBlockTime, f.asset)},
	} {
		receipt := execute(t, f.host, f.chain.context(1, resolutionBlockTime, uint64(i+1)), step.activity, step.actor.authority())
		require.Zero(t, receipt.ResultCode)
	}
	_, f.payee, _ = AssetAccountID(recipient.public, f.asset.id)
	send, sendAuthority := repairM909Send(t, f.host, f.actor, f.asset, f.payee, repeated(0x91), 4, 0)
	f.paid = execute(t, f.host, f.chain.context(1, resolutionBlockTime, 6), send, sendAuthority)
	require.Zero(t, f.paid.ResultCode)
	require.Equal(t, f.paid.Encoded, f.chain.get(idempotencyKey(send.IdempotencyKey)))
	name, main, err := MainAccountID(f.actor.public)
	require.NoError(t, err)
	// Programs resolves one payment account per asset. The main account's
	// separate bootstrap asset must not duplicate the real funded asset account.
	// Its independent ledger sequence admits the lifecycle fixture's FUND at 9.
	payer, err := m909fixture.Account(name, [32]byte{0x91}, &f.actor.public, 100000000, 9)
	require.NoError(t, err)
	f.chain.context(1, resolutionBlockTime, 7).KVStore(f.chain.key).Set(accountKey(main), payer)
	f.treasury, err = kernelAccountID([]byte("system:fees"))
	require.NoError(t, err)
	treasury, err := m909fixture.Account("system:fees", f.asset.id, nil, 0, 0)
	require.NoError(t, err)
	f.chain.context(1, resolutionBlockTime, 7).KVStore(f.chain.key).Set(accountKey(f.treasury), treasury)
	wire, err := codec.DecodeReceipt(f.paid.Encoded)
	require.NoError(t, err)
	digest := wire.Digest()
	wasm := simulateFundedGuest(f.asset.id, f.payee, f.treasury, digest)
	hash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), hash[:]...)
	iface = append(iface, 0, 2, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64)
	// Scoped writes require both principal and shared storage in the static
	// interface mask; the invocation grants only the shared scope it uses.
	iface = append(iface, 0, 6, 1, 3, 4)
	caps := []byte{0, 5, 3}
	destinations := [][32]byte{f.payee, f.treasury}
	sort.Slice(destinations, func(i, j int) bool { return bytes.Compare(destinations[i][:], destinations[j][:]) < 0 })
	for _, to := range destinations {
		iface = append(iface, 6)
		iface = append(iface, f.asset.id[:]...)
		iface = append(iface, to[:]...)
		iface = append(iface, make([]byte, 15)...)
		iface = append(iface, 3)
		caps = append(caps, 5)
		caps = append(caps, f.asset.id[:]...)
		caps = append(caps, to[:]...)
		caps = append(caps, make([]byte, 15)...)
		caps = append(caps, 3)
	}
	iface = append(iface, 8)
	iface = append(iface, digest[:]...)
	iface = append(iface, 0, 0, 0, 0)
	caps = append(caps, 6)
	caps = append(caps, digest[:]...)
	caps = append(caps, 8)
	deploy := append([]byte(nil), f.program[:]...)
	deploy = append(deploy, 0, 2, 1, 0)
	deploy = append(deploy, f.actor.id[:]...)
	deploy = append(deploy, hash[:]...)
	deploy = binary.BigEndian.AppendUint32(deploy, uint32(len(wasm)))
	deploy = binary.BigEndian.AppendUint32(deploy, uint32(len(iface)))
	deploy = append(deploy, iface...)
	deploy = append(deploy, wasm...)
	receipt := execute(t, f.host, f.chain.context(1, resolutionBlockTime, 7), f.actor.activity(t, f.host, resolutionDeployType, 5, resolutionBlockTime, deploy), f.authority)
	require.Zero(t, receipt.ResultCode)
	access := []byte("LayerX/programs/access-declaration/v1\x00\x00")
	payload := append([]byte(nil), f.program[:]...)
	payload = append(payload, 0, 2, 0, 11)
	payload = binary.BigEndian.AppendUint32(payload, 0)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(caps)))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(access)))
	payload = binary.BigEndian.AppendUint32(payload, 64)
	for _, budget := range []uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096} {
		payload = binary.BigEndian.AppendUint64(payload, budget)
	}
	payload = append(payload, "layerx_call"...)
	payload = append(payload, caps...)
	payload = append(payload, access...)
	f.call = f.actor.activity(t, f.host, 0x00090003, 6, resolutionBlockTime, payload)
	binary.BigEndian.PutUint64(f.call.FeeLimit[8:], 100000000)
	signing, err := f.host.SigningHash(f.call)
	require.NoError(t, err)
	f.call.Signature = ed25519.Sign(f.actor.private, signing[:])
	f.call.ReceiptAuthorities = []ReceiptAuthority{{CanonicalReceipt: f.paid.Encoded, BlockHeight: 1}}
	return f
}

func (f *repairM909Fixture) reopen(t *testing.T) {
	t.Helper()
	f.chain.commit()
	committed := f.chain.cms.LastCommitID()
	rows := f.chain.dump()
	f.close()
	f.chain, f.host, f.close = repairM909Open(t, f.directory)
	require.Equal(t, committed, f.chain.cms.LastCommitID())
	require.Equal(t, rows, f.chain.dump())
}

func TestRepairM_909_SimulationSuccessProof(t *testing.T) {
	f := repairM909Funded(t)
	first := simulateFundedParity(t, f.host, f.chain, f.chain.context(2, resolutionBlockTime, 8), f.call, f.authority)
	f.reopen(t)
	again := simulateFundedParity(t, f.host, f.chain, f.chain.context(2, resolutionBlockTime, 8), f.call, f.authority)
	require.Equal(t, first, again)
}

func TestRepairM_909_SimulationTransferProjection(t *testing.T) {
	f := repairM909Funded(t)
	for _, reopen := range []bool{false, true} {
		if reopen {
			f.reopen(t)
		}
		receipt := simulateFundedParity(t, f.host, f.chain, f.chain.context(2, resolutionBlockTime, 8), f.call, f.authority)
		require.ElementsMatch(t, []TransferLeg{{Asset: f.asset.id, From: f.asset.account, To: f.payee, Amount: sdk.NewInt(1)}, {Asset: f.asset.id, From: f.asset.account, To: f.treasury, Amount: sdk.NewInt(2)}}, receipt.Transfers)
	}
}

func TestRepairM_909_SimulationReceiptAuthorities(t *testing.T) {
	f := repairM909Funded(t)
	f.reopen(t)
	persisted := f.chain.get(idempotencyKey(repeated(0x91)))
	require.Equal(t, f.paid.Encoded, persisted)
	f.call.ReceiptAuthorities[0].CanonicalReceipt = persisted
	receipt := simulateFundedParity(t, f.host, f.chain, f.chain.context(2, resolutionBlockTime, 8), f.call, f.authority)
	wire, err := codec.DecodeReceipt(persisted)
	require.NoError(t, err)
	digest := wire.Digest()
	require.NotEmpty(t, receipt.ProgramOutcome.Events)
	require.True(t, bytes.Contains(receipt.ProgramOutcome.Events, digest[:]), "guest must emit the actual receipt facts")
	facts := append([]byte(nil), digest[:]...)
	facts = binary.BigEndian.AppendUint32(facts, uint32(wire.ResultCode))
	facts = append(facts, f.asset.id[:]...)
	facts = append(facts, make([]byte, 15)...)
	facts = append(facts, 3)
	facts = append(facts, f.paid.ResultingStateRoot[:]...)
	expectedEvents := binary.BigEndian.AppendUint32([]byte("LayerX/programs/events/v1\x00"), 1)
	expectedEvents = append(expectedEvents, f.program[:]...)
	expectedEvents = append(expectedEvents, f.actor.id[:]...)
	expectedEvents = append(expectedEvents, make([]byte, 9)...)
	expectedEvents = binary.BigEndian.AppendUint32(expectedEvents, 32)
	expectedEvents = append(expectedEvents, facts[:32]...)
	expectedEvents = binary.BigEndian.AppendUint32(expectedEvents, 84)
	expectedEvents = append(expectedEvents, facts[32:]...)
	require.Equal(t, expectedEvents, receipt.ProgramOutcome.Events, "guest must project every verified payment fact in the canonical event envelope")
	for _, height := range []uint64{2, 3} {
		activity := f.call
		activity.ReceiptAuthorities = []ReceiptAuthority{{CanonicalReceipt: persisted, BlockHeight: height}}
		before := f.chain.dump()
		ctx := f.chain.context(2, resolutionBlockTime, 8)
		sibling, _ := ctx.CacheContext()
		_, executeErr := f.host.Execute(sibling, activity, f.authority)
		require.Equal(t, codeNonCanonical, executeErr)
		got, simulateErr := f.host.Simulate(ctx, activity, f.authority)
		require.Equal(t, executeErr, simulateErr)
		require.Equal(t, Receipt{}, got)
		require.Equal(t, before, f.chain.dump())
	}
	wrongCfg := f.host.cfg
	wrongCfg.NetworkID++
	wrong, err := NewHost(wrongCfg)
	require.NoError(t, err)
	defer wrong.Close()
	before := f.chain.dump()
	ctx := f.chain.context(2, resolutionBlockTime, 8)
	sibling, _ := ctx.CacheContext()
	_, executeErr := wrong.Execute(sibling, f.call, f.authority)
	require.Error(t, executeErr)
	got, simulateErr := wrong.Simulate(ctx, f.call, f.authority)
	require.Equal(t, executeErr, simulateErr)
	require.Equal(t, Receipt{}, got)
	require.Equal(t, before, f.chain.dump())
	absent := f.call
	absent.ReceiptAuthorities = nil
	sibling, _ = ctx.CacheContext()
	refused, err := f.host.Execute(sibling, absent, f.authority)
	require.NoError(t, err)
	require.NotZero(t, refused.ResultCode)
	simulated, err := f.host.Simulate(ctx, absent, f.authority)
	require.NoError(t, err)
	require.Equal(t, refused, simulated)
}

func TestRepairM_909_SimulationPaymentReference(t *testing.T) {
	f := repairM909Funded(t)
	for _, reopen := range []bool{false, true} {
		if reopen {
			f.reopen(t)
		}
		ctx := f.chain.context(2, resolutionBlockTime, 8)
		before := f.chain.dump()
		duplicate, authority := repairM909Send(t, f.host, f.actor, f.asset, f.payee, repeated(0x91), 6, 1)
		sibling, _ := ctx.CacheContext()
		executed, executeErr := f.host.Execute(sibling, duplicate, authority)
		require.Equal(t, codeIdempotentReplay, executeErr)
		require.Equal(t, Receipt{}, executed)
		simulated, err := f.host.Simulate(ctx, duplicate, authority)
		require.Equal(t, executeErr, err)
		require.Equal(t, Receipt{}, simulated)
		require.Equal(t, before, f.chain.dump())
		fresh, _ := repairM909Send(t, f.host, f.actor, f.asset, f.payee, repeated(0x92), 6, 1)
		sibling, _ = ctx.CacheContext()
		positive, err := f.host.Execute(sibling, fresh, authority)
		require.NoError(t, err)
		require.Zero(t, positive.ResultCode)
		hypothetical, err := f.host.Simulate(ctx, fresh, authority)
		require.NoError(t, err)
		require.Equal(t, positive, hypothetical)
		require.Equal(t, before, f.chain.dump())
	}
}

func TestRepairM_909_SimulationFeeRecordLoading(t *testing.T) {
	fixture := newFeeRecordLoadFixture(t, false)
	directory := t.TempDir()
	chain, host, closeStore := repairM909Open(t, directory)
	for key, value := range fixture.chain.dump() {
		chain.context(1, feeRecordLoadTime, 2).KVStore(chain.key).Set([]byte(key), value)
	}
	records := repairM909Records(t, chain, fixture.asset, fixture.actor.id, repeated(0x71))
	for _, reopen := range []bool{false, true} {
		if reopen {
			chain.commit()
			committed := chain.cms.LastCommitID()
			closeStore()
			chain, host, closeStore = repairM909Open(t, directory)
			require.Equal(t, committed, chain.cms.LastCommitID())
		}
		ctx := chain.context(2, feeRecordLoadTime, 2)
		before := chain.dump()
		sibling, _ := ctx.CacheContext()
		executed, err := host.Execute(sibling, fixture.activity, fixture.actor.authority())
		require.NoError(t, err)
		require.Zero(t, executed.ResultCode)
		simulated, err := host.Simulate(ctx, fixture.activity, fixture.actor.authority())
		require.NoError(t, err)
		require.Equal(t, executed, simulated)
		require.Equal(t, before, chain.dump())
		grant, err := host.ReadGrant(sibling, fixture.grantID)
		require.NoError(t, err)
		require.True(t, grant.FeeBudget.Present)
		require.Equal(t, fixture.asset, grant.FeeBudget.AssetID)
		require.Equal(t, records["progfee/active/v1"], chain.get(moduleKVKey(9, []byte("progfee/active/v1"))))
	}
}

func TestRepairM_909_ProgramReadProducerProof(t *testing.T) {
	testReadProgramLifecycleProducers(t)
}
