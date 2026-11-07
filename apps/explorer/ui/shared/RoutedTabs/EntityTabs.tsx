import React from 'react';

import type { TabItemRegular } from 'toolkit/components/AdaptiveTabs/types';

import RoutedTabs from 'toolkit/components/RoutedTabs/RoutedTabs';

export function getEntityTabIds(tabs: Array<TabItemRegular>): Array<string> {
  return tabs.flatMap((tab) => [ ...(Array.isArray(tab.id) ? tab.id : [ tab.id ]), ...(tab.subTabs ?? []) ]);
}

export function entityTabGroup(id: string, title: string, tabs: Array<TabItemRegular>): TabItemRegular {
  return {
    id,
    title,
    subTabs: getEntityTabIds(tabs),
    component: tabs.length ? <RoutedTabs tabs={ tabs } displayMode="entity"/> : <p>No additional sections are available for this entity.</p>,
  };
}
