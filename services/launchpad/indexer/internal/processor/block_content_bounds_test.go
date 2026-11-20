package processor

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

func blockContentContract(token, recipient common.Address) []byte {
	// Calldata specifies the data length and number of LOG0 entries. The final
	// call transfers one token so the receipt also contains an authorized log.
	code := []byte{0x60, 0x20, 0x35, 0x60, 0x00, 0x52}
	loop := len(code)
	code = append(code, 0x5b, 0x60, 0x00, 0x51, 0x15, 0x61, 0x00, 0x00, 0x57)
	endJump := len(code) - 3
	code = append(code, 0x60, 0x00, 0x35, 0x60, 0x40, 0xa0)
	code = append(code, 0x60, 0x00, 0x51, 0x60, 0x01, 0x90, 0x03, 0x60, 0x00, 0x52)
	code = append(code, 0x61, byte(loop>>8), byte(loop), 0x56)
	end := len(code)
	code[endJump], code[endJump+1] = byte(end>>8), byte(end)
	code = append(code, 0x5b, 0x63, 0xa9, 0x05, 0x9c, 0xbb, 0x60, 0xe0, 0x1b, 0x60, 0x00, 0x52, 0x73)
	code = append(code, recipient[:]...)
	code = append(code, 0x60, 0x04, 0x52, 0x60, 0x01, 0x60, 0x24, 0x52)
	code = append(code, 0x60, 0x20, 0x60, 0x00, 0x60, 0x44, 0x60, 0x00, 0x60, 0x00, 0x73)
	code = append(code, token[:]...)
	code = append(code, 0x5a, 0xf1, 0x15, 0x61, 0x00, 0x00, 0x57)
	callJump := len(code) - 3
	code = append(code, 0x60, 0x00, 0x51, 0x15, 0x61, 0x00, 0x00, 0x57, 0x00)
	resultJump := len(code) - 4
	failure := len(code)
	code[callJump], code[callJump+1] = byte(failure>>8), byte(failure)
	code[resultJump], code[resultJump+1] = byte(failure>>8), byte(failure)
	code = append(code, 0x5b, 0x60, 0x00, 0x60, 0x00, 0xfd)
	init := []byte{0x61, byte(len(code) >> 8), byte(len(code)), 0x61, 0x00, 0x0f, 0x60, 0x00, 0x39, 0x61, byte(len(code) >> 8), byte(len(code)), 0x60, 0x00, 0xf3}
	return append(init, code...)
}

func TestBlockContentBoundsQualifiedSourceAndProcessor(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	n := h.node
	token := common.HexToAddress(n.roles["SidioraERC20"].Address)
	emptyABI, err := abi.JSON(strings.NewReader(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	address, tx, contract, err := bind.DeployContract(n.auth, emptyABI, blockContentContract(token, n.owner), n.client)
	if err != nil {
		t.Fatal(err)
	}
	n.mined(t, tx)
	funded := n.transact(t, "SidioraERC20", "transfer", address, big.NewInt(10))
	if err := h.proc.ProcessBatch(n.ctx, h.created.BlockNumber.Int64()+1, funded.BlockNumber.Int64()); err != nil {
		t.Fatal(err)
	}
	manifest := strings.Repeat("a", 64)
	epochs := make([]source.AddressEpoch, 0, len(n.options.Epochs))
	for _, e := range n.options.Epochs {
		epochs = append(epochs, source.AddressEpoch{Address: strings.ToLower(e.Deployment.Address.Hex()), Role: e.Deployment.Role, ABIRevision: e.Deployment.ABIRevision, ManifestDigest: manifest, ActivationHeight: e.Deployment.From.Block})
	}
	committed, err := source.NewCommittedSource(n.ctx, source.CommittedOptions{Endpoints: map[string]string{"primary": n.endpoint}, GenesisHash: n.options.GenesisHash, ManifestDigest: manifest, Epochs: epochs})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := committed.Close(); err != nil {
			t.Error(err)
		}
	})
	durable := funded.BlockNumber.Uint64()
	qualifiedSource := &source.LiveQualifiedSource{RPC: committed, Authority: h.authority, DurableHeight: func() uint64 { return durable }}
	h.proc.deps.QualifiedBatchSource = qualifiedSource
	for _, tc := range []struct {
		name            string
		logs, dataBytes int
	}{{"log data", 1, (64 << 10) + 1}, {"receipt logs", 4097, 0}, {"raw summary", 1, decode.MaxEnvelopeBytes + 1}} {
		t.Run(tc.name, func(t *testing.T) {
			input := make([]byte, 64)
			new(big.Int).SetInt64(int64(tc.dataBytes)).FillBytes(input[:32])
			new(big.Int).SetInt64(int64(tc.logs)).FillBytes(input[32:])
			n.auth.GasLimit = 15000000
			tx, err := contract.RawTransact(n.auth, input)
			n.auth.GasLimit = 0
			if err != nil {
				t.Fatal(err)
			}
			mined := n.mined(t, tx)
			if len(mined.Logs) != tc.logs+1 || mined.Logs[0].Address != address || len(mined.Logs[0].Data) != tc.dataBytes || mined.Logs[tc.logs].Address != token {
				t.Fatalf("contract log content: logs=%d", len(mined.Logs))
			}
			for _, log := range mined.Logs[:tc.logs] {
				env, err := h.authority.Registry().Decode(*log)
				if err != nil || env.Status != "opaque" || env.FailureReason != "untrusted_emitter" || env.Event != nil || env.Family != nil {
					t.Fatalf("foreign log envelope: %+v: %v", env, err)
				}
				if tc.dataBytes > decode.MaxEnvelopeBytes && (env.RawSummary == nil || env.RawSummary.DataBytes != tc.dataBytes || env.Raw.Data != "") {
					t.Fatal("foreign log summary required")
				}
			}
			height := mined.BlockNumber.Uint64()
			batch, receipts, err := qualifiedSource.FetchQualifiedBatch(n.ctx, durable+1, height)
			if err != nil || len(receipts) != 1 || len(batch.Records) != 1 || batch.Records[0].Emitter != strings.ToLower(token.Hex()) {
				t.Fatalf("qualified block content: receipts=%d records=%d: %v", len(receipts), len(batch.Records), err)
			}
			events, err := receipts[0].QualifiedEvents(n.ctx)
			if err != nil || len(events) != 1 || events[0].EventName != "Transfer" {
				t.Fatalf("authorized event after opaque logs: %d: %v", len(events), err)
			}
			if err := h.proc.ProcessBatch(n.ctx, int64(durable)+1, int64(height)); err != nil {
				t.Fatal(err)
			}
			watermark, err := h.proc.deps.Store.GetCanonicalWatermark(n.ctx, 125, realStream)
			if err != nil || watermark != int64(height) {
				t.Fatalf("block content watermark: %d: %v", watermark, err)
			}
			durable = height
		})
	}
}
