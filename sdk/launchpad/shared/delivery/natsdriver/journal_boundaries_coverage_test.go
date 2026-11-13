package natsdriver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestEpochJournalRequiresPrivateCanonicalFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(root, "private")
	if err := os.WriteFile(private, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, root} {
		if _, err := readEpochJournal(path); err == nil {
			t.Fatal("nonregular journal admitted")
		}
	}
	if err := os.Chmod(private, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readEpochJournal(private); err == nil {
		t.Fatal("public journal admitted")
	}
	if err := os.Chmod(private, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative", filepath.Join(root, "missing"), private} {
		if err := privateEpochDirectory(path); err == nil {
			t.Fatal("noncanonical journal directory admitted")
		}
	}
	if err := os.Chmod(root, 0770); err != nil {
		t.Fatal(err)
	}
	if err := privateEpochDirectory(root); err == nil {
		t.Fatal("shared journal directory admitted")
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	oversized := &epochFenceRecord{Identity: epochFenceIdentity{Proposal: strings.Repeat("x", (1<<20)+1)}}
	if err := writeEpochJournal(filepath.Join(root, "large"), oversized); err == nil {
		t.Fatal("unbounded journal admitted")
	}
	if err := writeEpochJournal(filepath.Join(root, "missing", "journal"), &epochFenceRecord{}); err == nil {
		t.Fatal("missing journal directory admitted")
	}
	if err := writeEpochJournal(root, &epochFenceRecord{}); err == nil {
		t.Fatal("journal replaced directory")
	}
}

func TestEpochJournalRejectsAuthorityChangesAndConcurrentWriters(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "journal.json")
	plan := EpochRecoveryPlan{Proposal: "cutover", Actor: "operator", Epoch: 3, Region: "primary", Revision: 2}
	store := &fileEpochFenceStore{path: path, plan: plan}
	id := epochFenceIdentity{Proposal: plan.Proposal, Actor: plan.Actor, Epoch: plan.Epoch, Region: plan.Region, ConfigurationSHA256: strings.Repeat("a", 64)}
	ctx := context.Background()
	rejected := errors.New("member transition rejected")
	if err := store.run(ctx, id, false, func(record *epochFenceRecord, save func() error) error {
		record.Members["member"] = &epochMemberProgress{Stage: "prepared", Acks: map[string]string{}}
		if err := save(); err != nil {
			return err
		}
		return rejected
	}); !errors.Is(err, rejected) {
		t.Fatal(err)
	}
	wire, err := readEpochJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	var recorded epochFenceRecord
	if err := json.Unmarshal(wire, &recorded); err != nil || recorded.State != valueFailed || recorded.Result != nil {
		t.Fatalf("incomplete journal state: %+v %v", recorded, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.run(cancelled, id, false, func(*epochFenceRecord, func() error) error { t.Fatal("cancelled journal invoked"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	changed := id
	changed.Actor = "other"
	if err := store.run(ctx, changed, false, func(*epochFenceRecord, func() error) error { return nil }); err == nil {
		t.Fatal("changed operator admitted")
	}
	for _, mode := range []string{"malformed", "revision", "configuration", "state", "members"} {
		altered := recorded
		switch mode {
		case "revision":
			altered.Revision++
		case "configuration":
			altered.Identity.ConfigurationSHA256 = strings.Repeat("b", 64)
		case "state":
			altered.State = "unknown"
		case "members":
			altered.Members = nil
		}
		encoded, err := json.Marshal(altered)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "malformed" {
			encoded = []byte("{")
		}
		if err := os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		if err := store.run(ctx, id, false, func(*epochFenceRecord, func() error) error { t.Fatal("altered journal invoked"); return nil }); err == nil {
			t.Fatal("altered durable authority admitted")
		}
	}
	lock, err := os.OpenFile(path+".lock", os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := store.run(ctx, id, false, func(*epochFenceRecord, func() error) error { return nil }); err == nil {
		t.Fatal("concurrent journal writer admitted")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path+".lock", 0644); err != nil {
		t.Fatal(err)
	}
	if err := store.run(ctx, id, false, func(*epochFenceRecord, func() error) error { return nil }); err == nil {
		t.Fatal("public writer lock admitted")
	}
}

func TestEpochMemberLocksAndConfigurationFilesArePrivate(t *testing.T) {
	member, _, _ := epochBrokerFixture(t)
	for _, mode := range []string{"missing", "symlink", "public", "directory", "parent"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			candidate := member
			candidate.ConfigPath = filepath.Join(root, "server.conf")
			switch mode {
			case "symlink":
				if err := os.Symlink(member.ConfigPath, candidate.ConfigPath); err != nil {
					t.Fatal(err)
				}
			case "public":
				if err := os.WriteFile(candidate.ConfigPath, []byte("config"), 0644); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(candidate.ConfigPath, 0700); err != nil {
					t.Fatal(err)
				}
			case "parent":
				candidate.ConfigPath = filepath.Join(root, "missing", "server.conf")
			}
			if err := replaceEpochConfig(candidate); err == nil {
				t.Fatal("unadmitted configuration file replaced")
			}
		})
	}
	release, err := lockEpochMembers([]EpochMemberAuthority{member})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockEpochMembers([]EpochMemberAuthority{member}); err == nil {
		t.Fatal("concurrent member transition admitted")
	}
	release()
	if err := os.Chmod(member.ConfigPath+".epoch.lock", 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := lockEpochMembers([]EpochMemberAuthority{member}); err == nil {
		t.Fatal("public member transition lock admitted")
	}
	if err := os.Remove(member.ConfigPath + ".epoch.lock"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(member.ConfigPath, member.ConfigPath+".epoch.lock"); err != nil {
		t.Fatal(err)
	}
	if _, err := lockEpochMembers([]EpochMemberAuthority{member}); err == nil {
		t.Fatal("symlink member lock admitted")
	}
	candidate := member
	candidate.ConfigPath = filepath.Join(t.TempDir(), "missing", "member")
	if _, err := lockEpochMembers([]EpochMemberAuthority{candidate}); err == nil {
		t.Fatal("missing member directory admitted")
	}
	for _, edit := range []func(*EpochMemberAuthority){func(m *EpochMemberAuthority) { m.Probe.Batch = 0 }, func(m *EpochMemberAuthority) { m.Probe.Subject = "accounting.events" }, func(m *EpochMemberAuthority) { m.Credentials = append(m.Credentials, m.Credentials[0]) }} {
		candidate := member
		edit(&candidate)
		if err := validateEpochMember(candidate); err == nil {
			t.Fatal("unreviewed member inventory admitted")
		}
	}
	if _, err := newBrokerEpochController(nil, 0, "", nil); err == nil {
		t.Fatal("missing epoch authority admitted")
	}
	if _, err := newBrokerEpochController(&fileEpochFenceStore{}, 3, "primary", []EpochMemberAuthority{member, member}); err == nil {
		t.Fatal("duplicate member authority admitted")
	}
}
