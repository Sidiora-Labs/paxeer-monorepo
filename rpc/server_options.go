package evmrpc

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
)

type PendingOption struct{ source PendingSnapshot }

func NewPendingOption(source PendingSnapshot) (PendingOption, error) {
	if source == nil {
		return PendingOption{}, errors.New("pending option needs a snapshot source")
	}
	switch value := reflect.ValueOf(source); value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return PendingOption{}, errors.New("pending option needs a snapshot source")
		}
	}
	return PendingOption{source: source}, nil
}

func (PendingOption) serverOption() {}

type SequencerForwardOption struct{ url *url.URL }

func NewSequencerForwardOption(raw string) (SequencerForwardOption, error) {
	endpoint, err := url.Parse(raw)
	if err != nil {
		return SequencerForwardOption{}, fmt.Errorf("invalid sequencer forwarding URL: %w", err)
	}
	if (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Hostname() == "" {
		return SequencerForwardOption{}, errors.New("sequencer forwarding URL needs an http or https scheme and a host")
	}
	return SequencerForwardOption{url: endpoint}, nil
}

func (SequencerForwardOption) serverOption() {}
