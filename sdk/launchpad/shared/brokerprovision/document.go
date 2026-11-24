// Package brokerprovision manages the launchpad's file-backed streams and durables.
package brokerprovision

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go/jetstream"
)

const MaxDocumentBytes = 1 << 20

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var literalToken = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Document struct {
	Streams  []Stream                    `json:"streams"`
	Durables []natsdriver.ConsumerConfig `json:"durables"`
}

type Stream struct {
	Name     string       `json:"name"`
	Subjects []string     `json:"subjects"`
	Limits   StreamLimits `json:"limits"`
}

type StreamLimits struct {
	MaxMessages     int64         `json:"maxMessages"`
	MaxBytes        int64         `json:"maxBytes"`
	MaxAge          time.Duration `json:"maxAge"`
	MaxMessageBytes int32         `json:"maxMessageBytes"`
	MaxConsumers    int           `json:"maxConsumers"`
	Replicas        int           `json:"replicas"`
	DuplicateWindow time.Duration `json:"duplicateWindow"`
	Discard         string        `json:"discard"`
}

func Read(r io.Reader) (Document, error) {
	var d Document
	if r == nil {
		return d, errors.New("brokerprovision: document required")
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxDocumentBytes+1))
	if err != nil || len(raw) > MaxDocumentBytes {
		return d, errors.New("brokerprovision: cannot read bounded document")
	}
	tokens := json.NewDecoder(bytes.NewReader(raw))
	if err = uniqueValue(tokens); err != nil {
		return d, errors.New("brokerprovision: invalid JSON document")
	}
	if _, err = tokens.Token(); err != io.EOF {
		return d, errors.New("brokerprovision: one JSON document required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&d); err != nil {
		return Document{}, errors.New("brokerprovision: invalid document fields")
	}
	if err = d.Validate(); err != nil {
		return Document{}, err
	}
	return d, nil
}

func uniqueValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return errors.New("null value")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[strings.ToLower(name)] {
				return errors.New("duplicate field")
			}
			seen[strings.ToLower(name)] = true
			if err = uniqueValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueValue(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected delimiter")
	}
	_, err = d.Token()
	return err
}

func (d Document) Validate() error {
	if len(d.Streams) == 0 || d.Durables == nil {
		return errors.New("brokerprovision: streams and durables required")
	}
	streams := make(map[string]Stream, len(d.Streams))
	for _, s := range d.Streams {
		l := s.Limits
		if !identifier.MatchString(s.Name) || len(s.Subjects) == 0 || l.MaxMessages < 1 || l.MaxMessageBytes < 1 || l.MaxBytes < int64(l.MaxMessageBytes) || l.MaxAge <= 0 || l.MaxConsumers < 1 || l.MaxConsumers > 10000 || l.Replicas < 1 || l.Replicas > 5 || l.DuplicateWindow <= 0 || l.DuplicateWindow > l.MaxAge || (l.Discard != "new" && l.Discard != "old") {
			return fmt.Errorf("brokerprovision: invalid stream at position %d", len(streams))
		}
		if _, exists := streams[s.Name]; exists {
			return fmt.Errorf("brokerprovision: duplicate stream %s", s.Name)
		}
		seen := map[string]bool{}
		for _, subject := range s.Subjects {
			if !validFilter(subject) || seen[subject] {
				return fmt.Errorf("brokerprovision: invalid subjects for stream %s", s.Name)
			}
			seen[subject] = true
		}
		streams[s.Name] = s
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	for i, c := range d.Durables {
		if _, err := c.BrokerConsumerConfig(); err != nil {
			return fmt.Errorf("brokerprovision: invalid durable at position %d", i)
		}
		stream, exists := streams[c.Stream]
		if !exists {
			return fmt.Errorf("brokerprovision: durable %s/%s references an unlisted stream", c.Stream, c.Durable)
		}
		covered := false
		for _, filter := range stream.Subjects {
			covered = covered || Covers(filter, c.Subject)
		}
		if !covered {
			return fmt.Errorf("brokerprovision: stream %s does not cover durable %s", c.Stream, c.Durable)
		}
		key := c.Stream + "/" + c.Durable
		if seen[key] {
			return fmt.Errorf("brokerprovision: duplicate durable %s", key)
		}
		seen[key] = true
		counts[c.Stream]++
		if counts[c.Stream] > stream.Limits.MaxConsumers {
			return fmt.Errorf("brokerprovision: stream %s consumer limit exceeded", c.Stream)
		}
	}
	return nil
}

func validFilter(subject string) bool {
	if len(subject) == 0 || len(subject) > 256 {
		return false
	}
	tokens := strings.Split(subject, ".")
	for i, token := range tokens {
		if token == ">" {
			return i == len(tokens)-1
		}
		if token != "*" && !literalToken.MatchString(token) {
			return false
		}
	}
	return true
}

// Covers reports whether a stream filter captures a literal consumer subject.
func Covers(filter, subject string) bool {
	if !validFilter(filter) || !validFilter(subject) || strings.ContainsAny(subject, "*>") {
		return false
	}
	outer, inner := strings.Split(filter, "."), strings.Split(subject, ".")
	for i, token := range outer {
		if token == ">" {
			return len(inner) > i
		}
		if i >= len(inner) || (token != "*" && token != inner[i]) {
			return false
		}
	}
	return len(inner) == len(outer)
}

func (s Stream) BrokerConfig() jetstream.StreamConfig {
	discard := jetstream.DiscardOld
	if s.Limits.Discard == "new" {
		discard = jetstream.DiscardNew
	}
	return jetstream.StreamConfig{Name: s.Name, Subjects: append([]string(nil), s.Subjects...), Storage: jetstream.FileStorage, Retention: jetstream.LimitsPolicy, Discard: discard, MaxMsgs: s.Limits.MaxMessages, MaxBytes: s.Limits.MaxBytes, MaxAge: s.Limits.MaxAge, MaxMsgSize: s.Limits.MaxMessageBytes, MaxConsumers: s.Limits.MaxConsumers, Replicas: s.Limits.Replicas, Duplicates: s.Limits.DuplicateWindow, MaxMsgsPerSubject: -1}
}
