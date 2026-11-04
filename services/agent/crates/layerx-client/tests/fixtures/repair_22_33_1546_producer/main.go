// Integration producer for repair_22_31_222: real MemIAVL, SDK multistore,
// Comet header/vote codecs, and the node's MsgSequenceBatch codec.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	tmed "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	tmversion "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/memiavl"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
func hx(value []byte) string { return "0x" + hex.EncodeToString(value) }
func unhex(value string) []byte {
	if len(value) > 1 && value[:2] == "0x" {
		value = value[2:]
	}
	return must(hex.DecodeString(value))
}
func n(value any) uint64 { return must(strconv.ParseUint(value.(string)[2:], 16, 64)) }

const chain = "hyperpax_125-1"

var validatorKey = must(tmed.SecretKeyFromSecretBytes(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x72}, 32))))

func committed(height uint64, appHash, dataHash []byte) map[string]any {
	validator := tmtypes.NewValidator(validatorKey.Public(), 10)
	set := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	nonzero := bytes.Repeat([]byte{0x33}, 32)
	header := &tmtypes.Header{
		Version: tmversion.Consensus{Block: 11, App: 1}, ChainID: chain, Height: int64(height), Time: time.Unix(1000, 0).UTC(),
		LastBlockID:    tmtypes.BlockID{Hash: nonzero, PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: nonzero}},
		LastCommitHash: nonzero, DataHash: dataHash, ValidatorsHash: set.Hash(), NextValidatorsHash: set.Hash(),
		ConsensusHash: nonzero, AppHash: appHash, LastResultsHash: nonzero, EvidenceHash: nonzero, ProposerAddress: validator.Address,
	}
	blockID := tmtypes.BlockID{Hash: header.Hash(), PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: nonzero}}
	vote := &tmtypes.Vote{Type: tmproto.PrecommitType, Height: int64(height), Round: 0, BlockID: blockID,
		Timestamp: header.Time, ValidatorAddress: validator.Address, ValidatorIndex: 0}
	vote.Signature = utils.Some(validatorKey.Sign(tmtypes.VoteSignBytes(chain, vote.ToProto())))
	commit := &tmtypes.Commit{Height: int64(height), Round: 0, BlockID: blockID, Signatures: []tmtypes.CommitSig{vote.CommitSig()}}
	signed := must(tmjson.Marshal(&tmtypes.SignedHeader{Header: header, Commit: commit}))
	var signedJSON any
	if err := json.Unmarshal(signed, &signedJSON); err != nil {
		panic(err)
	}
	return map[string]any{
		"height": height, "hash": hx(header.Hash()), "appHash": hx(appHash),
		"material": map[string]any{"height": fmt.Sprint(height), "signedHeader": signedJSON},
		"validators": map[string]any{"height": fmt.Sprint(height), "total": "1", "validators": []any{map[string]any{
			"address": hex.EncodeToString(validator.Address), "pubKey": map[string]any{"type": "tendermint/PubKeyEd25519", "value": []byte(validatorKey.Public().Bytes())}, "votingPower": "10"}}},
		"block": map[string]any{"number": fmt.Sprintf("%#x", height), "hash": hx(header.Hash()), "parentHash": hx(nonzero), "timestamp": "0x3e8", "transactionsRoot": hx(dataHash), "receiptsRoot": hx(nonzero), "transactions": []any{}},
	}
}

func main() {
	tree := memiavl.NewWithInitialVersion(1200)
	defer func() {
		if err := tree.Close(); err != nil {
			panic(err)
		}
	}()
	address := make([]byte, 20)
	address[18] = 0x10
	address[19] = 1
	keys := [][]byte{[]byte("record-A"), []byte("record-B")}
	slots := make([][]byte, len(keys))
	stateKeys := make([][]byte, len(keys))
	values := make([][]byte, len(keys))
	for i, key := range keys {
		slots[i] = crypto.Keccak256(append([]byte{0, 1}, key...))
		stateKeys[i] = append(append([]byte{3}, address...), slots[i]...)
		values[i] = make([]byte, 32)
		values[i][31] = byte(i + 17)
		tree.Set(stateKeys[i], values[i])
	}
	root, version, err := tree.SaveVersion(true)
	if err != nil {
		panic(err)
	}
	ci := storetypes.CommitInfo{Version: version, StoreInfos: []storetypes.StoreInfo{{Name: "evm", CommitId: storetypes.CommitID{Version: version, Hash: root}}}}
	outer := ci.ProofOp("evm")
	responses := make([]any, len(keys))
	for i, key := range stateKeys {
		proof := must(tree.GetMembershipProof(key))
		inner := storetypes.NewIavlCommitmentOp(key, proof).ProofOp()
		responses[i] = map[string]any{"address": hx(address), "hexValues": []string{hex.EncodeToString(values[i])}, "storageProof": []any{map[string]any{"ops": []any{inner, outer}}}}
	}
	empty := sha256.Sum256(nil)
	storage := committed(uint64(version), ci.Hash(), empty[:])
	storage["storeRoot"] = hx(root)
	storage["address"] = hx(address)
	storage["keys"] = keys
	storage["slots"] = []string{hx(slots[0]), hx(slots[1])}
	storage["responses"] = responses
	storage["values"] = []string{hx(values[0]), hx(values[1])}
	fixture := must(os.ReadFile("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"))
	var records struct {
		Cases []struct {
			ReceiptJSON map[string]any `json:"receiptJson"`
			PrevSeqHash string         `json:"prevSeqHash"`
			PinnedKey   any            `json:"pinnedKey"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(fixture, &records); err != nil {
		panic(err)
	}
	record := records.Cases[0]
	seq := n(record.ReceiptJSON["seq"])
	height := n(record.ReceiptJSON["height"]) + 1
	id := [32]byte(unhex(record.ReceiptJSON["txHash"].(string)))
	nextID := sha256.Sum256([]byte("another sequenced activity"))
	ids := [][32]byte{id, nextID}
	previous := [32]byte(unhex(record.PrevSeqHash))
	last := seqtypes.SeqHash(seqtypes.SeqHash(previous, seq, id), seq+1, nextID)
	txRoot := seqtypes.TxRoot(ids)
	batch := seqtypes.MsgSequenceBatch{Height: height, FromSeq: seq, ToSeq: seq + 1, PrevSeqHash: previous[:], LastSeqHash: last[:], TxRoot: txRoot[:], KeyId: unhex(record.ReceiptJSON["keyId"].(string))}
	digest := must(batch.SignBytes(chain))
	batch.Sig = ed25519.Sign(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x5e}, 32)), digest[:])
	batchTx := must(seqtypes.EncodeSequenceBatchTx(batch))
	txs := tmtypes.Txs{tmtypes.Tx(batchTx)}
	receipt := committed(height, ci.Hash(), txs.Hash())
	receipt["batch"] = hx(batchTx)
	receipt["proof"] = txs.Proof(0).Proof
	landing := &ethtypes.Receipt{Type: 2, Status: n(record.ReceiptJSON["status"]), BlockNumber: new(big.Int).SetUint64(height - 1), TransactionIndex: uint(n(record.ReceiptJSON["index"])), GasUsed: n(record.ReceiptJSON["gasUsed"]), TxHash: common.BytesToHash(id[:]), Logs: make([]*ethtypes.Log, 0)}
	receipt["landing"] = json.RawMessage(must(landing.MarshalJSON()))
	receipt["activityIds"] = []string{hx(id[:]), hx(nextID[:])}
	receipt["view"] = record.ReceiptJSON
	receipt["key"] = record.PinnedKey
	variants := make(map[string]any)
	for _, kind := range []string{"sequence", "root"} {
		variant := batch
		if kind == "sequence" {
			variant.FromSeq++
			variant.ToSeq++
		} else {
			variant.PrevSeqHash = bytes.Clone(batch.PrevSeqHash)
			variant.PrevSeqHash[0] ^= 1
		}
		variantLast := seqtypes.SeqHash(seqtypes.SeqHash([32]byte(variant.PrevSeqHash), variant.FromSeq, id), variant.ToSeq, nextID)
		variant.LastSeqHash = variantLast[:]
		variantDigest := must(variant.SignBytes(chain))
		variant.Sig = ed25519.Sign(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x5e}, 32)), variantDigest[:])
		variantTx := must(seqtypes.EncodeSequenceBatchTx(variant))
		variantTxs := tmtypes.Txs{tmtypes.Tx(variantTx)}
		produced := committed(height, ci.Hash(), variantTxs.Hash())
		produced["batch"] = hx(variantTx)
		produced["proof"] = variantTxs.Proof(0).Proof
		produced["activityIds"] = receipt["activityIds"]
		produced["view"] = record.ReceiptJSON
		produced["key"] = record.PinnedKey
		produced["landing"] = receipt["landing"]
		variants[kind] = produced
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"storage": storage, "receipt": receipt, "variants": variants}); err != nil {
		panic(err)
	}
}
