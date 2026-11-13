package source

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
)

func TestCommittedExplorerRejectsMalformedDurableRows(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		kind      ErrorKind
	}{
		{"fractional timestamp", `UPDATE public.blocks SET timestamp=timestamp+interval '0.5 second' WHERE number=101`, IncompleteError},
		{"short block hash", `UPDATE public.blocks SET hash=decode('11','hex') WHERE number=101`, IncompleteError},
		{"parent disagreement", `UPDATE public.blocks SET parent_hash=decode(repeat('11',32),'hex') WHERE number=101`, IntegrityError},
		{"short sender", `UPDATE public.transactions SET from_address_hash=decode('11','hex')`, IncompleteError},
		{"transaction order gap", `UPDATE public.transactions SET "index"=2`, IncompleteError},
		{"duplicate transaction position", `INSERT INTO public.transactions SELECT decode(repeat('44',32),'hex'),block_hash,"index",from_address_hash FROM public.transactions`, IntegrityError},
		{"missing block", `DELETE FROM public.blocks WHERE number=100`, IncompleteError},
		{"short log topic", `UPDATE public.logs SET first_topic=decode('11','hex')`, IncompleteError},
		{"log topic gap", `UPDATE public.logs SET first_topic=NULL,second_topic=decode(repeat('11',32),'hex')`, IncompleteError},
		{"log negative index", `UPDATE public.logs SET "index"=-1`, IncompleteError},
		{"log transaction absent", `UPDATE public.logs SET transaction_hash=decode(repeat('22',32),'hex')`, IntegrityError},
		{"log transaction mismatch", `UPDATE public.logs SET transaction_index=1`, IntegrityError},
		{"duplicate log identity", `INSERT INTO public.logs SELECT * FROM public.logs`, IntegrityError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			node := newCapacityChain(t)
			node.head.Store(101)
			live := repair997Open(t, fopts(node.server.URL))
			dsn, pool := canonicaltest.New(t)
			explorer := capacityExplorer(t, dsn, pool, live, 101, node.logs...)
			if _, err := pool.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			result, err := explorer.FetchLogs(ctx, FetchOptions{FromBlock: 100, ToBlock: 101, Historical: true})
			var sourceErr *SourceError
			if !errors.As(err, &sourceErr) || sourceErr.Kind != tc.kind || result.Complete {
				t.Fatalf("invalid explorer rows returned complete=%v: %v", result.Complete, err)
			}
		})
	}
	ctx := context.Background()
	node := newCapacityChain(t)
	node.head.Store(101)
	live := repair997Open(t, fopts(node.server.URL))
	dsn, pool := canonicaltest.New(t)
	explorer := capacityExplorer(t, dsn, pool, live, 101, node.logs...)
	for _, bounds := range [][2]int64{{-1, 101}, {102, 101}, {0, 2000}} {
		if result, err := explorer.FetchLogs(ctx, FetchOptions{FromBlock: bounds[0], ToBlock: bounds[1], Historical: true}); err == nil || result.Complete {
			t.Fatalf("unbounded explorer request %+v %v", bounds, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := explorer.FetchLogs(canceled, FetchOptions{FromBlock: 100, ToBlock: 101, Historical: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled explorer %v", err)
	}
	// Schema validation checks the concrete reader schema at each admission.
	if _, err := pool.Exec(ctx, `ALTER TABLE public.logs ALTER COLUMN first_topic TYPE text USING encode(first_topic,'hex')`); err != nil {
		t.Fatal(err)
	}
	if err := explorer.validateSchema(ctx); err == nil || !strings.Contains(err.Error(), string(InvalidError)) {
		t.Fatalf("changed explorer schema %v", err)
	}
}
