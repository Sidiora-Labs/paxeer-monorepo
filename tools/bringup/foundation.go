package bringup

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	coretypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	sequencer "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

type NativeGenesis struct {
	Doc           *tmtypes.GenesisDoc
	Digest        string
	ReceiptPublic []byte
}

// ReadNativeGenesis consumes paxd's JSON genesis and the same hexadecimal
// receipt seed used by --sequencer.receipt-key-file. Module validation uses
// the chain's real bank and sequencer validators; there is no LXGB producer.
func ReadNativeGenesis(raw, receiptSeed []byte) (NativeGenesis, error) {
	if e := uniqueJSON(raw); e != nil {
		return NativeGenesis{}, e
	}
	doc, e := tmtypes.GenesisDocFromJSON(raw)
	if e != nil {
		return NativeGenesis{}, e
	}
	if doc.ChainID != "hyperpax_125-1" || len(doc.Validators) == 0 {
		return NativeGenesis{}, errors.New("merged native validators and chain 125 required")
	}
	var app map[string]json.RawMessage
	if json.Unmarshal(doc.AppState, &app) != nil {
		return NativeGenesis{}, errors.New("native application state required")
	}
	for _, name := range []string{"layerxcustody", "layerxanchor", "guarantor", "receipt-authority"} {
		if _, ok := app[name]; ok {
			return NativeGenesis{}, errors.New("retired genesis module refused")
		}
	}
	for _, name := range []string{"bank", "sequencer", "lxphost"} {
		if len(app[name]) == 0 || bytes.Equal(bytes.TrimSpace(app[name]), []byte("null")) {
			return NativeGenesis{}, errors.New("merged genesis module missing")
		}
	}
	var bank banktypes.GenesisState
	if e = json.Unmarshal(app["bank"], &bank); e != nil {
		return NativeGenesis{}, e
	}
	if e = bank.Validate(); e != nil {
		return NativeGenesis{}, e
	}
	var sequencerFields map[string]json.RawMessage
	if json.Unmarshal(app["sequencer"], &sequencerFields) != nil {
		return NativeGenesis{}, errors.New("sequencer genesis malformed")
	}
	for name := range sequencerFields {
		switch name {
		case "leaders", "receipt_keys", "last_seq", "last_seq_hash", "activation_height":
		default:
			return NativeGenesis{}, errors.New("unknown sequencer initialization field")
		}
	}
	if e = (sequencer.AppModuleBasic{}).ValidateGenesis(nil, nil, app["sequencer"]); e != nil {
		return NativeGenesis{}, e
	}
	var kernel struct {
		LastSequence uint64            `json:"last_sequence"`
		Records      []json.RawMessage `json:"records"`
	}
	decoder := json.NewDecoder(bytes.NewReader(app["lxphost"]))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&kernel); e != nil || kernel.LastSequence != 0 || len(kernel.Records) != 0 {
		return NativeGenesis{}, errors.New("initialization requires the native empty kernel store; import exports through paxd")
	}
	seed, e := hex.DecodeString(strings.TrimSpace(string(receiptSeed)))
	if e != nil || len(seed) != ed25519.SeedSize || bytes.Equal(seed, make([]byte, 32)) {
		return NativeGenesis{}, errors.New("native receipt seed must be nonzero hexadecimal")
	}
	public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	id := sha256.Sum256(public)
	var wire struct {
		Keys []struct {
			ID     []byte `json:"key_id"`
			Public []byte `json:"public_key"`
			First  uint64 `json:"first_height"`
			Last   uint64 `json:"last_height"`
		} `json:"receipt_keys"`
	}
	if json.Unmarshal(app["sequencer"], &wire) != nil {
		return NativeGenesis{}, errors.New("native receipt pins malformed")
	}
	pinned := false
	for _, key := range wire.Keys {
		if bytes.Equal(key.ID, id[:]) && bytes.Equal(key.Public, public) && key.First <= uint64(doc.InitialHeight) && key.Last >= uint64(doc.InitialHeight) {
			pinned = true
		}
	}
	if !pinned {
		return NativeGenesis{}, errors.New("receipt signer is not pinned at initialization height")
	}
	sum := sha256.Sum256(raw)
	return NativeGenesis{Doc: doc, Digest: hex.EncodeToString(sum[:]), ReceiptPublic: public}, nil
}

type FoundationCheckpoint struct {
	Schema    string          `json:"schema"`
	Genesis   string          `json:"genesis"`
	Height    int64           `json:"height"`
	BlockHash string          `json:"block_hash"`
	AppHash   string          `json:"app_hash"`
	Commit    json.RawMessage `json:"commit"`
}
type FoundationState struct {
	root     string
	genesis  NativeGenesis
	retained *FoundationCheckpoint
}

func OpenFoundationState(root string, genesis NativeGenesis) (*FoundationState, error) {
	if genesis.Doc == nil || !fleetSHA.MatchString(genesis.Digest) {
		return nil, errors.New("validated native genesis required")
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	s := &FoundationState{root: root, genesis: genesis}
	raw, e := os.ReadFile(filepath.Join(root, "committed.json"))
	if errors.Is(e, os.ErrNotExist) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	var checkpoint FoundationCheckpoint
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&checkpoint); e != nil {
		return nil, e
	}
	if e = s.validate(checkpoint); e != nil {
		return nil, e
	}
	s.retained = &checkpoint
	return s, nil
}
func (s *FoundationState) Close() error {
	if s.root == "" {
		return errors.New("foundation closed")
	}
	s.root = ""
	return nil
}
func (s *FoundationState) validate(p FoundationCheckpoint) error {
	if p.Schema != "layerx.foundation.committed.v1" || p.Genesis != s.genesis.Digest || p.Height <= 0 || !fleetSHA.MatchString(p.BlockHash) || !fleetSHA.MatchString(p.AppHash) {
		return errors.New("retained foundation binding differs")
	}
	var result coretypes.ResultCommit
	if json.Unmarshal(p.Commit, &result) != nil || !result.CanonicalCommit || result.Header == nil || result.Commit == nil {
		return errors.New("canonical signed native commit required")
	}
	if e := result.SignedHeader.ValidateBasic(s.genesis.Doc.ChainID); e != nil {
		return e
	}
	if result.Header.Height != p.Height || hex.EncodeToString(result.Header.Hash()) != p.BlockHash || hex.EncodeToString(result.Header.AppHash) != p.AppHash || !bytes.Equal(result.Header.ValidatorsHash, s.genesis.Doc.ValidatorHash()) {
		return errors.New("native committed header differs from initialization")
	}
	return s.genesis.Doc.ValidatorSet().VerifyCommitLight(s.genesis.Doc.ChainID, result.Commit.BlockID, p.Height, result.Commit)
}

// Observe reads the actual Comet commit and its same-height EVM block. Only
// verified commits advance the durable restart floor; open heads never do.
func (s *FoundationState) Observe(ctx context.Context, client *http.Client, node, comet string) (FoundationCheckpoint, error) {
	if s.root == "" {
		return FoundationCheckpoint{}, errors.New("foundation closed")
	}
	status, e := callRPC(ctx, client, comet, "status", map[string]any{}, "", false)
	if e != nil {
		return FoundationCheckpoint{}, e
	}
	var wire struct {
		Node struct {
			Network string `json:"network"`
		} `json:"node_info"`
		Sync struct {
			Height   string `json:"latest_block_height"`
			Hash     string `json:"latest_block_hash"`
			Catching *bool  `json:"catching_up"`
		} `json:"sync_info"`
	}
	if json.Unmarshal(status, &wire) != nil || wire.Node.Network != s.genesis.Doc.ChainID || wire.Sync.Catching == nil || *wire.Sync.Catching {
		return FoundationCheckpoint{}, errors.New("native chain is unavailable or catching up")
	}
	height, e := strconv.ParseInt(wire.Sync.Height, 10, 64)
	if e != nil || height <= 0 {
		return FoundationCheckpoint{}, errors.New("committed foundation height required")
	}
	commit, e := callRPC(ctx, client, comet, "commit", map[string]string{"height": wire.Sync.Height}, "", false)
	if e != nil {
		return FoundationCheckpoint{}, e
	}
	var result coretypes.ResultCommit
	if json.Unmarshal(commit, &result) != nil || result.Header == nil {
		return FoundationCheckpoint{}, errors.New("native commit missing")
	}
	checkpoint := FoundationCheckpoint{Schema: "layerx.foundation.committed.v1", Genesis: s.genesis.Digest, Height: height, BlockHash: strings.ToLower(wire.Sync.Hash), AppHash: hex.EncodeToString(result.Header.AppHash), Commit: commit}
	if e = s.validate(checkpoint); e != nil {
		return FoundationCheckpoint{}, e
	}
	chain, e := callRPC(ctx, client, node, "eth_chainId", []any{}, "", false)
	if e != nil || string(chain) != `"0x7d"` {
		return FoundationCheckpoint{}, errors.New("foundation EVM chain differs")
	}
	number := "0x" + strconv.FormatInt(height, 16)
	block, e := callRPC(ctx, client, node, "eth_getBlockByNumber", []any{number, false}, "", false)
	if e != nil {
		return FoundationCheckpoint{}, e
	}
	var b struct{ Number, Hash string }
	if json.Unmarshal(block, &b) != nil || b.Number != number || strings.ToLower(b.Hash) != "0x"+checkpoint.BlockHash {
		return FoundationCheckpoint{}, errors.New("EVM result is not the native committed block")
	}
	if s.retained != nil && (checkpoint.Height < s.retained.Height || checkpoint.Height == s.retained.Height && (checkpoint.BlockHash != s.retained.BlockHash || checkpoint.AppHash != s.retained.AppHash)) {
		return FoundationCheckpoint{}, errors.New("foundation committed state rolled back or diverged")
	}
	raw, e := json.Marshal(checkpoint)
	if e != nil {
		return FoundationCheckpoint{}, e
	}
	if e = edgeAtomic(filepath.Join(s.root, "committed.json"), raw, 0600); e != nil {
		return FoundationCheckpoint{}, e
	}
	s.retained = &checkpoint
	return checkpoint, nil
}

func ReadNativeGenesisFiles(genesis, seed string) (NativeGenesis, error) {
	raw, e := os.ReadFile(genesis)
	if e != nil {
		return NativeGenesis{}, e
	}
	key, e := os.ReadFile(seed)
	if e != nil {
		return NativeGenesis{}, e
	}
	return ReadNativeGenesis(raw, key)
}
func WriteNativeGenesisReport(w io.Writer, g NativeGenesis) error {
	return json.NewEncoder(w).Encode(map[string]any{"schema": "layerx.foundation.genesis.v1", "chain_id": g.Doc.ChainID, "genesis_sha256": g.Digest, "receipt_public_key": hex.EncodeToString(g.ReceiptPublic), "validators": len(g.Doc.Validators), "modules": []string{"bank", "sequencer", "lxphost"}})
}

// QualifyNativeFoundation retains the shared byte/provenance checks while
// binding the v2 handoff to native initialization and a signed committed state.
func QualifyNativeFoundation(directory, revision, binding, consumer string, raw []byte) error {
	if e := qualifyFoundation(directory, revision, binding, consumer, raw, 2); e != nil {
		return e
	}
	var manifest struct {
		ChainID       int                  `json:"chain_id"`
		NetworkID     int                  `json:"network_id"`
		CosmosChainID string               `json:"cosmos_chain_id"`
		Inputs        map[string]string    `json:"inputs"`
		Committed     FoundationCheckpoint `json:"committed"`
	}
	if e := json.Unmarshal(raw, &manifest); e != nil {
		return e
	}
	if manifest.ChainID != 125 || manifest.NetworkID != 125 || manifest.CosmosChainID != "hyperpax_125-1" {
		return errors.New("native foundation chain identity differs")
	}
	required := map[string]string{"native_genesis": "chain/config/genesis.json", "receipt_key": "chain/config/receipt.key", "initialization": "genesis/initialization.json", "committed": "run/committed/committed.json"}
	for key, path := range required {
		if manifest.Inputs[key] != path {
			return errors.New("native foundation input binding missing: " + key)
		}
	}
	genesis, e := ReadNativeGenesisFiles(filepath.Join(directory, required["native_genesis"]), filepath.Join(directory, required["receipt_key"]))
	if e != nil {
		return e
	}
	var report bytes.Buffer
	if e = WriteNativeGenesisReport(&report, genesis); e != nil {
		return e
	}
	actual, e := os.ReadFile(filepath.Join(directory, required["initialization"]))
	if e != nil {
		return e
	}
	var expectedReport, actualReport map[string]any
	if json.Unmarshal(report.Bytes(), &expectedReport) != nil || json.Unmarshal(actual, &actualReport) != nil || !reflect.DeepEqual(expectedReport, actualReport) {
		return errors.New("native initialization report differs")
	}
	state, e := OpenFoundationState(filepath.Join(directory, "run/committed"), genesis)
	if e != nil {
		return e
	}
	defer state.Close()
	if state.retained == nil {
		return errors.New("native foundation committed state missing")
	}
	expected, e := json.Marshal(state.retained)
	if e != nil {
		return e
	}
	observed, e := json.Marshal(manifest.Committed)
	if e != nil {
		return e
	}
	if !bytes.Equal(expected, observed) {
		return errors.New("native foundation retained commit differs")
	}
	return nil
}
