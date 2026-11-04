package types

import (
	"bytes"
	"encoding/binary"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

var ErrInvalidGenesis = sdkerrors.Register(ModuleName, 3, "invalid genesis")

func DefaultParams() Params {
	return Params{Paused: true}
}

func (Params) Validate() error {
	return nil
}

type GenesisState struct {
	Params              Params    `json:"params"`
	KernelRecords       []kv.Pair `json:"kernel_records"`
	AgentStreamMetadata []kv.Pair `json:"agent_stream_metadata"`
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

func (g GenesisState) Validate() error {
	if err := g.Params.Validate(); err != nil {
		return err
	}
	if len(g.AgentStreamMetadata) > MaxKernelRecords {
		return ErrInvalidGenesis.Wrapf("agent stream metadata count %d exceeds %d", len(g.AgentStreamMetadata), MaxKernelRecords)
	}
	metadataKeys := make(map[string]struct{}, len(g.AgentStreamMetadata))
	for i, record := range g.AgentStreamMetadata {
		if len(record.Key) != len(AgentStreamMetadataPrefix)+32 || !bytes.HasPrefix(record.Key, []byte(AgentStreamMetadataPrefix)) {
			return ErrInvalidGenesis.Wrapf("agent stream metadata %d has an invalid key", i)
		}
		if bytes.Equal(record.Key[len(AgentStreamMetadataPrefix):], make([]byte, 32)) {
			return ErrInvalidGenesis.Wrapf("agent stream metadata %d has a zero stream id", i)
		}
		if len(record.Value) != 60 {
			return ErrInvalidGenesis.Wrapf("agent stream metadata %d has an invalid value length", i)
		}
		key := string(record.Key)
		if _, duplicate := metadataKeys[key]; duplicate {
			return ErrInvalidGenesis.Wrapf("duplicate agent stream metadata %d", i)
		}
		metadataKeys[key] = struct{}{}
	}
	// Kernel families retain records across their lifetime without an aggregate
	// admission limit. Bound each record rather than rejecting valid exports by
	// count; MaxKernelRecords bounds recovery chunks and admitted agent metadata.
	records := make(map[string][]byte, len(g.KernelRecords))
	for i, record := range g.KernelRecords {
		if len(record.Key) < 4 || len(record.Key) > 3+MaxKernelKeyBytes || record.Key[0] != KernelKVPrefix[0] {
			return ErrInvalidGenesis.Wrapf("kernel record %d has an invalid key", i)
		}
		moduleID := binary.BigEndian.Uint16(record.Key[1:3])
		if moduleID == 0 || moduleID > 11 {
			return ErrInvalidGenesis.Wrapf("kernel record %d has an unknown module %d", i, moduleID)
		}
		if len(record.Value) > MaxKernelValueBytes {
			return ErrInvalidGenesis.Wrapf("kernel record %d value exceeds %d bytes", i, MaxKernelValueBytes)
		}
		key := string(record.Key)
		if _, duplicate := records[key]; duplicate {
			return ErrInvalidGenesis.Wrapf("duplicate kernel record %d", i)
		}
		records[key] = record.Value
		if moduleID != GovernanceModuleID || !bytes.HasPrefix(record.Key[3:], []byte("module-enable:")) {
			continue
		}
		known := false
		for _, gatedModule := range []uint16{EscrowModuleID, BudgetModuleID, StreamModuleID, ServiceModuleID, 6, 10, 11} {
			gateKey, err := KernelModuleEnableKey(gatedModule)
			if err != nil {
				return err
			}
			if bytes.Equal(record.Key, gateKey) {
				known = true
				break
			}
		}
		if !known {
			return ErrInvalidGenesis.Wrapf("kernel record %d has an unknown enable flag", i)
		}
		if len(record.Value) != 32 || !bytes.Equal(record.Value[:31], make([]byte, 31)) || record.Value[31] > 1 {
			return ErrInvalidGenesis.Wrapf("kernel record %d has a non-canonical enable flag", i)
		}
	}
	for _, module := range []struct {
		id      uint16
		name    string
		enabled bool
	}{
		{EscrowModuleID, "escrow", g.Params.EscrowEnabled},
		{BudgetModuleID, "budget", g.Params.BudgetEnabled},
		{StreamModuleID, "stream", g.Params.StreamEnabled},
		{ServiceModuleID, "service", g.Params.ServiceEnabled},
	} {
		if !module.enabled {
			continue
		}
		key, err := KernelModuleEnableKey(module.id)
		if err != nil {
			return err
		}
		flag := records[string(key)]
		if len(flag) != 32 || flag[31] != 1 {
			return ErrInvalidGenesis.Wrapf("%s enabled while kernel genesis gate is clear", module.name)
		}
	}
	return nil
}
