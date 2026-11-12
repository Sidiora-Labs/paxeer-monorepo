package natsdriver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEpochProbesRequireActualBrokerIdentityAndDurableDelivery(t *testing.T) {
	member, _, _ := epochBrokerFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, mode := range []string{"authentication", "server identity", "stream identity", "durable identity", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			candidate := member
			candidate.Credentials = append([]EpochCredential(nil), member.Credentials...)
			callCtx := ctx
			switch mode {
			case "authentication":
				candidate.Credentials[0].Password = "invalid-credential"
			case "server identity":
				candidate.ServerID = "other"
			case "stream identity":
				candidate.Probe.Stream = "ABSENT"
			case "durable identity":
				candidate.Probe.Durable = "absent"
			case "cancelled":
				bounded, stop := context.WithCancel(ctx)
				stop()
				callCtx = bounded
			}
			probes, err := prepareEpochProbes(callCtx, candidate, "probe-identity")
			if err == nil || probes != nil {
				for _, p := range probes {
					p.conn.Close()
				}
				t.Fatalf("unbound %s probe admitted: %v", mode, err)
			}
		})
	}
	if err := epochCommandDenied(ctx, member, member.Credentials[0], member.Probe.Subject, []byte("live-credential")); err == nil {
		t.Fatal("live credential certified as revoked")
	}
	wrong := member
	wrong.ServerID = "other"
	if err := epochCommandDenied(ctx, wrong, member.Credentials[0], member.Probe.Subject, nil); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("foreign denial broker: %v", err)
	}
	wrong.URL = "%"
	if err := epochCommandDenied(ctx, wrong, member.Credentials[0], member.Probe.Subject, nil); err == nil {
		t.Fatal("malformed denial address admitted")
	}
	wrong.URL = member.URL
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if err := epochCommandDenied(cancelled, wrong, member.Credentials[0], member.Probe.Subject, nil); err == nil {
		t.Fatal("network unavailability certified as denial")
	}
	progress := &epochMemberProgress{Acks: map[string]string{}}
	if _, err := verifyResumedEpochMember(ctx, member, progress); err == nil {
		t.Fatal("missing acknowledgement identity admitted")
	}
	progress.Acks[member.Credentials[0].ID] = "$JS.ACK." + member.Probe.Stream + "." + member.Probe.Durable + ".1.1.1.1.1"
	if _, err := verifyResumedEpochMember(ctx, member, progress); err == nil {
		t.Fatal("live resumed credential certified as revoked")
	}
}

func TestBrokerConfigurationRejectsUnboundFilesystemIdentity(t *testing.T) {
	for _, mode := range []string{"missing directory", "directory permissions", "missing file", "symlink", "public file", "replaced bytes"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "server.conf")
			original, replacement := []byte("original broker configuration"), []byte("reviewed broker configuration")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			member := EpochMemberAuthority{ConfigPath: path, ExistingSHA256: epochSHA(original), Replacement: replacement, ReplacementSHA256: epochSHA(replacement)}
			switch mode {
			case "missing directory":
				member.ConfigPath = filepath.Join(directory, "absent", "server.conf")
			case "directory permissions":
				if err := os.Chmod(directory, 0777); err != nil {
					t.Fatal(err)
				}
			case "missing file":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(directory, "actual.conf")
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "public file":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "replaced bytes":
				if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := replaceEpochConfig(member); err == nil {
				t.Fatal("unbound broker configuration replaced")
			}
		})
	}
}
