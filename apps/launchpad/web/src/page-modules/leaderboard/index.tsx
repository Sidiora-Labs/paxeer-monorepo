'use client';

import { LeaderboardView } from '@/widgets/leaderboard/leaderboard-view';
import { CalloutsRail } from '@/widgets/social/callouts-rail';

export default function LeaderboardModule({ initialTab, initialPeriod }: { initialTab?: string; initialPeriod?: string }) {
  return <LeaderboardView initialTab={initialTab} initialPeriod={initialPeriod} rail={<CalloutsRail />} className="[&_:focus-visible]:outline-focus" />;
}
