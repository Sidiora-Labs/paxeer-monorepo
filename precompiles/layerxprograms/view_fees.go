package layerxprograms

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type feeScheduleHistoryReader interface {
	ReadFeeScheduleAt(sdk.Context, uint32) ([]byte, error)
}

type feeScheduleRecord struct {
	Version                 uint32
	Prices                  [7]uint64
	Asset                   [32]byte
	Demand                  [6]uint64
	ActivationBatch         uint64
	LastOccupancyBatch      uint64
	GovernanceSequence      uint64
	GovernanceReceiptDigest [32]byte
	ObservedOccupancy       [16]byte
}

type pendingFeeScheduleRecord struct {
	Prices                  [7]uint64
	Asset                   [32]byte
	Demand                  [6]uint64
	ActivationBatch         uint64
	StagedBatch             uint64
	GovernanceSequence      uint64
	GovernanceReceiptDigest [32]byte
}

func feeScheduleFieldsValid(prices [7]uint64, asset [32]byte, demand [6]uint64, governanceSequence uint64, digest [32]byte) bool {
	for _, price := range prices {
		if price == 0 {
			return false
		}
	}
	for _, value := range demand {
		if value == 0 {
			return false
		}
	}
	return demand[2] <= demand[3] && demand[4] <= demand[5] &&
		prices[6] >= demand[4] && prices[6] <= demand[5] &&
		asset != ([32]byte{}) && governanceSequence != 0 && digest != ([32]byte{})
}

func decodeFeeScheduleRecord(encoded []byte) (feeScheduleRecord, error) {
	var out feeScheduleRecord
	if len(encoded) != 217 || string(encoded[:5]) != "LXFR1" {
		return out, lxphost.Code(-3)
	}
	out.Version = binary.BigEndian.Uint32(encoded[5:9])
	for i := range out.Prices {
		out.Prices[i] = binary.BigEndian.Uint64(encoded[9+i*8 : 17+i*8])
	}
	copy(out.Asset[:], encoded[65:97])
	for i := range out.Demand {
		out.Demand[i] = binary.BigEndian.Uint64(encoded[97+i*8 : 105+i*8])
	}
	out.ActivationBatch = binary.BigEndian.Uint64(encoded[145:153])
	out.LastOccupancyBatch = binary.BigEndian.Uint64(encoded[153:161])
	out.GovernanceSequence = binary.BigEndian.Uint64(encoded[161:169])
	copy(out.GovernanceReceiptDigest[:], encoded[169:201])
	copy(out.ObservedOccupancy[:], encoded[201:217])
	if out.Version == 0 || out.ActivationBatch == 0 || out.LastOccupancyBatch == math.MaxUint64 ||
		!feeScheduleFieldsValid(out.Prices, out.Asset, out.Demand, out.GovernanceSequence, out.GovernanceReceiptDigest) {
		return feeScheduleRecord{}, lxphost.Code(-1002)
	}
	return out, nil
}

func decodePendingFeeScheduleRecord(encoded []byte) (pendingFeeScheduleRecord, error) {
	var out pendingFeeScheduleRecord
	if len(encoded) != 197 || string(encoded[:5]) != "LXFP1" {
		return out, lxphost.Code(-3)
	}
	for i := range out.Prices {
		out.Prices[i] = binary.BigEndian.Uint64(encoded[5+i*8 : 13+i*8])
	}
	copy(out.Asset[:], encoded[61:93])
	for i := range out.Demand {
		out.Demand[i] = binary.BigEndian.Uint64(encoded[93+i*8 : 101+i*8])
	}
	out.ActivationBatch = binary.BigEndian.Uint64(encoded[141:149])
	out.StagedBatch = binary.BigEndian.Uint64(encoded[149:157])
	out.GovernanceSequence = binary.BigEndian.Uint64(encoded[157:165])
	copy(out.GovernanceReceiptDigest[:], encoded[165:197])
	if out.StagedBatch == 0 || out.ActivationBatch <= out.StagedBatch ||
		!feeScheduleFieldsValid(out.Prices, out.Asset, out.Demand, out.GovernanceSequence, out.GovernanceReceiptDigest) {
		return pendingFeeScheduleRecord{}, lxphost.Code(-1002)
	}
	return out, nil
}

func feeScheduleReadError(err error, method string) error {
	if errors.Is(err, lxphost.Code(-7)) || errors.Is(err, lxphost.Code(-101)) {
		return fmt.Errorf("LXP_NOT_FOUND: %s: %w", method, err)
	}
	var code lxphost.Code
	if errors.As(err, &code) {
		return kernelError(code, method)
	}
	return fmt.Errorf("%s: %w", method, err)
}

func getCurrentFeeScheduleHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	const method = "getCurrentFeeSchedule"
	if _, err := decodeArgs(call.method, args); err != nil {
		return nil, err
	}
	encoded, err := p.host.ReadFeeSchedule(ctx)
	if err != nil {
		return nil, feeScheduleReadError(err, method)
	}
	record, err := decodeFeeScheduleRecord(encoded)
	if err != nil {
		return nil, feeScheduleReadError(err, method)
	}
	return call.method.Outputs.Pack(record.Version, record.Prices, record.Asset)
}

func getFeeScheduleHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	const method = "getFeeSchedule"
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	version, ok := decoded[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: expected uint32 fee schedule version, got %T", decoded[0])
	}
	if version == 0 {
		return nil, kernelError(lxphost.Code(-3), method)
	}
	encoded, err := p.host.ReadFeeScheduleAt(ctx, version)
	if err != nil {
		return nil, feeScheduleReadError(err, method)
	}
	record, err := decodeFeeScheduleRecord(encoded)
	if err != nil {
		return nil, feeScheduleReadError(err, method)
	}
	if record.Version != version {
		return nil, kernelError(lxphost.Code(-1002), method)
	}
	return call.method.Outputs.Pack(record.Prices, record.Asset)
}

func getPendingFeeScheduleHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	const method = "getPendingFeeSchedule"
	if _, err := decodeArgs(call.method, args); err != nil {
		return nil, err
	}
	encoded, err := p.host.ReadPendingFeeSchedule(ctx)
	if err != nil {
		if errors.Is(err, lxphost.Code(-7)) {
			return nil, fmt.Errorf("LXP_NOT_FOUND: no pending fee schedule: %w", err)
		}
		return nil, feeScheduleReadError(err, method)
	}
	record, err := decodePendingFeeScheduleRecord(encoded)
	if err != nil {
		return nil, feeScheduleReadError(err, method)
	}
	return call.method.Outputs.Pack(record.Prices, record.Asset, record.Demand, record.ActivationBatch)
}

func init() {
	register("getCurrentFeeSchedule", getCurrentFeeScheduleHandler)
	register("getFeeSchedule", getFeeScheduleHandler)
	register("getPendingFeeSchedule", getPendingFeeScheduleHandler)
}
