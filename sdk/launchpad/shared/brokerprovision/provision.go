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
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

var requestWait = 10 * time.Second

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
		stream, _, err := brokerRequest(ctx, func(request context.Context) (jetstream.Stream, error) {
			return js.Stream(request, s.Name)
		})
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			differences = append(differences, Difference{Kind: kindStream, Stream: s.Name, State: stateAbsent})
			continue
		}
		if err != nil {
			return nil, inspectionError(ctx, "stream "+s.Name)
		}
		info, _, err := brokerRequest(ctx, func(request context.Context) (*jetstream.StreamInfo, error) {
			return stream.Info(request)
		})
		if err != nil {
			return nil, inspectionError(ctx, "stream "+s.Name)
		}
		if !equalStream(info.Config, s.BrokerConfig()) {
			differences = append(differences, Difference{Kind: kindStream, Stream: s.Name, State: stateDiffers})
		}
	}
	for _, c := range d.Durables {
		consumer, _, err := brokerRequest(ctx, func(request context.Context) (jetstream.Consumer, error) {
			return js.Consumer(request, c.Stream, c.Durable)
		})
		if errors.Is(err, jetstream.ErrConsumerNotFound) || errors.Is(err, jetstream.ErrStreamNotFound) {
			differences = append(differences, Difference{Kind: kindDurable, Stream: c.Stream, Durable: c.Durable, State: stateAbsent})
			continue
		}
		if err != nil {
			return nil, inspectionError(ctx, "durable "+c.Stream+"/"+c.Durable)
		}
		info, _, err := brokerRequest(ctx, func(request context.Context) (*jetstream.ConsumerInfo, error) {
			return consumer.Info(request)
		})
		if err != nil {
			return nil, inspectionError(ctx, "durable "+c.Stream+"/"+c.Durable)
		}
		expected, _ := c.BrokerConsumerConfig()
		replicas := streamReplicas(d, c.Stream)
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
						var retried bool
						_, retried, err = brokerRequest(ctx, func(request context.Context) (jetstream.Stream, error) {
							return js.CreateStream(request, s.BrokerConfig())
						})
						if retried && errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
							info, readErr := inspectStream(ctx, js, s.Name)
							if readErr == nil && equalStream(info.Config, s.BrokerConfig()) {
								err = nil
							} else if ctx.Err() != nil {
								err = ctx.Err()
							}
						}
					} else {
						_, _, err = brokerRequest(ctx, func(request context.Context) (jetstream.Stream, error) {
							return js.UpdateStream(request, s.BrokerConfig())
						})
					}
					break
				}
			}
		} else {
			for _, c := range d.Durables {
				if c.Stream == difference.Stream && c.Durable == difference.Durable {
					config, _ := c.BrokerConsumerConfig()
					if difference.State == stateAbsent {
						var retried bool
						_, retried, err = brokerRequest(ctx, func(request context.Context) (jetstream.Consumer, error) {
							return js.CreateConsumer(request, c.Stream, config)
						})
						if retried && errors.Is(err, jetstream.ErrConsumerExists) {
							info, readErr := inspectConsumer(ctx, js, c.Stream, c.Durable)
							if readErr == nil && info.Stream == c.Stream && info.Name == c.Durable && equalConsumer(info.Config, config, streamReplicas(d, c.Stream)) {
								err = nil
							} else if ctx.Err() != nil {
								err = ctx.Err()
							}
						}
					} else {
						_, _, err = brokerRequest(ctx, func(request context.Context) (jetstream.Consumer, error) {
							return js.UpdateConsumer(request, c.Stream, config)
						})
					}
					break
				}
			}
		}
		if err != nil {
			return applied, fmt.Errorf("brokerprovision: cannot apply %s: %w", difference.String(), err)
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

func brokerRequest[T any](ctx context.Context, send func(context.Context) (T, error)) (T, bool, error) {
	var zero T
	retried := false
	for {
		if err := ctx.Err(); err != nil {
			return zero, retried, err
		}
		request, cancel := context.WithTimeout(ctx, requestWait)
		result, err := send(request)
		expired := errors.Is(request.Err(), context.DeadlineExceeded)
		cancel()
		if callerErr := ctx.Err(); callerErr != nil {
			return zero, retried, callerErr
		}
		if expired && errors.Is(err, context.DeadlineExceeded) {
			retried = true
			continue
		}
		return result, retried, err
	}
}

func inspectStream(ctx context.Context, js jetstream.JetStream, name string) (*jetstream.StreamInfo, error) {
	stream, _, err := brokerRequest(ctx, func(request context.Context) (jetstream.Stream, error) {
		return js.Stream(request, name)
	})
	if err != nil {
		return nil, err
	}
	info, _, err := brokerRequest(ctx, func(request context.Context) (*jetstream.StreamInfo, error) {
		return stream.Info(request)
	})
	return info, err
}

func inspectConsumer(ctx context.Context, js jetstream.JetStream, stream, durable string) (*jetstream.ConsumerInfo, error) {
	consumer, _, err := brokerRequest(ctx, func(request context.Context) (jetstream.Consumer, error) {
		return js.Consumer(request, stream, durable)
	})
	if err != nil {
		return nil, err
	}
	info, _, err := brokerRequest(ctx, func(request context.Context) (*jetstream.ConsumerInfo, error) {
		return consumer.Info(request)
	})
	return info, err
}

func inspectionError(ctx context.Context, name string) error {
	message := "brokerprovision: cannot inspect " + name
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	return errors.New(message)
}

func streamReplicas(d Document, name string) int {
	for _, s := range d.Streams {
		if s.Name == name {
			return s.Limits.Replicas
		}
	}
	return 0
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
