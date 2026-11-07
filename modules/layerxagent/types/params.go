package types

import "fmt"

const (
	DefaultMaxJobsPerCreator  uint64 = 1024
	DefaultMaxScheduleHorizon uint64 = 2_592_000
	DefaultMinDepositFactor   uint64 = 2
	DefaultMaxJobsPerBlock    uint64 = 64
	DefaultMaxJobGasLimit     uint64 = 10_000_000
	DefaultMaxReasonBytes     uint64 = 256
	DefaultMaxCallDataBytes   uint64 = 8192
	DefaultMaxBlockGas        uint64 = 30_000_000
)

func DefaultParams() Params {
	return Params{
		MaxJobsPerCreator:  DefaultMaxJobsPerCreator,
		MaxScheduleHorizon: DefaultMaxScheduleHorizon,
		MinDepositFactor:   DefaultMinDepositFactor,
		MaxJobsPerBlock:    DefaultMaxJobsPerBlock,
		MaxJobGasLimit:     DefaultMaxJobGasLimit,
		MaxReasonBytes:     DefaultMaxReasonBytes,
		MaxCallDataBytes:   DefaultMaxCallDataBytes,
		MaxBlockGas:        DefaultMaxBlockGas,
		Paused:             false,
	}
}

func (p Params) Validate() error {
	for _, field := range []struct {
		name  string
		value uint64
	}{
		{"max jobs per creator", p.MaxJobsPerCreator},
		{"max schedule horizon", p.MaxScheduleHorizon},
		{"min deposit factor", p.MinDepositFactor},
		{"max jobs per block", p.MaxJobsPerBlock},
		{"max job gas limit", p.MaxJobGasLimit},
		{"max reason bytes", p.MaxReasonBytes},
		{"max call data bytes", p.MaxCallDataBytes},
		{"max block gas", p.MaxBlockGas},
	} {
		if field.value == 0 {
			return fmt.Errorf("layerxagent params: %s must be positive", field.name)
		}
	}
	if p.MaxJobGasLimit > p.MaxBlockGas {
		return fmt.Errorf("layerxagent params: max job gas limit must not exceed max block gas")
	}
	return nil
}
