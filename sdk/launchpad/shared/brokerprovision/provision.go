package brokerprovision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

type Difference struct {
	Kind    string
	Stream  string
	Durable string
	State   string
}

func (d Difference) String() string {
	if d.Kind == kindDurable {
		return fmt.Sprintf("durable %s/%s %s", d.Stream, d.Durable, d.State)
	}
	return fmt.Sprintf("stream %s %s", d.Stream, d.State)
}

type Updates struct {
	Streams  []string
	Durables []string // Stream/durable pairs.
}

func (u Updates) validate(d Document) error {
	streams, durables := map[string]bool{}, map[string]bool{}
	for _, s := range d.Streams {
		streams[s.Name] = true
	}
	for _, c := range d.Durables {
		durables[c.Stream+"/"+c.Durable] = true
	}
	for _, group := range []struct {
		names []string
		known map[string]bool
	}{{u.Streams, streams}, {u.Durables, durables}} {
		seen := map[string]bool{}
		for _, name := range group.names {
			if !group.known[name] || seen[name] {
				return errors.New("brokerprovision: update must name a unique listed resource")
			}
			seen[name] = true
		}
	}
	return nil
}

// Check reads every listed resource and reports only missing or unequal entries.
func Check(ctx context.Context, js jetstream.JetStream, d Document) ([]Difference, error) {
	if ctx == nil || js == nil {
		return nil, errors.New("brokerprovision: context and broker required")
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	differences := []Difference{}
	for _, s := range d.Streams {
		stream, err := js.Stream(ctx, s.Name)
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			differences = append(differences, Difference{Kind: kindStream, Stream: s.Name, State: stateAbsent})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("brokerprovision: cannot inspect stream %s", s.Name)
		}
		info, err := stream.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("brokerprovision: cannot inspect stream %s", s.Name)
		}
		if !equalStream(info.Config, s.BrokerConfig()) {
			differences = append(differences, Difference{Kind: kindStream, Stream: s.Name, State: stateDiffers})
		}
	}
	for _, c := range d.Durables {
		consumer, err := js.Consumer(ctx, c.Stream, c.Durable)
		if errors.Is(err, jetstream.ErrConsumerNotFound) || errors.Is(err, jetstream.ErrStreamNotFound) {
			differences = append(differences, Difference{Kind: kindDurable, Stream: c.Stream, Durable: c.Durable, State: stateAbsent})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("brokerprovision: cannot inspect durable %s/%s", c.Stream, c.Durable)
		}
		info, err := consumer.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("brokerprovision: cannot inspect durable %s/%s", c.Stream, c.Durable)
		}
		expected, _ := c.BrokerConsumerConfig()
		replicas := 0
		for _, s := range d.Streams {
			if s.Name == c.Stream {
				replicas = s.Limits.Replicas
			}
		}
		if info.Stream != c.Stream || info.Name != c.Durable || !equalConsumer(info.Config, expected, replicas) {
			differences = append(differences, Difference{Kind: kindDurable, Stream: c.Stream, Durable: c.Durable, State: stateDiffers})
		}
	}
	return differences, nil
}

// Provision preflights all drift before creating or updating listed resources.
// Updates use broker update APIs without deleting resources.
func Provision(ctx context.Context, js jetstream.JetStream, d Document, u Updates) ([]Difference, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	if err := u.validate(d); err != nil {
		return nil, err
	}
	differences, err := Check(ctx, js, d)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, name := range u.Streams {
		allowed["stream "+name] = true
	}
	for _, name := range u.Durables {
		allowed["durable "+name] = true
	}
	for _, difference := range differences {
		name := difference.Kind + " " + difference.Stream
		if difference.Kind == kindDurable {
			name += "/" + difference.Durable
		}
		if difference.State == stateDiffers && !allowed[name] {
			return nil, fmt.Errorf("brokerprovision: %s differs; explicit update required", name)
		}
	}
	applied := []Difference{}
	for _, difference := range differences {
		if difference.Kind == kindStream {
			for _, s := range d.Streams {
				if s.Name == difference.Stream {
					if difference.State == stateAbsent {
						_, err = js.CreateStream(ctx, s.BrokerConfig())
					} else {
						_, err = js.UpdateStream(ctx, s.BrokerConfig())
					}
					break
				}
			}
		} else {
			for _, c := range d.Durables {
				if c.Stream == difference.Stream && c.Durable == difference.Durable {
					config, _ := c.BrokerConsumerConfig()
					if difference.State == stateAbsent {
						_, err = js.CreateConsumer(ctx, c.Stream, config)
					} else {
						_, err = js.UpdateConsumer(ctx, c.Stream, config)
					}
					break
				}
			}
		}
		if err != nil {
			return applied, fmt.Errorf("brokerprovision: cannot apply %s", difference.String())
		}
		applied = append(applied, difference)
	}
	remaining, err := Check(ctx, js, d)
	if err != nil {
		return applied, err
	}
	if len(remaining) != 0 {
		return applied, errors.New("brokerprovision: broker configuration differs after provisioning")
	}
	return applied, nil
}

func equalStream(actual, expected jetstream.StreamConfig) bool {
	actual.Subjects = append([]string(nil), actual.Subjects...)
	sort.Strings(actual.Subjects)
	sort.Strings(expected.Subjects)
	actual.Metadata = applicationMetadata(actual.Metadata)
	return equalJSON(actual, expected)
}

func equalConsumer(actual, expected jetstream.ConsumerConfig, replicas int) bool {
	// Zero requests inheritance of the stream's replication factor.
	if actual.Replicas == replicas {
		actual.Replicas = 0
	}
	actual.Metadata = applicationMetadata(actual.Metadata)
	return equalJSON(actual, expected)
}

func applicationMetadata(metadata map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range metadata {
		if !strings.HasPrefix(key, "_nats.") {
			result[key] = value
		}
	}
	return result
}

func equalJSON(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	var x, y any
	leftDecoder, rightDecoder := json.NewDecoder(bytes.NewReader(left)), json.NewDecoder(bytes.NewReader(right))
	leftDecoder.UseNumber()
	rightDecoder.UseNumber()
	if leftDecoder.Decode(&x) != nil || rightDecoder.Decode(&y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
