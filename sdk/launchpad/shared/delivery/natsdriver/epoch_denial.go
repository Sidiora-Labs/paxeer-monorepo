package natsdriver

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func verifyResumedEpochMember(ctx context.Context, m EpochMemberAuthority, progress *epochMemberProgress) ([]EpochCredentialEvidence, error) {
	evidence := make([]EpochCredentialEvidence, 0, len(m.Credentials))
	for _, credential := range m.Credentials {
		ack := progress.Acks[credential.ID]
		prefix := "$JS.ACK." + m.Probe.Stream + "." + m.Probe.Durable + "."
		if !strings.HasPrefix(ack, prefix) || strings.ContainsAny(ack, " \r\n\t") || len(ack) > 512 {
			return nil, errors.New("natsdriver: durable broker acknowledgement identity required")
		}
		conn, err := connectEpochCredential(m, credential)
		if conn != nil {
			conn.Close()
		}
		if !errors.Is(err, nats.ErrAuthorization) {
			return nil, errors.New("natsdriver: broker must explicitly deny resumed old authentication")
		}
		// Publish both a new message and the original outstanding acknowledgement
		// under the old credentials. A network failure is never denial evidence.
		if err = epochCommandDenied(ctx, m, credential, m.Probe.Subject, []byte("revoked-epoch-probe")); err != nil {
			return nil, err
		}
		if err = epochCommandDenied(ctx, m, credential, ack, []byte("+ACK")); err != nil {
			return nil, err
		}
		evidence = append(evidence, EpochCredentialEvidence{Member: m.Name, ServerID: m.ServerID, CredentialID: credential.ID, ConfigurationSHA256: m.ReplacementSHA256, AuthenticationDenied: true, PublishDenied: true, AckDenied: true})
	}
	if !m.Control.IsConnected() || m.Control.ConnectedServerId() != m.ServerID {
		return nil, errors.New("natsdriver: surviving control authority required")
	}
	if err := m.Control.FlushWithContext(ctx); err != nil {
		return nil, err
	}
	return evidence, nil
}

func epochCommandDenied(ctx context.Context, m EpochMemberAuthority, credential EpochCredential, subject string, payload []byte) error {
	address, err := url.Parse(m.URL)
	if err != nil {
		return err
	}
	host := address.Host
	if address.Port() == "" {
		host = net.JoinHostPort(address.Hostname(), "4222")
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(bounded, "tcp", host)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline, _ := bounded.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(conn, 64<<10)
	line, err := reader.ReadString('\n')
	if err != nil || len(line) > 64<<10 || !strings.HasPrefix(line, "INFO ") {
		return errors.New("natsdriver: explicit broker greeting required for denial")
	}
	var info struct {
		ServerID    string `json:"server_id"`
		TLSRequired bool   `json:"tls_required"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "INFO "))), &info) != nil || info.ServerID != m.ServerID {
		return errors.New("natsdriver: denial broker identity mismatch")
	}
	if address.Scheme == "tls" || info.TLSRequired {
		secured := tls.Client(conn, &tls.Config{ServerName: address.Hostname(), MinVersion: tls.VersionTLS12})
		if err = secured.HandshakeContext(bounded); err != nil {
			return err
		}
		conn = secured
		reader = bufio.NewReaderSize(conn, 64<<10)
	}
	auth, err := json.Marshal(map[string]any{"user": credential.Username, "pass": credential.Password, "verbose": false, "pedantic": true, "protocol": 1})
	if err != nil {
		return err
	}
	command := fmt.Sprintf("CONNECT %s\r\nPUB %s _INBOX.epoch_denial %d\r\n%s\r\nPING\r\n", auth, subject, len(payload), payload)
	if _, err = io.WriteString(conn, command); err != nil {
		return err
	}
	for lines := 0; lines < 8; lines++ {
		line, err = reader.ReadString('\n')
		if err != nil {
			return errors.New("natsdriver: explicit command denial unavailable")
		}
		if len(line) > 64<<10 {
			return errors.New("natsdriver: oversized denial response")
		}
		if strings.TrimSpace(line) == "-ERR 'Authorization Violation'" || strings.TrimSpace(line) == `-ERR "Authorization Violation"` {
			return nil
		}
		if strings.TrimSpace(line) == "PONG" || strings.HasPrefix(line, "MSG ") || strings.HasPrefix(line, "-ERR ") {
			return errors.New("natsdriver: old credential command not explicitly denied")
		}
	}
	return errors.New("natsdriver: bounded command denial unavailable")
}
