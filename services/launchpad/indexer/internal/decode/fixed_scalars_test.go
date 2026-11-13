package decode

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestFixedScalarCanonicalDecodingMatchesABIValidation(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(`[{"type":"event","name":"Scalars","inputs":[{"name":"from","type":"address","indexed":true},{"name":"sequence","type":"uint256","indexed":true},{"name":"to","type":"address"},{"name":"amount","type":"uint256"}]},{"type":"event","name":"Indexed","inputs":[{"name":"from","type":"address","indexed":true}]},{"type":"event","name":"Empty","inputs":[]},{"type":"event","name":"Flag","inputs":[{"name":"flag","type":"bool"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	event := parsed.Events["Scalars"]
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	address := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	data, err := event.Inputs.NonIndexed().Pack(address, maximum)
	if err != nil {
		t.Fatal(err)
	}
	topics := []common.Hash{event.ID, common.BytesToHash(address[:]), common.BigToHash(maximum)}
	badTopics := append([]common.Hash(nil), topics...)
	badTopics[1][0] = 1
	badData := append([]byte(nil), data...)
	badData[0] = 1
	for _, tc := range []struct {
		name     string
		event    abi.Event
		topics   []common.Hash
		data     []byte
		approved []string
		valid    bool
	}{
		{"maximum", event, topics, data, nil, true},
		{"zero", event, []common.Hash{event.ID, {}, {}}, make([]byte, 64), nil, true},
		{"missing_topic", event, topics[:2], data, nil, false},
		{"extra_topic", event, append(append([]common.Hash(nil), topics...), common.Hash{}), data, nil, false},
		{"indexed_padding", event, badTopics, data, nil, false},
		{"data_padding", event, topics, badData, nil, false},
		{"short_data", event, topics, data[:63], nil, false},
		{"extra_data", event, topics, append(append([]byte(nil), data...), 0), nil, false},
		{"indexed_only", parsed.Events["Indexed"], topics[:2], nil, nil, true},
		{"indexed_with_data", parsed.Events["Indexed"], topics[:2], []byte{0}, nil, false},
		{"empty", parsed.Events["Empty"], topics[:1], nil, nil, true},
		{"strict_bool", parsed.Events["Flag"], topics[:1], common.BigToHash(big.NewInt(2)).Bytes(), nil, false},
		{"approved_bool", parsed.Events["Flag"], topics[:1], common.BigToHash(big.NewInt(2)).Bytes(), []string{"flag"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, reason, gotErr := exactArgs(tc.event, tc.topics, tc.data, tc.approved)
			want, wantReason, wantErr := exactGeneralArgs(tc.event, tc.topics, tc.data, tc.approved)
			if (gotErr == nil) != tc.valid || (gotErr == nil) != (wantErr == nil) || reason != wantReason || !reflect.DeepEqual(got, want) {
				t.Fatalf("canonical values %v/%s/%v; ABI values %v/%s/%v", got, reason, gotErr, want, wantReason, wantErr)
			}
			if tc.name == "maximum" && (got["amount"] != maximum.String() || got["sequence"] != maximum.String() || got["from"] != strings.ToLower(address.Hex()) || got["to"] != strings.ToLower(address.Hex())) {
				t.Fatalf("scalar values %v", got)
			}
		})
	}
}

func TestDecodedPayloadSizeProofPreservesJSONEnvelopeLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  map[string]any
		valid bool
	}{
		{"small_strings", map[string]any{"from": "0x125", "amount": "123"}, true},
		{"escaped_strings", map[string]any{"<\"": "\x00\xff"}, true},
		{"large_string", map[string]any{"v": strings.Repeat("x", MaxEnvelopeBytes-8)}, true},
		{"oversized_string", map[string]any{"v": strings.Repeat("x", MaxEnvelopeBytes-7)}, false},
		{"large_name", map[string]any{strings.Repeat("x", MaxEnvelopeBytes-7): ""}, true},
		{"oversized_name", map[string]any{strings.Repeat("x", MaxEnvelopeBytes-6): ""}, false},
		{"escaped_limit", map[string]any{"v": strings.Repeat("\x00", MaxEnvelopeBytes/6)}, false},
		{"integer", map[string]any{"v": 10}, true},
		{"unencodable", map[string]any{"v": make(chan int)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, marshalErr := json.Marshal(tc.args)
			want := marshalErr == nil && len(body) <= MaxEnvelopeBytes
			gotErr := validateDecodedPayload(tc.args)
			if want != tc.valid || (gotErr == nil) != want {
				t.Fatalf("payload acceptance %v; JSON size=%d error=%v; expected=%v", gotErr, len(body), marshalErr, tc.valid)
			}
		})
	}
}
