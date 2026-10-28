package layerxpay

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type escrowTestActor struct {
	private ed25519.PrivateKey
	public  [32]byte
	did     []byte
	actor   [32]byte
	caller  common.Address
	account [32]byte
}

type escrowTestFixture struct {
	h           *payPrecompileHarness
	owner       escrowTestActor
	beneficiary escrowTestActor
	arbiter     escrowTestActor
	asset       [32]byte
}

func escrowTestExecute(t *testing.T, h *payPrecompileHarness, a escrowTestActor, kind uint32, payload []byte, key [32]byte) {
	t.Helper()
	ctx, host := h.db.Ctx(), h.keeper.Host()
	last, err := host.ReadLastSequence(ctx, a.did)
	require.NoError(t, err)
	seq, err := host.ReadIdentitySequence(ctx, a.did)
	require.NoError(t, err)
	activity := lxphost.Activity{ProtocolVersion: 3, Type: kind, ActorDID: a.did, AuthorityKey: a.public,
		AccountSequence: seq, NotBefore: uint64(ctx.BlockTime().UnixMilli()),
		NotAfter: uint64(ctx.BlockTime().UnixMilli()) + host.TimestampWindowMs(), IdempotencyKey: key, Payload: payload}
	digest, err := host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(a.private, digest[:])
	receipt, err := host.HostExecute(lxphost.WithSequence(ctx, last+1), activity, lxphost.KernelAuthority{Actor: a.actor, Principal: a.actor, VerifiedKey: a.public})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
}

func escrowTestRegister(t *testing.T, h *payPrecompileHarness, seed [32]byte, caller common.Address, bind bool) escrowTestActor {
	t.Helper()
	a := escrowTestActor{private: ed25519.NewKeyFromSeed(seed[:]), caller: caller}
	copy(a.public[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte(evmtypes.LayerXDid(a.public))
	var err error
	a.actor, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	if bind {
		keeper := &testkeeper.EVMTestApp.EvmKeeper
		message, err := evmtypes.LayerXBindMessage(keeper.ChainID(h.db.Ctx()), caller, 0)
		require.NoError(t, err)
		var signature [64]byte
		copy(signature[:], ed25519.Sign(a.private, message))
		_, err = keeper.BindLayerX(h.db.Ctx(), caller, a.public, signature)
		require.NoError(t, err)
	}
	payload := append([]byte{0x71, 1, 0, 2}, a.actor[:]...)
	payload = append(payload, a.public[:]...)
	escrowTestExecute(t, h, a, 0x00070001, payload, sha256.Sum256(a.did))
	return a
}

func escrowTestHostKey(t *testing.T, h *payPrecompileHarness) sdk.StoreKey {
	t.Helper()
	for _, key := range h.db.Ctx().MultiStore().StoreKeys() {
		if key.Name() == "lxphost" {
			return key
		}
	}
	t.Fatal("lxphost store is not mounted")
	return nil
}

func escrowTestMain(t *testing.T, h *payPrecompileHarness, a escrowTestActor, asset [32]byte) [32]byte {
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
	h.db.Ctx().KVStore(escrowTestHostKey(t, h)).Set(append([]byte{0x06}, id[:]...), row)
	return id
}

func newEscrowTestFixture(t *testing.T) *escrowTestFixture {
	t.Helper()
	h := newPayPrecompileHarness(t)
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{EscrowEnabled: true})
	f := &escrowTestFixture{h: h}
	f.owner = escrowTestRegister(t, h, [32]byte{0x28, 0x01}, h.caller, false)
	f.beneficiary = escrowTestRegister(t, h, [32]byte{0x28, 0x02}, common.HexToAddress("0xa2282"), true)
	f.arbiter = escrowTestRegister(t, h, [32]byte{0x28, 0x03}, common.HexToAddress("0xa2283"), true)
	salt := [32]byte{0x28, 0x04}
	f.asset = sha256.Sum256(append(append([]byte("LX:ASSET:v1"), f.owner.actor[:]...), salt[:]...))
	f.owner.account = escrowTestMain(t, h, f.owner, f.asset)
	f.beneficiary.account = escrowTestMain(t, h, f.beneficiary, f.asset)
	f.arbiter.account = escrowTestMain(t, h, f.arbiter, f.asset)
	assetName, assetAccount, err := lxphost.AssetAccountID(f.owner.public, f.asset)
	require.NoError(t, err)
	payload := append([]byte{0, 1}, f.asset[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 2000)
	payload = append(payload, 1, 0)
	escrowTestExecute(t, h, f.owner, 0x00010001, payload, [32]byte{0x11})
	escrowTestExecute(t, h, f.owner, 0x00010004, append([]byte{0, 1}, f.asset[:]...), [32]byte{0x12})
	payload = append(append([]byte{0, 1}, f.asset[:]...), assetAccount[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	escrowTestExecute(t, h, f.owner, 0x0001000a, payload, [32]byte{0x13})
	amount := (codec.U128{Lo: 1000}).Bytes()
	key := [32]byte{0x14}
	material := append(append(append(append([]byte(nil), assetAccount[:]...), f.owner.account[:]...), f.asset[:]...), amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(bytes.Clone(material), key[:]...))
	require.NoError(t, err)
	account, err := h.keeper.Host().ReadAccount(h.db.Ctx(), assetName, assetAccount)
	require.NoError(t, err)
	transfer := binary.BigEndian.AppendUint64(material, account.NextSequence)
	transfer = append(transfer, key[:]...)
	transfer = binary.BigEndian.AppendUint64(transfer, uint64(h.db.Ctx().BlockTime().UnixMilli())+10000)
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
	payload = append([]byte{0x53, 1, 0, 10}, transfer...)
	payload = append(payload, 1)
	payload = append(payload, assetAccount[:]...)
	payload = append(payload, f.owner.public[:]...)
	payload = append(payload, ed25519.Sign(f.owner.private, digest[:])...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	escrowTestExecute(t, h, f.owner, 0x00010005, payload, key)
	return f
}

func (f *escrowTestFixture) openArgs(t *testing.T, id [32]byte) []interface{} {
	t.Helper()
	account, err := codec.DeriveAccountID([]byte("agent:" + string(f.owner.did) + ":escrow:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	return []interface{}{id, f.owner.account, account, f.beneficiary.account, f.arbiter.account, f.asset, big.NewInt(100), uint64(f.h.db.Ctx().BlockTime().UnixMilli()) + 1000, uint64(f.h.db.Ctx().BlockTime().UnixMilli()) + 10000, [32]byte{0x16}, [32]byte{0x17}}
}

func escrowTestWrite(t *testing.T, f *escrowTestFixture, caller common.Address, name, event string, ordinal uint16, state uint8, captured, locked int64, args ...interface{}) {
	t.Helper()
	h := f.h
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(t, err)
	before := len(h.db.GetAllLogs())
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	callOrdinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64()
	beforeState := h.keeper.ExportGenesis(h.db.Ctx())
	output, _, err := h.evm.StaticCall(caller, h.precompile.Address(), input, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	assert.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
	assert.Equal(t, beforeState, h.keeper.ExportGenesis(h.db.Ctx()))
	assert.Len(t, h.db.GetAllLogs(), before)
	assert.Equal(t, callOrdinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64())
	var disputeReceipt lxphost.Receipt
	var disputeRecord paytypes.EscrowRecord
	if name == "escrowDisputeOpen" {
		var selector [4]byte
		copy(selector[:], a.Methods[name].ID)
		key := paytypes.DeriveIdempotencyKey([32]byte(h.db.Ctx().EVMTxHash()), uint32(callOrdinal), selector)
		branch, _ := h.db.Ctx().CacheContext()
		disputeReceipt, err = h.keeper.EscrowDisputeOpen(branch, caller, args[0].([32]byte), key)
		require.NoError(t, err)
		var found bool
		disputeRecord, found = h.keeper.GetEscrow(branch, args[0].([32]byte))
		require.True(t, found)
		assert.Equal(t, beforeState, h.keeper.ExportGenesis(h.db.Ctx()))
	}
	output, _, err = h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%x", output)
	require.Empty(t, output)
	assert.Equal(t, callOrdinal+1, h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64())
	id := args[0].([32]byte)
	record, found := h.keeper.GetEscrow(h.db.Ctx(), id)
	require.True(t, found)
	assert.Equal(t, state, record.State)
	assert.Zero(t, big.NewInt(captured).Cmp(payAmount(record.CapturedAmount)))
	assert.Zero(t, big.NewInt(locked).Cmp(payAmount(record.LockedAmount)))
	if name == "escrowDisputeOpen" {
		assert.Equal(t, disputeRecord, record)
		require.Zero(t, disputeReceipt.ResultCode)
		decoded, err := codec.DecodeReceipt(disputeReceipt.Encoded)
		require.NoError(t, err)
		require.Len(t, decoded.Effects, 1)
		effect := decoded.Effects[0]
		assert.Equal(t, paytypes.EscrowModuleID, effect.ModuleID)
		assert.Equal(t, ordinal, effect.EventType)
		require.Len(t, effect.Body, 67)
		assert.Equal(t, id[:], effect.Body[:32])
		assert.Equal(t, ordinal, binary.BigEndian.Uint16(effect.Body[32:34]))
		assert.Equal(t, state, effect.Body[34])
		assert.Zero(t, big.NewInt(captured).Cmp(new(big.Int).SetBytes(effect.Body[35:51])))
		assert.Zero(t, big.NewInt(locked).Cmp(new(big.Int).SetBytes(effect.Body[51:67])))
	}
	logs := h.db.GetAllLogs()
	if assert.Len(t, logs, before+1) {
		log := logs[before]
		assert.Equal(t, h.precompile.Address(), log.Address)
		assert.Equal(t, []common.Hash{a.Events[event].ID, common.Hash(id)}, log.Topics)
		values, err := a.Events[event].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Len(t, values, 4)
		assert.Equal(t, ordinal, values[0])
		assert.Equal(t, state, values[1])
		assert.Zero(t, big.NewInt(captured).Cmp(values[2].(*big.Int)))
		assert.Zero(t, big.NewInt(locked).Cmp(values[3].(*big.Int)))
	}
}

func TestPrecompileEscrowLifecycle(t *testing.T) {
	for _, scenario := range []string{"capture", "release", "timeout", "dispute"} {
		t.Run(scenario, func(t *testing.T) {
			f := newEscrowTestFixture(t)
			id := [32]byte{0x20}
			args := f.openArgs(t, id)
			escrowTestWrite(t, f, f.owner.caller, "escrowOpen", "EscrowOpened", 1, 1, 0, 100, args...)
			switch scenario {
			case "capture":
				escrowTestWrite(t, f, f.beneficiary.caller, "escrowPartialCapture", "EscrowPartiallyCaptured", 3, 2, 25, 75, id, big.NewInt(25), [32]byte{0x21})
				escrowTestWrite(t, f, f.beneficiary.caller, "escrowCapture", "EscrowCaptured", 2, 3, 100, 0, id, big.NewInt(75), [32]byte{0x22})
			case "release":
				escrowTestWrite(t, f, f.owner.caller, "escrowRelease", "EscrowReleased", 4, 4, 0, 0, id, [32]byte{0x23})
			case "timeout":
				f.h.db.WithCtx(f.h.db.Ctx().WithBlockTime(time.UnixMilli(int64(args[7].(uint64)))))
				escrowTestWrite(t, f, f.beneficiary.caller, "escrowTimeout", "EscrowTimedOut", 5, 7, 0, 0, id, [32]byte{0x24})
			case "dispute":
				escrowTestWrite(t, f, f.beneficiary.caller, "escrowDisputeOpen", "EscrowDisputed", 6, 5, 0, 100, id)
				escrowTestWrite(t, f, f.arbiter.caller, "escrowDisputeResolve", "EscrowResolved", 7, 6, 33, 0, id, uint32(3333), [32]byte{0x25})
			}
			record, found := f.h.keeper.GetEscrow(f.h.db.Ctx(), id)
			require.True(t, found)
			for _, balance := range []struct {
				account [32]byte
				want    string
			}{
				{record.Owner, new(big.Int).Sub(big.NewInt(1000), payAmount(record.CapturedAmount)).String()},
				{record.Beneficiary, payAmount(record.CapturedAmount).String()},
				{record.EscrowAccount, "0"},
			} {
				got, err := f.h.keeper.Host().ReadBalance(f.h.db.Ctx(), balance.account, record.AssetID)
				require.NoError(t, err)
				assert.Equal(t, balance.want, got.String())
			}
		})
	}
}

func TestPrecompileEscrowViews(t *testing.T) {
	f := newEscrowTestFixture(t)
	h := f.h
	id, key := [32]byte{0x30}, [32]byte{0x31}
	a := h.precompile.GetABI()
	input, err := a.Pack("escrowOpen", f.openArgs(t, id)...)
	require.NoError(t, err)
	_, _, err = h.evm.Call(f.owner.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	input, err = a.Pack("escrowPartialCapture", id, big.NewInt(25), key)
	require.NoError(t, err)
	_, _, err = h.evm.Call(f.beneficiary.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logCount := len(h.db.GetAllLogs())
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	callOrdinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey)
	for _, name := range []string{"getEscrow", "escrowRemaining", "getEscrowResult"} {
		t.Run(name, func(t *testing.T) {
			query := id
			if name == "getEscrowResult" {
				query = key
			}
			input, err := a.Pack(name, query)
			require.NoError(t, err)
			output, _, err := h.evm.StaticCall(common.HexToAddress("0xdead"), h.precompile.Address(), input, 2000000)
			require.NoError(t, err, "%x", output)
			values, err := a.Methods[name].Outputs.Unpack(output)
			require.NoError(t, err)
			require.Len(t, values, 1)
			switch name {
			case "getEscrow":
				got := abi.ConvertType(values[0], new(escrowTuple)).(*escrowTuple)
				args := f.openArgs(t, id)
				assert.Equal(t, &escrowTuple{Owner: f.owner.account, EscrowAccount: args[2].([32]byte), Beneficiary: f.beneficiary.account,
					Arbiter: f.arbiter.account, AssetId: f.asset, LockedAmount: big.NewInt(75), CapturedAmount: big.NewInt(25), State: 2,
					ExpiryMs: args[7].(uint64), DisputeWindowMs: args[8].(uint64), TermsHash: [32]byte{0x16}, AgreementReference: [32]byte{0x17}}, got)
			case "escrowRemaining":
				assert.Equal(t, big.NewInt(75), values[0])
			case "getEscrowResult":
				raw := values[0].([]byte)
				result, err := paytypes.DecodeEscrowResult(raw)
				require.NoError(t, err)
				expected, found := h.keeper.GetEscrowResult(h.db.Ctx(), key)
				require.True(t, found)
				assert.Equal(t, expected, result)
				storageKey := paytypes.KernelKVKey(paytypes.EscrowModuleID, append([]byte("result:"), key[:]...))
				assert.Equal(t, h.db.Ctx().KVStore(escrowTestHostKey(t, h)).Get(storageKey), raw)
				assert.Equal(t, id, result.EscrowID)
				assert.Equal(t, uint16(3), result.Ordinal)
				assert.Equal(t, codec.U128{Lo: 25}, result.Amount)
			}
		})
		t.Run(name+"Missing", func(t *testing.T) {
			input, err := a.Pack(name, [32]byte{0xff})
			require.NoError(t, err)
			output, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(output)
			require.NoError(t, err)
			assert.Contains(t, reason, "LXP_UNKNOWN_FIELD")
		})
	}
	assert.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	assert.Len(t, h.db.GetAllLogs(), logCount)
	assert.Equal(t, callOrdinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey))
}

func TestPrecompileEscrowUnboundCallerReverts(t *testing.T) {
	h := newPayPrecompileHarness(t)
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{EscrowEnabled: true})
	a := h.precompile.GetABI()
	input, err := a.Pack("escrowOpen", [32]byte{1}, [32]byte{2}, [32]byte{3}, [32]byte{4}, [32]byte{5}, [32]byte{6}, big.NewInt(100), uint64(1700000001000), uint64(10000), [32]byte{7}, [32]byte{8})
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	callOrdinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey)
	output, _, err := h.evm.Call(common.HexToAddress("0xdead"), h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	assert.Contains(t, reason, "LXP_PAY_UNBOUND_CALLER")
	assert.Empty(t, h.db.GetAllLogs())
	assert.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	assert.Equal(t, callOrdinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey))
}
