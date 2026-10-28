package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestVectorV2ReceiptedRevertVerifies(t *testing.T) {
	data, err := os.ReadFile("vectors/vector-v2-receipted-revert.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectorV2
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if err := verifyVectorV2(&v); err != nil {
		t.Fatal(err)
	}
	if vectors[vectorV2Name] == nil {
		t.Fatal("V2 recorder is not registered")
	}
	r := &v.Receipt
	verified, err := verify.ReceiptSignature(r.Receipt, [32]byte(r.SigningKey))
	if err != nil {
		t.Fatalf("pinned receipt signature: %v", err)
	}
	if r.Status != 0 || verified.Receipt.ResultCode != 1 {
		t.Fatal("signed V2 receipt must report a revert")
	}
	if sha256.Sum256(r.ReturnData) != verified.Receipt.ActivityRoot {
		t.Fatal("signed receipt lost the revert data digest")
	}
	reason, err := abi.UnpackRevert(r.ReturnData)
	if err != nil || reason != v.RevertReason {
		t.Fatalf("Error(string) revert data: %q, %v", reason, err)
	}
	if !bytes.HasPrefix(r.ReturnData, []byte{0x08, 0xc3, 0x79, 0xa0}) {
		t.Fatal("revert is not Error(string)")
	}
	for _, mutation := range []string{"signature", "status", "returnData", "reason", "rawTransaction", "pin", "height"} {
		t.Run(mutation, func(t *testing.T) {
			var changed vectorV2
			if err := json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "signature":
				changed.Receipt.Receipt[len(changed.Receipt.Receipt)-1] ^= 1
			case "status":
				changed.Receipt.Status = 1
			case "returnData":
				changed.Receipt.ReturnData[len(changed.Receipt.ReturnData)-1] ^= 1
			case "reason":
				changed.RevertReason += " altered"
			case "rawTransaction":
				changed.RawTransaction[len(changed.RawTransaction)-1] ^= 1
			case "pin":
				changed.ReceiptKeys[0].KeyId[0] ^= 1
			case "height":
				changed.Provenance.LastHeight = uint64(changed.Receipt.Height) - 1
			}
			if err := verifyVectorV2(&changed); err == nil {
				t.Fatalf("accepted altered %s", mutation)
			}
		})
	}
}
