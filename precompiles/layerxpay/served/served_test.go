package served_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func newPayServedApp(t *testing.T) *app.TestWrapper {
	t.Helper()
	w := app.NewTestWrapper(t, time.UnixMilli(1_800_000_000_000).UTC(), secp256k1.GenPrivKey().PubKey(), true)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	w.Ctx = w.Ctx.WithBlockHeight(100)
	w.App.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
	require.EqualValues(t, 100, w.App.UpgradeKeeper.GetDoneHeight(w.Ctx, app.MergeUpgrade))
	return w
}

func TestPayServedAfterUpgrade(t *testing.T) {
	w := newPayServedApp(t)
	for _, tc := range []struct {
		address string
		name    string
		methods []string
	}{
		{"0x101B", "layerxpay", []string{"escrowOpen", "getEscrow"}},
		{"0x0906", "streams", []string{"open", "getStream", "settle"}},
	} {
		t.Run(tc.address, func(t *testing.T) {
			address := common.HexToAddress(tc.address)
			for _, tracing := range []bool{false, true} {
				before := w.Ctx.WithBlockHeight(99).WithIsTracing(tracing)
				require.NotContains(t, w.App.EvmKeeper.CustomPrecompiles(before), address)
				require.NotContains(t, w.App.EvmKeeper.GetCustomPrecompilesVersions(before), address)
				after := w.Ctx.WithIsTracing(tracing)
				served := w.App.EvmKeeper.CustomPrecompiles(after)
				require.Contains(t, served, address)
				contract, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok)
				require.Equal(t, address, contract.Address())
				require.Equal(t, tc.name, contract.GetName())
				for _, method := range tc.methods {
					require.Contains(t, contract.GetABI().Methods, method)
				}
				require.Equal(t, app.MergeUpgrade, w.App.EvmKeeper.GetCustomPrecompilesVersions(after)[address])
			}
		})
	}
}

type payServedActor struct {
	private ed25519.PrivateKey
	public  [32]byte
	did     []byte
	actor   [32]byte
	caller  common.Address
	account [32]byte
}

func payServedExecute(t *testing.T, w *app.TestWrapper, actor payServedActor, kind uint32, payload []byte, key [32]byte) {
	t.Helper()
	host, ctx := w.App.LayerXHost, w.Ctx
	last, err := host.ReadLastSequence(ctx, actor.did)
	require.NoError(t, err)
	sequence, err := host.ReadIdentitySequence(ctx, actor.did)
	require.NoError(t, err)
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: kind, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(ctx.BlockTime().UnixMilli()) + host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}
	digest, err := host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(actor.private, digest[:])
	receipt, err := host.HostExecute(lxphost.WithSequence(ctx, last+1), activity,
		lxphost.KernelAuthority{Actor: actor.actor, Principal: actor.actor, VerifiedKey: actor.public})
	require.NoError(t, err, "activity %#x", kind)
	require.Zero(t, receipt.ResultCode, "activity %#x", kind)
}

func payServedBind(t *testing.T, w *app.TestWrapper, seed byte) payServedActor {
	t.Helper()
	material := [32]byte{0x38, 0x02, seed}
	a := payServedActor{private: ed25519.NewKeyFromSeed(material[:]), caller: common.BytesToAddress([]byte{0xa2, 0x38, seed})}
	copy(a.public[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte(evmtypes.LayerXDid(a.public))
	var err error
	a.actor, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	message, err := evmtypes.LayerXBindMessage(w.App.EvmKeeper.ChainID(w.Ctx), a.caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(a.private, message))
	nonce, err := w.App.EvmKeeper.BindLayerX(w.Ctx, a.caller, a.public, signature)
	require.NoError(t, err)
	require.Zero(t, nonce)
	bound, found := w.App.EvmKeeper.GetLayerXDid(w.Ctx, a.caller)
	require.True(t, found)
	require.Equal(t, a.public, bound)
	payload := append([]byte{0x71, 1, 0, 2}, a.actor[:]...)
	payload = append(payload, a.public[:]...)
	payServedExecute(t, w, a, 0x00070001, payload, sha256.Sum256(a.did))
	identity, err := w.App.LayerXHost.ReadIdentityState(w.Ctx, a.did)
	require.NoError(t, err)
	require.Equal(t, a.public, identity.PrimaryKey)
	return a
}

func payServedMainGenesis(t *testing.T, w *app.TestWrapper, actor payServedActor, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := lxphost.MainAccountID(actor.public)
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
	row = field(row, actor.public[:])
	row = append(row, 1)
	require.NoError(t, lxphost.InitStoreGenesis(w.Ctx, w.App.GetKey(app.LayerXHostStoreKey), lxphost.StoreGenesis{
		Records: []kv.Pair{{Key: append([]byte{0x06}, id[:]...), Value: row}},
	}))
	account, err := w.App.LayerXHost.ReadAccount(w.Ctx, name, id)
	require.NoError(t, err)
	require.Zero(t, account.NextSequence)
	return id
}

func payServedFund(t *testing.T, w *app.TestWrapper, owner payServedActor, asset, salt [32]byte) {
	t.Helper()
	assetName, assetAccount, err := lxphost.AssetAccountID(owner.public, asset)
	require.NoError(t, err)
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 2000)
	payload = append(payload, 1, 0)
	payServedExecute(t, w, owner, 0x00010001, payload, [32]byte{0x11})
	payServedExecute(t, w, owner, 0x00010004, append([]byte{0, 1}, asset[:]...), [32]byte{0x12})
	payload = append(append([]byte{0, 1}, asset[:]...), assetAccount[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	payServedExecute(t, w, owner, 0x0001000a, payload, [32]byte{0x13})
	amount := (codec.U128{Lo: 1000}).Bytes()
	key := [32]byte{0x14}
	material := append([]byte(nil), assetAccount[:]...)
	material = append(material, owner.account[:]...)
	material = append(material, asset[:]...)
	material = append(material, amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(append([]byte(nil), material...), key[:]...))
	require.NoError(t, err)
	account, err := w.App.LayerXHost.ReadAccount(w.Ctx, assetName, assetAccount)
	require.NoError(t, err)
	transfer := binary.BigEndian.AppendUint64(material, account.NextSequence)
	transfer = append(transfer, key[:]...)
	transfer = binary.BigEndian.AppendUint64(transfer, uint64(w.Ctx.BlockTime().UnixMilli())+10_000)
	transfer = append(transfer, contextHash[:]...)
	transfer = append(transfer, 0)
	network := w.App.EvmKeeper.ChainID(w.Ctx)
	require.True(t, network.IsUint64())
	require.LessOrEqual(t, network.Uint64(), uint64(math.MaxUint32))
	suffix := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint32(nil, uint32(network.Uint64())), 3)
	message := append([]byte{0x53, 1}, transfer...)
	message = append(message, 1)
	message = append(message, assetAccount[:]...)
	message = append(message, contextHash[:]...)
	message = append(message, suffix...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	payload = append([]byte{0x53, 1, 0, 10}, transfer...)
	payload = append(payload, 1)
	payload = append(payload, assetAccount[:]...)
	payload = append(payload, owner.public[:]...)
	payload = append(payload, ed25519.Sign(owner.private, digest[:])...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	payServedExecute(t, w, owner, 0x00010005, payload, key)
}

type payServedEscrowTuple struct {
	Owner              [32]byte
	EscrowAccount      [32]byte
	Beneficiary        [32]byte
	Arbiter            [32]byte
	AssetId            [32]byte
	LockedAmount       *big.Int
	CapturedAmount     *big.Int
	State              uint8
	ExpiryMs           uint64
	DisputeWindowMs    uint64
	TermsHash          [32]byte
	AgreementReference [32]byte
}

func TestPayServedEscrowOpenThroughEVM(t *testing.T) {
	w := newPayServedApp(t)
	keepers := w.App.GetPrecompileKeepers().(*app.PrecompileKeepers)
	require.Same(t, w.App.LayerXPayKeeper, keepers.LayerXPayK())
	require.Same(t, w.App.LayerXHost, keepers.LayerXHost())
	require.Same(t, w.App.LayerXHost, w.App.LayerXPayKeeper.Host())
	params := w.App.LayerXPayKeeper.GetParams(w.Ctx)
	params.Paused, params.EscrowEnabled = false, true
	w.App.LayerXPayKeeper.SetParams(w.Ctx, params)
	enableKey, err := paytypes.KernelModuleEnableKey(paytypes.EscrowModuleID)
	require.NoError(t, err)
	var enabled [32]byte
	enabled[31] = 1
	w.Ctx.KVStore(w.App.GetKey(app.LayerXHostStoreKey)).Set(enableKey, enabled[:])
	payer, payee := payServedBind(t, w, 1), payServedBind(t, w, 2)
	salt := [32]byte{0x38, 0x02}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), payer.actor[:]...), salt[:]...))
	payer.account = payServedMainGenesis(t, w, payer, asset)
	payee.account = payServedMainGenesis(t, w, payee, asset)
	payServedFund(t, w, payer, asset, salt)
	beforeBalance, err := w.App.LayerXHost.ReadBalance(w.Ctx, payer.account, asset)
	require.NoError(t, err)
	require.Equal(t, "1000", beforeBalance.String())
	id := [32]byte{0x38, 0x02, 0x01}
	escrowAccount, err := codec.DeriveAccountID([]byte("agent:" + string(payer.did) + ":escrow:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	expiry, window := uint64(w.Ctx.BlockTime().UnixMilli())+1_000_000, uint64(12_345)
	terms, agreement := [32]byte{0x38, 3}, [32]byte{0x38, 4}
	_, found := w.App.LayerXPayKeeper.GetEscrow(w.Ctx, id)
	require.False(t, found)
	last, err := w.App.LayerXHost.ReadLastSequence(w.Ctx, payer.did)
	require.NoError(t, err)
	sequence, err := w.App.LayerXHost.ReadIdentitySequence(w.Ctx, payer.did)
	require.NoError(t, err)
	ctx := w.Ctx.WithEVMTxHash(common.HexToHash("0x2382")).WithEventManager(sdk.NewEventManager())
	served := w.App.EvmKeeper.CustomPrecompiles(ctx)
	address := common.HexToAddress("0x101B")
	require.Contains(t, served, address)
	contract, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok)
	a := contract.GetABI()
	db := state.NewDBImpl(ctx, &w.App.EvmKeeper, false)
	block, err := w.App.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(w.App.EvmKeeper.ChainID(db.Ctx()))
	evm := vm.NewEVM(*block, db, chain, vm.Config{}, served)
	input, err := a.Pack("escrowOpen", id, payer.account, escrowAccount, payee.account, payer.actor,
		asset, big.NewInt(600), expiry, window, terms, agreement)
	require.NoError(t, err)
	output, _, err := evm.Call(payer.caller, address, input, 2_000_000, uint256.NewInt(0))
	require.NoError(t, err, "escrowOpen output: %x", output)
	require.Empty(t, output)
	require.NoError(t, db.Err())
	want := paytypes.EscrowRecord{
		EscrowID: id, Owner: payer.account, EscrowAccount: escrowAccount, Beneficiary: payee.account,
		Arbiter: payer.actor, AssetID: asset, LockedAmount: codec.U128{Lo: 600}, State: 1,
		ExpiryMs: expiry, DisputeWindowMs: window, TermsHash: terms, AgreementReference: agreement,
	}
	record, found := w.App.LayerXPayKeeper.GetEscrow(db.Ctx(), id)
	require.True(t, found)
	require.Equal(t, want, record)
	storageKey := paytypes.KernelKVKey(paytypes.EscrowModuleID, append([]byte("hold:"), id[:]...))
	raw := append([]byte(nil), db.Ctx().KVStore(w.App.GetKey(app.LayerXHostStoreKey)).Get(storageKey)...)
	stored, err := paytypes.DecodeEscrowRecord(raw)
	require.NoError(t, err)
	require.Equal(t, want, stored)
	for _, balance := range []struct {
		account [32]byte
		want    string
	}{{payer.account, "400"}, {escrowAccount, "600"}, {payee.account, "0"}} {
		got, err := w.App.LayerXHost.ReadBalance(db.Ctx(), balance.account, asset)
		require.NoError(t, err)
		require.Equal(t, balance.want, got.String())
	}
	after, err := w.App.LayerXHost.ReadLastSequence(db.Ctx(), payer.did)
	require.NoError(t, err)
	require.Equal(t, last+1, after)
	after, err = w.App.LayerXHost.ReadIdentitySequence(db.Ctx(), payer.did)
	require.NoError(t, err)
	require.Equal(t, sequence+1, after)
	logs := db.GetAllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, address, logs[0].Address)
	require.Equal(t, []common.Hash{a.Events["EscrowOpened"].ID, common.Hash(id)}, logs[0].Topics)
	input, err = a.Pack("getEscrow", id)
	require.NoError(t, err)
	output, _, err = evm.StaticCall(payee.caller, address, input, 2_000_000)
	require.NoError(t, err, "getEscrow output: %x", output)
	require.NoError(t, db.Err())
	values, err := a.Methods["getEscrow"].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, values, 1)
	got := abi.ConvertType(values[0], new(payServedEscrowTuple)).(*payServedEscrowTuple)
	require.Equal(t, record.Owner, got.Owner)
	require.Equal(t, record.EscrowAccount, got.EscrowAccount)
	require.Equal(t, record.Beneficiary, got.Beneficiary)
	require.Equal(t, record.Arbiter, got.Arbiter)
	require.Equal(t, record.AssetID, got.AssetId)
	require.Zero(t, got.LockedAmount.Cmp(big.NewInt(600)))
	require.Zero(t, got.CapturedAmount.Sign())
	require.Equal(t, record.State, got.State)
	require.Equal(t, record.ExpiryMs, got.ExpiryMs)
	require.Equal(t, record.DisputeWindowMs, got.DisputeWindowMs)
	require.Equal(t, record.TermsHash, got.TermsHash)
	require.Equal(t, record.AgreementReference, got.AgreementReference)
	require.Equal(t, raw, db.Ctx().KVStore(w.App.GetKey(app.LayerXHostStoreKey)).Get(storageKey))
	require.Len(t, db.GetAllLogs(), 1)
}
