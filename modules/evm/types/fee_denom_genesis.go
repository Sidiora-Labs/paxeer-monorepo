package types

import (
	"bytes"
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func ValidateAccountFeeDenomGenesisEntry(prefix, key, value []byte) error {
	if len(key) == 0 && len(prefix) > 1 {
		prefix, key = prefix[:1], prefix[1:]
	}
	if !bytes.Equal(prefix, AccountFeeDenomKeyPrefix) {
		return nil
	}
	if len(key) != common.AddressLength {
		return fmt.Errorf("account fee denom genesis: key must be %d bytes, got %d", common.AddressLength, len(key))
	}
	if err := sdk.ValidateDenom(string(value)); err != nil {
		return fmt.Errorf("account fee denom genesis: %w", err)
	}
	return nil
}
