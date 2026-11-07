package layerxprograms_test

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"math/rand"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func conservationTotals(t *testing.T, h *harness) map[[32]byte]sdk.Int {
	t.Helper()
	ctx := h.stateDB.Ctx()
	totals := make(map[[32]byte]sdk.Int)
	accounts := make(map[[32]byte]string)
	func() {
		iterator := ctx.KVStore(h.key).Iterator([]byte{6}, []byte{7})
		defer iterator.Close()
		for ; iterator.Valid(); iterator.Next() {
			require.Len(t, iterator.Key(), 33)
			row := iterator.Value()
			require.GreaterOrEqual(t, len(row), 40)
			require.Equal(t, uint32(32), binary.BigEndian.Uint32(row[:4]))
			nameLength := int(binary.BigEndian.Uint32(row[36:40]))
			require.GreaterOrEqual(t, len(row), 40+nameLength)
			id := [32]byte(iterator.Key()[1:])
			require.Equal(t, id[:], row[4:36])
			accounts[id] = string(row[40 : 40+nameLength])
		}
	}()
	for id, name := range accounts {
		account, err := h.host.ReadAccount(ctx, name, id)
		require.NoError(t, err)
		require.False(t, account.Balance.IsNegative())
		if account.HasAsset {
			total, exists := totals[account.AssetID]
			if !exists {
				total = sdk.ZeroInt()
			}
			totals[account.AssetID] = total.Add(account.Balance)
		}
	}
	return totals
}

func conservationCall(t *testing.T, h *harness, method string, args ...interface{}) []byte {
	t.Helper()
	require.Equal(t, common.HexToAddress("0x101d"), h.precompile.Address())
	input, err := lp.ProgramsABI.Pack(method, args...)
	require.NoError(t, err)
	output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 20000000, uint256.NewInt(0))
	require.NoError(t, err, "%s reverted: %x", method, output)
	_, err = h.stateDB.Finalize()
	require.NoError(t, err)
	h.stateDB.CleanupForTracer()
	return output
}

func conservationBalance(t *testing.T, h *harness, account, asset [32]byte) sdk.Int {
	t.Helper()
	row := h.stateDB.Ctx().KVStore(h.key).Get(append([]byte{6}, account[:]...))
	require.GreaterOrEqual(t, len(row), 40)
	require.Equal(t, uint32(32), binary.BigEndian.Uint32(row[:4]))
	require.Equal(t, account[:], row[4:36])
	nameLength := int(binary.BigEndian.Uint32(row[36:40]))
	require.GreaterOrEqual(t, len(row), 40+nameLength)
	record, err := h.host.ReadAccount(h.stateDB.Ctx(), string(row[40:40+nameLength]), account)
	require.NoError(t, err)
	require.True(t, record.HasAsset)
	require.Equal(t, asset, record.AssetID)
	return record.Balance
}

func conservationTransferSequence(t *testing.T, h *harness) {
	t.Helper()
	ctx := h.stateDB.Ctx()
	_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	name, id, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	store := ctx.KVStore(h.key)
	key := append([]byte{6}, id[:]...)
	row := append([]byte(nil), store.Get(key)...)
	offset := 4 + 32 + 4 + len(name) + 1 + 16 + 4 + 32 + 1
	require.GreaterOrEqual(t, len(row), offset+8)
	binary.BigEndian.PutUint64(row[offset:offset+8], sequence)
	store.Set(key, row)
}

func conservationProgramAccount(t *testing.T, h *harness, program, asset [32]byte, seed string) [32]byte {
	t.Helper()
	require.Empty(t, conservationCall(t, h, "registerAccount", program, uint8(1), asset, []byte(seed)))
	account, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), program, asset, []byte(seed))
	require.NoError(t, err)
	require.Equal(t, program, account.ProgramID)
	require.Equal(t, asset, account.AssetID)
	require.True(t, conservationBalance(t, h, account.AccountID, asset).IsZero())
	return account.AccountID
}

func conservationDeployProgram(t *testing.T, h *harness, program [32]byte) [32]byte {
	t.Helper()
	authority, _, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	codeHash := sha256.Sum256(wasm)
	require.Empty(t, conservationCall(t, h, "deploy", program, uint16(2), uint8(1), authority.Principal, codeHash, []byte{}, wasm))
	return codeHash
}

func TestProgramsCallsConserveFeeAsset(t *testing.T) {
	t.Run("deploy call and transfer", func(t *testing.T) {
		h, args, _ := programsCallFixture(t)
		fee, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(fee), 97)
		asset := [32]byte(fee[65:97])
		_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
		require.NoError(t, err)
		treasury, err := codec.DeriveAccountID([]byte("system:fees"))
		require.NoError(t, err)
		before := conservationTotals(t, h)
		require.True(t, before[asset].IsPositive())

		program := sha256.Sum256([]byte(t.Name()))
		codeHash := conservationDeployProgram(t, h, program)
		record, err := h.host.ReadProgram(h.stateDB.Ctx(), program)
		require.NoError(t, err)
		require.True(t, record.Active)
		require.Equal(t, uint16(2), record.ABIVersion)
		require.Equal(t, codeHash, record.CodeHash)
		require.Equal(t, before, conservationTotals(t, h), "deploy must conserve every asset")

		first := conservationProgramAccount(t, h, program, asset, "first")
		second := conservationProgramAccount(t, h, program, asset, "second")
		conservationTransferSequence(t, h)
		before = conservationTotals(t, h)
		payerBefore := conservationBalance(t, h, payer, asset)
		legs := []lp.TransferLeg{
			{From: payer, Asset: asset, To: first, Amount: big.NewInt(30)},
			{From: payer, Asset: asset, To: second, Amount: big.NewInt(40)},
		}
		require.Empty(t, conservationCall(t, h, "transfer", program, legs))
		require.Equal(t, payerBefore.Sub(sdk.NewInt(70)), conservationBalance(t, h, payer, asset))
		require.Equal(t, sdk.NewInt(30), conservationBalance(t, h, first, asset))
		require.Equal(t, sdk.NewInt(40), conservationBalance(t, h, second, asset))
		require.Equal(t, before, conservationTotals(t, h), "transfer must conserve every asset")

		random := rand.New(rand.NewSource(13292))
		for i := 0; i < 10; i++ {
			params := make([]byte, 1+random.Intn(64))
			_, err := random.Read(params)
			require.NoError(t, err)
			args[3] = params
			payerBefore = conservationBalance(t, h, payer, asset)
			treasuryBefore := conservationBalance(t, h, treasury, asset)
			output := conservationCall(t, h, "call", args...)
			values, err := lp.ProgramsABI.Unpack("call", output)
			require.NoError(t, err)
			require.Len(t, values, 3)
			require.Equal(t, int32(0), values[0])
			require.Equal(t, uint8(1), values[1])
			charged := payerBefore.Sub(conservationBalance(t, h, payer, asset))
			require.True(t, charged.IsPositive(), "call must exercise a nonzero fee")
			require.Equal(t, charged, conservationBalance(t, h, treasury, asset).Sub(treasuryBefore))
			require.Equal(t, before, conservationTotals(t, h), "call fee debit must equal the fee account credit")
		}
	})

	t.Run("sandboxFund", func(t *testing.T) {
		h := newHarness(t)
		f := newSandboxFundFixture(t, h)
		authority, _, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
		require.NoError(t, err)
		fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
		for _, rate := range f.fees {
			fee = binary.BigEndian.AppendUint64(fee, rate)
		}
		fee = append(fee, f.asset[:]...)
		for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
			fee = binary.BigEndian.AppendUint64(fee, value)
		}
		fee = append(fee, authority.Actor[:]...)
		fee = append(fee, make([]byte, 16)...)
		store := h.stateDB.Ctx().KVStore(h.key)
		store.Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), fee)
		store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)...), fee)
		before := conservationTotals(t, h)
		require.True(t, before[f.asset].IsPositive())
		assertSandboxFundBalances(t, h, f, 10, 0)
		input := f.input(t, f.request)
		output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 20000000, uint256.NewInt(0))
		require.NoError(t, err, "sandboxFund reverted: %x", output)
		require.Empty(t, output)
		assertSandboxFundBalances(t, h, f, 5, 5)
		require.Equal(t, before, conservationTotals(t, h), "sandboxFund must conserve the fee asset across caller and escrow")
		state, err := h.host.ReadSandbox(h.stateDB.Ctx(), f.lease, 0)
		require.NoError(t, err)
		require.Greater(t, len(state), f.stateOffset)
		require.Equal(t, byte(1), state[f.stateOffset])
	})
}

func TestProgramsTransferConservesProgramValue(t *testing.T) {
	for _, scenario := range []string{"multiple assets", "source overdraft", "aggregate overdraft", "program source refused"} {
		t.Run(scenario, func(t *testing.T) {
			h, _, _ := programsCallFixture(t)
			program := sha256.Sum256([]byte(t.Name()))
			conservationDeployProgram(t, h, program)
			fee, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(fee), 97)
			assets := [][32]byte{[32]byte(fee[65:97]), registerAccountAsset(t, h, 0x73)}
			authority, did, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
			require.NoError(t, err)
			execute := func(activityType uint32, payload []byte) {
				t.Helper()
				ctx := h.stateDB.Ctx()
				sequence, err := h.host.ReadIdentitySequence(ctx, did)
				require.NoError(t, err)
				last, err := h.host.ReadLastSequence(ctx, did)
				require.NoError(t, err)
				receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, last+1), h.host, authority, activityType, payload, sequence)
				require.NoError(t, err)
				require.Zero(t, receipt.ResultCode)
			}
			execute(0x00010004, append([]byte{0, 1}, assets[1][:]...))
			_, secondPayer, err := lxphost.AssetAccountID(h.publicKey, assets[1])
			require.NoError(t, err)
			mint := append([]byte{0, 1}, assets[1][:]...)
			mint = append(mint, secondPayer[:]...)
			mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 1000)
			execute(0x0001000a, mint)

			var funding, legs []lp.TransferLeg
			for i, asset := range assets {
				seed := string(rune('a' + i))
				from := conservationProgramAccount(t, h, program, asset, seed+"-source")
				to := conservationProgramAccount(t, h, program, asset, seed+"-destination")
				_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
				require.NoError(t, err)
				funding = append(funding, lp.TransferLeg{From: payer, Asset: asset, To: from, Amount: big.NewInt(100)})
				legs = append(legs, lp.TransferLeg{From: payer, Asset: asset, To: to, Amount: big.NewInt(int64(30 + i*10))})
			}
			conservationTransferSequence(t, h)
			before := conservationTotals(t, h)
			require.Empty(t, conservationCall(t, h, "transfer", program, funding))
			require.Equal(t, before, conservationTotals(t, h))
			sourceBalances := make(map[[32]byte]sdk.Int)
			for i, leg := range legs {
				require.Equal(t, sdk.NewInt(100), conservationBalance(t, h, funding[i].To, leg.Asset))
				require.True(t, conservationBalance(t, h, leg.To, leg.Asset).IsZero())
				sourceBalances[leg.From] = conservationBalance(t, h, leg.From, leg.Asset)
				require.True(t, sourceBalances[leg.From].GT(sdk.NewIntFromBigInt(leg.Amount)))
			}
			supply := make(map[[32]byte]sdk.Int)
			for _, asset := range assets {
				supply[asset], err = h.host.ReadTotalUnits(h.stateDB.Ctx(), asset)
				require.NoError(t, err)
			}
			before = conservationTotals(t, h)
			stateBefore := h.dump(h.stateDB.Ctx())
			logsBefore := h.stateDB.GetAllLogs()
			refusal := lxphost.Code(-400)
			if scenario == "source overdraft" {
				legs[1].Amount = sourceBalances[legs[1].From].Add(sdk.OneInt()).BigInt()
			} else if scenario == "aggregate overdraft" {
				legs = []lp.TransferLeg{legs[0], legs[0]}
				balance := sourceBalances[legs[0].From]
				first := balance.QuoRaw(2)
				second := balance.Sub(first).Add(sdk.OneInt())
				legs[0].Amount = first.BigInt()
				legs[1].Amount = second.BigInt()
				legs[1].To = funding[0].To
				require.True(t, first.LTE(balance))
				require.True(t, second.LTE(balance))
				require.True(t, first.Add(second).GT(balance))
			} else if scenario == "program source refused" {
				legs[1].From = funding[1].To
				require.True(t, conservationBalance(t, h, legs[1].From, legs[1].Asset).GT(sdk.NewIntFromBigInt(legs[1].Amount)))
				refusal = lxphost.Code(-204)
			}
			input, err := lp.ProgramsABI.Pack("transfer", program, legs)
			require.NoError(t, err)
			output, _, callErr := h.evm.Call(h.caller, h.precompile.Address(), input, 20000000, uint256.NewInt(0))
			if scenario == "multiple assets" {
				require.NoError(t, callErr, "program value transfer reverted: %x", output)
				require.Empty(t, output)
				for _, leg := range legs {
					amount := sdk.NewIntFromBigInt(leg.Amount)
					require.Equal(t, sourceBalances[leg.From].Sub(amount), conservationBalance(t, h, leg.From, leg.Asset))
					require.Equal(t, amount, conservationBalance(t, h, leg.To, leg.Asset))
				}
			} else {
				require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
				reason, err := abi.UnpackRevert(output)
				require.NoError(t, err)
				require.Equal(t, lp.KernelError(refusal, "transfer").Error(), reason)
				require.ErrorIs(t, h.stateDB.GetPrecompileError(), refusal)
				require.Equal(t, stateBefore, h.dump(h.stateDB.Ctx()), "refused transfer must roll back every leg")
				require.Equal(t, logsBefore, h.stateDB.GetAllLogs())
			}
			require.Equal(t, before, conservationTotals(t, h), "each asset must be conserved independently")
			for _, asset := range assets {
				actual, err := h.host.ReadTotalUnits(h.stateDB.Ctx(), asset)
				require.NoError(t, err)
				require.Equal(t, supply[asset], actual)
			}
		})
	}
}
