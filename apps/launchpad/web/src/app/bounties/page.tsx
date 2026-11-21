import type { Metadata } from 'next';
import BountiesPage from '@/page-modules/bounties';

export const metadata: Metadata = { title: 'Bounties' };

export default async function BountiesRoute({
  searchParams,
}: {
  searchParams: Promise<{ tab?: string | string[] }>;
}) {
  const { tab } = await searchParams;
  return <BountiesPage requestedTab={Array.isArray(tab) ? tab[0] ?? null : tab ?? null} />;
}
