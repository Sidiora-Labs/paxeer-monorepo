package l18

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/nats-io/nats.go"
)

// This committed protocol peer retains the producer's exact HPUB frame. The
// production natsdriver client owns serialization, headers and persistence-ack
// decoding; delivery.Store owns consumer identity and transactional effects.
type repair1019JetStream struct {
	dropPublishAck  atomic.Bool
	dropConsumerAck atomic.Bool
}

func repair1019Broker(t *testing.T) (*repair1019JetStream, *nats.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	peer := &repair1019JetStream{}
	var worker sync.WaitGroup
	worker.Add(1)
	go func() {
		defer worker.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if err = peer.serve(conn); err != nil {
			t.Errorf("JetStream transcript: %v", err)
		}
	}()
	nc, err := nats.Connect("nats://"+listener.Addr().String(), nats.NoReconnect(), nats.Timeout(time.Second))
	if err != nil {
		listener.Close()
		worker.Wait()
		t.Fatal(err)
	}
	t.Cleanup(func() { nc.Close(); listener.Close(); worker.Wait() })
	return peer, nc
}

func (p *repair1019JetStream) serve(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	if _, err := writer.WriteString("INFO {\"server_id\":\"L18_RECORDED_PEER\",\"version\":\"2.11.0\",\"proto\":1,\"headers\":true,\"max_payload\":1048576}\r\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	subscriptions := map[string]string{}
	var savedWire, savedHeaders []byte
	var savedID, savedSubject string
	acknowledged := false
	deliver := func(subject, reply string, headers, wire []byte) error {
		for sid, filter := range subscriptions {
			match := filter == subject || strings.HasSuffix(filter, ".*") && strings.HasPrefix(subject, strings.TrimSuffix(filter, "*"))
			if !match {
				continue
			}
			if len(headers) > 0 {
				if _, err := fmt.Fprintf(writer, "HMSG %s %s %s %d %d\r\n", subject, sid, reply, len(headers), len(headers)+len(wire)); err != nil {
					return err
				}
				if _, err := writer.Write(headers); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(writer, "MSG %s %s %d\r\n", subject, sid, len(wire)); err != nil {
					return err
				}
			}
			if _, err := writer.Write(wire); err != nil {
				return err
			}
			if _, err := writer.WriteString("\r\n"); err != nil {
				return err
			}
		}
		return nil
	}
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF || errorsClosed(err) {
			return nil
		}
		if err != nil {
			return err
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return fmt.Errorf("empty protocol frame")
		}
		switch fields[0] {
		case "CONNECT", "PONG":
		case "PING":
			_, err = writer.WriteString("PONG\r\n")
		case "SUB":
			if len(fields) != 3 && len(fields) != 4 {
				return fmt.Errorf("invalid SUB")
			}
			subscriptions[fields[len(fields)-1]] = fields[1]
		case "UNSUB":
			if len(fields) == 2 {
				delete(subscriptions, fields[1])
			}
		case "PUB", "HPUB":
			size, parseErr := strconv.Atoi(fields[len(fields)-1])
			if parseErr != nil || size < 0 || size > delivery.MaxEnvelopeBytes+4096 {
				return fmt.Errorf("invalid payload bound")
			}
			frame := make([]byte, size+2)
			if _, err = io.ReadFull(reader, frame); err != nil {
				return err
			}
			if !bytes.HasSuffix(frame, []byte("\r\n")) {
				return fmt.Errorf("invalid payload terminator")
			}
			body := frame[:size]
			subject, reply := fields[1], ""
			if fields[0] == "HPUB" {
				if len(fields) != 5 {
					return fmt.Errorf("publisher must request persistence ack")
				}
				reply = fields[2]
				headerLength, parseErr := strconv.Atoi(fields[3])
				if parseErr != nil || headerLength < 12 || headerLength > size {
					return fmt.Errorf("invalid header length")
				}
				headers, wire := body[:headerLength], body[headerLength:]
				envelope, decodeErr := delivery.Decode(wire, RewardDeliveryPolicy(delivery.Policy{}), subject)
				if decodeErr != nil {
					return decodeErr
				}
				if !bytes.Contains(headers, []byte("Nats-Msg-Id: "+envelope.EventID+"\r\n")) || !bytes.Contains(headers, []byte("Nats-Expected-Stream: L18\r\n")) {
					return fmt.Errorf("publisher omitted identity or stream admission header: %s", headers)
				}
				duplicate := savedID != ""
				if duplicate && (savedID != envelope.EventID || savedSubject != subject || !bytes.Equal(savedWire, wire)) {
					return delivery.ErrIntegrity
				}
				if !duplicate {
					savedID, savedSubject = envelope.EventID, subject
					savedWire, savedHeaders = append([]byte(nil), wire...), append([]byte(nil), headers...)
				}
				if !acknowledged {
					err = deliver(savedSubject, "$JS.ACK.L18.reward-projection.1.1.1.1.0", savedHeaders, savedWire)
				}
				if err == nil && !p.dropPublishAck.Swap(false) {
					err = deliver(reply, "", nil, []byte(fmt.Sprintf(`{"stream":"L18","seq":1,"duplicate":%t}`, duplicate)))
				}
			} else {
				if len(fields) == 4 {
					reply = fields[2]
				}
				if !strings.HasPrefix(subject, "$JS.ACK.L18.") || string(body) != "+ACK" || reply == "" {
					return fmt.Errorf("unrecorded broker command: %s", line)
				}
				if !p.dropConsumerAck.Swap(false) {
					acknowledged = true
					err = deliver(reply, "", nil, nil)
				}
			}
		default:
			return fmt.Errorf("unrecorded NATS frame: %s", line)
		}
		if err != nil {
			return err
		}
		if err = writer.Flush(); err != nil {
			return err
		}
	}
}

func errorsClosed(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
}
