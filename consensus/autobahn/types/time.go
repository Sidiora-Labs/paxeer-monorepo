package types

import (
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/pb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/protoutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
)

const (
	minTimestampSeconds int64 = -62135596800
	maxTimestampSeconds int64 = 253402300799
)

func validateTimestamp(seconds int64, nanos int32) error {
	if seconds < minTimestampSeconds || seconds > maxTimestampSeconds {
		return errors.New("seconds: outside supported timestamp range")
	}
	if nanos < 0 || nanos >= 1e9 {
		return errors.New("nanos: outside canonical timestamp range")
	}
	return nil
}

// TimeConv is the protobuf converter for time.Time.
var TimeConv = protoutils.Conv[time.Time, *pb.Timestamp]{
	// implementation based on
	// https://github.com/pbbuffers/protobuf-go/blob/v1.36.5/types/known/timestamppb/timestamp.pb.go
	Encode: func(t time.Time) *pb.Timestamp {
		if err := validateTimestamp(t.Unix(), int32(t.Nanosecond())); err != nil {
			panic(err)
		}
		return &pb.Timestamp{
			Seconds: proto.Int64(t.Unix()),
			Nanos:   proto.Int32(int32(t.Nanosecond())),
		}
	},
	Decode: func(p *pb.Timestamp) (time.Time, error) {
		if p.Seconds == nil {
			return utils.Zero[time.Time](), errors.New("seconds: missing")
		}
		if p.Nanos == nil {
			return utils.Zero[time.Time](), errors.New("nanos: missing")
		}
		if err := validateTimestamp(*p.Seconds, *p.Nanos); err != nil {
			return utils.Zero[time.Time](), err
		}
		return time.Unix(*p.Seconds, int64(*p.Nanos)).UTC(), nil
	},
}

// DurationConv is the protobuf converter for time.Duration.
var DurationConv = protoutils.Conv[time.Duration, *pb.Duration]{
	// implementation based on
	// https://github.com/pbbuffers/protobuf-go/blob/v1.36.5/types/known/durationpb/duration.pb.go
	Encode: func(d time.Duration) *pb.Duration {
		nanos := d.Nanoseconds()
		secs := nanos / 1e9
		nanos -= secs * 1e9
		return &pb.Duration{
			Seconds: proto.Int64(secs),
			Nanos:   proto.Int32(int32(nanos)),
		}
	},
	Decode: func(p *pb.Duration) (time.Duration, error) {
		if p.Seconds == nil {
			return utils.Zero[time.Duration](), errors.New("seconds: missing")
		}
		if p.Nanos == nil {
			return utils.Zero[time.Duration](), errors.New("nanos: missing")
		}
		secs := *p.Seconds
		nanos := *p.Nanos
		d := time.Duration(secs) * time.Second
		overflow := d/time.Second != time.Duration(secs)
		d += time.Duration(nanos) * time.Nanosecond
		overflow = overflow || (secs < 0 && nanos < 0 && d > 0)
		overflow = overflow || (secs > 0 && nanos > 0 && d < 0)
		if overflow {
			return utils.Zero[time.Duration](), errors.New("overflow")
		}
		return d, nil
	},
}
