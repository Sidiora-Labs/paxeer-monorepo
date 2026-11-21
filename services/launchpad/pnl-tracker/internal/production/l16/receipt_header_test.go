package l16

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestReceiptHeaderDigestMatchesIndexerProvenance(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(coordinatortest.RepositoryRoot(t), "services/launchpad/indexer/internal/processor/testdata/native_accounting_batches.json"))
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Responses map[string]struct{ Result json.RawMessage }
	}
	if err = json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	var header types.Header
	if err = json.Unmarshal(packet.Responses[`eth_getBlockByNumber ["0x2063ec4",false]`].Result, &header); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/indexer_real_market/creation.json")
	if err != nil {
		t.Fatal(err)
	}
	var producer struct {
		Creations []struct{ Provenance accounting.Provenance } `json:"indexer.indexer_canonical_creations"`
	}
	if err = json.Unmarshal(raw, &producer); err != nil || len(producer.Creations) != 1 {
		t.Fatal("canonical creation provenance required", err)
	}
	want := producer.Creations[0].Provenance.HeaderSHA256
	if want != "1b7f1e594d0e4acb937e4e4d368eff29440fd385c27a49f0bb932c83bccbb6ae" {
		t.Fatal("canonical creation digest differs", want)
	}
	got, err := receiptHeaderDigest(&header)
	if err != nil || got != want {
		t.Fatal("header does not match indexer provenance", got, want, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*types.Header)
	}{
		{"receipt root", func(h *types.Header) { h.ReceiptHash[0] ^= 1 }},
		{"timestamp", func(h *types.Header) { h.Time++ }},
		{"extra data", func(h *types.Header) { h.Extra = append([]byte{1}, h.Extra...) }},
		{"access list", func(h *types.Header) { value := common.HexToHash("0x1"); h.BlockAccessListHash = &value }},
		{"slot", func(h *types.Header) { value := uint64(1); h.SlotNumber = &value }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := types.CopyHeader(&header)
			tc.change(changed)
			digest, err := receiptHeaderDigest(changed)
			if err != nil || digest == want {
				t.Fatal("changed header retained provenance", digest, err)
			}
		})
	}
	if _, err = receiptHeaderDigest(nil); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal("missing header accepted", err)
	}
}
