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
	if err := validateOwnedGenesisRecords(g.KernelRecords, records); err != nil {
		return err
	}
	for i, metadata := range g.AgentStreamMetadata {
		id := metadata.Key[len(AgentStreamMetadataPrefix):]
		stream, err := DecodeStreamRecord(records[string(KernelKVKey(StreamModuleID, append([]byte("stream:"), id...)))])
		if err != nil || stream.Mode != StreamModeTime || stream.RateUnit != 1000 {
			return ErrInvalidGenesis.Wrapf("agent stream metadata %d has no matching agent stream", i)
		}
		if bytes.Equal(metadata.Value[:20], make([]byte, 20)) ||
			bytes.Equal(metadata.Value[20:40], make([]byte, 20)) ||
			bytes.Equal(metadata.Value[:20], metadata.Value[20:40]) {
			return ErrInvalidGenesis.Wrapf("agent stream metadata %d has invalid participants", i)
		}
	}
	return nil
}

func validateOwnedGenesisRecords(ordered []kv.Pair, records map[string][]byte) error {
	for _, record := range ordered {
		wireKey, body := record.Key, record.Value
		module := binary.BigEndian.Uint16(wireKey[1:3])
		local := wireKey[3:]
		var id [32]byte
		var err error
		var prefix string
		keyID := func(p string, extra int) bool {
			prefix = p
			return len(local) == len(p)+32+extra &&
				!bytes.Equal(local[len(p):len(p)+32], make([]byte, 32))
		}
		switch {
		case module == EscrowModuleID && bytes.HasPrefix(local, []byte("hold:")):
			if !keyID("hold:", 0) {
				return ErrInvalidGenesis.Wrap("invalid escrow hold key")
			}
			var r EscrowRecord
			r, err = DecodeEscrowRecord(body)
			id = r.EscrowID
		case module == EscrowModuleID && bytes.HasPrefix(local, []byte("result:")):
			if !keyID("result:", 0) {
				return ErrInvalidGenesis.Wrap("invalid escrow result key")
			}
			r, decodeErr := DecodeEscrowResult(body)
			if decodeErr != nil {
				return ErrInvalidGenesis.Wrap("invalid escrow result body")
			}
			if _, ok := records[string(KernelKVKey(module, append([]byte("hold:"), r.EscrowID[:]...)))]; !ok {
				return ErrInvalidGenesis.Wrap("escrow result has no hold")
			}
			continue // Result keys are replay keys, not escrow ids.
		case module == BudgetModuleID && bytes.HasPrefix(local, []byte("budget:")):
			if !keyID("budget:", 0) {
				return ErrInvalidGenesis.Wrap("invalid budget key")
			}
			var r BudgetRecord
			r, err = DecodeBudgetRecord(body)
			id = r.BudgetID
		case module == StreamModuleID && bytes.HasPrefix(local, []byte("stream:")):
			if !keyID("stream:", 0) {
				return ErrInvalidGenesis.Wrap("invalid stream key")
			}
			var r StreamRecord
			r, err = DecodeStreamRecord(body)
			id = r.StreamID
		case module == StreamModuleID && bytes.HasPrefix(local, []byte("result:")):
			if !keyID("result:", 0) {
				return ErrInvalidGenesis.Wrap("invalid stream result key")
			}
			r, decodeErr := DecodeStreamResult(body)
			if decodeErr != nil {
				return ErrInvalidGenesis.Wrap("invalid stream result body")
			}
			if _, ok := records[string(KernelKVKey(module, append([]byte("stream:"), r.StreamID[:]...)))]; !ok {
				return ErrInvalidGenesis.Wrap("stream result has no stream")
			}
			continue
		case module == ServiceModuleID:
			for _, p := range []string{"offer::1", "agree::1", "commit:1", "execut:1", "delivr:1", "delitm:1", "disput:1", "progrs:1"} {
				if bytes.HasPrefix(local, []byte(p[:7])) {
					prefix = p
					break
				}
			}
			if prefix == "" {
				continue
			}
			extra := 0
			if prefix == "delitm:1" {
				extra = 1
			}
			if !bytes.HasPrefix(local, []byte(prefix)) || !keyID(prefix, extra) {
				return ErrInvalidGenesis.Wrap("invalid service record key or version")
			}
			switch prefix {
			case "offer::1":
				var r ServiceOffer
				r, err = DecodeServiceOffer(body)
				id = r.OfferID
			case "agree::1":
				var r ServiceAgreement
				r, err = DecodeServiceAgreement(body)
				id = r.AgreementID
			case "commit:1":
				var r ServiceCommitment
				r, err = DecodeServiceCommitment(body)
				id = r.CommitmentID
			case "execut:1":
				var r ServiceExecution
				r, err = DecodeServiceExecution(body)
				id = r.AttestationID
			case "disput:1":
				var r ServiceDispute
				r, err = DecodeServiceDispute(body)
				id = r.DisputeID
			case "progrs:1":
				var r ServiceProgress
				r, err = DecodeServiceProgress(body)
				id = r.ReportID
			case "delitm:1":
				headerKey := KernelKVKey(module, append([]byte("delivr:1"), local[8:40]...))
				header := records[string(headerKey)]
				if len(header) != 138 || int(local[40]) >= int(header[129]) || len(body) != 73 || body[0] != 1 {
					return ErrInvalidGenesis.Wrap("invalid or orphaned service delivery item")
				}
				continue
			case "delivr:1":
				if len(body) != 138 || body[0] != 1 || body[129] == 0 || int(body[129]) > ServiceMaxDeliverables {
					return ErrInvalidGenesis.Wrap("invalid service delivery header")
				}
				id = [32]byte(body[1:33])
				assembled := bytes.Clone(body)
				for i := byte(0); i < body[129]; i++ {
					itemKey := KernelKVKey(module, append(append([]byte("delitm:1"), local[8:40]...), i))
					item := records[string(itemKey)]
					if len(item) != 73 {
						return ErrInvalidGenesis.Wrap("missing or invalid service delivery item")
					}
					assembled = append(assembled, item...)
				}
				_, err = DecodeServiceDelivery(assembled)
			}
		default:
			continue
		}
		if err != nil {
			return ErrInvalidGenesis.Wrapf("invalid %s record body: %v", prefix, err)
		}
		if id == ([32]byte{}) || !bytes.Equal(id[:], local[len(prefix):len(prefix)+32]) {
			return ErrInvalidGenesis.Wrapf("%s key/body id mismatch", prefix)
		}
	}
	return nil
}
