package layerxprograms_test

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func registerAccountAsset(t *testing.T, h *harness, saltByte byte) [32]byte {
	t.Helper()
	ctx := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	salt := [32]byte{saltByte}
	material := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	assetID := sha256.Sum256(append(material, salt[:]...))
	payload := append([]byte{0, 1}, assetID[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	payload = append(payload, 1, 0)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	lastSequence, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, lastSequence+1), h.host, authority, 0x00010001, payload, sequence)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	asset, err := h.host.ReadAsset(ctx, assetID)
	require.NoError(t, err)
	require.Equal(t, assetID, asset.AssetID)

	return assetID
}

func registerAccountFixture(t *testing.T) (*harness, [32]byte, [32]byte) {
	t.Helper()
	h := newHarness(t)
	assetID := registerAccountAsset(t, h, 0x71)
	ctx := h.stateDB.Ctx()
	authority, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	seedDeployMetering(t, h, authority.Principal)
	programID := sha256.Sum256([]byte(t.Name()))
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	payload, err := lp.EncodeDeployPayload(programID, 2, 1, authority.Principal, sha256.Sum256(wasm), nil, wasm)
	require.NoError(t, err)
	method := lp.ProgramsABI.Methods["deploy"]
	receipt, err := lp.Submit(ctx, h.executor, lp.NewCallContext(h.caller, &method), 0x00090001, payload, [16]byte{})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	program, err := h.host.ReadProgram(ctx, programID)
	require.NoError(t, err)
	require.True(t, program.Active)
	require.Equal(t, uint16(2), program.ABIVersion)
	return h, programID, assetID
}

func TestProgramsRegisterAccountProfile1(t *testing.T) {
	for _, profile := range []uint8{1, 2} {
		t.Run(fmt.Sprintf("LXPA%d", profile), func(t *testing.T) {
			h, programID, assetID := registerAccountFixture(t)
			for _, seed := range [][]byte{{}, []byte("vault\x00reserve"), make([]byte, 128)} {
				logsBefore := len(h.stateDB.GetAllLogs())
				input, err := lp.ProgramsABI.Pack("registerAccount", programID, profile, assetID, seed)
				require.NoError(t, err)
				before := h.dump(h.stateDB.Ctx())
				data, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, err := abi.UnpackRevert(data)
				require.NoError(t, err)
				require.Equal(t, lp.ErrReadOnlyWrite.Error(), reason)
				require.Equal(t, before, h.dump(h.stateDB.Ctx()))
				require.Len(t, h.stateDB.GetAllLogs(), logsBefore)
				data, _, err = h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
				require.NoError(t, err, "registerAccount must be served by 0x101D: %x", data)
				require.Empty(t, data)
				account, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), programID, assetID, seed)
				require.NoError(t, err)
				material := append([]byte("LayerX/programs/program-account/v1\x00"), programID[:]...)
				material = binary.BigEndian.AppendUint32(material, uint32(len(seed)))
				require.Equal(t, sha256.Sum256(append(material, seed...)), account.AccountID)
				require.Equal(t, programID, account.ProgramID)
				require.Equal(t, assetID, account.AssetID)
				require.Equal(t, seed, account.Seed)
				require.Positive(t, account.RegisteredSequence)
				byID, err := h.host.ReadProgramAccountByID(h.stateDB.Ctx(), account.AccountID)
				require.NoError(t, err)
				require.Equal(t, account, byID)
				logs := h.stateDB.GetAllLogs()
				require.NotEmpty(t, logs)
				require.Len(t, logs, logsBefore+1)
				log := logs[len(logs)-1]
				definition := lp.ProgramsABI.Events["ProgramAccountRegistered"]
				require.Equal(t, h.precompile.Address(), log.Address)
				require.Equal(t, []common.Hash{definition.ID, common.Hash(programID)}, log.Topics)
				values, err := definition.Inputs.NonIndexed().Unpack(log.Data)
				require.NoError(t, err)
				require.Equal(t, []interface{}{account.AccountID, assetID, sha256.Sum256(seed), uint16(len(seed)), account.RegisteredSequence}, values)
				input, err = lp.ProgramsABI.Pack("getProgramAccount", programID, assetID, seed)
				require.NoError(t, err)
				data, _, err = h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
				require.NoError(t, err)
				values, err = lp.ProgramsABI.Methods["getProgramAccount"].Outputs.Unpack(data)
				require.NoError(t, err)
				require.Equal(t, []interface{}{account.AccountID, assetID, account.RegisteredSequence}, values)
			}
			if profile == 2 {
				actual, err := h.host.ReadAccountProfile(h.stateDB.Ctx(), programID)
				require.NoError(t, err)
				require.Equal(t, uint8(2), actual.Profile)
			}
		})
	}
}

func TestProgramsRegisterAccountDuplicateIsIdempotent(t *testing.T) {
	for _, profile := range []uint8{1, 2} {
		t.Run(fmt.Sprintf("LXPA%d", profile), func(t *testing.T) {
			h, programID, assetID := registerAccountFixture(t)
			otherAssetID := registerAccountAsset(t, h, 0x72)
			require.NotEqual(t, assetID, otherAssetID)
			seed := []byte("duplicate")
			input, err := lp.ProgramsABI.Pack("registerAccount", programID, profile, assetID, seed)
			require.NoError(t, err)
			data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.NoError(t, err, "initial registration must succeed: %x", data)
			require.Empty(t, data)
			account, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), programID, assetID, seed)
			require.NoError(t, err)
			logsBefore := h.stateDB.GetAllLogs()
			readInput, err := lp.ProgramsABI.Pack("getProgramAccount", programID, assetID, seed)
			require.NoError(t, err)
			readBefore, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), readInput, 2000000)
			require.NoError(t, err)
			values, err := lp.ProgramsABI.Methods["getProgramAccount"].Outputs.Unpack(readBefore)
			require.NoError(t, err)
			require.Equal(t, []interface{}{account.AccountID, assetID, account.RegisteredSequence}, values)

			data, _, err = h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.NoError(t, err, "identical registration must succeed: %x", data)
			require.Empty(t, data)
			after, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), programID, assetID, seed)
			require.NoError(t, err)
			require.Equal(t, account, after)
			readAfter, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), readInput, 2000000)
			require.NoError(t, err)
			require.Equal(t, readBefore, readAfter)
			require.Equal(t, logsBefore, h.stateDB.GetAllLogs())

			before := h.dump(h.stateDB.Ctx())
			input, err = lp.ProgramsABI.Pack("registerAccount", programID, profile, otherAssetID, seed)
			require.NoError(t, err)
			data, _, err = h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-402))
			reason, err := abi.UnpackRevert(data)
			require.NoError(t, err)
			require.Equal(t, lp.KernelError(lxphost.Code(-402), "registerAccount").Error(), reason)
			require.Contains(t, reason, "LXP_ASSET_MISMATCH")
			require.Equal(t, before, h.dump(h.stateDB.Ctx()))
			require.Equal(t, logsBefore, h.stateDB.GetAllLogs())
			after, err = h.host.ReadProgramAccount(h.stateDB.Ctx(), programID, assetID, seed)
			require.NoError(t, err)
			require.Equal(t, account, after)
			readAfter, _, err = h.evm.StaticCall(h.caller, h.precompile.Address(), readInput, 2000000)
			require.NoError(t, err)
			require.Equal(t, readBefore, readAfter)
		})
	}
}
