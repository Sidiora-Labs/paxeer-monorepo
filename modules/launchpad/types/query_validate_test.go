package types_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestQueryPageBounds(t *testing.T) {
	require.Equal(t, 100, types.MaxQueryPageLimit)
	for limit := uint64(1); limit <= 100; limit++ {
		t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
			require.NoError(t, types.ValidatePage(0, limit))
			require.NoError(t, types.ValidatePage(42, limit))
			require.NoError(t, types.ValidatePage(math.MaxUint64-limit, limit))
			require.ErrorIs(t, types.ValidatePage(math.MaxUint64-limit+1, limit), types.ErrInvalidRange)
		})
	}
	for _, tc := range []struct {
		name   string
		offset uint64
		limit  uint64
	}{
		{"zero_limit", 0, 0},
		{"zero_limit_max_offset", math.MaxUint64, 0},
		{"above_limit", 0, 101},
		{"max_limit", 0, math.MaxUint64},
		{"max_offset", math.MaxUint64, 1},
		{"max_offset_full_page", math.MaxUint64, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, types.ValidatePage(tc.offset, tc.limit), types.ErrInvalidRange)
		})
	}
}

func TestQueryHexAddress(t *testing.T) {
	want := common.Address{
		0x85, 0xfc, 0xd1, 0x37, 0x35, 0xf4, 0x30, 0x98, 0x33, 0xa5,
		0x03, 0xee, 0x80, 0x4e, 0xa3, 0x23, 0x95, 0x85, 0x14, 0x79,
	}
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"lowercase", "0x85fcd13735f4309833a503ee804ea32395851479"},
		{"uppercase_digits", "0x85FCD13735F4309833A503EE804EA32395851479"},
		{"checksummed", "0x85FcD13735F4309833A503EE804ea32395851479"},
		{"mixed_case", "0x85fCd13735f4309833A503eE804Ea32395851479"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address, err := types.ParseHexAddress("pool", tc.value)
			require.NoError(t, err)
			require.Equal(t, want, address)
		})
	}
	for _, tc := range []struct {
		name  string
		value string
		err   *sdkerrors.Error
	}{
		{"empty", "", sdkerrors.ErrInvalidAddress},
		{"prefix_only", "0x", sdkerrors.ErrInvalidAddress},
		{"missing_prefix", strings.Repeat("a", 40), sdkerrors.ErrInvalidAddress},
		{"uppercase_prefix", "0X" + strings.Repeat("a", 40), sdkerrors.ErrInvalidAddress},
		{"short", "0x" + strings.Repeat("a", 39), sdkerrors.ErrInvalidAddress},
		{"long", "0x" + strings.Repeat("a", 41), sdkerrors.ErrInvalidAddress},
		{"non_hex", "0x" + strings.Repeat("a", 39) + "g", sdkerrors.ErrInvalidAddress},
		{"leading_space", " 0x" + strings.Repeat("a", 40), sdkerrors.ErrInvalidAddress},
		{"trailing_space", "0x" + strings.Repeat("a", 40) + " ", sdkerrors.ErrInvalidAddress},
		{"embedded_space", "0x" + strings.Repeat("a", 39) + " ", sdkerrors.ErrInvalidAddress},
		{"embedded_nul", "0x" + strings.Repeat("a", 39) + "\x00", sdkerrors.ErrInvalidAddress},
		{"short_zero", "0x0", sdkerrors.ErrInvalidAddress},
		{"unprefixed_zero", strings.Repeat("0", 40), sdkerrors.ErrInvalidAddress},
		{"zero", "0x" + strings.Repeat("0", 40), types.ErrZeroAddress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, field := range []string{"holder", "token"} {
				address, err := types.ParseHexAddress(field, tc.value)
				require.ErrorIs(t, err, tc.err)
				require.Contains(t, err.Error(), field)
				codespace, code, log := sdkerrors.ABCIInfo(err, false)
				require.Equal(t, tc.err.Codespace(), codespace)
				require.Equal(t, tc.err.ABCICode(), code)
				require.Contains(t, log, field)
				require.Equal(t, common.Address{}, address)
			}
		})
	}
}
