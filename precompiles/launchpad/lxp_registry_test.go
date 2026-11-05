package launchpad_test

import (
	"strings"
	"testing"

	launchpadtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestLaunchpadCodesInLXPRegistry(t *testing.T) {
	errors := []*sdkerrors.Error{
		launchpadtypes.ErrInvalidParams,
		launchpadtypes.ErrOverflow,
		launchpadtypes.ErrDivisionByZero,
		launchpadtypes.ErrInsufficientInput,
		launchpadtypes.ErrInsufficientLiquidity,
		launchpadtypes.ErrVirtualFloorBreached,
		launchpadtypes.ErrSlippageExceeded,
		launchpadtypes.ErrDeadlineExpired,
		launchpadtypes.ErrZeroAddress,
		launchpadtypes.ErrZeroAmount,
		launchpadtypes.ErrUnknownMarket,
		launchpadtypes.ErrPaused,
		launchpadtypes.ErrNotPaused,
		launchpadtypes.ErrNotGuardian,
		launchpadtypes.ErrNotFeeRightsHolder,
		launchpadtypes.ErrWrongStrategy,
		launchpadtypes.ErrInvalidStrategy,
		launchpadtypes.ErrNoFeesAccumulated,
		launchpadtypes.ErrAirdropNotTriggered,
		launchpadtypes.ErrAlreadyClaimed,
		launchpadtypes.ErrInvalidMarket,
		launchpadtypes.ErrUnauthorized,
		launchpadtypes.ErrInvalidGenesis,
		launchpadtypes.ErrHoldingHistory,
		launchpadtypes.ErrUnsupportedHoldingEpoch,
		launchpadtypes.ErrInvalidAirdropEpoch,
		launchpadtypes.ErrLegacyAirdropEpoch,
		launchpadtypes.ErrAirdropBasis,
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
	require.Len(t, errors, 74)
	for i, err := range errors {
		t.Run(err.Error(), func(t *testing.T) {
			require.Equal(t, uint32(i+2), err.ABCICode())
			reason, unpackErr := abi.UnpackRevert(pcommon.LXPRevert(err.Error(), ""))
			require.NoError(t, unpackErr)
			require.False(t, strings.HasPrefix(reason, "LXP_UNKNOWN"), "reason: %s", reason)
			require.True(t, strings.HasPrefix(reason, err.Error()+":"), "reason: %s", reason)
		})
	}
}
