package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
)

const maxLifetime = 30 * 24 * time.Hour
const maxTokenBytes = 2 << 20
const maxSafeInteger = 1<<53 - 1

var operations = []string{"generate", "sign", "import", "refresh"}

type memberFlags []string

func (m *memberFlags) String() string { return "id=certificate.pem" }
func (m *memberFlags) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: inventory {init|approve|renew|show} -private-key FILE -issuer ISSUER -tenant TENANT [flags]")
	}
	command := args[0]
	if command != "init" && command != "approve" && command != "renew" && command != "show" {
		return errors.New("inventory subcommand must be init, approve, renew or show")
	}
	flags := flag.NewFlagSet("inventory "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	privatePath := flags.String("private-key", "", "owner P-256 PEM file (0600, owned by effective user)")
	issuer := flags.String("issuer", "", "inventory issuer")
	tenant := flags.String("tenant", "", "inventory tenant")
	lifetimeText := flags.String("lifetime", "720h", "validity duration, at most 720h")
	inventoryPath := flags.String("inventory", "", "present signed inventory")
	output := flags.String("output", "", "destination for the signed inventory")
	var members memberFlags
	var requests string
	var listKeys bool
	if command == "init" {
		flags.Var(&members, "member", "member id=certificate.pem (repeat five times)")
	}
	if command == "approve" {
		flags.StringVar(&requests, "requests", "", "JSON document containing inventory_requests")
	}
	if command == "show" {
		flags.BoolVar(&listKeys, "keys", false, "print one line per key")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(stdout)
			flags.PrintDefaults()
			return nil
		}
		return errors.New("invalid inventory command flags")
	}
	if flags.NArg() != 0 {
		return errors.New("inventory command does not accept positional inputs")
	}
	for _, required := range []struct{ name, value string }{{"private-key", *privatePath}, {"issuer", *issuer}, {"tenant", *tenant}} {
		if required.value == "" {
			return fmt.Errorf("-%s is required", required.name)
		}
	}
	lifetime, err := time.ParseDuration(*lifetimeText)
	if err != nil || lifetime < time.Second || lifetime > maxLifetime || lifetime%time.Second != 0 {
		return errors.New("-lifetime must be a whole number of seconds between 1s and 720h")
	}
	key, err := loadPrivateKey(*privatePath)
	if err != nil {
		return err
	}
	verifier := &agent.SignedVerifier{Key: &key.PublicKey, Issuer: *issuer, Tenant: *tenant}
	var inventory server.WalletInventory
	if command == "init" {
		inventory = server.WalletInventory{
			SignedClaims: agent.SignedClaims{Version: 1, Issuer: *issuer, Tenant: *tenant, Audience: agent.InventoryAudience, Sequence: "1"},
			Protocol:     "wallet", Threshold: 3, Keys: []server.InventoryKey{},
		}
		if len(members) != 5 {
			return errors.New("-member requires exactly five certificates")
		}
		for _, member := range members {
			id, path, ok := strings.Cut(member, "=")
			if !ok || id == "" || path == "" {
				return errors.New("-member must be id=certificate.pem")
			}
			pin, err := certificatePin(path)
			if err != nil {
				return err
			}
			inventory.Members = append(inventory.Members, server.InventoryMember{ID: id, Pin: pin})
		}
	} else {
		if *inventoryPath == "" {
			return errors.New("-inventory is required")
		}
		inventory, err = readInventory(*inventoryPath, verifier)
		if err != nil {
			return err
		}
		if command == "show" {
			return showInventory(stdout, inventory, listKeys)
		}
		sequence, _ := strconv.ParseUint(inventory.Sequence, 10, 64)
		if sequence == ^uint64(0) {
			return fmt.Errorf("inventory %q: sequence exhausted", *inventoryPath)
		}
		inventory.Sequence = strconv.FormatUint(sequence+1, 10)
		if command == "approve" {
			if requests == "" {
				return errors.New("-requests is required")
			}
			if err := approveRequests(&inventory, requests); err != nil {
				return err
			}
		}
	}
	if *output == "" {
		return errors.New("-output is required")
	}
	if sameFile(*output, *privatePath) {
		return errors.New("-output must differ from -private-key")
	}
	inventory.IssuedAt = time.Now().Unix()
	inventory.ExpiresAt = inventory.IssuedAt + int64(lifetime/time.Second)
	if err := validateInventory(inventory); err != nil {
		return fmt.Errorf("inventory inputs: %w", err)
	}
	token, err := encodeInventory(key, inventory)
	if err != nil {
		return err
	}
	var checked server.WalletInventory
	if err := verifier.Decode(token, agent.InventoryAudience, maxLifetime, &checked); err != nil {
		return errors.New("inventory inputs: invalid signed claims")
	}
	return writeInventory(*output, []byte(token+"\n"))
}

func loadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	refused := fmt.Errorf("private key file %q: require an owned regular 0600 P-256 PEM file", path)
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, refused
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, refused
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || stat.Uid != uint32(os.Geteuid()) || info.Size() < 1 || info.Size() > 16384 {
		return nil, refused
	}
	raw, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(raw) > 16384 {
		return nil, refused
	}
	block, rest := pem.Decode(raw)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, refused
	}
	var parsed any
	switch block.Type {
	case "EC PRIVATE KEY":
		parsed, err = x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	default:
		return nil, refused
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if err != nil || !ok || key.Curve != elliptic.P256() {
		return nil, refused
	}
	return key, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("input file %q: unavailable", path)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input file %q: require a regular file", path)
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, fmt.Errorf("input file %q: unreadable or exceeds size bound", path)
	}
	return raw, nil
}

func certificatePin(path string) (string, error) {
	raw, err := readBounded(path, 1<<20)
	if err != nil {
		return "", err
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return "", fmt.Errorf("certificate file %q: invalid PEM certificate", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("certificate file %q: invalid certificate", path)
	}
	pin := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(pin[:]), nil
}

func readInventory(path string, verifier *agent.SignedVerifier) (server.WalletInventory, error) {
	var inventory server.WalletInventory
	raw, err := readBounded(path, maxTokenBytes)
	if err != nil {
		return inventory, err
	}
	if verifier.Decode(strings.TrimSpace(string(raw)), agent.InventoryAudience, maxLifetime, &inventory) != nil {
		return inventory, fmt.Errorf("inventory file %q: invalid signature, scope or lifetime", path)
	}
	if err := validateInventory(inventory); err != nil {
		return inventory, fmt.Errorf("inventory file %q: %w", path, err)
	}
	return inventory, nil
}

func validateInventory(inventory server.WalletInventory) error {
	if inventory.Protocol != "wallet" || inventory.Threshold != 3 || len(inventory.Members) != 5 || inventory.Keys == nil || len(inventory.Keys) > 65536 {
		return errors.New("invalid protocol, threshold, member count or key count")
	}
	ids, pins := map[string]bool{}, map[string]bool{}
	for _, member := range inventory.Members {
		if member.ID == "" || ids[member.ID] || pins[member.Pin] || !lowerHex(member.Pin, 32) {
			return errors.New("invalid or duplicate member identity or pin")
		}
		ids[member.ID], pins[member.Pin] = true, true
	}
	keys := map[string]bool{}
	for _, key := range inventory.Keys {
		if keys[key.KeyID] || !validKey(key) {
			return fmt.Errorf("key %q: invalid or duplicate inventory entry", key.KeyID)
		}
		keys[key.KeyID] = true
	}
	return nil
}

func lowerHex(value string, size int) bool {
	if len(value) != size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validKey(key server.InventoryKey) bool {
	if key.KeyID == "" || key.Owner == "" || key.Epoch > maxSafeInteger || (key.Curve != "ed25519" && key.Curve != "secp256k1") || len(key.Operations) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, op := range key.Operations {
		if seen[op] || (op != "generate" && op != "sign" && op != "import" && op != "refresh") {
			return false
		}
		seen[op] = true
	}
	if key.PublicKey == "" {
		return key.Epoch == 0 && len(seen) == 1 && seen["generate"]
	}
	if key.Curve == "ed25519" {
		return lowerHex(key.PublicKey, 32)
	}
	return lowerHex(key.PublicKey, 65) && strings.HasPrefix(key.PublicKey, "04")
}

func decodeJSON(raw []byte, out any, strict bool) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		d.DisallowUnknownFields()
	}
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("expected one JSON document")
	}
	return nil
}

func approveRequests(inventory *server.WalletInventory, path string) error {
	raw, err := readBounded(path, maxTokenBytes)
	if err != nil {
		return err
	}
	var document struct {
		Requests json.RawMessage `json:"inventory_requests"`
	}
	if decodeJSON(raw, &document, false) != nil || len(document.Requests) == 0 || bytes.Equal(bytes.TrimSpace(document.Requests), []byte("null")) {
		return fmt.Errorf("requests file %q: require an inventory_requests array", path)
	}
	var entries []json.RawMessage
	if decodeJSON(document.Requests, &entries, true) != nil || len(entries) > 65536 {
		return fmt.Errorf("requests file %q: invalid inventory_requests array", path)
	}
	keys := append([]server.InventoryKey{}, inventory.Keys...)
	indices := map[string]int{}
	for i, key := range keys {
		indices[key.KeyID] = i
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		var request server.InventoryKey
		var identity struct {
			KeyID string `json:"key_id"`
		}
		_ = json.Unmarshal(entry, &identity)
		refused := fmt.Errorf("requests file %q: key %q refused", path, identity.KeyID)
		var fields map[string]json.RawMessage
		if decodeJSON(entry, &request, true) != nil || json.Unmarshal(entry, &fields) != nil {
			return refused
		}
		for _, field := range []string{"key_id", "epoch", "curve", "public_key", "owner", "operations"} {
			if len(fields[field]) == 0 || bytes.Equal(bytes.TrimSpace(fields[field]), []byte("null")) {
				return refused
			}
		}
		if seen[request.KeyID] || !validKey(request) {
			return refused
		}
		seen[request.KeyID] = true
		if i, exists := indices[request.KeyID]; exists {
			prior := keys[i]
			if request.Owner != prior.Owner || request.Curve != prior.Curve || request.Epoch < prior.Epoch || (prior.PublicKey != "" && request.PublicKey != prior.PublicKey) {
				return refused
			}
			keys[i] = request
		} else {
			if len(keys) == 65536 {
				return refused
			}
			indices[request.KeyID] = len(keys)
			keys = append(keys, request)
		}
	}
	inventory.Keys = keys
	return nil
}

func encodeInventory(key *ecdsa.PrivateKey, inventory server.WalletInventory) (string, error) {
	header, _ := json.Marshal(struct {
		Alg  string `json:"alg"`
		Type string `json:"typ"`
	}{"ES256", agent.InventoryAudience + "+jwt"})
	payload, err := json.Marshal(inventory)
	if err != nil {
		return "", errors.New("inventory inputs: cannot encode document")
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	if len(input)+1+base64.RawURLEncoding.EncodedLen(64)+1 > maxTokenBytes {
		return "", errors.New("inventory inputs: signed document exceeds size bound")
	}
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", errors.New("inventory signing failed")
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func sameFile(a, b string) bool {
	left, leftErr := filepath.Abs(a)
	right, rightErr := filepath.Abs(b)
	if leftErr == nil && rightErr == nil && left == right {
		return true
	}
	li, le := os.Stat(a)
	ri, re := os.Stat(b)
	return le == nil && re == nil && os.SameFile(li, ri)
}

func writeInventory(path string, raw []byte) error {
	refused := fmt.Errorf("output file %q: cannot write signed inventory", path)
	f, err := os.CreateTemp(filepath.Dir(path), ".inventory-*")
	if err != nil {
		return refused
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return refused
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return refused
	}
	if err = f.Close(); err != nil {
		return refused
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return refused
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return refused
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return refused
	}
	return nil
}

func showInventory(w io.Writer, inventory server.WalletInventory, listKeys bool) error {
	var out strings.Builder
	fmt.Fprintf(&out, "sequence: %s\nissued: %s\nexpires: %s\nmembers: %d\nkeys: %d\n", inventory.Sequence, time.Unix(inventory.IssuedAt, 0).UTC().Format(time.RFC3339), time.Unix(inventory.ExpiresAt, 0).UTC().Format(time.RFC3339), len(inventory.Members), len(inventory.Keys))
	counts := map[string]int{}
	for _, key := range inventory.Keys {
		for _, op := range key.Operations {
			counts[op]++
		}
	}
	for _, op := range operations {
		fmt.Fprintf(&out, "%s: %d\n", op, counts[op])
	}
	if listKeys {
		for _, key := range inventory.Keys {
			fmt.Fprintf(&out, "key %q epoch=%d curve=%s owner=%q operations=%s public_key=%s\n", key.KeyID, key.Epoch, key.Curve, key.Owner, strings.Join(key.Operations, ","), key.PublicKey)
		}
	}
	_, err := io.WriteString(w, out.String())
	return err
}
