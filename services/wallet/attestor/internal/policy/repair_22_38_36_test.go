package policy

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
)

func roomProgramRule() ProgramCallRule {
	program := [32]byte{0x36}
	return ProgramCallRule{ProgramID: hex.EncodeToString(program[:]), Entrypoints: []string{"layerx_call"}, FeeLimit: "1000"}
}

func roomProgramData(t *testing.T, program [32]byte, entrypoint string, fee int64) []byte {
	t.Helper()
	return repair244Pack(t, "layerxprograms", "call", program, uint16(5), entrypoint,
		[]byte{1}, []byte{}, []byte{}, uint32(16384), [7]uint64{1}, big.NewInt(fee))
}

func TestRoomProgramCallPolicy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		program     [32]byte
		entrypoint  string
		fee, value  int64
		withoutRule bool
		code        string
	}{
		{"listed_at_fee_limit", [32]byte{0x36}, "layerx_call", 1000, 0, false, CodeAllowed},
		{"unlisted_program", [32]byte{0x37}, "layerx_call", 1000, 0, false, CodeSelectorNotAllowed},
		{"unlisted_entrypoint", [32]byte{0x36}, "other", 1000, 0, false, CodeSelectorNotAllowed},
		{"fee_above_limit", [32]byte{0x36}, "layerx_call", 1001, 0, false, CodeSelectorNotAllowed},
		{"nonpayable_value", [32]byte{0x36}, "layerx_call", 1000, 1, false, CodeDecodeError},
		{"without_program_calls", [32]byte{0x36}, "layerx_call", 1000, 0, true, CodeSelectorNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			to, _ := evm.PrecompileAddress("layerxprograms")
			h.policy.doc.Defaults.Selectors[to.Hex()] = []string{selectorHex(t, "layerxprograms", "call")}
			if !tc.withoutRule {
				h.policy.doc.Defaults.ProgramCalls = []ProgramCallRule{roomProgramRule()}
			}
			data := roomProgramData(t, tc.program, tc.entrypoint, tc.fee)
			d := h.policy.Evaluate(testAccount, h.tx(t, to, big.NewInt(tc.value), data), h.ledger)
			expect(t, d, tc.code)
			if tc.code == CodeAllowed {
				if len(d.Spends) != 0 {
					t.Fatalf("program call spends = %+v", d.Spends)
				}
				compiled, err := compile(h.policy.doc.Defaults)
				if err != nil {
					t.Fatal(err)
				}
				out, err := inspectCallContext(Context{programCalls: compiled.programCalls}, to, big.NewInt(0), data)
				if err != nil {
					t.Fatal(err)
				}
				if len(out.ProgramCalls) != 1 || out.ProgramCalls[0].ProgramID != tc.program || out.ProgramCalls[0].Entrypoint != tc.entrypoint || out.ProgramCalls[0].FeeLimit.Cmp(big.NewInt(tc.fee)) != 0 {
					t.Fatalf("program inspection = %+v", out.ProgramCalls)
				}
			}
		})
	}
}

func TestRoomStreamMeterPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		value        int64
		code         string
	}{
		{"meter_funded_stream", "streamMeter", 0, CodeAllowed},
		{"meter_nonpayable", "streamMeter", 1, CodeDecodeError},
		{"unresolved_method", "streamPause", 0, CodeSelectorNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			to, _ := evm.PrecompileAddress("layerxpay")
			h.policy.doc.Defaults.Selectors[to.Hex()] = []string{selectorHex(t, "layerxpay", tc.method)}
			args := []any{[32]byte{0x36}}
			if tc.method == "streamMeter" {
				args = append(args, uint64(2), [32]byte{3}, make([]byte, 64))
			}
			data := repair244Pack(t, "layerxpay", tc.method, args...)
			d := h.policy.Evaluate(testAccount, h.tx(t, to, big.NewInt(tc.value), data), h.ledger)
			expect(t, d, tc.code)
			if d.Allowed && len(d.Spends) != 0 {
				t.Fatalf("meter spends = %+v", d.Spends)
			}
		})
	}
}

func TestRoomProgramCallsParse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ProgramCallRule)
	}{
		{"short_program", func(r *ProgramCallRule) { r.ProgramID = "36" }},
		{"nonhex_program", func(r *ProgramCallRule) { r.ProgramID = "zz" + r.ProgramID[2:] }},
		{"missing_entrypoints", func(r *ProgramCallRule) { r.Entrypoints = nil }},
		{"empty_entrypoint", func(r *ProgramCallRule) { r.Entrypoints = []string{""} }},
		{"repeated_entrypoint", func(r *ProgramCallRule) { r.Entrypoints = []string{"layerx_call", "layerx_call"} }},
		{"missing_fee", func(r *ProgramCallRule) { r.FeeLimit = "" }},
		{"negative_fee", func(r *ProgramCallRule) { r.FeeLimit = "-1" }},
		{"signed_fee", func(r *ProgramCallRule) { r.FeeLimit = "+1" }},
		{"overflow_fee", func(r *ProgramCallRule) { r.FeeLimit = "340282366920938463463374607431768211456" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Parse(testDocument(t))
			if err != nil {
				t.Fatal(err)
			}
			rule := roomProgramRule()
			tc.change(&rule)
			doc.Defaults.ProgramCalls = []ProgramCallRule{rule}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Parse(raw); err == nil {
				t.Fatal("malformed program_calls accepted")
			}
		})
	}
	doc, err := Parse(testDocument(t))
	if err != nil {
		t.Fatal(err)
	}
	doc.Defaults.ProgramCalls = []ProgramCallRule{roomProgramRule()}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw); err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["defaults"].(map[string]any)["program_calls"].([]any)[0].(map[string]any)["unknown"] = true
	raw, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw); err == nil {
		t.Fatal("unknown program rule field accepted")
	}
}

func TestRoomProgramCallsAccountRules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override *Rules
		selector bool
		code     string
	}{
		{"inherited_default", nil, true, CodeAllowed},
		{"empty_account_list", &Rules{ProgramCalls: []ProgramCallRule{}}, true, CodeSelectorNotAllowed},
		{"account_fee_limit", &Rules{ProgramCalls: []ProgramCallRule{{ProgramID: roomProgramRule().ProgramID, Entrypoints: []string{"layerx_call"}, FeeLimit: "999"}}}, true, CodeSelectorNotAllowed},
		{"selector_still_required", nil, false, CodeSelectorNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			to, _ := evm.PrecompileAddress("layerxprograms")
			h.policy.doc.Defaults.ProgramCalls = []ProgramCallRule{roomProgramRule()}
			if tc.selector {
				h.policy.doc.Defaults.Selectors[to.Hex()] = []string{selectorHex(t, "layerxprograms", "call")}
			}
			if tc.override != nil {
				h.policy.doc.Accounts[testAccount] = *tc.override
			}
			data := roomProgramData(t, [32]byte{0x36}, "layerx_call", 1000)
			expect(t, h.policy.Evaluate(testAccount, h.tx(t, to, big.NewInt(0), data), h.ledger), tc.code)
		})
	}
}
