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

func admissionEvidenceVector(root, directory string) []byte {
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	decode := func(s string) []byte { b, err := hex.DecodeString(s); must(err); return b }
	load := func(path string, out any) {
		b, err := os.ReadFile(filepath.Join(root, path))
		must(err)
		must(json.Unmarshal(b, out))
	}
	var activity struct {
		Signed string `json:"signed_activity_hex"`
		ID     string `json:"activity_id_hex"`
	}
	fixture := "sdk/platform/conformance/fixtures/native-program-call-v4.json"
	load(fixture, &activity)
	signed, activityID := decode(activity.Signed), decode(activity.ID)
	digest := sha256.Sum256(append([]byte("LXP/v1/activity-id\x00"), signed...))
	if !bytes.Equal(digest[:], activityID) {
		panic("native activity identifier mismatch")
	}
	var pins struct {
		Entries []struct {
			Value   string `json:"value"`
			Receipt string `json:"receipt"`
			Height  uint64 `json:"height"`
		} `json:"entries"`
	}
	load("programs/tests/vectors/ics23-receipt-keys-app-hash.json", &pins)
	if len(pins.Entries) != 1 {
		panic("one real receipt-key vector required")
	}
	keyTable, receipt := decode(pins.Entries[0].Value), decode(pins.Entries[0].Receipt)
	blob, err := os.ReadFile(filepath.Join(root, "programs/fixtures/pay5/token-lxt20.wasm"))
	must(err)
	codeHash := sha256.Sum256(blob)
	program := sha256.Sum256([]byte("admission app-hash program"))
	asset := sha256.Sum256([]byte("admission fee asset"))
	name := []byte("agent:did:lxp:native-lifecycle-fixture:main")
	accountID := sha256.Sum256(append(binary.BigEndian.AppendUint32([]byte("LX:ACCOUNT:v1"), uint32(len(name))), name...))
	vector := func(b, value []byte) []byte {
		b = binary.BigEndian.AppendUint32(b, uint32(len(value)))
		return append(b, value...)
	}
	leaf := func(key, value []byte) [32]byte {
		b := binary.BigEndian.AppendUint32([]byte("LXP/v1/state-leaf\x00"), uint32(len(key)))
		b = binary.BigEndian.AppendUint32(b, uint32(len(value)))
		b = append(b, key...)
		b = append(b, value...)
		return sha256.Sum256(b)
	}
	node := func(a, b [32]byte) [32]byte {
		v := append([]byte("LXP/v1/state-node\x00"), a[:]...)
		return sha256.Sum256(append(v, b[:]...))
	}
	tree := func(leaves [][32]byte, index int) ([32]byte, [][32]byte) {
		if len(leaves) == 0 {
			return sha256.Sum256([]byte("LXP/v1/state-leaf\x00")), nil
		}
		level := append([][32]byte(nil), leaves...)
		var path [][32]byte
		for len(level) > 1 {
			sibling := index ^ 1
			if sibling >= len(level) {
				sibling = index
			}
			path = append(path, level[sibling])
			var next [][32]byte
			for i := 0; i < len(level); i += 2 {
				j := i + 1
				if j == len(level) {
					j = i
				}
				next = append(next, node(level[i], level[j]))
			}
			level, index = next, index/2
		}
		return level[0], path
	}
	pathBytes := func(b []byte, path [][32]byte) []byte {
		b = append(b, byte(len(path)))
		for _, h := range path {
			b = append(b, h[:]...)
		}
		return b
	}
	account := binary.BigEndian.AppendUint16(nil, uint16(len(name)))
	account = append(account, name...)
	account = append(account, 1)
	account = append(account, make([]byte, 8)...)
	account = binary.BigEndian.AppendUint64(account, 1000000)
	account = append(account, asset[:]...)
	account = append(account, 1)
	account = binary.BigEndian.AppendUint64(account, 0)
	account = binary.BigEndian.AppendUint64(account, 1)
	account = append(account, 0, 0)
	account = append(account, make([]byte, 32)...)
	account = append(account, 0)
	accountKey := append([]byte{4}, accountID[:]...)
	accountRoot := leaf(accountKey, account)
	var modules [12]map[string][]byte
	for i := range modules {
		modules[i] = map[string][]byte{}
	}
	modules[0]["account-tree"] = accountRoot[:]
	modules[0]["sequence"] = binary.BigEndian.AppendUint64(nil, 7)
	record := append([]byte{0}, make([]byte, 32)...)
	record = append(record, codeHash[:]...)
	record = binary.BigEndian.AppendUint16(record, 1)
	record = binary.BigEndian.AppendUint32(record, 1)
	modules[9][string(append([]byte("program\x00"), program[:]...))] = record
	actor := sha256.Sum256([]byte("LXP/v1/did-id\x00did:lxp:native-lifecycle-fixture"))
	identity := append([]byte("LXGI1"), actor[:]...)
	identity = append(identity, make([]byte, 223-len(identity))...)
	modules[7][string(actor[:])] = identity
	var roots [][32]byte
	var keys [12][]string
	var hashes [12][][32]byte
	for i, m := range modules {
		for k := range m {
			keys[i] = append(keys[i], k)
		}
		sort.Strings(keys[i])
		for _, k := range keys[i] {
			hashes[i] = append(hashes[i], leaf([]byte(k), m[k]))
		}
		r, _ := tree(hashes[i], 0)
		roots = append(roots, r)
	}
	var composite [][32]byte
	for i, r := range roots {
		composite = append(composite, leaf(binary.BigEndian.AppendUint16(nil, uint16(i)), r[:]))
	}
	stateRoot, _ := tree(composite, 0)
	witness := func(module, index int, key, value []byte, accountPath bool) []byte {
		b := binary.BigEndian.AppendUint16(nil, 2)
		b = binary.BigEndian.AppendUint16(b, uint16(module))
		b = vector(b, key)
		b = vector(b, value)
		if accountPath {
			b = binary.BigEndian.AppendUint32(b, 0)
			b = binary.BigEndian.AppendUint32(b, 1)
			b = append(b, 0)
		}
		b = binary.BigEndian.AppendUint32(b, uint32(index))
		b = binary.BigEndian.AppendUint32(b, uint32(len(keys[module])))
		_, inner := tree(hashes[module], index)
		b = pathBytes(b, inner)
		b = binary.BigEndian.AppendUint32(b, uint32(len(roots)))
		_, outer := tree(composite, module)
		return pathBytes(b, outer)
	}
	moduleBytes := func(module int) []byte {
		b := binary.BigEndian.AppendUint16(nil, uint16(module))
		b = append(b, roots[module][:]...)
		b = binary.BigEndian.AppendUint32(b, uint32(module))
		b = binary.BigEndian.AppendUint32(b, uint32(len(roots)))
		_, outer := tree(composite, module)
		b = pathBytes(b, outer)
		b = binary.BigEndian.AppendUint32(b, uint32(len(keys[module])))
		for i, k := range keys[module] {
			b = vector(b, witness(module, i, []byte(k), modules[module][k], false))
		}
		return b
	}
	native := binary.BigEndian.AppendUint16(nil, 1)
	native = binary.BigEndian.AppendUint32(native, 7)
	native = append(native, activityID...)
	native = binary.BigEndian.AppendUint64(native, 7)
	native = append(native, stateRoot[:]...)
	native = binary.BigEndian.AppendUint16(native, uint16(len(roots)))
	for _, r := range roots {
		native = append(native, r[:]...)
	}
	native = append(native, moduleBytes(0)...)
	native = append(native, moduleBytes(9)...)
	native = binary.BigEndian.AppendUint32(native, 1)
	native = vector(native, witness(0, 0, accountKey, account, true))
	v2 := binary.BigEndian.AppendUint16(nil, 2)
	v2 = vector(v2, native)
	v2 = binary.BigEndian.AppendUint16(v2, 2)
	v2 = append(v2, moduleBytes(6)...)
	v2 = append(v2, moduleBytes(11)...)
	v3 := binary.BigEndian.AppendUint16(nil, 3)
	v3 = vector(v3, v2)
	v3 = vector(v3, signed)
	v3 = binary.BigEndian.AppendUint16(v3, 3)
	for _, m := range []int{1, 3, 7} {
		v3 = append(v3, moduleBytes(m)...)
	}
	receiptDigest := sha256.Sum256([]byte("admission pinned receipt digest"))
	header := append([]byte("LXP/arbiter-admission/v1\x00"), receiptDigest[:]...)
	header = vector(header, v3)
	headerKey := append([]byte{1, 0, 9}, []byte("admission/v1/")...)
	headerKey = append(headerKey, activityID...)
	records := map[string][]byte{string(headerKey): header, string(append([]byte{6}, accountID[:]...)): account}
	for _, m := range []int{7, 9} {
		for k, v := range modules[m] {
			key := binary.BigEndian.AppendUint16([]byte{1}, uint16(m))
			key = append(key, []byte(k)...)
			records[string(key)] = v
		}
	}
	configuration := config.DefaultStateCommitConfig()
	configuration.MemIAVLConfig.AsyncCommitBuffer = 0
	store := rootmultiv2.NewStore(directory, configuration, config.StateStoreConfig{}, nil)
	defer func() { must(store.Close()) }()
	hostKey, seqKey := storetypes.NewKVStoreKey("lxphost"), storetypes.NewKVStoreKey("sequencer")
	store.MountStoreWithDB(hostKey, storetypes.StoreTypeIAVL, nil)
	store.MountStoreWithDB(seqKey, storetypes.StoreTypeIAVL, nil)
	must(store.LoadLatestVersion())
	must(store.SetInitialVersion(int64(pins.Entries[0].Height)))
	query := func(storeName string, key, value []byte, height int64, appHash []byte) map[string]any {
		response := store.Query(context.Background(), abci.RequestQuery{Path: "/" + storeName + "/key", Data: key, Height: height, Prove: true})
		if response.Code != 0 || !bytes.Equal(response.Value, value) || response.ProofOps == nil || len(response.ProofOps.Ops) != 2 {
			panic(fmt.Sprintf("real keeper entry proof required: %+v", response))
		}
		var leafProof, outer ics23.CommitmentProof
		must(leafProof.Unmarshal(response.ProofOps.Ops[0].Data))
		must(outer.Unmarshal(response.ProofOps.Ops[1].Data))
		if leafProof.GetExist() == nil || outer.GetExist() == nil {
			panic("existence proofs required")
		}
		storeRoot := outer.GetExist().Value
		if !ics23.VerifyMembership(ics23.IavlSpec, storeRoot, &leafProof, key, value) || !ics23.VerifyMembership(ics23.TendermintSpec, appHash, &outer, []byte(storeName), storeRoot) {
			panic("keeper app-hash proof failed")
		}
		p, err := leafProof.GetExist().Marshal()
		must(err)
		return map[string]any{"height": height, "app_hash": hex.EncodeToString(appHash), "store_root": hex.EncodeToString(storeRoot), "multistore_proof": hex.EncodeToString(response.ProofOps.Ops[1].Data), "key": hex.EncodeToString(key), "value": hex.EncodeToString(value), "proof": hex.EncodeToString(p)}
	}
	var entries []any
	for i := 0; i < 2; i++ {
		cache := store.CacheMultiStore()
		host := cache.GetKVStore(hostKey)
		var ordered []string
		for k := range records {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		for _, k := range ordered {
			host.Set([]byte(k), records[k])
		}
		host.Set([]byte("admission-vector-revision"), []byte{byte(i)})
		cache.GetKVStore(seqKey).Set([]byte{3}, keyTable)
		cache.Write()
		commit := store.Commit(true)
		var proofs []any
		for _, k := range ordered {
			proofs = append(proofs, query("lxphost", []byte(k), records[k], commit.Version, commit.Hash))
		}
		chainCommit := query("sequencer", []byte{3}, keyTable, commit.Version, commit.Hash)
		chainCommit["receipt"] = hex.EncodeToString(receipt)
		entries = append(entries, map[string]any{"activity_id": activity.ID, "network_id": 7, "sequence": 7, "state_root": hex.EncodeToString(stateRoot[:]), "receipt_digest": hex.EncodeToString(receiptDigest[:]), "native": hex.EncodeToString(native), "signed_activity": activity.Signed, "records": proofs, "chain_commit": chainCommit})
	}
	if entries[0].(map[string]any)["records"].([]any)[0].(map[string]any)["app_hash"] == entries[1].(map[string]any)["records"].([]any)[0].(map[string]any)["app_hash"] {
		panic("distinct committed app hashes required")
	}
	missingKey := append([]byte{1, 0, 7}, actor[:]...)
	cache := store.CacheMultiStore()
	cache.GetKVStore(hostKey).Delete(missingKey)
	cache.Write()
	missingCommit := store.Commit(true)
	absentResponse := store.Query(context.Background(), abci.RequestQuery{Path: "/lxphost/key", Data: missingKey, Height: missingCommit.Version, Prove: true})
	if absentResponse.Code != 0 || len(absentResponse.Value) != 0 || absentResponse.ProofOps == nil || len(absentResponse.ProofOps.Ops) != 2 {
		panic("real missing governance entry required")
	}
	var absentLeaf, absentOuter ics23.CommitmentProof
	must(absentLeaf.Unmarshal(absentResponse.ProofOps.Ops[0].Data))
	must(absentOuter.Unmarshal(absentResponse.ProofOps.Ops[1].Data))
	if absentLeaf.GetNonexist() == nil || absentOuter.GetExist() == nil || !ics23.VerifyNonMembership(ics23.IavlSpec, absentOuter.GetExist().Value, &absentLeaf, missingKey) || !ics23.VerifyMembership(ics23.TendermintSpec, missingCommit.Hash, &absentOuter, []byte("lxphost"), absentOuter.GetExist().Value) {
		panic("real nonmembership under the app hash required")
	}
	negative := func(commitHeight int64, appHash []byte, excluded string) map[string]any {
		var ordered []string
		for k := range records {
			if k != excluded {
				ordered = append(ordered, k)
			}
		}
		sort.Strings(ordered)
		var proofs []any
		for _, k := range ordered {
			proofs = append(proofs, query("lxphost", []byte(k), records[k], commitHeight, appHash))
		}
		return map[string]any{"activity_id": activity.ID, "records": proofs}
	}
	absent := negative(missingCommit.Version, missingCommit.Hash, string(missingKey))
	changed := bytes.Clone(identity)
	changed[len(changed)-1] ^= 1
	records[string(missingKey)] = changed
	cache = store.CacheMultiStore()
	cache.GetKVStore(hostKey).Set(missingKey, changed)
	cache.Write()
	mismatchCommit := store.Commit(true)
	mismatch := negative(mismatchCommit.Version, mismatchCommit.Hash, "")
	out, err := json.MarshalIndent(map[string]any{"entries": entries, "receipt_entries": []any{entries[0].(map[string]any)["chain_commit"]}, "absent": absent, "mismatch": mismatch, "source": map[string]any{"generator": "modules/sequencer/keeper/ics23_admission_evidence_vector_test.go::TestAdmissionEvidenceAppHashVector", "activity_fixture": fixture}}, "", "  ")
	must(err)
	return append(out, '\n')
}

func TestAdmissionEvidenceAppHashVector(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	encoded := admissionEvidenceVector(root, t.TempDir())
	path := filepath.Join(root, "programs/tests/vectors/ics23-admission-evidence-app-hash.json")
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
