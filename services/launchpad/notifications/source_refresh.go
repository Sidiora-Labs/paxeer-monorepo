package notifications

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
)

type storedRefreshSource struct {
	ref      discovery.SourceRef
	document json.RawMessage
}

type resolvedSourceAuthority struct {
	discovery.SourceAuthority
	resource discovery.PublicResource
	listed   map[string]bool
}

func (a resolvedSourceAuthority) ResolvePublic(ctx context.Context, kind, id string) (discovery.PublicResource, error) {
	if err := ctx.Err(); err != nil {
		return discovery.PublicResource{}, err
	}
	if kind != a.resource.Ref.Kind || id != a.resource.Ref.ID {
		return discovery.PublicResource{}, discovery.ErrInvalid
	}
	return a.resource, nil
}

func (a resolvedSourceAuthority) CanonicalPool(ctx context.Context, pool string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if a.listed[pool] {
		return true, nil
	}
	return a.SourceAuthority.CanonicalPool(ctx, pool)
}

type sourceRefreshPass struct {
	runtime                        *HTTPRuntime
	stored                         map[string]storedRefreshSource
	listed, seen                   map[string]bool
	failures                       int
	firstKind, firstID, firstCause string
}

func (p *sourceRefreshPass) group(ctx context.Context, refs []discovery.SourceRef, resolved bool) error {
	unchanged := []json.RawMessage{}
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.seen[ref.ID] {
			continue
		}
		p.seen[ref.ID] = true
		old, exists := p.stored[ref.ID]
		if ref.Tombstone || (exists && old.ref.Tombstone) {
			continue
		}
		resource := discovery.PublicResource{Ref: ref}
		var err error
		if !resolved {
			if p.runtime.Discovery.Feed.Sources == nil {
				return ErrUnavailable
			}
			resource, err = p.runtime.Discovery.Feed.Sources.ResolvePublic(ctx, ref.Kind, ref.ID)
		}
		if err == nil {
			authority := resolvedSourceAuthority{SourceAuthority: p.runtime.Discovery.Feed.Sources, resource: resource, listed: p.listed}
			if exists && resource.Ref.Public && resource.Ref.Authorized && resource.Ref.Validate() == nil && resource.Ref.Equal(old.ref) {
				for _, pool := range resource.Ref.Pools {
					var canonical bool
					canonical, err = authority.CanonicalPool(ctx, pool)
					if err != nil {
						break
					}
					if !canonical {
						err = discovery.ErrInvalid
						break
					}
				}
				if err == nil {
					unchanged = append(unchanged, old.document)
				}
			} else {
				now := time.Now().UTC()
				event := discovery.PublicEvent{Consumer: "authority-refresh-v2", EventID: resource.Ref.EventID + ":" + now.Format(time.RFC3339Nano), Source: resource.Ref, ObservedAt: now}
				err = p.runtime.Discovery.Store.Consume(ctx, authority, event)
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			p.failures++
			if p.failures == 1 {
				p.firstKind, p.firstID, p.firstCause = ref.Kind, ref.ID, err.Error()
			}
		}
	}
	return p.runtime.Discovery.Store.RenewSources(ctx, unchanged, time.Now().UTC())
}
