// Proves: lxphost.Host.ReadGrant and layerxauth/keeper.Keeper -> keeper.FromKernel, layerxauth.RevertError, common.Precompile.Run/LXPReason -> errors.Is/errors.As and abi.UnpackRevert.
//
// The consumer is this external Go package: the interface inventory has no
// downstream caller yet. The existing fixtures execute signed kernel activities
// against the real host and bind real EVM callers; the production precompile
// supplies every return value. Errors own no persistent state, so each observation
// uses a fresh EVM StateDB and actually invokes the producer again.
//
// Compatibility finding: precompiles/common/precompiles.go, LXPReason forwards
// bare LXP_ names unchanged, although the interface describes "LXP_<CODE>:
// <detail>". The proof asserts that current code-only behavior as well as the
// detailed static-call refusal; changing that shared transport is outside scope.
package layerxauth_test

import (
	"errors"
	"math"
	"testing"

	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestProofLayerxauthErrors(t *testing.T) {
	f := newPrecompileViewFixture(t)
	a := f.precompile.GetABI()

	// This helper only calls the real transport. In particular, it neither
	// supplies an executor nor manufactures an ABI return or revert payload.
	call := func(t *testing.T, method string, readOnly bool, args ...interface{}) ([]byte, error, error) {
		t.Helper()
		input, err := a.Pack(method, args...)
		require.NoError(t, err)
		evm, db := f.evm()
		before := hostSnapshot(t, db.Ctx(), f.hostKey)
		data, runErr := f.precompile.Run(evm, f.caller, f.precompile.Address(), input, nil, readOnly, false, nil)
		require.Equal(t, before, hostSnapshot(t, db.Ctx(), f.hostKey))
		require.Empty(t, db.GetAllLogs())
		return data, runErr, db.GetPrecompileError()
	}
	decodeRefusal := func(t *testing.T, data []byte, want string) {
		t.Helper()
		require.Greater(t, len(data), 4+32)
		require.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
		reason, err := abi.UnpackRevert(data)
		require.NoError(t, err)
		require.Equal(t, want, reason)

		// Truncate actual producer output at the string length word. The same
		// consumer decoder must refuse it, rather than accept a cached reason.
		reason, err = abi.UnpackRevert(data[:4+32])
		require.Error(t, err)
		require.Empty(t, reason)
	}

	t.Run("successful producer result twice", func(t *testing.T) {
		for observation := 0; observation < 2; observation++ {
			body, producerErr := f.keeper.GetGrant(f.ctx, f.sessionID)
			require.NoError(t, producerErr)
			require.Equal(t, f.sessionBody, body)
			require.Nil(t, layerxauth.RevertError(producerErr))
			require.Nil(t, authkeeper.FromKernel(f.sessionReceipt.ResultCode))

			data, runErr, observedErr := call(t, "getGrant", true, f.sessionID)
			require.NoError(t, runErr)
			require.NoError(t, observedErr)
			values, err := a.Methods["getGrant"].Outputs.Unpack(data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{body, false, uint64(0)}, values)
			_, err = a.Methods["getGrant"].Outputs.Unpack(data[:len(data)-32])
			require.Error(t, err, "consumer must refuse a truncated grant result")
		}
	})

	t.Run("kernel unknown identifier twice", func(t *testing.T) {
		missing := [32]byte{0xff}
		for observation := 0; observation < 2; observation++ {
			_, producerErr := f.host.ReadGrant(f.ctx, missing)
			require.Error(t, producerErr)
			var code lxphost.Code
			require.ErrorAs(t, producerErr, &code)
			require.Equal(t, lxphost.Code(-7), code)
			converted := layerxauth.RevertError(authkeeper.FromKernel(code))
			require.EqualError(t, converted, "LXP_UNKNOWN_FIELD")
			require.ErrorIs(t, converted, code)
			var recovered lxphost.Code
			require.ErrorAs(t, converted, &recovered)
			require.Equal(t, code, recovered)

			data, runErr, observedErr := call(t, "getGrant", true, missing)
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			require.ErrorIs(t, observedErr, code)
			require.ErrorAs(t, observedErr, &recovered)
			require.Equal(t, code, recovered)
			decodeRefusal(t, data, "LXP_UNKNOWN_FIELD")
		}
	})

	t.Run("registered malformed input twice", func(t *testing.T) {
		for observation := 0; observation < 2; observation++ {
			live, reason, producerErr := f.keeper.IsLive(f.ctx, f.sessionID, 43, math.MaxUint64)
			require.False(t, live)
			require.Empty(t, reason)
			require.ErrorIs(t, producerErr, types.ErrNonCanonical)
			wrapped := sdkerrors.Wrap(producerErr, "consumer context")
			converted := layerxauth.RevertError(wrapped)
			require.EqualError(t, converted, "LXP_NON_CANONICAL")
			require.Same(t, wrapped, errors.Unwrap(converted))
			require.Same(t, wrapped, converted.(interface{ Cause() error }).Cause())
			require.ErrorIs(t, converted, types.ErrNonCanonical)
			var registered *sdkerrors.Error
			require.ErrorAs(t, converted, &registered)
			require.Same(t, types.ErrNonCanonical, registered)

			data, runErr, observedErr := call(t, "isLive", true, f.sessionID, uint64(43), uint64(math.MaxUint64))
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			require.ErrorIs(t, observedErr, types.ErrNonCanonical)
			require.ErrorAs(t, observedErr, &registered)
			require.Equal(t, "layerxauth", registered.Codespace())
			require.Equal(t, uint32(8), registered.ABCICode())
			decodeRefusal(t, data, "LXP_NON_CANONICAL")
		}
	})

	t.Run("detailed precompile refusal twice", func(t *testing.T) {
		for observation := 0; observation < 2; observation++ {
			data, runErr, observedErr := call(t, "grantSession", true, f.sessionBody, uint64(100), [32]byte{0x44})
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			require.ErrorIs(t, observedErr, types.ErrAuthScope)
			decodeRefusal(t, data, "LXP_AUTH_SCOPE: cannot call a layerxauth state change from staticcall")
		}
	})

	t.Run("governance pause uses replacement codespace twice", func(t *testing.T) {
		params := f.keeper.GetParams(f.ctx)
		params.Paused = true
		require.NoError(t, f.keeper.SetParams(f.ctx, params))
		actor, err := f.keeper.Actor(f.ctx, f.caller)
		require.NoError(t, err)
		for observation := 0; observation < 2; observation++ {
			_, producerErr := f.keeper.GrantSession(f.ctx, actor, f.sessionBody, 100, [32]byte{0x44}, [32]byte{0x45})
			require.ErrorIs(t, producerErr, govtypes.ErrPausedScope)
			converted := layerxauth.RevertError(producerErr)
			require.EqualError(t, converted, "LXP_PAUSED_SCOPE")
			require.ErrorIs(t, converted, govtypes.ErrPausedScope)

			data, runErr, observedErr := call(t, "grantSession", false, f.sessionBody, uint64(100), [32]byte{0x44})
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			require.ErrorIs(t, observedErr, govtypes.ErrPausedScope)
			var registered *sdkerrors.Error
			require.ErrorAs(t, observedErr, &registered)
			require.Equal(t, govtypes.ModuleName, registered.Codespace())
			require.Equal(t, uint32(735), registered.ABCICode())
			decodeRefusal(t, data, "LXP_PAUSED_SCOPE")
		}
	})
}
