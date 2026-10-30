package keeper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	ics23 "github.com/confio/ics23/go"
)

func marketNamespacesVector(root, directory string) []byte {
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	decode := func(s string) []byte {
		if len(s) > 1 && s[:2] == "0x" {
			s = s[2:]
		}
		b, err := hex.DecodeString(s)
		must(err)
		return b
	}
	load := func(path string, out any) {
		b, err := os.ReadFile(filepath.Join(root, path))
		must(err)
		must(json.Unmarshal(b, out))
	}
	var primary struct {
		Vectors []struct {
			ID      string `json:"id"`
			Receipt string `json:"receipt"`
			Keys    []struct {
				ID     string `json:"keyId"`
				Public string `json:"publicKey"`
				First  uint64 `json:"firstHeight"`
				Last   uint64 `json:"lastHeight"`
			} `json:"receiptKeys"`
		} `json:"vectors"`
	}
	load("sdk/platform/conformance/fixtures/receipt-sequencer-v1.json", &primary)
	var rpc struct {
		Cases []struct {
			ID      string `json:"id"`
			Receipt string `json:"signedReceipt"`
			Pin     struct {
				ID     string `json:"keyId"`
				Public string `json:"publicKey"`
				First  uint64 `json:"firstHeight"`
				Last   uint64 `json:"lastHeight"`
			} `json:"pinnedKey"`
		} `json:"cases"`
	}
	load("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", &rpc)
	var receipts [][]byte
	var pins [][]byte
	for _, v := range primary.Vectors {
		if v.ID == "receipted-primary-key" {
			receipts = append(receipts, decode(v.Receipt))
			for _, k := range v.Keys {
				p := append(decode(k.ID), decode(k.Public)...)
				p = binary.BigEndian.AppendUint64(p, k.First)
				p = binary.BigEndian.AppendUint64(p, k.Last)
				pins = append(pins, p)
			}
		}
	}
	for _, v := range rpc.Cases {
		if v.ID == "V1-pax-transfer" {
			receipts = append(receipts, decode(v.Receipt))
			p := append(decode(v.Pin.ID), decode(v.Pin.Public)...)
			p = binary.BigEndian.AppendUint64(p, v.Pin.First)
			p = binary.BigEndian.AppendUint64(p, v.Pin.Last)
			pins = append(pins, p)
		}
	}
	if len(receipts) != 2 || bytes.Equal(receipts[0], receipts[1]) {
		panic("two distinct recorded signed receipts required")
	}
	sort.Slice(pins, func(i, j int) bool { return bytes.Compare(pins[i], pins[j]) < 0 })
	var keyTable []byte
	for _, p := range pins {
		keyTable = append(keyTable, p...)
	}
	configuration := config.DefaultStateCommitConfig()
	configuration.MemIAVLConfig.AsyncCommitBuffer = 0
	store := rootmultiv2.NewStore(directory, configuration, config.StateStoreConfig{}, nil)
	defer func() { must(store.Close()) }()
	hostKey := storetypes.NewKVStoreKey("lxphost")
	sequencerKey := storetypes.NewKVStoreKey("sequencer")
	store.MountStoreWithDB(hostKey, storetypes.StoreTypeIAVL, nil)
	store.MountStoreWithDB(sequencerKey, storetypes.StoreTypeIAVL, nil)
	must(store.LoadLatestVersion())
	must(store.SetInitialVersion(1200))
	program := sha256.Sum256([]byte("market namespace app hash vector"))
	id := func(n byte) []byte { return bytes.Repeat([]byte{n}, 32) }
	// Canonical Market offer and funded lease records, decoded independently by Rust.
	offer := []byte{1, 1, 3}
	for _, n := range []byte{1, 2, 3, 4, 5} {
		offer = append(offer, id(n)...)
	}
	offer = binary.BigEndian.AppendUint16(offer, 5)
	offer = append(offer, []byte("stake")...)
	offer = append(offer, make([]byte, 15)...)
	offer = append(offer, 100)
	offer = append(offer, make([]byte, 15)...)
	offer = append(offer, 1)
	for _, n := range []uint64{100, 90, 1, 10, 2000} {
		offer = binary.BigEndian.AppendUint64(offer, n)
	}
	lease := []byte{1, 1, 3}
	for _, n := range []byte{6, 1, 2, 7, 3, 8, 4, 9} {
		lease = append(lease, id(n)...)
	}
	lease = binary.BigEndian.AppendUint16(lease, 6)
	lease = append(lease, []byte("escrow")...)
	for _, n := range []uint64{10} {
		lease = binary.BigEndian.AppendUint64(lease, n)
	}
	lease = append(lease, make([]byte, 15)...)
	lease = append(lease, 10)
	lease = binary.BigEndian.AppendUint64(lease, 1199)
	lease = binary.BigEndian.AppendUint64(lease, 1900)
	records := map[string][]byte{string(append([]byte("lx.market.offer/"), id(1)...)): offer, string(append([]byte("lx.market.lease/"), id(6)...)): lease}
	headKey := append([]byte{1, 0, 9}, []byte("progstor")...)
	headKey = append(headKey, program[:]...)
	headKey = append(headKey, 1)
	var entries []any
	for i, receipt := range receipts {
		if i == 1 { // A genuine later namespace snapshot: the offer loses another leased unit.
			offer = bytes.Clone(offer)
			binary.BigEndian.PutUint64(offer[len(offer)-32:len(offer)-24], 89)
			records[string(append([]byte("lx.market.offer/"), id(1)...))] = offer
		}
		keys := make([]string, 0, len(records))
		for k := range records {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		manifest := binary.BigEndian.AppendUint16(nil, 1)
		manifest = binary.BigEndian.AppendUint32(manifest, uint32(len(keys)))
		var cells []any
		for _, k := range keys {
			v := records[k]
			hash := sha256.Sum256(v)
			manifest = binary.BigEndian.AppendUint16(manifest, uint16(len(k)))
			manifest = append(manifest, []byte(k)...)
			manifest = append(manifest, hash[:]...)
			manifest = binary.BigEndian.AppendUint32(manifest, uint32(len(v)))
			cells = append(cells, map[string]any{"key": hex.EncodeToString([]byte(k)), "value": hex.EncodeToString(v), "hash": hex.EncodeToString(hash[:])})
		}
		mh := sha256.Sum256(manifest)
		head := binary.BigEndian.AppendUint16(nil, 1)
		head = binary.BigEndian.AppendUint32(head, uint32(len(keys)))
		head = append(head, mh[:]...)
		receiptHash := sha256.Sum256(receipt)
		receiptKey := append([]byte("ics23-market/receipt/"), receiptHash[:]...)
		cache := store.CacheMultiStore()
		host := cache.GetKVStore(hostKey)
		host.Set(headKey, head)
		host.Set(receiptKey, receipt)
		cache.GetKVStore(sequencerKey).Set([]byte{3}, keyTable)
		cache.Write()
		commit := store.Commit(true)
		query := func(name string, key, value []byte) map[string]any {
			response := store.Query(context.Background(), abci.RequestQuery{Path: "/" + name + "/key", Data: key, Height: commit.Version, Prove: true})
			if response.Code != 0 || response.ProofOps == nil || len(response.ProofOps.Ops) != 2 {
				panic(fmt.Sprintf("keeper cannot prove market entries: %+v", response))
			}
			var leaf, outer ics23.CommitmentProof
			must(leaf.Unmarshal(response.ProofOps.Ops[0].Data))
			must(outer.Unmarshal(response.ProofOps.Ops[1].Data))
			if leaf.GetExist() == nil || outer.GetExist() == nil {
				panic("existence proofs required")
			}
			storeRoot := outer.GetExist().Value
			if !ics23.VerifyMembership(ics23.IavlSpec, storeRoot, &leaf, key, value) || !ics23.VerifyMembership(ics23.TendermintSpec, commit.Hash, &outer, []byte(name), storeRoot) {
				panic("keeper proof verification failed")
			}
			proof, err := leaf.GetExist().Marshal()
			must(err)
			return map[string]any{"height": commit.Version, "key": hex.EncodeToString(key), "value": hex.EncodeToString(value), "store_root": hex.EncodeToString(storeRoot), "app_hash": hex.EncodeToString(commit.Hash), "proof": hex.EncodeToString(proof), "multistore_proof": hex.EncodeToString(response.ProofOps.Ops[1].Data)}
		}
		namespace := query("lxphost", headKey, head)
		namespace["manifest"] = hex.EncodeToString(manifest)
		namespace["cells"] = cells
		entries = append(entries, map[string]any{"namespace": namespace, "chain_commit": query("sequencer", []byte{3}, keyTable), "receipt": query("lxphost", receiptKey, receipt)})
	}
	vector := map[string]any{"market_program": hex.EncodeToString(program[:]), "entries": entries, "source": map[string]any{"generator": "modules/sequencer/keeper/ics23_market_namespaces_vector_test.go::TestMarketNamespacesAppHashVector", "receipt_fixtures": []string{"sdk/platform/conformance/fixtures/receipt-sequencer-v1.json::receipted-primary-key", "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json::V1-pax-transfer"}}}
	encoded, err := json.MarshalIndent(vector, "", "  ")
	must(err)
	return append(encoded, '\n')
}

func TestMarketNamespacesAppHashVector(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	encoded := marketNamespacesVector(root, t.TempDir())
	path := filepath.Join(root, "programs/tests/vectors/ics23-market-namespaces-app-hash.json")
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(committed, encoded) {
		t.Fatal("regenerate with LXP_WRITE_VECTORS=1")
	}
}
