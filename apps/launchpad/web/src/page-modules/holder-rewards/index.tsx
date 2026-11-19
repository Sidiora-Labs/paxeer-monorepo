'use client';

import { PageWithFooter } from '@/shell/platform-metrics';
import HolderRewardsView from '@/widgets/holder-rewards/holder-rewards-view';

export default function HolderRewardsModule() {
  return <PageWithFooter><div data-holder-rewards-route className="min-w-0"><HolderRewardsView /></div></PageWithFooter>;
}
