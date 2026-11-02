package types

import (
	"fmt"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type Params struct {
	Authority            common.Address `json:"authority"`
	MinBond              sdk.Int        `json:"min_bond"`
	CreationFee          sdk.Int        `json:"creation_fee"`
	MinGasAllowance      uint64         `json:"min_gas_allowance"`
	MaxGasAllowance      uint64         `json:"max_gas_allowance"`
	MaxTotalGasAllowance uint64         `json:"max_total_gas_allowance"`
	MinByteAllowance     uint64         `json:"min_byte_allowance"`
	MaxByteAllowance     uint64         `json:"max_byte_allowance"`
	MaxChains            uint32         `json:"max_chains"`
	MaxFeeShareBps       uint16         `json:"max_fee_share_bps"`
}

func DefaultParams() Params {
	// One PAX is 10^6 bank base units; the EVM converts each bank unit to 10^12 wei.
	return Params{MinBond: sdk.NewInt(100_000_000), CreationFee: sdk.NewInt(10_000_000),
		MinGasAllowance: 250000, MaxGasAllowance: 4000000, MaxTotalGasAllowance: 6250000,
		MinByteAllowance: 16384, MaxByteAllowance: 1048576, MaxChains: 1024, MaxFeeShareBps: 5000}
}
func (p Params) Validate() error {
	if p.MinBond.IsNil() || p.MinBond.IsNegative() || p.CreationFee.IsNil() || p.CreationFee.IsNegative() {
		return fmt.Errorf("invalid bond or creation fee")
	}
	if p.MinGasAllowance < 2 || p.MaxGasAllowance < p.MinGasAllowance || p.MaxTotalGasAllowance < p.MaxGasAllowance {
		return ErrAllowanceOutOfRange
	}
	if p.MinByteAllowance == 0 || p.MaxByteAllowance < p.MinByteAllowance || p.MaxChains == 0 || p.MaxFeeShareBps > 10000 {
		return ErrAllowanceOutOfRange
	}
	return nil
}
func (p Params) ValidateAllowances(gas, bytes uint64, share uint16) error {
	if gas < p.MinGasAllowance || gas > p.MaxGasAllowance || bytes < p.MinByteAllowance || bytes > p.MaxByteAllowance || share > p.MaxFeeShareBps {
		return ErrAllowanceOutOfRange
	}
	return nil
}
