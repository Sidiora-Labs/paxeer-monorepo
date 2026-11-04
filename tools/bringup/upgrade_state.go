package bringup

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type UpgradeUnit struct {
	ID          string `json:"id"`
	Destination string `json:"destination"`
	Unit        string `json:"unit"`
	Binary      string `json:"binary"`
	Node        string `json:"node"`
	Comet       string `json:"comet"`
}
type UpgradePlan struct {
	Version         string        `json:"version"`
	Name            string        `json:"name"`
	SHA256          string        `json:"sha256"`
	Height          uint64        `json:"height"`
	RecoverySeconds int64         `json:"recovery_seconds"`
	Units           []UpgradeUnit `json:"units"`
}
type UpgradeObservation struct {
	Unit       string `json:"unit"`
	Active     string `json:"active"`
	PID        int    `json:"pid"`
	StartTicks uint64 `json:"start_ticks"`
	RunningSHA string `json:"running_sha256"`
	DiskSHA    string `json:"disk_sha256"`
	Height     uint64 `json:"height"`
	Ready      bool   `json:"ready"`
	Halt       bool   `json:"halt"`
	Problem    string `json:"problem"`
}
type UpgradeEvidence struct {
	At          int64              `json:"at"`
	Reason      string             `json:"reason"`
	Observation UpgradeObservation `json:"observation"`
}
type UpgradeProgress struct {
	Phase    string             `json:"phase"`
	Baseline uint64             `json:"baseline"`
	Deadline int64              `json:"deadline"`
	Last     UpgradeObservation `json:"last"`
	Failures []UpgradeEvidence  `json:"failures"`
}
type upgradeDocument struct {
	Schema   int                         `json:"schema"`
	Plan     UpgradePlan                 `json:"plan"`
	Progress map[string]*UpgradeProgress `json:"progress"`
}
type UpgradeState struct {
	Path     string
	document upgradeDocument
}

var upgradeName = regexp.MustCompile(`^[A-Za-z0-9@_.-]+$`)
var upgradeDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func upgradeRoot(endpoint string) error {
	if e := protectedURL(endpoint); e != nil {
		return e
	}
	// Both the node and consensus RPC contracts are root transports.
	parsed, e := http.NewRequest(http.MethodPost, endpoint, nil)
	if e != nil || (parsed.URL.Path != "" && parsed.URL.Path != "/") {
		return errors.New("upgrade RPC must use the node or consensus root")
	}
	return nil
}
func (p UpgradePlan) validate() error {
	if !upgradeName.MatchString(p.Version) || !upgradeName.MatchString(p.Name) || !upgradeDigest.MatchString(p.SHA256) || p.Height == 0 || p.RecoverySeconds < 1 || p.RecoverySeconds > 3600 || len(p.Units) == 0 {
		return errors.New("invalid upgrade plan")
	}
	seen := map[string]bool{}
	identities := map[string]bool{}
	endpoints := map[string]bool{}
	for _, u := range p.Units {
		identity := u.Destination + "\x00" + u.Unit
		if endpoints[u.Destination+"\x00"+u.Node] || endpoints[u.Destination+"\x00"+u.Comet] || !upgradeName.MatchString(u.ID) || seen[u.ID] || identities[identity] || !upgradeName.MatchString(u.Unit) || u.Destination == "" || strings.HasPrefix(u.Destination, "-") || strings.ContainsAny(u.Destination, " \n\r\t'\";`$\\") || !filepath.IsAbs(u.Binary) || !regexp.MustCompile(`^/[A-Za-z0-9_./-]+$`).MatchString(u.Binary) || filepath.Clean(u.Binary) != u.Binary || upgradeRoot(u.Node) != nil || upgradeRoot(u.Comet) != nil {
			return errors.New("invalid or duplicate upgrade unit")
		}
		seen[u.ID] = true
		identities[identity] = true
		endpoints[u.Destination+"\x00"+u.Node] = true
		endpoints[u.Destination+"\x00"+u.Comet] = true
	}
	return nil
}
func OpenUpgradeState(path string) (*UpgradeState, error) {
	path, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	s := &UpgradeState{Path: path}
	raw, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("upgrade journal must be a protected regular file")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if uniqueJSON(raw) != nil || decoder.Decode(&s.document) != nil || s.document.Schema != 1 || s.document.Plan.validate() != nil || len(s.document.Progress) != len(s.document.Plan.Units) {
		return nil, errors.New("invalid upgrade journal")
	}
	for _, u := range s.document.Plan.Units {
		r := s.document.Progress[u.ID]
		if r == nil {
			return nil, errors.New("missing per-unit upgrade progress")
		}
		switch r.Phase {
		case "waiting", "installing", "restarting", "recovering", "checking", "current", "failed":
		default:
			return nil, errors.New("invalid upgrade phase")
		}
		if r.Baseline < s.document.Plan.Height || r.Deadline < 0 || (r.Phase == "failed" && len(r.Failures) == 0) {
			return nil, errors.New("invalid upgrade recovery evidence")
		}
	}
	return s, nil
}
func (s *UpgradeState) Close() { s.Path = "" }
func (s *UpgradeState) save() error {
	if s.Path == "" {
		return errors.New("upgrade journal closed")
	}
	raw, e := json.Marshal(s.document)
	if e != nil {
		return e
	}
	return edgeAtomic(s.Path, raw, 0600)
}
func (s *UpgradeState) Begin(plan UpgradePlan) error {
	if s.Path == "" {
		return errors.New("upgrade journal closed")
	}
	if e := plan.validate(); e != nil {
		return e
	}
	if s.document.Schema != 0 {
		a, _ := json.Marshal(plan)
		b, _ := json.Marshal(s.document.Plan)
		if !bytes.Equal(a, b) {
			return errors.New("upgrade journal belongs to a different plan or inventory")
		}
	} else {
		s.document = upgradeDocument{Schema: 1, Plan: plan, Progress: map[string]*UpgradeProgress{}}
		for _, u := range plan.Units {
			s.document.Progress[u.ID] = &UpgradeProgress{Phase: "waiting", Baseline: plan.Height}
		}
	}
	for _, r := range s.document.Progress {
		if r.Phase == "current" {
			r.Phase = "checking"
			r.Deadline = 0
		} else if r.Phase == "recovering" {
			r.Phase = "restarting"
		}
	}
	return s.save()
}
func (s *UpgradeState) Failure(id, reason string, now time.Time) error {
	r := s.document.Progress[id]
	if r == nil || reason == "" {
		return errors.New("unknown upgrade unit or failure")
	}
	r.Phase = "failed"
	r.Failures = append(r.Failures, UpgradeEvidence{now.Unix(), reason, r.Last})
	return s.save()
}
func (s *UpgradeState) Applied(id, action string) error {
	r := s.document.Progress[id]
	if r == nil {
		return errors.New("unknown upgrade unit")
	}
	if action == "swap" && r.Phase == "installing" {
		r.Phase = "restarting"
	} else if action == "restart" && r.Phase == "restarting" {
		r.Phase = "recovering"
	} else {
		return errors.New("unexpected upgrade acknowledgement")
	}
	return s.save()
}

// Next persists intent before a mutation and accepts success only from the
// observed running executable and advancing canonical committed height.
func (s *UpgradeState) Next(id string, o UpgradeObservation, now time.Time) (string, error) {
	if s.Path == "" || s.document.Schema != 1 {
		return "", errors.New("upgrade plan is not open")
	}
	r := s.document.Progress[id]
	if r == nil {
		return "", errors.New("unknown upgrade unit")
	}
	switch o.Active {
	case "active", "inactive", "failed", "activating", "deactivating", "reloading":
	default:
		return "", errors.New("unknown observed unit state")
	}
	var unit UpgradeUnit
	for _, u := range s.document.Plan.Units {
		if u.ID == id {
			unit = u
		}
	}
	if o.Unit != unit.Unit || !upgradeDigest.MatchString(o.DiskSHA) || (o.RunningSHA != "" && !upgradeDigest.MatchString(o.RunningSHA)) || o.PID < 0 || (o.Ready && (o.Height == 0 || o.Active != "active" || o.PID == 0 || o.StartTicks == 0 || o.RunningSHA == "")) {
		return "", errors.New("malformed upgrade observation")
	}
	for other, p := range s.document.Progress {
		if other == id || p.Phase == "waiting" || p.Phase == "current" {
			continue
		}
		if p.Phase == "checking" && r.Phase != "waiting" {
			continue
		}
		if p.Phase == "failed" && r.Phase == "failed" {
			continue
		}
		return "blocked", nil
	}
	r.Last = o
	running := o.Active == "active" && o.PID > 0 && o.StartTicks > 0 && o.RunningSHA == s.document.Plan.SHA256 && o.DiskSHA == s.document.Plan.SHA256
	recovered := running && o.Ready && o.Height > r.Baseline
	if r.Phase == "failed" {
		if !recovered {
			return "failed", s.save()
		}
		r.Phase = "current"
		return "current", s.save()
	}
	if r.Phase == "current" && recovered {
		return "current", s.save()
	}
	if r.Phase == "waiting" || r.Phase == "checking" || r.Phase == "current" {
		if running {
			r.Phase = "recovering"
			r.Deadline = now.Unix() + s.document.Plan.RecoverySeconds
			if o.Height > r.Baseline {
				r.Baseline = o.Height
			}
			return "recover", s.save()
		}
		if !o.Halt && r.Phase == "waiting" {
			return "wait", s.save()
		}
		r.Deadline = now.Unix() + s.document.Plan.RecoverySeconds
		if o.Height > r.Baseline {
			r.Baseline = o.Height
		}
		if o.DiskSHA == s.document.Plan.SHA256 {
			r.Phase = "restarting"
		} else {
			r.Phase = "installing"
		}
	}
	if r.Deadline == 0 {
		r.Deadline = now.Unix() + s.document.Plan.RecoverySeconds
	}
	if now.Unix() >= r.Deadline {
		if e := s.Failure(id, "recovery deadline exceeded", now); e != nil {
			return "", e
		}
		return "failed", nil
	}
	if recovered {
		r.Phase = "current"
		return "current", s.save()
	}
	if running {
		// A crash after restart and before its acknowledgement must never cause
		// another restart of the already activated target generation.
		r.Phase = "recovering"
		return "recover", s.save()
	}
	switch r.Phase {
	case "installing":
		if o.DiskSHA != s.document.Plan.SHA256 {
			return "swap", s.save()
		}
		r.Phase = "restarting"
	case "recovering":
		return "recover", s.save()
	}
	return "restart", s.save()
}

func upgradeFileSHA(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func upgradeStart(pid int) (uint64, error) {
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e != nil {
		return 0, e
	}
	end := strings.LastIndexByte(string(b), ')')
	if end < 0 {
		return 0, errors.New("process stat unavailable")
	}
	fields := strings.Fields(string(b[end+1:]))
	if len(fields) < 20 || fields[0] == "Z" {
		return 0, errors.New("process stopped")
	}
	return strconv.ParseUint(fields[19], 10, 64)
}

// ObserveUpgrade consumes actual systemd show/journal output, reads the live
// /proc executable (including an unlinked old inode), and probes root RPCs.
func ObserveUpgrade(ctx context.Context, client *http.Client, unit, binary, node, comet, name string, height uint64, input io.Reader) (UpgradeObservation, error) {
	o := UpgradeObservation{Unit: unit}
	scanner := bufio.NewScanner(io.LimitReader(input, 4<<20))
	journal := false
	seen := map[string]bool{}
	halt := regexp.MustCompile(`UPGRADE "` + regexp.QuoteMeta(name) + `" NEEDED at height:? ?` + strconv.FormatUint(height, 10) + `([^0-9]|$)`)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "@@journal" {
			journal = true
			continue
		}
		if journal {
			o.Halt = o.Halt || halt.MatchString(line)
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || seen[k] {
			return o, errors.New("invalid systemd observation")
		}
		seen[k] = true
		switch k {
		case "ActiveState":
			o.Active = v
		case "MainPID":
			pid, e := strconv.Atoi(v)
			if e != nil || pid < 0 {
				return o, errors.New("invalid main PID")
			}
			o.PID = pid
		default:
			return o, errors.New("unknown systemd observation field")
		}
	}
	if scanner.Err() != nil || !seen["ActiveState"] || !seen["MainPID"] || !journal {
		return o, errors.New("incomplete unit observation")
	}
	switch o.Active {
	case "active", "inactive", "failed", "activating", "deactivating", "reloading":
	default:
		return o, errors.New("unknown unit active state")
	}
	var e error
	o.DiskSHA, e = upgradeFileSHA(binary)
	if e != nil {
		return o, e
	}
	if o.PID > 0 {
		o.StartTicks, e = upgradeStart(o.PID)
		if e == nil {
			o.RunningSHA, e = upgradeFileSHA(fmt.Sprintf("/proc/%d/exe", o.PID))
		}
		if e != nil {
			o.Problem = "process unavailable"
			return o, nil
		}
	}
	if o.Active != "active" || o.PID == 0 {
		o.Problem = "unit is not active"
		return o, nil
	}
	if upgradeRoot(node) != nil || upgradeRoot(comet) != nil {
		return o, errors.New("node and consensus roots required")
	}
	o.Height, e = upgradeCommittedHeight(ctx, client, node, comet)
	if e != nil {
		o.Problem = "committed readiness unavailable"
		return o, nil
	}
	ticks, e := upgradeStart(o.PID)
	if e != nil || ticks != o.StartTicks {
		o.Problem = "process changed during readiness"
		return o, nil
	}
	running, e := upgradeFileSHA(fmt.Sprintf("/proc/%d/exe", o.PID))
	disk, diskErr := upgradeFileSHA(binary)
	if e != nil || diskErr != nil || running != o.RunningSHA || disk != o.DiskSHA {
		o.Problem = "executable changed during readiness"
		return o, nil
	}
	o.Ready = true
	return o, nil
}
func upgradeCommittedHeight(ctx context.Context, client *http.Client, node, comet string) (uint64, error) {
	invalid := errors.New("node lacks canonical committed readiness")
	chain, e := callRPC(ctx, client, node, "eth_chainId", []any{}, "", false)
	if e != nil || string(chain) != `"0x7d"` {
		return 0, invalid
	}
	raw, e := callRPC(ctx, client, node, "eth_blockNumber", []any{}, "", false)
	if e != nil {
		return 0, e
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return 0, invalid
	}
	head, e := quantity(text)
	if e != nil || !head.IsUint64() || head.Sign() == 0 {
		return 0, invalid
	}
	raw, e = callRPC(ctx, client, comet, "status", map[string]any{}, "", false)
	if e != nil {
		return 0, e
	}
	var status struct {
		Node struct {
			Network string `json:"network"`
		} `json:"node_info"`
		Sync struct {
			Height   string `json:"latest_block_height"`
			Catching *bool  `json:"catching_up"`
		} `json:"sync_info"`
	}
	if json.Unmarshal(raw, &status) != nil || status.Sync.Catching == nil || *status.Sync.Catching || status.Node.Network == "" {
		return 0, invalid
	}
	latest, e := strconv.ParseUint(status.Sync.Height, 10, 64)
	if e != nil || latest < head.Uint64() {
		return 0, invalid
	}
	raw, e = callRPC(ctx, client, comet, "commit", map[string]string{"height": head.String()}, "", false)
	if e != nil {
		return 0, e
	}
	var commit struct {
		Canonical bool `json:"canonical"`
		Signed    struct {
			Header struct {
				Height  string `json:"height"`
				Chain   string `json:"chain_id"`
				AppHash string `json:"app_hash"`
			} `json:"header"`
			Commit struct {
				Height string `json:"height"`
				Block  struct {
					Hash string `json:"hash"`
				} `json:"block_id"`
				Signatures []json.RawMessage `json:"signatures"`
			} `json:"commit"`
		} `json:"signed_header"`
	}
	if json.Unmarshal(raw, &commit) != nil || !commit.Canonical || commit.Signed.Header.Height != head.String() || commit.Signed.Commit.Height != head.String() || commit.Signed.Header.Chain != status.Node.Network || commit.Signed.Header.AppHash == "" || commit.Signed.Commit.Block.Hash == "" || len(commit.Signed.Commit.Signatures) == 0 {
		return 0, invalid
	}
	return head.Uint64(), nil
}
