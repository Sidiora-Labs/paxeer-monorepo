package decode

import (
	"encoding/json"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func deploymentBoundsLog(t *testing.T, event abi.Event, address common.Address, fields map[string]any) (ethtypes.Log, map[string]any) {
	t.Helper()
	log, want := syntheticLog(t, event, address)
	values := make([]any, 0)
	for _, input := range event.Inputs.NonIndexed() {
		value, ok := fields[input.Name]
		if !ok {
			value = sample(input.Type)
		}
		values = append(values, value)
		want[input.Name] = humanizeExact(value, input.Type)
	}
	var err error
	log.Data, err = event.Inputs.NonIndexed().Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	return log, want
}

func TestDeploymentLogBoundsTokenDeployed(t *testing.T) {
	d, a := deployment(t, "EventEmitter", common.HexToAddress("0x1"))
	r := registry(t, d)
	for _, tc := range []struct {
		name, tokenName, symbol string
		alerts                  []string
	}{
		{"invalid UTF8", string([]byte{0xff, 0xfe}), "COIN", []string{"withheld:name:invalid_utf8"}},
		{"zero character", "coin\x00name", "COIN", []string{"withheld:name:nul"}},
		{"long symbol", "Coin", strings.Repeat("s", MaxFieldBytes+1), []string{"withheld:symbol:max_field_bytes"}},
		{"long name", strings.Repeat("n", MaxFieldBytes+1), "COIN", []string{"withheld:name:max_field_bytes"}},
		{"empty text", "", "", nil},
		{"field limit", strings.Repeat("n", MaxFieldBytes), strings.Repeat("s", MaxFieldBytes), nil},
		{"valid UTF8", "Coin\n\u202e", "\u00e9", nil},
		{"both withheld", "coin\x00", string([]byte{0xff}), []string{"withheld:name:nul", "withheld:symbol:invalid_utf8"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log, want := deploymentBoundsLog(t, a.Events["TokenDeployed"], d.Address, map[string]any{"name": tc.tokenName, "symbol": tc.symbol})
			for _, alert := range tc.alerts {
				want[strings.Split(alert, ":")[1]] = ""
			}
			env, err := r.Decode(log)
			if err != nil || env.Status != valueDecoded || env.Event == nil || env.FailureReason != "" || env.RawSummary != nil || !reflect.DeepEqual(env.Alerts, tc.alerts) || !reflect.DeepEqual(env.Raw, rawLog(log)) || !reflect.DeepEqual(env.Event.Args, want) || !reflect.DeepEqual(env.Family.Fields, want) {
				t.Fatalf("token fields: %+v: %v", env, err)
			}
			if _, err := json.Marshal(env); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeploymentLogBoundsCatalogFields(t *testing.T) {
	for _, tc := range []struct {
		role, event, field string
		value              any
		cause              string
	}{
		{"EventEmitter", "EventLog", "eventName", strings.Repeat("e", MaxFieldBytes+1), "max_field_bytes"},
		{"EventEmitter", "EventLog1", "eventName", "event\x00", "nul"},
		{"EventEmitter", "EventLog2", "eventName", string([]byte{0xff}), "invalid_utf8"},
		{"EventEmitter", "Governance", "payload", make([]byte, MaxFieldBytes+1), "max_field_bytes"},
		{"EventEmitter", "OpticalExecuted", "hookName", string([]byte{0xff}), "invalid_utf8"},
		{"EventEmitter", "OpticalExecuted", "data", make([]byte, MaxFieldBytes+1), "max_field_bytes"},
		{"EventEmitter", "OpticalLifecycle", "payload", make([]byte, MaxFieldBytes+1), "max_field_bytes"},
		{"EventEmitter", "PecorOrderLifecycle", "payload", make([]byte, MaxFieldBytes+1), "max_field_bytes"},
		{"MetaAGRouter", "AdapterRegistered", "name", strings.Repeat("a", MaxFieldBytes+1), "max_field_bytes"},
	} {
		t.Run(tc.role+"/"+tc.event+"/"+tc.field, func(t *testing.T) {
			d, a := deployment(t, tc.role, common.HexToAddress("0x1"))
			d.Enabled, d.Optional = true, tc.role == "MetaAGRouter"
			d.EnabledOptionalEvents = []string{tc.event}
			r := registry(t, d)
			log, want := deploymentBoundsLog(t, a.Events[tc.event], d.Address, map[string]any{tc.field: tc.value})
			want[tc.field] = ""
			if _, ok := tc.value.([]byte); ok {
				want[tc.field] = "0x"
			}
			env, err := r.Decode(log)
			if err != nil || env.Status != valueDecoded || env.Event == nil || !reflect.DeepEqual(env.Event.Args, want) || !reflect.DeepEqual(env.Raw, rawLog(log)) || !reflect.DeepEqual(env.Alerts, []string{"withheld:" + tc.field + ":" + tc.cause}) {
				t.Fatalf("catalog fields: %+v: %v", env, err)
			}
		})
	}
}

func TestDeploymentLogBoundsNestedArguments(t *testing.T) {
	d, a := deployment(t, "EventEmitter", common.HexToAddress("0x1"))
	r := registry(t, d)
	for _, name := range []string{"EventLog", "EventLog1", "EventLog2"} {
		t.Run(name, func(t *testing.T) {
			event := a.Events[name]
			var typ abi.Type
			for _, input := range event.Inputs {
				if input.Name == "eventData" {
					typ = input.Type
				}
			}
			value := reflect.ValueOf(sample(typ))
			items := value.FieldByName("StringItems").FieldByName("Items")
			stringsValue := reflect.MakeSlice(items.Type(), 2, 2)
			stringsValue.Index(0).FieldByName("Key").SetString("display")
			stringsValue.Index(0).FieldByName("Value").SetString("coin\x00")
			stringsValue.Index(1).FieldByName("Key").SetString(string([]byte{0xff}))
			stringsValue.Index(1).FieldByName("Value").SetString("kept")
			// ABI tuple values are copied so their slice fields can be populated.
			data := reflect.New(value.Type()).Elem()
			data.Set(value)
			data.FieldByName("StringItems").FieldByName("Items").Set(stringsValue)
			arrays := data.FieldByName("StringItems").FieldByName("ArrayItems")
			arrays.Set(reflect.MakeSlice(arrays.Type(), 1, 1))
			arrays.Index(0).FieldByName("Key").SetString("labels")
			arrays.Index(0).FieldByName("Value").Set(reflect.ValueOf([]string{"kept", strings.Repeat("x", MaxFieldBytes+1)}))
			bytesItems := data.FieldByName("BytesItems").FieldByName("ArrayItems")
			bytesItems.Set(reflect.MakeSlice(bytesItems.Type(), 1, 1))
			bytesItems.Index(0).FieldByName("Key").SetString("content")
			bytesItems.Index(0).FieldByName("Value").Set(reflect.ValueOf([][]byte{{1, 2}, make([]byte, MaxFieldBytes+1)}))
			log, _ := deploymentBoundsLog(t, event, d.Address, map[string]any{"eventData": data.Interface()})
			stringsValue.Index(0).FieldByName("Value").SetString("")
			stringsValue.Index(1).FieldByName("Key").SetString("")
			arrays.Index(0).FieldByName("Value").Set(reflect.ValueOf([]string{"kept", ""}))
			bytesItems.Index(0).FieldByName("Value").Set(reflect.ValueOf([][]byte{{1, 2}, nil}))
			want := humanizeExact(data.Interface(), typ)
			alerts := []string{"withheld:eventData.stringItems.items[0].value:nul", "withheld:eventData.stringItems.items[1].key:invalid_utf8", "withheld:eventData.stringItems.arrayItems[0].value[1]:max_field_bytes", "withheld:eventData.bytesItems.arrayItems[0].value[1]:max_field_bytes"}
			sort.Strings(alerts)
			env, err := r.Decode(log)
			if err != nil || env.Status != valueDecoded || env.Event == nil || !reflect.DeepEqual(env.Event.Args["eventData"], want) || !reflect.DeepEqual(env.Alerts, alerts) || !reflect.DeepEqual(env.Raw, rawLog(log)) {
				t.Fatalf("nested fields: %+v: %v", env, err)
			}
		})
	}
	array := mustType("string[2][]")
	event := abi.NewEvent("TextArrays", "TextArrays", false, abi.Arguments{{Name: "labels", Type: array}})
	data, err := event.Inputs.Pack([][2]string{{"kept", "zero\x00"}, {string([]byte{0xff}), ""}})
	if err != nil {
		t.Fatal(err)
	}
	var alerts []string
	args, _, err := exactArgs(event, []common.Hash{event.ID}, data, nil, &alerts)
	if err != nil || !reflect.DeepEqual(args["labels"], []any{[]any{"kept", ""}, []any{"", ""}}) || !reflect.DeepEqual(alerts, []string{"withheld:labels[0][1]:nul", "withheld:labels[1][0]:invalid_utf8"}) {
		t.Fatalf("fixed arrays: %+v %+v: %v", args, alerts, err)
	}
}

func TestDeploymentLogBoundsRawAndPayload(t *testing.T) {
	d, a := deployment(t, "EventEmitter", common.HexToAddress("0x1"))
	d.EnabledOptionalEvents = []string{"BestRouteSwap"}
	r := registry(t, d)
	base, _ := syntheticLog(t, a.Events["TokenDeployed"], d.Address)
	for _, tc := range []struct {
		name string
		log  ethtypes.Log
	}{
		{"raw data", func() ethtypes.Log { l := base; l.Data = make([]byte, MaxEnvelopeBytes+1); return l }()},
		{"topic count", func() ethtypes.Log { l := base; l.Topics = make([]common.Hash, 5); return l }()},
		{"raw total", func() ethtypes.Log { l := base; l.Data = make([]byte, MaxEnvelopeBytes); return l }()},
		{"array payload", func() ethtypes.Log {
			hops := make([]common.Address, 24000)
			for i := range hops {
				hops[i] = d.Address
			}
			l, _ := deploymentBoundsLog(t, a.Events["BestRouteSwap"], d.Address, map[string]any{"hops": hops})
			if !rawSizeAllowed(l) {
				t.Fatal("array must fit raw admission")
			}
			return l
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, err := r.Decode(tc.log)
			if err != nil || env.Status != valueQuarantined || env.FailureReason != "bounds" || env.Event != nil || env.Family != nil || env.Operational != nil || !reflect.DeepEqual(env.Raw, rawMetadata(tc.log)) || !reflect.DeepEqual(env.RawSummary, summarizeRaw(tc.log)) {
				t.Fatalf("bounded envelope: %+v: %v", env, err)
			}
			encoded, err := json.Marshal(env)
			if err != nil || len(encoded) > 2048 {
				t.Fatalf("summary bytes=%d: %v", len(encoded), err)
			}
		})
	}
	log, _ := deploymentBoundsLog(t, a.Events["OpticalExecuted"], d.Address, map[string]any{"hookName": string([]byte{0xff}), "data": make([]byte, MaxFieldBytes+1), "timestamp": big.NewInt(1)})
	log.Data = append(log.Data, make([]byte, 32)...)
	env, err := r.Decode(log)
	if err == nil || env.Status != valueQuarantined || env.FailureReason != scalarOffsetFailure || env.Event != nil || len(env.Alerts) != 0 || !reflect.DeepEqual(env.Raw, rawLog(log)) {
		t.Fatalf("canonical bytes required before withholding: %+v: %v", env, err)
	}
}
