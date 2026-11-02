package layerxprograms

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

var Address = common.HexToAddress("0x000000000000000000000000000000000000101D")

type programsReceiptLog struct {
	name         string
	programID    common.Hash
	hasProgramID bool
	values       []interface{}
}

func receiptEventInvariant(code uint16, detail string) error {
	return fmt.Errorf("%w: programs receipt event %d: %s", lxphost.Code(-1001), code, detail)
}

func decodeProgramsReceiptEvent(activityType uint32, event lxphost.ReceiptEvent) (programsReceiptLog, error) {
	var log programsReceiptLog
	var activity uint32
	var length int
	switch event.Code {
	case 1:
		log.name, activity, length = "ProgramDeployed", 0x00090001, 32
	case 2:
		log.name, activity, length = "ProgramUpgraded", 0x00090002, 64
	case 4:
		log.name, activity, length = "RegistryRead", 0x00090004, 32
	case 5:
		log.name, activity, length = "ProgramTransferred", 0x00090005, 32
	case 8:
		log.name, activity, length = "ProgramAccountRegistered", 0x00090006, 143
	case 9:
		log.name, activity, length = "ExitRouteSet", 0x00090007, 128
	case 10:
		log.name, activity, length = "ProgramDeprecated", 0x00090007, 119
	case 11:
		log.name, activity, length = "ProgramTombstoned", 0x00090007, 119
	case 12:
		log.name, activity, length = "ProgramValueExited", 0x00090007, 96
	default:
		return log, receiptEventInvariant(event.Code, "unsupported event code")
	}
	if activityType != activity {
		return log, receiptEventInvariant(event.Code, fmt.Sprintf("unexpected activity type %#08x", activityType))
	}
	if len(event.Body) != length {
		return log, receiptEventInvariant(event.Code, fmt.Sprintf("body length %d, want %d", len(event.Body), length))
	}
	body := event.Body
	word := func(offset int) [32]byte { return [32]byte(body[offset : offset+32]) }
	switch event.Code {
	case 1, 5:
		log.values = []interface{}{word(0)}
	case 2:
		log.values = []interface{}{word(0), word(32)}
	case 4:
		log.programID, log.hasProgramID = common.Hash(word(0)), true
	case 8:
		if !bytes.Equal(body[:5], []byte("LXPA1")) {
			return log, receiptEventInvariant(event.Code, "invalid account event magic")
		}
		log.programID, log.hasProgramID = common.Hash(word(5)), true
		log.values = []interface{}{
			word(37), word(69), word(103),
			binary.BigEndian.Uint16(body[101:103]), binary.BigEndian.Uint64(body[135:143]),
		}
	case 9:
		log.programID, log.hasProgramID = common.Hash(word(0)), true
		log.values = []interface{}{word(32), word(64), word(96)}
	case 10, 11:
		if body[0] != 1 {
			return log, receiptEventInvariant(event.Code, "unsupported lifecycle history version")
		}
		log.values = []interface{}{
			body[1], body[2], word(3), word(35),
			binary.BigEndian.Uint64(body[67:75]), binary.BigEndian.Uint64(body[75:83]),
			binary.BigEndian.Uint16(body[83:85]), binary.BigEndian.Uint16(body[85:87]), word(87),
		}
	case 12:
		log.programID, log.hasProgramID = common.Hash(word(0)), true
		log.values = []interface{}{word(32), word(64)}
	}
	return log, nil
}

func emitReceiptEvents(evm *vm.EVM, activityType uint32, programID [32]byte, receipt lxphost.Receipt) error {
	type packedLog struct {
		topics []common.Hash
		data   []byte
	}
	logs := make([]packedLog, 0, len(receipt.Events))
	for _, event := range receipt.Events {
		log, err := decodeProgramsReceiptEvent(activityType, event)
		if err != nil {
			return err
		}
		if !log.hasProgramID {
			log.programID = common.Hash(programID)
		}
		definition, ok := programsABI.Events[log.name]
		if !ok {
			return receiptEventInvariant(event.Code, "event is missing from programs ABI")
		}
		data, err := definition.Inputs.NonIndexed().Pack(log.values...)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", lxphost.Code(-1001), log.name, err)
		}
		logs = append(logs, packedLog{topics: []common.Hash{definition.ID, log.programID}, data: data})
	}
	for _, log := range logs {
		if err := pcommon.EmitEVMLog(evm, Address, log.topics, log.data); err != nil {
			return err
		}
	}
	return nil
}
