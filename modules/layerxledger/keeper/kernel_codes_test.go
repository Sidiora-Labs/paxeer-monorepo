package keeper

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func TestLedgerFromKernelCodes(t *testing.T) {
	header, err := os.ReadFile(filepath.Join("..", "..", "..", "include", "layerx", "lxp_result.h"))
	if err != nil {
		t.Fatalf("read kernel result table: %v", err)
	}
	entry := regexp.MustCompile(`X\((LXP_[A-Z0-9_]+), (-?[0-9]+)\)`)
	codes := make(map[string]lxphost.Code)
	for _, m := range entry.FindAllStringSubmatch(string(header), -1) {
		value, err := strconv.ParseInt(m[2], 10, 32)
		if err != nil {
			t.Fatalf("parse code of %s: %v", m[1], err)
		}
		codes[m[1]] = lxphost.Code(value)
	}
	if len(codes) == 0 {
		t.Fatal("kernel result table has no entries")
	}

	mappings := []struct {
		name string
		code lxphost.Code
		err  *sdkerrors.Error
	}{
		{"LXP_ERR_NON_CANONICAL", -3, types.ErrNonCanonical},
		{"LXP_ERR_MALFORMED_SEND", -107, types.ErrMalformedSend},
		{"LXP_ERR_UNAUTHORIZED_DEBIT", -207, types.ErrUnauthorizedDebit},
		{"LXP_ERR_CONTEXT_MISMATCH", -213, types.ErrContextMismatch},
		{"LXP_ERR_NO_PAYER_GRANT", -214, types.ErrNoPayerGrant},
		{"LXP_ERR_IDEMPOTENT_REPLAY", -302, types.ErrIdempotentReplay},
		{"LXP_ERR_EXPIRED", -303, types.ErrExpired},
		{"LXP_ERR_SEQUENCE_MISMATCH", -305, types.ErrSequenceMismatch},
		{"LXP_ERR_SEQUENCE_EXHAUSTED", -307, types.ErrSequenceExhausted},
		{"LXP_ERR_INSUFFICIENT_BALANCE", -400, types.ErrInsufficientBalance},
		{"LXP_ERR_ZERO_AMOUNT", -401, types.ErrZeroAmount},
		{"LXP_ERR_ASSET_MISMATCH", -402, types.ErrAssetMismatch},
		{"LXP_ERR_ASSET_PAUSED", -403, types.ErrAssetPaused},
		{"LXP_ERR_ASSET_ALREADY_REGISTERED", -411, types.ErrAssetAlreadyRegistered},
		{"LXP_ERR_PAUSED_SCOPE", -735, types.ErrPausedScope},
		{"LXP_ERR_ARENA_EXHAUSTED", -905, types.ErrArenaExhausted},
		{"LXP_FATAL_SUPPLY_MISMATCH", -1003, types.ErrFatalSupplyMismatch},
	}
	mapped := make(map[lxphost.Code]bool)
	for _, tc := range mappings {
		mapped[tc.code] = true
		t.Run(tc.name, func(t *testing.T) {
			if code, ok := codes[tc.name]; !ok || code != tc.code {
				t.Fatalf("kernel code for %s = %d (found %t), want %d", tc.name, code, ok, tc.code)
			}
			if got := FromKernel(tc.code); got != tc.err {
				t.Fatalf("FromKernel(%d) = %v, want registered error %v", tc.code, got, tc.err)
			}
			if got := tc.err.ABCICode(); got != uint32(-tc.code) {
				t.Errorf("registered code = %d, want %d", got, -tc.code)
			}
			if got := tc.err.Codespace(); got != "layerxledger" {
				t.Errorf("codespace = %q, want layerxledger", got)
			}
			wantText := strings.Replace(tc.name, "LXP_ERR_", "LXP_", 1)
			if got := tc.err.Error(); got != wantText {
				t.Errorf("error text = %q, want %q", got, wantText)
			}
		})
	}
	if got := FromKernel(0); got != nil {
		t.Errorf("FromKernel(0) = %v, want nil", got)
	}
	for name, code := range codes {
		if code == 0 || mapped[code] {
			continue
		}
		t.Run("passthrough/"+name, func(t *testing.T) {
			got := FromKernel(code)
			if got != code {
				t.Fatalf("FromKernel(%d) = %T(%v), want unchanged lxphost.Code", code, got, got)
			}
			if got.Error() != name {
				t.Errorf("passthrough text = %q, want %q", got.Error(), name)
			}
		})
	}
	for _, code := range []lxphost.Code{1, 3, 107, 735, 1003, 2147483647, -2147483648, -9999} {
		if got := FromKernel(code); got != code {
			t.Errorf("FromKernel(%d) = %T(%v), want unchanged lxphost.Code", code, got, got)
		}
	}
}
