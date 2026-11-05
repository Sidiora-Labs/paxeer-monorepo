import { useRouter } from 'next/router';

import type { TabItem } from '../AdaptiveTabs/types';

import { castToString } from '../../utils/guards';

export default function useActiveTabFromQuery(tabs: Array<TabItem>, defaultTabId?: string) {
  const router = useRouter();
  const tabFromQuery = castToString(router.query.tab);

  if (!tabFromQuery) {
    if (defaultTabId) {
      return tabs.find((tab) => tab.id === defaultTabId);
    }

    return;
  }

  return tabs.find((tab) => {
    const ids = Array.isArray(tab.id) ? tab.id : [ tab.id ];
    return ids.includes(tabFromQuery) || ('subTabs' in tab && tab.subTabs?.includes(tabFromQuery));
  });
}
