import { notFound } from 'next/navigation';
import { dehydrate, HydrationBoundary, QueryClient } from '@tanstack/react-query';
import { decodeV2Contract, Operations, SdkClient } from '@kindle/sdk';
import { backendOrigins } from '@/core/backend-origins';
import { marketCachePolicy } from '@/core/cache-policy';
import { resolvePoolSnapshot } from '@/core/market/pool-resolver';
import { queryKeys } from '@/core/query-keys';
import TerminalModule from '@/page-modules/terminal';
import { parseTerminalSlug } from '@/widgets/terminal/sub-nav';

interface Props { params: Promise<{ slug: string }>; }

export const metadata = { title: 'Terminal' };

export default async function TerminalSlugPage({ params }: Props) {
  const { slug } = await params;
  const target = parseTerminalSlug(slug);
  if (target.kind === 'invalid') notFound();
  if (target.kind === 'pool') {
    const origins = backendOrigins();
    const market = new SdkClient({
      apiOrigin: origins.data,
      identityOrigin: origins.gateway,
      mediaOrigin: origins.metadata,
      streamOrigin: origins.stream,
      approvedStreamOrigins: [origins.stream],
      production: process.env.NODE_ENV === 'production',
    });
    const operations = new Operations(market);
    const client = new QueryClient();
    await Promise.all([client.prefetchQuery({
      queryKey: queryKeys.poolSnapshot(target.pool),
      queryFn: ({ signal }) => resolvePoolSnapshot(target.pool, { operations }, signal),
      ...marketCachePolicy.poolSnapshot,
    }), client.prefetchQuery({
      queryKey: queryKeys.poolPriceSnapshot(target.pool),
      queryFn: async ({ signal }) => {
        const topic = `market.price.${target.pool}`;
        const response = await market.requestV2(`/v2/snapshots?topics=${encodeURIComponent(topic)}`, 'TopicSnapshotReadResponse', { signal, authenticated: false, idempotentRead: true });
        if (response.quality === 'quarantined' || response.watermark?.integrity === 'quarantined') throw new Error('Market integrity quarantined');
        const entry = response.data.snapshot.topics.find(item => item.topic === topic);
        if (entry?.watermark?.integrity === 'quarantined') throw new Error('Market integrity quarantined');
        return entry ? decodeV2Contract('Price', entry.payload) : null;
      },
      ...marketCachePolicy.poolSnapshot,
    })]);
    return <HydrationBoundary state={dehydrate(client)}><TerminalModule slug={target.pool} /></HydrationBoundary>;
  }
  return <TerminalModule slug={target.view} />;
}
