package layerxledger_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	proofcodec "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type precompileViewFixture struct {
	*harness
	private   ed25519.PrivateKey
	key       [32]byte
	did       []byte
	owner     [32]byte
	asset     [32]byte
	account   [32]byte
	recipient [32]byte
	grantID   [32]byte
	grant     []byte
	salt      [32]byte
	cap       sdk.Int
	minted    sdk.Int
	global    uint64
}

func (f *precompileViewFixture) execute(t *testing.T, private ed25519.PrivateKey, kind uint32, sequence uint64, payload []byte, idempotency [32]byte) {
	t.Helper()
	f.global++
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	did := []byte("did:layerx:" + hex.EncodeToString(key[:]))
	owner, err := lxphost.DIDID(did)
	require.NoError(t, err)
	if idempotency == [32]byte{} {
		idempotency = sha256.Sum256(binary.BigEndian.AppendUint64(nil, f.global))
	}
	now := uint64(f.ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: kind, ActorDID: did, AuthorityKey: key,
		AccountSequence: sequence, NotBefore: now - 1000, NotAfter: now + 1000,
		IdempotencyKey: idempotency, Payload: payload,
	}
	hash, err := f.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(private, hash[:])
	receipt, err := f.host.Execute(lxphost.WithSequence(f.ctx, f.global), activity,
		lxphost.KernelAuthority{Actor: owner, Principal: owner, VerifiedKey: key})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
}

func newPrecompileViewFixture(t *testing.T) *precompileViewFixture {
	t.Helper()
	f := &precompileViewFixture{harness: newHarness(t), salt: [32]byte{0x31}}
	var seed [ed25519.SeedSize]byte
	seed[0] = 1
	f.private = ed25519.NewKeyFromSeed(seed[:])
	copy(f.key[:], f.private.Public().(ed25519.PublicKey))
	f.did = []byte("did:layerx:" + hex.EncodeToString(f.key[:]))
	var err error
	f.owner, err = lxphost.DIDID(f.did)
	require.NoError(t, err)
	f.asset = sha256.Sum256(append(append([]byte("LX:ASSET:v1"), f.owner[:]...), f.salt[:]...))
	_, f.account, err = lxphost.AssetAccountID(f.key, f.asset)
	require.NoError(t, err)
	f.cap = sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 100)).AddRaw(1000)
	f.minted = sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 80)).AddRaw(250)
	registration, err := types.EncodeRegisterPayload(types.RegisterArgs{
		AssetID: f.asset, Salt: f.salt, Symbol: "TKN", Name: "Token", Decimals: 6,
		SupplyCap: f.cap, IssuerKind: 2, CustodyReference: []byte("reserve"),
	})
	require.NoError(t, err)
	f.execute(t, f.private, 0x00010001, 0, registration, [32]byte{})
	f.execute(t, f.private, 0x00010004, 1, types.EncodeOpenAccountPayload(f.asset), [32]byte{})
	mint, err := types.EncodeSupplyPayload(f.asset, f.account, f.minted)
	require.NoError(t, err)
	f.execute(t, f.private, 0x0001000a, 2, mint, [32]byte{})
	seed[0] = 2
	recipientPrivate := ed25519.NewKeyFromSeed(seed[:])
	var recipientKey [32]byte
	copy(recipientKey[:], recipientPrivate.Public().(ed25519.PublicKey))
	_, f.recipient, err = lxphost.AssetAccountID(recipientKey, f.asset)
	require.NoError(t, err)
	f.execute(t, recipientPrivate, 0x00010004, 0, types.EncodeOpenAccountPayload(f.asset), [32]byte{})

	burn, err := types.EncodeSupplyPayload(f.asset, f.account, sdk.NewInt(10))
	require.NoError(t, err)
	f.execute(t, f.private, 0x0001000b, 3, burn, [32]byte{})
	recipientMint, err := types.EncodeSupplyPayload(f.asset, f.recipient, sdk.NewInt(10))
	require.NoError(t, err)
	f.execute(t, f.private, 0x0001000a, 4, recipientMint, [32]byte{})

	body := append(append([]byte(nil), f.account[:]...), f.recipient[:]...)
	body = append(body, f.asset[:]...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 40)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 100)
	body = append(body, 0)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	body = append(body, bytes.Repeat([]byte{0x22}, 32)...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, f.key[:]...)
	f.grantID, err = proofcodec.DomainHash(proofcodec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	f.grant = append(append([]byte(nil), f.grantID[:]...), body...)
	f.grant = append(f.grant, ed25519.Sign(f.private, f.grantID[:])...)
	require.NoError(t, types.ValidateIssueGrantPayload(f.grant))
	f.execute(t, f.private, 0x00010007, 5, f.grant, [32]byte{})
	f.execute(t, f.private, 0x00010008, 6, types.EncodeRevokeGrantPayload(f.grantID, 6), [32]byte{})
	f.execute(t, f.private, 0x00010002, 7, types.EncodePausePayload(f.asset), [32]byte{})
	return f
}

type precompileViewCase struct {
	name string
	args []interface{}
	want []interface{}
}

func (f *precompileViewFixture) cases(t *testing.T) []precompileViewCase {
	t.Helper()
	asset, err := f.keeper.GetAsset(f.ctx, f.asset)
	require.NoError(t, err)
	require.Equal(t, f.asset, asset.AssetID)
	require.Equal(t, f.cap, asset.SupplyCap)
	require.Equal(t, f.minted, asset.TotalUnits)
	require.True(t, asset.Paused)
	balance, err := f.keeper.BalanceOf(f.ctx, f.account, f.asset)
	require.NoError(t, err)
	require.Equal(t, f.minted.SubRaw(10), balance)
	total, err := f.keeper.TotalUnits(f.ctx, f.asset)
	require.NoError(t, err)
	require.Equal(t, f.minted, total)
	owner, err := f.keeper.AccountOwner(f.ctx, f.account)
	require.NoError(t, err)
	require.Equal(t, f.owner, owner)
	grant, err := f.keeper.GetPayerGrant(f.ctx, f.grantID)
	require.NoError(t, err)
	require.Equal(t, f.grant, grant.Grant)
	require.True(t, grant.Revoked)
	require.NotZero(t, grant.RevokedAt)
	sequence, err := f.keeper.NextSequence(f.ctx, f.key, f.asset)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	return []precompileViewCase{
		{"getAsset", []interface{}{f.asset}, []interface{}{asset.AssetID, asset.Symbol, asset.Name, asset.Decimals,
			asset.SupplyCap.BigInt(), asset.IssuerDID, asset.Salt, asset.IssuerKind, asset.CustodyKind,
			asset.CustodyReference, asset.Paused, asset.TotalUnits.BigInt()}},
		{"balanceOf", []interface{}{f.account, f.asset}, []interface{}{balance.BigInt()}},
		{"totalUnits", []interface{}{f.asset}, []interface{}{total.BigInt()}},
		{"accountId", []interface{}{f.key, f.asset}, []interface{}{f.account}},
		{"accountOwner", []interface{}{f.account}, []interface{}{owner}},
		{"getPayerGrant", []interface{}{f.grantID}, []interface{}{grant.Grant,
			new(big.Int).SetBytes(grant.DrawnTotal.BigInt().FillBytes(make([]byte, 32))),
			new(big.Int).SetBytes(grant.DrawnThisPeriod.BigInt().FillBytes(make([]byte, 32))),
			grant.WindowStart, grant.RevokedAt, grant.Revoked, grant.InvoiceSettled}},
		{"nextSequence", []interface{}{f.key, f.asset}, []interface{}{sequence}},
	}
}

func (f *precompileViewFixture) input(t *testing.T, name string, args []interface{}) []byte {
	t.Helper()
	input, err := f.precompile.GetABI().Pack(name, args...)
	require.NoError(t, err)
	return input
}

func (f *precompileViewFixture) assertView(t *testing.T, caller common.Address, readOnly bool, view precompileViewCase) {
	t.Helper()
	input := f.input(t, view.name, view.args)
	hostBefore := storeSnapshot(t, f.ctx.KVStore(f.hostKey))
	ledgerBefore := storeSnapshot(t, f.ctx.KVStore(f.storeKey))
	eventsBefore := len(f.ctx.EventManager().Events())
	logsBefore := len(f.stateDB.GetAllLogs())
	data, err := f.precompile.Run(f.evm, caller, common.HexToAddress("0xdead"), input, nil, readOnly, false, nil)
	require.Equal(t, hostBefore, storeSnapshot(t, f.ctx.KVStore(f.hostKey)))
	require.Equal(t, ledgerBefore, storeSnapshot(t, f.ctx.KVStore(f.storeKey)))
	require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
	require.Len(t, f.stateDB.GetAllLogs(), logsBefore)
	require.NoError(t, err, "view %s: return data %x", view.name, data)
	method := f.precompile.GetABI().Methods[view.name]
	require.True(t, method.IsConstant())
	want, err := method.Outputs.Pack(view.want...)
	require.NoError(t, err)
	require.Equal(t, want, data)
	decoded, err := method.Outputs.Unpack(data)
	require.NoError(t, err)
	require.Equal(t, view.want, decoded)
}

func TestPrecompileLedgerViews(t *testing.T) {
	f := newPrecompileViewFixture(t)
	f.bindCaller(t)
	unbound := common.HexToAddress("0x2320000000000000000000000000000000000099")
	_, err := f.keeper.Actor(f.ctx, unbound)
	require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
	t.Run("unbound write", func(t *testing.T) {
		hostBefore := storeSnapshot(t, f.ctx.KVStore(f.hostKey))
		ledgerBefore := storeSnapshot(t, f.ctx.KVStore(f.storeKey))
		data, err := f.precompile.Run(f.evm, unbound, common.Address{}, f.input(t, "register", ledgerArguments("register")), nil, false, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_UNAUTHORIZED_DEBIT")
		require.Equal(t, hostBefore, storeSnapshot(t, f.ctx.KVStore(f.hostKey)))
		require.Equal(t, ledgerBefore, storeSnapshot(t, f.ctx.KVStore(f.storeKey)))
	})
	for _, caller := range []struct {
		name    string
		address common.Address
	}{{"bound", f.caller}, {"unbound", unbound}} {
		for _, readOnly := range []bool{false, true} {
			for _, view := range f.cases(t) {
				t.Run(fmt.Sprintf("%s/static=%t/%s", caller.name, readOnly, view.name), func(t *testing.T) {
					f.assertView(t, caller.address, readOnly, view)
				})
			}
		}
	}
	for _, missing := range []struct {
		name   string
		args   []interface{}
		reason string
	}{
		{"getAsset", []interface{}{[32]byte{}}, "LXP_ASSET_MISMATCH"},
		{"balanceOf", []interface{}{f.account, [32]byte{}}, "LXP_ASSET_MISMATCH"},
		{"totalUnits", []interface{}{[32]byte{}}, "LXP_ASSET_MISMATCH"},
		{"accountOwner", []interface{}{[32]byte{}}, "LXP_UNKNOWN_ACCOUNT_NAMESPACE"},
		{"getPayerGrant", []interface{}{[32]byte{}}, "LXP_NO_PAYER_GRANT"},
		{"nextSequence", []interface{}{[32]byte{}, f.asset}, "LXP_UNKNOWN_ACCOUNT_NAMESPACE"},
	} {
		t.Run("missing/"+missing.name, func(t *testing.T) {
			hostBefore := storeSnapshot(t, f.ctx.KVStore(f.hostKey))
			ledgerBefore := storeSnapshot(t, f.ctx.KVStore(f.storeKey))
			data, err := f.precompile.Run(f.evm, f.caller, common.Address{}, f.input(t, missing.name, missing.args), nil, true, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLedgerRevert(t, data, missing.reason)
			require.Equal(t, hostBefore, storeSnapshot(t, f.ctx.KVStore(f.hostKey)))
			require.Equal(t, ledgerBefore, storeSnapshot(t, f.ctx.KVStore(f.storeKey)))
		})
	}
	t.Run("malformed", func(t *testing.T) {
		method := f.precompile.GetABI().Methods["getAsset"]
		data, err := f.precompile.Run(f.evm, f.caller, common.Address{}, method.ID, nil, true, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(data)
		require.NoError(t, err)
		require.Contains(t, reason, "LXP_")
	})
}

func TestPrecompileViewsIgnorePause(t *testing.T) {
	f := newPrecompileViewFixture(t)
	f.bindCaller(t)
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	require.True(t, f.keeper.GetParams(f.ctx).Paused)
	unbound := common.HexToAddress("0x2320000000000000000000000000000000000099")
	_, err := f.keeper.Actor(f.ctx, unbound)
	require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
	for _, caller := range []struct {
		name    string
		address common.Address
	}{{"bound", f.caller}, {"unbound", unbound}} {
		for _, readOnly := range []bool{false, true} {
			for _, view := range f.cases(t) {
				t.Run(fmt.Sprintf("%s/static=%t/%s", caller.name, readOnly, view.name), func(t *testing.T) {
					f.assertView(t, caller.address, readOnly, view)
				})
			}
		}
	}
	t.Run("register", func(t *testing.T) {
		hostBefore := storeSnapshot(t, f.ctx.KVStore(f.hostKey))
		ledgerBefore := storeSnapshot(t, f.ctx.KVStore(f.storeKey))
		data, err := f.precompile.Run(f.evm, f.caller, common.Address{}, f.input(t, "register", ledgerArguments("register")), nil, false, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_PAUSED_SCOPE")
		require.Equal(t, hostBefore, storeSnapshot(t, f.ctx.KVStore(f.hostKey)))
		require.Equal(t, ledgerBefore, storeSnapshot(t, f.ctx.KVStore(f.storeKey)))
	})
}
