package layerxprograms

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

func emitCallEvents(evm *vm.EVM, receipt lxphost.Receipt) error {
	outcome := receipt.ProgramOutcome
	if outcome == nil {
		return fmt.Errorf("programs: missing call program outcome")
	}
	if outcome.FeeScheduleVersion > math.MaxUint32 || outcome.MeteringScheduleVersion > math.MaxUint32 {
		return fmt.Errorf("programs: call schedule version exceeds uint32")
	}
	var guests []*types.Log
	var terminal *types.Log
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
		values := []interface{}{
			[32]byte(body[38:70]), [32]byte(body[70:102]),
			binary.BigEndian.Uint64(body[102:110]), depth,
		}
		name := "ProgramGuestEvent"
		if guest {
			if binary.BigEndian.Uint16(body[115:117]) > 64 || binary.BigEndian.Uint32(body[149:153]) > 65536 {
				return fmt.Errorf("programs: guest event lengths exceed kernel bounds")
			}
			values = append(values, binary.BigEndian.Uint32(body[111:115]),
				binary.BigEndian.Uint16(body[115:117]), [32]byte(body[117:149]),
				binary.BigEndian.Uint32(body[149:153]), [32]byte(body[153:185]))
		} else {
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
			name = "ProgramCallOutcome"
			values = append(values, binary.BigEndian.Uint16(body[111:113]),
				binary.BigEndian.Uint16(body[113:115]), uint32(outcome.FeeScheduleVersion),
				uint32(outcome.MeteringScheduleVersion), outcome.Status,
				[32]byte(body[offset:offset+32]), [32]byte(body[offset+32:offset+64]),
				[32]byte(body[offset+64:offset+96]), [32]byte(body[offset+96:offset+128]))
		}
		abiEvent := programsABI.Events[name]
		data, err := abiEvent.Inputs.NonIndexed().Pack(values...)
		if err != nil {
			return fmt.Errorf("programs: pack %s: %w", name, err)
		}
		log := &types.Log{
			Address: common.HexToAddress("0x101D"),
			Topics:  []common.Hash{abiEvent.ID, common.BytesToHash(body[6:38])},
			Data:    data,
		}
		if guest {
			guests = append(guests, log)
		} else {
			terminal = log
		}
	}
	if terminal == nil {
		return fmt.Errorf("programs: missing call outcome envelope")
	}
	for _, log := range append(guests, terminal) {
		if err := pcommon.EmitEVMLog(evm, log.Address, log.Topics, log.Data); err != nil {
			return err
		}
	}
	return nil
}
