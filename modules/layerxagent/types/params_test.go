package types

import (
	"fmt"
	"testing"
)

func TestAgentParamsDefaultsAndValidate(t *testing.T) {
	want := Params{
		MaxJobsPerCreator:  1024,
		MaxScheduleHorizon: 2_592_000,
		MinDepositFactor:   2,
		MaxJobsPerBlock:    64,
		MaxJobGasLimit:     10_000_000,
		MaxReasonBytes:     256,
		MaxCallDataBytes:   8192,
		MaxBlockGas:        30_000_000,
		Paused:             false,
	}
	if got := DefaultParams(); got != want {
		t.Fatalf("defaults = %+v, want %+v", got, want)
	}
	if err := want.Validate(); err != nil {
		t.Fatalf("default params refused: %v", err)
	}
	for _, tc := range []struct {
		name  string
		bound func(*Params) *uint64
	}{
		{"max jobs per creator", func(p *Params) *uint64 { return &p.MaxJobsPerCreator }},
		{"max schedule horizon", func(p *Params) *uint64 { return &p.MaxScheduleHorizon }},
		{"min deposit factor", func(p *Params) *uint64 { return &p.MinDepositFactor }},
		{"max jobs per block", func(p *Params) *uint64 { return &p.MaxJobsPerBlock }},
		{"max job gas limit", func(p *Params) *uint64 { return &p.MaxJobGasLimit }},
		{"max reason bytes", func(p *Params) *uint64 { return &p.MaxReasonBytes }},
		{"max call data bytes", func(p *Params) *uint64 { return &p.MaxCallDataBytes }},
		{"max block gas", func(p *Params) *uint64 { return &p.MaxBlockGas }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := want
			*tc.bound(&params) = 0
			err := params.Validate()
			expected := fmt.Sprintf("layerxagent params: %s must be positive", tc.name)
			if err == nil || err.Error() != expected {
				t.Fatalf("zero bound: err = %v, want %q", err, expected)
			}
		})
	}
	for _, paused := range []bool{false, true} {
		params := want
		params.Paused = paused
		params.MaxJobGasLimit = params.MaxBlockGas
		if err := params.Validate(); err != nil {
			t.Fatalf("equal gas limits with paused=%v refused: %v", paused, err)
		}
		params.MaxJobGasLimit++
		if err := params.Validate(); err == nil {
			t.Fatalf("job gas above block gas accepted with paused=%v", paused)
		}
	}
	minimum := Params{MaxJobsPerCreator: 1, MaxScheduleHorizon: 1, MinDepositFactor: 1,
		MaxJobsPerBlock: 1, MaxJobGasLimit: 1, MaxReasonBytes: 1, MaxCallDataBytes: 1, MaxBlockGas: 1}
	if err := minimum.Validate(); err != nil {
		t.Fatalf("minimum positive bounds refused: %v", err)
	}
}
