package common

import (
	"fmt"
	"strings"
	"testing"

	launchpadtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func lxpAssertMarketRow(t *testing.T, code string, registered *sdkerrors.Error) {
	t.Helper()
	entry, ok := lxpRevertEntryByCode(code)
	require.True(t, ok, "missing registry row for %s", code)
	require.Same(t, registered, entry.goErr, code)
	require.False(t, entry.kernel, code)
	require.NotEmpty(t, entry.message, code)

	reason := lxpDecodeRevertForTest(t, LXPRevert(code, ""))
	require.False(t, strings.HasPrefix(reason, "LXP_UNKNOWN"), reason)
	require.Equal(t, code+": "+entry.message, reason)

	for _, err := range []error{registered, fmt.Errorf("market operation: %w", registered)} {
		got, found := lxpCodeOf(err)
		require.True(t, found, "missing Go error lookup for %s", code)
		require.Equal(t, code, got)
	}
	wrapped := fmt.Errorf("market operation: %w", registered)
	require.Equal(t, code+": "+wrapped.Error(), lxpDecodeRevertForTest(t, LXPReason(wrapped)))
}

func TestLXPPerpsRowsCoverRegisteredErrors(t *testing.T) {
	registered := perpstypes.RegisteredErrors()
	require.Len(t, registered, 44)
	for i, err := range registered {
		t.Run(err.Error(), func(t *testing.T) {
			require.Equal(t, uint32(i+2), err.ABCICode())
			require.Equal(t, perpstypes.ModuleName, err.Codespace())
			require.True(t, strings.HasPrefix(err.Error(), "LXP_PERPS_"))
			lxpAssertMarketRow(t, err.Error(), err)
		})
	}
}

func TestLXPPricefeedRowsCoverRegisteredErrors(t *testing.T) {
	registered := pricefeedtypes.LXPCodes()
	require.Len(t, registered, 15)
	for i, row := range registered {
		t.Run(row.Name, func(t *testing.T) {
			require.Equal(t, uint32(i+2), row.Err.ABCICode())
			require.Equal(t, pricefeedtypes.ModuleName, row.Err.Codespace())
			require.True(t, strings.HasPrefix(row.Name, "LXP_PRICEFEED_"))
			lxpAssertMarketRow(t, row.Name, row.Err)
		})
	}
}

func TestLXPLaunchpadRowsCoverRegisteredErrors(t *testing.T) {
	registered := []*sdkerrors.Error{
		launchpadtypes.ErrInvalidParams,
		launchpadtypes.ErrInvalidGenesis,
		launchpadtypes.ErrInvalidRange,
		launchpadtypes.ErrAlreadyInitialized,
		launchpadtypes.ErrAlreadyMinted,
		launchpadtypes.ErrApproveFailed,
		launchpadtypes.ErrDuplicatePool,
		launchpadtypes.ErrDuplicateToken,
		launchpadtypes.ErrFeeOutOfRange,
		launchpadtypes.ErrIndexOutOfBounds,
		launchpadtypes.ErrInsufficientBalance,
		launchpadtypes.ErrInsufficientCreationFee,
		launchpadtypes.ErrInsufficientSurplus,
		launchpadtypes.ErrInvalidBalance,
		launchpadtypes.ErrInvalidConfiguration,
		launchpadtypes.ErrInvalidHookResponse,
		launchpadtypes.ErrInvalidImplementation,
		launchpadtypes.ErrInvalidRoleAccount,
		launchpadtypes.ErrLastDefaultAdmin,
		launchpadtypes.ErrMissingRole,
		launchpadtypes.ErrMulticallFailed,
		launchpadtypes.ErrNonERC721Receiver,
		launchpadtypes.ErrNonPayableUpgrade,
		launchpadtypes.ErrNotApproved,
		launchpadtypes.ErrNotInitializing,
		launchpadtypes.ErrNotTokenOwner,
		launchpadtypes.ErrOpticalQuoteUnsupported,
		launchpadtypes.ErrPoolIndexMigrationRequired,
		launchpadtypes.ErrReentrancyGuardReentrantCall,
		launchpadtypes.ErrSameToken,
		launchpadtypes.ErrTokenNotFound,
		launchpadtypes.ErrTransferFailed,
		launchpadtypes.ErrTransferFromFailed,
		launchpadtypes.ErrUUPSNotThroughActiveProxy,
		launchpadtypes.ErrUUPSNotThroughProxy,
		launchpadtypes.ErrUnauthorizedUpgrade,
		launchpadtypes.ErrUnexecutableQuote,
		launchpadtypes.ErrUnsupportedSnapshotToken,
		launchpadtypes.ErrUpgradeInitializationFailed,
		launchpadtypes.ErrInvalidToken,
		launchpadtypes.ErrReentrantCall,
		launchpadtypes.ErrAccessControlUnauthorizedAccount,
		launchpadtypes.ErrRetired,
		launchpadtypes.ErrFutureLookup,
		launchpadtypes.ErrInsufficientAllowance,
		launchpadtypes.ErrInvalidPermit,
		launchpadtypes.ErrInvalidPoolBinding,
		launchpadtypes.ErrPermitExpired,
	}
	require.Len(t, registered, 48)
	for i, err := range registered {
		t.Run(err.Error(), func(t *testing.T) {
			expectedCode := uint32(i + 28)
			switch i {
			case 0:
				expectedCode = 2
			case 1:
				expectedCode = 24
			}
			require.Equal(t, expectedCode, err.ABCICode())
			require.Equal(t, launchpadtypes.ModuleName, err.Codespace())
			require.True(t, strings.HasPrefix(err.Error(), "LXP_LAUNCHPAD_"))
			lxpAssertMarketRow(t, err.Error(), err)
		})
	}
}
