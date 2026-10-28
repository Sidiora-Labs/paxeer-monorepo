package keeper

import (
	"bytes"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
)

type Event struct {
	Name    string
	GrantID [32]byte
	DIDID   [32]byte
	Sub     uint8
	Chunk   uint8
	Body    []byte
}

func AuthEvents(receipt lxphost.Receipt) ([]Event, error) {
	events := make([]Event, 0, len(receipt.Events))
	var grantID [32]byte
	var sub uint8
	nextChunk, previousSize := 0, 0
	for _, raw := range receipt.Events {
		event := Event{Body: bytes.Clone(raw.Body)}
		switch raw.Code {
		case 0x7145:
			if len(raw.Body) != 209 || !bytes.Equal(raw.Body[:5], []byte("LXGS2")) {
				return nil, types.ErrNonCanonical
			}
			event.Name = "SessionGranted"
			copy(event.GrantID[:], raw.Body[5:37])
			grantID, sub = event.GrantID, 5
			nextChunk, previousSize = 0, 0
		case 0x7148:
			if len(raw.Body) != 32 {
				return nil, types.ErrNonCanonical
			}
			event.Name = "CapabilityGranted"
			copy(event.GrantID[:], raw.Body)
			grantID, sub = event.GrantID, 8
			nextChunk, previousSize = 0, 0
		case 0x7105, 0x7125, 0x7108, 0x7128:
			chunkSub := uint8(raw.Code & 0xf)
			first := raw.Code == 0x7105 || raw.Code == 0x7108
			if sub != chunkSub || len(raw.Body) == 0 || len(raw.Body) > 256 || nextChunk > 255 ||
				(first && nextChunk != 0) || (!first && (nextChunk == 0 || previousSize != 256)) {
				return nil, types.ErrNonCanonical
			}
			event.Name = "GrantBody"
			event.GrantID, event.Sub, event.Chunk = grantID, sub, uint8(nextChunk)
			nextChunk++
			previousSize = len(raw.Body)
		case 0x7106:
			if len(raw.Body) != 41 {
				return nil, types.ErrNonCanonical
			}
			event.Name = "GrantRevoked"
			copy(event.GrantID[:], raw.Body[:32])
			sub = 0
		case 0x7110:
			if len(raw.Body) != 223 || !bytes.Equal(raw.Body[:5], []byte("LXGI1")) {
				return nil, types.ErrNonCanonical
			}
			event.Name = "IdentityStateUpdated"
			copy(event.DIDID[:], raw.Body[5:37])
			sub = 0
		default:
			return nil, types.ErrNonCanonical
		}
		events = append(events, event)
	}
	return events, nil
}
