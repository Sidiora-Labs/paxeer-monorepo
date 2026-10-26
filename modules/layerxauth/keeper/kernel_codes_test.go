package keeper

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func TestAuthFromKernelCodes(t *testing.T) {
	mappings := []struct {
		code lxphost.Code
		err  *sdkerrors.Error
	}{
		{-204, types.ErrAuthScope},
		{-203, types.ErrAuthRevoked},
		{-202, types.ErrAuthExpired},
		{-222, types.ErrAuthNotMature},
		{-304, types.ErrNotYetValid},
		{-301, types.ErrSequenceReused},
		{-3, types.ErrNonCanonical},
		{-207, types.ErrUnauthorizedDebit},
		{-302, types.ErrIdempotentReplay},
	}
	for _, tc := range mappings {
		t.Run(tc.err.Error(), func(t *testing.T) {
			if got := FromKernel(tc.code); got != tc.err {
				t.Fatalf("FromKernel(%d) = %v, want registered error %v", tc.code, got, tc.err)
			}
		})
	}
	if got := FromKernel(0); got != nil {
		t.Errorf("FromKernel(0) = %v, want nil", got)
	}
	for _, code := range []lxphost.Code{-735, -107, -303, -400, -1003, -1, -9999, 1, 3, 204, 735, 2147483647, -2147483648} {
		got := FromKernel(code)
		if got != code {
			t.Fatalf("FromKernel(%d) = %T(%v), want unchanged lxphost.Code", code, got, got)
		}
		if got.Error() != code.Error() {
			t.Errorf("FromKernel(%d) text = %q, want %q", code, got.Error(), code.Error())
		}
		if code == -735 && got.Error() != "LXP_ERR_PAUSED_SCOPE" {
			t.Errorf("FromKernel(-735) text = %q, want LXP_ERR_PAUSED_SCOPE", got.Error())
		}
	}
}
