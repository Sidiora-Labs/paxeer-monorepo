package perps_test

import (
	"encoding/json"
	"errors"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func repair16RoundTrip(t *testing.T, produced error) abci.ResponseDeliverTx {
	t.Helper()
	response := sdkerrors.ResponseDeliverTx(produced, 100, 75, false)
	wire, err := response.Marshal()
	require.NoError(t, err)
	var received abci.ResponseDeliverTx
	require.NoError(t, received.Unmarshal(wire))
	require.Equal(t, response, received)
	return received
}

func repair16InvalidGenesis(t *testing.T, produced error, detail string) {
	t.Helper()
	require.ErrorIs(t, produced, types.ErrInvalidGenesis)
	require.True(t, types.ErrInvalidGenesis.Is(produced))
	require.Contains(t, produced.Error(), "failed to unmarshal perps genesis state")
	require.Contains(t, produced.Error(), detail)
	received := repair16RoundTrip(t, produced)
	require.Equal(t, "perps", received.Codespace)
	require.Equal(t, uint32(39), received.Code)
	require.Equal(t, produced.Error(), received.Log)
	require.Contains(t, received.Log, "LXP_PERPS_INVALID_GENESIS")
	require.Equal(t, int64(100), received.GasWanted)
	require.Equal(t, int64(75), received.GasUsed)
	decoded := sdkerrors.ABCIError(received.Codespace, received.Code, received.Log)
	require.ErrorIs(t, decoded, types.ErrInvalidGenesis)
	require.Contains(t, decoded.Error(), detail)
	require.Contains(t, decoded.Error(), "failed to unmarshal perps genesis state")
	for _, err := range []error{produced, decoded} {
		reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
		require.NoError(t, unpackErr)
		require.Equal(t, "LXP_PERPS_INVALID_GENESIS: "+err.Error(), reason)
	}
	require.NotErrorIs(t, sdkerrors.ABCIError("sdk", received.Code, received.Log), types.ErrInvalidGenesis)
	require.NotErrorIs(t, sdkerrors.ABCIError(received.Codespace, 1, received.Log), types.ErrInvalidGenesis)
}

func TestRepair_16_ModuleGenesisABCI(t *testing.T) {
	am := perps.NewAppModuleBasic()
	for _, tc := range []struct {
		name, input, detail string
		typeError           bool
	}{
		{name: "truncated", input: `{"params":`, detail: "unexpected end of JSON input"},
		{name: "malformed", input: `{"params":!}`, detail: "invalid character '!'"},
		{name: "wrong params type", input: `{"params":[]}`, detail: "cannot unmarshal array", typeError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			produced := am.ValidateGenesis(nil, nil, json.RawMessage(tc.input))
			repair16InvalidGenesis(t, produced, tc.detail)
			if tc.typeError {
				var decodeErr *json.UnmarshalTypeError
				require.ErrorAs(t, produced, &decodeErr)
				require.Equal(t, "array", decodeErr.Value)
			} else {
				var decodeErr *json.SyntaxError
				require.ErrorAs(t, produced, &decodeErr)
				require.Positive(t, decodeErr.Offset)
			}
		})
	}
}

func TestRepair_16_StreamGenesisABCI(t *testing.T) {
	am := perps.NewAppModuleBasic()
	genesisCh := make(chan json.RawMessage, 2)
	genesisCh <- am.DefaultGenesis(nil)
	genesisCh <- json.RawMessage(`{"params":`)
	close(genesisCh)
	produced := am.ValidateGenesisStream(nil, nil, genesisCh)
	repair16InvalidGenesis(t, produced, "unexpected end of JSON input")
	var decodeErr *json.SyntaxError
	require.ErrorAs(t, produced, &decodeErr)
}

func TestRepair_16_InternalErrorDoesNotImpersonateGenesis(t *testing.T) {
	// Even the registered description in an internal error's log cannot
	// substitute for the producer's registered codespace and numeric code.
	produced := errors.New("internal decoder failure: LXP_PERPS_INVALID_GENESIS")
	received := repair16RoundTrip(t, produced)
	require.Equal(t, sdkerrors.UndefinedCodespace, received.Codespace)
	require.Equal(t, uint32(1), received.Code)
	require.NotErrorIs(t, produced, types.ErrInvalidGenesis)
	decoded := sdkerrors.ABCIError(received.Codespace, received.Code, received.Log)
	require.NotErrorIs(t, decoded, types.ErrInvalidGenesis)
	reason, err := abi.UnpackRevert(pcommon.LXPReason(produced))
	require.NoError(t, err)
	require.Equal(t, "LXP_UNKNOWN: unknown; "+produced.Error(), reason)
}

func TestRepair_16_ValidGenesis(t *testing.T) {
	am := perps.NewAppModuleBasic()
	genesis := am.DefaultGenesis(nil)
	require.NoError(t, am.ValidateGenesis(nil, nil, genesis))
	genesisCh := make(chan json.RawMessage, 1)
	genesisCh <- genesis
	close(genesisCh)
	require.NoError(t, am.ValidateGenesisStream(nil, nil, genesisCh))
	received := repair16RoundTrip(t, nil)
	require.Zero(t, received.Code)
	require.Empty(t, received.Codespace)
	require.Empty(t, received.Log)
}

func TestRepair_16_InvalidGenesisABISelector(t *testing.T) {
	parsed, err := perpsabi.ABI()
	require.NoError(t, err)
	custom, ok := parsed.Errors["PerpsInvalidGenesis"]
	require.True(t, ok)
	require.Empty(t, custom.Inputs)
	require.Equal(t, "d06690f2", common.Bytes2Hex(custom.ID[:4]))
	decoded, err := parsed.ErrorByID([4]byte{0xd0, 0x66, 0x90, 0xf2})
	require.NoError(t, err)
	require.Equal(t, "PerpsInvalidGenesis", decoded.Name)
}
