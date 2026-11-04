package qualification

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"regexp"
)

const EvidenceSchema = "paxeer-node-rejection-policy/v1"

// PolicyEvidence is emitted from retained node admission and committed execution,
// not from the retired kernel ordering/disposition table. Amounts are wei.
type PolicyEvidence struct {
	Schema   string       `json:"schema"`
	Revision string       `json:"revision"`
	Cases    []PolicyCase `json:"cases"`
}

type PolicyState struct {
	Nonce     uint64 `json:"nonce"`
	Sequence  uint64 `json:"sequence"`
	Payer     string `json:"payer"`
	Recipient string `json:"recipient"`
	Storage   string `json:"storage"`
}

type PolicyCase struct {
	Name             string      `json:"name"`
	Transaction      string      `json:"transaction"`
	Before           PolicyState `json:"before"`
	After            PolicyState `json:"after"`
	Committed        PolicyState `json:"committed"`
	Admitted         bool        `json:"admitted"`
	Code             uint32      `json:"code"`
	GasUsed          uint64      `json:"gas_used"`
	GasLimit         uint64      `json:"gas_limit"`
	GasPrice         string      `json:"gas_price"`
	ReceiptStatus    *uint32     `json:"receipt_status"`
	CommittedCode    uint32      `json:"committed_code"`
	CommittedGasUsed uint64      `json:"committed_gas_used"`
	Value            string      `json:"value"`
}

var PolicyCases = []string{"malformed", "wrong_network", "bad_signature", "sequence_gap", "sequence_reused", "fee_unpayable", "fee_below_floor", "success", "execution_failure", "historical_replay"}
var decimal = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func amount(text string) (*big.Int, error) {
	if !decimal.MatchString(text) {
		return nil, fmt.Errorf("invalid unsigned amount %q", text)
	}
	value, _ := new(big.Int).SetString(text, 10)
	return value, nil
}

func ReadPolicyEvidence(reader io.Reader, revision string) (*PolicyEvidence, error) {
	var evidence PolicyEvidence
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("expected one evidence document")
	}
	if err := CheckPolicyEvidence(evidence, revision); err != nil {
		return nil, err
	}
	return &evidence, nil
}

func CheckPolicyEvidence(evidence PolicyEvidence, revision string) error {
	if evidence.Schema != EvidenceSchema {
		return fmt.Errorf("unsupported policy evidence schema %q", evidence.Schema)
	}
	if !revisionPattern.MatchString(revision) || evidence.Revision != revision {
		return fmt.Errorf("policy evidence revision mismatch")
	}
	rows := make(map[string]PolicyCase)
	for _, row := range evidence.Cases {
		if _, exists := rows[row.Name]; exists {
			return fmt.Errorf("duplicate policy case %s", row.Name)
		}
		rows[row.Name] = row
	}
	if len(rows) != len(PolicyCases) {
		return fmt.Errorf("policy cases are not exhaustive")
	}
	for _, name := range PolicyCases {
		row, ok := rows[name]
		if !ok {
			return fmt.Errorf("missing policy case %s", name)
		}
		if err := checkPolicyCase(row); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	// Historical replay must run the very same signed bytes and recover the
	// original state and execution result, rather than only match a case label.
	original, replay := rows["success"], rows["historical_replay"]
	if original.Transaction != replay.Transaction || original.Before != replay.Before || original.After != replay.After || original.Committed != replay.Committed || original.Code != replay.Code || original.GasUsed != replay.GasUsed || original.GasLimit != replay.GasLimit || original.GasPrice != replay.GasPrice || original.Value != replay.Value {
		return fmt.Errorf("historical replay differs from original execution")
	}
	return nil
}

func checkPolicyCase(row PolicyCase) error {
	tx, err := hex.DecodeString(row.Transaction)
	if err != nil || len(tx) == 0 {
		return fmt.Errorf("missing canonical transaction bytes")
	}
	for _, state := range []PolicyState{row.Before, row.After, row.Committed} {
		if _, err := amount(state.Payer); err != nil {
			return err
		}
		if _, err := amount(state.Recipient); err != nil {
			return err
		}
		storage, err := hex.DecodeString(state.Storage)
		if err != nil || len(storage) != 32 {
			return fmt.Errorf("invalid module storage commitment")
		}
	}
	price, err := amount(row.GasPrice)
	if err != nil {
		return err
	}
	value, err := amount(row.Value)
	if err != nil {
		return err
	}
	success := row.Name == "success" || row.Name == "historical_replay"
	admitted := success || row.Name == "execution_failure"
	if row.Admitted != admitted {
		return fmt.Errorf("unexpected admission decision")
	}
	if !admitted {
		if row.Code == 0 || row.Before != row.After || row.Before != row.Committed || row.GasUsed != 0 || row.ReceiptStatus != nil || row.CommittedCode != 0 || row.CommittedGasUsed != 0 {
			return fmt.Errorf("admission refusal consumed sequence, fee, effects or receipt")
		}
		return nil
	}
	if row.Before.Nonce == ^uint64(0) || row.Before.Sequence == ^uint64(0) || row.After.Nonce != row.Before.Nonce+1 || row.After.Sequence != row.Before.Sequence+1 {
		return fmt.Errorf("execution must consume nonce and sequence exactly once")
	}
	if row.Committed != row.After || row.CommittedCode != row.Code || row.CommittedGasUsed != row.GasUsed {
		return fmt.Errorf("committed execution differs from admission execution")
	}
	if row.GasUsed == 0 || row.GasUsed > row.GasLimit || price.Sign() == 0 || row.ReceiptStatus == nil {
		return fmt.Errorf("missing execution fee or receipt")
	}
	wantStatus := uint32(0)
	if success {
		wantStatus = 1
	}
	if *row.ReceiptStatus != wantStatus || (row.Code == 0) != success {
		return fmt.Errorf("execution result and receipt disagree")
	}
	beforePayer, _ := amount(row.Before.Payer)
	afterPayer, _ := amount(row.After.Payer)
	beforeRecipient, _ := amount(row.Before.Recipient)
	afterRecipient, _ := amount(row.After.Recipient)
	charge := new(big.Int).Mul(new(big.Int).SetUint64(row.GasUsed), price)
	if success {
		charge.Add(charge, value)
	} else if row.After.Storage != row.Before.Storage {
		return fmt.Errorf("failed execution left module effects")
	}
	if new(big.Int).Sub(beforePayer, afterPayer).Cmp(charge) != 0 {
		return fmt.Errorf("incorrect execution fee")
	}
	received := new(big.Int).Sub(afterRecipient, beforeRecipient)
	if success {
		if received.Cmp(value) != 0 {
			return fmt.Errorf("success transfer effects disagree")
		}
	} else if received.Sign() != 0 {
		return fmt.Errorf("failed execution transferred value")
	}
	return nil
}
