package layerx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

type committedRecordedBlockID struct {
	Hash  string `json:"hash"`
	Parts struct {
		Total uint32 `json:"total"`
		Hash  string `json:"hash"`
	} `json:"parts"`
}

func (id committedRecordedBlockID) decode(t *testing.T) CometBlockID {
	t.Helper()
	return CometBlockID{Hash: fixtureBytes(t, id.Hash), PartSetTotal: id.Parts.Total, PartSetHash: fixtureBytes(t, id.Parts.Hash)}
}

type committedRecordedHeader struct {
	Version struct {
		Block uint64 `json:"block,string"`
		App   uint64 `json:"app,string"`
	} `json:"version"`
	ChainID            string                   `json:"chain_id"`
	Height             int64                    `json:"height,string"`
	Time               time.Time                `json:"time"`
	LastBlockID        committedRecordedBlockID `json:"last_block_id"`
	LastCommitHash     string                   `json:"last_commit_hash"`
	DataHash           string                   `json:"data_hash"`
	ValidatorsHash     string                   `json:"validators_hash"`
	NextValidatorsHash string                   `json:"next_validators_hash"`
	ConsensusHash      string                   `json:"consensus_hash"`
	AppHash            string                   `json:"app_hash"`
	LastResultsHash    string                   `json:"last_results_hash"`
	EvidenceHash       string                   `json:"evidence_hash"`
	ProposerAddress    string                   `json:"proposer_address"`
}

func (h committedRecordedHeader) decode(t *testing.T) CommittedHeader {
	t.Helper()
	return CommittedHeader{
		VersionBlock: h.Version.Block, VersionApp: h.Version.App, ChainID: h.ChainID, Height: h.Height, Time: h.Time,
		LastBlockID: h.LastBlockID.decode(t), LastCommitHash: fixtureBytes(t, h.LastCommitHash), DataHash: fixtureBytes(t, h.DataHash),
		ValidatorsHash: fixtureBytes(t, h.ValidatorsHash), NextValidatorsHash: fixtureBytes(t, h.NextValidatorsHash),
		ConsensusHash: fixtureBytes(t, h.ConsensusHash), AppHash: fixtureBytes(t, h.AppHash), LastResultsHash: fixtureBytes(t, h.LastResultsHash),
		EvidenceHash: fixtureBytes(t, h.EvidenceHash), ProposerAddress: fixtureBytes(t, h.ProposerAddress),
	}
}

type committedRecordedCommit struct {
	Height     int64                    `json:"height,string"`
	Round      int32                    `json:"round"`
	BlockID    committedRecordedBlockID `json:"block_id"`
	Signatures []struct {
		Flag      uint8     `json:"block_id_flag"`
		Address   string    `json:"validator_address"`
		Timestamp time.Time `json:"timestamp"`
		Signature []byte    `json:"signature"`
	} `json:"signatures"`
}

func (c committedRecordedCommit) decode(t *testing.T) CometCommit {
	t.Helper()
	commit := CometCommit{Height: c.Height, Round: c.Round, BlockID: c.BlockID.decode(t)}
	for _, s := range c.Signatures {
		commit.Signatures = append(commit.Signatures, CometCommitSignature{
			BlockIDFlag: s.Flag, ValidatorAddress: fixtureBytes(t, s.Address), Timestamp: s.Timestamp, Signature: s.Signature,
		})
	}
	return commit
}

type committedRecordedProof struct {
	Total int64    `json:"total,string"`
	Index int64    `json:"index,string"`
	Aunts [][]byte `json:"aunts"`
}

func (p committedRecordedProof) decode(t *testing.T) CommittedProof {
	t.Helper()
	proof := CommittedProof{Total: p.Total, Index: p.Index}
	for _, aunt := range p.Aunts {
		if len(aunt) != 32 {
			t.Fatalf("recorded aunt has %d bytes", len(aunt))
		}
		var hash [32]byte
		copy(hash[:], aunt)
		proof.Aunts = append(proof.Aunts, hash)
	}
	return proof
}

type committedRecordedTxProof struct {
	Data  []byte                 `json:"data"`
	Proof committedRecordedProof `json:"proof"`
}

func TestCommittedInclusionBindsTheWrappedTransaction(t *testing.T) {
	recorded, err := os.ReadFile("../../../daemon/committed-fixture/vectors/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Authorization struct {
			PublicKey        [32]byte
			FirstBatchNumber uint64
			LastBatchNumber  uint64
		} `json:"authorization"`
		Settlement struct {
			RawTransaction string          `json:"raw_transaction"`
			Receipt        json.RawMessage `json:"receipt"`
			Batch          struct {
				PreviousSequenceHash []byte `json:"PrevSeqHash"`
				KeyID                []byte `json:"KeyId"`
			} `json:"sequence_batch"`
			BatchProof       committedRecordedTxProof `json:"sequence_batch_proof"`
			TransactionProof committedRecordedTxProof `json:"transaction_proof"`
			Result           struct {
				Code      uint32 `json:"code"`
				Data      []byte `json:"data"`
				GasWanted int64  `json:"gas_wanted"`
				GasUsed   int64  `json:"gas_used"`
			} `json:"exec_tx_result"`
			ResultProof     committedRecordedProof  `json:"exec_tx_result_proof"`
			ExecutionHeader committedRecordedHeader `json:"execution_header"`
			ExecutionCommit committedRecordedCommit `json:"execution_commit"`
			Header          committedRecordedHeader `json:"header"`
			Commit          committedRecordedCommit `json:"commit"`
			Validators      []struct {
				Key struct {
					Type  string `json:"type"`
					Value []byte `json:"value"`
				} `json:"pub_key"`
				Power int64 `json:"voting_power,string"`
			} `json:"validators"`
		} `json:"settlement"`
	}
	if err := json.Unmarshal(recorded, &vector); err != nil {
		t.Fatal(err)
	}
	s := vector.Settlement
	if len(s.Batch.KeyID) != 32 || len(s.Batch.PreviousSequenceHash) != 32 {
		t.Fatal("invalid recorded receipt key or sequence hash")
	}
	var keyID, previous [32]byte
	copy(keyID[:], s.Batch.KeyID)
	copy(previous[:], s.Batch.PreviousSequenceHash)
	keys := ReceiptKeys{PreviousSequenceHash: previous, Pins: []ReceiptKey{{KeyID: keyID, PublicKey: vector.Authorization.PublicKey,
		FirstHeight: vector.Authorization.FirstBatchNumber, LastHeight: vector.Authorization.LastBatchNumber}}}
	raw := sequencerFixtureBytes(t, s.RawTransaction)
	var receiptEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(s.Receipt, &receiptEnvelope); err != nil {
		t.Fatal(err)
	}
	if string(receiptEnvelope["rung"]) != `"committed"` {
		t.Fatal("recorded settlement is not committed")
	}
	// Verify the signed receipt at the receipted rung before proving its block inclusion.
	receiptEnvelope["rung"] = json.RawMessage(`"receipted"`)
	receiptBytes, err := json.Marshal(receiptEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := VerifySequencerReceipt(receiptBytes, raw, keys)
	if err != nil {
		t.Fatalf("recorded sequencer receipt: %v", err)
	}
	var validators []CometValidator
	for _, validator := range s.Validators {
		if validator.Key.Type != "tendermint/PubKeyEd25519" || len(validator.Key.Value) != 32 {
			t.Fatal("invalid recorded validator key")
		}
		var public [32]byte
		copy(public[:], validator.Key.Value)
		validators = append(validators, CometValidator{PublicKey: public, VotingPower: validator.Power})
	}
	header, commit := s.ExecutionHeader.decode(t), s.ExecutionCommit.decode(t)
	inclusion := ReceiptInclusion{
		Transaction: s.TransactionProof.Data, TransactionProof: s.TransactionProof.Proof.decode(t),
		Result:      ExecTxResult{Code: s.Result.Code, Data: s.Result.Data, GasWanted: s.Result.GasWanted, GasUsed: s.Result.GasUsed},
		ResultProof: s.ResultProof.decode(t), BatchTransaction: s.BatchProof.Data, BatchProof: s.BatchProof.Proof.decode(t),
		NextHeader: s.Header.decode(t), NextCommit: s.Commit.decode(t), NextValidators: validators,
	}
	extracted, ok := committedEVMTransaction(inclusion.Transaction)
	if !ok || !bytes.Equal(extracted, raw) {
		t.Fatalf("wrapped transaction differs from recorded signed EVM bytes: %x", extracted)
	}
	if receiptKeccak256(inclusion.Transaction) == receipt.Receipt.ActivityID {
		t.Fatal("fixture must distinguish block bytes from EVM bytes")
	}
	if !committedVerifyTransactionProof(inclusion.Transaction, inclusion.TransactionProof, header.DataHash) {
		t.Fatal("recorded block transaction proof failed")
	}
	result, err := VerifyCommittedReceipt(receipt, header, inclusion, commit, validators)
	if err != nil {
		t.Fatalf("recorded V4 settlement: %v", err)
	}
	if result.Rung != "committed" || result.Level != "committed" || result.Height != uint64(header.Height) ||
		result.BlockHash != header.Hash() || result.ReceiptDigest != receipt.ReceiptDigest || result.Status != receipt.Status {
		t.Fatalf("unexpected committed result: %+v", result)
	}
	// The recorded payExact calldata appears verbatim inside the typed LegacyTx.
	calldata, err := hex.DecodeString("78ea11db")
	if err != nil {
		t.Fatal(err)
	}
	inside := bytes.Index(inclusion.Transaction, calldata)
	if inside < 0 {
		t.Fatal("recorded payExact selector missing from wrapper")
	}
	for _, mutation := range []struct {
		name       string
		offset     int
		evmChanged bool
	}{
		{"wrapped EVM transaction", inside, true},
		{"outer block transaction", len(inclusion.Transaction) - 1, false},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := inclusion
			changed.Transaction = append([]byte(nil), inclusion.Transaction...)
			changed.Transaction[mutation.offset] ^= 1
			changedEVM, ok := committedEVMTransaction(changed.Transaction)
			if !ok || (receiptKeccak256(changedEVM) != receipt.Receipt.ActivityID) != mutation.evmChanged {
				t.Fatal("mutation did not isolate the expected EVM or wrapper bytes")
			}
			if committedVerifyTransactionProof(changed.Transaction, changed.TransactionProof, header.DataHash) {
				t.Fatal("mutated block transaction retained inclusion")
			}
			result, err := VerifyCommittedReceipt(receipt, header, changed, commit, validators)
			var verification *VerificationError
			if !errors.As(err, &verification) || verification.Check != "committed-inclusion" || !errors.Is(err, ErrVerification) || result.Rung != "" {
				t.Fatalf("expected committed-inclusion refusal, got rung %q, error %v", result.Rung, err)
			}
		})
	}
	t.Run("verbatim signed EVM bytes", func(t *testing.T) {
		message := committedBytes(1, []byte("/ethermint.evm.v1.MsgEthereumTx"))
		message = append(message, committedBytes(2, committedBytes(1, raw))...)
		block := committedBytes(1, committedBytes(1, message))
		got, ok := committedEVMTransaction(block)
		if !ok || !bytes.Equal(got, raw) {
			t.Fatal("verbatim signed transaction was not preserved")
		}
		if _, ok := committedEVMTransaction(raw); ok {
			t.Fatal("raw EVM bytes were accepted as a Cosmos block transaction")
		}
		for end := 0; end < len(block); end++ {
			if _, ok := committedEVMTransaction(block[:end]); ok {
				t.Fatalf("accepted truncated wrapper at %d", end)
			}
		}
	})
}
