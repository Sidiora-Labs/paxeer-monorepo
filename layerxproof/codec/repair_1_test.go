package codec_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
)

type repairNativePath struct {
	Index    uint32
	Count    uint32
	Siblings []string
}

type repairNativeWitness struct {
	Name                   string
	Module                 uint16
	Key, Value, Leaf, Root string
	Proof                  string
	Account, A, B          repairNativePath
}

func repairBytes(t *testing.T, encoded string) []byte {
	t.Helper()
	value, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func repairHash(t *testing.T, encoded string) [32]byte {
	t.Helper()
	value := repairBytes(t, encoded)
	if len(value) != 32 {
		t.Fatalf("native hash has %d bytes", len(value))
	}
	return [32]byte(value)
}

func repairCheckPath(t *testing.T, native repairNativePath, index, count uint32, siblings [][32]byte) {
	t.Helper()
	if index != native.Index || count != native.Count || len(siblings) != len(native.Siblings) {
		t.Fatalf("Go path (%d, %d, %d siblings) differs from native path %+v", index, count, len(siblings), native)
	}
	for i, sibling := range siblings {
		if sibling != repairHash(t, native.Siblings[i]) {
			t.Fatalf("sibling %d differs from native path", i)
		}
	}
}

func TestRepair_1_NativeProducerTransportConsumer(t *testing.T) {
	root, err := testvectors.RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	// Compile the actual host's C preamble so lxphost_reset selects the same
	// interfaces as production, including programs storage retirement.
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
	if err := os.WriteFile(filepath.Join(temp, "lxphost_native.h"), []byte(strings.Join(lines, "\n")), 0600); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(temp, "repair-native")
	library := filepath.Join(root, "build/liblayerx.a")
	compile := exec.CommandContext(t.Context(), "cc", "-std=c17", "-O2",
		"-I"+temp, "-I"+filepath.Join(root, "include"), "-I"+filepath.Join(root, "build/generated"),
		filepath.Join(root, "layerxproof/codec/testdata/repair_1.c"), library,
		filepath.Join(root, "programs/target/release/liblayerx_programs_sandbox.a"), library,
		"-lssl", "-lcrypto", "-pthread", "-ldl", "-lm", "-o", binaryPath)
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile native witness producer: %v\n%s", err, output)
	}
	output, err := exec.CommandContext(t.Context(), binaryPath).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("native witness producer: %v\n%s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	var document struct {
		MaxModule uint16 `json:"max_module"`
		Spot      uint16
		Service   uint16
		Modules   []uint16
		Vectors   []repairNativeWitness
	}
	if err := json.Unmarshal(output, &document); err != nil {
		t.Fatal(err)
	}
	if codec.MaxStateModuleID != document.MaxModule {
		t.Fatalf("Go module bound %d differs from LXP_MODULE_RESERVED_COUNT %d", codec.MaxStateModuleID, document.MaxModule)
	}
	if document.Spot != 10 || document.Service != 5 || len(document.Modules) != 9 {
		t.Fatalf("unexpected native host registrations: %+v", document.Modules)
	}
	width := uint32(9)
	modules := map[uint16]int{}
	for _, module := range document.Modules {
		if _, exists := modules[module]; exists || module == 0 || module > document.MaxModule {
			t.Fatalf("invalid native module registration %d", module)
		}
		modules[module] = 0
		if uint32(module)+1 > width {
			width = uint32(module) + 1
		}
	}
	if width != 11 || len(document.Vectors) != 5+3*len(modules) {
		t.Fatalf("native producer emitted %d witnesses at width %d", len(document.Vectors), width)
	}
	var trustedRoot [32]byte
	for i, native := range document.Vectors {
		stateRoot := repairHash(t, native.Root)
		if i == 0 {
			trustedRoot = stateRoot
		} else if stateRoot != trustedRoot {
			t.Fatal("native witnesses do not share a single committed state")
		}
		if native.Module != 0 {
			if _, exists := modules[native.Module]; !exists {
				t.Fatalf("unregistered module %d produced a witness", native.Module)
			}
			modules[native.Module]++
		}
		t.Run(native.Name, func(t *testing.T) {
			raw := repairBytes(t, native.Proof)
			witness, err := codec.DecodeStateWitness(raw)
			if err != nil {
				t.Fatalf("decode C-built witness: %v", err)
			}
			if witness.ModuleID != native.Module || witness.LeafCountB != width ||
				!bytes.Equal(witness.Key, repairBytes(t, native.Key)) ||
				!bytes.Equal(witness.Value, repairBytes(t, native.Value)) {
				t.Fatal("decoded leaf differs from native state")
			}
			if codec.StateLeafHash(witness.Key, witness.Value) != repairHash(t, native.Leaf) {
				t.Fatal("Go and C leaf hashes differ")
			}
			repairCheckPath(t, native.A, witness.LeafIndexA, witness.LeafCountA, witness.SiblingsA)
			repairCheckPath(t, native.B, uint32(witness.ModuleID), witness.LeafCountB, witness.SiblingsB)
			if witness.AccountPath != nil {
				repairCheckPath(t, native.Account, witness.AccountPath.Index, witness.AccountPath.Count, witness.AccountPath.Siblings)
			} else if native.Account.Count != 0 {
				t.Fatal("native account path lost during decoding")
			}
			computed, err := witness.Root()
			if err != nil || computed != stateRoot {
				t.Fatalf("Go root %x differs from C root %x: %v", computed, stateRoot, err)
			}
			if err := witness.Verify(stateRoot); err != nil {
				t.Fatal(err)
			}
			consumed, err := verify.StateProof(raw, stateRoot)
			if err != nil || !reflect.DeepEqual(consumed, witness) {
				t.Fatalf("StateProof consumer differs from native witness: %v", err)
			}
			if strings.HasPrefix(native.Name, "account_") {
				var index int
				if _, err := fmt.Sscanf(native.Name, "account_%d", &index); err != nil {
					t.Fatal(err)
				}
				name := []byte(fmt.Sprintf("agent:did:key:%c:main", 'a'+index))
				id, err := codec.DeriveAccountID(name)
				if err != nil {
					t.Fatal(err)
				}
				asset := [32]byte{1}
				account, err := verify.AccountProof(raw, stateRoot, id, &asset)
				if err != nil {
					t.Fatalf("AccountProof consumer: %v", err)
				}
				if account.AccountID != id || !bytes.Equal(account.Name, name) ||
					!account.HasAsset || account.AssetID != asset ||
					account.Balance.Hi != 0 || account.Balance.Lo != uint64(100+index) {
					t.Fatalf("AccountProof lost committed account balance: %+v", account)
				}
				wrongID := id
				wrongID[0] ^= 1
				if _, err := verify.AccountProof(raw, stateRoot, wrongID, &asset); !errors.Is(err, codec.ErrAccountIdentity) {
					t.Fatalf("wrong account accepted: %v", err)
				}
				asset[0] ^= 1
				if _, err := verify.AccountProof(raw, stateRoot, id, &asset); !errors.Is(err, verify.ErrAssetIdentity) {
					t.Fatalf("wrong asset accepted: %v", err)
				}
			} else if _, err := verify.AccountProof(raw, stateRoot, [32]byte{}, nil); !errors.Is(err, verify.ErrNotAccountProof) {
				t.Fatalf("non-account witness accepted as account: %v", err)
			}
			if native.Name == "balance" {
				want := binary.BigEndian.AppendUint64(nil, 1)
				want = binary.BigEndian.AppendUint64(want, 2)
				if !bytes.Equal(witness.Value, want) {
					t.Fatalf("module-zero balance changed: %x", witness.Value)
				}
			}
			if native.Module != 0 && (len(witness.Key) != 1 || witness.Key[0] < 1 || witness.Key[0] > 3 ||
				!bytes.Equal(witness.Value, []byte{byte(native.Module), witness.Key[0], 0xa5})) {
				t.Fatalf("module %d lost committed KV value", native.Module)
			}
			repairRefusals(t, raw, witness, stateRoot)
		})
	}
	for module, count := range modules {
		if count != 3 {
			t.Fatalf("module %d has %d native proofs, want 3", module, count)
		}
	}
}

func repairRefusals(t *testing.T, raw []byte, witness *codec.StateWitness, root [32]byte) {
	t.Helper()
	wrongRoot := root
	wrongRoot[0] ^= 1
	if _, err := verify.StateProof(raw, wrongRoot); !errors.Is(err, codec.ErrStateRoot) {
		t.Fatalf("wrong root accepted: %v", err)
	}
	valueOffset := 12 + len(witness.Key)
	aOffset := valueOffset + len(witness.Value)
	if witness.AccountPath != nil {
		aOffset += 9 + 32*len(witness.AccountPath.Siblings)
	}
	bOffset := aOffset + 9 + 32*len(witness.SiblingsA)
	valueError := codec.ErrStateRoot
	siblingError := codec.ErrStateRoot
	if repairHasPromotion(uint32(witness.ModuleID), witness.LeafCountB) {
		valueError = codec.ErrStatePath
		siblingError = codec.ErrStatePath
	}
	if repairHasPromotion(witness.LeafIndexA, witness.LeafCountA) ||
		(witness.AccountPath != nil && repairHasPromotion(witness.AccountPath.Index, witness.AccountPath.Count)) {
		valueError = codec.ErrStatePath
	}
	cases := []struct {
		name   string
		mutate func([]byte)
		want   error
	}{
		{"unsupported_module", func(b []byte) { binary.BigEndian.PutUint16(b[2:4], codec.MaxStateModuleID+1) }, codec.ErrStateModule},
		{"short_composite", func(b []byte) { binary.BigEndian.PutUint32(b[bOffset:], 8) }, codec.ErrStateModule},
		{"wide_composite", func(b []byte) { binary.BigEndian.PutUint32(b[bOffset:], uint32(codec.MaxStateModuleID)+2) }, codec.ErrStateModule},
		{"index_outside_subtree", func(b []byte) { binary.BigEndian.PutUint32(b[aOffset:], witness.LeafCountA) }, codec.ErrStatePath},
		{"empty_subtree", func(b []byte) { binary.BigEndian.PutUint32(b[aOffset+4:], 0) }, codec.ErrStatePath},
		{"oversize_depth", func(b []byte) { b[aOffset+8] = codec.MaxProofDepth + 1 }, codec.ErrStatePath},
		{"changed_value", func(b []byte) { b[valueOffset] ^= 1 }, valueError},
		{"changed_composite_sibling", func(b []byte) { b[bOffset+5] ^= 1 }, siblingError},
	}
	if witness.ModuleID == 10 {
		cases = append(cases, struct {
			name   string
			mutate func([]byte)
			want   error
		}{"module_outside_composite", func(b []byte) { binary.BigEndian.PutUint32(b[bOffset:], 9) }, codec.ErrStateModule})
	}
	for _, refusal := range cases {
		t.Run(refusal.name, func(t *testing.T) {
			mutated := bytes.Clone(raw)
			refusal.mutate(mutated)
			if _, err := verify.StateProof(mutated, root); !errors.Is(err, refusal.want) {
				t.Fatalf("got %v, want %v", err, refusal.want)
			}
		})
	}
	for _, malformed := range [][]byte{raw[:len(raw)-1], append(bytes.Clone(raw), 0)} {
		if _, err := codec.DecodeStateWitness(malformed); !errors.Is(err, codec.ErrStateEncoding) {
			t.Fatalf("malformed wire accepted: %v", err)
		}
	}
	short := *witness
	short.SiblingsB = short.SiblingsB[:len(short.SiblingsB)-1]
	if _, err := short.Root(); !errors.Is(err, codec.ErrStatePath) {
		t.Fatalf("short path accepted: %v", err)
	}
	extra := *witness
	extra.SiblingsB = append(append([][32]byte(nil), extra.SiblingsB...), [32]byte{})
	if _, err := extra.Root(); !errors.Is(err, codec.ErrStatePath) {
		t.Fatalf("extra path accepted: %v", err)
	}
	if witness.LeafIndexA == 2 && witness.LeafCountA == 3 {
		forged := *witness
		forged.SiblingsA = append([][32]byte(nil), witness.SiblingsA...)
		forged.SiblingsA[0][0] ^= 1
		if _, err := forged.Root(); !errors.Is(err, codec.ErrStatePath) {
			t.Fatalf("forged duplicate-last sibling accepted: %v", err)
		}
	}
}

func repairHasPromotion(index, count uint32) bool {
	for count > 1 {
		if index^1 >= count {
			return true
		}
		index /= 2
		count = count/2 + count%2
	}
	return false
}
