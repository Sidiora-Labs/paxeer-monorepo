package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func TestVectorV4CommittedVerifies(t *testing.T) {
	data, err := os.ReadFile("vectors/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector v4Vector
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&vector); err != nil {
		t.Fatal(err)
	}
	if vector.Name != "vector-v4-committed" || vector.Provenance == "" {
		t.Fatal("missing vector identity or recording provenance")
	}
	assertV4CommittedVerifies(t, &vector)
}

func assertV4CommittedVerifies(t *testing.T, vector *v4Vector) {
	t.Helper()
	check := func(t *testing.T, entry v4Case) (*verify.VerifiedReceipt, *codec.BatchHeader, error) {
		t.Helper()
		if len(entry.HeaderSignature) != 64 {
			t.Fatal("header signature must be exactly 64 bytes")
		}
		header, err := codec.DecodeBatchHeader(entry.Header)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := verify.ReceiptAtRoot(entry.Receipt, &entry.Proof, header.ReceiptMerkleRoot, vector.Authorization.PublicKey)
		if err != nil {
			return nil, nil, err
		}
		if err := verify.Ed25519Digest(vector.Authorization.PublicKey, [64]byte(entry.HeaderSignature), codec.BatchHeaderDigest(entry.Header)); err != nil {
			return nil, nil, err
		}
		return receipt, header, nil
	}
	positive := vector.Positive
	if positive.Name != "committed-payExact" {
		t.Fatal("positive must be the locally recorded committed payExact receipt")
	}
	receipt, header, err := check(t, positive)
	if err != nil {
		t.Fatalf("positive receipt inclusion and header signature: %v", err)
	}
	if err := verify.Ed25519Digest(vector.Authorization.PublicKey, [64]byte(positive.HeaderSignature), codec.BatchHeaderDigest(positive.Header)); err != nil {
		t.Fatalf("positive header signature: %v", err)
	}
	if !bytes.Equal(v4EncodeHeader(*header), positive.Header) {
		t.Fatal("header does not round trip canonically")
	}
	if len(vector.Refusals) != 2 {
		t.Fatalf("want the two named refusals, got %d", len(vector.Refusals))
	}
	seen := make(map[string]bool)
	for _, refusal := range vector.Refusals {
		t.Run(refusal.Name, func(t *testing.T) {
			if seen[refusal.Name] {
				t.Fatal("duplicate refusal")
			}
			seen[refusal.Name] = true
			if !bytes.Equal(refusal.Receipt, positive.Receipt) || !bytes.Equal(refusal.Header, positive.Header) {
				t.Fatal("refusal altered the base receipt or header")
			}
			_, _, err := check(t, refusal)
			switch refusal.Name {
			case "flipped-proof-sibling":
				if !errors.Is(err, codec.ErrMerkleRootMismatch) {
					t.Fatalf("want Merkle root mismatch, got %v", err)
				}
				if !bytes.Equal(refusal.HeaderSignature, positive.HeaderSignature) {
					t.Fatal("sibling refusal also changed the signature")
				}
				expected := positive.Proof
				expected.Siblings = append([][32]byte(nil), positive.Proof.Siblings...)
				if len(expected.Siblings) == 0 {
					t.Fatal("positive must have a proof sibling")
				}
				expected.Siblings[0][0] ^= 1
				if !reflect.DeepEqual(refusal.Proof, expected) {
					t.Fatal("refusal must flip exactly one bit in a sibling")
				}
				if err := verify.Ed25519Digest(vector.Authorization.PublicKey, [64]byte(refusal.HeaderSignature), codec.BatchHeaderDigest(refusal.Header)); err != nil {
					t.Fatalf("sibling mutation must preserve header validity: %v", err)
				}
			case "header-signature-by-unpinned-key":
				if err == nil {
					t.Fatalf("want unpinned header signature refusal, got %v", err)
				}
				if !reflect.DeepEqual(refusal.Proof, positive.Proof) {
					t.Fatal("unpinned-key refusal changed the proof")
				}
				if len(refusal.SigningKey) != 32 || bytes.Equal(refusal.SigningKey, vector.Authorization.PublicKey[:]) {
					t.Fatal("refusal must carry a distinct unpinned public key")
				}
				if _, err := verify.ReceiptAtRoot(refusal.Receipt, &refusal.Proof, header.ReceiptMerkleRoot, vector.Authorization.PublicKey); err != nil {
					t.Fatalf("unpinned-key refusal must preserve receipt inclusion: %v", err)
				}
				if err := verify.Ed25519Digest([32]byte(refusal.SigningKey), [64]byte(refusal.HeaderSignature), codec.BatchHeaderDigest(refusal.Header)); err != nil {
					t.Fatalf("refusal must be a valid signature under the unpinned key: %v", err)
				}
				if err := verify.Ed25519Digest(vector.Authorization.PublicKey, [64]byte(refusal.HeaderSignature), codec.BatchHeaderDigest(refusal.Header)); err == nil {
					t.Fatal("Ed25519Digest accepted an unpinned key")
				}
			default:
				t.Fatalf("unexpected refusal %q", refusal.Name)
			}
		})
	}
	if !seen["flipped-proof-sibling"] || !seen["header-signature-by-unpinned-key"] {
		t.Fatal("missing named refusal")
	}
	s := vector.Settlement
	if s == nil || s.Receipt == nil || s.Header == nil || s.ExecutionHeader == nil || s.ExecutionCommit == nil || s.Commit == nil || s.InsufficientCommit == nil || s.ExecTxResult == nil || len(s.Validators) == 0 {
		t.Fatal("missing settled payExact receipt, execution proofs, committed header, commit or validator set")
	}
	if !bytes.Equal(s.Receipt.Receipt, positive.Receipt) || s.Receipt.Rung != evmrpc.RungCommitted {
		t.Fatal("settlement must authenticate the positive committed receipt")
	}
	keyID := sha256.Sum256(vector.Authorization.PublicKey[:])
	pins := seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: vector.Authorization.PublicKey[:], FirstHeight: 1, LastHeight: 1<<63 - 1}}
	if err := s.Receipt.Verify(pins); err != nil {
		t.Fatalf("settled RPC receipt: %v", err)
	}
	if s.Receipt.BlockNumber == nil || s.Receipt.BlockHash == nil || int64(*s.Receipt.BlockNumber) != s.ExecutionHeader.Height || !bytes.Equal(s.Receipt.BlockHash.Bytes(), s.ExecutionHeader.Hash()) {
		t.Fatal("receipt does not identify the proven execution block")
	}
	if s.Header.ChainID != fixtureChainID || s.ExecutionHeader.ChainID != fixtureChainID || s.Header.Height != s.ExecutionHeader.Height+1 || !bytes.Equal(s.Header.LastBlockID.Hash, s.ExecutionHeader.Hash()) {
		t.Fatal("committed header H must follow and bind the execution block")
	}
	if !bytes.Equal(s.Commit.BlockID.Hash, s.Header.Hash()) || s.Commit.Height != s.Header.Height {
		t.Fatal("commit is not for header H")
	}
	set := tmtypes.NewValidatorSet(s.Validators)
	if !bytes.Equal(set.Hash(), s.Header.ValidatorsHash) {
		t.Fatal("validator set does not match header H")
	}
	if err := set.VerifyCommit(fixtureChainID, s.Commit.BlockID, s.Header.Height, s.Commit); err != nil {
		t.Fatalf("positive Comet commit: %v", err)
	}
	if s.ExecutionCommit.Height != s.ExecutionHeader.Height || !bytes.Equal(s.ExecutionCommit.BlockID.Hash, s.ExecutionHeader.Hash()) {
		t.Fatal("execution commit is not for the execution header")
	}
	if err := set.VerifyCommit(fixtureChainID, s.ExecutionCommit.BlockID, s.ExecutionHeader.Height, s.ExecutionCommit); err != nil {
		t.Fatalf("positive Comet execution commit: %v", err)
	}
	if !bytes.Equal(s.Header.LastCommitHash, s.ExecutionCommit.Hash()) {
		t.Fatal("committed header does not bind the execution commit")
	}
	if !reflect.DeepEqual(s.InsufficientCommit.BlockID, s.Commit.BlockID) || s.InsufficientCommit.Height != s.Commit.Height || s.InsufficientCommit.Round != s.Commit.Round || len(s.InsufficientCommit.Signatures) != len(s.Validators) {
		t.Fatal("insufficient-power refusal changed commit identity")
	}
	absentJSON, err := json.Marshal(tmtypes.NewCommitSigAbsent())
	if err != nil {
		t.Fatal(err)
	}
	var absent tmtypes.CommitSig
	if err := json.Unmarshal(absentJSON, &absent); err != nil {
		t.Fatal(err)
	}
	var signedPower int64
	for i, sig := range s.InsufficientCommit.Signatures {
		if err := sig.ValidateBasic(); err != nil {
			t.Fatalf("invalid insufficient-power commit signature: %v", err)
		}
		if sig.BlockIDFlag == tmtypes.BlockIDFlagCommit {
			if !reflect.DeepEqual(sig, s.Commit.Signatures[i]) {
				t.Fatal("insufficient-power commit introduced a new signature")
			}
			signedPower += s.Validators[i].VotingPower
		} else if !reflect.DeepEqual(sig, absent) {
			t.Fatal("refusal must retain signatures or mark them absent")
		}
	}
	if signedPower*3 > set.TotalVotingPower()*2 {
		t.Fatal("refusal carries more than two thirds voting power")
	}
	if err := set.VerifyCommit(fixtureChainID, s.Commit.BlockID, s.Header.Height, s.InsufficientCommit); !tmtypes.IsErrNotEnoughVotingPowerSigned(err) {
		t.Fatalf("want insufficient voting power refusal, got %v", err)
	}
	if err := s.BatchProof.Validate(s.ExecutionHeader.DataHash); err != nil {
		t.Fatalf("MsgSequenceBatch inclusion: %v", err)
	}
	batch, err := seqtypes.DecodeSequenceBatchTx(s.BatchProof.Data)
	if err != nil || !reflect.DeepEqual(batch, s.Batch) {
		t.Fatalf("batch bytes differ from recorded fields: %v", err)
	}
	if s.BatchProof.Proof.Index != 0 || batch.Height != uint64(s.ExecutionHeader.Height) || receipt.Receipt.GlobalSequence < batch.FromSeq || receipt.Receipt.GlobalSequence > batch.ToSeq {
		t.Fatal("batch does not cover receipt at tx zero")
	}
	if err := batch.VerifySignature(fixtureChainID, vector.Authorization.PublicKey); err != nil {
		t.Fatalf("batch signature: %v", err)
	}
	if err := s.TransactionProof.Validate(s.ExecutionHeader.DataHash); err != nil {
		t.Fatalf("payment transaction inclusion: %v", err)
	}
	if s.TransactionProof.Proof.Index != int64(s.Receipt.Index) || s.ExecTxResultProof.Index != int64(s.Receipt.Index) || s.ExecTxResultProof.Total != s.TransactionProof.Proof.Total || s.BatchProof.Proof.Total != s.TransactionProof.Proof.Total {
		t.Fatal("transaction and result proofs must identify the receipt index")
	}
	if s.TransactionProof.Proof.Index < 1 || receipt.Receipt.GlobalSequence != batch.FromSeq+uint64(s.TransactionProof.Proof.Index)-1 || uint64(s.TransactionProof.Proof.Total) != batch.ToSeq-batch.FromSeq+2 {
		t.Fatal("payment sequence and transaction position differ from MsgSequenceBatch")
	}
	encoded, err := abci.MarshalTxResults([]*abci.ExecTxResult{s.ExecTxResult})
	if err != nil || len(encoded) != 1 || !bytes.Equal(encoded[0], s.ExecTxResultBytes) {
		t.Fatalf("ExecTxResult deterministic encoding: %v", err)
	}
	if s.ExecTxResult.Code != 0 || s.ExecTxResult.GasUsed != int64(s.Receipt.GasUsed) {
		t.Fatal("payment execution result is unsuccessful or has different gas")
	}
	if err := s.ExecTxResultProof.Verify(s.Header.LastResultsHash, s.ExecTxResultBytes); err != nil {
		t.Fatalf("ExecTxResult inclusion: %v", err)
	}
	var raw ethtypes.Transaction
	if err := raw.UnmarshalBinary(s.RawTransaction); err != nil {
		t.Fatal(err)
	}
	if raw.Hash() != common.Hash(receipt.Receipt.ActivityID) || raw.Hash() != s.Receipt.TxHash || raw.To() == nil || *raw.To() != common.HexToAddress(layerxledger.LedgerAddress) {
		t.Fatal("payment raw transaction is not bound to the receipt")
	}
	ledgerABI := layerxledger.GetABI()
	method := ledgerABI.Methods["payExact"]
	if len(raw.Data()) != 4+6*32 || !bytes.Equal(raw.Data()[:4], method.ID) {
		t.Fatal("settled transaction is not payExact")
	}
	inputs, err := method.Inputs.Unpack(raw.Data()[4:])
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := app.MakeEncodingConfig().TxConfig.TxDecoder()(s.TransactionProof.Data)
	if err != nil {
		t.Fatalf("decode committed Cosmos transaction: %v", err)
	}
	msg := evmtypes.GetEVMTransactionMessage(decoded)
	if msg == nil {
		t.Fatal("committed transaction is not an EVM transaction")
	}
	committed, _ := msg.AsTransaction()
	if committed == nil || committed.Hash() != raw.Hash() {
		t.Fatal("committed EVM transaction differs from payment")
	}
	paymentEvent := ledgerABI.Events["PaymentExact"].ID
	found := false
	for _, log := range s.Receipt.Logs {
		if log.Address == *raw.To() && len(log.Topics) == 4 && log.Topics[0] == paymentEvent {
			if found || log.Topics[1] != common.Hash(inputs[5].([32]byte)) || log.Topics[2] != common.Hash(inputs[0].([32]byte)) || log.Topics[3] != common.Hash(inputs[1].([32]byte)) {
				t.Fatal("PaymentExact must uniquely bind the payment reference and accounts")
			}
			values, err := ledgerABI.Events["PaymentExact"].Inputs.NonIndexed().Unpack(log.Data)
			if err != nil || len(values) != 2 {
				t.Fatalf("decode PaymentExact data: %v", err)
			}
			if values[0].([32]byte) != inputs[2].([32]byte) || values[1].(*big.Int).Cmp(inputs[3].(*big.Int)) != 0 {
				t.Fatal("PaymentExact asset or amount differs from payExact")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("settled payExact receipt lacks its PaymentExact log")
	}
}

func TestRepairM_486_CommittedVectorChainID(t *testing.T) {
	data, err := os.ReadFile("vectors/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector v4Vector
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	if vector.Settlement == nil || vector.Settlement.Receipt == nil {
		t.Fatal("missing committed settlement")
	}
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(vector.Settlement.RawTransaction); err != nil {
		t.Fatal(err)
	}
	t.Run("chain-125", func(t *testing.T) {
		if tx.ChainId().Cmp(big.NewInt(125)) != 0 {
			t.Fatalf("committed transaction chain id = %s, want 125", tx.ChainId())
		}
	})
	t.Run("EIP-1559", func(t *testing.T) {
		if tx.Type() != ethtypes.DynamicFeeTxType {
			t.Fatalf("committed transaction type = %d, want 2", tx.Type())
		}
	})
	if tx.Hash() != vector.Settlement.Receipt.TxHash {
		t.Fatal("committed transaction hash differs from receipt")
	}
}

func TestRepairM_486_DerivedCopies(t *testing.T) {
	source, err := os.ReadFile("vectors/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	explorer, err := os.ReadFile("../../apps/explorer/ui/tx/details/fixtures/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, explorer) {
		t.Fatal("explorer must retain the exact producer recording")
	}
	var recorded v4Vector
	if err := json.Unmarshal(source, &recorded); err != nil {
		t.Fatal(err)
	}
	var converted struct {
		Provenance struct {
			SHA256 string `json:"sha256"`
		} `json:"provenance"`
		Positive struct {
			RawTransaction hexutil.Bytes `json:"rawTransaction"`
			SignedReceipt  hexutil.Bytes `json:"signedReceipt"`
			ReceiptDigest  hexutil.Bytes `json:"receiptDigest"`
		} `json:"positive"`
	}
	data, err := os.ReadFile("../../sdk/platform/conformance/fixtures/vector-v4-committed.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &converted); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(source))
	if converted.Provenance.SHA256 != digest {
		t.Fatal("conformance source digest differs from producer recording")
	}
	if recorded.Settlement == nil || recorded.Settlement.Receipt == nil {
		t.Fatal("missing producer settlement")
	}
	if !bytes.Equal(converted.Positive.RawTransaction, recorded.Settlement.RawTransaction) || !bytes.Equal(converted.Positive.SignedReceipt, recorded.Settlement.Receipt.Receipt) {
		t.Fatal("conformance transaction or receipt differs from producer recording")
	}
	verified, err := verify.ReceiptSignature(converted.Positive.SignedReceipt, recorded.Authorization.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(converted.Positive.ReceiptDigest, verified.Digest[:]) {
		t.Fatal("conformance receipt digest differs from authenticated bytes")
	}
	var portable struct {
		Provenance struct {
			CommittedSHA256 string `json:"committedSha256"`
			ReceiptedSHA256 string `json:"receiptedSha256"`
		} `json:"provenance"`
		Bundles []struct {
			Level          string        `json:"verificationLevel"`
			RawTransaction hexutil.Bytes `json:"rawTransaction"`
			SignedReceipt  hexutil.Bytes `json:"signedReceipt"`
			ReceiptDigest  hexutil.Bytes `json:"receiptDigest"`
		} `json:"bundles"`
	}
	data, err = os.ReadFile("../../sdk/platform/conformance/fixtures/vector-v5-portable.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &portable); err != nil {
		t.Fatal(err)
	}
	receipted, err := os.ReadFile("../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if portable.Provenance.CommittedSHA256 != digest || portable.Provenance.ReceiptedSHA256 != fmt.Sprintf("%x", sha256.Sum256(receipted)) {
		t.Fatal("portable provenance differs from source recordings")
	}
	if len(portable.Bundles) != 2 || portable.Bundles[1].Level != "committed" {
		t.Fatal("missing portable committed case")
	}
	bundle := portable.Bundles[1]
	if !bytes.Equal(bundle.RawTransaction, converted.Positive.RawTransaction) || !bytes.Equal(bundle.SignedReceipt, converted.Positive.SignedReceipt) || !bytes.Equal(bundle.ReceiptDigest, converted.Positive.ReceiptDigest) {
		t.Fatal("portable committed case differs from V4")
	}
}
