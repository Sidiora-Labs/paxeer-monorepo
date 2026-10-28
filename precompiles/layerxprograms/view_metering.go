package layerxprograms

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type meteringSchedule struct {
	version       uint32
	coefficients  [9]uint64
	activation    uint64
	authorityKind uint8
	authority     [32]byte
}

func decodeMeteringSchedule(record []byte) (meteringSchedule, error) {
	var schedule meteringSchedule
	if len(record) != 122 || string(record[:5]) != "LXMR1" {
		return schedule, lxphost.Code(-3)
	}
	schedule.version = binary.BigEndian.Uint32(record[5:9])
	for i := range schedule.coefficients {
		schedule.coefficients[i] = binary.BigEndian.Uint64(record[9+i*8 : 17+i*8])
	}
	schedule.activation = binary.BigEndian.Uint64(record[81:89])
	schedule.authorityKind = record[89]
	copy(schedule.authority[:], record[90:122])
	if schedule.version == 0 || schedule.activation == 0 ||
		(schedule.authorityKind != 1 && schedule.authorityKind != 2) || schedule.authority == ([32]byte{}) {
		return meteringSchedule{}, lxphost.Code(-3)
	}
	genesis := [9]uint64{1, 1, 1, 1, 1, 8, 8, 64, 8}
	for i, coefficient := range schedule.coefficients {
		if coefficient == 0 || (schedule.version == 1 && coefficient != genesis[i]) {
			return meteringSchedule{}, lxphost.Code(-3)
		}
	}
	return schedule, nil
}

func meteringReadError(err error, method string) error {
	if errors.Is(err, lxphost.Code(-7)) || errors.Is(err, lxphost.Code(-101)) {
		return fmt.Errorf("LXP_NOT_FOUND: %s: %w", method, err)
	}
	var code lxphost.Code
	if errors.As(err, &code) {
		return kernelError(code, method)
	}
	return err
}

func getCurrentMeteringScheduleHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	const method = "getCurrentMeteringSchedule"
	if _, err := decodeArgs(call.method, args); err != nil {
		return nil, err
	}
	record, err := p.host.ReadMeteringSchedule(ctx)
	if err != nil {
		return nil, meteringReadError(err, method)
	}
	schedule, err := decodeMeteringSchedule(record)
	if err != nil {
		return nil, meteringReadError(err, method)
	}
	return call.method.Outputs.Pack(schedule.version, schedule.coefficients)
}

func getMeteringScheduleHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	const method = "getMeteringSchedule"
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	version, ok := decoded[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: expected uint32 metering schedule version, got %T", decoded[0])
	}
	record, err := p.host.ReadMeteringScheduleAt(ctx, version)
	if err != nil {
		return nil, meteringReadError(err, method)
	}
	schedule, err := decodeMeteringSchedule(record)
	if err != nil {
		return nil, meteringReadError(err, method)
	}
	if schedule.version != version {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: %s: record version does not match requested version", method)
	}
	return call.method.Outputs.Pack(schedule.coefficients, schedule.authorityKind)
}

func init() {
	register("getCurrentMeteringSchedule", getCurrentMeteringScheduleHandler)
	register("getMeteringSchedule", getMeteringScheduleHandler)
}
