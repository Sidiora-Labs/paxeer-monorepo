package layerxprograms_test

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"

	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProgramsValueAccountMatchesBankBalance(t *testing.T) {
	h, program, funding := newProgramsTransferHarness(t)
	ctx := h.stateDB.Ctx()
	asset, source := funding[0].Asset, funding[0].From
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	seedDeployMetering(t, h, authority.Principal)
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	codeHash := sha256.Sum256(wasm)
	call := func(method string, args ...interface{}) {
		t.Helper()
		input, err := lp.ProgramsABI.Pack(method, args...)
		require.NoError(t, err)
		data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.NoError(t, err, "%s returned %x", method, data)
		require.Empty(t, data)
	}
	call("deploy", program, uint16(2), uint8(1), authority.Principal, codeHash, []byte(nil), wasm)

	accounts := make([][32]byte, 3)
	seeds := []string{"treasury", "settlement", "paxai/rewards/v1"}
	for i, seed := range seeds {
		call("registerAccount", program, uint8(1), asset, []byte(seed))
		account, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), program, asset, []byte(seed))
		require.NoError(t, err)
		require.Equal(t, program, account.ProgramID)
		require.Equal(t, asset, account.AssetID)
		require.Equal(t, []byte(seed), account.Seed)
		material := append([]byte("LayerX/programs/program-account/v1\x00"), program[:]...)
		material = binary.BigEndian.AppendUint32(material, uint32(len(seed)))
		require.Equal(t, sha256.Sum256(append(material, seed...)), account.AccountID)
		accounts[i] = account.AccountID
	}

	sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	store := h.stateDB.Ctx().KVStore(h.key)
	accountKey := append([]byte{0x06}, source[:]...)
	row := append([]byte(nil), store.Get(accountKey)...)
	nameLength := int(binary.BigEndian.Uint32(row[36:40]))
	sequenceOffset := 4 + 32 + 4 + nameLength + 1 + 16 + 4 + 32 + 1
	binary.BigEndian.PutUint64(row[sequenceOffset:sequenceOffset+8], sequence)
	store.Set(accountKey, row)

	expected := [3]int64{}
	remaining := int64(100)
	assertParity := func() {
		t.Helper()
		before := h.dump(h.stateDB.Ctx())
		logsBefore := h.stateDB.GetAllLogs()
		for i, account := range accounts {
			balance, err := h.host.ReadBalance(h.stateDB.Ctx(), account, asset)
			require.NoError(t, err, "bank balance for %s", seeds[i])
			require.Equal(t, sdk.NewInt(expected[i]), balance, "bank balance for %s", seeds[i])
			input, err := lp.ProgramsABI.Pack("getValueAccount", program, account)
			require.NoError(t, err)
			data, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
			if err != nil {
				reason, decodeErr := abi.UnpackRevert(data)
				t.Logf("getValueAccount for %s (%x), asset %x: precompile error=%v, revert reason=%q, decode error=%v",
					seeds[i], account, asset, h.stateDB.GetPrecompileError(), reason, decodeErr)
			}
			require.NoError(t, err, "getValueAccount for %s returned %x", seeds[i], data)
			values, err := lp.ProgramsABI.Methods["getValueAccount"].Outputs.Unpack(data)
			require.NoError(t, err)
			require.Len(t, values, 2)
			require.Zero(t, balance.BigInt().Cmp(values[0].(*big.Int)), "EVM/bank parity for %s (%x), asset %x", seeds[i], account, asset)
			require.Equal(t, true, values[1], "registered value account %s (%x) must exist", seeds[i], account)
		}
		balance, err := h.host.ReadBalance(h.stateDB.Ctx(), source, asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(remaining), balance)
		require.Equal(t, int64(100), remaining+expected[0]+expected[1]+expected[2])
		total, err := h.host.ReadTotalUnits(h.stateDB.Ctx(), asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(1000), total)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
		require.Equal(t, logsBefore, h.stateDB.GetAllLogs())
	}
	assertParity()
	for _, transfer := range []struct {
		account int
		amount  int64
		reward  int64
	}{
		{0, 10, 5},
		{1, 20, 10},
		{0, 30, 25},
	} {
		legs := []lp.TransferLeg{
			{From: source, Asset: asset, To: accounts[transfer.account], Amount: big.NewInt(transfer.amount)},
			{From: source, Asset: asset, To: accounts[2], Amount: big.NewInt(transfer.reward)},
		}
		call("transfer", program, legs)
		expected[transfer.account] += transfer.amount
		expected[2] += transfer.reward
		remaining -= transfer.amount + transfer.reward
		assertParity()
	}
}
