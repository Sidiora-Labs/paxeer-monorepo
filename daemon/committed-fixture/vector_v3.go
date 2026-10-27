package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type v3Refusal struct {
	Name        string                        `json:"name"`
	Expected    string                        `json:"expected"`
	ReceiptJSON evmrpc.SequencerReceiptResult `json:"receiptJson"`
}

type v3Vector struct {
	Name           string                        `json:"name"`
	Provenance     json.RawMessage               `json:"provenance"`
	RawTransaction hexutil.Bytes                 `json:"rawTransaction"`
	ReceiptKeys    seqkeeper.ReceiptKeys         `json:"receiptKeys"`
	Base           evmrpc.SequencerReceiptResult `json:"receiptJson"`
	Cases          []v3Refusal                   `json:"cases"`
}

var errV3Activity = errors.New("activity_id is not keccak256 of the raw transaction")
var errV3Sequence = errors.New("sequence hash mismatch")

func init() {
	registerVector("vector-v3-receipt-refusals", recordV3)
}

func recordV3(n *node) (any, error) {
	record, ok := vectors["vector-v1-receipted-positive"]
	if !ok {
		return nil, errors.New("V3 requires the vector-v1-receipted-positive recorder")
	}
	value, err := record(n)
	if err != nil {
		return nil, fmt.Errorf("record V1 for V3: %w", err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var vector v3Vector
	if err := json.Unmarshal(data, &vector); err != nil {
		return nil, fmt.Errorf("decode V1 rawTransaction and receiptJson: %w", err)
	}
	var genesis struct {
		AppState struct {
			Sequencer struct {
				ReceiptKeys seqkeeper.ReceiptKeys `json:"receipt_keys"`
			} `json:"sequencer"`
		} `json:"app_state"`
	}
	if err := readJSON(filepath.Join(n.home, "config", "genesis.json"), &genesis); err != nil {
		return nil, err
	}
	vector.ReceiptKeys = genesis.AppState.Sequencer.ReceiptKeys
	private, err := v3ReceiptPrivateKey(n)
	if err != nil {
		return nil, err
	}
	defer clear(private)
	return deriveV3(vector, private)
}

func v3ReceiptPrivateKey(n *node) (ed25519.PrivateKey, error) {
	info, err := os.Stat(n.receiptKeyFile)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("V3 receipt seed must be an owner-only regular file")
	}
	raw, err := os.ReadFile(n.receiptKeyFile)
	if err != nil {
		return nil, err
	}
	defer clear(raw)
	seed, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(seed) != ed25519.SeedSize {
		clear(seed)
		return nil, errors.New("V3 receipt key must contain a 32-byte Ed25519 seed")
	}
	private := ed25519.NewKeyFromSeed(seed)
	clear(seed)
	if !bytes.Equal(private.Public().(ed25519.PublicKey), n.receiptPublicKey) {
		clear(private)
		return nil, errors.New("V3 receipt seed differs from the recorder's generated key")
	}
	return private, nil
}

func verifyV3Receipt(vector *v3Vector, view *evmrpc.SequencerReceiptResult) error {
	if len(vector.RawTransaction) == 0 || view.SequencerReceiptView == nil {
		return errors.New("V3 requires a raw transaction and receipt view")
	}
	if err := vector.ReceiptKeys.Validate(); err != nil {
		return err
	}
	verified, err := vector.ReceiptKeys.VerifyReceipt(view.Receipt)
	if err != nil {
		return err
	}
	receipt := verified.Receipt
	if receipt.ActivityID != [32]byte(crypto.Keccak256Hash(vector.RawTransaction)) {
		return errV3Activity
	}
	if receipt.ResultingStateRoot != seqtypes.SeqHash(receipt.PreviousStateRoot, receipt.GlobalSequence, receipt.ActivityID) {
		return errV3Sequence
	}
	return view.Verify(vector.ReceiptKeys)
}

func v3Sign(unsigned []byte, private ed25519.PrivateKey) ([]byte, error) {
	receipt, err := codec.DecodeReceipt(unsigned)
	if err != nil {
		return nil, err
	}
	if receipt.SequencerSignature != nil {
		return nil, errors.New("V3 signing requires unsigned canonical receipt bytes")
	}
	digest := receipt.Digest()
	signed := append(bytes.Clone(unsigned[:len(unsigned)-1]), 1)
	signed = binary.BigEndian.AppendUint32(signed, ed25519.SignatureSize)
	return append(signed, ed25519.Sign(private, digest[:])...), nil
}

func deriveV3(vector v3Vector, private ed25519.PrivateKey) (*v3Vector, error) {
	if len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("V3 requires the recorder's Ed25519 private key")
	}
	if err := verifyV3Receipt(&vector, &vector.Base); err != nil {
		return nil, fmt.Errorf("V3 positive baseline: %w", err)
	}
	if !bytes.Equal(private.Public().(ed25519.PublicKey), vector.Base.SigningKey) {
		return nil, errors.New("V1 receipt was not signed by this recorder's per-run key")
	}
	base, err := codec.DecodeReceipt(vector.Base.Receipt)
	if err != nil {
		return nil, err
	}
	if len(vector.Base.Logs) == 0 || len(vector.Base.Logs[0].Data) == 0 || base.Supply != nil || base.ProgramOutcome != nil {
		return nil, errors.New("V3 requires a sequencer V1 receipt with nonempty log data")
	}
	height, err := seqkeeper.ReceiptHeight(base)
	if err != nil {
		return nil, err
	}
	pin, err := vector.ReceiptKeys.KeyForHeight(base.ContextHash, height)
	if err != nil {
		return nil, err
	}
	if pin.LastHeight == math.MaxUint64 {
		return nil, errors.New("V3 cannot place a height above this key's last height")
	}
	// The sequencer layout has a 166-byte prefix, then bounded effect bodies,
	// fee_charged and the batch_id array's length prefix.
	batchOffset := 166 + 16 + 4
	for _, effect := range base.Effects {
		batchOffset += 48 + len(effect.Body)
	}
	unsigned := base.UnsignedBytes()
	contextOffset := len(unsigned) - 1 - 8 - 32
	if batchOffset+32 > len(unsigned) || !bytes.Equal(unsigned[batchOffset:batchOffset+32], base.BatchID[:]) || !bytes.Equal(unsigned[contextOffset:contextOffset+32], base.ContextHash[:]) {
		return nil, errors.New("V3 requires the sequencer receipt wire layout")
	}
	names := []string{"key-id-not-pinned", "height-outside-range", "activity-id-not-raw-hash", "sequence-hash-mismatch", "tampered-log", "non-canonical-receipt-bytes"}
	vector.Name = "vector-v3-receipt-refusals"
	vector.Cases = make([]v3Refusal, 0, len(names))
	baseJSON, err := json.Marshal(vector.Base)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		entry := v3Refusal{Name: name}
		if err := json.Unmarshal(baseJSON, &entry.ReceiptJSON); err != nil {
			return nil, err
		}
		body := bytes.Clone(unsigned)
		switch name {
		case "key-id-not-pinned":
			keyID := base.ContextHash
			for {
				keyID = codec.ReceiptDigest(keyID[:])
				found := false
				for _, other := range vector.ReceiptKeys {
					found = found || bytes.Equal(keyID[:], other.KeyId)
				}
				if !found {
					break
				}
			}
			copy(body[contextOffset:contextOffset+32], keyID[:])
			entry.ReceiptJSON.KeyID = common.Hash(keyID)
			entry.Expected = seqkeeper.ErrReceiptKeyNotPinned.Error()
		case "height-outside-range":
			binary.BigEndian.PutUint64(body[batchOffset+24:batchOffset+32], pin.LastHeight+1)
			entry.ReceiptJSON.Height = hexutil.Uint64(pin.LastHeight + 1)
			entry.Expected = seqkeeper.ErrReceiptKeyNotPinned.Error()
		case "activity-id-not-raw-hash":
			body[10] ^= 1
			entry.ReceiptJSON.TxHash = common.BytesToHash(body[10:42])
			entry.Expected = errV3Activity.Error()
		case "sequence-hash-mismatch":
			body[90] ^= 1
			entry.Expected = errV3Sequence.Error()
		case "tampered-log":
			entry.ReceiptJSON.Logs[0].Data[0] ^= 1
			entry.Expected = evmrpc.ErrSequencerReceiptMismatch.Error()
		case "non-canonical-receipt-bytes":
			entry.Expected = codec.ErrNonCanonical.Error()
		}
		entry.ReceiptJSON.Receipt, err = v3Sign(body, private)
		if err != nil {
			return nil, fmt.Errorf("sign %s: %w", name, err)
		}
		if name == "non-canonical-receipt-bytes" {
			entry.ReceiptJSON.Receipt[len(entry.ReceiptJSON.Receipt)-69] = 2
		}
		if err := verifyV3Receipt(&vector, &entry.ReceiptJSON); err == nil || !strings.Contains(err.Error(), entry.Expected) {
			return nil, fmt.Errorf("%s: want refusal %q, got %v", name, entry.Expected, err)
		}
		vector.Cases = append(vector.Cases, entry)
	}
	return &vector, nil
}
