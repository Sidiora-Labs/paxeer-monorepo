package meshpki

import (
	"bytes"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaterialFilesRequireBoundedRegularContents(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	for _, size := range []int{0, 1, 1048576, 1048577} {
		name := filepath.Join(dir, "material")
		contents := bytes.Repeat([]byte{'x'}, size)
		if err := os.WriteFile(name, contents, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := readFile(root, "material")
		if size == 1 || size == 1048576 {
			if err != nil || !bytes.Equal(got, contents) {
				t.Fatalf("bounded material size %d: %v", size, err)
			}
		} else if err == nil || got != nil {
			t.Fatalf("unbounded material size %d accepted", size)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "directory"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("material", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "directory", "link"} {
		if got, err := readFile(root, name); err == nil || got != nil {
			t.Fatalf("nonregular material %s accepted", name)
		}
	}
	if _, err := resolveDirectory(""); err == nil {
		t.Fatal("empty output directory accepted")
	}
	if _, err := openStore(filepath.Join(dir, "missing"), false); err == nil {
		t.Fatal("missing public store accepted")
	}
	if err := os.Symlink("cycle", filepath.Join(dir, "cycle")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveDirectory(filepath.Join(dir, "cycle")); err == nil {
		t.Fatal("cyclic output directory accepted")
	}
}

func TestServiceMaterialRequiresMatchingAuthorityAndIdentity(t *testing.T) {
	for _, material := range []string{"missing authority", "invalid authority", "missing key", "different key", "different service", "missing public key"} {
		t.Run(material, func(t *testing.T) {
			dir := t.TempDir()
			if err := CreateAuthority(dir, 24*time.Hour); err != nil {
				t.Fatal(err)
			}
			if material == "different service" || material == "missing public key" {
				if err := IssueService(dir, "user", []string{"user"}, time.Hour); err != nil {
					t.Fatal(err)
				}
			}
			switch material {
			case "missing authority":
				if err := os.Remove(filepath.Join(dir, "ca.cert.pem")); err != nil {
					t.Fatal(err)
				}
			case "invalid authority":
				if err := os.WriteFile(filepath.Join(dir, "ca.cert.pem"), []byte("certificate"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing key":
				if err := os.Remove(filepath.Join(dir, "ca.key.pem")); err != nil {
					t.Fatal(err)
				}
			case "different key":
				other := t.TempDir()
				if err := CreateAuthority(other, 24*time.Hour); err != nil {
					t.Fatal(err)
				}
				key, err := os.ReadFile(filepath.Join(other, "ca.key.pem"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "ca.key.pem"), key, 0600); err != nil {
					t.Fatal(err)
				}
			case "different service":
				if err := os.Rename(filepath.Join(dir, "user.cert.pem"), filepath.Join(dir, "other.cert.pem")); err != nil {
					t.Fatal(err)
				}
			}
			if material == "different service" {
				if _, err := DescribeService(dir, "other"); err == nil || err.Error() != "certificate service does not match" {
					t.Fatalf("service identity: %v", err)
				}
			} else if material == "missing public key" {
				if _, err := DescribeService(dir, "user"); !os.IsNotExist(err) {
					t.Fatalf("assertion public key required: %v", err)
				}
			} else if err := IssueService(dir, "user", []string{"user"}, time.Hour); err == nil {
				t.Fatal("incomplete or mismatched authority accepted")
			}
		})
	}
	for _, raw := range [][]byte{
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1}}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1}}),
	} {
		if _, err := certificate(raw); err == nil {
			t.Fatal("invalid certificate material accepted")
		}
	}
	if validDNS(strings.Repeat("a.", 127)+"a") || validDNS("user-") {
		t.Fatal("unbounded or hyphen-terminated DNS name accepted")
	}
}
