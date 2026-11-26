'use client';

import { useSyncExternalStore } from 'react';
import { deploymentColorMode, getColorMode, getColorScheme, subscribeColorMode } from './color-mode';

export function useColorMode() {
  const mode = useSyncExternalStore(subscribeColorMode, getColorMode, deploymentColorMode);
  const scheme = useSyncExternalStore(subscribeColorMode, getColorScheme, () => {
    const initial = deploymentColorMode();
    return initial === 'device' ? 'light' : initial;
  });
  return { mode, scheme };
}

export function ColorModeSync() {
  useColorMode();
  return null;
}
