package meshpki_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/meshpki"
)

func TestLifetimeAndNameBounds(t *testing.T) {
	for _, duration := range []time.Duration{0, -time.Hour, meshpki.MinLifetime - time.Nanosecond, meshpki.MaxAuthorityLifetime + time.Nanosecond} {
		if err := meshpki.CreateAuthority(filepath.Join(t.TempDir(), "material"), duration); err == nil {
			t.Fatal("authority lifetime outside bounds accepted")
		}
	}
	for _, duration := range []time.Duration{meshpki.MinLifetime, meshpki.MaxAuthorityLifetime} {
		dir := t.TempDir()
		if err := meshpki.CreateAuthority(dir, duration); err != nil {
			t.Fatal(err)
		}
		cert := parseCert(t, read(t, dir, "ca.cert.pem"))
		if cert.NotAfter.Sub(cert.NotBefore) != duration {
			t.Fatal("authority validity was clipped")
		}
	}
	dir := t.TempDir()
	authority(t, dir)
	for _, duration := range []time.Duration{0, -time.Hour, meshpki.MinLifetime - time.Nanosecond, meshpki.MaxServiceLifetime + time.Nanosecond} {
		if err := meshpki.IssueService(dir, "user", []string{"user"}, duration); err == nil {
			t.Fatal("service lifetime outside bounds accepted")
		}
	}
	for i, duration := range []time.Duration{meshpki.MinLifetime, meshpki.MaxServiceLifetime} {
		name := []string{"short", "long"}[i]
		if err := meshpki.IssueService(dir, name, []string{name}, duration); err != nil {
			t.Fatal(err)
		}
		cert := parseCert(t, read(t, dir, name+".cert.pem"))
		if cert.NotAfter.Sub(cert.NotBefore) != duration {
			t.Fatal("service validity was clipped")
		}
	}
	short := t.TempDir()
	if err := meshpki.CreateAuthority(short, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := meshpki.IssueService(short, "user", []string{"user"}, 2*time.Hour); err == nil {
		t.Fatal("service outliving authority accepted")
	}
	for _, name := range []string{"", "../user", "User", "user/service", "ca", strings.Repeat("a", 64)} {
		if err := meshpki.CreateAssertionKey(dir, name); err == nil {
			t.Fatal("invalid service accepted for assertion")
		}
		if err := meshpki.IssueService(dir, name, []string{"user"}, time.Hour); err == nil {
			t.Fatal("invalid service accepted for certificate")
		}
		if _, err := meshpki.DescribeService(dir, name); err == nil {
			t.Fatal("invalid service accepted for description")
		}
	}
	for _, names := range [][]string{nil, {""}, {"user", "user"}, {"https://user"}, {"*.user"}, {"-user"}, {"user."}, {"user_1"}, {strings.Repeat("a", 64)}, make([]string, 65)} {
		if err := meshpki.IssueService(dir, "user", names, time.Hour); err == nil {
			t.Fatal("invalid certificate names accepted")
		}
	}
}

func TestWorktreeDirectoriesAreRefused(t *testing.T) {
	for _, markerIsFile := range []bool{false, true} {
		dir := t.TempDir()
		worktree := filepath.Join(dir, "checkout")
		if err := os.Mkdir(worktree, 0o700); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(worktree, ".git")
		if markerIsFile {
			if err := os.WriteFile(marker, []byte("gitdir: ../metadata"), 0o600); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Mkdir(marker, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		link := filepath.Join(dir, "alias")
		if err := os.Symlink(worktree, link); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(dir, "outside")
		if err := os.Mkdir(outside, 0o700); err != nil {
			t.Fatal(err)
		}
		outwardLink := filepath.Join(worktree, "outward")
		if err := os.Symlink(outside, outwardLink); err != nil {
			t.Fatal(err)
		}
		for _, target := range []string{worktree, filepath.Join(worktree, "nested", "material"), filepath.Join(link, "nested"), outwardLink} {
			actions := [][]string{
				{"authority", "--dir", target, "--lifetime", "8760h"},
				{"issue", "--dir", target, "--service", "user", "--names", "user", "--lifetime", "24h"},
				{"assertion", "--dir", target, "--service", "user"},
				{"describe", "--dir", target, "--service", "user"},
			}
			for _, args := range actions {
				var out bytes.Buffer
				err := meshpki.Run(args, &out, &out)
				if err == nil || !strings.Contains(err.Error(), "outside a Git working copy") {
					t.Fatalf("%s did not refuse worktree: %v", args[0], err)
				}
				if out.Len() != 0 {
					t.Fatal("worktree refusal emitted material")
				}
			}
		}
		entries, err := os.ReadDir(worktree)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatal("refusal wrote into worktree")
		}
		outsideEntries, err := os.ReadDir(outside)
		if err != nil {
			t.Fatal(err)
		}
		if len(outsideEntries) != 0 {
			t.Fatal("refusal wrote through worktree link")
		}
	}
	// Exercise the actual source checkout without writing to it.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := meshpki.CreateAssertionKey(cwd, "user"); err == nil {
		t.Fatal("source checkout accepted")
	}
}

func TestExclusiveCreationPreservesCollisionsAndLinks(t *testing.T) {
	for _, collision := range []string{"ca.key.pem", "user.key.pem", "user.assertion.hex"} {
		t.Run(collision, func(t *testing.T) {
			dir := t.TempDir()
			if collision == "user.key.pem" {
				authority(t, dir)
			}
			existing := []byte("operator-owned material")
			if err := os.WriteFile(filepath.Join(dir, collision), existing, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			switch collision {
			case "ca.key.pem":
				err = meshpki.CreateAuthority(dir, time.Hour)
			case "user.key.pem":
				err = meshpki.IssueService(dir, "user", []string{"user"}, time.Hour)
			case "user.assertion.hex":
				err = meshpki.CreateAssertionKey(dir, "user")
			}
			if err == nil {
				t.Fatal("existing file accepted")
			}
			if !bytes.Equal(read(t, dir, collision), existing) {
				t.Fatal("existing material replaced")
			}
			after, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			var beforeNames, afterNames []string
			for _, entry := range before {
				beforeNames = append(beforeNames, entry.Name())
			}
			for _, entry := range after {
				afterNames = append(afterNames, entry.Name())
			}
			if !reflect.DeepEqual(beforeNames, afterNames) {
				t.Fatal("collision left partial outputs")
			}
		})
	}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "existing")
	if err := os.WriteFile(target, []byte("operator-owned material"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "ca.cert.pem")); err != nil {
		t.Fatal(err)
	}
	if err := meshpki.CreateAuthority(dir, time.Hour); err == nil {
		t.Fatal("output symlink accepted")
	}
	if !bytes.Equal(read(t, filepath.Dir(target), filepath.Base(target)), []byte("operator-owned material")) {
		t.Fatal("symlink target changed")
	}
}

func TestCommandRejectsIncompleteArguments(t *testing.T) {
	dir := t.TempDir()
	cases := [][]string{nil, {"unknown"}, {"authority"}, {"authority", "--dir", dir}, {"authority", "--dir", dir, "--lifetime", "invalid"}, {"assertion", "--dir", dir}, {"describe", "--dir", dir, "--service", "user"}, {"assertion", "--dir", dir, "--service", "user", "extra"}, {"assertion", "--dir", dir, "--service", "user", "--unknown"}}
	for _, args := range cases {
		var out, diagnostic bytes.Buffer
		if err := meshpki.Run(args, &out, &diagnostic); err == nil {
			t.Fatal("incomplete arguments accepted")
		}
		if out.Len() != 0 || diagnostic.Len() != 0 {
			t.Fatal("invalid command emitted data")
		}
	}
}

func TestDescribeRejectsInvalidPublicMaterial(t *testing.T) {
	for _, material := range []string{"authority", "certificate", "public"} {
		t.Run(material, func(t *testing.T) {
			dir := t.TempDir()
			authority(t, dir)
			service(t, dir, "user", []string{"user"})
			switch material {
			case "authority":
				other := t.TempDir()
				authority(t, other)
				if err := os.WriteFile(filepath.Join(dir, "ca.cert.pem"), read(t, other, "ca.cert.pem"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "certificate":
				if err := os.WriteFile(filepath.Join(dir, "user.cert.pem"), []byte("invalid certificate"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "public":
				if err := os.WriteFile(filepath.Join(dir, "user.assertion.public"), []byte("invalid"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			if err := meshpki.Run([]string{"describe", "--dir", dir, "--service", "user"}, &out, &out); err == nil {
				t.Fatal("invalid public material accepted")
			}
			if out.Len() != 0 {
				t.Fatal("invalid description emitted output")
			}
		})
	}
}
