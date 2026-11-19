import type { Metadata } from 'next';
import LeaderboardModule from '@/page-modules/leaderboard';

export const metadata: Metadata = { title: 'Leaderboard', description: 'See the traders and squads leading Sidiora.' };

export default async function LeaderboardPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const search = await searchParams;
  return <LeaderboardModule initialTab={typeof search.tab === 'string' ? search.tab : undefined} initialPeriod={typeof search.period === 'string' ? search.period : undefined} />;
}
