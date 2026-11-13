package natsdriver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalMemberProgressRequiresCompleteBoundAuthority(t *testing.T) {
	member, _, _ := epochBrokerFixture(t)
	for _, mode := range []string{"excess members", "unknown member", "invalid stage", "incomplete acknowledgements", "member lock", "invalid inventory", "missing original", "resumed configuration"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "journal.json")
			plan := EpochRecoveryPlan{Proposal: "catalog-cutover", Actor: "operator", Epoch: 3, Region: "primary", Revision: 2}
			candidate := member
			if mode == "missing original" || mode == "resumed configuration" {
				candidate.ConfigPath = filepath.Join(root, "member.conf")
			}
			if mode == "resumed configuration" {
				if err := os.WriteFile(candidate.ConfigPath, []byte("unbound configuration"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			controller, err := NewJournalBrokerEpochController(path, plan, []EpochMemberAuthority{candidate})
			if err != nil {
				t.Fatal(err)
			}
			record := &epochFenceRecord{Identity: epochFenceIdentity{Proposal: plan.Proposal, Actor: plan.Actor, Epoch: plan.Epoch, Region: plan.Region, ConfigurationSHA256: controller.configurationSHA256}, Revision: plan.Revision, State: valueApplying, Members: map[string]*epochMemberProgress{}}
			switch mode {
			case "excess members":
				record.Members["one"] = &epochMemberProgress{}
				record.Members["two"] = &epochMemberProgress{}
			case "unknown member":
				record.Members["unknown"] = &epochMemberProgress{}
			case "invalid stage":
				record.Members[member.Name] = &epochMemberProgress{Stage: "unbound", Acks: map[string]string{}}
			case "incomplete acknowledgements":
				record.Members[member.Name] = &epochMemberProgress{Stage: "prepared", Acks: map[string]string{}}
			case "resumed configuration":
				progress := &epochMemberProgress{Stage: "prepared", Acks: map[string]string{}}
				for _, credential := range member.Credentials {
					progress.Acks[credential.ID] = "$JS.ACK." + member.Probe.Stream + "." + member.Probe.Durable + ".1.1.1.1.1"
				}
				record.Members[member.Name] = progress
			case "member lock":
				if err := os.WriteFile(member.ConfigPath+".epoch.lock", nil, 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(member.ConfigPath+".epoch.lock", 0644); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.Remove(member.ConfigPath + ".epoch.lock"); err != nil {
						t.Error(err)
					}
				}()
			case "invalid inventory":
				controller.members[0].Probe.Batch = 0
			}
			if err := writeEpochJournal(path, record); err != nil {
				t.Fatal(err)
			}
			if result, err := controller.Fence(context.Background(), plan.Proposal, plan.Actor); err == nil || result.ProofSHA256 != "" {
				t.Fatalf("unbound progress admitted: %+v %v", result, err)
			}
		})
	}
	for _, input := range []struct {
		path string
		plan EpochRecoveryPlan
	}{{"relative", EpochRecoveryPlan{}}, {filepath.Join(t.TempDir(), "missing", "journal"), EpochRecoveryPlan{Proposal: "cutover", Actor: "operator", Revision: 1}}} {
		if _, err := NewJournalBrokerEpochController(input.path, input.plan, []EpochMemberAuthority{member}); err == nil {
			t.Fatal("unbound journal authority admitted")
		}
	}
	var absent *BrokerEpochController
	if _, err := absent.Fence(context.Background(), "cutover", "operator"); err == nil {
		t.Fatal("missing controller admitted")
	}
	secured := member
	secured.URL = strings.Replace(member.URL, "nats://", "tls://", 1)
	if err := epochCommandDenied(context.Background(), secured, member.Credentials[0], member.Probe.Subject, nil); err == nil {
		t.Fatal("unverified broker TLS certified denial")
	}
}
