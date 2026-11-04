package bringup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type FleetLimits struct{ Connect, Transfer, Recovery time.Duration }

func (l FleetLimits) validate() error {
	if l.Connect <= 0 || l.Transfer <= 0 || l.Recovery <= 0 || l.Connect > l.Transfer || l.Transfer > 24*time.Hour || l.Recovery > 10*time.Minute {
		return errors.New("invalid fleet deadlines")
	}
	return nil
}

// FleetSSH bounds dialing, handshake, remote execution and streamed IO. Closing
// the session alone is insufficient: the connection deadline also bounds a
// peer which stops reading, and timeout bounds descendants on the remote host.
type FleetSSH struct {
	Limits FleetLimits
	Config func(string) (*ssh.ClientConfig, error)
}

func fleetQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

func (s FleetSSH) Run(ctx context.Context, dest, script string, input io.Reader, output io.Writer) error {
	if e := s.Limits.validate(); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, s.Limits.Transfer)
	defer cancel()
	config, e := s.Config(dest)
	if e != nil {
		return e
	}
	address := dest
	if _, host, ok := strings.Cut(address, "@"); ok {
		address = host
	}
	if _, _, e = net.SplitHostPort(address); e != nil {
		address = net.JoinHostPort(address, "22")
	}
	dialer := net.Dialer{Timeout: s.Limits.Connect}
	conn, e := dialer.DialContext(ctx, "tcp", address)
	if e != nil {
		return errors.New("fleet SSH connection failed; reconcile pending operation")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	handshake := time.Now().Add(s.Limits.Connect)
	if deadline.Before(handshake) {
		handshake = deadline
	}
	if e = conn.SetDeadline(handshake); e != nil {
		return e
	}
	c, chans, requests, e := ssh.NewClientConn(conn, address, config)
	if e != nil {
		return errors.New("fleet SSH handshake failed; reconcile pending operation")
	}
	client := ssh.NewClient(c, chans, requests)
	defer client.Close()
	if e = conn.SetDeadline(deadline); e != nil {
		return e
	}
	session, e := client.NewSession()
	if e != nil {
		return e
	}
	defer session.Close()
	stop()
	session.Stdin = input
	session.Stdout = output
	stopSession := context.AfterFunc(ctx, func() {
		_ = conn.SetWriteDeadline(time.Now().Add(s.Limits.Connect))
		_ = session.Signal(ssh.SIGTERM)
		_ = conn.Close()
	})
	defer stopSession()
	seconds := strconv.FormatFloat(s.Limits.Transfer.Seconds(), 'f', 3, 64)
	e = session.Run("timeout --signal=TERM --kill-after=5s " + seconds + "s sh -c " + fleetQuote(script))
	if ctx.Err() != nil {
		return fmt.Errorf("fleet transfer interrupted: %w; reopen the same state directory to reconcile", ctx.Err())
	}
	if e != nil {
		return errors.New("fleet remote operation failed; reopen the same state directory to reconcile")
	}
	return nil
}

// OperatorFleetSSH uses the operator's existing agent/keys and known_hosts.
// It never accepts an unknown host or changes trust on a failed connection.
func OperatorFleetSSH(limits FleetLimits) (FleetSSH, func(), error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return FleetSSH{}, nil, e
	}
	verify, e := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if e != nil {
		return FleetSSH{}, nil, e
	}
	var auth []ssh.AuthMethod
	var agentConn net.Conn
	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		agentConn, e = net.DialTimeout("unix", socket, limits.Connect)
		if e != nil {
			return FleetSSH{}, nil, e
		}
		auth = append(auth, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
	}
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		raw, e := os.ReadFile(filepath.Join(home, ".ssh", name))
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return FleetSSH{}, nil, e
		}
		key, e := ssh.ParsePrivateKey(raw)
		if e != nil {
			if agentConn != nil {
				continue
			}
			return FleetSSH{}, nil, e
		}
		auth = append(auth, ssh.PublicKeys(key))
	}
	closeFn := func() {
		if agentConn != nil {
			_ = agentConn.Close()
		}
	}
	if len(auth) == 0 {
		closeFn()
		return FleetSSH{}, nil, errors.New("operator SSH identity required")
	}
	return FleetSSH{Limits: limits, Config: func(dest string) (*ssh.ClientConfig, error) {
		user := os.Getenv("USER")
		if before, _, ok := strings.Cut(dest, "@"); ok {
			user = before
		}
		if user == "" {
			return nil, errors.New("SSH user required")
		}
		return &ssh.ClientConfig{User: user, Auth: auth, HostKeyCallback: verify}, nil
	}}, closeFn, nil
}

type FleetResyncPlan struct{ Source, Target, Home, Binary, Asset, SHA256, ID string }
type fleetPending struct {
	Plan                                           FleetResyncPlan
	Phase                                          string
	SourceState, SourceSHA, TargetState, TargetSHA string
}
type FleetOwner struct {
	root   string
	ssh    FleetSSH
	lock   *os.File
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	closed bool
}

func OpenFleetOwner(root string, transport FleetSSH) (*FleetOwner, error) {
	if e := transport.Limits.validate(); e != nil {
		return nil, e
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(root)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return nil, errors.New("fleet state requires owned private directory")
	}
	lock, e := os.OpenFile(filepath.Join(root, "owner.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		lock.Close()
		return nil, errors.New("fleet owner already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &FleetOwner{root: root, ssh: transport, lock: lock, ctx: ctx, cancel: cancel}, nil
}

// Close stops this owner and its outstanding transports. Recovery is durable
// and deliberately belongs to the next owner, including after abrupt death.
func (o *FleetOwner) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return errors.New("fleet owner closed")
	}
	o.closed = true
	o.cancel()
	_ = syscall.Flock(int(o.lock.Fd()), syscall.LOCK_UN)
	return o.lock.Close()
}
func (o *FleetOwner) run(ctx context.Context, dest, script string, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(o.ctx, cancel)
	defer stop()
	return o.ssh.Run(ctx, dest, script, in, out)
}
func fleetScript(op string, p FleetResyncPlan, body string) string {
	// The first line binds an operation to its paths for retained diagnostics.
	raw, _ := json.Marshal(struct {
		Operation string
		Plan      FleetResyncPlan
	}{op, p})
	return "# bringup-fleet " + string(raw) + "\nset -eu\nexec flock -x " + fleetQuote(p.Home+"/.bringup-fleet.lock") + " sh -c " + fleetQuote(body)
}
func (o *FleetOwner) action(ctx context.Context, dest, op string, p FleetResyncPlan, body string) (string, error) {
	var out bytes.Buffer
	e := o.run(ctx, dest, fleetScript(op, p, body), nil, &out)
	return strings.TrimSpace(out.String()), e
}
func (o *FleetOwner) save(p fleetPending) error {
	b, e := json.Marshal(p)
	if e != nil {
		return e
	}
	return edgeAtomic(filepath.Join(o.root, "pending.json"), b, 0600)
}
func (o *FleetOwner) pending() (*fleetPending, error) {
	b, e := os.ReadFile(filepath.Join(o.root, "pending.json"))
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var p fleetPending
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&p); e != nil {
		return nil, e
	}
	if e = validateFleetPlan(p.Plan); e != nil {
		return nil, e
	}
	if p.Phase != "copying" && p.Phase != "copied" && p.Phase != "swapping" && p.Phase != "committed" {
		return nil, errors.New("invalid pending fleet phase")
	}
	if !fleetSHA.MatchString(p.SourceSHA) || !fleetSHA.MatchString(p.TargetSHA) || (p.SourceState != "active" && p.SourceState != "inactive") || (p.TargetState != "active" && p.TargetState != "inactive") {
		return nil, errors.New("invalid pending fleet state")
	}
	return &p, nil
}

var fleetSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validateFleetPlan(p FleetResyncPlan) error {
	if p.Source == "" || p.Target == "" || p.Source == p.Target || !safeIdentity(p.ID) || !fleetSHA.MatchString(p.SHA256) {
		return errors.New("invalid resync identity")
	}
	for _, s := range []string{p.Source, p.Target} {
		if strings.ContainsAny(s, "\n\r\t ") || strings.HasPrefix(s, "-") {
			return errors.New("invalid resync destination")
		}
	}
	for _, s := range []string{p.Home, p.Binary, p.Asset} {
		if !filepath.IsAbs(s) || filepath.Clean(s) != s || s == "/" {
			return errors.New("absolute normalized resync paths required")
		}
	}
	return nil
}
func (o *FleetOwner) state(ctx context.Context, dest string, p FleetResyncPlan) (string, string, error) {
	text, e := o.action(ctx, dest, "state", p, "state=$(systemctl show paxd.service -p ActiveState --value); printf '%s\\n' \"$state\"; sha256sum "+fleetQuote(p.Binary)+" | cut -d' ' -f1; if [ \"$state\" = active ]; then pid=$(systemctl show paxd.service -p MainPID --value); test \"$pid\" -gt 0; sha256sum \"/proc/$pid/exe\" | cut -d' ' -f1; else printf '%s\\n' -; fi")
	f := strings.Fields(text)
	if e != nil {
		return "", "", e
	}
	if len(f) != 3 || (f[0] != "active" && f[0] != "inactive") || !fleetSHA.MatchString(f[1]) || (f[0] == "active" && f[2] != f[1]) || (f[0] == "inactive" && f[2] != "-") {
		return "", "", errors.New("fleet unit is failed, transitioning or not running its installed executable")
	}
	return f[0], f[1], nil
}
func (o *FleetOwner) unit(ctx context.Context, dest string, p FleetResyncPlan, verb string) error {
	_, e := o.action(ctx, dest, verb, p, "systemctl "+verb+" paxd.service")
	return e
}
func (o *FleetOwner) prepare(ctx context.Context, p FleetResyncPlan) (*fleetPending, error) {
	if e := validateFleetPlan(p); e != nil {
		return nil, e
	}
	if e := o.Recover(ctx); e != nil {
		return nil, e
	}
	raw, e := os.ReadFile(p.Asset)
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != p.SHA256 {
		return nil, errors.New("local resync asset digest differs")
	}
	ss, sh, e := o.state(ctx, p.Source, p)
	if e != nil {
		return nil, e
	}
	ts, th, e := o.state(ctx, p.Target, p)
	if e != nil {
		return nil, e
	}
	pending := &fleetPending{Plan: p, Phase: "copying", SourceState: ss, SourceSHA: sh, TargetState: ts, TargetSHA: th}
	if e = o.save(*pending); e != nil {
		return nil, e
	}
	if ss == "active" {
		e = o.unit(ctx, p.Source, p, "stop")
	}
	return pending, e
}
func (o *FleetOwner) copyData(ctx context.Context, p *fleetPending) error {
	plan := p.Plan
	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	copied := make(chan error, 1)
	go func() {
		e := o.run(ctx, plan.Source, fleetScript("copy-source", plan, "tar -C "+fleetQuote(plan.Home+"/data")+" --exclude=priv_validator_state.json --exclude=snapshots -cf - ."), nil, writer)
		_ = writer.CloseWithError(e)
		if e != nil {
			cancel()
		}
		copied <- e
	}()
	incoming := plan.Home + "/data.incoming-" + plan.ID
	e := o.run(ctx, plan.Target, fleetScript("copy-target", plan, "mkdir -p "+fleetQuote(incoming)+"; tar -C "+fleetQuote(incoming)+" -xf -"), reader, io.Discard)
	_ = reader.CloseWithError(e)
	if e != nil {
		cancel()
	}
	sourceError := <-copied
	if e != nil {
		return e
	}
	if sourceError != nil {
		return sourceError
	}
	if e = o.restoreSource(ctx, *p); e != nil {
		return e
	}
	p.Phase = "copied"
	return o.save(*p)
}
func (o *FleetOwner) restoreSource(ctx context.Context, p fleetPending) error {
	if p.SourceState == "active" {
		if e := o.unit(ctx, p.Plan.Source, p.Plan, "start"); e != nil {
			return e
		}
	}
	state, sha, e := o.state(ctx, p.Plan.Source, p.Plan)
	if e != nil {
		return e
	}
	if state != p.SourceState || sha != p.SourceSHA {
		return errors.New("source recovery did not restore its original running version")
	}
	return nil
}
func (o *FleetOwner) swap(ctx context.Context, p *fleetPending) error {
	plan := p.Plan
	asset, e := os.Open(plan.Asset)
	if e != nil {
		return e
	}
	defer asset.Close()
	staged := plan.Binary + ".new-" + plan.ID
	if e = o.run(ctx, plan.Target, fleetScript("stage", plan, "cat > "+fleetQuote(staged)+"; test \"$(sha256sum "+fleetQuote(staged)+" | cut -d' ' -f1)\" = "+fleetQuote(plan.SHA256)+"; chmod 0755 "+fleetQuote(staged)), asset, io.Discard); e != nil {
		return e
	}
	p.Phase = "swapping"
	if e = o.save(*p); e != nil {
		return e
	}
	incoming, stale := plan.Home+"/data.incoming-"+plan.ID, plan.Home+"/data.stale-"+plan.ID
	body := "test -d " + fleetQuote(incoming+"/tendermint/blockstore.db") + "; test -d " + fleetQuote(incoming+"/tendermint/state.db") + "; test -d " + fleetQuote(incoming+"/state_store") + "; systemctl stop paxd.service; if [ -f " + fleetQuote(plan.Home+"/data/priv_validator_state.json") + " ]; then cp -p " + fleetQuote(plan.Home+"/data/priv_validator_state.json") + " " + fleetQuote(incoming) + "/; fi; cp -p " + fleetQuote(plan.Binary) + " " + fleetQuote(plan.Binary+".pre-"+plan.ID) + "; mv " + fleetQuote(plan.Home+"/data") + " " + fleetQuote(stale) + "; mv " + fleetQuote(incoming) + " " + fleetQuote(plan.Home+"/data") + "; mv -f " + fleetQuote(staged) + " " + fleetQuote(plan.Binary)
	if _, e = o.action(ctx, plan.Target, "swap", plan, body); e != nil {
		return e
	}
	if p.TargetState == "active" {
		if e = o.unit(ctx, plan.Target, plan, "start"); e != nil {
			return e
		}
	}
	state, sha, e := o.state(ctx, plan.Target, plan)
	if e != nil {
		return e
	}
	if state != p.TargetState || sha != plan.SHA256 {
		return errors.New("target running version differs after swap")
	}
	p.Phase = "committed"
	if e = o.save(*p); e != nil {
		return e
	}
	return o.clear()
}
func (o *FleetOwner) clear() error {
	if e := os.Remove(filepath.Join(o.root, "pending.json")); e != nil {
		return e
	}
	return edgeSyncDir(o.root)
}
func (o *FleetOwner) Recover(ctx context.Context) error {
	p, e := o.pending()
	if e != nil || p == nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, o.ssh.Limits.Recovery)
	defer cancel()
	if e = o.restoreSource(ctx, *p); e != nil {
		return fmt.Errorf("resync source recovery pending: %w", e)
	}
	if p.Phase == "swapping" {
		plan := p.Plan
		stale := plan.Home + "/data.stale-" + plan.ID
		data := plan.Home + "/data"
		backup := plan.Binary + ".pre-" + plan.ID
		body := "systemctl stop paxd.service; if [ -d " + fleetQuote(stale) + " ]; then if [ -e " + fleetQuote(data) + " ]; then mv " + fleetQuote(data) + " " + fleetQuote(plan.Home+"/data.failed-"+plan.ID) + "; fi; mv " + fleetQuote(stale) + " " + fleetQuote(data) + "; fi; if [ -f " + fleetQuote(backup) + " ]; then cp -p " + fleetQuote(backup) + " " + fleetQuote(plan.Binary) + "; fi"
		if _, e = o.action(ctx, plan.Target, "rollback", plan, body); e != nil {
			return e
		}
		if p.TargetState == "active" {
			if e = o.unit(ctx, plan.Target, plan, "start"); e != nil {
				return e
			}
		}
		state, sha, e := o.state(ctx, plan.Target, plan)
		if e != nil {
			return e
		}
		if state != p.TargetState || sha != p.TargetSHA {
			return errors.New("target rollback running version differs")
		}
	}
	if p.Phase == "committed" {
		state, sha, e := o.state(ctx, p.Plan.Target, p.Plan)
		if e != nil {
			return e
		}
		if state != p.TargetState || sha != p.Plan.SHA256 {
			return errors.New("committed resync running version differs")
		}
	}
	return o.clear()
}
func (o *FleetOwner) Resync(ctx context.Context, p FleetResyncPlan) error {
	pending, e := o.prepare(ctx, p)
	if e == nil {
		e = o.copyData(ctx, pending)
	}
	if e == nil {
		e = o.swap(ctx, pending)
	}
	if e != nil {
		recovery, cancel := context.WithTimeout(context.Background(), o.ssh.Limits.Recovery)
		defer cancel()
		if r := o.Recover(recovery); r != nil {
			return fmt.Errorf("%w; recovery retained: %v", e, r)
		}
	}
	return e
}

func FetchFleetRelease(ctx context.Context, limits FleetLimits, origin, version, directory string) (string, string, error) {
	if e := limits.validate(); e != nil {
		return "", "", e
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`).MatchString(version) {
		return "", "", errors.New("invalid release version")
	}
	if e := protectedURL(origin); e != nil {
		return "", "", e
	}
	ctx, cancel := context.WithTimeout(ctx, limits.Transfer)
	defer cancel()
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: limits.Connect}).DialContext, TLSHandshakeTimeout: limits.Connect, ResponseHeaderTimeout: limits.Connect}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("release redirect limit")
		}
		return protectedURL(r.URL.String())
	}}
	fetch := func(name string, max int64) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, limits.Transfer)
		defer cancel()
		r, e := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(origin, "/")+"/"+name, nil)
		if e != nil {
			return nil, e
		}
		response, e := client.Do(r)
		if e != nil {
			return nil, errors.New("release transfer timed out or failed; no asset published")
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return nil, errors.New("release HTTP status refused")
		}
		b, e := io.ReadAll(io.LimitReader(response.Body, max+1))
		if e != nil || int64(len(b)) > max {
			return nil, errors.New("release body incomplete or exceeds limit")
		}
		return b, nil
	}
	asset := "paxd-" + version + "-linux-amd64"
	sums, e := fetch("SHA256SUMS", 1<<20)
	if e != nil {
		return "", "", e
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			if want != "" || !fleetSHA.MatchString(f[0]) {
				return "", "", errors.New("release checksum ambiguous or invalid")
			}
			want = f[0]
		}
	}
	if want == "" {
		return "", "", errors.New("release checksum missing")
	}
	data, e := fetch(asset, 512<<20)
	if e != nil {
		return "", "", e
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return "", "", errors.New("release checksum differs")
	}
	if e = os.MkdirAll(directory, 0700); e != nil {
		return "", "", e
	}
	path := filepath.Join(directory, asset)
	if e = edgeAtomic(path, data, 0700); e != nil {
		return "", "", e
	}
	if e = edgeAtomic(filepath.Join(directory, "SHA256SUMS"), sums, 0600); e != nil {
		return "", "", e
	}
	return path, want, nil
}
