package types

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"math"
	"math/big"
)

const (
	StatusActive           uint8  = 1
	StatusPaused           uint8  = 2
	StatusGovernancePaused uint8  = 3
	StatusRetired          uint8  = 4
	VirtualChainIDBase     uint64 = 125000000
)

type Domain struct {
	Id                uint32         `json:"id"`
	Name              string         `json:"name"`
	Operator          common.Address `json:"operator"`
	Status            uint8          `json:"status"`
	GasAllowance      uint64         `json:"gas_allowance"`
	ByteAllowance     uint64         `json:"byte_allowance"`
	FeeShareBps       uint16         `json:"fee_share_bps"`
	Bond              sdk.Int        `json:"bond"`
	TargetGasPerBlock uint64         `json:"target_gas_per_block"`
	CreatedHeight     int64          `json:"created_height"`
}

func ChainIDOf(id uint32) *big.Int { return new(big.Int).SetUint64(VirtualChainIDBase + uint64(id)) }
func DomainOfChainID(chainID *big.Int) (uint32, bool) {
	if chainID == nil || !chainID.IsUint64() {
		return 0, false
	}
	n := chainID.Uint64()
	if n == 125 {
		return 0, true
	}
	if n <= VirtualChainIDBase || n > VirtualChainIDBase+math.MaxUint32 {
		return 0, false
	}
	return uint32(n - VirtualChainIDBase), true
}
func ValidateName(name string) error {
	if len(name) < 3 || len(name) > 32 || name[0] == '-' || name[len(name)-1] == '-' {
		return ErrInvalidName
	}
	for _, c := range []byte(name) {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return ErrInvalidName
		}
	}
	return nil
}
