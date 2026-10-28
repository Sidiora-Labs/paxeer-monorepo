package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestVectorV1ReceiptedPositiveVerifies(t *testing.T) {
	var vector v1Vector
	if err := readJSON("vectors/"+v1Name+".json", &vector); err != nil {
		t.Fatal(err)
	}
	if vector.Name != v1Name || vectors[v1Name] == nil {
		t.Fatal("V1 vector name and recorder registration must agree")
	}
	p := vector.Provenance
	if revision, err := hex.DecodeString(p.PaxdRevision); err != nil || len(revision) != 20 {
		t.Fatal("provenance must name the full paxd Git revision")
	}
	if p.ChainID != fixtureChainID || !strings.Contains(p.RecorderCommand, "committed-fixture record --paxd ") ||
		!strings.Contains(p.RecorderCommand, " --out ") || p.FirstHeight == 0 || p.LastHeight < p.FirstHeight {
		t.Fatal("provenance must record the fixture chain, heights and recorder command")
	}
	if len(vector.ReceiptKeys) != 1 {
		t.Fatal("V1 must pin exactly one receipt key")
	}
	key := vector.ReceiptKeys[0]
	if len(key.PublicKey) != 32 {
		t.Fatal("pinned public key must be 32 bytes")
	}
	pin := seqtypes.ReceiptKey{KeyId: [32]byte(key.KeyID), PublicKey: [32]byte(key.PublicKey), FirstHeight: key.FirstHeight, LastHeight: key.LastHeight}
	if sha256.Sum256(pin.PublicKey[:]) != pin.KeyId {
		t.Fatal("key id must be the SHA-256 digest of the generated public key")
	}
	verified, err := verify.ReceiptSignature(vector.SignedReceipt, pin.PublicKey)
	if err != nil {
		t.Fatalf("verify recorded receipt signature: %v", err)
	}
	receipt := verified.Receipt
	if !bytes.Equal(receipt.BatchID[:24], make([]byte, 24)) {
		t.Fatal("batch id must encode a height")
	}
	height := binary.BigEndian.Uint64(receipt.BatchID[24:])
	if !pin.Covers(height) || pin.FirstHeight != 1 || pin.LastHeight < pin.FirstHeight ||
		height < p.FirstHeight || height > p.LastHeight || receipt.ContextHash != pin.KeyId {
		t.Fatal("receipt height and key id must be authorized by the genesis pin and provenance")
	}
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(vector.RawTransaction); err != nil {
		t.Fatal(err)
	}
	canonical, err := tx.MarshalBinary()
	if err != nil || !bytes.Equal(canonical, vector.RawTransaction) || tx.To() != nil || tx.Value().Sign() != 0 ||
		!bytes.Equal(tx.Data(), common.FromHex(v1Creation)) || tx.GasPrice().Sign() <= 0 || tx.ChainId().Sign() <= 0 || p.EVMChainID != "0x"+tx.ChainId().Text(16) {
		t.Fatal("raw transaction must be the canonical, chain-bound two-log contract creation")
	}
	from, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), &tx)
	if err != nil {
		t.Fatalf("verify transaction signature: %v", err)
	}
	if receipt.ActivityID != [32]byte(crypto.Keccak256Hash(vector.RawTransaction)) ||
		receipt.ResultCode != 0 || receipt.FromSequence != tx.Nonce() || receipt.GlobalSequence == 0 || len(receipt.Effects) != 3 {
		t.Fatal("receipt must bind the successful creation, transaction nonce and two log effects")
	}
	result := vector.ReceiptJSON
	if result == nil || result.SequencerReceiptView == nil || result.Rung != evmrpc.RungReceipted {
		t.Fatal("recorded RPC answer must be receipted")
	}
	pins := seqkeeper.ReceiptKeys{{KeyId: pin.KeyId[:], PublicKey: pin.PublicKey[:], FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight}}
	if err := pins.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := result.Verify(pins); err != nil {
		t.Fatalf("verify RPC fields and log digests against the pinned receipt: %v", err)
	}
	created := crypto.CreateAddress(from, tx.Nonce())
	if !bytes.Equal(result.Receipt, vector.SignedReceipt) || result.TxHash != tx.Hash() || result.From != from ||
		result.To != created || result.ContractAddress != created || uint64(result.Height) != height || len(result.Logs) != 2 {
		t.Fatal("RPC answer must bind the raw creation transaction and both logs")
	}
	for i, log := range result.Logs {
		wantData := make([]byte, 32)
		wantData[31] = byte(i + 1)
		wantTopic := common.Hash{}
		wantTopic[31] = byte(0x11 * (i + 1))
		if log.Address != created || uint64(log.LogIndex) != uint64(i) || len(log.Topics) != 1 ||
			log.Topics[0] != wantTopic {
			t.Fatalf("log %d must have the created address, ordinal and constructor topic", i)
		}
		if !bytes.Equal(log.Data, wantData) {
			t.Fatalf("log %d data differs from constructor output", i)
		}
	}
	tampered := bytes.Clone(vector.SignedReceipt)
	tampered[len(tampered)-1] ^= 1
	if _, err := verify.ReceiptSignature(tampered, pin.PublicKey); err == nil {
		t.Fatal("changed receipt signature must be refused")
	}
}
