package rehearsal

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The historical integration suite uses real processes. The bounded repair
// tests use the same configuration boundary with in-process transcript peers.
func RepairM174DaemonOptions(t *testing.T, bin string) Options {
	t.Helper()
	o := repairM174IsolationFixture(t).options
	root := filepath.Dir(o.AttestorConfig.CAFile)
	signer, err := newAuthority()
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&signer.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicFile := filepath.Join(root, "inventory-public.pem")
	if err := writePEM(publicFile, "PUBLIC KEY", public); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	members := make([]map[string]string, 0, 5)
	for _, member := range o.AttestorConfig.Nodes {
		members = append(members, map[string]string{"id": member.ID, "spki_sha256": hex.EncodeToString(member.Pin[:])})
	}
	header, err := json.Marshal(map[string]string{"alg": "ES256", "typ": "wallet-custody-inventory+jwt"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{"version": 1, "iss": "rehearsal-fixture", "aud": "wallet-custody-inventory", "tenant": "rehearsal", "sequence": "1", "iat": now, "exp": now + 3600, "protocol": "wallet", "threshold": 3, "members": members, "keys": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	signing := b64(header) + "." + b64(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, signer.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	inventory := filepath.Join(root, "inventory.jwt")
	if err := os.WriteFile(inventory, []byte(signing+"."+b64(signature)), 0600); err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(root, "policy.json")
	if err := os.WriteFile(policy, []byte(`{"version":1,"defaults":{"chain_id":125,"kinds":["personal_message"],"rate_per_minute":1000}}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, group := range []struct {
		name     string
		delivery bool
	}{{"delivery", true}, {"isolated", false}} {
		cfg := *o.AttestorConfig
		if group.delivery {
			cfg = *o.DeliveryAttestorConfig
		}
		cfg.Nodes = append(cfg.Nodes[:0:0], cfg.Nodes...)
		dir := filepath.Join(root, group.name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		cluster := &Cluster{Config: cfg, dir: dir}
		t.Cleanup(cluster.Stop)
		addresses, err := freeAddrs(2 * len(cfg.Nodes))
		if err != nil {
			t.Fatal(err)
		}
		for i := range cfg.Nodes {
			cfg.Nodes[i].URL = "https://" + addresses[2*i]
		}
		cluster.Config = cfg
		for i, node := range cfg.Nodes {
			var peers, pins []string
			for j, peer := range cfg.Nodes {
				if i != j {
					peers = append(peers, peer.ID+"="+addresses[2*j+1])
					pins = append(pins, peer.ID+"="+hex.EncodeToString(peer.Pin[:]))
				}
			}
			var nodeKey [32]byte
			if _, err := rand.Read(nodeKey[:]); err != nil {
				t.Fatal(err)
			}
			keyFile := filepath.Join(dir, node.ID+".key")
			if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(nodeKey[:])), 0600); err != nil {
				t.Fatal(err)
			}
			zero(nodeKey[:])
			logFile, err := os.Create(filepath.Join(dir, node.ID+".log"))
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(bin)
			command.Env = []string{
				"ATTESTOR_NODE_ID=" + node.ID, "ATTESTOR_REGION=rehearsal", "ATTESTOR_LISTEN_ADDR=" + addresses[2*i], "ATTESTOR_PEER_LISTEN_ADDR=" + addresses[2*i+1],
				"ATTESTOR_PEERS=" + strings.Join(peers, ","), "ATTESTOR_PEER_PINS=" + strings.Join(pins, ","), "ATTESTOR_NODE_KEY_FILE=" + keyFile, "ATTESTOR_DATA_DIR=" + filepath.Join(dir, node.ID),
				"ATTESTOR_CEREMONY=true", "ATTESTOR_CHAIN_ID=" + strconv.Itoa(ChainID), "ATTESTOR_POLICY_FILE=" + policy,
				"ATTESTOR_TLS_CERT_FILE=" + filepath.Join(root, node.ID+".pem"), "ATTESTOR_TLS_KEY_FILE=" + filepath.Join(root, node.ID+"-key.pem"), "ATTESTOR_TLS_CA_FILE=" + cfg.CAFile, "ATTESTOR_OPERATOR_CA_FILE=" + filepath.Join(root, "operator-ca.pem"),
				"ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE=" + publicFile, "ATTESTOR_AUTHORITY_ISSUER=rehearsal-fixture", "ATTESTOR_AUTHORITY_TENANT=rehearsal", "ATTESTOR_INVENTORY_PUBLIC_KEY_FILE=" + publicFile, "ATTESTOR_INVENTORY_FILE=" + inventory,
			}
			command.Stdout, command.Stderr = logFile, logFile
			err = command.Start()
			logFile.Close()
			if err != nil {
				t.Fatal(err)
			}
			exited := make(chan struct{})
			go func() { _ = command.Wait(); close(exited) }()
			cluster.procs = append(cluster.procs, command)
			cluster.exited = append(cluster.exited, exited)
		}
		if err := cluster.waitReady(ctx); err != nil {
			t.Fatalf("real %s group: %v", group.name, err)
		}
		if group.delivery {
			o.DeliveryAttestorConfig = &cluster.Config
		} else {
			o.AttestorConfig = &cluster.Config
		}
	}
	if err := o.validateIsolation(); err != nil {
		t.Fatal(err)
	}
	return o
}

func RepairM174DeliveryDigest(t *testing.T, o Options) [32]byte {
	t.Helper()
	root := filepath.Join(filepath.Dir(o.DeliveryAttestorConfig.CAFile), "delivery")
	hash := sha256.New()
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasSuffix(path, ".log") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%d:%s:%d:", len(relative), relative, len(content))
		hash.Write(content)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}
