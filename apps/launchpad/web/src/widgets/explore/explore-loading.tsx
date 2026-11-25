'use client';

import { ExploreView } from './explore-view';
import { defaultPreset, PresetTabs } from './preset-tabs';

export function ExploreLoading() {
  return (
    <div aria-busy="true">
      <span className="sr-only">Loading coins</span>
      <ExploreView
        rows={[]}
        loading
        hasMore={false}
        loadingMore={false}
        moreError={null}
        onLoadMore={() => undefined}
        onRetryMore={() => undefined}
        defaultSort={defaultPreset.defaultSort}
        sortResetKey={defaultPreset.id}
        sourceLabel="coins"
        leading={<PresetTabs active={defaultPreset.id} />}
        caption={`${defaultPreset.label} coins`}
        empty={null}
      />
    </div>
  );
}

export function ExplorePageLoading() {
  return (
    <div className="mx-auto flex w-full max-w-content-max-width flex-col gap-4 px-4 py-4 md:px-6">
      <h1 className="text-lg font-medium text-text">Explore coins</h1>
      <ExploreLoading />
    </div>
  );
}
