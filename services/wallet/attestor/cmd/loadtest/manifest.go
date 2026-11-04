package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

type RunManifest struct {
	Version   int                   `json:"version"`
	RunID     string                `json:"run_id"`
	Owner     string                `json:"owner"`
	ChainID   uint64                `json:"chain_id"`
	Endpoints []endpoint            `json:"endpoints"`
	Keys      []KeySummary          `json:"keys"`
	Inventory []server.InventoryKey `json:"inventory_requests"`
}

func sameMembers(ids []string, eps []endpoint) bool {
	want := make([]string, len(eps))
	for i, ep := range eps {
		want[i] = ep.ID
	}
	got := append([]string(nil), ids...)
	sort.Strings(want)
	sort.Strings(got)
	return len(got) == len(want) && strings.Join(got, "\x00") == strings.Join(want, "\x00")
}

func prepareManifest(cfg *settings) error {
	if cfg.manifestPath == "" {
		return errors.New("-manifest is required; use -phase export to begin the inventory approval handoff")
	}
	if cfg.phase != "export" && cfg.phase != "generate" && cfg.phase != "sign" {
		return errors.New("-phase must be export, generate, or sign")
	}
	if len(cfg.endpoints) != 5 {
		return errors.New("wallet inventory requires exactly five attestors")
	}
	if cfg.phase == "export" {
		pins := map[string]string{}
		for _, part := range strings.Split(cfg.pins, ",") {
			id, pin, ok := strings.Cut(part, "=")
			raw, err := hex.DecodeString(pin)
			if !ok || id == "" || pins[id] != "" || err != nil || len(raw) != 32 || pin != strings.ToLower(pin) {
				return errors.New("-pins must name each attestor once with a lowercase SHA256 hex pin")
			}
			pins[id] = pin
		}
		if len(pins) != len(cfg.endpoints) {
			return errors.New("-pins must match manifest endpoints")
		}
		seenPins, seenAddrs := map[string]bool{}, map[string]bool{}
		for i := range cfg.endpoints {
			pin := pins[cfg.endpoints[i].ID]
			if pin == "" || seenPins[pin] || seenAddrs[cfg.endpoints[i].Addr] {
				return errors.New("-pins must match distinct manifest endpoints and identities")
			}
			seenPins[pin], seenAddrs[cfg.endpoints[i].Addr] = true, true
			cfg.endpoints[i].Pin = pin
		}
		id, err := randomHex(8)
		if err != nil {
			return err
		}
		cfg.manifest = &RunManifest{Version: 1, RunID: id, Owner: cfg.subject, ChainID: cfg.chainID, Endpoints: cfg.endpoints}
		for _, curve := range []string{store.CurveSecp256k1, store.CurveEd25519} {
			cfg.manifest.Keys = append(cfg.manifest.Keys, KeySummary{Curve: curve, KeyID: "loadtest-" + id + "-" + curve})
		}
		cfg.manifest.inventoryRequests()
		return saveManifest(cfg.manifestPath, cfg.manifest, true)
	}
	if cfg.pins != "" {
		return errors.New("-pins is only accepted during export; resume uses manifest pins")
	}
	f, err := os.Open(cfg.manifestPath)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 1<<20))
	dec.DisallowUnknownFields()
	var m RunManifest
	if err := dec.Decode(&m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("manifest has trailing data")
	}
	if m.Version != 1 || m.RunID == "" || m.Owner != cfg.subject || m.ChainID != cfg.chainID || len(m.Endpoints) != len(cfg.endpoints) || len(m.Keys) != 2 {
		return errors.New("manifest version, owner, chain or inventory shape differs from run settings")
	}
	seenPins, seenAddrs := map[string]bool{}, map[string]bool{}
	for i, ep := range m.Endpoints {
		raw, err := hex.DecodeString(ep.Pin)
		if ep.ID != cfg.endpoints[i].ID || ep.Addr != cfg.endpoints[i].Addr || err != nil || len(raw) != 32 || ep.Pin != strings.ToLower(ep.Pin) || seenPins[ep.Pin] || seenAddrs[ep.Addr] {
			return errors.New("manifest endpoint identity, address or pin differs or is duplicated")
		}
		seenPins[ep.Pin], seenAddrs[ep.Addr] = true, true
	}
	for i, curve := range []string{store.CurveSecp256k1, store.CurveEd25519} {
		key := m.Keys[i]
		if key.Curve != curve || key.KeyID == "" || key.KeyID == m.Keys[1-i].KeyID {
			return errors.New("manifest must name distinct secp256k1 and ed25519 keys")
		}
		if cfg.phase == "generate" && key.KeyID != "loadtest-"+m.RunID+"-"+curve {
			return errors.New("generation key differs from exported run identity")
		}
		if key.PublicKey != "" {
			if err := validateManifestKey(key); err != nil {
				return err
			}
		} else if cfg.phase == "sign" {
			return errors.New("manifest keys have not been generated; run -phase generate, then approve signing inventory")
		}
	}
	cfg.manifest, cfg.endpoints = &m, m.Endpoints
	return nil
}

func validateManifestKey(k KeySummary) error {
	raw, err := hex.DecodeString(k.PublicKey)
	if err != nil || k.PublicKey != strings.ToLower(k.PublicKey) {
		return errors.New("invalid manifest public key")
	}
	if k.Curve == store.CurveSecp256k1 {
		pub, err := gethcrypto.UnmarshalPubkey(raw)
		if err != nil || !common.IsHexAddress(k.Address) || gethcrypto.PubkeyToAddress(*pub) != common.HexToAddress(k.Address) {
			return errors.New("manifest EVM public key and address differ")
		}
	} else {
		if len(raw) != ed25519.PublicKeySize {
			return errors.New("invalid manifest ed25519 public key")
		}
		var pub [32]byte
		copy(pub[:], raw)
		if k.DID != lxwire.DIDFromKey(pub) {
			return errors.New("manifest ed25519 public key and DID differ")
		}
	}
	return nil
}

func (m *RunManifest) inventoryRequests() {
	m.Inventory = nil
	for _, key := range m.Keys {
		op := "generate"
		if key.PublicKey != "" {
			op = "sign"
		}
		m.Inventory = append(m.Inventory, server.InventoryKey{KeyID: key.KeyID, Curve: key.Curve, PublicKey: key.PublicKey, Owner: m.Owner, Operations: []string{op}})
	}
}

func saveManifest(path string, m *RunManifest, create bool) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if create {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("export manifest: %w", err)
		}
		_, writeErr := f.Write(raw)
		syncErr := f.Sync()
		closeErr := f.Close()
		return errors.Join(writeErr, syncErr, closeErr)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".loadtest-manifest-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func exportManifest(stdout, stderr io.Writer, m *RunManifest) error {
	if err := json.NewEncoder(stdout).Encode(m); err != nil {
		return err
	}
	_, err := fmt.Fprintln(stderr, "Paused: have the inventory owner review, sign and publish inventory_requests to the pinned attestors before resuming with -phase generate or -phase sign. This manifest grants no authority.")
	return err
}

func (lt *loadTester) generateManifest(ctx context.Context) error {
	m := lt.cfg.manifest
	for i := range m.Keys {
		if m.Keys[i].PublicKey != "" {
			continue
		}
		account := ""
		if i == 1 {
			account = m.Keys[0].Address
		}
		kr, took, err := lt.generate(ctx, m.Keys[i].Curve, account)
		if err != nil {
			return err
		}
		key := KeySummary{Curve: kr.Curve, KeyID: kr.KeyID, PublicKey: kr.PublicKey, Address: kr.Address, DID: kr.DID, GenerateMillis: millis(took)}
		if err := validateManifestKey(key); err != nil {
			return err
		}
		m.Keys[i] = key
		m.inventoryRequests()
		if err := saveManifest(lt.cfg.manifestPath, m, false); err != nil {
			return err
		}
	}
	return nil
}

func (lt *loadTester) useManifestKeys() error {
	keys := lt.cfg.manifest.Keys
	for _, key := range keys {
		if err := validateManifestKey(key); err != nil {
			return err
		}
	}
	lt.evmKey, lt.evmAddress = keys[0].KeyID, common.HexToAddress(keys[0].Address)
	lt.edKey = keys[1].KeyID
	pub, _ := hex.DecodeString(keys[1].PublicKey)
	copy(lt.edPublic[:], pub)
	return nil
}
