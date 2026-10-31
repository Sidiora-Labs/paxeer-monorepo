package layerxpay_test

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestPayBudgetPathMatchesHostExecute(t *testing.T) {
	f := newEscrowTestFixture(t)
	viaEVM, direct := f.h, newPayPrecompileHarness(t)
	for _, h := range []*payPrecompileHarness{viaEVM, direct} {
		h.keeper.SetParams(h.db.Ctx(), paytypes.Params{BudgetEnabled: true})
	}
	require.Equal(t, common.HexToAddress("0x101b"), viaEVM.precompile.Address())
	require.Equal(t, viaEVM.db.Ctx().BlockHeight(), direct.db.Ctx().BlockHeight())
	require.Equal(t, viaEVM.db.Ctx().BlockTime(), direct.db.Ctx().BlockTime())
	require.Equal(t, viaEVM.db.Ctx().EVMTxHash(), direct.db.Ctx().EVMTxHash())
	id := [32]byte{0xb1}
	createArgs := budgetTestArgs(t, f, id)
	account := createArgs[1].([32]byte)
	purpose := createArgs[3].([32]byte)
	start, expiry := createArgs[8].(uint64), createArgs[9].(uint64)
	capability := make([]byte, 195)
	capability[1] = 1
	copy(capability[2:34], f.beneficiary.actor[:])
	copy(capability[34:66], f.asset[:])
	copy(capability[66:98], f.beneficiary.account[:])
	copy(capability[98:130], purpose[:])
	maximum := (codec.U128{Lo: 30}).Bytes()
	copy(capability[130:146], maximum[:])
	copy(capability[146:162], maximum[:])
	binary.BigEndian.PutUint64(capability[178:186], expiry)
	capabilityKey := paytypes.KernelKVKey(paytypes.BudgetModuleID, append(append([]byte("budget-delegate:"), id[:]...), f.beneficiary.actor[:]...))
	viaEVM.db.Ctx().KVStore(escrowTestHostKey(t, viaEVM)).Set(capabilityKey, capability)

	hostRows := func(h *payPrecompileHarness) map[string][]byte {
		store := h.db.Ctx().KVStore(escrowTestHostKey(t, h))
		iterator := store.Iterator(nil, nil)
		defer iterator.Close()
		rows := make(map[string][]byte)
		for ; iterator.Valid(); iterator.Next() {
			rows[string(iterator.Key())] = bytes.Clone(iterator.Value())
		}
		return rows
	}
	seed := hostRows(viaEVM)
	require.NotEmpty(t, seed)
	directStore := direct.db.Ctx().KVStore(escrowTestHostKey(t, direct))
	for key, value := range seed {
		directStore.Set([]byte(key), bytes.Clone(value))
	}
	require.Equal(t, seed, hostRows(direct))

	want := paytypes.BudgetRecord{
		BudgetID: id, Owner: f.owner.account, BudgetAccount: account,
		AssetID: f.asset, PurposeHash: purpose,
		PerPeriodLimit: codec.U128{Lo: 300}, ConfiguredPeriodLimit: codec.U128{Lo: 300},
		CarryCap: codec.U128{Lo: 50}, PeriodLengthMs: 60000,
		PeriodStartMs: start, ExpiryMs: expiry, RolloverPolicy: 2, Delegates: [][32]byte{},
	}
	steps := []struct {
		name      string
		actor     escrowTestActor
		activity  uint32
		args      []interface{}
		payload   func([32]byte) ([]byte, error)
		spent     uint64
		balances  [3]string
		delegated bool
		closed    bool
	}{
		{
			name: "budgetCreate", actor: f.owner, activity: paytypes.ActivityBudgetCreate, args: createArgs,
			payload: func(_ [32]byte) ([]byte, error) {
				return paytypes.EncodeBudgetCreate(id, account, f.asset, purpose, codec.U128{Lo: 300}, codec.U128{Lo: 50}, codec.U128{Lo: 200}, 60000, start, expiry, 0, 2)
			},
			balances: [3]string{"800", "200", "0"},
		},
		{
			name: "budgetFund", actor: f.owner, activity: paytypes.ActivityBudgetFund,
			args: []interface{}{id, big.NewInt(50)},
			payload: func(_ [32]byte) ([]byte, error) {
				return paytypes.EncodeBudgetFund(id, codec.U128{Lo: 50})
			},
			balances: [3]string{"750", "250", "0"},
		},
		{
			name: "budgetSpend", actor: f.owner, activity: paytypes.ActivityBudgetSpend,
			args: []interface{}{id, f.beneficiary.account, big.NewInt(20)},
			payload: func(_ [32]byte) ([]byte, error) {
				return paytypes.EncodeBudgetSpend(id, f.beneficiary.account, codec.U128{Lo: 20})
			},
			spent: 20, balances: [3]string{"750", "230", "20"},
		},
		{
			name: "budgetDelegateAdd", actor: f.owner, activity: paytypes.ActivityBudgetDelegateAdd,
			args: []interface{}{id, f.beneficiary.actor},
			payload: func(_ [32]byte) ([]byte, error) {
				return paytypes.EncodeBudgetDelegate(id, f.beneficiary.actor)
			},
			spent: 20, balances: [3]string{"750", "230", "20"}, delegated: true,
		},
		{
			name: "budgetDelegateSpend", actor: f.beneficiary, activity: paytypes.ActivityBudgetDelegateSpend,
			args: []interface{}{id, f.beneficiary.account, big.NewInt(30)},
			payload: func(key [32]byte) ([]byte, error) {
				return paytypes.EncodeBudgetDelegateSpend(id, f.beneficiary.account, codec.U128{Lo: 30}, key)
			},
			spent: 50, balances: [3]string{"750", "200", "50"}, delegated: true,
		},
		{
			name: "budgetClose", actor: f.owner, activity: paytypes.ActivityBudgetClose,
			args: []interface{}{id, uint64(1)},
			payload: func(_ [32]byte) ([]byte, error) {
				return paytypes.EncodeBudgetClose(id, 1)
			},
			spent: 50, balances: [3]string{"950", "0", "50"}, delegated: true, closed: true,
		},
	}
	a := viaEVM.precompile.GetABI()
	ordinalSlot := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	recordKey := paytypes.KernelKVKey(paytypes.BudgetModuleID, append([]byte("budget:"), id[:]...))
	for ordinal, step := range steps {
		if !t.Run(step.name, func(t *testing.T) {
			require.Equal(t, uint64(ordinal), viaEVM.db.GetTransientState(viaEVM.precompile.Address(), ordinalSlot).Big().Uint64())
			var selector [4]byte
			copy(selector[:], a.Methods[step.name].ID)
			key := paytypes.DeriveIdempotencyKey([32]byte(direct.db.Ctx().EVMTxHash()), uint32(ordinal), selector)
			require.NotEqual(t, [32]byte{}, key)
			switch step.name {
			case "budgetCreate":
				key = id
			case "budgetSpend", "budgetDelegateAdd":
				key = [32]byte{}
			}
			payload, err := step.payload(key)
			require.NoError(t, err)
			if step.name == "budgetDelegateSpend" {
				require.Equal(t, key[:], payload[82:114])
			}
			before := hostRows(direct)
			input, err := a.Pack(step.name, step.args...)
			require.NoError(t, err)
			output, _, err := viaEVM.evm.Call(step.actor.caller, viaEVM.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.NoError(t, err, "%x", output)
			require.Empty(t, output)
			require.Equal(t, uint64(ordinal+1), viaEVM.db.GetTransientState(viaEVM.precompile.Address(), ordinalSlot).Big().Uint64())
			require.Equal(t, before, hostRows(direct))

			ctx, host := direct.db.Ctx(), direct.keeper.Host()
			last, err := host.ReadLastSequence(ctx, step.actor.did)
			require.NoError(t, err)
			sequence, err := host.ReadIdentitySequence(ctx, step.actor.did)
			require.NoError(t, err)
			now := uint64(ctx.BlockTime().UnixMilli())
			receipt, err := host.HostExecute(lxphost.WithSequence(ctx, last+1), lxphost.Activity{
				ProtocolVersion: 3, Type: step.activity, ActorDID: step.actor.did, AuthorityKey: step.actor.public,
				AccountSequence: sequence, NotBefore: now, NotAfter: now + host.TimestampWindowMs(),
				IdempotencyKey: key, Payload: payload,
			}, lxphost.KernelAuthority{
				Bound: true, Actor: step.actor.actor, Principal: step.actor.account, VerifiedKey: step.actor.public,
			})
			require.NoError(t, err)
			require.Zero(t, receipt.ResultCode)

			evmRows, directRows := hostRows(viaEVM), hostRows(direct)
			evmRecord, err := paytypes.DecodeBudgetRecord(evmRows[string(recordKey)])
			require.NoError(t, err)
			directRecord, err := paytypes.DecodeBudgetRecord(directRows[string(recordKey)])
			require.NoError(t, err)
			want.SpentThisPeriod = codec.U128{Lo: step.spent}
			if step.delegated {
				want.Delegates = [][32]byte{f.beneficiary.actor}
			}
			if step.closed {
				want.Closed, want.RevocationSequence = true, 1
			}
			require.Equal(t, want, evmRecord)
			require.Equal(t, evmRecord, directRecord)
			require.Equal(t, evmRows, directRows)
			consumed := codec.U128{}
			if step.name == "budgetDelegateSpend" || step.closed {
				consumed.Lo = 30
			}
			require.Equal(t, consumed.Bytes(), [16]byte(evmRows[string(capabilityKey)][162:178]))
			for _, h := range []*payPrecompileHarness{viaEVM, direct} {
				for i, account := range [][32]byte{f.owner.account, account, f.beneficiary.account} {
					budgetTestBalance(t, h, account, f.asset, step.balances[i])
				}
			}
		}) {
			t.FailNow()
		}
	}
}
