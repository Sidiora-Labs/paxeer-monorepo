package scmigrate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/tmhash"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repair121Payload(t *testing.T, height int64) []byte {
	t.Helper()
	key := ed25519.TestSecretKey([]byte("scmigrate-repair-121"))
	dir := t.TempDir()
	validator := privval.NewFilePV(key, filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
	validators := types.NewValidatorSet([]*types.Validator{types.NewValidator(key.Public(), 10)})
	blockID := types.BlockID{
		Hash:          tmhash.Sum([]byte("block")),
		PartSetHeader: types.PartSetHeader{Total: 1, Hash: tmhash.Sum([]byte("parts"))},
	}
	votes := types.NewVoteSet("repair-121", height, 0, tmproto.PrecommitType, validators)
	commit, err := types.MakeCommit(t.Context(), blockID, height, 0, votes,
		[]types.PrivValidator{validator}, time.Unix(height, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	memory := dbm.NewMemDB()
	defer func() { _ = memory.Close() }()
	if err := store.NewBlockStore(memory).SaveSeenCommit(height, commit); err != nil {
		t.Fatal(err)
	}
	payload, err := memory.Get(makeKeyFromPrefix(prefixSeenCommit))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Clone(payload)
}

func repair121Open(t *testing.T, dir string) dbm.DB {
	t.Helper()
	db, err := dbm.NewGoLevelDB("seen-commit", dir)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func repair121Close(t *testing.T, db dbm.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func repair121Seed(t *testing.T, db dbm.DB, canonicalHeight int64, winnerAtSource bool) []byte {
	t.Helper()
	winner := repair121Payload(t, 11)
	batch := db.NewBatch()
	defer func() { _ = batch.Close() }()
	if err := batch.Set(makeKeyFromPrefix(prefixSeenCommit), repair121Payload(t, canonicalHeight)); err != nil {
		t.Fatal(err)
	}
	for _, height := range []int64{3, 8} {
		if err := batch.Set(makeKeyFromPrefix(prefixSeenCommit, height), repair121Payload(t, height)); err != nil {
			t.Fatal(err)
		}
	}
	if winnerAtSource {
		if err := batch.Set(makeKeyFromPrefix(prefixSeenCommit, 11), winner); err != nil {
			t.Fatal(err)
		}
	}
	if err := batch.Set([]byte("unrelated"), []byte("preserve")); err != nil {
		t.Fatal(err)
	}
	if err := batch.WriteSync(); err != nil {
		t.Fatal(err)
	}
	return winner
}

func repair121AssertWinner(t *testing.T, db dbm.DB, winner []byte) {
	t.Helper()
	records, err := getAllSeenCommits(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || !bytes.Equal(records[0].key, makeKeyFromPrefix(prefixSeenCommit)) {
		t.Fatalf("expected exactly one canonical seen commit, got %v", records)
	}
	payload, err := db.Get(makeKeyFromPrefix(prefixSeenCommit))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, winner) {
		t.Fatal("migration did not preserve the highest commit's original bytes")
	}
	commit := store.NewBlockStore(db).LoadSeenCommit()
	if commit == nil || commit.Height != 11 {
		t.Fatalf("block store recovered commit %v, want height 11", commit)
	}
	if err := commit.ValidateBasic(); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(commit.ToProto(), records[0].commit.ToProto()) {
		t.Fatal("block store and migration decoded different commits")
	}
	unrelated, err := db.Get([]byte("unrelated"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unrelated, []byte("preserve")) {
		t.Fatal("migration changed an unrelated record")
	}
}

func TestRepair_121_OlderCanonicalNewerLegacy(t *testing.T) {
	dir := t.TempDir()
	db := repair121Open(t, dir)
	winner := repair121Seed(t, db, 7, true)
	repair121Close(t, db)
	for range 3 {
		db = repair121Open(t, dir)
		if err := Migrate(t.Context(), db); err != nil {
			t.Fatal(err)
		}
		repair121Close(t, db)
		db = repair121Open(t, dir)
		repair121AssertWinner(t, db, winner)
		repair121Close(t, db)
	}
}

func TestRepair_121_InterruptedRepeatedMixedFormats(t *testing.T) {
	if dir := os.Getenv("SCMIGRATE_REPAIR121_DIR"); dir != "" {
		db := repair121Open(t, dir)
		if os.Getenv("SCMIGRATE_REPAIR121_PHASE") == "scanned" {
			records, err := getAllSeenCommits(t.Context(), db)
			if err != nil {
				t.Fatal(err)
			}
			sortMigrations(records)
			if len(records) == 0 || records[0].commit.Height != 11 {
				t.Fatal("migration scan did not select the highest commit")
			}
		} else if err := Migrate(t.Context(), db); err != nil {
			t.Fatal(err)
		}
		// Exit without closing the database: recovery must rely on durable data.
		os.Exit(0)
	}

	for _, initial := range []struct {
		name            string
		canonicalHeight int64
		winnerAtSource  bool
	}{
		{"older-canonical", 7, true},
		{"winner-copied", 11, true},
		{"winner-renamed", 11, false},
	} {
		t.Run(initial.name, func(t *testing.T) {
			dir := t.TempDir()
			db := repair121Open(t, dir)
			winner := repair121Seed(t, db, initial.canonicalHeight, initial.winnerAtSource)
			before, err := getAllSeenCommits(t.Context(), db)
			if err != nil {
				t.Fatal(err)
			}
			repair121Close(t, db)
			for _, phase := range []string{"scanned", "written", "written"} {
				cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRepair_121_InterruptedRepeatedMixedFormats$")
				cmd.Env = append(os.Environ(), "SCMIGRATE_REPAIR121_DIR="+dir, "SCMIGRATE_REPAIR121_PHASE="+phase)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("interrupted migration (%s): %v\n%s", phase, err, output)
				}
				db = repair121Open(t, dir)
				if phase == "scanned" {
					after, err := getAllSeenCommits(t.Context(), db)
					if err != nil {
						t.Fatal(err)
					}
					if len(after) != len(before) {
						t.Fatalf("interrupted scan changed record count: %d -> %d", len(before), len(after))
					}
					for i := range before {
						if !bytes.Equal(before[i].key, after[i].key) || !proto.Equal(before[i].commit.ToProto(), after[i].commit.ToProto()) {
							t.Fatal("interrupted scan changed a durable commit")
						}
					}
				} else {
					repair121AssertWinner(t, db, winner)
				}
				repair121Close(t, db)
			}
		})
	}
}
