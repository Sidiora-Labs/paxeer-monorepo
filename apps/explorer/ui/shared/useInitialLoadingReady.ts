import React from 'react';

import durations from 'toolkit/theme/foundations/durations';

const SKELETON_DELAY = Number.parseInt(durations.shimmer.value, 10);

export default function useInitialLoadingReady(isLoading: boolean) {
  const [ ready, setReady ] = React.useState(!isLoading);

  React.useEffect(() => {
    if (ready) {
      return;
    }
    if (!isLoading) {
      setReady(true);
      return;
    }
    const timeout = window.setTimeout(() => setReady(true), SKELETON_DELAY);
    return () => window.clearTimeout(timeout);
  }, [ isLoading, ready ]);

  return !isLoading || ready;
}
