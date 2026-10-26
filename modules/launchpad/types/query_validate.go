package types

import (
	"fmt"
	"math"
	"strings"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
)

const MaxQueryPageLimit = 100

func ValidatePage(offset, limit uint64) error {
	if limit == 0 || limit > MaxQueryPageLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidRange, MaxQueryPageLimit)
	}
	if offset > math.MaxUint64-limit {
		return fmt.Errorf("%w: offset plus limit overflows uint64", ErrInvalidRange)
	}
	return nil
}

func ParseHexAddress(field, value string) (common.Address, error) {
	if !strings.HasPrefix(value, "0x") || !common.IsHexAddress(value) {
		return common.Address{}, fmt.Errorf("%w: %s must be a 0x-prefixed 40-hex-digit address", sdkerrors.ErrInvalidAddress, field)
	}
	address := common.HexToAddress(value)
	if address == (common.Address{}) {
		return common.Address{}, fmt.Errorf("%w: %s must not be the zero address", ErrZeroAddress, field)
	}
	return address, nil
}
