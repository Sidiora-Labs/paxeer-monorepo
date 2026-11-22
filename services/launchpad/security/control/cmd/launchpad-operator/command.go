package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Sidiora-Technologies/KindleLaunch/security/control"
	"github.com/Sidiora-Technologies/KindleLaunch/security/control/enrolment"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	enrolCommand     = "enrol"
	revokeCommand    = "revoke"
	showCommand      = "show"
	reduceCommand    = "reduce"
	raiseCommand     = "raise"
	runtimeSetting   = "OPERATOR_DATABASE_URL"
	registrarSetting = "OPERATOR_REGISTRAR_DATABASE_URL"
	factorSetting    = "OPERATOR_FACTOR_KEY"
	gatewaySetting   = "OPERATOR_GATEWAY_URL"
	platformScope    = "platform"
)

var (
	actorPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	capabilities = [...]string{"pool_creation", "quote", "socket", "paid_features"}
	errArguments = errors.New("invalid command arguments")
	errGateway   = errors.New("invalid OPERATOR_GATEWAY_URL")
	errResponse  = errors.New("invalid control response")
)

type invocation struct {
	verb, actor, approver, wallet, role, capability, reason string
	scopes                                                  []string
	limit                                                   uint64
	hours                                                   int
}

func parse(args []string) (invocation, error) {
	var q invocation
	if len(args) == 0 {
		return q, errArguments
	}
	q.verb = args[0]
	switch q.verb {
	case enrolCommand:
		if len(args) != 5 {
			return q, errArguments
		}
		q.actor, q.wallet, q.role = args[1], args[2], args[3]
		q.scopes = strings.Split(args[4], ",")
	case revokeCommand:
		if len(args) != 2 {
			return q, errArguments
		}
		q.actor = args[1]
	case showCommand:
		if len(args) != 1 {
			return q, errArguments
		}
		return q, nil
	case reduceCommand, raiseCommand:
		fields := args[1:]
		if q.verb == raiseCommand {
			if len(fields) != 6 {
				return q, errArguments
			}
			q.approver = strings.ToLower(fields[1])
			fields = append([]string{fields[0]}, fields[2:]...)
		} else if len(fields) != 5 {
			return q, errArguments
		}
		q.actor, q.capability, q.reason = fields[0], fields[1], fields[4]
		validCapability := false
		for _, name := range capabilities {
			if q.capability == name {
				validCapability = true
			}
		}
		var err error
		q.limit, err = strconv.ParseUint(fields[2], 10, 64)
		if err != nil || !validCapability {
			return q, errArguments
		}
		q.hours, err = strconv.Atoi(fields[3])
		if err != nil || q.hours < 1 || q.hours > 24 || !utf8.ValidString(q.reason) || len(q.reason) == 0 || len(q.reason) > 1000 || strings.TrimSpace(q.reason) != q.reason || strings.ContainsAny(q.reason, "\x00\r\n") {
			return q, errArguments
		}
		if q.verb == raiseCommand && (q.limit < 1 || !actorPattern.MatchString(q.approver) || strings.EqualFold(q.actor, q.approver)) {
			return q, errArguments
		}
	default:
		return q, errArguments
	}
	q.actor = strings.ToLower(q.actor)
	if !actorPattern.MatchString(q.actor) {
		return q, errArguments
	}
	return q, nil
}

// Run executes one operator command without exposing session authority.
func Run(args, environment []string, input io.Reader, output io.Writer) int {
	q, err := parse(args)
	if err == nil {
		err = execute(q, environment, input, output)
	}
	if err != nil {
		_, _ = fmt.Fprintln(output, err.Error())
		return 1
	}
	return 0
}

func settings(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		if name, value, ok := strings.Cut(entry, "="); ok {
			values[name] = value
		}
	}
	return values
}

func gateway(raw string) (*gatewayClient, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" {
		return nil, errGateway
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, errGateway
		}
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || (!strings.EqualFold(u.Hostname(), "localhost") && (ip == nil || !ip.IsLoopback())) {
			return nil, errGateway
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &gatewayClient{base: strings.TrimSuffix(u.String(), "/"), client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

type gatewayClient struct {
	base   string
	client *http.Client
}

func (g *gatewayClient) request(ctx context.Context, method, path, credential string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return errors.New("invalid control request")
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base+"/v2/control/"+path, body)
	if err != nil {
		return errors.New("invalid control request")
	}
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := g.client.Do(req)
	if err != nil {
		return errors.New("control route unavailable")
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return errResponse
	}
	if response.StatusCode != http.StatusOK {
		var refusal control.WorkflowError
		if json.Unmarshal(raw, &refusal) != nil {
			return fmt.Errorf("control route status=%d code=INVALID_RESPONSE", response.StatusCode)
		}
		switch refusal.Code {
		case "AUTH_REQUIRED", "SCOPE_DENIED", "CURSOR_INVALID", "FIELD_INVALID", "NOT_FOUND", "IDEMPOTENCY_CONFLICT", "DEPENDENCY_UNAVAILABLE":
		default:
			refusal.Code = "INVALID_RESPONSE"
		}
		return fmt.Errorf("control route status=%d code=%s", response.StatusCode, refusal.Code)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Data) == 0 || bytes.Equal(envelope.Data, []byte("null")) {
		return errResponse
	}
	if out != nil && json.Unmarshal(envelope.Data, out) != nil {
		return errResponse
	}
	return nil
}

func (g *gatewayClient) cap(ctx context.Context, name string) (control.CapWarning, uint64, error) {
	var warning control.CapWarning
	if err := g.request(ctx, http.MethodGet, "caps/"+name, "", nil, &warning); err != nil {
		return warning, 0, err
	}
	revision, err := strconv.ParseUint(warning.Revision, 10, 64)
	if err != nil || warning.Capability != name {
		return warning, 0, errResponse
	}
	return warning, revision, nil
}

func printCap(output io.Writer, warning control.CapWarning) error {
	_, err := fmt.Fprintf(output, "%s allowed=%t stale=%t limit=%d expires=%s revision=%s\n", warning.Capability, warning.Allowed, warning.Stale, warning.Limit, warning.Expires.UTC().Format(time.RFC3339Nano), warning.Revision)
	if err != nil {
		return errors.New("operator output unavailable")
	}
	return nil
}

func execute(q invocation, environment []string, input io.Reader, output io.Writer) error {
	values := settings(environment)
	ctx := context.Background()
	var g *gatewayClient
	if q.verb == showCommand || q.verb == raiseCommand || q.verb == reduceCommand {
		var err error
		g, err = gateway(values[gatewaySetting])
		if err != nil {
			return err
		}
		defer g.client.CloseIdleConnections()
	}
	if q.verb == showCommand {
		for _, name := range capabilities {
			warning, _, err := g.cap(ctx, name)
			if err != nil {
				return err
			}
			if err := printCap(output, warning); err != nil {
				return err
			}
		}
		return nil
	}
	setting := runtimeSetting
	if q.verb == enrolCommand || q.verb == revokeCommand {
		setting = registrarSetting
	}
	if values[setting] == "" {
		return fmt.Errorf("missing %s", setting)
	}
	config, err := pgxpool.ParseConfig(values[setting])
	if err != nil {
		return fmt.Errorf("invalid %s", setting)
	}
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	key := make([]byte, 32)
	if q.verb != revokeCommand {
		key, err = hex.DecodeString(values[factorSetting])
		if err != nil || len(key) != 32 {
			return errors.New("invalid OPERATOR_FACTOR_KEY")
		}
	}
	defer clear(key)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("operator database unavailable")
	}
	defer pool.Close()
	store, err := enrolment.New(pool, key)
	if err != nil {
		return errors.New("operator authority unavailable")
	}
	switch q.verb {
	case enrolCommand:
		secret, err := store.Enrol(ctx, q.actor, q.wallet, q.role, q.scopes)
		if err != nil {
			return err
		}
		defer clear(secret)
		address := url.URL{Scheme: "otpauth", Host: "totp", Path: "/launchpad:" + q.actor}
		query := url.Values{"secret": {base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)}, "issuer": {"launchpad"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
		address.RawQuery = query.Encode()
		if _, err := fmt.Fprintln(output, address.String()); err != nil {
			return errors.New("operator output unavailable")
		}
		return nil
	case revokeCommand:
		return store.Revoke(ctx, q.actor)
	default:
		return changeCap(ctx, q, store, g, input, output)
	}
}

func readCode(scanner *bufio.Scanner) (string, error) {
	if !scanner.Scan() {
		return "", errors.New("one-time code required on standard input")
	}
	code := scanner.Text()
	if len(code) != 6 {
		return "", errors.New("invalid one-time code")
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return "", errors.New("invalid one-time code")
		}
	}
	return code, nil
}

func changeCap(ctx context.Context, q invocation, store *enrolment.Store, g *gatewayClient, input io.Reader, output io.Writer) (result error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64), 128)
	open := func(actor string) (string, string, error) {
		code, err := readCode(scanner)
		if err != nil {
			return "", "", err
		}
		return store.Open(ctx, actor, code)
	}
	closeSession := func(session string) {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result = errors.Join(result, store.Close(cleanup, session))
	}
	requester, requesterSession, err := open(q.actor)
	if err != nil {
		return err
	}
	defer closeSession(requesterSession)
	_, revision, err := g.cap(ctx, q.capability)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	cap := control.Cap{Capability: q.capability, Version: 1, Limit: q.limit, Effective: now, Expires: now.Add(time.Duration(q.hours) * time.Hour)}
	var approvalKey string
	if q.verb == raiseCommand {
		approvalKey = rand.Text()
		var proposal control.Action
		if err := g.request(ctx, http.MethodPost, "actions", requester, control.ActionRequest{Key: approvalKey, Kind: "cap_change", Scope: platformScope, Reason: q.reason, PayloadHash: control.CapDigest(cap), Expires: cap.Expires}, &proposal); err != nil {
			return err
		}
		approver, approverSession, err := open(q.approver)
		if err != nil {
			return err
		}
		defer closeSession(approverSession)
		if err := g.request(ctx, http.MethodPost, "transition", approver, control.TransitionRequest{Key: approvalKey, Scope: platformScope, Reason: q.reason, Next: "approved", Expected: &proposal.Revision}, nil); err != nil {
			return err
		}
	}
	if err := g.request(ctx, http.MethodPost, "caps", requester, control.CapRequest{Scope: platformScope, Reason: q.reason, ApprovalKey: approvalKey, Cap: cap, Expected: &revision}, nil); err != nil {
		return err
	}
	warning, _, err := g.cap(ctx, q.capability)
	if err != nil {
		return err
	}
	return printCap(output, warning)
}
