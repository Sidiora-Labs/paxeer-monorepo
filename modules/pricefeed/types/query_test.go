package types

import (
	"bytes"
	"compress/gzip"
	"io"
	"math"
	"reflect"
	"testing"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/api/annotations"
)

func TestQueryRequestsRoundTrip(t *testing.T) {
	params := DefaultParams(DefaultAuthority())
	validator := "paxvaloper1qyqszqgpqyqszqgpqyqszqgpqyqszqgpwfhxcu"
	feedAccount := "pax1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"
	price := QueryPriceResponse{
		Index: "65000500000000000000000", Mark: "65001000000000000000000",
		TimestampMs: 1700000000123, Height: 123456, Sources: 7, Fresh: true,
	}
	type wireMessage interface {
		gogoproto.Message
		Marshal() ([]byte, error)
		Unmarshal([]byte) error
		Size() int
	}
	messages := []wireMessage{
		&QueryParamsRequest{},
		&QueryParamsResponse{
			Authority: params.Authority, Markets: params.Markets,
			StalenessWindowMs: params.StalenessWindowMs, MaxClockSkewMs: params.MaxClockSkewMs,
			MinSources: params.MinSources, MaxDivergenceBps: params.MaxDivergenceBps,
			MaxDeviationBps:              params.MaxDeviationBps,
			MaxObservationsPerSubmission: params.MaxObservationsPerSubmission,
			MaxSubmissionBytes:           params.MaxSubmissionBytes, MarkEmaTauMs: params.MarkEmaTauMs,
			MaxMarkBasisBps: params.MaxMarkBasisBps, MaxPriceAgeMs: params.MaxPriceAgeMs,
		},
		&QueryPriceRequest{MarketId: math.MaxUint64},
		&price,
		&QueryPricesRequest{MarketIds: []uint64{1, 0, math.MaxUint64, 1}},
		&QueryPricesResponse{Prices: []QueryPriceResponse{
			price,
			{Index: "0", Mark: "0", TimestampMs: math.MaxUint64, Height: math.MinInt64, Sources: math.MaxUint32, Fresh: false},
		}},
		&QueryFeedKeyRequest{Validator: validator},
		&QueryFeedKeyResponse{PubKey: "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798", FeedAccount: feedAccount},
		&QueryValidatorByFeedAccountRequest{FeedAccount: feedAccount},
		&QueryValidatorByFeedAccountResponse{Validator: validator},
		&QueryObservationRequest{MarketId: 17, Validator: validator},
		&QueryObservationResponse{
			Sequence: math.MaxUint64, IndexPrice: price.Index, MarkPrice: price.Mark,
			SourceTimestampMs: price.TimestampMs, Height: math.MaxInt64,
		},
	}
	for _, message := range messages {
		t.Run(reflect.TypeOf(message).Elem().Name(), func(t *testing.T) {
			for _, original := range []wireMessage{message, reflect.New(reflect.TypeOf(message).Elem()).Interface().(wireMessage)} {
				encoded, err := original.Marshal()
				require.NoError(t, err)
				require.Len(t, encoded, original.Size())
				decoded := reflect.New(reflect.TypeOf(original).Elem()).Interface().(wireMessage)
				require.NoError(t, decoded.Unmarshal(encoded))
				require.Equal(t, original, decoded)
				again, err := decoded.Marshal()
				require.NoError(t, err)
				require.Equal(t, encoded, again)
			}
		})
	}

	compressed, _ := (&QueryParamsRequest{}).Descriptor()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	fileBytes, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var file descriptor.FileDescriptorProto
	require.NoError(t, proto.Unmarshal(fileBytes, &file))
	require.Equal(t, "paxprotocol.paxchain.pricefeed", file.GetPackage())
	require.Len(t, file.Service, 1)
	service := file.Service[0]
	require.Equal(t, "Query", service.GetName())
	routes := map[string]string{
		"Params":                 "/paxprotocol/paxchain/pricefeed/params",
		"Price":                  "/paxprotocol/paxchain/pricefeed/price/{market_id}",
		"Prices":                 "/paxprotocol/paxchain/pricefeed/prices",
		"FeedKey":                "/paxprotocol/paxchain/pricefeed/feed_key/{validator}",
		"ValidatorByFeedAccount": "/paxprotocol/paxchain/pricefeed/validator_by_feed_account/{feed_account}",
		"Observation":            "/paxprotocol/paxchain/pricefeed/observation/{market_id}/{validator}",
	}
	require.Len(t, service.Method, len(routes))
	require.Len(t, messages, 2*len(routes))
	for _, method := range service.Method {
		name := method.GetName()
		route, exists := routes[name]
		require.True(t, exists, "unexpected query %s", name)
		extension, err := proto.GetExtension(method.GetOptions(), annotations.E_Http)
		require.NoError(t, err)
		rule, ok := extension.(*annotations.HttpRule)
		require.True(t, ok)
		require.Equal(t, route, rule.GetGet())
		require.False(t, method.GetClientStreaming())
		require.False(t, method.GetServerStreaming())
		require.Equal(t, ".paxprotocol.paxchain.pricefeed.Query"+name+"Request", method.GetInputType())
		require.Equal(t, ".paxprotocol.paxchain.pricefeed.Query"+name+"Response", method.GetOutputType())
		delete(routes, name)
	}
	require.Empty(t, routes)
}
