package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
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
	vector_v2_test(t, &v, verified.Digest)
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

func vector_v2_test(t *testing.T, v *vectorV2, digest [32]byte) {
	t.Helper()
	const sourcePath = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
	data, err := os.ReadFile("../../" + sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		ChainID uint64 `json:"chainId"`
		Cases   []struct {
			ID             string                        `json:"id"`
			RawTransaction hexutil.Bytes                 `json:"rawTransaction"`
			Receipt        evmrpc.SequencerReceiptResult `json:"receiptJson"`
			SignedReceipt  hexutil.Bytes                 `json:"signedReceipt"`
			ReceiptDigest  common.Hash                   `json:"receiptDigest"`
			PinnedKey      struct {
				KeyID       hexutil.Bytes `json:"keyId"`
				PublicKey   hexutil.Bytes `json:"publicKey"`
				FirstHeight uint64        `json:"firstHeight"`
				LastHeight  uint64        `json:"lastHeight"`
			} `json:"pinnedKey"`
			LXPCode string `json:"lxpCode"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	if (*big.Int)(v.ChainID).Cmp(new(big.Int).SetUint64(source.ChainID)) != 0 {
		t.Fatal("V2 chain ID differs from conformance source")
	}
	for _, c := range source.Cases {
		if c.ID != "V2-layerxledger-revert" {
			continue
		}
		if v.Receipt.TxHash != c.Receipt.TxHash {
			t.Fatalf("V2 transaction hash %s differs from conformance source %s", v.Receipt.TxHash, c.Receipt.TxHash)
		}
		if !bytes.Equal(v.RawTransaction, c.RawTransaction) {
			t.Fatal("V2 transaction bytes differ from conformance source")
		}
		if !reflect.DeepEqual(v.Receipt, c.Receipt) || !bytes.Equal(v.Receipt.Receipt, c.SignedReceipt) {
			t.Fatal("V2 receipt differs from conformance source")
		}
		if common.Hash(digest) != c.ReceiptDigest {
			t.Fatalf("V2 receipt digest %s differs from conformance source %s", common.Hash(digest), c.ReceiptDigest)
		}
		if len(v.ReceiptKeys) != 1 {
			t.Fatal("V2 must pin the conformance source key")
		}
		key := v.ReceiptKeys[0]
		if !bytes.Equal(key.KeyId, c.PinnedKey.KeyID) || !bytes.Equal(key.PublicKey, c.PinnedKey.PublicKey) ||
			key.FirstHeight != c.PinnedKey.FirstHeight || key.LastHeight != c.PinnedKey.LastHeight {
			t.Fatal("V2 receipt key differs from conformance source")
		}
		if v.LXPCode != c.LXPCode || v.Provenance.Source != sourcePath+"#"+c.ID {
			t.Fatal("V2 revert code or provenance differs from conformance source")
		}
		return
	}
	t.Fatal("conformance source lacks the V2 revert case")
}

// The embedded executable contains paxd's real command tree. The bounded repair
// gate exercises the same recorder and strict consumers in process below.
func TestVectorV2RecordsPrimaryStandby(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	n, err := prepareHome(ctx, executable)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(n.home) })
	_, err = n.prepareStandby()
	require.NoError(t, err)
	require.NoError(t, n.configureV2Genesis())
	stop, err := n.start()
	require.NoError(t, err)
	t.Cleanup(stop)
	value, err := recordV2FromHome(n, "4299cb15eced23eb9a839162acd11e9a937edd05")
	require.NoError(t, err)
	v := value.(*vectorV2)
	require.Len(t, v.ReceiptKeys, 2)
	require.GreaterOrEqual(t, uint64(v.Receipt.Height), uint64(3))
	repairM1092VerifyV2Copies(t, v)
}
