package layerxprograms

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

func emitSandboxEvents(evm *vm.EVM, receipt lxphost.Receipt) error {
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	if err != nil {
		return fmt.Errorf("sandbox events: receipt: %w", err)
	}
	var logs []*ethtypes.Log
	var leaseID common.Hash
	var destructionPending bool
	var retirementNext, retirementCount uint8
	for _, effect := range decoded.Effects {
		if effect.ModuleID != 9 || effect.Kind != 3 {
			continue
		}
		body := effect.Body
		var name string
		var topic common.Hash
		var values []interface{}
		switch effect.EventType {
		case 0x0909:
			if len(body) <= 52 || !bytes.Equal(body[:4], []byte("LXUR")) {
				return fmt.Errorf("sandbox events: malformed usage frame")
			}
			activityID := common.BytesToHash(body[4:36])
			sequence := binary.BigEndian.Uint64(body[36:44])
			index := binary.BigEndian.Uint16(body[44:46])
			count := binary.BigEndian.Uint16(body[46:48])
			length := binary.BigEndian.Uint32(body[48:52])
			if activityID != common.Hash(decoded.ActivityID) || length == 0 || length > 4096 ||
				uint32(count) != (length+203)/204 || index >= count {
				return fmt.Errorf("sandbox events: invalid usage chunk header")
			}
			chunkLength := uint32(204)
			if index == count-1 {
				chunkLength = length - uint32(index)*204
			}
			if uint32(len(body)-52) != chunkLength {
				return fmt.Errorf("sandbox events: invalid usage chunk length")
			}
			name, topic = "SandboxUsageReceipt", activityID
			values = []interface{}{sequence, index, count, length, body[52:]}
		case 0x090A:
			if len(body) < 8 || !bytes.Equal(body[:4], []byte("LXDT")) ||
				body[4] > 1 || body[5] != 2 || body[6] != 1 || body[7] != 0 {
				return fmt.Errorf("sandbox events: malformed destruction frame")
			}
			if body[4] == 0 {
				if destructionPending || len(body) != 256 || !bytes.Equal(body[8:13], []byte("LXSD1")) ||
					!bytes.Equal(body[45:77], decoded.ActivityID[:]) {
					return fmt.Errorf("sandbox events: invalid destruction header")
				}
				leaseID = common.BytesToHash(body[13:45])
				destructionPending = true
			} else {
				if !destructionPending || len(body) != 112 {
					return fmt.Errorf("sandbox events: orphan destruction continuation")
				}
				destructionPending = false
			}
			name, topic = "SandboxDestroyed", leaseID
			values = []interface{}{body[4], body}
		case 0x090B:
			if len(body) < 7 || !bytes.Equal(body[:4], []byte("LXRT")) || body[6] != 1 ||
				body[5] < 2 || body[5] > 4 || body[4] >= body[5] {
				return fmt.Errorf("sandbox events: malformed retirement frame")
			}
			if body[4] == 0 {
				if retirementNext != retirementCount || len(body) != 226 ||
					!bytes.Equal(body[7:39], decoded.ActivityID[:]) || body[199] != body[5]-1 {
					return fmt.Errorf("sandbox events: invalid retirement header")
				}
				retirementNext, retirementCount = 1, body[5]
			} else {
				if body[4] != retirementNext || body[5] != retirementCount || len(body) != 117 {
					return fmt.Errorf("sandbox events: orphan retirement candidate")
				}
				retirementNext++
			}
			name, topic = "StorageRetired", common.Hash(decoded.ActivityID)
			values = []interface{}{body[4], body}
		default:
			continue
		}
		event := programsABI.Events[name]
		data, err := event.Inputs.NonIndexed().Pack(values...)
		if err != nil {
			return fmt.Errorf("sandbox events: %s: %w", name, err)
		}
		logs = append(logs, &ethtypes.Log{
			Address: common.HexToAddress("0x000000000000000000000000000000000000101D"),
			Topics:  []common.Hash{event.ID, topic},
			Data:    data,
		})
	}
	if destructionPending || retirementNext != retirementCount {
		return fmt.Errorf("sandbox events: incomplete terminal frames")
	}
	for _, log := range logs {
		if err := pcommon.EmitEVMLog(evm, log.Address, log.Topics, log.Data); err != nil {
			return err
		}
	}
	return nil
}
