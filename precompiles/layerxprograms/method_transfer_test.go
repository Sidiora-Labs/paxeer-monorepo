package layerxprograms_test

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func programsTransferAccount(t *testing.T, h *harness, key, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := lxphost.MainAccountID(key)
	require.NoError(t, err)
	row := binary.BigEndian.AppendUint32(nil, 32)
	row = append(row, id[:]...)
	row = binary.BigEndian.AppendUint32(row, uint32(len(name)))
	row = append(row, name...)
	row = append(row, 1)
	row = append(row, make([]byte, 16)...)
	row = binary.BigEndian.AppendUint32(row, 32)
	row = append(row, asset[:]...)
	row = append(row, 1)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = append(row, 0, 0)
	row = binary.BigEndian.AppendUint32(row, 32)
	row = append(row, key[:]...)
	row = append(row, 1)
	h.stateDB.Ctx().KVStore(h.key).Set(append([]byte{0x06}, id[:]...), row)
	account, err := h.host.ReadAccount(h.stateDB.Ctx(), name, id)
	require.NoError(t, err)
	require.Equal(t, id, account.ID)
	require.Equal(t, key, account.AuthorityKey)
	require.Equal(t, sdk.ZeroInt(), account.Balance)
	return id
}

func newProgramsTransferHarness(t *testing.T) (*harness, [32]byte, []lp.TransferLeg) {
	t.Helper()
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	salt := [32]byte{0x61}
	material := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	execute := func(activityType uint32, payload []byte) {
		t.Helper()
		sequence, err := h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		last, err := h.host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		now := uint64(ctx.BlockTime().UnixMilli())
		key := sha256.Sum256(binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint32(nil, activityType), last+1))
		receipt, err := h.host.Execute(lxphost.WithSequence(ctx, last+1), lxphost.Activity{
			ProtocolVersion: 3, Type: activityType, ActorDID: did, AuthorityKey: h.publicKey,
			AccountSequence: sequence, NotBefore: now, NotAfter: now, IdempotencyKey: key, Payload: payload,
		}, authority)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode, "kernel setup activity %#x: %v", activityType, receipt.ResultCode)
	}
	registration := append([]byte{0, 1}, asset[:]...)
	registration = append(registration, salt[:]...)
	registration = append(registration, 1, 'T', 1, 'T', 6)
	registration = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(registration, 0), 1000)
	registration = append(registration, 1, 0)
	execute(0x00010001, registration)
	from := programsTransferAccount(t, h, h.publicKey, asset)
	to := programsTransferAccount(t, h, [32]byte{0x62}, asset)
	other := programsTransferAccount(t, h, [32]byte{0x63}, asset)
	execute(0x00010004, append([]byte{0, 1}, asset[:]...))
	assetName, assetAccount, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	mint := append([]byte{0, 1}, asset[:]...)
	mint = append(mint, assetAccount[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 1000)
	execute(0x0001000a, mint)
	funded, err := h.host.ReadAccount(ctx, assetName, assetAccount)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(1000), funded.Balance)
	last, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	key := sha256.Sum256(binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint32(nil, 0x00010005), last+1))
	material = append(append(append([]byte(nil), assetAccount[:]...), from[:]...), asset[:]...)
	material = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(material, 0), 100)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	send, err := ledgertypes.EncodeSendPayload(ledgertypes.SendArgs{
		From: assetAccount, To: from, Asset: asset, Amount: sdk.NewInt(100), Sequence: funded.NextSequence,
		IdempotencyKey: key, ExpiresAt: uint64(ctx.BlockTime().UnixMilli()) + 1000,
		ContextHash: contextHash, PublicKey: h.publicKey, NetworkID: 7, ProtocolVersion: 3,
	})
	require.NoError(t, err)
	execute(0x00010005, send)
	balance, err := h.host.ReadBalance(ctx, from, asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(100), balance)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	name, id, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	require.Equal(t, from, id)
	owner, err := lxphost.HolderDid([]byte(name))
	require.NoError(t, err)
	require.Equal(t, authority.Principal, owner)
	store := ctx.KVStore(h.key)
	accountKey := append([]byte{0x06}, from[:]...)
	row := append([]byte(nil), store.Get(accountKey)...)
	sequenceOffset := 4 + 32 + 4 + len(name) + 1 + 16 + 4 + 32 + 1
	binary.BigEndian.PutUint64(row[sequenceOffset:sequenceOffset+8], sequence)
	store.Set(accountKey, row)
	source, err := h.host.ReadAccount(ctx, name, from)
	require.NoError(t, err)
	require.Equal(t, sequence, source.NextSequence)
	require.Equal(t, sdk.NewInt(100), source.Balance)
	return h, [32]byte{0x64}, []lp.TransferLeg{
		{From: from, Asset: asset, To: to, Amount: big.NewInt(30)},
		{From: from, Asset: asset, To: other, Amount: big.NewInt(40)},
	}
}

func assertProgramsTransferBalances(t *testing.T, h *harness, legs []lp.TransferLeg, source, first, second int64) {
	t.Helper()
	for _, balance := range []struct {
		account [32]byte
		amount  int64
	}{{legs[0].From, source}, {legs[0].To, first}, {legs[1].To, second}} {
		amount, err := h.host.ReadBalance(h.stateDB.Ctx(), balance.account, legs[0].Asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(balance.amount), amount)
	}
	total, err := h.host.ReadTotalUnits(h.stateDB.Ctx(), legs[0].Asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(1000), total)
}

func TestProgramsTransferMovesLegs(t *testing.T) {
	h, program, legs := newProgramsTransferHarness(t)
	assertProgramsTransferBalances(t, h, legs, 100, 0, 0)
	ctx := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	beforeSequence, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	input, err := lp.ProgramsABI.Pack("transfer", program, legs)
	require.NoError(t, err)
	var selector [4]byte
	copy(selector[:], input[:4])
	idempotency := lp.IdempotencyKey([32]byte(ctx.EVMTxHash()), 0, selector)
	payload, err := lp.EncodeTransferPayload(program, legs)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	reference, _ := ctx.CacheContext()
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	receipt, err := h.host.Execute(lxphost.WithSequence(reference, beforeSequence+1), lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00090005, ActorDID: did, AuthorityKey: authority.VerifiedKey,
		AccountSequence: sequence, NotBefore: timestamp, NotAfter: timestamp,
		IdempotencyKey: idempotency, Payload: payload,
	}, authority)
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "transfer revert data: %x", data)
	require.Empty(t, data)
	assertProgramsTransferBalances(t, h, legs, 30, 30, 40)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	require.Zero(t, receipt.ResultCode)
	require.Equal(t, beforeSequence+1, receipt.GlobalSequence)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(9), decoded.ModuleID)
	require.Len(t, receipt.Events, 1)
	require.Equal(t, uint16(5), receipt.Events[0].Code)
	require.Len(t, receipt.Events[0].Body, 32)
	root := [32]byte(receipt.Events[0].Body)
	require.NotEqual(t, [32]byte{}, root)
	require.Len(t, decoded.Effects, 1)
	var events []codec.Effect
	for _, effect := range decoded.Effects {
		require.NotEqual(t, uint8(2), effect.Kind)
		if effect.Kind == 3 {
			events = append(events, effect)
		}
	}
	require.Len(t, events, 1)
	require.Equal(t, uint16(5), events[0].EventType)
	require.Equal(t, root[:], events[0].Body)
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, lp.Address, logs[0].Address)
	require.Equal(t, []common.Hash{lp.ProgramsABI.Events["ProgramTransferred"].ID, common.Hash(program)}, logs[0].Topics)
	require.Equal(t, root[:], logs[0].Data)
}

func TestProgramsTransferOverdraftReverts(t *testing.T) {
	h, program, legs := newProgramsTransferHarness(t)
	assertProgramsTransferBalances(t, h, legs, 100, 0, 0)
	legs[1].Amount = big.NewInt(71)
	before := h.dump(h.stateDB.Ctx())
	input, err := lp.ProgramsABI.Pack("transfer", program, legs)
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, lp.KernelError(lxphost.Code(-400), "transfer").Error(), reason)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-400))
	assertProgramsTransferBalances(t, h, legs, 100, 0, 0)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	require.Empty(t, h.stateDB.GetAllLogs())
}
