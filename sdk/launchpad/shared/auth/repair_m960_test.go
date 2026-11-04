package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const repairM960StreamInfo = `{"type":"io.nats.jetstream.api.v1.stream_info_response","config":{"name":"AUTH_REPAIR","subjects":["security.audit.repair"],"storage":"file","num_replicas":1},"state":{}}`

func repairM960ConsumerInfo(durable string) string {
	return fmt.Sprintf(`{"type":"io.nats.jetstream.api.v1.consumer_info_response","stream_name":"AUTH_REPAIR","name":%q,"config":{"name":%q,"durable_name":%q,"deliver_policy":"all","ack_policy":"explicit","ack_wait":5000000000,"max_deliver":5,"filter_subject":"security.audit.repair","replay_policy":"instant","max_ack_pending":20,"max_waiting":1,"max_batch":10,"max_expires":1000000000,"num_replicas":0}}`, durable, durable, durable)
}

type repairM960Exchange struct {
	subject, request, response string
}

func repairM960AttachTranscript(durable string) []repairM960Exchange {
	info := repairM960ConsumerInfo(durable)
	var response struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal([]byte(info), &response); err != nil {
		panic(err)
	}
	return []repairM960Exchange{
		{"$JS.API.CONSUMER.CREATE.AUTH_REPAIR." + durable + ".security.audit.repair", `{"stream_name":"AUTH_REPAIR","config":` + string(response.Config) + `,"action":"create"}`, info},
		{"$JS.API.STREAM.INFO.AUTH_REPAIR", "", repairM960StreamInfo},
		{"$JS.API.STREAM.INFO.AUTH_REPAIR", "", repairM960StreamInfo},
		{"$JS.API.CONSUMER.INFO.AUTH_REPAIR." + durable, "", info},
		{"$JS.API.CONSUMER.INFO.AUTH_REPAIR." + durable, "", info},
	}
}

func TestRepairM_960_AuditDurableFixture(t *testing.T) {
	for _, durable := range []string{"audit_fixture", "audit_replay"} {
		t.Run(durable, func(t *testing.T) {
			nc, js := repairM960Peer(t, repairM960AttachTranscript(durable))
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			consumer, err := newAuditFixtureConsumer(ctx, nc, js, "AUTH_REPAIR", durable, "security.audit.repair")
			if err != nil {
				t.Fatalf("provision and attach %s with the reviewed broker configuration: %v", durable, err)
			}
			if consumer == nil {
				t.Fatal("successful attachment returned no consumer")
			}
			consumer.Close()
		})
	}
}

func TestRepairM_960_UnprovisionedDurablesRefused(t *testing.T) {
	for _, durable := range []string{"audit_fixture", "audit_replay"} {
		t.Run(durable, func(t *testing.T) {
			nc, _ := repairM960Peer(t, []repairM960Exchange{
				{"$JS.API.STREAM.INFO.AUTH_REPAIR", "", repairM960StreamInfo},
				{"$JS.API.STREAM.INFO.AUTH_REPAIR", "", repairM960StreamInfo},
				{"$JS.API.CONSUMER.INFO.AUTH_REPAIR." + durable, "", `{"type":"io.nats.jetstream.api.v1.consumer_info_response","error":{"code":404,"err_code":10014,"description":"consumer not found"}}`},
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cfg := natsdriver.ConsumerConfig{Stream: "AUTH_REPAIR", Durable: durable, Subject: "security.audit.repair", Batch: 10, MaxAckPending: 20, MaxWaiting: 1, MaxDeliver: 5, FetchWait: time.Second, AckWait: 5 * time.Second}
			consumer, err := natsdriver.NewConsumer(ctx, nc, cfg)
			if consumer != nil {
				consumer.Close()
				t.Fatal("attachment created an absent durable")
			}
			if !errors.Is(err, jetstream.ErrConsumerNotFound) {
				t.Fatalf("missing durable must retain the broker refusal: %v", err)
			}
		})
	}
}

func TestRepairM_960_ProvisioningFailures(t *testing.T) {
	for _, durable := range []string{"audit_fixture", "audit_replay"} {
		t.Run(durable, func(t *testing.T) {
			t.Run("create_refused", func(t *testing.T) {
				transcript := repairM960AttachTranscript(durable)[:1]
				transcript[0].response = `{"type":"io.nats.jetstream.api.v1.consumer_create_response","error":{"code":503,"err_code":10008,"description":"JetStream system temporarily unavailable"}}`
				nc, js := repairM960Peer(t, transcript)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				consumer, err := newAuditFixtureConsumer(ctx, nc, js, "AUTH_REPAIR", durable, "security.audit.repair")
				if consumer != nil {
					consumer.Close()
					t.Fatal("consumer attached after provisioning failed")
				}
				var apiError *jetstream.APIError
				if !errors.As(err, &apiError) || apiError.Code != 503 || apiError.ErrorCode != 10008 {
					t.Fatalf("provisioning refusal was lost: %v", err)
				}
			})
			t.Run("configuration_drift", func(t *testing.T) {
				transcript := repairM960AttachTranscript(durable)
				transcript[4].response = strings.Replace(transcript[4].response, `"max_ack_pending":20`, `"max_ack_pending":21`, 1)
				nc, js := repairM960Peer(t, transcript)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				consumer, err := newAuditFixtureConsumer(ctx, nc, js, "AUTH_REPAIR", durable, "security.audit.repair")
				if consumer != nil {
					consumer.Close()
					t.Fatal("consumer attached with unreviewed broker limits")
				}
				if err == nil || err.Error() != "natsdriver: durable configuration differs from reviewed configuration" {
					t.Fatalf("configuration mismatch was not preserved: %v", err)
				}
			})
		})
	}
}

// The peer replays a bounded JetStream API transcript over loopback TCP. The
// fixture helper, configuration codec, NATS transport and attaching driver are
// the real implementations; the peer has no consumer or storage implementation.
func repairM960Peer(t *testing.T, transcript []repairM960Exchange) (*nats.Conn, jetstream.JetStream) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			done <- err
			return
		}
		done <- repairM960Replay(conn, transcript)
	}()
	t.Cleanup(func() { _ = listener.Close() })
	nc, err := nats.Connect("nats://"+listener.Addr().String(), nats.NoReconnect(), nats.Timeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		nc.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	return nc, js
}

func repairM960Replay(conn net.Conn, transcript []repairM960Exchange) error {
	if _, err := io.WriteString(conn, "INFO {\"server_id\":\"AUTH_REPAIR_TRANSCRIPT\",\"version\":\"2.12.2\",\"proto\":1,\"headers\":true,\"max_payload\":1048576}\r\n"); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	subscriptions := map[string]string{}
	next := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && next == len(transcript) {
				return nil
			}
			return fmt.Errorf("transcript ended after %d/%d exchanges: %w", next, len(transcript), err)
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return fmt.Errorf("empty NATS protocol line")
		}
		switch fields[0] {
		case "CONNECT", "UNSUB", "PONG":
		case "PING":
			if _, err := io.WriteString(conn, "PONG\r\n"); err != nil {
				return err
			}
		case "SUB":
			if len(fields) != 3 {
				return fmt.Errorf("unexpected subscription: %q", line)
			}
			subscriptions[fields[1]] = fields[2]
		case "PUB":
			if len(fields) != 4 || next >= len(transcript) {
				return fmt.Errorf("unexpected request after %d exchanges: %q", next, line)
			}
			size, err := strconv.Atoi(fields[3])
			if err != nil || size < 0 || size > 65536 {
				return fmt.Errorf("invalid transcript request size: %q", fields[3])
			}
			body := make([]byte, size+2)
			if _, err := io.ReadFull(reader, body); err != nil {
				return err
			}
			step := transcript[next]
			if fields[1] != step.subject || string(body[size:]) != "\r\n" {
				return fmt.Errorf("exchange %d: got %q, want %q", next, fields[1], step.subject)
			}
			var actual, expected any
			if size != 0 || step.request != "" {
				if err := json.Unmarshal(body[:size], &actual); err != nil {
					return err
				}
				if err := json.Unmarshal([]byte(step.request), &expected); err != nil {
					return err
				}
				if !reflect.DeepEqual(actual, expected) {
					return fmt.Errorf("exchange %d: request %s differs from reviewed configuration %s", next, body[:size], step.request)
				}
			}
			sid := ""
			for subject, id := range subscriptions {
				if subject == fields[2] || strings.HasSuffix(subject, ".*") && strings.HasPrefix(fields[2], strings.TrimSuffix(subject, "*")) {
					sid = id
				}
			}
			if sid == "" {
				return fmt.Errorf("no reply subscription for %q", fields[2])
			}
			if _, err := fmt.Fprintf(conn, "MSG %s %s %d\r\n%s\r\n", fields[2], sid, len(step.response), step.response); err != nil {
				return err
			}
			next++
		default:
			return fmt.Errorf("unexpected NATS protocol command: %q", line)
		}
	}
}
