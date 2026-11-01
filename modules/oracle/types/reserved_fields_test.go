package types

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/stretchr/testify/require"
)

func TestOracleDescriptorsReserveRemovedFields(t *testing.T) {
	paramsDescriptor, paramsPath := (&Params{}).Descriptor()
	voteDescriptor, votePath := (&MsgAggregateExchangeRateVote{}).Descriptor()

	for _, tc := range []struct {
		name          string
		file          string
		compressed    []byte
		path          []int
		reserved      int32
		reservedNames []string
		fields        map[string]int32
	}{
		{
			name:       "Params",
			file:       "oracle/oracle.proto",
			compressed: paramsDescriptor,
			path:       paramsPath,
			reserved:   8,
			fields: map[string]int32{
				"vote_period": 1, "vote_threshold": 2, "reward_band": 3,
				"whitelist": 4, "slash_fraction": 5, "slash_window": 6,
				"min_valid_per_window": 7, "lookback_duration": 9,
			},
		},
		{
			name:          "MsgAggregateExchangeRateVote",
			file:          "oracle/tx.proto",
			compressed:    voteDescriptor,
			path:          votePath,
			reserved:      1,
			reservedNames: []string{"salt"},
			fields: map[string]int32{
				"exchange_rates": 2, "feeder": 3, "validator": 4,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.compressed, proto.FileDescriptor(tc.file))
			reader, err := gzip.NewReader(bytes.NewReader(tc.compressed))
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			var file descriptor.FileDescriptorProto
			require.NoError(t, proto.Unmarshal(data, &file))
			require.Equal(t, tc.file, file.GetName())
			require.Equal(t, "paxprotocol.paxchain.oracle", file.GetPackage())
			require.Len(t, tc.path, 1)
			require.GreaterOrEqual(t, tc.path[0], 0)
			require.Less(t, tc.path[0], len(file.GetMessageType()))
			message := file.GetMessageType()[tc.path[0]]
			require.Equal(t, tc.name, message.GetName())
			ranges := message.GetReservedRange()
			require.Len(t, ranges, 1)
			require.Equal(t, tc.reserved, ranges[0].GetStart())
			require.Equal(t, tc.reserved+1, ranges[0].GetEnd())
			require.Equal(t, tc.reservedNames, message.GetReservedName())
			fields := make(map[string]int32)
			for _, field := range message.GetField() {
				require.NotEqual(t, tc.reserved, field.GetNumber())
				for _, name := range tc.reservedNames {
					require.NotEqual(t, name, field.GetName())
				}
				fields[field.GetName()] = field.GetNumber()
			}
			require.Equal(t, tc.fields, fields)
		})
	}
}
