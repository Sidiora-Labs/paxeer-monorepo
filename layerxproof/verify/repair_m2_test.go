package verify_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
)

type repairM2Path struct {
	Index    uint32   `json:"index"`
	Count    uint32   `json:"count"`
	Siblings []string `json:"siblings"`
}

type repairM2Account struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Kind          uint8        `json:"kind"`
	Asset         string       `json:"asset"`
	Balance       codec.U128   `json:"balance"`
	Proof         string       `json:"proof"`
	Key           string       `json:"key"`
	Value         string       `json:"value"`
	AccountPath   repairM2Path `json:"account_path"`
	ModulePath    repairM2Path `json:"module_path"`
	CompositePath repairM2Path `json:"composite_path"`
}

type repairM2State struct {
	Root         string            `json:"root"`
	Width        uint32            `json:"width"`
	NextSequence uint64            `json:"next_sequence"`
	Modules      []uint16          `json:"modules"`
	Accounts     []repairM2Account `json:"accounts"`
}

type repairM2Transcript struct {
	Producer       string            `json:"producer"`
	SourceHashes   map[string]string `json:"source_hashes"`
	GlobalSequence uint64            `json:"global_sequence"`
	Before         repairM2State     `json:"before"`
	Snapshot       string            `json:"snapshot"`
	SnapshotDigest string            `json:"snapshot_digest"`
	ReceiptRoot    string            `json:"receipt_root"`
	After          repairM2State     `json:"after"`
}

func repairM2Bytes(t *testing.T, encoded string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty native transcript field")
	}
	return raw
}

func repairM2Hash(t *testing.T, encoded string) [32]byte {
	t.Helper()
	raw := repairM2Bytes(t, encoded)
	if len(raw) != 32 {
		t.Fatalf("native hash has %d bytes", len(raw))
	}
	return [32]byte(raw)
}

func repairM2Load(t *testing.T) repairM2Transcript {
	t.Helper()
	raw, err := os.ReadFile("testdata/repair_m2.json")
	if err != nil {
		t.Fatal(err)
	}
	var transcript repairM2Transcript
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transcript); err != nil {
		t.Fatal(err)
	}
	if transcript.Producer != "modules/lxphost/host.go:lxphost_reset" || transcript.GlobalSequence != 1 {
		t.Fatalf("unexpected native recorder: %q at sequence %d", transcript.Producer, transcript.GlobalSequence)
	}
	sources := []string{
		"modules/lxphost/host.go",
		"layerxproof/verify/testdata/repair_m2.c",
		"src/state/lxp_state_root.c",
		"src/state/lxp_state_proof.c",
		"src/state/lxp_snapshot.c",
		"src/storage/lxp_snapshot_store.c",
	}
	if len(transcript.SourceHashes) != len(sources) {
		t.Fatal("native transcript lacks its complete producer provenance")
	}
	for _, source := range sources {
		raw, err := os.ReadFile(filepath.Join("..", "..", source))
		if err != nil {
			t.Fatal(err)
		}
		if sha256.Sum256(raw) != repairM2Hash(t, transcript.SourceHashes[source]) {
			t.Fatalf("native producer %s changed; regenerate the recorded transcript", source)
		}
	}
	return transcript
}

func repairM2CheckPath(t *testing.T, native repairM2Path, index, count uint32, siblings [][32]byte) {
	t.Helper()
	if native.Index != index || native.Count != count || len(native.Siblings) != len(siblings) {
		t.Fatalf("Go path (%d/%d, %d siblings) differs from C path %+v", index, count, len(siblings), native)
	}
	for i, sibling := range siblings {
		if sibling != repairM2Hash(t, native.Siblings[i]) {
			t.Fatalf("sibling %d differs from the C producer", i)
		}
	}
}

func repairM2CheckState(t *testing.T, state repairM2State) {
	t.Helper()
	root := repairM2Hash(t, state.Root)
	if state.Width != 11 || state.NextSequence != 2 {
		t.Fatalf("current host width/sequence = %d/%d, want 11/2", state.Width, state.NextSequence)
	}
	wantModules := map[uint16]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 9: true, 10: true}
	if len(state.Modules) != len(wantModules) {
		t.Fatalf("current host registrations: %v", state.Modules)
	}
	for _, module := range state.Modules {
		if !wantModules[module] {
			t.Fatalf("unexpected or duplicate native registration %d", module)
		}
		delete(wantModules, module)
	}
	if len(state.Accounts) != 3 {
		t.Fatalf("native account count = %d, want 3", len(state.Accounts))
	}
	seenIndices := make(map[uint32]bool)
	seenIDs := make(map[[32]byte]bool)
	for _, native := range state.Accounts {
		name := string(repairM2Bytes(t, native.Name))
		t.Run(name, func(t *testing.T) {
			id, asset := repairM2Hash(t, native.ID), repairM2Hash(t, native.Asset)
			if seenIDs[id] {
				t.Fatal("duplicate native account")
			}
			seenIDs[id] = true
			var ordinal uint64
			switch name {
			case "agent:did:key:a:main":
				ordinal = 1
			case "agent:did:key:b:main":
				ordinal = 2
			case "agent:did:key:c:main":
				ordinal = 3
			default:
				t.Fatalf("unexpected native account name %q", name)
			}
			wantAsset := [32]byte{byte(ordinal)}
			if native.Kind != 1 || asset != wantAsset || native.Balance != (codec.U128{Hi: ordinal, Lo: 99 + ordinal}) {
				t.Fatalf("native account no longer matches the funded recorder inputs: %+v", native)
			}
			derived, err := codec.DeriveAccountID([]byte(name))
			if err != nil || derived != id {
				t.Fatalf("Go/C account identity differs: %x, %v", derived, err)
			}
			wire := repairM2Bytes(t, native.Proof)
			witness, err := verify.StateProof(wire, root)
			if err != nil {
				t.Fatalf("C witness -> Go StateProof: %v", err)
			}
			computed, err := witness.Root()
			if err != nil || computed != root {
				t.Fatalf("Go root differs from the host commit: %x, %v", computed, err)
			}
			key := append([]byte{4}, id[:]...)
			if witness.ModuleID != 0 || !bytes.Equal(witness.Key, key) ||
				!bytes.Equal(witness.Key, repairM2Bytes(t, native.Key)) ||
				!bytes.Equal(witness.Value, repairM2Bytes(t, native.Value)) || witness.AccountPath == nil {
				t.Fatal("Go witness differs from the native account leaf")
			}
			if witness.AccountPath.Count != 3 || witness.LeafCountB != state.Width || len(witness.SiblingsB) != 4 {
				t.Fatal("witness lacks the complete current account/module trees")
			}
			if seenIndices[witness.AccountPath.Index] {
				t.Fatal("duplicate native account tree position")
			}
			seenIndices[witness.AccountPath.Index] = true
			repairM2CheckPath(t, native.AccountPath, witness.AccountPath.Index, witness.AccountPath.Count, witness.AccountPath.Siblings)
			repairM2CheckPath(t, native.ModulePath, witness.LeafIndexA, witness.LeafCountA, witness.SiblingsA)
			repairM2CheckPath(t, native.CompositePath, uint32(witness.ModuleID), witness.LeafCountB, witness.SiblingsB)
			account, err := verify.AccountProof(wire, root, id, &asset)
			if err != nil {
				t.Fatalf("C witness -> Go AccountProof: %v", err)
			}
			if account.AccountID != id || string(account.Name) != name || account.Kind != native.Kind ||
				!account.HasAsset || account.AssetID != asset || account.Balance != native.Balance {
				t.Fatalf("proven account differs from the C registry: %+v", account)
			}
			withoutAsset, err := verify.AccountProof(wire, root, id, nil)
			if err != nil || !reflect.DeepEqual(account, withoutAsset) {
				t.Fatalf("optional asset constraint changed the proven account: %v", err)
			}
		})
	}
	for index := uint32(0); index < 3; index++ {
		if !seenIndices[index] {
			t.Fatalf("account path position %d was not exercised", index)
		}
	}
}

func TestRepairM_2_CurrentCHostStateAndAccountProof(t *testing.T) {
	transcript := repairM2Load(t)
	repairM2CheckState(t, transcript.Before)
}

func TestRepairM_2_PersistedCHostRecovery(t *testing.T) {
	// This transcript was captured across snapshot_store_write, host destruction,
	// snapshot_store_read and snapshot_load into a newly created production host.
	// The verifier is the real consumer; its C peer is a recorded transcript.
	transcript := repairM2Load(t)
	snapshot := repairM2Bytes(t, transcript.Snapshot)
	root := repairM2Hash(t, transcript.Before.Root)
	receiptRoot := repairM2Hash(t, transcript.ReceiptRoot)
	if root == receiptRoot {
		t.Fatal("fixture must distinguish the canonical root from the receipt chain root")
	}
	var fields [80]byte
	binary.BigEndian.PutUint64(fields[:8], transcript.GlobalSequence)
	copy(fields[8:40], root[:])
	copy(fields[40:72], receiptRoot[:])
	binary.BigEndian.PutUint64(fields[72:], uint64(len(snapshot)))
	canonical := append([]byte("LXS2"), fields[:]...)
	canonical = append(canonical, snapshot...)
	digest, err := codec.DomainHash(codec.DomainSnapshot, canonical)
	if err != nil || digest != repairM2Hash(t, transcript.SnapshotDigest) {
		t.Fatalf("C snapshot manifest does not bind the committed root and persisted bytes: %x, %v", digest, err)
	}
	// Snapshot records use canonical ID order; registry insertion order is not
	// part of the committed state.
	sort.Slice(transcript.Before.Accounts, func(i, j int) bool {
		return transcript.Before.Accounts[i].ID < transcript.Before.Accounts[j].ID
	})
	sort.Slice(transcript.After.Accounts, func(i, j int) bool {
		return transcript.After.Accounts[i].ID < transcript.After.Accounts[j].ID
	})
	if !reflect.DeepEqual(transcript.Before, transcript.After) {
		t.Fatal("snapshot reload changed the host root, registrations, accounts or regenerated witness bytes")
	}
	repairM2CheckState(t, transcript.After)
}

func TestRepairM_2_RestoredProofRejectsForeignBindingsAndTampering(t *testing.T) {
	transcript := repairM2Load(t)
	root := repairM2Hash(t, transcript.After.Root)
	for _, native := range transcript.After.Accounts {
		t.Run(string(repairM2Bytes(t, native.Name)), func(t *testing.T) {
			wire := repairM2Bytes(t, native.Proof)
			id, asset := repairM2Hash(t, native.ID), repairM2Hash(t, native.Asset)
			foreignID := id
			foreignID[0] ^= 1
			if _, err := verify.AccountProof(wire, root, foreignID, &asset); !errors.Is(err, codec.ErrAccountIdentity) {
				t.Fatalf("foreign account: got %v, want ErrAccountIdentity", err)
			}
			foreignAsset := asset
			foreignAsset[0] ^= 1
			if _, err := verify.AccountProof(wire, root, id, &foreignAsset); !errors.Is(err, verify.ErrAssetIdentity) {
				t.Fatalf("foreign asset: got %v, want ErrAssetIdentity", err)
			}
			wrongRoot := repairM2Hash(t, transcript.ReceiptRoot)
			if _, err := verify.StateProof(wire, wrongRoot); !errors.Is(err, codec.ErrStateRoot) {
				t.Fatalf("receipt root substituted for committed state root: %v", err)
			}
			if _, err := verify.AccountProof(wire, wrongRoot, id, &asset); !errors.Is(err, codec.ErrStateRoot) {
				t.Fatalf("account under substituted committed root: %v", err)
			}
			witness, err := verify.StateProof(wire, root)
			if err != nil {
				t.Fatal(err)
			}
			valueOffset := bytes.Index(wire, witness.Value)
			if valueOffset < 0 {
				t.Fatal("native value missing from its wire encoding")
			}
			balanceOffset := valueOffset + 2 + len(repairM2Bytes(t, native.Name)) + 1
			// The account-tree leaf is module-zero index 9 of 11. Its
			// parent at index 2 of 3 duplicates itself, so changing a balance
			// violates the canonical duplicate sibling before root comparison.
			if witness.LeafIndexA != 9 || witness.LeafCountA != 11 {
				t.Fatal("recorder no longer exercises the expected odd-width module-zero path")
			}
			for _, corruption := range []struct {
				offset int
				want   error
			}{
				{balanceOffset, codec.ErrStatePath},
				{balanceOffset + 15, codec.ErrStatePath},
				{len(wire) - 1, codec.ErrStateRoot},
			} {
				tampered := bytes.Clone(wire)
				tampered[corruption.offset] ^= 1
				if _, err := verify.StateProof(tampered, root); !errors.Is(err, corruption.want) {
					t.Fatalf("tampered native balance/path at %d: got %v, want %v", corruption.offset, err, corruption.want)
				}
				if _, err := verify.AccountProof(tampered, root, id, &asset); !errors.Is(err, corruption.want) {
					t.Fatalf("tampered account balance/path at %d: got %v, want %v", corruption.offset, err, corruption.want)
				}
			}
			if _, err := verify.StateProof(append(bytes.Clone(wire), 0), root); !errors.Is(err, codec.ErrStateEncoding) {
				t.Fatalf("trailing native witness byte: %v", err)
			}
		})
	}
}
