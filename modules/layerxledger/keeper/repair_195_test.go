package keeper_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type repair195Keepers struct {
	*app.PrecompileKeepers
	ledger *ledgerkeeper.Keeper
}

func (k repair195Keepers) LayerXLedgerK() *ledgerkeeper.Keeper { return k.ledger }

type repair195Fixture struct {
	*mirrorFixture
	precompile                             *pcommon.Precompile
	issuer, payer, payee                   ledgerkeeper.Actor
	issuerCaller, payerCaller, payeeCaller common.Address
	main, holding, to                      [32]byte
}

func (f *repair195Fixture) bind(t *testing.T, identity ledgerViewActor, caller common.Address) ledgerkeeper.Actor {
	t.Helper()
	evmKeeper := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evmKeeper.ChainID(f.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(identity.private, message))
	_, err = evmKeeper.BindLayerX(f.ctx, caller, identity.key, signature)
	require.NoError(t, err)
	actor, err := f.keeper.Actor(f.ctx, caller)
	require.NoError(t, err)
	require.Equal(t, identity.key, actor.DIDKey)
	return actor
}

func (f *repair195Fixture) signed(t *testing.T, identity ledgerViewActor, sequence uint64, reference [32]byte, payload []byte) lxphost.Activity {
	t.Helper()
	now := uint64(f.ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{
		ProtocolVersion: codec.StateCommitmentProtocolVersion, Type: 0x00070001,
		ActorDID: identity.did, AuthorityKey: identity.key, AccountSequence: sequence,
		NotBefore: now - 1000, NotAfter: now + 1000, IdempotencyKey: reference, Payload: payload,
	}
	hash, err := f.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(identity.private, hash[:])
	return activity
}

func (f *repair195Fixture) governance(t *testing.T, identity ledgerViewActor, payload []byte, reference [32]byte) {
	t.Helper()
	sequence, err := f.host.ReadIdentitySequence(f.ctx, identity.did)
	require.NoError(t, err)
	ctx, err := ledgerkeeper.WithSequence(f.ctx)
	require.NoError(t, err)
	receipt, err := f.host.Execute(ctx, f.signed(t, identity, sequence, reference, payload), lxphost.KernelAuthority{
		Actor: identity.id, Principal: identity.id, VerifiedKey: identity.key,
	})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.NotEmpty(t, receipt.Encoded)
}

func repair195Consent(t *testing.T, activity lxphost.Activity) []byte {
	t.Helper()
	hash, err := codec.DomainHash(codec.DomainPayloadHash, activity.Payload)
	require.NoError(t, err)
	encoded := binary.BigEndian.AppendUint16(nil, activity.ProtocolVersion)
	encoded = binary.BigEndian.AppendUint16(encoded, 0x1001)
	encoded = append(encoded, 12)
	field := func(tag byte, value []byte) { encoded = append(append(encoded, tag), value...) }
	bytesField := func(tag byte, value []byte) {
		field(tag, binary.BigEndian.AppendUint32(nil, uint32(len(value))))
		encoded = append(encoded, value...)
	}
	field(1, binary.BigEndian.AppendUint16(nil, activity.ProtocolVersion))
	field(2, binary.BigEndian.AppendUint32(nil, 125))
	field(3, binary.BigEndian.AppendUint32(nil, activity.Type))
	bytesField(4, activity.ActorDID)
	bytesField(5, activity.AuthorityKey[:])
	field(6, binary.BigEndian.AppendUint64(nil, activity.AccountSequence))
	field(7, binary.BigEndian.AppendUint64(nil, activity.NotBefore))
	encoded = binary.BigEndian.AppendUint64(encoded, activity.NotAfter)
	bytesField(8, activity.IdempotencyKey[:])
	field(9, activity.FeeLimit[:])
	bytesField(10, hash[:])
	bytesField(11, activity.Payload)
	bytesField(12, activity.Signature)
	return encoded
}

func newRepair195Fixture(t *testing.T, openHolding bool) *repair195Fixture {
	t.Helper()
	f := &repair195Fixture{mirrorFixture: newMirrorFixture(t, nil),
		issuerCaller: common.Address{0x19, 0x5, 1}, payerCaller: common.Address{0x19, 0x5, 2},
		payeeCaller: common.Address{0x19, 0x5, 3}}
	f.ctx = f.ctx.WithChainID("hyperpax_125-1")
	payer := newLedgerViewActor(t, 201)
	f.issuer = f.bind(t, f.actor, f.issuerCaller)
	f.payer = f.bind(t, payer, f.payerCaller)
	f.payee = f.bind(t, newLedgerViewActor(t, 202), f.payeeCaller)
	var err error
	f.precompile, err = layerxledger.NewPrecompile(repair195Keepers{
		PrecompileKeepers: testkeeper.EVMTestApp.GetPrecompileKeepers().(*app.PrecompileKeepers), ledger: f.keeper,
	})
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), f.precompile.Address())

	// Configure the native occupancy asset before the real onboarding producer
	// creates the payer's main account. No account row or sequence is fabricated.
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, f.asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, f.issuer.Authority.Actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	require.Len(t, fee, 217)
	moduleKey := func(key []byte) []byte { return append([]byte{0x01, 0, 9}, key...) }
	f.ctx.KVStore(f.hostKey).Set(moduleKey([]byte("progfee/active/v1")), fee)
	f.ctx.KVStore(f.hostKey).Set(moduleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)
	registration := append([]byte{0x71, 1, 0, 2}, f.actor.id[:]...)
	f.governance(t, f.actor, append(registration, f.actor.key[:]...), [32]byte{0x91})
	require.Equal(t, f.asset, f.host.ProgramsOccupancyAsset())
	name, main, err := lxphost.MainAccountID(f.payer.DIDKey)
	require.NoError(t, err)
	f.main = main
	reference := [32]byte{0x92}
	consent := append([]byte{0x71, 1, 3, 5}, f.issuer.Authority.Actor[:]...)
	consent = append(consent, f.asset[:]...)
	consent = append(consent, main[:]...)
	consent = append(consent, reference[:]...)
	consent = binary.BigEndian.AppendUint64(consent, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	encoded := repair195Consent(t, f.signed(t, payer, 0, reference, consent))
	wrapper := binary.BigEndian.AppendUint32([]byte{0x71, 1, 2, 1}, uint32(len(encoded)))
	f.governance(t, f.actor, append(wrapper, encoded...), reference)
	account, err := f.host.ReadAccount(f.ctx, name, main)
	require.NoError(t, err)
	require.Equal(t, uint8(1), account.Kind)
	require.Equal(t, f.asset, account.AssetID)
	require.Zero(t, account.NextSequence)

	var salt [32]byte
	for i := range salt {
		salt[i] = 0x41
	}
	material := append(append([]byte("LX:ASSET:v1"), f.issuer.Authority.Actor[:]...), salt[:]...)
	require.Equal(t, f.asset, sha256.Sum256(material))
	values, _ := f.success(t, f.issuerCaller, "register", f.asset, salt,
		"RPR", "Repair ledger", uint8(6), big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{})
	require.Equal(t, []interface{}{f.asset}, values)
	if openHolding {
		values, _ = f.success(t, f.payerCaller, "openAccount", f.asset)
		f.holding = values[0].([32]byte)
		id, err := ledgerkeeper.AccountID(f.payer.DIDKey, f.asset)
		require.NoError(t, err)
		require.Equal(t, id, f.holding)
	}
	values, _ = f.success(t, f.payeeCaller, "openAccount", f.asset)
	f.to = values[0].([32]byte)
	return f
}

func (f *repair195Fixture) call(t *testing.T, caller common.Address, method string, args ...interface{}) ([]byte, []*ethtypes.Log, error) {
	t.Helper()
	input, err := f.precompile.GetABI().Pack(method, args...)
	require.NoError(t, err)
	evmKeeper := &testkeeper.EVMTestApp.EvmKeeper
	db := state.NewDBImpl(f.ctx.WithEventManager(sdk.NewEventManager()), evmKeeper, false)
	block, err := evmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	config := evmtypes.DefaultChainConfig().EthereumConfig(evmKeeper.ChainID(f.ctx))
	evm := vm.NewEVM(*block, db, config, vm.Config{}, map[common.Address]vm.PrecompiledContract{f.precompile.Address(): f.precompile})
	evm.TxContext.Origin = caller
	var data []byte
	if f.precompile.GetABI().Methods[method].IsConstant() {
		data, _, err = evm.StaticCall(caller, f.precompile.Address(), input, 2_000_000)
	} else {
		data, _, err = evm.Call(caller, f.precompile.Address(), input, 2_000_000, uint256.NewInt(0))
	}
	logs := db.GetAllLogs()
	if err == nil {
		_, finalizeErr := db.Finalize()
		require.NoError(t, finalizeErr)
	}
	return data, logs, err
}

func (f *repair195Fixture) success(t *testing.T, caller common.Address, method string, args ...interface{}) ([]interface{}, []*ethtypes.Log) {
	t.Helper()
	data, logs, err := f.call(t, caller, method, args...)
	require.NoError(t, err, "%s: %x", method, data)
	values, err := f.precompile.GetABI().Methods[method].Outputs.Unpack(data)
	require.NoError(t, err)
	return values, logs
}

func (f *repair195Fixture) balance(t *testing.T, id [32]byte, want int64) {
	t.Helper()
	values, _ := f.success(t, f.payerCaller, "balanceOf", id, f.asset)
	require.Zero(t, values[0].(*big.Int).Cmp(big.NewInt(want)))
	actual, err := f.keeper.BalanceOf(f.ctx, id, f.asset)
	require.NoError(t, err)
	require.True(t, actual.Equal(sdk.NewInt(want)))
	require.True(t, actual.Equal(f.bank.GetBalance(f.ctx, lxphost.BankAddress(id), types.MirrorDenom(f.asset)).Amount))
}

func (f *repair195Fixture) sequence(t *testing.T, id [32]byte, want uint64) {
	t.Helper()
	values, _ := f.success(t, f.payerCaller, "nextSequence", id, f.asset)
	require.Equal(t, []interface{}{want}, values)
}

func (f *repair195Fixture) envelope(t *testing.T, actor ledgerkeeper.Actor, from, to [32]byte, asset [32]byte, sequence uint64, amount int64, reference byte) []byte {
	t.Helper()
	args := types.SendArgs{From: from, To: to, Asset: asset, Amount: sdk.NewInt(amount), Sequence: sequence,
		IdempotencyKey: [32]byte{reference}, ExpiresAt: uint64(f.ctx.BlockTime().UnixMilli()) + 60_000,
		PublicKey: actor.DIDKey, NetworkID: 125, ProtocolVersion: codec.StateCommitmentProtocolVersion}
	payload, err := types.EncodeSendPayload(args)
	require.NoError(t, err)
	args.ContextHash, err = codec.DomainHash(codec.DomainContextHash, append(append([]byte(nil), payload[4:116]...), args.IdempotencyKey[:]...))
	require.NoError(t, err)
	payload, err = types.EncodeSendPayload(args)
	require.NoError(t, err)
	require.NoError(t, types.ValidateSendEnvelope(payload))
	return payload
}

func (f *repair195Fixture) log(t *testing.T, log *ethtypes.Log, name string, want map[string]interface{}) {
	t.Helper()
	definition := f.precompile.GetABI().Events[name]
	require.Equal(t, f.precompile.Address(), log.Address)
	require.Equal(t, definition.ID, log.Topics[0])
	values := map[string]interface{}{}
	require.NoError(t, definition.Inputs.NonIndexed().UnpackIntoMap(values, log.Data))
	var indexed abi.Arguments
	for _, input := range definition.Inputs {
		if input.Indexed {
			indexed = append(indexed, input)
		}
	}
	require.NoError(t, abi.ParseTopicsIntoMap(values, indexed, log.Topics[1:]))
	require.Equal(t, want, values)
}

func (f *repair195Fixture) refused(t *testing.T, caller common.Address, reason, method string, args ...interface{}) {
	t.Helper()
	hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
	ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)
	bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
	bankBefore := ledgerViewStore(t, f.ctx, bankKey)
	eventsBefore := f.ctx.EventManager().Events()
	data, logs, err := f.call(t, caller, method, args...)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	message, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, reason, message)
	require.Empty(t, logs)
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
	require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
	require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
	require.Equal(t, eventsBefore, f.ctx.EventManager().Events())
}

func TestRepair_195_SendOpenedAccount(t *testing.T) {
	f := newRepair195Fixture(t, true)
	f.success(t, f.issuerCaller, "mint", f.asset, f.holding, big.NewInt(100))
	f.balance(t, f.holding, 100)
	f.sequence(t, f.holding, 0)
	payload := f.envelope(t, f.payer, f.holding, f.to, f.asset, 0, 17, 0xa1)
	f.refused(t, f.payeeCaller, "LXP_UNAUTHORIZED_DEBIT", "send", payload)
	f.refused(t, f.payerCaller, "LXP_ASSET_MISMATCH", "send", f.envelope(t, f.payer, f.holding, f.to, [32]byte{0xff}, 0, 17, 0xa2))
	values, logs := f.success(t, f.payerCaller, "send", payload)
	require.Empty(t, values)
	require.Len(t, logs, 1)
	f.log(t, logs[0], "Transfer", map[string]interface{}{
		"from": f.holding, "to": f.to, "assetId": f.asset, "amount": big.NewInt(17), "kind": uint8(5),
	})
	f.balance(t, f.holding, 83)
	f.balance(t, f.to, 17)
	f.sequence(t, f.holding, 1)
	f.sequence(t, f.to, 0)
	f.sequence(t, f.main, 0)
	f.refused(t, f.payerCaller, "LXP_SEQUENCE_MISMATCH", "send", f.envelope(t, f.payer, f.holding, f.to, f.asset, 0, 1, 0xa3))
}

func TestRepair_195_PayExactSourceSequence(t *testing.T) {
	f := newRepair195Fixture(t, true)
	f.success(t, f.issuerCaller, "mint", f.asset, f.holding, big.NewInt(100))
	f.success(t, f.payerCaller, "send", f.envelope(t, f.payer, f.holding, f.main, f.asset, 0, 60, 0xb1))
	f.sequence(t, f.holding, 1)
	f.sequence(t, f.main, 0)
	f.balance(t, f.main, 60)
	expiry := uint64(f.ctx.BlockTime().UnixMilli()) + 60_000
	for i := uint64(0); i < 3; i++ {
		reference := [32]byte{0xb2, byte(i)}
		values, logs := f.success(t, f.payerCaller, "payExact", f.main, f.to, f.asset, big.NewInt(7), expiry, reference)
		require.Empty(t, values)
		require.Len(t, logs, 2)
		f.log(t, logs[0], "Transfer", map[string]interface{}{
			"from": f.main, "to": f.to, "assetId": f.asset, "amount": big.NewInt(7), "kind": uint8(5),
		})
		f.log(t, logs[1], "PaymentExact", map[string]interface{}{
			"paymentReference": reference, "from": f.main, "to": f.to, "assetId": f.asset, "amount": big.NewInt(7),
		})
		f.sequence(t, f.main, i+1)
		f.sequence(t, f.holding, 1)
	}
	f.balance(t, f.main, 39)
	f.balance(t, f.holding, 40)
	f.balance(t, f.to, 21)
	f.refused(t, f.payeeCaller, "LXP_UNAUTHORIZED_DEBIT", "payExact", f.main, f.to, f.asset, big.NewInt(1), expiry, [32]byte{0xb3})
	f.refused(t, f.payeeCaller, "LXP_UNAUTHORIZED_DEBIT", "payExact", f.holding, f.to, f.asset, big.NewInt(1), expiry, [32]byte{0xb4})
	f.refused(t, f.payerCaller, "LXP_ASSET_MISMATCH", "payExact", f.main, f.to, [32]byte{0xff}, big.NewInt(1), expiry, [32]byte{0xb5})
	// Canonical asset accounts still select their own debit sequence.
	f.success(t, f.payerCaller, "payExact", f.holding, f.to, f.asset, big.NewInt(5), expiry, [32]byte{0xb6})
	f.sequence(t, f.holding, 2)
	f.sequence(t, f.main, 3)
	f.balance(t, f.holding, 35)
	f.balance(t, f.to, 26)
}

func TestRepair_195_PayExactMainWithoutAssetAccount(t *testing.T) {
	f := newRepair195Fixture(t, false)
	_, err := f.keeper.NextSequence(f.ctx, f.payer.DIDKey, f.asset)
	require.ErrorIs(t, err, lxphost.Code(-208))
	// Mint accepts canonical asset accounts. Fund the main account through
	// the public send path without opening the payer's asset account.
	f.success(t, f.issuerCaller, "mint", f.asset, f.to, big.NewInt(30))
	f.success(t, f.payeeCaller, "send", f.envelope(t, f.payee, f.to, f.main, f.asset, 0, 30, 0xc0))
	f.balance(t, f.main, 30)
	f.balance(t, f.to, 0)
	f.sequence(t, f.main, 0)
	f.sequence(t, f.to, 1)
	expiry := uint64(f.ctx.BlockTime().UnixMilli()) + 60_000
	f.success(t, f.payerCaller, "payExact", f.main, f.to, f.asset, big.NewInt(4), expiry, [32]byte{0xc1})
	f.sequence(t, f.main, 1)
	f.balance(t, f.main, 26)
	f.balance(t, f.to, 4)
	_, err = f.keeper.NextSequence(f.ctx, f.payer.DIDKey, f.asset)
	require.ErrorIs(t, err, lxphost.Code(-208))
}
