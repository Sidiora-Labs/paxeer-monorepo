package source

import (
	"context"
	"testing"
)

func TestPrunedAnchorAdmissionRequiresCorroboratingArchive(t *testing.T) {
	ctx := context.Background()
	primary, archive := repair997Serve(t), repair997Serve(t)
	primary.pruned.Store(true)
	opts := fopts(primary.server.URL)
	opts.GenesisHash = [32]byte{}
	opts.Anchor = &CommittedAnchor{Height: 1, Hash: committedFixtureHash(1)}
	opts.HistoricalEndpoints = map[string]string{"archive": archive.server.URL}
	s := repair997Open(t, opts)
	health := s.EndpointHealth()
	if len(health) != 2 {
		t.Fatalf("corroborated endpoints %+v", health)
	}
	var candidate *committedEndpoint
	for _, e := range s.endpoints {
		if !e.historical {
			candidate = e
		}
	}
	if candidate == nil {
		t.Fatal("corroborated primary absent")
	}
	if err := s.corroborate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	isolated := &CommittedSource{opts: s.opts, client: s.client}
	if err := isolated.corroborate(ctx, candidate); err == nil {
		t.Fatal("unverified chain admitted a pruned anchor")
	}
	historical, err := CreateHistorical(ctx, fopts(archive.server.URL), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if historical.Name() != "historical-committed" || historical.RPC == nil || historical.Explorer != nil {
		t.Fatalf("historical sources %+v", historical)
	}
	if err = historical.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = historical.FetchLogs(ctx, FetchOptions{FromBlock: 1, ToBlock: 2}); err == nil {
		t.Fatal("closed historical source fetched a range")
	}
	if _, err = CreateHistorical(ctx, CommittedOptions{}, "", nil); err == nil {
		t.Fatal("unbound historical identity accepted")
	}
	if _, err = CreateHistorical(ctx, fopts(archive.server.URL), "", &CommittedPaxscanOptions{}); err == nil {
		t.Fatal("uncertified explorer admitted")
	}
	opts.HistoricalEndpoints = nil
	if _, err = NewCommittedSource(ctx, opts); err == nil {
		t.Fatal("pruned anchor admitted without a verified peer")
	}
}
