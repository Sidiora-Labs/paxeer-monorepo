package source

import (
	"context"
	"errors"
	"testing"
)

func TestSourceReadinessTracksAdmissionCapacityAndShutdown(t *testing.T) {
	ctx := context.Background()
	isKind := func(err error, kind ErrorKind) bool {
		var failure *SourceError
		return errors.As(err, &failure) && failure.Kind == kind
	}
	node := repair997Serve(t)
	live := repair997Open(t, fopts(node.server.URL))
	if err := live.Ready(); err != nil {
		t.Fatalf("healthy source: %v", err)
	}
	releases := make([]func(), 0, 1024)
	for i := 0; i < 1024; i++ {
		_, done, err := live.lifecycle.begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, done)
	}
	if err := live.Ready(); !isKind(err, OverloadError) {
		t.Fatalf("saturated source: %v", err)
	}
	for _, done := range releases {
		done()
	}
	if err := live.Ready(); err != nil {
		t.Fatalf("drained source: %v", err)
	}
	node.invalid.Store(true)
	var identity string
	for i := 0; i < 3; i++ {
		if err := live.rpc(ctx, live.endpoints[0], false, "eth_chainId", nil, &identity); err == nil {
			t.Fatal("invalid endpoint identity admitted")
		}
	}
	if err := live.Ready(); !isKind(err, IdentityError) {
		t.Fatalf("unavailable endpoints: %v", err)
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	if err := live.Ready(); !isKind(err, ClosedError) {
		t.Fatalf("closed source: %v", err)
	}
	if err := (*CommittedSource)(nil).Ready(); !isKind(err, IdentityError) {
		t.Fatalf("absent source: %v", err)
	}
}
