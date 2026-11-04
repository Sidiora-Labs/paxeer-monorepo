package codec_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec/internal/nativefixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
)

type repairM1Document struct {
	MaxModule uint16 `json:"max_module"`
	Spot      uint16
	Service   uint16
	Modules   []uint16
	Vectors   []repairNativeWitness
}

func repairM1Fixture(t *testing.T, compatibility bool) repairM1Document {
	t.Helper()
	raw, err := nativefixture.Document(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	var document repairM1Document
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.MaxModule != 11 {
		t.Fatalf("native module bound %d differs from reserved ID 11", document.MaxModule)
	}
	if codec.MaxStateModuleID != document.MaxModule {
		t.Fatalf("native module bound %d differs from codec %d", document.MaxModule, codec.MaxStateModuleID)
	}
	return document
}

func repairM1CheckWitness(t *testing.T, native repairNativeWitness, width uint32) {
	t.Helper()
	raw := repairBytes(t, native.Proof)
	trusted := repairHash(t, native.Root)
	witness, err := codec.DecodeStateWitness(raw)
	if err != nil {
		t.Fatalf("decode C-produced witness: %v", err)
	}
	if witness.ModuleID != native.Module || witness.LeafCountB != width ||
		!bytes.Equal(witness.Key, repairBytes(t, native.Key)) ||
		!bytes.Equal(witness.Value, repairBytes(t, native.Value)) {
		t.Fatalf("C leaf or composite width changed in transport: %+v", witness)
	}
	if codec.StateLeafHash(witness.Key, witness.Value) != repairHash(t, native.Leaf) {
		t.Fatal("Go and C leaf hashes differ")
	}
	repairCheckPath(t, native.A, witness.LeafIndexA, witness.LeafCountA, witness.SiblingsA)
	repairCheckPath(t, native.B, uint32(witness.ModuleID), witness.LeafCountB, witness.SiblingsB)
	if witness.AccountPath != nil {
		repairCheckPath(t, native.Account, witness.AccountPath.Index, witness.AccountPath.Count, witness.AccountPath.Siblings)
	} else if native.Account.Count != 0 {
		t.Fatal("native account path lost")
	}
	computed, err := witness.Root()
	if err != nil || computed != trusted {
		t.Fatalf("Go root differs from C: %v", err)
	}
	if err := witness.Verify(trusted); err != nil {
		t.Fatal(err)
	}
	consumed, err := verify.StateProof(raw, trusted)
	if err != nil || !reflect.DeepEqual(consumed, witness) {
		t.Fatalf("StateProof consumer differs from C witness: %v", err)
	}
	if native.Module != 0 && (len(witness.Key) != 1 || witness.Key[0] < 1 || witness.Key[0] > 3 ||
		!bytes.Equal(witness.Value, []byte{byte(native.Module), witness.Key[0], 0xa5})) {
		t.Fatal("native module KV state changed")
	}
	if witness.AccountPath != nil {
		id := [32]byte(witness.Key[1:])
		asset := [32]byte{1}
		account, err := verify.AccountProof(raw, trusted, id, &asset)
		if err != nil {
			t.Fatal(err)
		}
		var index int
		if _, err := fmt.Sscanf(native.Name, "account_%d", &index); err != nil {
			t.Fatal(err)
		}
		name := []byte(fmt.Sprintf("agent:did:key:%c:main", 'a'+index))
		expectedID, err := codec.DeriveAccountID(name)
		if err != nil {
			t.Fatal(err)
		}
		if index < 0 || index > 2 || account.AccountID != expectedID || !bytes.Equal(account.Name, name) ||
			!account.HasAsset || account.AssetID != asset || account.Balance.Hi != 0 || account.Balance.Lo != uint64(100+index) {
			t.Fatalf("native account balance lost: %+v", account)
		}
		wrongID := id
		wrongID[0] ^= 1
		if _, err := verify.AccountProof(raw, trusted, wrongID, &asset); !errors.Is(err, codec.ErrAccountIdentity) {
			t.Fatalf("wrong account: %v", err)
		}
		asset[0] ^= 1
		if _, err := verify.AccountProof(raw, trusted, id, &asset); !errors.Is(err, verify.ErrAssetIdentity) {
			t.Fatalf("wrong asset: %v", err)
		}
	} else if _, err := verify.AccountProof(raw, trusted, [32]byte{}, nil); !errors.Is(err, verify.ErrNotAccountProof) {
		t.Fatalf("non-account witness: %v", err)
	}
	if native.Name == "balance" {
		want := binary.BigEndian.AppendUint64(nil, 1)
		want = binary.BigEndian.AppendUint64(want, 2)
		if !bytes.Equal(witness.Value, want) {
			t.Fatal("native balance changed")
		}
	}
	if native.Name == "sequence" && !bytes.Equal(witness.Value, binary.BigEndian.AppendUint64(nil, 2)) {
		t.Fatalf("native sequence changed: %x", witness.Value)
	}
	repairRefusals(t, raw, witness, trusted)
	// Exercise every out-of-contract width at the boundaries, including a
	// reserved module that falls outside an otherwise supported tree width.
	bOffset := len(raw) - 5 - 32*len(witness.SiblingsB)
	for _, width := range []uint32{0, 1, 8, 13, ^uint32(0)} {
		malformed := bytes.Clone(raw)
		binary.BigEndian.PutUint32(malformed[bOffset:], width)
		if _, err := verify.StateProof(malformed, trusted); !errors.Is(err, codec.ErrStateModule) {
			t.Fatalf("malformed width %d: got %v, want ErrStateModule", width, err)
		}
	}
	if witness.ModuleID == 11 {
		malformed := bytes.Clone(raw)
		binary.BigEndian.PutUint32(malformed[bOffset:], 11)
		if _, err := verify.StateProof(malformed, trusted); !errors.Is(err, codec.ErrStateModule) {
			t.Fatalf("module 11 outside width 11: %v", err)
		}
	}
}

func repairM1CurrentHost(t *testing.T) {
	t.Helper()
	root, err := testvectors.RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(root, "modules/lxphost/host.go"))
	if err != nil {
		t.Fatal(err)
	}
	_, preamble, ok := strings.Cut(string(source), "/*")
	if !ok {
		t.Fatal("native host preamble missing")
	}
	preamble, _, ok = strings.Cut(preamble, "*/")
	if !ok {
		t.Fatal("native host preamble unterminated")
	}
	var lines []string
	for _, line := range strings.Split(preamble, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#cgo ") {
			lines = append(lines, line)
		}
	}
	preamble, _, ok = strings.Cut(strings.Join(lines, "\n"), "static void *lxphost_accounts")
	if !ok {
		t.Fatal("native host fixture boundary missing")
	}
	snapshot, err := os.ReadFile(filepath.Join(root, "layerxproof/codec/internal/nativefixture/lxphost_native.h"))
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot) != preamble {
		t.Fatal("fixture must compile the current production node creation and reset code")
	}
	document := repairM1Fixture(t, false)
	want := map[uint16]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 9: 0, 10: 0}
	if document.Spot != 10 || document.Service != 5 || len(document.Modules) != len(want) {
		t.Fatalf("unexpected current host registration: %+v", document)
	}
	registered := map[uint16]bool{}
	for _, module := range document.Modules {
		if _, ok := want[module]; !ok || registered[module] {
			t.Fatalf("unexpected module %d", module)
		}
		registered[module] = true
	}
	if len(document.Vectors) != 32 {
		t.Fatalf("got %d host witnesses, want 32", len(document.Vectors))
	}
	trusted := document.Vectors[0].Root
	accounts, balance, sequence := 0, 0, 0
	names := map[string]bool{}
	for _, native := range document.Vectors {
		if native.Root != trusted || names[native.Name] {
			t.Fatal("host state root or witness identity differs")
		}
		names[native.Name] = true
		if native.Module != 0 {
			if !registered[native.Module] {
				t.Fatalf("unregistered witness module %d", native.Module)
			}
			want[native.Module]++
		} else if strings.HasPrefix(native.Name, "account_") {
			accounts++
		} else if native.Name == "balance" {
			balance++
		} else if native.Name == "sequence" {
			sequence++
		} else {
			t.Fatalf("unknown universal witness %s", native.Name)
		}
		t.Run(native.Name, func(t *testing.T) { repairM1CheckWitness(t, native, 11) })
	}
	for module, count := range want {
		if count != 3 {
			t.Fatalf("registered module %d has %d witnesses, want 3", module, count)
		}
	}
	if accounts != 3 || balance != 1 || sequence != 1 {
		t.Fatalf("universal witnesses: accounts=%d balance=%d sequence=%d", accounts, balance, sequence)
	}
}

func repairM1Compatibility(t *testing.T) {
	t.Helper()
	document := repairM1Fixture(t, true)
	if len(document.Vectors) != 2 {
		t.Fatalf("got %d compatibility witnesses, want 2", len(document.Vectors))
	}
	seen := map[uint16]bool{}
	for _, native := range document.Vectors {
		if native.Module != 10 && native.Module != 11 || seen[native.Module] {
			t.Fatalf("unexpected compatibility module %d", native.Module)
		}
		seen[native.Module] = true
		if native.Root != document.Vectors[0].Root {
			t.Fatal("compatibility witnesses have different roots")
		}
		t.Run(native.Name, func(t *testing.T) { repairM1CheckWitness(t, native, 12) })
	}
	if !seen[10] || !seen[11] {
		t.Fatal("missing module 10 or 11 inclusion")
	}
}

func TestRepairM_1_CurrentHostNativeWitnesses(t *testing.T) { repairM1CurrentHost(t) }

func TestRepairM_1_TwelveLeafModuleInclusion(t *testing.T) { repairM1Compatibility(t) }

func TestRepairM_1_PreservedRefusals(t *testing.T) {
	TestStateWitnessRefusalClasses(t)
	testHistoricalNativeStateProofsFromCore(t)
}
