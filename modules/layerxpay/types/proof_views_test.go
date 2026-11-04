// Proves: lxphost.HostExecute kernel records -> types.KernelKVKey / sdk.KVStore / DecodeBudgetRecord, DecodeEscrowRecord, DecodeEscrowResult, DecodeServiceAgreement, DecodeServiceCommitment, DecodeServiceDelivery, DecodeServiceDispute, DecodeServiceExecution -> keeper.Keeper views and TestProofViews.
//
// The test owns its kernel host and committed in-memory database. Only genesis
// account metadata and module enable flags are seeded; every view record is
// emitted by a real kernel activity. Delivery crosses the keeper's real header
// and item assembly. The exported decoders have no persistent state, so each
// observation reads and decodes again, including after closing the producer host
// and reopening its committed store. Malformed records must produce a zero value
// and LXP_NON_CANONICAL; keeper views must refuse them, never return cached data.
package types_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	custodykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const proofViewsTime uint64 = 1700000000000

type proofViewsActor struct {
	private  ed25519.PrivateKey
	public   [32]byte
	did      []byte
	id, main [32]byte
	address  common.Address
}

type proofViewsHarness struct {
	ctx          sdk.Context
	cms          sdk.CommitMultiStore
	db           dbm.DB
	key, payKey  *sdk.KVStoreKey
	evmKey       *sdk.KVStoreKey
	custodyKey   *sdk.KVStoreKey
	transientKey *sdk.TransientStoreKey
	evm          *evmkeeper.Keeper
	custody      *custodykeeper.Keeper
	host         *lxphost.Host
	views        paykeeper.Keeper
	sequence     uint64
}

func (h *proofViewsHarness) open(t *testing.T) {
	t.Helper()
	h.cms = store.NewCommitMultiStore(h.db)
	h.cms.MountStoreWithDB(h.key, sdk.StoreTypeIAVL, h.db)
	h.cms.MountStoreWithDB(h.payKey, sdk.StoreTypeIAVL, h.db)
	h.cms.MountStoreWithDB(h.evmKey, sdk.StoreTypeIAVL, h.db)
	if h.custodyKey == nil {
		h.custodyKey = sdk.NewKVStoreKey(custodytypes.StoreKey)
	}
	h.cms.MountStoreWithDB(h.custodyKey, sdk.StoreTypeIAVL, h.db)
	if h.transientKey == nil {
		h.transientKey = sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	}
	h.cms.MountStoreWithDB(h.transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, h.cms.LoadLatestVersion())
	h.ctx = sdk.NewContext(h.cms, tmproto.Header{ChainID: "hyperpax_125-1", Height: 1, Time: time.UnixMilli(int64(proofViewsTime))}, false)
	var err error
	h.host, err = lxphost.NewHost(lxphost.Config{StoreKey: h.key, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err, "modules/lxphost/host.go: NewHost must provide the local producer")
	// Supply the real store-backed binding reader required by NewKeeper.
	// These views use only the shared host store; no EVM execution occurs.
	bindings := evmkeeper.NewKeeper(h.evmKey, nil,
		paramtypes.NewSubspace(nil, nil, h.evmKey, nil, evmtypes.ModuleName), nil,
		bankkeeper.BaseKeeper{}, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
	_, bound := bindings.GetLayerXDid(h.ctx, [20]byte{})
	require.False(t, bound)
	h.evm = bindings
	cdc := sdkcodec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	h.custody = custodykeeper.NewKeeper(cdc, h.custodyKey, nil, nil, nil, nil)
	h.views, err = paykeeper.NewKeeper(cdc, h.payKey, h.key, h.host, bindings, h.custody, "proof-views")
	require.NoError(t, err)
}

func (h *proofViewsHarness) execute(t *testing.T, a proofViewsActor, kind uint32, payload []byte, id [32]byte, bound bool) lxphost.Receipt {
	t.Helper()
	if kind>>16 == uint32(types.EscrowModuleID) {
		method := ""
		for name, activity := range types.MethodActivity {
			if activity.Type == kind {
				method = name
				break
			}
		}
		require.NotEmpty(t, method)
		receipt, err := h.views.Submit(h.ctx, a.address, method, payload, id)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
		h.sequence = receipt.GlobalSequence
		return receipt
	}
	sequence, err := h.host.ReadIdentitySequence(h.ctx, a.did)
	require.NoError(t, err)
	activity := lxphost.Activity{ProtocolVersion: 3, Type: kind, ActorDID: a.did, AuthorityKey: a.public,
		AccountSequence: sequence, NotBefore: proofViewsTime, NotAfter: proofViewsTime + 60000, IdempotencyKey: id, Payload: payload}
	authority := lxphost.KernelAuthority{Actor: a.id, Principal: a.id, VerifiedKey: a.public}
	if bound {
		authority.Bound, authority.Principal = true, a.main
	} else {
		hash, err := h.host.SigningHash(activity)
		require.NoError(t, err)
		activity.Signature = ed25519.Sign(a.private, hash[:])
	}
	h.sequence++
	receipt, err := h.host.HostExecute(lxphost.WithSequence(h.ctx, h.sequence), activity, authority)
	require.NoError(t, err, "kernel producer activity %#x", kind)
	require.Zero(t, receipt.ResultCode, "kernel producer activity %#x", kind)
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	return receipt
}

func (h *proofViewsHarness) actor(t *testing.T, seed byte) proofViewsActor {
	t.Helper()
	a := proofViewsActor{private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, 32))}
	copy(a.public[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte(evmtypes.LayerXDid(a.public))
	a.address = common.BytesToAddress([]byte{seed})
	message, err := evmtypes.LayerXBindMessage(h.evm.ChainID(h.ctx), a.address, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(a.private, message))
	_, err = h.evm.BindLayerX(h.ctx, a.address, a.public, signature)
	require.NoError(t, err)
	a.id, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	_, a.main, err = lxphost.MainAccountID(a.public)
	require.NoError(t, err)
	payload := append([]byte{0x71, 1, 0, 2}, a.id[:]...)
	payload = append(payload, a.public[:]...)
	activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x00070001, ActorDID: a.did, AuthorityKey: a.public,
		NotBefore: proofViewsTime, NotAfter: proofViewsTime + 60000, IdempotencyKey: sha256.Sum256(a.did), Payload: payload}
	hash, err := h.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(a.private, hash[:])
	h.sequence++
	receipt, err := h.host.Execute(lxphost.WithSequence(h.ctx, h.sequence), activity,
		lxphost.KernelAuthority{Actor: a.id, Principal: a.id, VerifiedKey: a.public})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return a
}

func (h *proofViewsHarness) mainAccount(t *testing.T, a proofViewsActor, asset [32]byte) {
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
	h.ctx.KVStore(h.key).Set(append([]byte{0x06}, id[:]...), row)
}

func proofViewsKey(module uint16, prefix string, id [32]byte) []byte {
	return types.KernelKVKey(module, append([]byte(prefix), id[:]...))
}

// proofViewsObserve calls both public entry points on each fresh store read.
// Truncations exercise the exported refusal contract; corrupting the actual
// stored header additionally exercises the keeper consumer's refusal path.
func proofViewsObserve[T any](t *testing.T, h *proofViewsHarness, key []byte, wire func() []byte, decode func([]byte) (T, error), view func() (T, bool, error), check func(T)) {
	t.Helper()
	before := bytes.Clone(h.ctx.KVStore(h.key).Get(key))
	require.NotEmpty(t, before, "missing kernel producer record at %x", key)
	for i := 0; i < 2; i++ {
		data := wire()
		original := bytes.Clone(data)
		got, err := decode(data)
		require.NoError(t, err)
		check(got)
		observed, found, err := view()
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, got, observed)
		check(observed)
		require.Equal(t, original, data, "decoder mutated producer bytes")
		clear(data)
		check(got)
		check(observed)
	}
	data := wire()
	var zero T
	for _, malformed := range [][]byte{nil, data[:1], data[:len(data)-1], append(bytes.Clone(data), 0)} {
		got, err := decode(malformed)
		require.EqualError(t, err, "LXP_NON_CANONICAL", "malformed length %d", len(malformed))
		require.Equal(t, zero, got)
	}
	h.ctx.KVStore(h.key).Set(key, before[:len(before)-1])
	corrupt, found, err := view()
	require.ErrorContains(t, err, "LXP_NON_CANONICAL", "keeper must refuse a malformed stored producer result")
	require.False(t, found)
	require.Equal(t, zero, corrupt)
	h.ctx.KVStore(h.key).Set(key, before)
	restored, found, err := view()
	require.NoError(t, err)
	require.True(t, found)
	check(restored)
}

func TestProofViews(t *testing.T) {
	proofViewsRun(t, nil)
}

func proofViewsRun(t *testing.T, inspect func(*proofViewsHarness)) {
	h := &proofViewsHarness{db: dbm.NewMemDB(), key: sdk.NewKVStoreKey("lxphost"), payKey: sdk.NewKVStoreKey(types.StoreKey), evmKey: sdk.NewKVStoreKey(evmtypes.StoreKey)}
	h.open(t)
	t.Cleanup(func() { h.host.Close(); require.NoError(t, h.db.Close()) })
	buyer, provider := h.actor(t, 1), h.actor(t, 2)
	for _, module := range []uint16{2, 3, 4, 5} {
		key, err := types.KernelModuleEnableKey(module)
		require.NoError(t, err)
		flag := make([]byte, 32)
		flag[31] = 1
		h.ctx.KVStore(h.key).Set(key, flag)
	}
	h.views.SetParams(h.ctx, types.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true})
	salt := [32]byte{0x31}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), buyer.id[:]...), salt[:]...))
	h.mainAccount(t, buyer, asset)
	h.mainAccount(t, provider, asset)
	assetName, assetAccount, err := lxphost.AssetAccountID(buyer.public, asset)
	require.NoError(t, err)
	registration := append(append([]byte{0, 1}, asset[:]...), salt[:]...)
	registration = append(registration, 1, 'T', 1, 'T', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 2000)
	registration = append(registration, 1, 0)
	h.execute(t, buyer, 0x00010001, registration, [32]byte{1}, false)
	h.execute(t, buyer, 0x00010004, append([]byte{0, 1}, asset[:]...), [32]byte{2}, false)
	mint := append(append([]byte{0, 1}, asset[:]...), assetAccount[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 1000)
	h.execute(t, buyer, 0x0001000a, mint, [32]byte{3}, false)
	amount := (codec.U128{Lo: 1000}).Bytes()
	transferKey := [32]byte{4}
	material := append(append(append(append([]byte(nil), assetAccount[:]...), buyer.main[:]...), asset[:]...), amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(bytes.Clone(material), transferKey[:]...))
	require.NoError(t, err)
	account, err := h.host.ReadAccount(h.ctx, assetName, assetAccount)
	require.NoError(t, err)
	transfer := binary.BigEndian.AppendUint64(material, account.NextSequence)
	transfer = append(transfer, transferKey[:]...)
	transfer = binary.BigEndian.AppendUint64(transfer, proofViewsTime+10000)
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
	payload := append([]byte{0x53, 1, 0, 10}, transfer...)
	payload = append(payload, 1)
	payload = append(payload, assetAccount[:]...)
	payload = append(payload, buyer.public[:]...)
	payload = append(payload, ed25519.Sign(buyer.private, digest[:])...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	h.execute(t, buyer, 0x00010005, payload, transferKey, false)

	escrowID, budgetID := [32]byte{20}, [32]byte{21}
	derive := func(family string, id [32]byte) [32]byte {
		account, err := codec.DeriveAccountID([]byte("agent:" + string(buyer.did) + ":" + family + ":" + hex.EncodeToString(id[:])))
		require.NoError(t, err)
		return account
	}
	escrowAccount, budgetAccount := derive("escrow", escrowID), derive("budget", budgetID)
	terms, spec := [32]byte{22}, [32]byte{23}
	escrowReceipt := h.execute(t, buyer, 0x00020001, types.EncodeEscrowOpen(escrowID, buyer.main, escrowAccount, provider.main,
		buyer.main, asset, codec.U128{Lo: 50}, proofViewsTime+1000000, 30000, terms, [32]byte{}), escrowID, true)
	payload, err = types.EncodeBudgetCreate(budgetID, budgetAccount, asset, [32]byte{24}, codec.U128{Lo: 900}, codec.U128{Lo: 50},
		codec.U128{Lo: 600}, 60000, proofViewsTime, proofViewsTime+120000, 7, 2)
	require.NoError(t, err)
	h.execute(t, buyer, 0x00030001, payload, budgetID, true)

	offerID, agreementID, commitmentID := [32]byte{30}, [32]byte{31}, [32]byte{32}
	payload, err = types.EncodeServiceOffer(offerID, asset, codec.U128{Lo: 50}, terms, spec, proofViewsTime+100000, 20000, 30000, 1, proofViewsTime+90000)
	require.NoError(t, err)
	offered := h.execute(t, provider, 0x00050001, payload, [32]byte{33}, true)
	payload, err = types.EncodeServicePropose(agreementID, offerID, terms, escrowID)
	require.NoError(t, err)
	h.execute(t, buyer, 0x00050003, payload, [32]byte{34}, true)
	payload, err = types.EncodeServiceIdentifier(agreementID)
	require.NoError(t, err)
	accepted := h.execute(t, provider, 0x00050004, payload, [32]byte{35}, true)
	payload, err = types.EncodeServiceCommit(commitmentID, agreementID, [32]byte{36}, escrowID, proofViewsTime+90000, 1000)
	require.NoError(t, err)
	committed := h.execute(t, provider, 0x00050005, payload, [32]byte{37}, true)
	attestationID, attestationKey := [32]byte{38}, [32]byte{39}
	execution := make([]byte, types.ServiceExecutionBytes)
	for i, field := range [][32]byte{attestationID, attestationKey, agreementID, commitmentID, {40}, {41}, {42}} {
		copy(execution[32*i:32*(i+1)], field[:])
	}
	binary.BigEndian.PutUint64(execution[224:232], proofViewsTime-100)
	binary.BigEndian.PutUint64(execution[232:240], proofViewsTime)
	binary.BigEndian.PutUint64(execution[240:248], 250)
	copy(execution[248:280], provider.main[:])
	reference := [32]byte{43}
	copy(execution[280:312], reference[:])
	copy(execution[312:344], provider.public[:])
	canonical := append([]byte("LXP:SERVICE:ATTEST:v1"), execution[:344]...)
	digest, err = codec.DomainHash(codec.DomainSignaturePreimage, canonical)
	require.NoError(t, err)
	copy(execution[344:408], ed25519.Sign(provider.private, digest[:]))
	payload, err = types.EncodeServiceToolExecAttest(execution)
	require.NoError(t, err)
	attested := h.execute(t, provider, 0x00050007, payload, attestationKey, true)
	reportID := [32]byte{44}
	payload, err = types.EncodeServiceProgress(reportID, commitmentID, [32]byte{45}, reference, 5000)
	require.NoError(t, err)
	progressed := h.execute(t, provider, 0x00050008, payload, [32]byte{46}, true)
	deliveryID := [32]byte{47}
	items := []types.ServiceDeliverable{{Hash: spec, ArtifactSize: 4096, AvailabilityReference: reference}}
	payload, err = types.EncodeServiceDeliver(deliveryID, agreementID, items)
	require.NoError(t, err)
	delivered := h.execute(t, provider, 0x00050009, payload, [32]byte{48}, true)
	payload, err = types.EncodeServiceReject(agreementID, 7, [][32]byte{spec})
	require.NoError(t, err)
	rejected := h.execute(t, buyer, 0x0005000b, payload, [32]byte{49}, true)
	disputeID := [32]byte{50}
	payload, err = types.EncodeServiceDisputeOpen(disputeID, agreementID, [][32]byte{{52}, {51}})
	require.NoError(t, err)
	disputed := h.execute(t, buyer, 0x0005000c, payload, [32]byte{53}, true)

	committedReceipt, err := codec.DecodeReceipt(committed.Encoded)
	require.NoError(t, err)
	disputedReceipt, err := codec.DecodeReceipt(disputed.Encoded)
	require.NoError(t, err)
	escrowEconomicReceipt, err := codec.DecodeReceipt(escrowReceipt.Encoded)
	require.NoError(t, err)
	escrowContextInput := append([]byte("escrow-replay-v2"), escrowID[:]...)
	escrowContextInput = binary.BigEndian.AppendUint16(escrowContextInput, 1)
	escrowContextInput = append(escrowContextInput, make([]byte, 161)...)
	escrowAmount := (codec.U128{Lo: 50}).Bytes()
	escrowContextInput = append(escrowContextInput, escrowAmount[:]...)
	escrowContextInput = append(escrowContextInput, escrowAccount[:]...)
	escrowContextInput = binary.BigEndian.AppendUint32(escrowContextInput, 0)
	escrowContext, err := codec.DomainHash(codec.DomainContextHash, escrowContextInput)
	require.NoError(t, err)

	if inspect != nil {
		inspect(h)
	}
	// Recreate the store and host before any consumer observation.
	h.cms.Commit(true)
	h.host.Close()
	h.open(t)
	read := func(key []byte) func() []byte {
		return func() []byte { return bytes.Clone(h.ctx.KVStore(h.key).Get(key)) }
	}
	t.Run("budget", func(t *testing.T) {
		key := proofViewsKey(3, "budget:", budgetID)
		want := types.BudgetRecord{BudgetID: budgetID, Owner: buyer.main, BudgetAccount: budgetAccount, AssetID: asset, PurposeHash: [32]byte{24},
			PerPeriodLimit: codec.U128{Lo: 900}, ConfiguredPeriodLimit: codec.U128{Lo: 900}, CarryCap: codec.U128{Lo: 50},
			PeriodLengthMs: 60000, PeriodStartMs: proofViewsTime, ExpiryMs: proofViewsTime + 120000, RevocationSequence: 7, RolloverPolicy: 2, Delegates: [][32]byte{}}
		// Budget exposes a typed error instead of panicking on corruption.
		for i := 0; i < 2; i++ {
			got, err := types.DecodeBudgetRecord(read(key)())
			require.NoError(t, err)
			require.Equal(t, want, got)
			observed, found, err := h.views.GetBudget(h.ctx, budgetID)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, want, observed)
			remaining, err := h.views.BudgetRemaining(h.ctx, budgetID)
			require.NoError(t, err)
			require.Equal(t, "600", remaining.String())
		}
		data := read(key)()
		for n := 0; n < len(data); n++ {
			got, err := types.DecodeBudgetRecord(data[:n])
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Equal(t, types.BudgetRecord{}, got)
		}
		h.ctx.KVStore(h.key).Set(key, data[:len(data)-1])
		got, found, err := h.views.GetBudget(h.ctx, budgetID)
		require.ErrorContains(t, err, "LXP_NON_CANONICAL")
		require.False(t, found)
		require.Equal(t, types.BudgetRecord{}, got)
		h.ctx.KVStore(h.key).Set(key, data)
	})
	t.Run("escrow", func(t *testing.T) {
		key := proofViewsKey(2, "hold:", escrowID)
		proofViewsObserve(t, h, key, read(key), types.DecodeEscrowRecord, func() (types.EscrowRecord, bool, error) { return h.views.GetEscrow(h.ctx, escrowID) }, func(r types.EscrowRecord) {
			require.Equal(t, types.EscrowRecord{EscrowID: escrowID, Owner: buyer.main, EscrowAccount: escrowAccount, Beneficiary: provider.main, Arbiter: buyer.main,
				AssetID: asset, LockedAmount: codec.U128{Lo: 50}, State: 1, ExpiryMs: proofViewsTime + 1000000, DisputeWindowMs: 30000, TermsHash: terms}, r)
		})
	})
	t.Run("escrow_result", func(t *testing.T) {
		key := proofViewsKey(2, "result:", escrowID)
		require.NotEqual(t, [32]byte{}, escrowEconomicReceipt.TransferSetRoot)
		stored, err := types.DecodeEscrowResult(read(key)())
		require.NoError(t, err)
		require.Equal(t, escrowEconomicReceipt.TransferSetRoot, stored.TransferSetRoot)
		proofViewsObserve(t, h, key, read(key), types.DecodeEscrowResult, func() (types.EscrowResult, bool, error) { return h.views.GetEscrowResult(h.ctx, escrowID) }, func(r types.EscrowResult) {
			require.Equal(t, escrowID, r.EscrowID)
			require.Equal(t, uint16(1), r.Ordinal)
			require.Equal(t, uint8(1), r.StateAfter)
			require.Equal(t, codec.U128{Lo: 50}, r.LockedAfter)
			require.Equal(t, codec.U128{Lo: 50}, r.Amount)
			require.Equal(t, asset, r.AssetID)
			require.Equal(t, buyer.main, r.From)
			require.Equal(t, escrowAccount, r.To)
			require.Equal(t, escrowReceipt.GlobalSequence, r.GlobalSequence)
			require.Equal(t, proofViewsTime, r.TimestampMs)
			require.Equal(t, escrowEconomicReceipt.TransferSetRoot, r.TransferSetRoot)
			require.Equal(t, codec.U128{}, r.CapturedAfter)
			require.Equal(t, codec.U128{}, r.SecondaryAmount)
			require.True(t, r.ContextBound)
			require.Equal(t, escrowContext, r.ContextDigest)
		})
	})
	t.Run("escrow_replay_after_reopen", func(t *testing.T) {
		before, err := h.host.ReadBalance(h.ctx, buyer.main, asset)
		require.NoError(t, err)
		require.Equal(t, "350", before.String())
		captureKey := [32]byte{0xe1}
		payload := types.EncodeEscrowCapture(escrowID, codec.U128{Lo: 20}, captureKey)
		receipt := h.execute(t, provider, types.ActivityEscrowPartialCapture, payload, [32]byte{0xe2}, true)
		economic, found, err := h.views.GetEscrowResult(h.ctx, captureKey)
		require.NoError(t, err)
		require.True(t, found)
		outer, err := codec.DecodeReceipt(receipt.Encoded)
		require.NoError(t, err)
		require.NotEqual(t, [32]byte{}, outer.TransferSetRoot)
		require.Equal(t, outer.TransferSetRoot, economic.TransferSetRoot)
		require.Equal(t, codec.U128{Lo: 20}, economic.Amount)
		require.Equal(t, codec.U128{Lo: 30}, economic.LockedAfter)
		require.Equal(t, codec.U128{Lo: 20}, economic.CapturedAfter)
		require.Equal(t, escrowAccount, economic.From)
		require.Equal(t, provider.main, economic.To)
		require.Equal(t, codec.U128{}, economic.SecondaryAmount)
		for round := 0; round < 2; round++ {
			h.cms.Commit(true)
			h.host.Close()
			h.open(t)
			replay := h.execute(t, provider, types.ActivityEscrowPartialCapture, payload, [32]byte{byte(0xe3 + round)}, true)
			replayOuter, err := codec.DecodeReceipt(replay.Encoded)
			require.NoError(t, err)
			require.Equal(t, outer.TransferSetRoot, replayOuter.TransferSetRoot)
			durable, found, err := h.views.GetEscrowReceipt(h.ctx, replay.GlobalSequence)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, replay.Encoded, durable.CanonicalBytes())
			for read := 0; read < 2; read++ {
				result, found, err := h.views.GetEscrowResult(h.ctx, captureKey)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, economic, result)
				for account, want := range map[[32]byte]string{buyer.main: "350", provider.main: "20", escrowAccount: "30"} {
					balance, err := h.host.ReadBalance(h.ctx, account, asset)
					require.NoError(t, err)
					require.Equal(t, want, balance.String())
				}
			}
		}
	})
	t.Run("agreement", func(t *testing.T) {
		key := proofViewsKey(5, "agree::1", agreementID)
		proofViewsObserve(t, h, key, read(key), types.DecodeServiceAgreement, func() (types.ServiceAgreement, bool, error) {
			return h.views.ServiceViews().GetAgreement(h.ctx, agreementID)
		}, func(r types.ServiceAgreement) {
			require.Equal(t, types.ServiceAgreement{AgreementID: agreementID, OfferID: offerID, Provider: provider.main, Buyer: buyer.main, TermsHash: terms, EscrowID: escrowID,
				DeliveryDeadlineMs: proofViewsTime + 100000, AcceptanceWindowEndMs: proofViewsTime + 120000, DisputeWindowEndMs: proofViewsTime + 150000,
				DefaultOutcome: 1, State: 6, AcceptedSequence: accepted.GlobalSequence, RejectionReason: 7, ContestedHashes: [][32]byte{spec},
				OutcomeSequence: rejected.GlobalSequence, OutcomeTimestampMs: proofViewsTime}, r)
		})
	})
	t.Run("commitment", func(t *testing.T) {
		key := proofViewsKey(5, "commit:1", commitmentID)
		proofViewsObserve(t, h, key, read(key), types.DecodeServiceCommitment, func() (types.ServiceCommitment, bool, error) {
			return h.views.ServiceViews().GetCommitment(h.ctx, commitmentID)
		}, func(r types.ServiceCommitment) {
			require.Equal(t, types.ServiceCommitment{CommitmentID: commitmentID, ActivityID: committedReceipt.ActivityID, Provider: provider.main, AgreementID: agreementID,
				TaskHash: [32]byte{36}, DeadlineMs: proofViewsTime + 90000, ResourceBound: 1000, EscrowID: escrowID, GlobalSequence: committed.GlobalSequence}, r)
		})
	})
	t.Run("execution", func(t *testing.T) {
		key := proofViewsKey(5, "execut:1", attestationID)
		proofViewsObserve(t, h, key, read(key), types.DecodeServiceExecution, func() (types.ServiceExecution, bool, error) {
			return h.views.ServiceViews().GetExecution(h.ctx, attestationID)
		}, func(r types.ServiceExecution) {
			require.Equal(t, attestationID, r.AttestationID)
			require.Equal(t, attestationKey, r.ActivityID)
			require.Equal(t, agreementID, r.AgreementID)
			require.Equal(t, commitmentID, r.CommitmentID)
			require.Equal(t, uint64(250), r.ResourceUnits)
			require.Equal(t, provider.main, r.AttestorIdentity)
			require.Equal(t, attested.GlobalSequence, r.GlobalSequence)
			require.Equal(t, reference, r.AvailabilityReference)
			require.Equal(t, canonical, r.CanonicalPayload[:r.CanonicalPayloadLength])
			require.True(t, ed25519.Verify(ed25519.PublicKey(r.PublicKey[:]), digest[:], r.Signature[:]))
		})
	})
	t.Run("delivery", func(t *testing.T) {
		key := proofViewsKey(5, "delivr:1", deliveryID)
		wire := func() []byte {
			data := read(key)()
			for i := range items {
				data = append(data, h.ctx.KVStore(h.key).Get(append(proofViewsKey(5, "delitm:1", deliveryID), byte(i)))...)
			}
			return data
		}
		proofViewsObserve(t, h, key, wire, types.DecodeServiceDelivery, func() (types.ServiceDelivery, bool, error) {
			return h.views.ServiceViews().GetDelivery(h.ctx, deliveryID)
		}, func(r types.ServiceDelivery) {
			require.Equal(t, types.ServiceDelivery{AgreementID: agreementID, Provider: provider.main, Deliverables: items, GlobalSequence: delivered.GlobalSequence}, r)
		})
		itemKey := append(proofViewsKey(5, "delitm:1", deliveryID), byte(0))
		item := read(itemKey)()
		h.ctx.KVStore(h.key).Delete(itemKey)
		missing, found, err := h.views.ServiceViews().GetDelivery(h.ctx, deliveryID)
		require.ErrorIs(t, err, paykeeper.ErrServiceCorruptState)
		require.False(t, found)
		require.Equal(t, types.ServiceDelivery{}, missing)
		h.ctx.KVStore(h.key).Set(itemKey, item)
	})
	t.Run("dispute", func(t *testing.T) {
		key := proofViewsKey(5, "disput:1", disputeID)
		proofViewsObserve(t, h, key, read(key), types.DecodeServiceDispute, func() (types.ServiceDispute, bool, error) { return h.views.ServiceViews().GetDispute(h.ctx, disputeID) }, func(r types.ServiceDispute) {
			require.Equal(t, types.ServiceDispute{DisputeID: disputeID, ActivityID: disputedReceipt.ActivityID, AgreementID: agreementID, Raiser: buyer.main,
				EvidenceHashes: [][32]byte{{51}, {52}}, GlobalSequence: disputed.GlobalSequence}, r)
		})
	})
	t.Run("offer", func(t *testing.T) {
		key := proofViewsKey(5, "offer::1", offerID)
		proofViewsObserve(t, h, key, read(key), types.DecodeServiceOffer, func() (types.ServiceOffer, bool, error) { return h.views.ServiceViews().GetOffer(h.ctx, offerID) }, func(r types.ServiceOffer) {
			require.Equal(t, offerID, r.OfferID)
			require.Equal(t, provider.main, r.OfferingAgent)
			require.Equal(t, asset, r.AssetID)
			require.Equal(t, offered.GlobalSequence, r.GlobalSequence)
			require.True(t, r.Accepted)
			require.False(t, r.Withdrawn)
		})
	})
	t.Run("progress", func(t *testing.T) {
		key := proofViewsKey(5, "progrs:1", reportID)
		proofViewsObserve(t, h, key, read(key), types.DecodeServiceProgress, func() (types.ServiceProgress, bool, error) {
			return h.views.ServiceViews().GetProgress(h.ctx, reportID)
		}, func(r types.ServiceProgress) {
			require.Equal(t, reportID, r.ReportID)
			require.Equal(t, commitmentID, r.CommitmentID)
			require.Equal(t, agreementID, r.AgreementID)
			require.Equal(t, uint32(5000), r.ProgressBPS)
			require.Equal(t, progressed.GlobalSequence, r.GlobalSequence)
		})
	})
	t.Run("unknown_identifiers", func(t *testing.T) {
		unknown := [32]byte{255}
		_, found, err := h.views.GetBudget(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = h.views.GetEscrow(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = h.views.GetEscrowResult(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = h.views.ServiceViews().GetAgreement(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = h.views.ServiceViews().GetCommitment(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = h.views.ServiceViews().GetExecution(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = h.views.ServiceViews().GetDelivery(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = h.views.ServiceViews().GetDispute(h.ctx, unknown)
		require.NoError(t, err)
		require.False(t, found)
	})
}
