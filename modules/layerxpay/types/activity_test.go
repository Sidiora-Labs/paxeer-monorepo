package types

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func TestMethodActivityMatchesKernelTypes(t *testing.T) {
	abiBytes, err := os.ReadFile("../../../precompiles/layerxpay/abi.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Type            string `json:"type"`
		Name            string `json:"name"`
		StateMutability string `json:"stateMutability"`
	}
	if err := json.Unmarshal(abiBytes, &entries); err != nil {
		t.Fatal(err)
	}

	kernelTypes := make(map[string]uint32)
	declaration := regexp.MustCompile(`(?m)^\s*(LX_[A-Z_]+)\s*=\s*(0x[0-9a-fA-F]+)\s*,?\s*$`)
	for _, header := range []string{"lx_escrow.h", "lx_budget.h", "lx_stream.h", "lx_service.h"} {
		data, err := os.ReadFile("../../../include/layerx/" + header)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range declaration.FindAllSubmatch(data, -1) {
			value, err := strconv.ParseUint(string(match[2]), 0, 32)
			if err != nil {
				t.Fatal(err)
			}
			kernelTypes[string(match[1])] = uint32(value)
		}
	}

	expected := map[string]struct {
		kernel string
		row    KernelActivity
	}{
		"escrowOpen":              {"LX_ESCROW_OPEN", KernelActivity{ActivityEscrowOpen, EscrowModuleID, 288}},
		"escrowCapture":           {"LX_ESCROW_CAPTURE", KernelActivity{ActivityEscrowCapture, EscrowModuleID, 80}},
		"escrowPartialCapture":    {"LX_ESCROW_PARTIAL_CAPTURE", KernelActivity{ActivityEscrowPartialCapture, EscrowModuleID, 80}},
		"escrowRelease":           {"LX_ESCROW_RELEASE", KernelActivity{ActivityEscrowRelease, EscrowModuleID, 64}},
		"escrowTimeout":           {"LX_ESCROW_TIMEOUT", KernelActivity{ActivityEscrowTimeout, EscrowModuleID, 64}},
		"escrowDisputeOpen":       {"LX_ESCROW_DISPUTE_OPEN", KernelActivity{ActivityEscrowDisputeOpen, EscrowModuleID, 32}},
		"escrowDisputeResolve":    {"LX_ESCROW_DISPUTE_RESOLVE", KernelActivity{ActivityEscrowDisputeResolve, EscrowModuleID, 68}},
		"budgetCreate":            {"LX_BUDGET_CREATE", KernelActivity{ActivityBudgetCreate, BudgetModuleID, 211}},
		"budgetCreateNative":      {"LX_BUDGET_CREATE", KernelActivity{ActivityBudgetCreate, BudgetModuleID, 251}},
		"budgetFund":              {"LX_BUDGET_FUND", KernelActivity{ActivityBudgetFund, BudgetModuleID, 50}},
		"budgetFundNative":        {"LX_BUDGET_FUND", KernelActivity{ActivityBudgetFund, BudgetModuleID, 58}},
		"budgetAmend":             {"LX_BUDGET_AMEND", KernelActivity{ActivityBudgetAmend, BudgetModuleID, 75}},
		"budgetDelegateAdd":       {"LX_BUDGET_DELEGATE_ADD", KernelActivity{ActivityBudgetDelegateAdd, BudgetModuleID, 66}},
		"budgetDelegateRemove":    {"LX_BUDGET_DELEGATE_REMOVE", KernelActivity{ActivityBudgetDelegateRemove, BudgetModuleID, 66}},
		"budgetSpend":             {"LX_BUDGET_SPEND", KernelActivity{ActivityBudgetSpend, BudgetModuleID, 82}},
		"budgetClose":             {"LX_BUDGET_CLOSE", KernelActivity{ActivityBudgetClose, BudgetModuleID, 42}},
		"budgetDefund":            {"LX_BUDGET_DEFUND", KernelActivity{ActivityBudgetDefund, BudgetModuleID, 50}},
		"budgetRevoke":            {"LX_BUDGET_REVOKE", KernelActivity{ActivityBudgetRevoke, BudgetModuleID, 42}},
		"budgetDelegateSpend":     {"LX_BUDGET_DELEGATE_SPEND", KernelActivity{ActivityBudgetDelegateSpend, BudgetModuleID, 114}},
		"budgetPull":              {"LX_BUDGET_PULL", KernelActivity{ActivityBudgetPull, BudgetModuleID, 460}},
		"streamOpen":              {"LX_STREAM_OPEN", KernelActivity{ActivityStreamOpen, StreamModuleID, 0}},
		"streamTopUp":             {"LX_STREAM_TOP_UP", KernelActivity{ActivityStreamTopUp, StreamModuleID, 50}},
		"streamMeter":             {"LX_STREAM_METER", KernelActivity{ActivityStreamMeter, StreamModuleID, 138}},
		"streamSettle":            {"LX_STREAM_SETTLE", KernelActivity{ActivityStreamSettle, StreamModuleID, 66}},
		"streamPause":             {"LX_STREAM_PAUSE", KernelActivity{ActivityStreamPause, StreamModuleID, 34}},
		"streamResume":            {"LX_STREAM_RESUME", KernelActivity{ActivityStreamResume, StreamModuleID, 34}},
		"streamClose":             {"LX_STREAM_CLOSE", KernelActivity{ActivityStreamClose, StreamModuleID, 66}},
		"serviceOfferPublish":     {"LX_SERVICE_OFFER_PUBLISH", KernelActivity{ActivityServiceOfferPublish, ServiceModuleID, 179}},
		"serviceOfferWithdraw":    {"LX_SERVICE_OFFER_WITHDRAW", KernelActivity{ActivityServiceOfferWithdraw, ServiceModuleID, 34}},
		"serviceAgreementPropose": {"LX_SERVICE_AGREEMENT_PROPOSE", KernelActivity{ActivityServiceAgreementPropose, ServiceModuleID, 130}},
		"serviceAgreementAccept":  {"LX_SERVICE_AGREEMENT_ACCEPT", KernelActivity{ActivityServiceAgreementAccept, ServiceModuleID, 34}},
		"serviceCommitTask":       {"LX_SERVICE_COMMIT_TASK", KernelActivity{ActivityServiceCommitTask, ServiceModuleID, 146}},
		"serviceCommitAbandon":    {"LX_SERVICE_COMMIT_ABANDON", KernelActivity{ActivityServiceCommitAbandon, ServiceModuleID, 36}},
		"serviceToolExecAttest":   {"LX_SERVICE_TOOL_EXEC_ATTEST", KernelActivity{ActivityServiceToolExecAttest, ServiceModuleID, 416}},
		"serviceProgressReport":   {"LX_SERVICE_PROGRESS_REPORT", KernelActivity{ActivityServiceProgressReport, ServiceModuleID, 134}},
		"serviceDeliver":          {"LX_SERVICE_DELIVER", KernelActivity{ActivityServiceDeliver, ServiceModuleID, 0}},
		"serviceAccept":           {"LX_SERVICE_ACCEPT", KernelActivity{ActivityServiceAccept, ServiceModuleID, 34}},
		"serviceReject":           {"LX_SERVICE_REJECT", KernelActivity{ActivityServiceReject, ServiceModuleID, 0}},
		"serviceDisputeOpen":      {"LX_SERVICE_DISPUTE_OPEN", KernelActivity{ActivityServiceDisputeOpen, ServiceModuleID, 0}},
		"serviceDisputeResolve":   {"LX_SERVICE_DISPUTE_RESOLVE", KernelActivity{ActivityServiceDisputeResolve, ServiceModuleID, 72}},
	}

	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.Type != "function" {
			continue
		}
		got, mapped := MethodActivity[entry.Name]
		if entry.StateMutability != "nonpayable" {
			if mapped {
				t.Errorf("%s method %s has an activity row", entry.StateMutability, entry.Name)
			}
			continue
		}
		if seen[entry.Name] {
			t.Fatalf("duplicate nonpayable ABI method %s", entry.Name)
		}
		seen[entry.Name] = true
		want, known := expected[entry.Name]
		if !known {
			t.Errorf("nonpayable ABI method %s has no frozen expectation", entry.Name)
			continue
		}
		t.Run(entry.Name, func(t *testing.T) {
			if !mapped || got != want.row {
				t.Fatalf("activity = %+v (present %t), want %+v", got, mapped, want.row)
			}
			kernelType, exists := kernelTypes[want.kernel]
			if !exists || got.Type != kernelType {
				t.Errorf("type = %#08x, kernel %s = %#08x (present %t)", got.Type, want.kernel, kernelType, exists)
			}
			if got.Type>>16 != uint32(got.Module) || got.Type&0xffff == 0 {
				t.Errorf("type %#08x does not encode module %d and a nonzero ordinal", got.Type, got.Module)
			}
		})
	}
	if len(seen) != len(expected) || len(MethodActivity) != len(expected) {
		t.Errorf("nonpayable ABI methods = %d, table rows = %d, want %d", len(seen), len(MethodActivity), len(expected))
	}
	for name := range expected {
		if !seen[name] {
			t.Errorf("frozen method %s absent from nonpayable ABI", name)
		}
	}
	for name := range MethodActivity {
		if !seen[name] {
			t.Errorf("activity row %s has no nonpayable ABI method", name)
		}
	}
	if got, exists := kernelTypes["LX_STREAM_RATE_CHANGE"]; !exists || ActivityStreamRateChange != got || ActivityStreamRateChange != 0x00040008 {
		t.Errorf("stream rate change = %#08x, kernel = %#08x (present %t)", ActivityStreamRateChange, got, exists)
	}
}

func TestMethodActivitySharedOrdinals(t *testing.T) {
	id, account, asset, purpose, key := [32]byte{1}, [32]byte{2}, [32]byte{3}, [32]byte{4}, [32]byte{5}
	amount := codec.U128{Lo: 10}
	create, err := EncodeBudgetCreate(id, account, asset, purpose, amount, amount, amount, 100, 1, 1000, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	createNative, err := EncodeBudgetCreateNative(id, account, asset, purpose, amount, amount, amount, 100, 1, 1000, 1, 0, account, 1)
	if err != nil {
		t.Fatal(err)
	}
	fund, err := EncodeBudgetFund(id, amount)
	if err != nil {
		t.Fatal(err)
	}
	fundNative, err := EncodeBudgetFundNative(id, amount, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		want    KernelActivity
		payload []byte
	}{
		{"escrowCapture", KernelActivity{0x00020002, 2, 80}, EncodeEscrowCapture(id, amount, key)},
		{"escrowPartialCapture", KernelActivity{0x00020003, 2, 80}, EncodeEscrowCapture(id, amount, key)},
		{"escrowRelease", KernelActivity{0x00020004, 2, 64}, EncodeEscrowRelease(id, key)},
		{"escrowTimeout", KernelActivity{0x00020005, 2, 64}, EncodeEscrowRelease(id, key)},
		{"budgetCreate", KernelActivity{0x00030001, 3, 211}, create},
		{"budgetCreateNative", KernelActivity{0x00030001, 3, 251}, createNative},
		{"budgetFund", KernelActivity{0x00030002, 3, 50}, fund},
		{"budgetFundNative", KernelActivity{0x00030002, 3, 58}, fundNative},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, exists := MethodActivity[tc.name]
			if !exists || got != tc.want {
				t.Fatalf("activity = %+v (present %t), want %+v", got, exists, tc.want)
			}
			if len(tc.payload) != got.PayloadBytes {
				t.Errorf("encoded payload bytes = %d, table declares %d", len(tc.payload), got.PayloadBytes)
			}
		})
	}
}
