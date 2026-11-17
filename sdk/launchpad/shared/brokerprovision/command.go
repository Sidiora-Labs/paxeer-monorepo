package brokerprovision

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type names []string

func (n *names) String() string         { return strings.Join(*n, ",") }
func (n *names) Set(value string) error { *n = append(*n, value); return nil }

type source struct{ file, env string }

func (s *source) flags(f *flag.FlagSet, name, description string) {
	f.StringVar(&s.file, name+"-file", "", description+" file")
	f.StringVar(&s.env, name+"-env", "", description+" environment variable name")
}
func (s source) present() bool { return s.file != "" || s.env != "" }
func (s source) read() ([]byte, error) {
	if (s.file == "") == (s.env == "") {
		return nil, errors.New("one file or variable required")
	}
	var raw []byte
	if s.file != "" {
		f, err := os.Open(s.file)
		if err != nil {
			return nil, errors.New("cannot read source")
		}
		defer f.Close()
		raw, err = io.ReadAll(io.LimitReader(f, MaxDocumentBytes+1))
		if err != nil {
			return nil, errors.New("cannot read source")
		}
	} else {
		raw = []byte(os.Getenv(s.env))
	}
	if len(raw) == 0 || len(raw) > MaxDocumentBytes {
		return nil, errors.New("source is empty or too large")
	}
	return raw, nil
}

// Run executes the command. Status 1 means drift; status 2 means an operational
// or input error. Broker and authentication errors never include source values.
func Run(ctx context.Context, args []string, out, errOut io.Writer) int {
	fail := func(message string) int {
		fmt.Fprintln(errOut, "brokerprovision: "+strings.TrimPrefix(message, "brokerprovision: "))
		return 2
	}
	if ctx == nil {
		return fail("context required")
	}
	if len(args) == 0 || (args[0] != "provision" && args[0] != "check") {
		return fail("expected provision or check")
	}
	flags := flag.NewFlagSet("brokerprovision", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var address, credentials, token source
	address.flags(flags, "broker", "broker address")
	credentials.flags(flags, "credentials", "NATS user credentials")
	token.flags(flags, "token", "NATS authentication token")
	document := flags.String("document", "", "stream and durable document file")
	timeout := flags.Duration("timeout", 30*time.Second, "total operation deadline (1s..5m)")
	var updates Updates
	var streams, durables names
	flags.Var(&streams, "update-stream", "allow update of this stream name (repeatable)")
	flags.Var(&durables, "update-durable", "allow update of this stream/durable pair (repeatable)")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return fail("invalid arguments")
	}
	updates.Streams, updates.Durables = streams, durables
	if *document == "" || *timeout < time.Second || *timeout > 5*time.Minute {
		return fail("document and bounded timeout required")
	}
	if args[0] == "check" && (len(streams) > 0 || len(durables) > 0) {
		return fail("check does not accept updates")
	}
	f, err := os.Open(*document)
	if err != nil {
		return fail("cannot open document")
	}
	d, err := Read(f)
	f.Close()
	if err != nil {
		return fail(err.Error())
	}
	if err = updates.validate(d); err != nil {
		return fail("update must name a unique listed resource")
	}
	rawAddress, err := address.read()
	if err != nil {
		return fail("broker source invalid")
	}
	broker := strings.TrimSpace(string(rawAddress))
	parsed, err := url.Parse(broker)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.Opaque != "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.ForceQuery || (parsed.Scheme != "nats" && parsed.Scheme != "tls") {
		return fail("broker address invalid")
	}
	if credentials.present() == token.present() {
		return fail("one credentials or token source required")
	}
	options := []nats.Option{nats.NoReconnect(), nats.Timeout(*timeout), nats.Name("launchpad-brokerprovision"), nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {})}
	if credentials.present() {
		raw, err := credentials.read()
		if err != nil || validateCredentials(raw) != nil {
			return fail("credentials source invalid")
		}
		options = append(options, nats.UserCredentialBytes(raw))
	} else {
		raw, err := token.read()
		if err != nil {
			return fail("token source invalid")
		}
		secret := strings.TrimSpace(string(raw))
		if secret == "" || strings.ContainsAny(secret, "\r\n") {
			return fail("token source invalid")
		}
		options = append(options, nats.Token(secret))
	}
	bounded, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	if bounded.Err() != nil {
		return fail("operation canceled")
	}
	options = append(options, nats.Timeout(time.Until(deadline(bounded))))
	nc, err := nats.Connect(broker, options...)
	if err != nil {
		return fail("broker connection failed")
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return fail("broker API unavailable")
	}
	if args[0] == "check" {
		differences, err := Check(bounded, js, d)
		if err != nil {
			return fail(err.Error())
		}
		for _, difference := range differences {
			if _, err = fmt.Fprintln(out, difference.String()); err != nil {
				return fail("cannot write report")
			}
		}
		if len(differences) > 0 {
			return 1
		}
		return 0
	}
	changes, err := Provision(bounded, js, d, updates)
	for _, change := range changes {
		if _, writeErr := fmt.Fprintln(out, change.String()); writeErr != nil {
			return fail("cannot write report")
		}
	}
	if err != nil {
		return fail(err.Error())
	}
	return 0
}

func deadline(ctx context.Context) time.Time { value, _ := ctx.Deadline(); return value }

func validateCredentials(raw []byte) error {
	token, err := jwt.ParseDecoratedJWT(raw)
	if err != nil {
		return errors.New("invalid credentials")
	}
	claims, err := jwt.DecodeUserClaims(token)
	if err != nil {
		return errors.New("invalid credentials")
	}
	key, err := jwt.ParseDecoratedUserNKey(raw)
	if err != nil {
		return errors.New("invalid credentials")
	}
	defer key.Wipe()
	public, err := key.PublicKey()
	if err != nil || public != claims.Subject {
		return errors.New("credentials identity differs")
	}
	return nil
}
