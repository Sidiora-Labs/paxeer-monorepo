package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProgramsSimulateMatchesCallForEveryFixtureEntrypoint(t *testing.T) {
	h, args, _ := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	fee, err := h.host.ReadFeeSchedule(ctx)
	require.NoError(t, err)
	asset := [32]byte(fee[65:97])
	destination, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	wasm := deployCounterWasm()
	require.Equal(t, bytes.Repeat([]byte{9}, 32), wasm[len(wasm)-64:len(wasm)-32])
	require.Equal(t, bytes.Repeat([]byte{10}, 32), wasm[len(wasm)-32:])
	copy(wasm[len(wasm)-64:], asset[:])
	copy(wasm[len(wasm)-32:], destination[:])
	codeHash := sha256.Sum256(wasm)
	iface := deployInterface(codeHash)
	capabilityOffset := len(iface) - 85
	require.Equal(t, byte(6), iface[capabilityOffset])
	copy(iface[capabilityOffset+1:], asset[:])
	copy(iface[capabilityOffset+33:], destination[:])
	program := sha256.Sum256([]byte(t.Name()))
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	payload, err := lp.EncodeDeployPayload(program, 1, 1, authority.Principal, codeHash, iface, wasm)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	last, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, last+1), h.host, authority, 0x00090001, payload, sequence)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)

	entrypoints := []string{"layerx_call"}
	interfaceOffset := len("LayerX/program-interface/v1\x00") + 32 + 2
	require.Equal(t, uint16(len(entrypoints)), binary.BigEndian.Uint16(iface[interfaceOffset:]))
	interfaceOffset += 2
	for _, entrypoint := range entrypoints {
		length := int(binary.BigEndian.Uint16(iface[interfaceOffset:]))
		require.Equal(t, entrypoint, string(iface[interfaceOffset+2:interfaceOffset+2+length]))
	}
	capabilities := append([]byte{0, 1, 5}, asset[:]...)
	capabilities = append(capabilities, destination[:]...)
	capabilities = append(capabilities, make([]byte, 15)...)
	capabilities = append(capabilities, 1)
	args[0], args[4] = program, capabilities
	for _, entrypoint := range entrypoints {
		t.Run(entrypoint, func(t *testing.T) {
			args[2] = entrypoint
			for _, calldata := range [][]byte{nil, {0, 0xff}, bytes.Repeat([]byte{0xa5}, 64)} {
				args[3] = calldata
				simulateInput, err := lp.ProgramsABI.Pack("simulateCall", args...)
				require.NoError(t, err)
				callInput, err := lp.ProgramsABI.Pack("call", args...)
				require.NoError(t, err)
				before := h.dump(h.stateDB.Ctx())
				height := h.stateDB.Ctx().BlockHeight()
				simulated, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), simulateInput, 20000000)
				require.NoError(t, err, "simulateCall(%s, %x): %x", entrypoint, calldata, simulated)
				require.Equal(t, before, h.dump(h.stateDB.Ctx()))
				called, _, err := h.evm.Call(h.caller, h.precompile.Address(), callInput, 20000000, uint256.NewInt(0))
				require.NoError(t, err, "call(%s, %x): %x", entrypoint, calldata, called)
				require.Equal(t, height, h.stateDB.Ctx().BlockHeight())
				simulation, err := lp.ProgramsABI.Methods["simulateCall"].Outputs.Unpack(simulated)
				require.NoError(t, err)
				actual, err := lp.ProgramsABI.Methods["call"].Outputs.Unpack(called)
				require.NoError(t, err)
				require.Len(t, simulation, 3)
				require.Len(t, actual, 3)
				require.Zero(t, actual[0])
				require.Equal(t, uint8(1), actual[1])
				require.Equal(t, simulation[0], actual[0], "status")
				require.Equal(t, simulation[1], actual[1], "kind")
				require.Equal(t, simulation[2], actual[2], "response")
				require.Equal(t, simulated, called)
				require.NotEqual(t, before, h.dump(h.stateDB.Ctx()))
			}
		})
	}
}
