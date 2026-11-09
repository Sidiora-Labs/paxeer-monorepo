package layerxprograms

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

type committedGuestEvent struct {
	program, principal common.Hash
	frame              []byte
	topic, payload     []byte
}

// The payload is receipt availability evidence, copied by lxphost from the
// native terminal and retained by receipt replay. Digest envelopes bind its
// identities, ordering and exact guest bytes before any EVM log is emitted.
func decodeCommittedGuestEvents(encoded []byte) ([]committedGuestEvent, error) {
	const domain = "LayerX/programs/events/v1\x00"
	// Failed activities publish the frozen empty envelope; it cannot carry guest bytes.
	if bytes.Equal(encoded, []byte("LXP/programs/events/v1\x00\x00\x00\x00\x00")) {
		return nil, nil
	}
	if len(encoded) < len(domain)+4 || string(encoded[:len(domain)]) != domain {
		return nil, fmt.Errorf("programs: missing canonical guest event evidence")
	}
	remaining := encoded[len(domain):]
	take := func(length int) ([]byte, bool) {
		if length < 0 || length > len(remaining) {
			return nil, false
		}
		value := remaining[:length]
		remaining = remaining[length:]
		return value, true
	}
	countBytes, _ := take(4)
	count := binary.BigEndian.Uint32(countBytes)
	if count > 64 {
		return nil, fmt.Errorf("programs: too many committed guest events")
	}
	events := make([]committedGuestEvent, 0, count)
	for index := uint32(0); index < count; index++ {
		header, ok := take(77) // program, principal, frame, depth, topic length
		if !ok {
			return nil, fmt.Errorf("programs: truncated guest event identity")
		}
		topicLength := binary.BigEndian.Uint32(header[73:77])
		if topicLength == 0 || topicLength > 64 {
			return nil, fmt.Errorf("programs: invalid committed event topic length")
		}
		topic, ok := take(int(topicLength))
		if !ok {
			return nil, fmt.Errorf("programs: truncated committed event topic")
		}
		length, ok := take(4)
		if !ok || binary.BigEndian.Uint32(length) > 65536 {
			return nil, fmt.Errorf("programs: invalid committed event payload length")
		}
		payload, ok := take(int(binary.BigEndian.Uint32(length)))
		if !ok {
			return nil, fmt.Errorf("programs: truncated committed event payload")
		}
		events = append(events, committedGuestEvent{
			program: common.BytesToHash(header[:32]), principal: common.BytesToHash(header[32:64]),
			frame: header[64:73], topic: topic, payload: payload,
		})
	}
	if len(remaining) != 0 {
		return nil, fmt.Errorf("programs: trailing committed guest event evidence")
	}
	return events, nil
}

func emitCallEvents(evm *vm.EVM, receipt lxphost.Receipt) error {
	if evm == nil {
		return fmt.Errorf("EVM is nil")
	}
	if evm.StateDB == nil {
		return fmt.Errorf("EVM StateDB is nil")
	}
	outcome := receipt.ProgramOutcome
	if outcome == nil {
		return fmt.Errorf("programs: missing call program outcome")
	}
	if outcome.FeeScheduleVersion > math.MaxUint32 || outcome.MeteringScheduleVersion > math.MaxUint32 {
		return fmt.Errorf("programs: call schedule version exceeds uint32")
	}
	committed, err := decodeCommittedGuestEvents(outcome.Events)
	if err != nil {
		return err
	}
	guestEvent := programsABI.Events["ProgramGuestEvent"]
	var guests []*types.Log
	var terminal *types.Log
	var callProgram common.Hash
	var callActivity common.Hash
	var guestActivities []common.Hash
	for _, event := range receipt.Events {
		if event.Code != 6 && event.Code != 7 {
			continue
		}
		body := event.Body
		guest := event.Code == 6
		legacy := !guest && len(body) == 251 && string(body[:6]) == "LXCO\x01\x02"
		if guest {
			if len(body) != 185 || string(body[:6]) != "LXGE\x01\x01" {
				return fmt.Errorf("programs: invalid guest event envelope")
			}
		} else if !legacy && (len(body) != 255 || string(body[:6]) != "LXMO\x02\x02") {
			return fmt.Errorf("programs: invalid call outcome envelope")
		}
		for _, offset := range []int{6, 38, 70} {
			if common.BytesToHash(body[offset:offset+32]) == (common.Hash{}) {
				return fmt.Errorf("programs: zero call event identity")
			}
		}
		depth := body[110]
		if depth > 8 {
			return fmt.Errorf("programs: invalid call event depth")
		}
		for i, component := range body[102:110] {
			if (i < int(depth)) != (component != 0) {
				return fmt.Errorf("programs: invalid call event frame path")
			}
		}
		if guest {
			index := len(guests)
			if index >= len(committed) || binary.BigEndian.Uint32(body[111:115]) != uint32(index) {
				return fmt.Errorf("programs: guest event order disagrees with receipt")
			}
			canonical := committed[index]
			topicDigest := sha256.Sum256(append([]byte("LayerX/programs/event-topic/v1"), canonical.topic...))
			dataDigest := sha256.Sum256(append([]byte("LayerX/programs/event-data/v1"), canonical.payload...))
			if canonical.program != common.BytesToHash(body[6:38]) || canonical.principal != common.BytesToHash(body[38:70]) ||
				!bytes.Equal(canonical.frame, body[102:111]) || int(binary.BigEndian.Uint16(body[115:117])) != len(canonical.topic) ||
				int(binary.BigEndian.Uint32(body[149:153])) != len(canonical.payload) ||
				!bytes.Equal(topicDigest[:], body[117:149]) || !bytes.Equal(dataDigest[:], body[153:185]) {
				return fmt.Errorf("programs: guest event bytes disagree with committed envelope")
			}
			guestActivities = append(guestActivities, common.BytesToHash(body[70:102]))
			data, err := guestEvent.Inputs.NonIndexed().Pack(
				[32]byte(body[38:70]), [32]byte(body[70:102]),
				binary.BigEndian.Uint64(body[102:110]), depth,
				binary.BigEndian.Uint32(body[111:115]), binary.BigEndian.Uint16(body[115:117]),
				topicDigest, binary.BigEndian.Uint32(body[149:153]), dataDigest,
			)
			if err != nil {
				return fmt.Errorf("programs: pack guest event: %w", err)
			}
			guests = append(guests, &types.Log{Address: common.HexToAddress("0x101D"), Topics: []common.Hash{guestEvent.ID, canonical.program}, Data: data})
			continue
		}
		if terminal != nil {
			return fmt.Errorf("programs: duplicate call outcome envelope")
		}
		fee := binary.BigEndian.Uint32(body[115:119])
		metering := uint32(1)
		offset := 119
		if !legacy {
			metering = binary.BigEndian.Uint32(body[offset : offset+4])
			offset += 4
		}
		status := int32(binary.BigEndian.Uint32(body[offset : offset+4]))
		offset += 4
		if uint64(fee) != outcome.FeeScheduleVersion || uint64(metering) != outcome.MeteringScheduleVersion || status != outcome.Status {
			return fmt.Errorf("programs: call outcome envelope disagrees with program outcome")
		}
		eventsDigest := sha256.Sum256(outcome.Events)
		if !bytes.Equal(eventsDigest[:], body[offset+96:offset+128]) {
			return fmt.Errorf("programs: guest event evidence digest disagrees with receipt")
		}
		values := []interface{}{
			[32]byte(body[38:70]), [32]byte(body[70:102]), binary.BigEndian.Uint64(body[102:110]), depth,
			binary.BigEndian.Uint16(body[111:113]), binary.BigEndian.Uint16(body[113:115]),
			uint32(outcome.FeeScheduleVersion), uint32(outcome.MeteringScheduleVersion), outcome.Status,
			[32]byte(body[offset : offset+32]), [32]byte(body[offset+32 : offset+64]),
			[32]byte(body[offset+64 : offset+96]), [32]byte(body[offset+96 : offset+128]),
		}
		abiEvent := programsABI.Events["ProgramCallOutcome"]
		data, err := abiEvent.Inputs.NonIndexed().Pack(values...)
		if err != nil {
			return fmt.Errorf("programs: pack ProgramCallOutcome: %w", err)
		}
		callProgram = common.BytesToHash(body[6:38])
		callActivity = common.BytesToHash(body[70:102])
		terminal = &types.Log{Address: common.HexToAddress("0x101D"), Topics: []common.Hash{abiEvent.ID, callProgram}, Data: data}
	}
	if terminal == nil || len(guests) != len(committed) {
		return fmt.Errorf("programs: missing call outcome or guest event envelope")
	}
	for index := range guests {
		if guestActivities[index] != callActivity {
			return fmt.Errorf("programs: guest activity disagrees with call receipt")
		}
	}
	for _, log := range append(guests, terminal) {
		if err := pcommon.EmitEVMLog(evm, log.Address, log.Topics, log.Data); err != nil {
			return err
		}
	}
	return nil
}
