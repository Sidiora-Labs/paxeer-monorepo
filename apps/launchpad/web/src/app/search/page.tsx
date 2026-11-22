import type { Metadata } from 'next';
import SearchModule from '@/page-modules/search';

export const metadata: Metadata = { title: 'Search coins and users' };

export default async function SearchPage({ searchParams }: { searchParams: Promise<{ q?: string | string[] }> }) {
  const { q } = await searchParams;
  return <SearchModule initialQuery={Array.isArray(q) ? q[0] ?? '' : q ?? ''} />;
}
