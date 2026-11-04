package nullify_test

import (
	"encoding/json"
	"reflect"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/nullify"
)

func TestRepairM_774_ResponseRoundTrip(t *testing.T) {
	produced := sdk.TxResponse{
		Height: 42,
		Logs: sdk.ABCIMessageLogs{
			{MsgIndex: 1, Events: sdk.StringEvents{
				{Type: "transfer", Attributes: []sdk.Attribute{{Key: "amount", Value: "7upax"}}},
				{Type: "empty"},
			}},
			{MsgIndex: 2},
		},
	}
	wire, err := json.Marshal(produced)
	if err != nil {
		t.Fatal(err)
	}
	var consumed sdk.TxResponse
	if err := json.Unmarshal(wire, &consumed); err != nil {
		t.Fatal(err)
	}
	want := sdk.TxResponse{
		Height: 42,
		Logs: sdk.ABCIMessageLogs{
			{MsgIndex: 1, Events: sdk.StringEvents{
				{Type: "transfer", Attributes: []sdk.Attribute{{Key: "amount", Value: "7upax"}}},
				{Type: "empty", Attributes: []sdk.Attribute{}},
			}},
			{MsgIndex: 2, Events: sdk.StringEvents{}},
		},
		Events: []abci.Event{},
	}
	got := nullify.Fill(&consumed)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response payload or normalization changed: got %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(consumed, want) {
		t.Fatalf("response was not normalized in place: %#v", consumed)
	}
	if again := nullify.Fill(&consumed); !reflect.DeepEqual(again, want) {
		t.Fatalf("normalization is not idempotent: %#v", again)
	}
}

func TestRepairM_774_DifferentPayloadsRemainDistinct(t *testing.T) {
	cases := []struct {
		name  string
		left  interface{}
		right interface{}
	}{
		{"bytes", &struct{ Data []byte }{[]byte{1, 2}}, &struct{ Data []byte }{[]byte{1, 3}}},
		{"length", &struct{ Data []int }{[]int{1}}, &struct{ Data []int }{[]int{1, 2}}},
		{"order", &struct{ Data []string }{[]string{"a", "b"}}, &struct{ Data []string }{[]string{"b", "a"}}},
		{"nested", &sdk.ABCIMessageLog{Events: sdk.StringEvents{{Type: "transfer", Attributes: []sdk.Attribute{{Key: "amount", Value: "7upax"}}}}}, &sdk.ABCIMessageLog{Events: sdk.StringEvents{{Type: "transfer", Attributes: []sdk.Attribute{{Key: "amount", Value: "8upax"}}}}}},
		{"coins", &struct{ Coins sdk.Coins }{sdk.NewCoins(sdk.NewInt64Coin("upax", 7))}, &struct{ Coins sdk.Coins }{sdk.NewCoins(sdk.NewInt64Coin("upax", 8))}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			left, right := nullify.Fill(tc.left), nullify.Fill(tc.right)
			if reflect.DeepEqual(left, right) {
				t.Fatalf("different nonempty payloads compare equal: %#v and %#v", left, right)
			}
		})
	}
}

func TestRepairM_774_NestedSlicesAndPointers(t *testing.T) {
	type payload struct {
		Bytes [][]byte
		Logs  []*sdk.ABCIMessageLog
	}
	input := payload{
		Bytes: [][]byte{nil, {}, {1, 2}},
		Logs:  []*sdk.ABCIMessageLog{nil, {MsgIndex: 3}},
	}
	logPointer := input.Logs[1]
	want := payload{
		Bytes: [][]byte{{}, {}, {1, 2}},
		Logs:  []*sdk.ABCIMessageLog{nil, {MsgIndex: 3, Events: sdk.StringEvents{}}},
	}
	if got := nullify.Fill(&input); !reflect.DeepEqual(got, want) {
		t.Fatalf("nested slice elements were lost or not normalized: got %#v, want %#v", got, want)
	}
	if input.Logs[1] != logPointer {
		t.Fatal("normalization replaced a nonnil element pointer")
	}
	var nilSlice []byte
	if got := nullify.Fill(&nilSlice); !reflect.DeepEqual(got, []byte{}) || nilSlice == nil {
		t.Fatalf("nil root slice was not normalized: %#v", got)
	}
	root := [][]byte{nil, {9}}
	if got := nullify.Fill(root); !reflect.DeepEqual(got, [][]byte{{}, {9}}) {
		t.Fatalf("root slice payload was not retained: %#v", got)
	}
}
