package decode

import (
	"context"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestMeasurementABIWidthsAndArchivedScalarBounds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		components []abi.ArgumentMarshaling
		width      int
		dynamic    bool
	}{
		{"uint256", nil, 1, false}, {"string", nil, 1, true}, {"bytes", nil, 1, true}, {"uint256[]", nil, 1, true},
		{"uint256[3]", nil, 3, false}, {"string[3]", nil, 1, true},
		{"tuple", []abi.ArgumentMarshaling{{Name: "n", Type: "uint256"}, {Name: "b", Type: "bool"}}, 2, false},
		{"tuple", []abi.ArgumentMarshaling{{Name: "s", Type: "string"}}, 1, true},
	} {
		typ, err := abi.NewType(tc.name, "", tc.components)
		if err != nil {
			t.Fatal(err)
		}
		width, dynamic := abiHeadWords(typ)
		if width != tc.width || dynamic != tc.dynamic {
			t.Fatalf("ABI width %s: %d/%v", tc.name, width, dynamic)
		}
	}
	parsed, err := abi.JSON(strings.NewReader(`[{"type":"event","name":"Bounds","inputs":[{"name":"n","type":"uint256"},{"name":"i","type":"int256"},{"name":"s","type":"string"},{"name":"b","type":"bytes"},{"name":"list","type":"uint256[]"},{"name":"tuple","type":"tuple","components":[{"name":"n","type":"uint256"}]}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	umax := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
	imin := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 255)).String()
	args := map[string]any{"n": umax, "i": imin, "s": strings.Repeat("x", MaxFieldBytes), "b": "0x" + strings.Repeat("ff", MaxFieldBytes), "list": []any{umax}, "tuple": map[string]any{"n": umax}}
	u, i, n, f := archivedScalarCases(parsed.Events["Bounds"], args, umax, imin)
	if !u || !i || !n || !f {
		t.Fatalf("exact scalar cases %v %v %v %v", u, i, n, f)
	}
	u, i, n, f = archivedScalarCases(parsed.Events["Bounds"], map[string]any{"n": 17, "i": "1"}, umax, imin)
	if u || i || n || f {
		t.Fatal("ordinary or noncanonical values reported as boundary evidence")
	}
	for _, tc := range []struct {
		value any
		over  bool
	}{
		{nil, false}, {(*string)(nil), false}, {strings.Repeat("x", MaxFieldBytes), false}, {strings.Repeat("x", MaxFieldBytes+1), true},
		{make([]byte, MaxFieldBytes+1), true}, {[]string{strings.Repeat("x", MaxFieldBytes+1)}, true}, {[1]string{"small"}, false},
		{struct{ Payload string }{strings.Repeat("x", MaxFieldBytes+1)}, true},
	} {
		if got := overSizedValue(reflect.ValueOf(tc.value)); got != tc.over {
			t.Fatalf("archive field %T oversized=%v", tc.value, got)
		}
	}
}

func TestBooleanMeasurementsRetainApprovedNonzeroWords(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(`[{"type":"event","name":"Flags","inputs":[{"name":"first","type":"bool","indexed":true},{"name":"second","type":"bool"},{"name":"ignored","type":"bool"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	for _, number := range []uint64{0, 1, 2, 3} {
		word := common.BigToHash(new(big.Int).SetUint64(number))
		log := types.Log{Topics: []common.Hash{parsed.Events["Flags"].ID, word}, Data: append(append([]byte{}, word.Bytes()...), word.Bytes()...)}
		got := map[uint64]bool{}
		collectBooleanWords(parsed.Events["Flags"], log, []string{"first", "second"}, got)
		if len(got) != boolCount(number <= 2) || number <= 2 && !got[number] {
			t.Fatalf("boolean word %d: %v", number, got)
		}
	}
	got := map[uint64]bool{}
	collectBooleanWords(parsed.Events["Flags"], types.Log{}, []string{"first", "second"}, got)
	if len(got) != 0 {
		t.Fatal("truncated boolean evidence was invented")
	}
}
func boolCount(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestDecoderBurstRetainsTypedOpaqueAndQuarantinedResultsOnCancellation(t *testing.T) {
	d, a := deployment(t, "SidioraERC20", common.HexToAddress("0x125"))
	r := registry(t, d)
	typed, _ := syntheticLog(t, a.Events["Transfer"], d.Address)
	opaque := typed
	opaque.Index++
	opaque.Topics = []common.Hash{common.HexToHash("0xfeed")}
	quarantined := typed
	quarantined.Index += 2
	quarantined.Data = []byte{1}
	records := []ArchivedDecoderLog{{Log: typed}, {Log: opaque}, {Log: quarantined}}
	expected := make([]*DecodedEnvelope, len(records))
	for i, record := range records {
		env, err := r.Decode(record.Log)
		if env == nil || i == 0 && err != nil || i == 1 && env.Status != "opaque" || i == 2 && env.Status != "quarantined" {
			t.Fatalf("decoder workload %d: %+v %v", i, env, err)
		}
		expected[i] = env
	}
	run := prepareDecoderBurst(r, records, expected, DecoderMeasurementInput{Workers: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	report, err := run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || report.Decoded == 0 || report.PreservedOpaque == 0 || report.PreservedQuarantined == 0 || report.IdentityFailures != 0 || report.DuplicateDeliveries != 0 || report.Completed != report.Decoded+report.PreservedOpaque+report.PreservedQuarantined || report.Completed+report.MissingDeliveries != DecoderBurstRate*DecoderBurstSeconds || len(report.Memory) < 3 || report.LargestPayloadBytes != len(typed.Data) || report.LargestPayloadDeliveries == 0 || report.LargestPayloadDecodeP99Nanoseconds < 0 {
		t.Fatalf("cancelled burst: %+v %v", report, err)
	}
}

func TestArchiveFieldInspectionAndResultBindings(t *testing.T) {
	d, a := deployment(t, "EventEmitter", common.HexToAddress("0x125"))
	r := registry(t, d)
	var dynamic abi.Event
	for _, event := range a.Events {
		for _, arg := range event.Inputs.NonIndexed() {
			if arg.Type.T == abi.StringTy {
				dynamic = event
				break
			}
		}
		if dynamic.Name != "" {
			break
		}
	}
	if dynamic.Name == "" {
		t.Fatal("dynamic emitter ABI absent")
	}
	log, _ := syntheticLog(t, dynamic, d.Address)
	values, err := dynamic.Inputs.NonIndexed().Unpack(log.Data)
	if err != nil {
		t.Fatal(err)
	}
	for i, arg := range dynamic.Inputs.NonIndexed() {
		if arg.Type.T == abi.StringTy {
			values[i] = strings.Repeat("x", MaxFieldBytes+1)
			break
		}
	}
	log.Data, err = dynamic.Inputs.NonIndexed().Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	if !oversizedArchivedField(r, log) {
		t.Fatal("oversized archived field absent")
	}
	for _, changed := range []types.Log{{}, {Address: d.Address, Topics: []common.Hash{common.HexToHash("0xfeed")}}, {Address: d.Address, Topics: log.Topics, Data: []byte{1}}} {
		if oversizedArchivedField(r, changed) {
			t.Fatal("invalid ABI bytes reported as decoded oversized field")
		}
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err = os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = unchangedMeasurementFile(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("changed measurement input accepted")
	}
	if err = unchangedMeasurementFile(path+".absent", strings.Repeat("0", 64)); err == nil {
		t.Fatal("missing measurement input accepted")
	}
	if got := measurementPercentile([]int64{9, 1, 3}); got != 9 {
		t.Fatalf("nearest-rank p99 %d", got)
	}
	if got := measurementPercentile(nil); got != 0 {
		t.Fatalf("empty p99 %d", got)
	}
}

func TestDecoderBurstConservesCompleteFiveMinuteWorkload(t *testing.T) {
	d, a := deployment(t, "SidioraERC20", common.HexToAddress("0x125"))
	r := registry(t, d)
	log, _ := syntheticLog(t, a.Events["Transfer"], d.Address)
	envelope, err := r.Decode(log)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	const rate = DecoderBurstRate / decoderCompleteWorkloadRateDivisor
	report, err := prepareDecoderBurstAtRate(r, []ArchivedDecoderLog{{Log: log}}, []*DecodedEnvelope{envelope}, DecoderMeasurementInput{Workers: 4, MaxP99Nanoseconds: int64(50 * time.Millisecond), MaxHeapGrowthBytes: 512 << 20, MaxAllocatedBytesPerDelivery: 1 << 20}, rate)(ctx)
	if err != nil {
		t.Fatalf("complete decoder workload %v: scheduled=%d enqueued=%d completed=%d drops=%d missing=%d p99=%d", err, report.Scheduled, report.Enqueued, report.Completed, report.AdmissionDrops, report.MissingDeliveries, report.EndToEndP99Nanoseconds)
	}
	count := uint64(rate * DecoderBurstSeconds)
	if report.Scheduled != count || report.Enqueued != count || report.Completed != count || report.Decoded != count || report.MissingDeliveries != 0 || report.DuplicateDeliveries != 0 || report.IdentityFailures != 0 || report.InputIdentitySHA256 != report.ObservedIdentitySHA256 || report.DurationNanoseconds < int64(5*time.Minute) {
		t.Fatalf("delivery conservation %+v", report)
	}
}
